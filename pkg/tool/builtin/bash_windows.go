//go:build windows

package toolbuiltin

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func bashOutputBaseDir() string {
	return filepath.Join(os.TempDir(), "agentsdk", "bash-output")
}

func newBashExecCmd(ctx context.Context, command string) *exec.Cmd {
	shell, args := resolveWindowsShell(command)
	cmd := exec.CommandContext(ctx, shell, args...)
	// Windows 下彻底静默无窗口执行：
	// CREATE_NO_WINDOW (0x08000000) — 不为子进程创建新的控制台窗口
	// CREATE_UNICODE_ENVIRONMENT (0x00040000) — 使用 Unicode 环境块
	// DETACHED_PROCESS (0x00000008) — 脱离父进程控制台
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000 | 0x20000000 | 0x00000008,
	}
	return cmd
}

func resolveWindowsShell(command string) (exe string, argv []string) {
	// 优先从 PATH 环境变量中查找 Git Bash，避免直接命中 WSL/System32/bash.exe
	for _, name := range []string{"bash.exe", "bash"} {
		if path, err := exec.LookPath(name); err == nil && path != "" && !isWindowsSystemBash(path) {
			return path, []string{"-c", command}
		}
	}
	// 其次通过 git 的位置推导 bash 路径
	for _, gitName := range []string{"git.exe", "git"} {
		if git, err := exec.LookPath(gitName); err == nil && git != "" {
			if bash := bashFromGitPath(git); bash != "" {
				return bash, []string{"-c", command}
			}
		}
	}
	// 最后回退到 cmd.exe
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		if path, err := exec.LookPath("cmd.exe"); err == nil {
			comspec = path
		} else {
			comspec = "cmd.exe"
		}
	}
	log.Println("ComSpec:", comspec)
	return comspec, []string{"/d", "/s", "/c", command}
}

func isWindowsSystemBash(path string) bool {
	p := filepath.Clean(strings.ToLower(strings.TrimSpace(path)))
	win := filepath.Clean(strings.ToLower(os.Getenv("WINDIR")))
	if win == "" {
		win = `c:\windows`
	}
	return p == filepath.Join(win, "system32", "bash.exe")
}

func bashFromGitPath(gitPath string) string {
	gitPath = strings.TrimSpace(gitPath)
	if gitPath == "" {
		return ""
	}
	dir := filepath.Dir(gitPath)
	root := filepath.Dir(dir)
	candidates := []string{
		filepath.Join(root, "bin", "bash.exe"),
		filepath.Join(root, "usr", "bin", "bash.exe"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}
