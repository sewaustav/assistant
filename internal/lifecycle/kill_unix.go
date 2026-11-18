//go:build !windows

package lifecycle

import (
	"os/exec"
	"syscall"
	"time"
)

func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessTree мягко останавливает всю группу процессов, затем добивает её.
// done закрывается горутиной, которая уже вызвала cmd.Wait(); повторно Wait
// вызывать нельзя, поэтому завершения ждём именно по этому каналу.
func killProcessTree(cmd *exec.Cmd, done <-chan struct{}) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	// SIGTERM всей группе
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)

	if done == nil {
		time.Sleep(5 * time.Second)
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		return
	}

	select {
	case <-done:
		// Умер сам
	case <-time.After(5 * time.Second):
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		// Не ждём вечно, если процесс завис в непрерываемом состоянии.
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}
}
