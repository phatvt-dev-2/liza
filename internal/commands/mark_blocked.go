package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

// MarkBlockedCommand marks a task as BLOCKED and prints the result to stdout.
// Delegates business logic to ops.MarkBlocked.
func MarkBlockedCommand(projectRoot, taskID, reason string, questions []string, agentID string) error {
	result, err := ops.MarkBlocked(projectRoot, taskID, reason, questions, agentID)
	if err != nil {
		return fmt.Errorf("mark blocked: %w", err)
	}

	fmt.Printf("Task %s marked as BLOCKED\nReason: %s\n", result.TaskID, result.Reason)
	return nil
}
