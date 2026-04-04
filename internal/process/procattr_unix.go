//go:build !windows

package process

import (
	"os/exec"
	"syscall"
)

// SetDetachedProcessGroup places the command in its own process group so the
// spawned agent survives the parent process exiting.
func SetDetachedProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
