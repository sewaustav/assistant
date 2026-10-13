package speech_recognition

import (
	"fmt"
	"sync"

	"github.com/gen2brain/malgo"
)

type AudioCapture struct {
	ctx      *malgo.AllocatedContext
	device   *malgo.Device
	dataChan chan []byte
	stopChan chan struct{}
	once     sync.Once
}

func NewAudioCapture() (*AudioCapture, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации аудио контекста: %w", err)
	}

	return &AudioCapture{
		ctx:      ctx,
		dataChan: make(chan []byte, 100), // Буфер на 100 чанков
		stopChan: make(chan struct{}),
	}, nil
}

func (c *AudioCapture) Start() error {
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16 // 16-bit PCM
	deviceConfig.Capture.Channels = 1             // Моно
	deviceConfig.SampleRate = 48000               // 16 kHz
	deviceConfig.Alsa.NoMMap = 1                  // Важный фикс для Linux ALSA [citation:1]

	onData := func(_, data []byte, framecount uint32) {
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

func (c *AudioCapture) AudioChan() <-chan []byte {
	return c.dataChan
}

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