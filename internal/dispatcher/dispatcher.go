package dispatcher

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

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
	LightOn   bool
	LastCmd   string
	LastCmdAt time.Time
}

// Dispatcher связывает STT и логику команд.
type Dispatcher struct {
	state      *State
	textChan   <-chan string
	stopFn     func()
	tts        *tts.Client
	recognizer *speech_recognition.Recognizer
}

func New(textChan <-chan string, stopFn func(), recognizer *speech_recognition.Recognizer) *Dispatcher {
	ttsClient, err := tts.New("")
	if err != nil {
		log.Fatalf("TTS init: %v", err)
	}

	return &Dispatcher{
		state:      &State{},
		textChan:   textChan,
		stopFn:     stopFn,
		tts:        ttsClient,
		recognizer: recognizer,
	}
}

func (d *Dispatcher) Close() {
	if d.tts != nil {
		d.tts.Close()
	}
}

// Run — главный цикл диспетчера.
func (d *Dispatcher) Run(ctx context.Context) {
	log.Println("[dispatcher] Запущен. Ожидаю команды...")
	for {
		select {
		case <-ctx.Done():
			log.Println("[dispatcher] Контекст отменён, выходим.")
			return
		case raw, ok := <-d.textChan:
			if !ok {
				log.Println("[dispatcher] Канал текста закрыт, выходим.")
				return
			}
			d.handleRaw(ctx, raw)
		}
	}
}

// handleRaw парсит сырой JSON от gigastt и решает, что делать.
func (d *Dispatcher) handleRaw(ctx context.Context, raw string) {
	var msg GigaSTTMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		log.Printf("[dispatcher] Ошибка парсинга JSON: %v | raw=%s", err, raw)
		return
	}

	switch msg.Type {
	case "ready":
		log.Printf("[dispatcher] Модель готова, сервер на связи.")
	case "partial":
		// Промежуточный результат — просто показываем, команд не трогаем.
		log.Printf("[dispatcher][partial] %s", msg.Text)
	case "final":
		log.Printf("[dispatcher][final] %s (conf=%.2f)", msg.Text, msg.Confidence)
		d.handleText(ctx, msg.Text)
	default:
		// Служебные/неизвестные типы — просто логируем.
		log.Printf("[dispatcher][%s] %s", msg.Type, raw)
	}
}

func (d *Dispatcher) handleText(ctx context.Context, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	log.Printf("[dispatcher] Распознано: %q — озвучиваю", text)

	// 1. Мьютим микрофон ДО начала TTS.
	d.recognizer.Mute()

	// 2. Синтез + воспроизведение.
	duration, err := d.tts.Speak(ctx, text, "xenia")
	if err != nil {
		log.Printf("[dispatcher] TTS ошибка: %v", err)
		d.recognizer.Unmute()
		return
	}

	// 3. Хвост эха: ждём, пока звук затихнет в комнате.
	const echoTail = 300 * time.Millisecond
	time.Sleep(echoTail)

	log.Printf("[dispatcher] TTS отыграл %.2f сек + хвост %v, размьючиваю", duration.Seconds(), echoTail)

	// 4. Размьютим микрофон.
	d.recognizer.Unmute()
}



func (d *Dispatcher) State() *State {
	return d.state
}
