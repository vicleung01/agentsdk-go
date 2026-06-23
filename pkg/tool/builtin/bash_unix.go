//go:build !windows

package toolbuiltin

import (
	"context"
	"os/exec"
	"path/filepath"
)

func bashOutputBaseDir() string {
	return filepath.Join(string(filepath.Separator), "tmp", "agentsdk", "bash-output")
}

func newBashExecCmd(ctx context.Context, command string) *exec.Cmd {
	return exec.CommandContext(ctx, "bash", "-c", command)
}
