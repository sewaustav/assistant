package speech_recognition

import (
	"context"
	"fmt"
	"log"
)

// Recognizer связывает микрофон и gigastt.
type Recognizer struct {
	capture    *AudioCapture
	stt        *GigaSTTClient
	rawChan    chan string
	started    bool
	sampleRate int
}

// NewRecognizer создаёт распознаватель.
func NewRecognizer(serverURL string, sampleRate int) (*Recognizer, error) {
	capture, err := NewAudioCapture(sampleRate)
	if err != nil {
		return nil, err
	}
	return &Recognizer{
		capture:    capture,
		stt:        NewGigaSTTClient(serverURL),
		rawChan:    make(chan string, 10),
		sampleRate: sampleRate,
	}, nil
}

// Start запускает захват микрофона и STT.
// Возвращает только ошибку. Канал доступен через RawChan().
func (r *Recognizer) Start(ctx context.Context) error {
	if r.started {
		return fmt.Errorf("Recognizer уже запущен")
	}

	// Сначала микрофон: нужно узнать фактическую частоту устройства,
	// чтобы правильно настроить gigastt (особенно на Windows).
	if err := r.capture.Start(); err != nil {
		return err
	}

	rawChan, err := r.stt.TranscribeStream(ctx, r.capture.AudioChan(), r.capture.SampleRate())
	if err != nil {
		r.capture.Stop()
		return err
	}

	go func() {
		for raw := range rawChan {
			select {
			case r.rawChan <- raw:
			case <-ctx.Done():
				return
			}
		}
	}()

	r.started = true
	log.Println("[recognizer] Слушаю микрофон...")
	return nil
}

// RawChan возвращает канал с сырым JSON от gigastt.
func (r *Recognizer) RawChan() <-chan string {
	return r.rawChan
}

// Mute приостанавливает отправку аудио.
func (r *Recognizer) Mute() {
	r.capture.Mute()
}

// Unmute возобновляет отправку аудио.
func (r *Recognizer) Unmute() {
	r.capture.Unmute()
}

// Stop останавливает Recognizer.
func (r *Recognizer) Stop() {
	r.stt.Close()
	r.capture.Stop()
	r.started = false
}
