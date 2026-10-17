//go:build windows

package lifecycle

import (
	"os/exec"
	"strconv"
)

func configureProcessGroup(cmd *exec.Cmd) {
	// На Windows можно создать новую группу, но для taskkill это не обязательно.
	// Оставляем пустым: taskkill /T сам найдёт дерево по PID.
}

func killProcessTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	pid := cmd.Process.Pid
	// /T — убить дерево, /F — принудительно.
	killCmd := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	if err := killCmd.Run(); err != nil {
		// Fallback: убить хотя бы прямой процесс.
		_ = cmd.Process.Kill()
	}
	// Ждём завершения, чтобы не оставить зомби.
	_ = cmd.Wait()
}