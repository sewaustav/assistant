//go:build linux

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
		return startDetached(exec.Command("xdg-open", url))
	}
	openPath = func(path string) error {
		return startDetached(exec.Command("xdg-open", path))
	}
	sendHotkey = linuxSendHotkey
	typeText = linuxTypeText
	lockScreen = linuxLockScreen
	screenshot = linuxScreenshot
}

func startDetached(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func linuxSendHotkey(keys string) error {
	c := parseCombo(keys)
	if c.key == "" {
		return fmt.Errorf("пустая комбинация клавиш")
	}

	if wt, err := exec.LookPath("wtype"); err == nil {
		return runWType(wt, c)
	}
	if xd, err := exec.LookPath("xdotool"); err == nil {
		return runXDoTool(xd, c)
	}
	return fmt.Errorf("нужен wtype (Wayland) или xdotool (X11): установи один из них")
}

func runWType(bin string, c combo) error {
	args := []string{}
	for _, mod := range c.mods {
		args = append(args, "-M", wtypeModifier(mod))
	}
	args = append(args, "-k", keysym(c.key))
	for i := len(c.mods) - 1; i >= 0; i-- {
		args = append(args, "-m", wtypeModifier(c.mods[i]))
	}
	cmd := exec.Command(bin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("wtype: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runXDoTool(bin string, c combo) error {
	parts := append([]string{}, c.mods...)
	parts = append(parts, keysym(c.key))
	keys := strings.Join(parts, "+")
	cmd := exec.Command(bin, "key", "--clearmodifiers", keys)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("xdotool: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func wtypeModifier(mod string) string {
	if mod == "super" {
		return "logo"
	}
	return mod
}

func keysym(key string) string {
	switch key {
	case "delete":
		return "Delete"
	case "insert":
		return "Insert"
	case "home":
		return "Home"
	case "end":
		return "End"
	case "pageup":
		return "Page_Up"
	case "pagedown":
		return "Page_Down"
	case "up":
		return "Up"
	case "down":
		return "Down"
	case "left":
		return "Left"
	case "right":
		return "Right"
	case "enter":
		return "Return"
	case "escape":
		return "Escape"
	case "tab":
		return "Tab"
	case "space":
		return "space"
	case "backspace":
		return "BackSpace"
	case "printscreen":
		return "Print"
	case "volumeup":
		return "XF86AudioRaiseVolume"
	case "volumedown":
		return "XF86AudioLowerVolume"
	case "mute":
		return "XF86AudioMute"
	case "playpause":
		return "XF86AudioPlay"
	case "medianext":
		return "XF86AudioNext"
	case "mediaprev":
		return "XF86AudioPrev"
	case "mediastop":
		return "XF86AudioStop"
	}
	if len(key) > 1 && key[0] == 'f' {
		return strings.ToUpper(key)
	}
	return key
}

func linuxTypeText(text string) error {
	if text == "" {
		return nil
	}
	if wt, err := exec.LookPath("wtype"); err == nil {
		cmd := exec.Command(wt, "--", text)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("wtype: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	if xd, err := exec.LookPath("xdotool"); err == nil {
		cmd := exec.Command(xd, "type", "--clearmodifiers", "--", text)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("xdotool: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	if err := setClipboard(text); err != nil {
		return err
	}
	return linuxSendHotkey("ctrl+v")
}

func setClipboard(text string) error {
	var cmd *exec.Cmd
	switch {
	case hasBin("wl-copy"):
		cmd = exec.Command("wl-copy")
	case hasBin("xclip"):
		cmd = exec.Command("xclip", "-selection", "clipboard")
	case hasBin("xsel"):
		cmd = exec.Command("xsel", "--clipboard", "--input")
	default:
		return fmt.Errorf("нужен wl-copy, xclip или xsel")
	}
	cmd.Stdin = strings.NewReader(text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("clipboard: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func linuxLockScreen() error {
	candidates := [][]string{
		{"loginctl", "lock-session"},
		{"xdg-screensaver", "lock"},
		{"gnome-screensaver-command", "-l"},
		{"swaylock"},
		{"hyprlock"},
		{"betterlockscreen", "-l"},
	}
	for _, candidate := range candidates {
		if !hasBin(candidate[0]) {
			continue
		}
		cmd := exec.Command(candidate[0], candidate[1:]...)
		if candidate[0] == "swaylock" || candidate[0] == "hyprlock" || candidate[0] == "betterlockscreen" {
			if err := startDetached(cmd); err == nil {
				return nil
			}
			continue
		}
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("не нашёл способ заблокировать экран (loginctl/swaylock/hyprlock)")
}

func linuxScreenshot() (string, error) {
	dir := picturesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "screenshot-"+time.Now().Format("2006-01-02-15-04-05")+".png")

	type candidate struct {
		bin  string
		args []string
	}
	candidates := []candidate{
		{"grim", []string{path}},
		{"gnome-screenshot", []string{"-f", path}},
		{"spectacle", []string{"-b", "-n", "-o", path}},
		{"scrot", []string{path}},
		{"xfce4-screenshooter", []string{"-f", "-s", path}},
		{"import", []string{"-window", "root", path}},
	}
	for _, c := range candidates {
		if !hasBin(c.bin) {
			continue
		}
		cmd := exec.Command(c.bin, c.args...)
		if err := cmd.Run(); err != nil {
			continue
		}
		if PathExists(path) {
			return path, nil
		}
	}
	return "", fmt.Errorf("не нашёл утилиту для скриншотов (grim/gnome-screenshot/spectacle/scrot)")
}

func picturesDir() string {
	if dir := os.Getenv("XDG_PICTURES_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "."
	}
	return filepath.Join(home, "Pictures", "Screenshots")
}

func hasBin(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
