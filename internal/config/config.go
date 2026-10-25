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

//go:embed commands.json
var defaultCommands []byte

type Mode string

const (
	ModeVoice Mode = "voice"
	ModeText  Mode = "text"
)

type Config struct {
	Mode    Mode   `json:"mode"`
	Verbose bool   `json:"verbose"`

	TTS      TTSConfig      `json:"tts"`
	ASR      ASRConfig      `json:"asr"`
	LLM      LLMConfig      `json:"llm"`
	WakeWord WakeWordConfig `json:"wake_word"`
}

type TTSConfig struct {
	Enabled    bool   `json:"enabled"`
	ServerURL  string `json:"server_url"`
	Speaker    string `json:"speaker"`
	SampleRate int    `json:"sample_rate"`
	EchoTailMs int    `json:"echo_tail_ms"`
}

type ASRConfig struct {
	Enabled   bool   `json:"enabled"`
	ServerURL string `json:"server_url"`
}

type LLMConfig struct {
	Enabled   bool   `json:"enabled"`
	Provider  string `json:"provider"`
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	Timeout   int    `json:"timeout"`
	APIKeyEnv string `json:"api_key_env"`
}

type WakeWordConfig struct {
	Enabled   bool    `json:"enabled"`
	ModelPath string  `json:"model_path"`
	Threshold float64 `json:"threshold"`
	Cooldown  int     `json:"cooldown_sec"`
}

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

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func CommandsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "commands.json"), nil
}

func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := &Config{}
		if err := json.Unmarshal(defaultConfig, cfg); err != nil {
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

	switch cfg.Mode {
	case ModeVoice, ModeText:
	default:
		cfg.Mode = ModeVoice
	}

	return &cfg, nil
}

func LoadCommands() (CommandsConfig, error) {
	path, err := CommandsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		var cmds CommandsConfig
		if err := json.Unmarshal(defaultCommands, &cmds); err != nil {
			return nil, fmt.Errorf("не удалось распарсить встроенные команды: %w", err)
		}
		if err := SaveCommands(cmds); err != nil {
			return nil, fmt.Errorf("не удалось создать файл команд: %w", err)
		}
		return cmds, nil
	}
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать команды: %w", err)
	}

	var cmds CommandsConfig
	if err := json.Unmarshal(data, &cmds); err != nil {
		return nil, fmt.Errorf("не удалось распарсить команды: %w", err)
	}
	return cmds, nil
}

func SaveCommands(cmds CommandsConfig) error {
	path, err := CommandsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cmds, "", "  ")
	if err != nil {
		return fmt.Errorf("не удалось сериализовать команды: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("не удалось записать команды: %w", err)
	}
	return nil
}

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