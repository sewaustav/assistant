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
	voiceChan chan string // сырой JSON от gigastt
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

	// handlers 
	llm *llm.Handler
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
	stopFn func(),
) *Dispatcher {
	ttsClient, err := tts.New(cfg.TTS.ServerURL)
	if err != nil {
		log.Fatalf("TTS init: %v", err)
	}

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
		llm:          llm.New(cfg),
		stopFn:       stopFn,
	}
}

func (d *Dispatcher) Close() {
	if d.tts != nil {
		d.tts.Close()
	}
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
		d.switchMode(ctx, config.Mode(parts[1]))

	case "quit", "exit":
		log.Println("[dispatcher] Команда выхода.")
		if d.stopFn != nil {
			d.stopFn()
		}

	default:
		d.responseChan <- fmt.Sprintf("Неизвестная команда: /%s", parts[0])
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
	}
}

func (d *Dispatcher) handleText(ctx context.Context, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	log.Printf("[dispatcher] Обрабатываю: %q", text)

	if d.state.Mode == config.ModeVoice {
		cli.PrintUser(text)
	}

	var response string
	var err error

	if d.cfg.LLM.Enabled {
		response, err = d.llm.Handle(ctx, text)
	} else {
		response = text
	}

	if err != nil {
		log.Printf("[dispatcher] Ошибка обработки: %v", err)
		d.responseChan <- fmt.Sprintf("Ошибка: %v", err)
		return
	}

	// TTS в голосовом режиме.
	if d.state.Mode == config.ModeVoice && d.recognizer != nil {
		d.recognizer.Mute()

		duration, err := d.tts.Speak(ctx, response, d.cfg.TTS.Speaker)
		if err != nil {
			log.Printf("[dispatcher] TTS ошибка: %v", err)
			d.recognizer.Unmute()
			d.responseChan <- fmt.Sprintf("TTS ошибка: %v", err)
			return
		}

		echoTail := time.Duration(d.cfg.TTS.EchoTailMs) * time.Millisecond
		time.Sleep(echoTail)

		log.Printf("[dispatcher] TTS отыграл %.2f сек + хвост %v", duration.Seconds(), echoTail)
		d.recognizer.Unmute()
	}

	d.responseChan <- response
}

func (d *Dispatcher) State() *State {
	return d.state
}

// startVoice поднимает сервера и запускает Recognizer.
func (d *Dispatcher) startVoice(ctx context.Context) error {
	// 1. Сервера.
	if err := d.mgr.Start(ctx, "gigastt"); err != nil {
		return fmt.Errorf("gigastt: %w", err)
	}
	if err := d.mgr.Start(ctx, "silero"); err != nil {
		return fmt.Errorf("silero: %w", err)
	}

	// 2. Recognizer — создаём заново.
	rec, err := speech_recognition.NewRecognizer()
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

// stopVoice гасит Recognizer и сервера.
func (d *Dispatcher) stopVoice() {
	if d.recognizer != nil {
		d.recognizer.Stop()
		d.recognizer = nil
	}
	_ = d.mgr.Stop("gigastt")
	_ = d.mgr.Stop("silero")
}