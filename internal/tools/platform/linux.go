//go:build linux

package platform

import "os/exec"

func init() {
	OpenBrowser = func(url string) error {
		return exec.Command("xdg-open", url).Start()
	}
	SendHotkey = func(keys string) error {
		return exec.Command("xdotool", "key", keys).Run()
	}
}