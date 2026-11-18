package tts

import (
	"bytes"
	"context"
	"encoding/binary"
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
	baseURL    string
	sampleRate int
	http       *http.Client

	initOnce sync.Once
	initErr  error
	otoCtx   *oto.Context

	mu sync.Mutex
}

// Request — тело запроса к серверу.
type Request struct {
	Text       string `json:"text"`
	Speaker    string `json:"speaker"`
	SampleRate int    `json:"sample_rate"`
}

// New создаёт клиент. Аудио-контекст инициализируется лениво, при первом
// воспроизведении: в текстовом режиме звук вообще не нужен, и приложение
// не должно падать на машине без аудиовыхода.
func New(baseURL string, sampleRate int) *Client {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:9877"
	}
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	return &Client{
		baseURL:    baseURL,
		sampleRate: sampleRate,
		http:       &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) ensureAudio() error {
	c.initOnce.Do(func() {
		op := &oto.NewContextOptions{
			SampleRate:   c.sampleRate,
			ChannelCount: 1,
			Format:       oto.FormatSignedInt16LE,
		}
		otoCtx, ready, err := oto.NewContext(op)
		if err != nil {
			c.initErr = fmt.Errorf("oto init: %w", err)
			return
		}
		select {
		case <-ready:
			c.otoCtx = otoCtx
		case <-time.After(10 * time.Second):
			c.initErr = fmt.Errorf("аудио-контекст не готов за 10 секунд")
		}
	})
	return c.initErr
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

	reqBody := Request{Text: text, Speaker: speaker, SampleRate: c.sampleRate}
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, fmt.Errorf("tts server вернул %d: %s", resp.StatusCode, string(body))
	}

	wav, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read response: %w", err)
	}

	pcm, rate, channels, bits, err := parseWAV(wav)
	if err != nil {
		return 0, err
	}
	if rate != c.sampleRate || channels != 1 || bits != 16 {
		return 0, fmt.Errorf("TTS вернул неподдерживаемый формат: %d Гц, %d кан., %d бит", rate, channels, bits)
	}

	bytesPerSecond := rate * channels * bits / 8
	duration := time.Duration(0)
	if bytesPerSecond > 0 {
		duration = time.Duration(float64(len(pcm)) / float64(bytesPerSecond) * float64(time.Second))
	}
	log.Printf("[tts] Получено %d байт PCM (~%.2f сек), воспроизвожу...", len(pcm), duration.Seconds())

	if err := c.playWAV(ctx, pcm); err != nil {
		return 0, err
	}
	return duration, nil
}

// parseWAV достаёт PCM-данные и параметры из WAV-файла.
func parseWAV(wav []byte) (pcm []byte, sampleRate, channels, bits int, err error) {
	if len(wav) < 12 || string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		return nil, 0, 0, 0, fmt.Errorf("TTS вернул не WAV")
	}
	pos := 12
	for pos+8 <= len(wav) {
		id := string(wav[pos : pos+4])
		size := int(binary.LittleEndian.Uint32(wav[pos+4 : pos+8]))
		pos += 8
		if size < 0 || pos+size > len(wav) {
			return nil, 0, 0, 0, fmt.Errorf("повреждённый WAV: чанк %s", id)
		}
		switch id {
		case "fmt ":
			if size < 16 {
				return nil, 0, 0, 0, fmt.Errorf("повреждённый WAV: fmt")
			}
			channels = int(binary.LittleEndian.Uint16(wav[pos+2 : pos+4]))
			sampleRate = int(binary.LittleEndian.Uint32(wav[pos+4 : pos+8]))
			bits = int(binary.LittleEndian.Uint16(wav[pos+14 : pos+16]))
		case "data":
			pcm = wav[pos : pos+size]
		}
		pos += size
		if size%2 == 1 {
			pos++ // выравнивание чанков
		}
	}
	if pcm == nil {
		return nil, 0, 0, 0, fmt.Errorf("в WAV нет data-чанка")
	}
	if sampleRate <= 0 || channels <= 0 || bits != 16 {
		return nil, 0, 0, 0, fmt.Errorf("неподдерживаемый WAV: %d Гц, %d кан., %d бит", sampleRate, channels, bits)
	}
	return pcm, sampleRate, channels, bits, nil
}

// playWAV проигрывает PCM16 через уже созданный oto-контекст.
func (c *Client) playWAV(ctx context.Context, pcm []byte) error {
	if len(pcm) == 0 {
		return fmt.Errorf("пустой PCM")
	}
	if err := c.ensureAudio(); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	player := c.otoCtx.NewPlayer(bytes.NewReader(pcm))
	player.Play()

	for player.IsPlaying() {
		select {
		case <-ctx.Done():
			player.Close()
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}

	player.Close()
	return nil
}

// Close освобождает аудио-контекст.
func (c *Client) Close() {}
