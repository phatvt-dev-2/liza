package commands

import (
	"fmt"
	"os"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
)

// WtDeleteCommand deletes a worktree for a task.
// For safety, deletion is only allowed for BLOCKED, ABANDONED, SUPERSEDED, or MERGED tasks.
// This prevents accidental destruction of in-progress work.
func WtDeleteCommand(projectRoot, taskID string) error {
	// Validate input
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}

	// Setup paths
	statePath := projectRoot + "/.liza/state.yaml"

	// Read state
	bb := db.New(statePath)
	state, err := bb.Read()
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	// Find task
	var task *models.Task
	for i := range state.Tasks {
		if state.Tasks[i].ID == taskID {
			task = &state.Tasks[i]
			break
		}
	}

	if task == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// Validate task status - only allow deletion for safe statuses
	switch task.Status {
	case models.TaskStatusBlocked, models.TaskStatusAbandoned, models.TaskStatusSuperseded:
		// Safe to delete
	case models.TaskStatusMerged:
		// Allow deletion but warn
		fmt.Fprintf(os.Stderr, "Warning: Task %s is MERGED — worktree should already be cleaned\n", taskID)
	default:
		return fmt.Errorf("Cannot delete worktree for task %s (status: %s)\nDeletion only allowed for: BLOCKED, ABANDONED, SUPERSEDED, MERGED\nIf task is CLAIMED, the Coder may be actively working in this worktree.", taskID, task.Status)
	}

	// Check if task has worktree
	if task.Worktree == nil {
		fmt.Printf("No worktree for task %s\n", taskID)
		return nil
	}

	// Initialize git wrapper
	gitWrapper := git.New(projectRoot)

	// Remove worktree
	if err := gitWrapper.RemoveWorktree(taskID); err != nil {
		// Log error but continue - directory might already be gone
		fmt.Fprintf(os.Stderr, "Warning: failed to remove worktree: %v\n", err)
	}

	// Delete branch (ignore errors if branch doesn't exist)
	branchName := "task/" + taskID
	_ = gitWrapper.DeleteBranch(branchName)

	// Update task.worktree to null in state
	err = bb.Modify(func(state *models.State) error {
		for i := range state.Tasks {
			if state.Tasks[i].ID == taskID {
				state.Tasks[i].Worktree = nil
				return nil
			}
		}
		return fmt.Errorf("task not found: %s", taskID)
	})

	if err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	fmt.Printf("Deleted worktree for %s (was %s)\n", taskID, task.Status)
	return nil
}
