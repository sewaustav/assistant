package platform

import "strings"

// combo — разобранная горячая клавиша.
type combo struct {
	mods []string
	key  string
}

var modifierAliases = map[string]string{
	"ctrl": "ctrl", "control": "ctrl", "ctl": "ctrl",
	"shift": "shift",
	"alt":   "alt", "option": "alt", "opt": "alt",
	"super": "super", "win": "super", "windows": "super", "meta": "super",
	"cmd": "super", "command": "super", "logo": "super",
}

var keyAliases = map[string]string{
	"return": "enter", "kp_enter": "enter", "enter": "enter",
	"esc": "escape", "escape": "escape",
	"del": "delete", "delete": "delete",
	"ins": "insert", "insert": "insert",
	"pgup": "pageup", "page_up": "pageup", "prior": "pageup", "pageup": "pageup",
	"pgdn": "pagedown", "page_down": "pagedown", "pagedown": "pagedown",
	"next":  "medianext",
	"print": "printscreen", "printscreen": "printscreen", "prtsc": "printscreen",
	"space": "space", "spacebar": "space",
	"backspace": "backspace", "bksp": "backspace",
	"volup": "volumeup", "volumeup": "volumeup", "volume_up": "volumeup", "xf86audioraisevolume": "volumeup",
	"voldown": "volumedown", "volumedown": "volumedown", "volume_down": "volumedown", "xf86audiolowervolume": "volumedown",
	"mute": "mute", "volumemute": "mute", "xf86audiomute": "mute",
	"play": "playpause", "playpause": "playpause", "mediaplaypause": "playpause", "xf86audioplay": "playpause",
	"nexttrack": "medianext", "medianext": "medianext", "xf86audionext": "medianext",
	"prevtrack": "mediaprev", "mediaprev": "mediaprev", "xf86audioprev": "mediaprev",
	"mediastop": "mediastop", "xf86audiostop": "mediastop",
}

// parseCombo разбирает "ctrl+shift+t" в модификаторы и клавишу.
func parseCombo(keys string) combo {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(keys)), "+")
	if len(parts) == 0 {
		return combo{}
	}

	var mods []string
	key := ""
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if mod, ok := modifierAliases[part]; ok {
			mods = append(mods, mod)
			continue
		}
		key = normalizeKey(part)
	}
	return combo{mods: mods, key: key}
}

func normalizeKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	if alias, ok := keyAliases[key]; ok {
		return alias
	}
	switch key {
	case "up", "down", "left", "right", "home", "end", "tab":
		return key
	}
	if len(key) == 1 {
		return key
	}
	if len(key) >= 2 && key[0] == 'f' {
		allDigits := true
		for _, r := range key[1:] {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return key
		}
	}
	return key
}
