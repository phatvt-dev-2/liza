package ops

import (
	"fmt"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// DeleteWorktreeResult contains the outcome of deleting a worktree.
type DeleteWorktreeResult struct {
	TaskID         string
	PreviousStatus models.TaskStatus
	Existed        bool
	Warnings       []string
}

// DeleteWorktree removes a task's git worktree and branch. For safety, only
// BLOCKED, ABANDONED, SUPERSEDED, or MERGED tasks are eligible. No terminal I/O.
func DeleteWorktree(projectRoot, taskID string) (*DeleteWorktreeResult, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID is required")
	}

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	_, task, err := readTaskState(bb, taskID)
	if err != nil {
		return nil, err
	}

	var warnings []string

	switch task.Status {
	case models.TaskStatusBlocked, models.TaskStatusAbandoned, models.TaskStatusSuperseded:
		// Safe to delete
	case models.TaskStatusMerged:
		warnings = append(warnings, fmt.Sprintf("Task %s is MERGED — worktree should already be cleaned", taskID))
	default:
		return nil, fmt.Errorf("cannot delete worktree for task %s (status: %s), deletion only allowed for: BLOCKED, ABANDONED, SUPERSEDED, MERGED", taskID, task.Status)
	}

	if task.Worktree == nil {
		return &DeleteWorktreeResult{
			TaskID:         taskID,
			PreviousStatus: task.Status,
			Existed:        false,
		}, nil
	}

	gitWrapper := git.New(projectRoot)

	if err := gitWrapper.RemoveWorktree(taskID); err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to remove worktree: %v", err))
	}

	branchName := paths.TaskBranchPrefix + taskID
	_ = gitWrapper.DeleteBranch(branchName)

	err = bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}
		task.Worktree = nil
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update state: %w", err)
	}

	return &DeleteWorktreeResult{
		TaskID:         taskID,
		PreviousStatus: task.Status,
		Existed:        true,
		Warnings:       warnings,
	}, nil
}
