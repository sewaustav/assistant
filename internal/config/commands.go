package config

type CommandsConfig map[string]CommandConfig

type CommandConfig struct {
	Description  string                       `json:"description"`
	Keywords     []string                     `json:"keywords"`
	LLMEnabled   bool                         `json:"llm_enabled"`
	LocalEnabled bool                         `json:"local_enabled"`
	Platforms    []string                     `json:"platforms"`
	Parameters   map[string]ParameterConfig   `json:"parameters,omitempty"`
	Action       ActionConfig                 `json:"action"`
}

type ParameterConfig struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type ActionConfig struct {
	Type    string            `json:"type"`
	Name    string            `json:"name,omitempty"`
	Method  string            `json:"method,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
}