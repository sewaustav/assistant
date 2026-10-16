package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

// Client — клиент к локальному Silero TTS серверу.
type Client struct {
	baseURL string
	http    *http.Client

	otoCtx *oto.Context
	mu     sync.Mutex
}

// Request — тело запроса к серверу.
type Request struct {
	Text       string `json:"text"`
	Speaker    string `json:"speaker"`
	SampleRate int    `json:"sample_rate"`
}

// New создаёт клиент и инициализирует аудио-контекст один раз.
func New(baseURL string) (*Client, error) {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:9877"
	}

	op := &oto.NewContextOptions{
		SampleRate:   48000,
		ChannelCount: 1,
		Format:       oto.FormatSignedInt16LE,
	}

	otoCtx, ready, err := oto.NewContext(op)
	if err != nil {
		return nil, fmt.Errorf("oto init: %w", err)
	}

	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("oto context не готов за 5 секунд")
	}

	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
		otoCtx:  otoCtx,
	}, nil
}

// Speak синтезирует текст и воспроизводит его.
// Возвращает длительность воспроизведённого аудио.
func (c *Client) Speak(ctx context.Context, text, speaker string) (time.Duration, error) {
	if text == "" {
		return 0, fmt.Errorf("пустой текст")
	}
	if speaker == "" {
		speaker = "xenia"
	}

	reqBody := Request{Text: text, Speaker: speaker, SampleRate: 48000}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return 0, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/tts", bytes.NewReader(jsonBody))
	if err != nil {
		return 0, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("tts server вернул %d: %s", resp.StatusCode, string(body))
	}

	wav, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read response: %w", err)
	}

	duration := wavDuration(wav)
	log.Printf("[tts] Получено %d байт WAV (~%.2f сек), воспроизвожу...", len(wav), duration.Seconds())

	if err := c.playWAV(ctx, wav); err != nil {
		return 0, err
	}
	return duration, nil
}

// wavDuration считает длительность WAV по заголовку.
// Предполагается PCM16, 48000 Hz, моно.
func wavDuration(wav []byte) time.Duration {
	if len(wav) < 44 {
		return 0
	}
	// Data size — байты 40..43 (LE uint32)
	dataSize := uint32(wav[40]) | uint32(wav[41])<<8 | uint32(wav[42])<<16 | uint32(wav[43])<<24
	// 48000 Hz * 1 канал * 2 байта = 96000 байт/сек
	seconds := float64(dataSize) / 96000.0
	return time.Duration(seconds * float64(time.Second))
}

// playWAV проигрывает WAV через уже созданный oto-контекст.
func (c *Client) playWAV(ctx context.Context, wavData []byte) error {
	if len(wavData) < 44 {
		return fmt.Errorf("слишком короткий WAV")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	pcm := wavData[44:]
	player := c.otoCtx.NewPlayer(bytes.NewReader(pcm))
	player.Play()

	for player.IsPlaying() {
		select {
		case <-ctx.Done():
			player.Close()
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}

	player.Close()
	return nil
}

// Close освобождает аудио-контекст.
func (c *Client) Close() {
}

