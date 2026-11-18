package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/sewaustav/assistant/internal/config"
)

const systemPrompt = `Ты — локальный голосовой ассистент на компьютере пользователя.
Отвечай по-русски, коротко и естественно: ответ будет озвучен.
Не используй markdown, списки и эмодзи.
Если пользователь просит выполнить действие с компьютером (открыть программу,
скопировать, вставить, изменить громкость, сделать скриншот и т.п.) — вызывай
подходящий инструмент. Не придумывай результат действия.
Если команда непонятна — задай один короткий уточняющий вопрос.`

// Handler общается с OpenAI-совместимым API.
type Handler struct {
	cfg    *config.Config
	tools  []Tool
	byName map[string]Tool
	client *http.Client
}

// New создаёт обработчик LLM.
func New(cfg *config.Config, tools []Tool) *Handler {
	timeout := time.Duration(cfg.LLM.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	h := &Handler{
		cfg:    cfg,
		tools:  tools,
		byName: make(map[string]Tool, len(tools)),
		client: &http.Client{Timeout: timeout},
	}
	for _, t := range tools {
		h.byName[t.Name()] = t
	}
	return h
}

// Handle отправляет текст в LLM, выполняет tool calls и возвращает ответ.
func (h *Handler) Handle(ctx context.Context, text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("пустой текст")
	}
	if !h.cfg.LLM.Enabled {
		return "", fmt.Errorf("LLM выключена в конфиге")
	}

	apiKey := ""
	if h.cfg.LLM.APIKeyEnv != "" {
		apiKey = os.Getenv(h.cfg.LLM.APIKeyEnv)
	}
	if apiKey == "" && !isLocalBaseURL(h.cfg.LLM.BaseURL) {
		return "", fmt.Errorf("LLM API key не задан: переменная %s", h.cfg.LLM.APIKeyEnv)
	}

	log.Printf("[llm] Handle: %q (model=%s, tools=%d)", text, h.cfg.LLM.Model, len(h.tools))

	messages := []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: text},
	}

	for round := 0; round < 5; round++ {
		reply, err := h.chat(ctx, apiKey, messages)
		if err != nil {
			return "", err
		}

		if len(reply.ToolCalls) == 0 {
			answer := strings.TrimSpace(reply.Content)
			if answer == "" {
				answer = "Готово."
			}
			return answer, nil
		}

		// Добавляем ответ ассистента с запрошенными инструментами.
		messages = append(messages, chatMessage{
			Role:      "assistant",
			ToolCalls: reply.ToolCalls,
		})

		for _, call := range reply.ToolCalls {
			result := h.executeTool(ctx, call)
			messages = append(messages, chatMessage{
				Role:       "tool",
				ToolCallID: call.ID,
				Name:       call.Function.Name,
				Content:    result,
			})
		}
	}

	return "", fmt.Errorf("LLM слишком долго вызывает инструменты")
}

func (h *Handler) executeTool(ctx context.Context, call toolCall) string {
	tool, ok := h.byName[call.Function.Name]
	if !ok {
		return fmt.Sprintf("Инструмент %q не найден", call.Function.Name)
	}
	args := decodeArgs(call.Function.Arguments)
	log.Printf("[llm] tool call: %s(%v)", call.Function.Name, args)
	result, err := tool.Execute(ctx, args)
	if err != nil {
		log.Printf("[llm] tool %s error: %v", call.Function.Name, err)
		return "Ошибка: " + err.Error()
	}
	return result
}

func (h *Handler) chat(ctx context.Context, apiKey string, messages []chatMessage) (chatMessage, error) {
	reqBody := chatRequest{
		Model:       h.cfg.LLM.Model,
		Messages:    messages,
		Tools:       h.toolSchemas(),
		ToolChoice:  "auto",
		Temperature: 0.2,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return chatMessage{}, fmt.Errorf("llm marshal: %w", err)
	}

	endpoint := strings.TrimRight(h.cfg.LLM.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return chatMessage{}, fmt.Errorf("llm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "assistant/1.0")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return chatMessage{}, fmt.Errorf("llm http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return chatMessage{}, fmt.Errorf("llm read: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return chatMessage{}, fmt.Errorf("llm вернул %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	var out chatResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return chatMessage{}, fmt.Errorf("llm parse: %w (%s)", err, truncate(string(body), 300))
	}
	if out.Error != nil {
		return chatMessage{}, fmt.Errorf("llm error: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return chatMessage{}, fmt.Errorf("llm не вернул ответ")
	}
	return out.Choices[0].Message, nil
}

func (h *Handler) toolSchemas() []toolSchema {
	if len(h.tools) == 0 {
		return nil
	}
	out := make([]toolSchema, 0, len(h.tools))
	for _, t := range h.tools {
		fn := functionSchema{
			Name:        t.Name(),
			Description: t.Description(),
		}
		if p, ok := t.(interface{ Parameters() map[string]any }); ok {
			fn.Parameters = p.Parameters()
		}
		out = append(out, toolSchema{Type: "function", Function: fn})
	}
	return out
}

func decodeArgs(raw string) map[string]string {
	args := make(map[string]string)
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return args
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return args
	}
	for key, value := range parsed {
		switch v := value.(type) {
		case string:
			args[key] = v
		case float64:
			if v == float64(int64(v)) {
				args[key] = fmt.Sprintf("%d", int64(v))
			} else {
				args[key] = fmt.Sprintf("%v", v)
			}
		case bool:
			args[key] = fmt.Sprintf("%t", v)
		case nil:
			continue
		default:
			data, _ := json.Marshal(v)
			args[key] = string(data)
		}
	}
	return args
}

func isLocalBaseURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0"
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Tools       []toolSchema  `json:"tools,omitempty"`
	ToolChoice  string        `json:"tool_choice,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type toolSchema struct {
	Type     string         `json:"type"`
	Function functionSchema `json:"function"`
}

type functionSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}
