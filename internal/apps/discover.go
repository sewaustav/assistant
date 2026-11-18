package apps

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// systemApps возвращает приложения, установленные в системе.
func systemApps() []App {
	switch runtime.GOOS {
	case "windows":
		return windowsApps()
	case "darwin":
		return darwinApps()
	default:
		return linuxApps()
	}
}

func linuxApps() []App {
	home := os.Getenv("HOME")
	dirs := []string{
		"/usr/share/applications",
		"/usr/local/share/applications",
		"/var/lib/flatpak/exports/share/applications",
		"/var/lib/snapd/desktop/applications",
	}
	if home != "" {
		dirs = append(dirs,
			filepath.Join(home, ".local", "share", "applications"),
			filepath.Join(home, ".local", "share", "flatpak", "exports", "share", "applications"),
		)
	}

	var out []App
	seen := make(map[string]struct{})
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".desktop") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			if app, ok := parseDesktopFile(path); ok {
				out = append(out, app)
			}
		}
	}
	return out
}

type desktopEntry struct {
	name        string
	names       []string
	genericName string
	exec        string
	noDisplay   bool
	hidden      bool
	entryType   string
}

func parseDesktopFile(path string) (App, bool) {
	f, err := os.Open(path)
	if err != nil {
		return App{}, false
	}
	defer f.Close()

	var entry desktopEntry
	inMain := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inMain = line == "[Desktop Entry]"
			continue
		}
		if !inMain {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch {
		case key == "Name":
			entry.name = value
			entry.names = append(entry.names, value)
		case key == "Name[ru]" || key == "Name[ru_RU]":
			entry.names = append(entry.names, value)
		case key == "GenericName":
			entry.genericName = value
		case key == "Exec":
			entry.exec = value
		case key == "NoDisplay":
			entry.noDisplay = value == "true"
		case key == "Hidden":
			entry.hidden = value == "true"
		case key == "Type":
			entry.entryType = value
		}
	}

	if entry.name == "" || entry.exec == "" || entry.noDisplay || entry.hidden {
		return App{}, false
	}
	if entry.entryType != "" && entry.entryType != "Application" {
		return App{}, false
	}

	execLine := cleanDesktopExec(entry.exec)
	if execLine == "" {
		return App{}, false
	}
	base := strings.TrimSuffix(filepath.Base(path), ".desktop")
	aliases := []string{base}
	aliases = append(aliases, entry.names...)
	if entry.genericName != "" {
		aliases = append(aliases, entry.genericName)
	}

	return App{
		Name:    entry.name,
		Aliases: aliases,
		Target:  path,
		launch: func() error {
			return launchCommand(execLine)
		},
	}, true
}

// cleanDesktopExec убирает %f/%U и прочие коды полей из Exec.
func cleanDesktopExec(execLine string) string {
	fields := strings.Fields(execLine)
	out := fields[:0]
	for _, field := range fields {
		if strings.HasPrefix(field, "%") {
			continue
		}
		// Убираем коды вида %f внутри аргумента (например, --file=%f).
		if strings.Contains(field, "%") {
			for _, code := range []string{"%f", "%F", "%u", "%U", "%d", "%D", "%n", "%N", "%i", "%c", "%k", "%v", "%m"} {
				field = strings.ReplaceAll(field, code, "")
			}
			if field == "" {
				continue
			}
		}
		out = append(out, field)
	}
	return strings.TrimSpace(strings.Join(out, " "))
}

func windowsApps() []App {
	dirs := []string{
		filepath.Join(os.Getenv("ProgramData"), "Microsoft", "Windows", "Start Menu", "Programs"),
		filepath.Join(os.Getenv("AppData"), "Microsoft", "Windows", "Start Menu", "Programs"),
	}

	var out []App
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".lnk" && ext != ".url" {
				return nil
			}
			name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			lower := strings.ToLower(name)
			if strings.Contains(lower, "uninstall") || strings.Contains(lower, "деинстал") ||
				strings.Contains(lower, "readme") || strings.Contains(lower, "license") {
				return nil
			}
			appPath := path
			out = append(out, App{
				Name:    name,
				Aliases: []string{strings.TrimSpace(strings.TrimSuffix(name, " - Shortcut"))},
				Target:  path,
				launch: func() error {
					return launchFile(appPath)
				},
			})
			return nil
		})
	}
	return out
}

func darwinApps() []App {
	home := os.Getenv("HOME")
	dirs := []string{"/Applications", "/System/Applications", "/System/Applications/Utilities"}
	if home != "" {
		dirs = append(dirs, filepath.Join(home, "Applications"))
	}

	var out []App
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".app") {
				path := filepath.Join(dir, entry.Name())
				name := strings.TrimSuffix(entry.Name(), ".app")
				out = append(out, App{
					Name:   name,
					Target: path,
					launch: func() error {
						return openAppBundle(path)
					},
				})
			}
		}
	}
	return out
}
