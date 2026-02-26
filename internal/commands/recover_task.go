package commands

import (
	"fmt"
	"os"

	"github.com/liza-mas/liza/internal/ops"
)

// RecoverTaskCommand recovers a task by releasing claims, removing worktree/branch,
// and optionally recovering the claiming agent. Prints results to stdout.
func RecoverTaskCommand(projectRoot, taskID string, force bool, reason string) error {
	result, err := ops.RecoverTask(projectRoot, taskID, force, reason)
	if err != nil {
		return fmt.Errorf("recover task: %w", err)
	}

	// Print warnings first — they're relevant even when "nothing to recover"
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "  Warning: %s\n", w)
	}

	if !result.InState && !result.WorktreeRemoved && !result.BranchRemoved {
		fmt.Printf("Task %s: nothing to recover (not in state, no git artifacts)\n", taskID)
		return nil
	}

	fmt.Printf("Recovered task %s\n", result.TaskID)
	if result.InState {
		fmt.Printf("  In state: yes\n")
	} else {
		fmt.Printf("  In state: no (git-only cleanup)\n")
	}
	if result.AgentID != "" {
		fmt.Printf("  Agent: %s (%s)\n", result.AgentID, result.AgentRole)
	}
	if result.ClaimReleased {
		fmt.Printf("  Claim released: yes\n")
	}
	if result.WorktreeRemoved {
		fmt.Printf("  Worktree removed: yes\n")
	}
	if result.BranchRemoved {
		fmt.Printf("  Branch removed: yes\n")
	}
	if result.AgentRecovered {
		fmt.Printf("  Agent recovered: yes\n")
	}

	return nil
}
