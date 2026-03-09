//go:build !windows

package ops

import (
	"os/exec"
	"syscall"
)

// configProcessGroupKill sets up the command to run in its own process group
// and kill the entire group on context cancellation. This prevents child
// processes from holding stdout/stderr pipes open after the parent is killed.
func configProcessGroupKill(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
