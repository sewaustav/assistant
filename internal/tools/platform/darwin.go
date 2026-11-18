//go:build darwin

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	openBrowser = func(url string) error {
		return startDetached(exec.Command("open", url))
	}
	openPath = func(path string) error {
		return startDetached(exec.Command("open", path))
	}
	sendHotkey = darwinSendHotkey
	typeText = darwinTypeText
	lockScreen = func() error {
		path := "/System/Library/CoreServices/Menu Extras/User.menu/Contents/Resources/CGSession"
		if _, err := os.Stat(path); err == nil {
			return exec.Command(path, "-suspend").Run()
		}
		return exec.Command("pmset", "displaysleepnow").Run()
	}
	screenshot = darwinScreenshot
}

func startDetached(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func darwinSendHotkey(keys string) error {
	c := parseCombo(keys)
	if c.key == "" {
		return fmt.Errorf("пустая комбинация клавиш")
	}

	switch c.key {
	case "volumeup":
		return runOsascript(`set volume output volume ((output volume of (get volume settings)) + 10)`)
	case "volumedown":
		return runOsascript(`set volume output volume ((output volume of (get volume settings)) - 10)`)
	case "mute":
		return runOsascript(`set v to output muted of (get volume settings)
set volume output muted (not v)`)
	case "playpause":
		return runOsascript(`tell application "Music" to playpause`)
	case "medianext":
		return runOsascript(`tell application "Music" to next track`)
	case "mediaprev":
		return runOsascript(`tell application "Music" to previous track`)
	}

	mods := []string{}
	for _, mod := range c.mods {
		switch mod {
		case "ctrl", "super":
			mods = append(mods, "command down")
		case "shift":
			mods = append(mods, "shift down")
		case "alt":
			mods = append(mods, "option down")
		}
	}
	using := ""
	if len(mods) > 0 {
		using = " using {" + strings.Join(mods, ", ") + "}"
	}

	if code, ok := darwinKeyCode(c.key); ok {
		return runOsascript(fmt.Sprintf(`tell application "System Events" to key code %d%s`, code, using))
	}

	if len(c.key) == 1 {
		escaped := strings.ReplaceAll(c.key, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		return runOsascript(fmt.Sprintf(`tell application "System Events" to keystroke "%s"%s`, escaped, using))
	}
	return fmt.Errorf("неизвестная клавиша для macOS: %s", c.key)
}

func runOsascript(script string) error {
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func darwinKeyCode(key string) (int, bool) {
	codes := map[string]int{
		"enter": 36, "escape": 53, "tab": 48, "space": 49,
		"delete": 51, "backspace": 51, "insert": 117,
		"home": 115, "end": 119, "pageup": 116, "pagedown": 121,
		"up": 126, "down": 125, "left": 123, "right": 124,
		"f1": 122, "f2": 120, "f3": 99, "f4": 118, "f5": 96, "f6": 97,
		"f7": 98, "f8": 100, "f9": 101, "f10": 109, "f11": 103, "f12": 111,
	}
	code, ok := codes[key]
	return code, ok
}

func darwinTypeText(text string) error {
	if text == "" {
		return nil
	}
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pbcopy: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return darwinSendHotkey("ctrl+v")
}

func darwinScreenshot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Pictures", "Screenshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "screenshot-"+time.Now().Format("2006-01-02-15-04-05")+".png")
	if out, err := exec.Command("screencapture", "-x", path).CombinedOutput(); err != nil {
		return "", fmt.Errorf("screencapture: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	if !PathExists(path) {
		return "", fmt.Errorf("screencapture не создал файл")
	}
	return path, nil
}

// darwinModifier в основном для отладки.
func darwinModifier(mod string) string {
	if mod == "super" {
		return "command"
	}
	return mod
}
