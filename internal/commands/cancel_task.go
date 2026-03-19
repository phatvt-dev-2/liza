package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

// CancelTaskCommand cancels a task (transitions to ABANDONED) and prints the result to stdout.
// Delegates business logic to ops.CancelTask.
func CancelTaskCommand(projectRoot, taskID, reason, agentID string) error {
	result, err := ops.CancelTask(projectRoot, taskID, reason, agentID)
	if err != nil {
		return fmt.Errorf("cancel task: %w", err)
	}

	fmt.Printf("Cancelled task %s (was %s)\n", result.TaskID, result.OriginalStatus)
	return nil
}
