//go:build windows

package lifecycle

import (
	"os/exec"
	"strconv"
	"time"
)

func configureProcessGroup(cmd *exec.Cmd) {
	// На Windows можно создать новую группу, но для taskkill это не обязательно.
	// Оставляем пустым: taskkill /T сам найдёт дерево по PID.
}

// killProcessTree убивает дерево процессов через taskkill.
// done закрывается горутиной, которая уже вызвала cmd.Wait(); повторно Wait
// вызывать нельзя, поэтому завершения ждём именно по этому каналу.
func killProcessTree(cmd *exec.Cmd, done <-chan struct{}) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	pid := cmd.Process.Pid
	// /T — убить дерево, /F — принудительно.
	killCmd := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	if err := killCmd.Run(); err != nil {
		// Fallback: убить хотя бы прямой процесс.
		_ = cmd.Process.Kill()
	}

	if done == nil {
		return
	}
	// Ждём завершения, чтобы не оставить зомби.
	select {
	case <-done:
	case <-time.After(10 * time.Second):
	}
}
