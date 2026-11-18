package speech_recognition

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
)

// captureBackend — платформенный источник PCM16-аудио.
// Каждая ОС реализует его по-своему: malgo (Linux/macOS) или winmm (Windows).
type captureBackend interface {
	Start(rate int) error
	Stop()
}

// AudioCapture управляет захватом звука с микрофона.
type AudioCapture struct {
	dataChan chan []byte
	stopChan chan struct{}
	once     sync.Once
	muted    atomic.Bool

	sampleRate int
	backend    captureBackend
}

// NewAudioCapture создаёт захватчик. preferredRate — желаемая частота
// (например, 48000 из конфига). Если устройство её не поддерживает,
// будут перебраны стандартные варианты.
func NewAudioCapture(preferredRate int) (*AudioCapture, error) {
	c := &AudioCapture{
		dataChan:   make(chan []byte, 100),
		stopChan:   make(chan struct{}),
		sampleRate: preferredRate,
	}

	backend, err := newCaptureBackend(c.deliver)
	if err != nil {
		return nil, err
	}
	c.backend = backend
	return c, nil
}

// deliver копирует данные от бэкенда и отправляет их в общий канал.
func (c *AudioCapture) deliver(data []byte) {
	if c.muted.Load() || len(data) == 0 {
		return
	}

	buf := make([]byte, len(data))
	copy(buf, data)

	select {
	case c.dataChan <- buf:
	case <-c.stopChan:
	}
}

// Start начинает запись с микрофона.
func (c *AudioCapture) Start() error {
	rates := candidateRates(c.sampleRate)
	var lastErr error
	for _, rate := range rates {
		if err := c.backend.Start(rate); err != nil {
			lastErr = err
			log.Printf("[microphone] частота %d Гц не подошла: %v", rate, err)
			continue
		}
		c.sampleRate = rate
		log.Printf("[microphone] захват запущен: %d Гц, моно, PCM16", rate)
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("нет доступных частот дискретизации")
	}
	return fmt.Errorf("не удалось открыть микрофон: %w", lastErr)
}

func candidateRates(preferred int) []int {
	out := make([]int, 0, 5)
	seen := make(map[int]struct{})
	for _, rate := range []int{preferred, 48000, 44100, 32000, 16000} {
		if rate <= 0 {
			continue
		}
		if _, ok := seen[rate]; ok {
			continue
		}
		seen[rate] = struct{}{}
		out = append(out, rate)
	}
	return out
}

// SampleRate возвращает фактическую частоту захвата.
func (c *AudioCapture) SampleRate() int {
	return c.sampleRate
}

// Mute приостанавливает отправку аудио в канал.
func (c *AudioCapture) Mute() {
	c.muted.Store(true)
}

// Unmute возобновляет отправку аудио.
func (c *AudioCapture) Unmute() {
	c.muted.Store(false)
}

// IsMuted возвращает текущее состояние.
func (c *AudioCapture) IsMuted() bool {
	return c.muted.Load()
}

// AudioChan возвращает канал с сырыми аудио-данными.
func (c *AudioCapture) AudioChan() <-chan []byte {
	return c.dataChan
}

// Stop останавливает запись и освобождает ресурсы.
func (c *AudioCapture) Stop() {
	c.once.Do(func() {
		close(c.stopChan)
		if c.backend != nil {
			c.backend.Stop()
		}
	})
}
