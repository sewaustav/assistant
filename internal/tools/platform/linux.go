//go:build linux

package platform

import (
	"os/exec"
	"strings"
)

func init() {
	OpenBrowser = func(url string) error {
		return exec.Command("xdg-open", url).Start()
	}

	SendHotkey = func(keys string) error {
		// wtype: -M ctrl c -m ctrl
		// "ctrl+c" -> "-M", "ctrl", "c", "-m", "ctrl"
		parts := strings.Split(strings.ToLower(keys), "+")
		if len(parts) < 2 {
			// Одиночная клавиша, например "delete"
			return exec.Command("wtype", "-P", parts[0], "-p", parts[0]).Run()
		}

		mod := parts[0]
		key := parts[1]

		args := []string{"-M", mod, key, "-m", mod}
		return exec.Command("wtype", args...).Run()
	}
}