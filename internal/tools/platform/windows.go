//go:build windows

package platform

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/micmonay/keybd_event"
)

func init() {
	openBrowser = func(url string) error {
		return startDetached(exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url))
	}
	openPath = func(path string) error {
		return startDetached(exec.Command("cmd", "/c", "start", "", path))
	}
	sendHotkey = windowsSendHotkey
	typeText = windowsTypeText
	lockScreen = func() error {
		return exec.Command("rundll32.exe", "user32.dll,LockWorkStation").Run()
	}
	screenshot = windowsScreenshot
}

func startDetached(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func windowsSendHotkey(keys string) error {
	c := parseCombo(keys)
	if c.key == "" {
		return fmt.Errorf("пустая комбинация клавиш")
	}

	code, ok := windowsKeyCode(c.key)
	if !ok {
		return fmt.Errorf("неизвестная клавиша: %s", c.key)
	}

	kbd, err := keybd_event.NewKeyBonding()
	if err != nil {
		return fmt.Errorf("keybd_event: %w", err)
	}
	kbd.SetKeys(code)
	for _, mod := range c.mods {
		switch mod {
		case "ctrl":
			kbd.HasCTRL(true)
		case "shift":
			kbd.HasSHIFT(true)
		case "alt":
			kbd.HasALT(true)
		case "super":
			kbd.HasSuper(true)
		}
	}
	if err := kbd.Launching(); err != nil {
		return fmt.Errorf("keybd_event: %w", err)
	}
	return nil
}

func windowsKeyCode(key string) (int, bool) {
	if len(key) == 1 {
		ch := key[0]
		switch {
		case ch >= 'a' && ch <= 'z':
			if code, ok := letterKeys[ch]; ok {
				return code, true
			}
			return 0, false
		case ch >= '0' && ch <= '9':
			if ch == '0' {
				return keybd_event.VK_0, true
			}
			return keybd_event.VK_1 + int(ch-'1'), true
		}
	}
	if strings.HasPrefix(key, "f") {
		switch key {
		case "f1":
			return keybd_event.VK_F1, true
		case "f2":
			return keybd_event.VK_F2, true
		case "f3":
			return keybd_event.VK_F3, true
		case "f4":
			return keybd_event.VK_F4, true
		case "f5":
			return keybd_event.VK_F5, true
		case "f6":
			return keybd_event.VK_F6, true
		case "f7":
			return keybd_event.VK_F7, true
		case "f8":
			return keybd_event.VK_F8, true
		case "f9":
			return keybd_event.VK_F9, true
		case "f10":
			return keybd_event.VK_F10, true
		case "f11":
			return keybd_event.VK_F11, true
		case "f12":
			return keybd_event.VK_F12, true
		}
	}

	switch key {
	case "delete":
		return keybd_event.VK_DELETE, true
	case "insert":
		return keybd_event.VK_INSERT, true
	case "home":
		return keybd_event.VK_HOME, true
	case "end":
		return keybd_event.VK_END, true
	case "pageup":
		return keybd_event.VK_PAGEUP, true
	case "pagedown":
		return keybd_event.VK_PAGEDOWN, true
	case "up":
		return keybd_event.VK_UP, true
	case "down":
		return keybd_event.VK_DOWN, true
	case "left":
		return keybd_event.VK_LEFT, true
	case "right":
		return keybd_event.VK_RIGHT, true
	case "enter":
		return keybd_event.VK_ENTER, true
	case "escape":
		return keybd_event.VK_ESC, true
	case "tab":
		return keybd_event.VK_TAB, true
	case "space":
		return keybd_event.VK_SPACE, true
	case "backspace":
		return keybd_event.VK_BACKSPACE, true
	case "printscreen":
		return keybd_event.VK_SNAPSHOT, true
	case "volumeup":
		return keybd_event.VK_VOLUME_UP, true
	case "volumedown":
		return keybd_event.VK_VOLUME_DOWN, true
	case "mute":
		return keybd_event.VK_VOLUME_MUTE, true
	case "playpause":
		return keybd_event.VK_MEDIA_PLAY_PAUSE, true
	case "medianext":
		return keybd_event.VK_MEDIA_NEXT_TRACK, true
	case "mediaprev":
		return keybd_event.VK_MEDIA_PREV_TRACK, true
	case "mediastop":
		return keybd_event.VK_MEDIA_STOP, true
	}
	return 0, false
}

// letterKeys — скан-коды клавиш A–Z из keybd_event.
var letterKeys = map[byte]int{
	'a': keybd_event.VK_A,
	'b': keybd_event.VK_B,
	'c': keybd_event.VK_C,
	'd': keybd_event.VK_D,
	'e': keybd_event.VK_E,
	'f': keybd_event.VK_F,
	'g': keybd_event.VK_G,
	'h': keybd_event.VK_H,
	'i': keybd_event.VK_I,
	'j': keybd_event.VK_J,
	'k': keybd_event.VK_K,
	'l': keybd_event.VK_L,
	'm': keybd_event.VK_M,
	'n': keybd_event.VK_N,
	'o': keybd_event.VK_O,
	'p': keybd_event.VK_P,
	'q': keybd_event.VK_Q,
	'r': keybd_event.VK_R,
	's': keybd_event.VK_S,
	't': keybd_event.VK_T,
	'u': keybd_event.VK_U,
	'v': keybd_event.VK_V,
	'w': keybd_event.VK_W,
	'x': keybd_event.VK_X,
	'y': keybd_event.VK_Y,
	'z': keybd_event.VK_Z,
}

func windowsTypeText(text string) error {
	if text == "" {
		return nil
	}
	script := `$ErrorActionPreference = 'Stop'; Set-Clipboard -Value ([Console]::In.ReadToEnd())`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Stdin = strings.NewReader(text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("clipboard: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return windowsSendHotkey("ctrl+v")
}

func windowsScreenshot() (string, error) {
	script := `
Add-Type -AssemblyName System.Windows.Forms,System.Drawing
$bounds = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
$bmp = New-Object System.Drawing.Bitmap $bounds.Width, $bounds.Height
$graphics = [System.Drawing.Graphics]::FromImage($bmp)
$graphics.CopyFromScreen($bounds.Location, [System.Drawing.Point]::Empty, $bounds.Size)
$dir = Join-Path ([Environment]::GetFolderPath('MyPictures')) 'Screenshots'
New-Item -ItemType Directory -Force -Path $dir | Out-Null
$path = Join-Path $dir ('screenshot-' + (Get-Date -Format 'yyyy-MM-dd-HH-mm-ss') + '.png')
$bmp.Save($path, [System.Drawing.Imaging.ImageFormat]::Png)
$graphics.Dispose()
$bmp.Dispose()
Write-Output $path
`
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return "", fmt.Errorf("screenshot: %w", err)
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", fmt.Errorf("screenshot: powershell не вернул путь к файлу")
	}
	return filepath.Clean(path), nil
}
