// Package process provides shared subprocess management for agent spawning.
// Used by both the TUI and the HTTP API server.
package process

import (
	"fmt"
	"os"
	"os/exec"
)

// SpawnAgent starts a detached `liza agent` subprocess with stdout/stderr
// redirected to /dev/null. The child process is placed in its own process
// group and a background goroutine reaps it to prevent zombie accumulation.
//
// Returns the started command and an error. The caller owns lifecycle
// management (the process is already started and will be reaped).
func SpawnAgent(projectRoot, role, cli string, extraArgs ...string) (*exec.Cmd, error) {
	args := []string{"agent", role, "--cli", cli}
	args = append(args, extraArgs...)

	cmd := exec.Command("liza", args...)
	cmd.Dir = projectRoot
	SetDetachedProcessGroup(cmd)

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("open devnull: %w", err)
	}
	cmd.Stdout = devNull
	cmd.Stderr = devNull

	if err := cmd.Start(); err != nil {
		devNull.Close()
		return nil, err
	}
	go func() {
		cmd.Wait()
		devNull.Close()
	}()

	return cmd, nil
}
