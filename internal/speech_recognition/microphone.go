package speech_recognition

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gen2brain/malgo"
)

// AudioCapture управляет захватом звука с микрофона.
type AudioCapture struct {
	ctx      *malgo.AllocatedContext
	device   *malgo.Device
	dataChan chan []byte
	stopChan chan struct{}
	once     sync.Once
	muted    atomic.Bool
}

// NewAudioCapture создаёт новый захватчик для 16kHz моно PCM.
func NewAudioCapture() (*AudioCapture, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации аудио контекста: %w", err)
	}

	return &AudioCapture{
		ctx:      ctx,
		dataChan: make(chan []byte, 100),
		stopChan: make(chan struct{}),
	}, nil
}

// Start начинает запись с микрофона.
func (c *AudioCapture) Start() error {
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = 1
	deviceConfig.SampleRate = 48000
	deviceConfig.Alsa.NoMMap = 1

	onData := func(_, data []byte, framecount uint32) {
		// Если микрофон замьючен — дропаем чанк, ничего не отправляем.
		if c.muted.Load() {
			return
		}

		buf := make([]byte, len(data))
		copy(buf, data)

		select {
		case c.dataChan <- buf:
		case <-c.stopChan:
		}
	}

	var err error
	c.device, err = malgo.InitDevice(c.ctx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: onData,
	})
	if err != nil {
		return fmt.Errorf("ошибка инициализации устройства: %w", err)
	}

	if err := c.device.Start(); err != nil {
		return fmt.Errorf("ошибка запуска устройства: %w", err)
	}

	return nil
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
		if c.device != nil {
			c.device.Uninit()
		}
		if c.ctx != nil {
			_ = c.ctx.Uninit()
			c.ctx.Free()
		}
	})
}