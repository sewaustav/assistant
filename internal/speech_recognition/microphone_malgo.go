//go:build !windows && cgo

package speech_recognition

import (
	"fmt"

	"github.com/gen2brain/malgo"
)

// malgoCapture — бэкенд захвата через miniaudio (Linux, macOS и др.).
// Требует CGO, поэтому на Windows используется отдельная реализация на winmm.
type malgoCapture struct {
	ctx    *malgo.AllocatedContext
	device *malgo.Device
	onData func([]byte)
}

func newCaptureBackend(onData func([]byte)) (captureBackend, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации аудио контекста: %w", err)
	}
	return &malgoCapture{ctx: ctx, onData: onData}, nil
}

func (m *malgoCapture) Start(rate int) error {
	if m.device != nil {
		return fmt.Errorf("захват уже запущен")
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = 1
	deviceConfig.SampleRate = uint32(rate)
	deviceConfig.PeriodSizeInMilliseconds = 20
	// На Linux это уменьшает задержку; на других ОС поле игнорируется.
	deviceConfig.Alsa.NoMMap = 1

	onData := func(_, data []byte, _ uint32) {
		m.onData(data)
	}

	device, err := malgo.InitDevice(m.ctx.Context, deviceConfig, malgo.DeviceCallbacks{Data: onData})
	if err != nil {
		return fmt.Errorf("инициализация устройства: %w", err)
	}
	if err := device.Start(); err != nil {
		device.Uninit()
		return fmt.Errorf("запуск устройства: %w", err)
	}
	m.device = device
	return nil
}

func (m *malgoCapture) Stop() {
	if m.device != nil {
		m.device.Uninit()
		m.device = nil
	}
	if m.ctx != nil {
		_ = m.ctx.Uninit()
		m.ctx.Free()
		m.ctx = nil
	}
}
