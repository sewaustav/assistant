package speech_recognition

import (
	"context"
	"fmt"
	"log"
)

type Recognizer struct {
	capture *AudioCapture
	stt     *GigaSTTClient
	rawChan chan string
	started bool
}

func NewRecognizer() (*Recognizer, error) {
	capture, err := NewAudioCapture()
	if err != nil {
		return nil, err
	}
	stt, err := NewGigaSTTClient("ws://127.0.0.1:9876/v1/ws")
	if err != nil {
		capture.Stop()
		return nil, err
	}
	return &Recognizer{
		capture: capture,
		stt:     stt,
		rawChan: make(chan string, 10),
	}, nil
}

// Start запускает захват микрофона и STT.
// Возвращает только ошибку. Канал доступен через RawChan().
func (r *Recognizer) Start(ctx context.Context) error {
	if r.started {
		return fmt.Errorf("Recognizer уже запущен")
	}

	if err := r.capture.Start(); err != nil {
		return err
	}

	rawChan, err := r.stt.TranscribeStream(ctx, r.capture.AudioChan())
	if err != nil {
		r.capture.Stop()
		return err
	}

	// Пробрасываем сырой JSON в наш канал.
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
	r.capture.Stop()
	r.started = false
}