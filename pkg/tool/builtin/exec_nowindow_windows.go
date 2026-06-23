//go:build windows

package toolbuiltin

import (
	"os/exec"
	"syscall"
)

func applyExecNoWindow(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
