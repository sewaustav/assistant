//go:build windows

package speech_recognition

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

// Захват звука на Windows через winmm (waveIn). Это чистый Go + syscall,
// поэтому Windows-сборка не требует CGO и MinGW.
const (
	waveMapper    = 0xFFFFFFFF // WAVE_MAPPER
	waveFormatPCM = 1

	callbackEvent = 0x00050000 // CALLBACK_EVENT

	whdrDone = 0x00000001 // WHDR_DONE

	waitObject0 = 0x00000000
	waitTimeout = 0x00000102

	waveBufferCount = 8
	waveBufferMs    = 20
)

var (
	winmm    = syscall.NewLazyDLL("winmm.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procWaveInOpen            = winmm.NewProc("waveInOpen")
	procWaveInPrepareHeader   = winmm.NewProc("waveInPrepareHeader")
	procWaveInUnprepareHeader = winmm.NewProc("waveInUnprepareHeader")
	procWaveInAddBuffer       = winmm.NewProc("waveInAddBuffer")
	procWaveInStart           = winmm.NewProc("waveInStart")
	procWaveInStop            = winmm.NewProc("waveInStop")
	procWaveInReset           = winmm.NewProc("waveInReset")
	procWaveInClose           = winmm.NewProc("waveInClose")

	procCreateEventW        = kernel32.NewProc("CreateEventW")
	procWaitForSingleObject = kernel32.NewProc("WaitForSingleObject")
	procCloseHandle         = kernel32.NewProc("CloseHandle")
)

type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	Size           uint16
}

type waveHdr struct {
	Data          *byte
	BufferLength  uint32
	BytesRecorded uint32
	User          uintptr
	Flags         uint32
	Loops         uint32
	Next          *waveHdr
	Reserved      uintptr
}

// waveInCapture — бэкенд записи через winmm.
type waveInCapture struct {
	onData func([]byte)

	mu      sync.Mutex
	handle  uintptr
	event   uintptr
	headers []*waveHdr
	buffers [][]byte
	stop    chan struct{}
	done    chan struct{}
	loop    bool
	stopped bool
}

func newCaptureBackend(onData func([]byte)) (captureBackend, error) {
	return &waveInCapture{
		onData: onData,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}, nil
}

func (w *waveInCapture) Start(rate int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stopped {
		return fmt.Errorf("захват уже остановлен")
	}
	if w.handle != 0 {
		return fmt.Errorf("захват уже запущен")
	}
	if rate <= 0 {
		return fmt.Errorf("некорректная частота дискретизации: %d", rate)
	}

	format := waveFormatEx{
		FormatTag:      waveFormatPCM,
		Channels:       1,
		SamplesPerSec:  uint32(rate),
		AvgBytesPerSec: uint32(rate * 2),
		BlockAlign:     2,
		BitsPerSample:  16,
	}

	event, _, err := procCreateEventW.Call(0, 0, 0, 0)
	if event == 0 {
		return fmt.Errorf("CreateEventW: %v", err)
	}
	w.event = event

	var handle uintptr
	if res, _, _ := procWaveInOpen.Call(
		uintptr(unsafe.Pointer(&handle)),
		uintptr(waveMapper),
		uintptr(unsafe.Pointer(&format)),
		event,
		0,
		callbackEvent,
	); res != 0 {
		w.closeLocked()
		return fmt.Errorf("waveInOpen: %s", mmResultText(res))
	}
	w.handle = handle

	frames := rate / (1000 / waveBufferMs)
	if frames < 1 {
		frames = 1
	}
	bufSize := frames * int(format.BlockAlign)
	if bufSize < 2 {
		bufSize = 2
	}

	w.buffers = make([][]byte, waveBufferCount)
	w.headers = make([]*waveHdr, waveBufferCount)
	for i := 0; i < waveBufferCount; i++ {
		buf := make([]byte, bufSize)
		hdr := &waveHdr{
			Data:         &buf[0],
			BufferLength: uint32(bufSize),
		}
		w.buffers[i] = buf
		w.headers[i] = hdr

		if res, _, _ := procWaveInPrepareHeader.Call(w.handle, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr)); res != 0 {
			w.closeLocked()
			return fmt.Errorf("waveInPrepareHeader: %s", mmResultText(res))
		}
		if res := w.addBuffer(hdr); res != 0 {
			w.closeLocked()
			return fmt.Errorf("waveInAddBuffer: %s", mmResultText(res))
		}
	}

	if res, _, _ := procWaveInStart.Call(w.handle); res != 0 {
		w.closeLocked()
		return fmt.Errorf("waveInStart: %s", mmResultText(res))
	}

	w.loop = true
	go w.loopFunc()
	return nil
}

func (w *waveInCapture) loopFunc() {
	defer close(w.done)

	for {
		select {
		case <-w.stop:
			return
		default:
		}

		res, _, _ := procWaitForSingleObject.Call(w.event, 200)
		switch res {
		case waitTimeout:
			continue
		case waitObject0:
			// Есть готовые буферы, обрабатываем все.
		default:
			return
		}

		for _, hdr := range w.headers {
			if hdr == nil || hdr.Flags&whdrDone == 0 {
				continue
			}
			if hdr.BytesRecorded > 0 {
				data := unsafe.Slice(hdr.Data, int(hdr.BytesRecorded))
				w.onData(data)
				hdr.BytesRecorded = 0
			}
			hdr.Flags &^= whdrDone
			if res := w.addBuffer(hdr); res != 0 {
				return
			}
		}
	}
}

func (w *waveInCapture) addBuffer(hdr *waveHdr) uintptr {
	res, _, _ := procWaveInAddBuffer.Call(w.handle, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr))
	return res
}

func (w *waveInCapture) Stop() {
	w.mu.Lock()
	if w.stopped {
		w.mu.Unlock()
		return
	}
	w.stopped = true
	loop := w.loop
	w.mu.Unlock()

	if loop {
		close(w.stop)
		<-w.done
	}

	w.mu.Lock()
	w.closeLocked()
	w.mu.Unlock()
}

// closeLocked освобождает ресурсы. Вызывается либо при ошибке Start,
// либо после остановки цикла записи.
func (w *waveInCapture) closeLocked() {
	if w.handle != 0 {
		_, _, _ = procWaveInStop.Call(w.handle)
		_, _, _ = procWaveInReset.Call(w.handle)
		for _, hdr := range w.headers {
			if hdr == nil {
				continue
			}
			_, _, _ = procWaveInUnprepareHeader.Call(w.handle, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr))
		}
		_, _, _ = procWaveInClose.Call(w.handle)
		w.handle = 0
	}
	if w.event != 0 {
		_, _, _ = procCloseHandle.Call(w.event)
		w.event = 0
	}
	w.headers = nil
	w.buffers = nil
	w.loop = false
}

func mmResultText(code uintptr) string {
	switch code {
	case 2:
		return "неверный идентификатор устройства (MMSYSERR_BADDEVICEID)"
	case 4:
		return "устройство уже используется (MMSYSERR_ALLOCATED)"
	case 6:
		return "нет аудиодрайвера (MMSYSERR_NODRIVER)"
	case 7:
		return "недостаточно памяти (MMSYSERR_NOMEM)"
	case 8:
		return "операция не поддерживается (MMSYSERR_NOTSUPPORTED)"
	case 11:
		return "неверный параметр (MMSYSERR_INVALPARAM)"
	case 32:
		return "устройство не поддерживает формат (WAVERR_BADFORMAT)"
	case 33:
		return "буферы ещё используются (WAVERR_STILLPLAYING)"
	case 34:
		return "буфер не подготовлен (WAVERR_UNPREPARED)"
	default:
		return fmt.Sprintf("код MMRESULT %d", code)
	}
}
