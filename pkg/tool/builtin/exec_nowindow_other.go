//go:build !windows

package toolbuiltin

import "os/exec"

func applyExecNoWindow(cmd *exec.Cmd) {}
