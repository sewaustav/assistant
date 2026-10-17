package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed default.json
var defaultConfig []byte

// Mode — режим работы ассистента.
type Mode string

const (
	ModeVoice Mode = "voice" // ввод с микрофона
	ModeText  Mode = "text"  // ввод из CLI
)

// Config — корневая структура конфига.
type Config struct {
	Mode    Mode   `json:"mode"`
	Verbose bool   `json:"verbose"`

	TTS      TTSConfig      `json:"tts"`
	ASR      ASRConfig      `json:"asr"`
	LLM      LLMConfig      `json:"llm"`
	WakeWord WakeWordConfig `json:"wake_word"`
}

// TTSConfig — настройки синтеза речи.
type TTSConfig struct {
	Enabled    bool   `json:"enabled"`
	ServerURL  string `json:"server_url"`
	Speaker    string `json:"speaker"`
	SampleRate int    `json:"sample_rate"`
	EchoTailMs int    `json:"echo_tail_ms"`
}

// ASRConfig — настройки распознавания речи.
type ASRConfig struct {
	Enabled   bool   `json:"enabled"`
	ServerURL string `json:"server_url"`
}

// LLMConfig — настройки языковой модели (на будущее).
type LLMConfig struct {
	Enabled   bool   `json:"enabled"`
	Provider  string `json:"provider"`
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	Timeout   int    `json:"timeout"`
	APIKeyEnv string `json:"api_key_env"`
}

// WakeWordConfig — настройки wake word (на будущее).
type WakeWordConfig struct {
	Enabled   bool    `json:"enabled"`
	ModelPath string  `json:"model_path"`
	Threshold float64 `json:"threshold"`
	Cooldown  int     `json:"cooldown_sec"`
}

// Dir возвращает кроссплатформенную папку для конфига.
// Windows: %AppData%\assistant
// Linux:   ~/.config/assistant
// macOS:   ~/Library/Application Support/assistant
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("не удалось получить UserConfigDir: %w", err)
	}
	dir := filepath.Join(base, "assistant")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("не удалось создать папку конфига: %w", err)
	}
	return dir, nil
}

// Path возвращает полный путь к файлу конфига.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load читает конфиг из файла.
// Если файла нет — создаёт его из встроенного default.json и возвращает дефолт.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Файла нет — создаём из встроенного дефолта.
		cfg, err := parseDefault()
		if err != nil {
			return nil, fmt.Errorf("не удалось распарсить встроенный дефолт: %w", err)
		}
		if err := cfg.Save(); err != nil {
			return nil, fmt.Errorf("не удалось создать дефолтный конфиг: %w", err)
		}
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать конфиг: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("не удалось распарсить конфиг: %w", err)
	}

	// Валидация режима: если что-то не то — ставим voice.
	switch cfg.Mode {
	case ModeVoice, ModeText:
		// ок
	default:
		cfg.Mode = ModeVoice
	}

	return &cfg, nil
}

// parseDefault парсит встроенный default.json.
func parseDefault() (*Config, error) {
	var cfg Config
	if err := json.Unmarshal(defaultConfig, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save записывает конфиг в файл.
func (c *Config) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("не удалось сериализовать конфиг: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("не удалось записать конфиг: %w", err)
	}
	return nil
}