//go:build windows

package process

import "os/exec"

// SetDetachedProcessGroup is a no-op on Windows. Windows has no Unix process
// groups; Start() is sufficient for detached child processes.
func SetDetachedProcessGroup(cmd *exec.Cmd) {
	_ = cmd
}
