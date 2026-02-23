package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/liza-mas/liza/internal/ops"
)

// DeleteAgentCommand removes an agent from the state database and prints the result.
// Handles interactive confirmation for running processes at the CLI level,
// then delegates business logic to ops.DeleteAgent.
// The stdin parameter allows for injected input in tests; pass os.Stdin for CLI usage.
func DeleteAgentCommand(projectRoot, agentID string, force bool, reason string, stdin io.Reader) error {
	if stdin == nil {
		stdin = os.Stdin
	}
	// Check if agent process is running (interactive confirmation is CLI-only)
	pidConfirmed := false
	if !force && agentID != "" {
		running, pid, err := ops.IsAgentProcessRunning(projectRoot, agentID)
		if err != nil {
			return fmt.Errorf("check agent process: %w", err)
		}
		if running {
			fmt.Fprintf(os.Stderr, "Agent %s is still running with PID %d, do you want to delete the agent from the state file? (y/n): ", agentID, pid)
			scanner := bufio.NewScanner(stdin)
			if !scanner.Scan() {
				return fmt.Errorf("deletion cancelled")
			}
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer != "y" && answer != "yes" {
				return fmt.Errorf("deletion cancelled by user")
			}
			// User confirmed PID — only bypass PID check, not lease/task checks
			pidConfirmed = true
		}
	}

	result, err := ops.DeleteAgent(projectRoot, agentID, force, pidConfirmed, reason)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}

	fmt.Printf("Deleted agent %s\n", result.AgentID)
	return nil
}
