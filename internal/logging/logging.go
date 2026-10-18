package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// Init перенаправляет стандартный log в файл.
// Возвращает путь к файлу лога.
func Init() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("не удалось получить UserCacheDir: %w", err)
	}

	dir := filepath.Join(base, "assistant")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("не удалось создать папку логов: %w", err)
	}

	path := filepath.Join(dir, "assistant.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return "", fmt.Errorf("не удалось открыть файл лога: %w", err)
	}

	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	return path, nil
}