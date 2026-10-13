package speech_recognition

import (
	"context"
	"fmt"
	"log"

	"github.com/gorilla/websocket"
)

type GigaSTTClient struct {
	conn *websocket.Conn
}

func NewGigaSTTClient(serverURL string) (*GigaSTTClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(serverURL, nil)
	if err != nil {
		return nil, fmt.Errorf("не удалось подключиться к gigastt: %w", err)
	}

	return &GigaSTTClient{conn: conn}, nil
}

func (c *GigaSTTClient) TranscribeStream(ctx context.Context, audioChan <-chan []byte) (<-chan string, error) {
	textChan := make(chan string, 10)

	go func() {
		defer close(textChan)
		defer c.conn.Close()

		go func() {
			for {
				_, message, err := c.conn.ReadMessage()
				if err != nil {
					log.Printf("GigaSTT: ошибка чтения: %v", err)
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
				return
			case chunk, ok := <-audioChan:
				if !ok {
					return
				}
				if err := c.conn.WriteMessage(websocket.BinaryMessage, chunk); err != nil {
					log.Printf("GigaSTT: ошибка отправки: %v", err)
					return
				}
			}
		}
	}()

	return textChan, nil
}