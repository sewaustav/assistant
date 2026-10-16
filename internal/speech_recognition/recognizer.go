package speech_recognition

import (
	"context"
	"log"
)

type Recognizer struct {
	capture *AudioCapture
	stt     *GigaSTTClient
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
	return &Recognizer{capture: capture, stt: stt}, nil
}

func (r *Recognizer) Start(ctx context.Context) (<-chan string, error) {
	if err := r.capture.Start(); err != nil {
		return nil, err
	}

	rawChan, err := r.stt.TranscribeStream(ctx, r.capture.AudioChan())
	if err != nil {
		r.capture.Stop()
		return nil, err
	}

	log.Println("[recognizer] Слушаю микрофон...")
	return rawChan, nil
}

// Mute приостанавливает отправку аудио в GigaAM.
func (r *Recognizer) Mute() {
	r.capture.Mute()
}

// Unmute возобновляет отправку аудио.
func (r *Recognizer) Unmute() {
	r.capture.Unmute()
}

func (r *Recognizer) Stop() {
	r.capture.Stop()
}