//go:build windows

package ops

import "os/exec"

// configProcessGroupKill is a no-op on Windows. exec.CommandContext already
// calls Process.Kill on deadline, which terminates the process. Windows does
// not have Unix process groups, but the WaitDelay set by the caller ensures
// cmd.Wait returns even if child processes hold pipes open.
func configProcessGroupKill(cmd *exec.Cmd) {
	// On Windows, CommandContext's default kill behavior is sufficient.
	// WaitDelay (set at the call site) handles pipe draining.
	_ = cmd
}
