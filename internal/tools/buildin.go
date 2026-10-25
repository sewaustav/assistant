package tools

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/sewaustav/assistant/internal/config"
	"github.com/sewaustav/assistant/internal/tools/platform"
)

type TimeTool struct{}

func (t *TimeTool) Name() string        { return "time" }
func (t *TimeTool) Description() string { return "Get the current time" }
func (t *TimeTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	now := time.Now()
	return fmt.Sprintf("%d часов %d минут", now.Hour(), now.Minute()), nil

}

type BrowserTool struct{}

func (t *BrowserTool) Name() string        { return "browser" }
func (t *BrowserTool) Description() string { return "Open the default web browser" }
func (t *BrowserTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	url := args["url"]
	if url == "" {
		url = "https://www.google.com"
	}
	if err := platform.OpenBrowser(url); err != nil {
		return "", fmt.Errorf("не удалось открыть браузер: %w", err)
	}
	return "Браузер открыт", nil
}

type HotkeyTool struct{}

func (t *HotkeyTool) Name() string        { return "hotkey" }
func (t *HotkeyTool) Description() string { return "Press a keyboard shortcut" }
func (t *HotkeyTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	keys := args["keys"]
	if keys == "" {
		return "", fmt.Errorf("не указаны клавиши")
	}
	if err := platform.SendHotkey(keys); err != nil {
		return "", fmt.Errorf("не удалось нажать хоткей: %w", err)
	}
	return fmt.Sprintf("Нажато: %s", keys), nil
}

type wrappedTool struct {
	inner       any
	name        string
	description string
	keywords    []string
}

func (w *wrappedTool) Name() string        { return w.name }
func (w *wrappedTool) Description() string { return w.description }
func (w *wrappedTool) Keywords() []string  { return w.keywords }
func (w *wrappedTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	type execer interface {
		Execute(context.Context, map[string]string) (string, error)
	}
	if e, ok := w.inner.(execer); ok {
		return e.Execute(ctx, args)
	}
	return "", fmt.Errorf("tool %q не умеет Execute", w.name)
}

func isPlatformSupported(platforms []string) bool {
	for _, p := range platforms {
		if p == runtime.GOOS {
			return true
		}
	}
	return false
}

func BuildFromConfig(cmds config.CommandsConfig) []any {
	var result []any
	for name, cfg := range cmds {
		if !isPlatformSupported(cfg.Platforms) {
			continue
		}
		var base any
		switch cfg.Action.Name {
		case "time":
			base = &TimeTool{}
		case "browser":
			base = &BrowserTool{}
		case "hotkey":
			base = &HotkeyTool{}
		default:
			continue
		}
		result = append(result, &wrappedTool{
			inner:       base,
			name:        name,
			description: cfg.Description,
			keywords:    cfg.Keywords,
		})
	}
	return result
}