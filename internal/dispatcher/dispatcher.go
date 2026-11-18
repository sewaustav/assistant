package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/sewaustav/assistant/internal/cli"
	"github.com/sewaustav/assistant/internal/config"
	"github.com/sewaustav/assistant/internal/lifecycle"
	"github.com/sewaustav/assistant/internal/llm"
	"github.com/sewaustav/assistant/internal/local"
	"github.com/sewaustav/assistant/internal/speech_recognition"
	"github.com/sewaustav/assistant/pkg/tts"
)

// GigaSTTMessage — структура сообщения от gigastt.
type GigaSTTMessage struct {
	Type       string  `json:"type"`
	Text       string  `json:"text"`
	Committed  string  `json:"committed"`
	Tentative  string  `json:"tentative"`
	IsFinal    bool    `json:"is_final"`
	Confidence float64 `json:"confidence"`
}

// State — локальное состояние ассистента.
type State struct {
	mu        sync.RWMutex
	Mode      config.Mode
	LightOn   bool
	LastCmd   string
	LastCmdAt time.Time
}

// Dispatcher связывает STT, TTS и логику.
type Dispatcher struct {
	state *State
	cfg   *config.Config

	// Входы
	voiceChan chan string   // сырой JSON от gigastt
	textChan  <-chan string // текст от CLI
	cmdChan   <-chan string // команды управления (/mode, /quit)

	// Выход
	responseChan chan<- string // ответы для CLI и TTS

	// Зависимости
	tts        *tts.Client
	recognizer *speech_recognition.Recognizer
	mgr        *lifecycle.Manager
	cli        *cli.CLI

	stopFn func()

	// Фоновый запуск TTS-сервера.
	sileroMu     sync.Mutex
	sileroCancel context.CancelFunc
	sileroGen    uint64

	// handlers
	llm   *llm.Handler
	local *local.Handler
}

// New создаёт диспетчер.
func New(
	cfg *config.Config,
	voiceChan chan string,
	textChan <-chan string,
	cmdChan <-chan string,
	responseChan chan<- string,
	recognizer *speech_recognition.Recognizer,
	mgr *lifecycle.Manager,
	cliClient *cli.CLI,
	local *local.Handler,
	llm *llm.Handler,
	stopFn func(),
) *Dispatcher {
	ttsClient := tts.New(cfg.TTS.ServerURL, cfg.TTS.SampleRate)

	return &Dispatcher{
		state: &State{
			Mode: cfg.Mode,
		},
		cfg:          cfg,
		voiceChan:    voiceChan,
		textChan:     textChan,
		cmdChan:      cmdChan,
		responseChan: responseChan,
		tts:          ttsClient,
		recognizer:   recognizer,
		mgr:          mgr,
		cli:          cliClient,
		llm:          llm,
		local:        local,
		stopFn:       stopFn,
	}
}

func (d *Dispatcher) Close() {
	d.stopVoice()
	if d.tts != nil {
		d.tts.Close()
	}
}

// StartVoice запускает распознавание (используется при старте приложения).
func (d *Dispatcher) StartVoice(ctx context.Context) error {
	return d.startVoice(ctx)
}

// SwitchMode переключает режим (используется при старте и из CLI).
func (d *Dispatcher) SwitchMode(ctx context.Context, mode config.Mode) {
	d.switchMode(ctx, mode)
}

// StopVoice останавливает распознавание и сервера.
func (d *Dispatcher) StopVoice() {
	d.stopVoice()
}

// Run — главный цикл диспетчера.
func (d *Dispatcher) Run(ctx context.Context) {
	log.Printf("[dispatcher] Запущен. Режим: %s", d.state.Mode)

	for {
		select {
		case <-ctx.Done():
			log.Println("[dispatcher] Контекст отменён, выходим.")
			return

		case raw, ok := <-d.voiceChan:
			if !ok {
				log.Println("[dispatcher] Голосовой канал закрыт.")
				d.voiceChan = nil
				continue
			}
			if d.state.Mode == config.ModeVoice {
				d.handleRaw(ctx, raw)
			}

		case text, ok := <-d.textChan:
			if !ok {
				log.Println("[dispatcher] Текстовый канал закрыт.")
				d.textChan = nil
				continue
			}
			if d.state.Mode == config.ModeText {
				d.handleText(ctx, text)
			}

		case cmd, ok := <-d.cmdChan:
			if !ok {
				log.Println("[dispatcher] Канал команд закрыт.")
				return
			}
			d.handleCommand(ctx, cmd)
		}
	}
}

// handleCommand обрабатывает команды управления.
func (d *Dispatcher) handleCommand(ctx context.Context, cmd string) {
	cmd = strings.TrimSpace(cmd)
	log.Printf("[dispatcher] Команда: %q", cmd)

	// Убираем префикс / или @.
	cmd = strings.TrimLeft(cmd, "/@")

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "mode":
		if len(parts) < 2 {
			d.responseChan <- "Использование: /mode voice | /mode text"
			return
		}
		mode := config.Mode(strings.ToLower(parts[1]))
		if mode != config.ModeVoice && mode != config.ModeText {
			d.responseChan <- "Режим должен быть voice или text"
			return
		}
		d.switchMode(ctx, mode)

	case "quit", "exit":
		log.Println("[dispatcher] Команда выхода.")
		if d.stopFn != nil {
			d.stopFn()
		}

	default:
		d.responseChan <- fmt.Sprintf("Неизвестная команда: /%s. Напиши /help", parts[0])
	}
}

func (d *Dispatcher) switchMode(ctx context.Context, newMode config.Mode) {
	d.state.mu.Lock()
	oldMode := d.state.Mode
	d.state.Mode = newMode
	d.state.mu.Unlock()

	if oldMode == newMode {
		d.responseChan <- fmt.Sprintf("Уже в режиме: %s", newMode)
		return
	}

	log.Printf("[dispatcher] Переключение режима: %s -> %s", oldMode, newMode)

	switch newMode {
	case config.ModeVoice:
		if err := d.startVoice(ctx); err != nil {
			log.Printf("[dispatcher] Ошибка запуска voice: %v", err)
			d.responseChan <- fmt.Sprintf("Ошибка запуска voice: %v", err)
			// Откатываемся обратно в text.
			d.state.mu.Lock()
			d.state.Mode = config.ModeText
			d.state.mu.Unlock()
			if d.cli != nil {
				d.cli.UpdateMode(config.ModeText)
			}
			return
		}

	case config.ModeText:
		d.stopVoice()
	}

	if d.cli != nil {
		d.cli.UpdateMode(newMode)
	}
	d.responseChan <- fmt.Sprintf("Режим переключён на: %s", newMode)
}

func (d *Dispatcher) handleRaw(ctx context.Context, raw string) {
	var msg GigaSTTMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		log.Printf("[dispatcher] Ошибка парсинга JSON: %v", err)
		return
	}

	switch msg.Type {
	case "ready":
		log.Printf("[dispatcher] Модель готова.") // только в файл, не в CLI
	case "final":
		log.Printf("[dispatcher][final] %s (conf=%.2f)", msg.Text, msg.Confidence)
		d.handleText(ctx, msg.Text)
	case "error":
		log.Printf("[dispatcher][gigastt error] %s", raw)
	}
}

func (d *Dispatcher) handleText(ctx context.Context, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	log.Printf("[dispatcher] Обрабатываю: %q", text)

	cli.PrintUser(text)

	var response string
	var err error

	if d.cfg.LLM.Enabled {
		response, err = d.llm.Handle(ctx, text)
		if err != nil {
			log.Printf("[dispatcher] LLM ошибка, пробую локальные команды: %v", err)
			response, err = d.local.Handle(ctx, text)
		}
	} else {
		response, err = d.local.Handle(ctx, text)
	}

	if err != nil {
		log.Printf("[dispatcher] Ошибка обработки: %v", err)
		d.responseChan <- fmt.Sprintf("Ошибка: %v", err)
		return
	}
	if strings.TrimSpace(response) == "" {
		response = "Готово."
	}

	// TTS в голосовом режиме.
	if d.state.Mode == config.ModeVoice && d.cfg.TTS.Enabled {
		d.speak(ctx, response)
	}

	d.responseChan <- response
}

// speak озвучивает ответ и на время глушит микрофон.
func (d *Dispatcher) speak(ctx context.Context, response string) {
	if d.recognizer != nil {
		d.recognizer.Mute()
		defer d.recognizer.Unmute()
	}

	duration, err := d.tts.Speak(ctx, response, d.cfg.TTS.Speaker)
	if err != nil {
		log.Printf("[dispatcher] TTS ошибка: %v", err)
		return
	}

	echoTail := time.Duration(d.cfg.TTS.EchoTailMs) * time.Millisecond
	select {
	case <-ctx.Done():
	case <-time.After(echoTail):
	}
	log.Printf("[dispatcher] TTS отыграл %.2f сек + хвост %v", duration.Seconds(), echoTail)
}

func (d *Dispatcher) State() *State {
	return d.state
}

// startVoice поднимает сервера и запускает Recognizer.
func (d *Dispatcher) startVoice(ctx context.Context) error {
	if !d.cfg.ASR.Enabled {
		return fmt.Errorf("распознавание речи выключено (asr.enabled=false)")
	}
	if !d.mgr.Registered("gigastt") {
		return fmt.Errorf("gigastt не найден. Установи: cargo install gigastt")
	}

	// 1. Сервера.
	if err := d.mgr.Start(ctx, "gigastt"); err != nil {
		return fmt.Errorf("gigastt: %w", err)
	}
	// TTS не должен мешать распознаванию: Silero грузится в фоне,
	// а если он не поднялся — просто отвечаем текстом.
	if d.cfg.TTS.Enabled && d.mgr.Registered("silero") {
		d.startSilero(ctx)
	}

	// 2. Recognizer — создаём заново.
	rec, err := speech_recognition.NewRecognizer(d.cfg.ASR.ServerURL, d.cfg.ASR.SampleRate)
	if err != nil {
		return fmt.Errorf("recognizer: %w", err)
	}
	if err := rec.Start(ctx); err != nil {
		rec.Stop()
		return fmt.Errorf("recognizer start: %w", err)
	}
	d.recognizer = rec

	// 3. Пробрасываем rawChan в voiceChan.
	go func() {
		for raw := range rec.RawChan() {
			select {
			case d.voiceChan <- raw:
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// startSilero поднимает TTS-сервер в фоне, чтобы загрузка модели
// не задерживала запуск распознавания речи.
func (d *Dispatcher) startSilero(ctx context.Context) {
	d.sileroMu.Lock()
	if d.sileroCancel != nil {
		d.sileroMu.Unlock()
		return
	}
	d.sileroGen++
	gen := d.sileroGen
	startCtx, cancel := context.WithCancel(ctx)
	d.sileroCancel = cancel
	d.sileroMu.Unlock()

	go func() {
		err := d.mgr.Start(startCtx, "silero")
		if err != nil && startCtx.Err() == nil {
			log.Printf("[dispatcher] silero не запустился, TTS будет недоступен: %v", err)
		}

		d.sileroMu.Lock()
		if d.sileroGen == gen {
			d.sileroCancel = nil
		}
		d.sileroMu.Unlock()
		cancel()
	}()
}

// stopSilero прерывает запуск TTS-сервера (если он ещё идёт) и гасит его.
func (d *Dispatcher) stopSilero() {
	d.sileroMu.Lock()
	cancel := d.sileroCancel
	d.sileroCancel = nil
	d.sileroGen++
	d.sileroMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if d.mgr.Registered("silero") {
		_ = d.mgr.Stop("silero")
	}
}

// stopVoice гасит Recognizer и сервера.
func (d *Dispatcher) stopVoice() {
	if d.recognizer != nil {
		d.recognizer.Stop()
		d.recognizer = nil
	}
	d.stopSilero()
	if d.mgr.Registered("gigastt") {
		_ = d.mgr.Stop("gigastt")
	}
}
