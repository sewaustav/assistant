package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/sewaustav/assistant/internal/config"
)

type Handler struct {
	cfg   *config.Config
	tools []Tool
}

func New(cfg *config.Config, tools []Tool) *Handler {
	return &Handler{cfg: cfg, tools: tools}
}

func (h *Handler) Handle(ctx context.Context, text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("пустой текст")
	}
	if !h.cfg.LLM.Enabled {
		return "", fmt.Errorf("LLM выключена в конфиге")
	}

	log.Printf("[llm] Handle: %q (provider=%s, model=%s, tools=%d)",
		text, h.cfg.LLM.Provider, h.cfg.LLM.Model, len(h.tools))

	schema, err := h.buildToolSchema()
	if err != nil {
		return "", fmt.Errorf("не удалось собрать tool schema: %w", err)
	}
	log.Printf("[llm] tool schema: %s", schema)

	return text, nil
}

func (h *Handler) buildToolSchema() (string, error) {
	type param struct {
		Type        string `json:"type"`
		Description string `json:"description"`
	}
	type function struct {
		Name        string           `json:"name"`
		Description string           `json:"description"`
		Parameters  map[string]param `json:"parameters,omitempty"`
	}
	type tool struct {
		Type     string   `json:"type"`
		Function function `json:"function"`
	}

	var out []tool
	for _, t := range h.tools {
		out = append(out, tool{
			Type: "function",
			Function: function{
				Name:        t.Name(),
				Description: t.Description(),
			},
		})
	}

	data, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(data), nil
}