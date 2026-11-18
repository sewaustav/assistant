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
	Mode    Mode `json:"mode"`
	Verbose bool `json:"verbose"`

	TTS       TTSConfig            `json:"tts"`
	ASR       ASRConfig            `json:"asr"`
	LLM       LLMConfig            `json:"llm"`
	WakeWord  WakeWordConfig       `json:"wake_word"`
	Paths     PathsConfig          `json:"paths"`
	SearchURL string               `json:"search_url"`
	Apps      map[string]AppConfig `json:"apps,omitempty"`
}

// PathsConfig позволяет переопределить пути к внешним компонентам.
// Пустые значения означают «найти автоматически».
type PathsConfig struct {
	GigaSTT string `json:"gigastt,omitempty"`
	Python  string `json:"python,omitempty"`
	TTSDir  string `json:"tts_dir,omitempty"`
}

// AppConfig описывает приложение, которое можно открыть голосом.
// Достаточно указать команду для нужной ОС.
type AppConfig struct {
	Aliases []string `json:"aliases,omitempty"`
	Name    string   `json:"name,omitempty"`
	Linux   string   `json:"linux,omitempty"`
	Windows string   `json:"windows,omitempty"`
	Darwin  string   `json:"darwin,omitempty"`
}

type TTSConfig struct {
	Enabled    bool   `json:"enabled"`
	ServerURL  string `json:"server_url"`
	Speaker    string `json:"speaker"`
	SampleRate int    `json:"sample_rate"`
	EchoTailMs int    `json:"echo_tail_ms"`
}

type ASRConfig struct {
	Enabled    bool     `json:"enabled"`
	ServerURL  string   `json:"server_url"`
	SampleRate int      `json:"sample_rate"`
	VAD        bool     `json:"vad"`
	ModelDir   string   `json:"model_dir,omitempty"`
	ExtraArgs  []string `json:"extra_args,omitempty"`
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
		cfg.applyDefaults()
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
	cfg.applyDefaults()

	switch cfg.Mode {
	case ModeVoice, ModeText:
	default:
		cfg.Mode = ModeVoice
	}

	return &cfg, nil
}

// applyDefaults заполняет поля, которых не было в старых версиях конфига.
func (c *Config) applyDefaults() {
	if c.ASR.ServerURL == "" {
		c.ASR.ServerURL = "ws://127.0.0.1:9876/v1/ws"
	}
	if c.ASR.SampleRate <= 0 {
		c.ASR.SampleRate = 48000
	}
	if c.TTS.ServerURL == "" {
		c.TTS.ServerURL = "http://127.0.0.1:9877"
	}
	if c.TTS.SampleRate <= 0 {
		c.TTS.SampleRate = 48000
	}
	if c.TTS.Speaker == "" {
		c.TTS.Speaker = "xenia"
	}
	if c.TTS.EchoTailMs <= 0 {
		c.TTS.EchoTailMs = 300
	}
	if c.LLM.Timeout <= 0 {
		c.LLM.Timeout = 30
	}
	if c.LLM.BaseURL == "" {
		c.LLM.BaseURL = "https://api.openai.com/v1"
	}
	if c.LLM.Model == "" {
		c.LLM.Model = "gpt-4o-mini"
	}
	if c.LLM.APIKeyEnv == "" {
		c.LLM.APIKeyEnv = "ASSISTANT_LLM_API_KEY"
	}
	if c.SearchURL == "" {
		c.SearchURL = "https://yandex.ru/search/?text={query}"
	}
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

	// Добавляем новые встроенные команды, не трогая пользовательские правки.
	merged, changed := mergeCommands(defaultCommandsMap(), cmds)
	if changed {
		if err := SaveCommands(merged); err != nil {
			return nil, fmt.Errorf("не удалось обновить файл команд: %w", err)
		}
	}
	return merged, nil
}

func defaultCommandsMap() CommandsConfig {
	var cmds CommandsConfig
	if err := json.Unmarshal(defaultCommands, &cmds); err != nil {
		return CommandsConfig{}
	}
	return cmds
}

// mergeCommands добавляет отсутствующие встроенные команды.
func mergeCommands(defaults, user CommandsConfig) (CommandsConfig, bool) {
	if user == nil {
		user = CommandsConfig{}
	}
	changed := false
	for name, cmd := range defaults {
		if _, ok := user[name]; ok {
			continue
		}
		user[name] = cmd
		changed = true
	}
	return user, changed
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
