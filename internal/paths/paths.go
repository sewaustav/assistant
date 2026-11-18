// Package paths находит файлы приложения на любой ОС.
//
// Раньше пути были захардкожены как ~/.cargo/bin/gigastt и
// tts-server/.venv/bin/python. На Windows это не работает: домашняя папка
// берётся из USERPROFILE, у бинарников есть суффикс .exe, а venv лежит в
// Scripts/. Этот пакет централизует всю эту логику.
package paths

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	gigaSTTName  = "gigastt"
	sileroScript = "silero_server.py"
)

// IsWindows сообщает, запущены ли мы на Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// Exe добавляет .exe к имени файла на Windows.
func Exe(name string) string {
	if IsWindows() && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		return name + ".exe"
	}
	return name
}

// HomeDir возвращает домашнюю папку пользователя.
func HomeDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	// На всякий случай: HOME на Unix, USERPROFILE на Windows.
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	return os.Getenv("USERPROFILE")
}

// ExecutableDir возвращает папку, в которой лежит запущенный бинарник.
func ExecutableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		exe, _ = os.Executable()
	}
	return filepath.Dir(exe)
}

// FindTTSDir ищет папку tts-server.
//
// Порядок: явный путь из конфига, переменная окружения, текущая папка и её
// родители, папка бинарника и её родители.
func FindTTSDir(override string) string {
	if override != "" {
		if abs, err := filepath.Abs(override); err == nil {
			return abs
		}
		return override
	}
	if env := os.Getenv("ASSISTANT_TTS_DIR"); env != "" {
		if abs, err := filepath.Abs(env); err == nil {
			return abs
		}
		return env
	}

	starts := []string{}
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if exeDir := ExecutableDir(); exeDir != "" {
		starts = append(starts, exeDir)
	}

	for _, start := range starts {
		dir := start
		for i := 0; i < 6; i++ {
			candidate := filepath.Join(dir, "tts-server")
			if fileExists(filepath.Join(candidate, sileroScript)) {
				return candidate
			}
			// Установленный вариант: скрипт лежит прямо рядом с бинарником.
			if fileExists(filepath.Join(dir, sileroScript)) {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	return ""
}

// FindPython ищет интерпретатор Python из venv, а затем системный.
func FindPython(ttsDir, override string) string {
	if override != "" {
		if fileExists(override) {
			return override
		}
		if p, err := exec.LookPath(override); err == nil {
			return p
		}
	}
	if ttsDir == "" {
		return ""
	}

	venvs := []string{
		filepath.Join(ttsDir, ".venv"),
		filepath.Join(ttsDir, "venv"),
	}
	for _, venv := range venvs {
		candidates := []string{
			filepath.Join(venv, "bin", "python3"),
			filepath.Join(venv, "bin", "python"),
			filepath.Join(venv, "Scripts", "python.exe"),
			filepath.Join(venv, "Scripts", "python3.exe"),
		}
		for _, candidate := range candidates {
			if fileExists(candidate) {
				return candidate
			}
		}
	}

	candidates := []string{"python3", "python"}
	if IsWindows() {
		candidates = []string{"python", "python3", "py"}
	}
	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// FindGigaSTT ищет бинарник gigastt.
func FindGigaSTT(override string) string {
	if override != "" {
		if fileExists(override) {
			return override
		}
		if p, err := exec.LookPath(override); err == nil {
			return p
		}
	}
	if env := os.Getenv("ASSISTANT_GIGASTT"); env != "" {
		if fileExists(env) {
			return env
		}
	}
	if p, err := exec.LookPath(gigaSTTName); err == nil {
		return p
	}
	home := HomeDir()
	if home == "" {
		return ""
	}
	candidates := []string{
		filepath.Join(home, ".cargo", "bin", Exe(gigaSTTName)),
		filepath.Join(home, "bin", Exe(gigaSTTName)),
	}
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
