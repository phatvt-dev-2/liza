package commands

import (
	"fmt"
	"os"

	"github.com/liza-mas/liza/internal/ops"
)

// WtDeleteCommand deletes a worktree for a task and prints the result to stdout.
// Delegates business logic to ops.DeleteWorktree.
func WtDeleteCommand(projectRoot, taskID string) error {
	result, err := ops.DeleteWorktree(projectRoot, taskID)
	if err != nil {
		return fmt.Errorf("delete worktree: %w", err)
	}

	if !result.Existed {
		fmt.Printf("No worktree for task %s\n", result.TaskID)
		return nil
	}

	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}

	fmt.Printf("Deleted worktree for %s (was %s)\n", result.TaskID, result.PreviousStatus)
	return nil
}
