package commands

import (
	"fmt"
	"os"

	"github.com/liza-mas/liza/internal/ops"
)

// WtCreateCommand creates a worktree for a task and prints the result to stdout.
// Delegates business logic to ops.CreateWorktree.
func WtCreateCommand(projectRoot, taskID string, fresh bool) error {
	result, err := ops.CreateWorktree(projectRoot, taskID, fresh)
	if err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}

	if result.AlreadyExisted {
		fmt.Printf("Worktree already exists: %s\n", result.WorktreeDir)
	} else {
		if fresh {
			fmt.Fprintln(os.Stderr, "Reassignment: deleting existing worktree")
		}
		fmt.Printf("Created worktree: %s\n", result.WorktreeDir)
	}

	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}
	return nil
}
