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
// Delegates business logic to ops.DeleteAgent.
// The stdin parameter allows for injected input in tests; pass os.Stdin for CLI usage.
func DeleteAgentCommand(projectRoot, agentID string, force bool, reason string, stdin io.Reader) error {
	if stdin == nil {
		stdin = os.Stdin
	}

	pidConfirmed := false
	if !force && agentID != "" {
		confirmed, err := confirmRunningProcess(projectRoot, agentID, stdin)
		if err != nil {
			return err
		}
		pidConfirmed = confirmed
	}

	result, err := ops.DeleteAgent(projectRoot, agentID, force, pidConfirmed, reason)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}

	fmt.Printf("Deleted agent %s\n", result.AgentID)
	return nil
}

// confirmRunningProcess prompts the user if the agent process is still running.
// Returns true if the user confirmed deletion, false if not running.
// Interactive confirmation is CLI-only — ops.DeleteAgent handles business logic validation.
func confirmRunningProcess(projectRoot, agentID string, stdin io.Reader) (bool, error) {
	running, pid, err := ops.IsAgentProcessRunning(projectRoot, agentID)
	if err != nil {
		return false, fmt.Errorf("check agent process: %w", err)
	}
	if !running {
		return false, nil
	}

	fmt.Fprintf(os.Stderr, "Agent %s is still running with PID %d, do you want to delete the agent from the state file? (y/n): ", agentID, pid)
	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		return false, fmt.Errorf("deletion cancelled")
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer != "y" && answer != "yes" {
		return false, fmt.Errorf("deletion cancelled by user")
	}
	// Only bypass PID check, not lease/task checks
	return true, nil
}
