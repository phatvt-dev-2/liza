package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// WtCreateCommand creates a worktree for a CLAIMED task.
// It creates a worktree from the integration branch and updates the task's base_commit.
// If fresh is true, it will delete any existing worktree before creating a new one.
func WtCreateCommand(projectRoot, taskID string, fresh bool) error {
	// Validate input
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}

	// Setup paths
	lp := paths.New(projectRoot)
	worktreeRel := filepath.Join(paths.WorktreesDirName, taskID)
	worktreeDir := filepath.Join(lp.ProjectRoot(), worktreeRel)

	// Read state
	bb := db.New(lp.StatePath())
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

	// Validate task status
	if task.Status != models.TaskStatusClaimed {
		return fmt.Errorf("task %s is not CLAIMED (status: %s)", taskID, task.Status)
	}

	// Get integration branch from config
	integrationBranch := state.Config.IntegrationBranch

	// Initialize git wrapper
	gitWrapper := git.New(lp.ProjectRoot())

	// Check if worktree already exists
	if _, err := os.Stat(worktreeDir); err == nil {
		// Worktree exists
		if !fresh {
			// Worktree already exists and fresh is false, just return success
			fmt.Printf("Worktree already exists: %s\n", worktreeDir)
			return nil
		}
		fmt.Fprintln(os.Stderr, "Reassignment: deleting existing worktree")
	}

	// Create worktree (with fresh flag if needed)
	var baseCommit string
	if fresh {
		baseCommit, err = gitWrapper.CreateWorktreeFresh(taskID, integrationBranch)
	} else {
		baseCommit, err = gitWrapper.CreateWorktree(taskID, integrationBranch)
	}

	if err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	// Update task.base_commit in state
	err = bb.Modify(func(state *models.State) error {
		for i := range state.Tasks {
			if state.Tasks[i].ID == taskID {
				state.Tasks[i].BaseCommit = &baseCommit
				return nil
			}
		}
		return fmt.Errorf("task not found: %s", taskID)
	})

	if err != nil {
		// Clean up worktree on failure
		_ = gitWrapper.RemoveWorktree(taskID)
		return fmt.Errorf("failed to update state: %w", err)
	}

	fmt.Printf("Created worktree: %s\n", worktreeDir)
	return nil
}
