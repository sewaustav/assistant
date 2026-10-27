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

type SelectAllTool struct{}

func (t *SelectAllTool) Name() string        { return "select_all" }
func (t *SelectAllTool) Description() string { return "Select all text" }
func (t *SelectAllTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	if err := platform.SendHotkey("ctrl+a"); err != nil {
		return "", fmt.Errorf("не удалось выделить всё: %w", err)
	}
	return "Выделено всё", nil
}

type CopyTool struct{}

func (t *CopyTool) Name() string        { return "copy" }
func (t *CopyTool) Description() string { return "Copy selected text" }
func (t *CopyTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	if err := platform.SendHotkey("ctrl+c"); err != nil {
		return "", fmt.Errorf("не удалось скопировать: %w", err)
	}
	return "Скопировано", nil
}

type CutTool struct{}

func (t *CutTool) Name() string        { return "cut" }
func (t *CutTool) Description() string { return "Cut selected text" }
func (t *CutTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	if err := platform.SendHotkey("ctrl+x"); err != nil {
		return "", fmt.Errorf("не удалось вырезать: %w", err)
	}
	return "Вырезано", nil
}

type PasteTool struct{}

func (t *PasteTool) Name() string        { return "paste" }
func (t *PasteTool) Description() string { return "Paste from clipboard" }
func (t *PasteTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	if err := platform.SendHotkey("ctrl+v"); err != nil {
		return "", fmt.Errorf("не удалось вставить: %w", err)
	}
	return "Вставлено", nil
}

type DeleteTool struct{}

func (t *DeleteTool) Name() string        { return "delete" }
func (t *DeleteTool) Description() string { return "Delete selected text" }
func (t *DeleteTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	if err := platform.SendHotkey("Delete"); err != nil {
		return "", fmt.Errorf("не удалось удалить: %w", err)
	}
	return "Удалено", nil
}

type UndoTool struct{}

func (t *UndoTool) Name() string        { return "undo" }
func (t *UndoTool) Description() string { return "Undo last action" }
func (t *UndoTool) Execute(ctx context.Context, args map[string]string) (string, error) {
	if err := platform.SendHotkey("ctrl+z"); err != nil {
		return "", fmt.Errorf("не удалось отменить: %w", err)
	}
	return "Отменено", nil
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
		case "select_all":
			base = &SelectAllTool{}
		case "copy":
			base = &CopyTool{}
		case "cut":
			base = &CutTool{}
		case "paste":
			base = &PasteTool{}
		case "delete":
			base = &DeleteTool{}
		case "undo":
			base = &UndoTool{}
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