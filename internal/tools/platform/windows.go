//go:build windows

package platform

import (
	"fmt"
	"os/exec"
)

func init() {
	OpenBrowser = func(url string) error {
		return exec.Command("cmd", "/c", "start", "", url).Start()
	}
	SendHotkey = func(keys string) error {
		// Преобразуем "ctrl+c" в формат SendKeys: "^c"
		// ctrl = ^, shift = +, alt = %
		var out string
		i := 0
		for i < len(keys) {
			switch {
			case keys[i] == 'c' && i+4 <= len(keys) && keys[i:i+4] == "ctrl":
				out += "^"
				i += 4
			case keys[i] == 's' && i+5 <= len(keys) && keys[i:i+5] == "shift":
				out += "+"
				i += 5
			case keys[i] == 'a' && i+3 <= len(keys) && keys[i:i+3] == "alt":
				out += "%"
				i += 3
			case keys[i] == '+':
				i++
			default:
				out += string(keys[i])
				i++
			}
		}

		script := fmt.Sprintf(
			`Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.SendKeys]::SendWait("%s")`,
			out,
		)
		return exec.Command("powershell", "-NoProfile", "-Command", script).Run()
	}
}