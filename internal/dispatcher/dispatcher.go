package dispatcher

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

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
	state    *State
	textChan <-chan string
	stopFn   func()
	tts      *tts.Client
}

func New(textChan <-chan string, stopFn func()) *Dispatcher {
	ttsClient, err := tts.New("")
	if err != nil {
		log.Fatalf("TTS init: %v", err)
	}

	return &Dispatcher{
		state:    &State{},
		textChan: textChan,
		stopFn:   stopFn,
		tts:      ttsClient,
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

// handleText — обработка финального текста как команды.
func (d *Dispatcher) handleText(ctx context.Context, text string) {
	log.Printf("[dispatcher] Распознано: %q — озвучиваю", text)
	
		// Пока просто повторяем за пользователем.
		// Команды (свет, погода, время) — вернём позже, когда решишь,
		// как именно они должны работать.
		if err := d.tts.Speak(ctx, text, "xenia"); err != nil {
			log.Printf("[dispatcher] TTS ошибка: %v", err)
	}
}



func (d *Dispatcher) State() *State {
	return d.state
}
