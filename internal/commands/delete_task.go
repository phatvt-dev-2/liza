package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
)

// DeleteTaskCommand removes a task from the state database.
// Handles interactive confirmation at the CLI level (APPROVED warning,
// worktree deletion prompt), then delegates business logic to ops.DeleteTask.
func DeleteTaskCommand(projectRoot, taskID string, force, deleteWorktree bool, reason string) error {
	// Pre-check: validate and get info for interactive decisions
	info, err := ops.CheckDeleteTask(projectRoot, taskID, force)
	if err != nil {
		return err
	}

	// Interactive: warn about deleting APPROVED task
	if info.Status == models.TaskStatusApproved && !force {
		fmt.Fprintf(os.Stderr, "Warning: The task you are deleting has been implemented, reviewed, and approved. Are you sure you want to delete the task? (y/N): ")
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			fmt.Println("n")
			return fmt.Errorf("deletion cancelled")
		}
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer != "y" && answer != "yes" {
			return fmt.Errorf("deletion cancelled")
		}
	}

	// Interactive: ask about worktree deletion
	resolvedDeleteWt := deleteWorktree
	if info.HasWorktree && !deleteWorktree {
		fmt.Fprintf(os.Stderr, "Delete worktree at %s? (y/N): ", info.WorktreePath)
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer == "y" || answer == "yes" {
				resolvedDeleteWt = true
			}
		}
		// If scan failed (EOF, closed stdin), treat as 'n'
	}

	// Warn about forced deletion with dependents (non-interactive, informational only)
	if len(info.DependentTasks) > 0 && force {
		fmt.Fprintf(os.Stderr, "Warning: Tasks %v depend on %s and will have dangling dependencies\n", info.DependentTasks, info.TaskID)
	}

	result, err := ops.DeleteTask(projectRoot, taskID, force, resolvedDeleteWt, reason)
	if err != nil {
		return err
	}

	printDeleteResult(result)
	return nil
}

func printDeleteResult(r *ops.DeleteTaskResult) {
	for _, w := range r.Warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}

	fmt.Printf("Deleted task %s (was %s)\n", r.TaskID, r.PreviousStatus)
	if r.WorktreeDeleted {
		fmt.Println("Also deleted worktree and branch")
	} else if r.WorktreePreserved {
		fmt.Printf("Worktree at %s preserved (use wt-delete to clean up later)\n", r.WorktreePath)
	}
}
