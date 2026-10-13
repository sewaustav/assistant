package dispatcher

import "strings"

type CommandType string

const (
	CmdUnknown CommandType = "unknown"
	CmdWeather  CommandType = "weather"
	CmdTime     CommandType = "time"
	CmdExit     CommandType = "exit"
)

type Command struct {
	Type    CommandType
	RawText string
	Args    map[string]string 
}

func ParseCommand(text string) Command {
	lower := strings.ToLower(strings.TrimSpace(text))

	cmd := Command{
		Type:    CmdUnknown,
		RawText: text,
		Args:    make(map[string]string),
	}

	switch {
	case containsAny(lower, "погода", "какая погода"):
		cmd.Type = CmdWeather
	case containsAny(lower, "время", "который час", "сколько времени"):
		cmd.Type = CmdTime
	case containsAny(lower, "выход", "стоп", "пока", "завершить"):
		cmd.Type = CmdExit
	}

	return cmd
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}