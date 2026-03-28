//go:build windows

package tui

import "os/exec"

// setDetachedProcessGroup is a no-op on Windows. Windows has no Unix process
// groups; Start() is sufficient for detached child processes.
func setDetachedProcessGroup(cmd *exec.Cmd) {
	_ = cmd
}
