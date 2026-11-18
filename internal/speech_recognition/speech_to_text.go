package speech_recognition

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// GigaSTTClient — WebSocket-клиент к gigastt.
type GigaSTTClient struct {
	url  string
	conn *websocket.Conn
}

// NewGigaSTTClient создаёт клиент. Соединение устанавливается в TranscribeStream.
func NewGigaSTTClient(serverURL string) *GigaSTTClient {
	if serverURL == "" {
		serverURL = "ws://127.0.0.1:9876/v1/ws"
	}
	return &GigaSTTClient{url: serverURL}
}

// TranscribeStream отправляет PCM16-аудио и возвращает канал с JSON-ответами.
func (c *GigaSTTClient) TranscribeStream(ctx context.Context, audioChan <-chan []byte, sampleRate int) (<-chan string, error) {
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	c.conn = conn

	// Сообщаем серверу частоту микрофона. Без этого он считает 48 кГц,
	// а на Windows устройство может отдать 44.1 или 16 кГц.
	configure := fmt.Sprintf(`{"type":"configure","sample_rate":%d}`, sampleRate)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(configure)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("не удалось отправить configure: %w", err)
	}

	textChan := make(chan string, 10)

	go func() {
		defer close(textChan)
		defer conn.Close()

		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				_, message, err := conn.ReadMessage()
				if err != nil {
					if ctx.Err() == nil {
						log.Printf("GigaSTT: ошибка чтения: %v", err)
					}
					return
				}

				select {
				case textChan <- string(message):
				case <-ctx.Done():
					return
				}
			}
		}()

		for {
			select {
			case <-ctx.Done():
				_ = conn.Close()
				<-done
				return
			case <-done:
				return
			case chunk, ok := <-audioChan:
				if !ok {
					return
				}
				if err := conn.WriteMessage(websocket.BinaryMessage, chunk); err != nil {
					if ctx.Err() == nil {
						log.Printf("GigaSTT: ошибка отправки: %v", err)
					}
					_ = conn.Close()
					<-done
					return
				}
			}
		}
	}()

	return textChan, nil
}

func (c *GigaSTTClient) dial(ctx context.Context) (*websocket.Conn, error) {
	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.url, nil)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
	return nil, fmt.Errorf("не удалось подключиться к gigastt (%s): %w", c.url, lastErr)
}

// Close закрывает соединение.
func (c *GigaSTTClient) Close() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
}
