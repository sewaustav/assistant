package apps

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type seed struct {
	name    string
	aliases []string
	linux   []string
	windows []string
	darwin  []string
}

// seedApps — небольшой список частых приложений и системных утилит.
// Он нужен, чтобы команды работали даже если в системе нет .desktop/меню «Пуск».
func seedApps() []App {
	var out []App
	for _, s := range seeds {
		candidates := s.linux
		switch runtime.GOOS {
		case "windows":
			candidates = s.windows
		case "darwin":
			candidates = s.darwin
		}

		cmd, ok := pickCommand(candidates)
		if !ok {
			continue
		}
		command := cmd
		out = append(out, App{
			Name:    s.name,
			Aliases: append([]string{}, s.aliases...),
			Target:  command,
			launch: func() error {
				return launchCommand(command)
			},
		})
	}
	return out
}

func pickCommand(candidates []string) (string, bool) {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if runtime.GOOS == "windows" {
			// cmd /c start умеет и calc.exe, и ms-settings:, проверять PATH не нужно.
			return candidate, true
		}
		if commandExists(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func commandExists(command string) bool {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}
	bin := fields[0]
	if filepath.IsAbs(bin) {
		return PathExists(bin)
	}
	_, err := exec.LookPath(bin)
	return err == nil
}

var seeds = []seed{
	{
		name:    "Браузер",
		aliases: []string{"browser", "веб", "интернет"},
		linux:   []string{"xdg-open https://ya.ru", "firefox", "chromium", "google-chrome", "brave-browser"},
		windows: []string{"msedge.exe", "chrome.exe", "firefox.exe"},
		darwin:  []string{"open -a Safari"},
	},
	{
		name:    "Терминал",
		aliases: []string{"terminal", "консоль", "командная строка", "шелл"},
		linux:   []string{"x-terminal-emulator", "gnome-terminal", "konsole", "alacritty", "kitty", "foot", "xterm"},
		windows: []string{"wt.exe", "cmd.exe", "powershell.exe"},
		darwin:  []string{"open -a Terminal"},
	},
	{
		name:    "Файлы",
		aliases: []string{"files", "проводник", "файловый менеджер", "папки", "домашняя папка"},
		linux:   []string{"nautilus", "thunar", "dolphin", "nemo", "pcmanfm", "xdg-open ."},
		windows: []string{"explorer.exe"},
		darwin:  []string{"open -a Finder"},
	},
	{
		name:    "Калькулятор",
		aliases: []string{"calculator", "calc", "счёт", "посчитать"},
		linux:   []string{"gnome-calculator", "kcalc", "galculator", "qalculate-gtk"},
		windows: []string{"calc.exe", "ms-calculator:"},
		darwin:  []string{"open -a Calculator"},
	},
	{
		name:    "Текстовый редактор",
		aliases: []string{"editor", "блокнот", "заметки", "редактор"},
		linux:   []string{"gnome-text-editor", "gedit", "kate", "kwrite", "mousepad", "xed", "pluma"},
		windows: []string{"notepad.exe"},
		darwin:  []string{"open -a TextEdit"},
	},
	{
		name:    "Настройки",
		aliases: []string{"settings", "параметры", "настройки системы"},
		linux:   []string{"gnome-control-center", "systemsettings", "xfce4-settings-manager"},
		windows: []string{"ms-settings:"},
		darwin:  []string{"open -a \"System Settings\""},
	},
	{
		name:    "Диспетчер задач",
		aliases: []string{"task manager", "монитор", "системный монитор", "процессы"},
		linux:   []string{"gnome-system-monitor", "plasma-systemmonitor", "ksysguard", "xfce4-taskmanager"},
		windows: []string{"taskmgr.exe"},
		darwin:  []string{"open -a \"Activity Monitor\""},
	},
	{
		name:    "Ножницы",
		aliases: []string{"snipping tool", "скриншот", "снимок экрана"},
		windows: []string{"snippingtool.exe"},
		darwin:  []string{"/System/Applications/Utilities/Screenshot.app"},
	},
	{
		name:    "Камера",
		aliases: []string{"camera", "фото"},
		windows: []string{"microsoft.windows.camera:"},
		darwin:  []string{"open -a \"Photo Booth\""},
	},
	{
		name:    "Часы",
		aliases: []string{"clock", "будильник", "таймер"},
		windows: []string{"ms-clock:"},
	},
	{
		name:    "Фотографии",
		aliases: []string{"photos", "галерея", "изображения"},
		windows: []string{"ms-photos:"},
		darwin:  []string{"open -a Photos"},
	},
	{
		name:    "Музыка",
		aliases: []string{"music", "плеер", "аудио", "песни"},
		linux:   []string{"spotify", "rhythmbox", "clementine", "audacious", "elisa", "lollypop", "mpv"},
		windows: []string{"wmplayer.exe"},
		darwin:  []string{"open -a Music"},
	},
}
