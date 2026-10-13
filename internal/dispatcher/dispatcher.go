package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
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
}

func New(textChan <-chan string, stopFn func()) *Dispatcher {
	return &Dispatcher{
		state:    &State{},
		textChan: textChan,
		stopFn:   stopFn,
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
	cmd := ParseCommand(text)
	log.Printf("[dispatcher] Распознано: %q -> команда: %s", text, cmd.Type)

	d.state.mu.Lock()
	d.state.LastCmd = string(cmd.Type)
	d.state.LastCmdAt = time.Now()
	d.state.mu.Unlock()

	switch cmd.Type {
	case CmdWeather:
		d.weather()
	case CmdTime:
		d.currentTime()
	case CmdExit:
		log.Println("[dispatcher] Команда выхода.")
		if d.stopFn != nil {
			d.stopFn()
		}
	case CmdUnknown:
		log.Printf("[dispatcher] Неизвестная команда: %q", text)
	}
}

// --- Обработчики команд (заглушки) ---

func (d *Dispatcher) lightOn() {
	d.state.mu.Lock()
	d.state.LightOn = true
	d.state.mu.Unlock()
	fmt.Println(">>> Свет включён (заглушка).")
}

func (d *Dispatcher) lightOff() {
	d.state.mu.Lock()
	d.state.LightOn = false
	d.state.mu.Unlock()
	fmt.Println(">>> Свет выключен (заглушка).")
}

func (d *Dispatcher) weather() {
	fmt.Println(">>> Запрос погоды (заглушка). Тут будет вызов API.")
}

func (d *Dispatcher) currentTime() {
	now := time.Now().Format("15:04:05")
	fmt.Printf(">>> Сейчас %s\n", now)
}

func (d *Dispatcher) State() *State {
	return d.state
}