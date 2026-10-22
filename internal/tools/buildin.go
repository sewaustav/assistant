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
func (t *TimeTool) Keywords() []string  { return nil }
func (t *TimeTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	return time.Now().Format("15:04:05"), nil
}

type BrowserTool struct{}

func (t *BrowserTool) Name() string        { return "browser" }
func (t *BrowserTool) Description() string { return "Open the default web browser" }
func (t *BrowserTool) Keywords() []string  { return nil }
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
func (t *HotkeyTool) Keywords() []string  { return nil }
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
	for _, cfg := range cmds {
		if !isPlatformSupported(cfg.Platforms) {
			continue
		}
		switch cfg.Action.Name {
		case "time":
			result = append(result, &TimeTool{})
		case "browser":
			result = append(result, &BrowserTool{})
		case "hotkey":
			result = append(result, &HotkeyTool{})
		}
	}
	return result
}