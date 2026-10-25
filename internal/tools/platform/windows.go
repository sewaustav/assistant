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
		return fmt.Errorf("hotkey на Windows пока не реализован")
	}
}