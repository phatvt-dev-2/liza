//go:build !windows

package tui

import (
	"os/exec"
	"syscall"
)

// setDetachedProcessGroup places the command in its own process group so the
// spawned agent survives the parent TUI exiting.
func setDetachedProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
