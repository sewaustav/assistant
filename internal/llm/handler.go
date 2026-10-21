package llm

import (
	"context"
	"fmt"
	"log"

	"github.com/sewaustav/assistant/internal/config"
)

// Handler — обработчик текста через LLM.
// Пока заглушка: возвращает текст обратно.
// Позже здесь будет вызов OpenAI/Anthropic/локальной модели.
type Handler struct {
	cfg *config.Config
}

// New создаёт Handler.
func New(cfg *config.Config) *Handler {
	return &Handler{cfg: cfg}
}

// Handle принимает текст пользователя и возвращает ответ.
// Пока — эхо. Позже — реальный вызов LLM.
func (h *Handler) Handle(ctx context.Context, text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("пустой текст")
	}

	// Проверяем, что LLM реально включена и настроена.
	if !h.cfg.LLM.Enabled {
		return "", fmt.Errorf("LLM выключена в конфиге")
	}

	log.Printf("[llm] Handle: %q (provider=%s, model=%s)",
		text, h.cfg.LLM.Provider, h.cfg.LLM.Model)

	// TODO: реальный вызов LLM.
	// Пока — заглушка: возвращаем текст обратно.
	return text, nil
}