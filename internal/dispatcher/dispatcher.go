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
	voiceChan <-chan string // сырой JSON от gigastt
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
}

// New создаёт диспетчер.
func New(
	cfg *config.Config,
	voiceChan <-chan string,
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

// switchMode переключает режим работы.
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
		// Поднимаем сервера для голоса.
		if err := d.mgr.Start(ctx, "gigastt"); err != nil {
			log.Printf("[dispatcher] Ошибка запуска gigastt: %v", err)
			d.responseChan <- fmt.Sprintf("Ошибка запуска gigastt: %v", err)
			return
		}
		if err := d.mgr.Start(ctx, "silero"); err != nil {
			log.Printf("[dispatcher] Ошибка запуска silero: %v", err)
			d.responseChan <- fmt.Sprintf("Ошибка запуска silero: %v", err)
			return
		}
		// TODO: запустить Recognizer (микрофон + WebSocket).

	case config.ModeText:
		// Гасим сервера для голоса.
		if err := d.mgr.Stop("gigastt"); err != nil {
			log.Printf("[dispatcher] Ошибка остановки gigastt: %v", err)
		}
		if err := d.mgr.Stop("silero"); err != nil {
			log.Printf("[dispatcher] Ошибка остановки silero: %v", err)
		}
		// TODO: остановить Recognizer.
	}

	// Обновляем CLI, чтобы промпт сменился.
	if d.cli != nil {
		d.cli.UpdateMode(newMode)
	}
	d.responseChan <- fmt.Sprintf("Режим переключён на: %s", newMode)
}

// handleRaw парсит сырой JSON от gigastt.
func (d *Dispatcher) handleRaw(ctx context.Context, raw string) {
	var msg GigaSTTMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		log.Printf("[dispatcher] Ошибка парсинга JSON: %v | raw=%s", err, raw)
		return
	}

	switch msg.Type {
	case "ready":
		log.Printf("[dispatcher] Модель готова.")
	case "partial":
		log.Printf("[dispatcher][partial] %s", msg.Text)
	case "final":
		log.Printf("[dispatcher][final] %s (conf=%.2f)", msg.Text, msg.Confidence)
		d.handleText(ctx, msg.Text)
	default:
		log.Printf("[dispatcher][%s] %s", msg.Type, raw)
	}
}

// handleText обрабатывает финальный текст (из голоса или CLI).
func (d *Dispatcher) handleText(ctx context.Context, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	log.Printf("[dispatcher] Обрабатываю: %q", text)

	response := text

	// В голосовом режиме — мьютим и озвучиваем (если recognizer есть).
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