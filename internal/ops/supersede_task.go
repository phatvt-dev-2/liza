package ops

import (
	"fmt"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// SupersedeResult contains the outcome of superseding a task.
type SupersedeResult struct {
	TaskID         string
	OriginalStatus models.TaskStatus
	ReplacementIDs []string
	Warnings       []string
}

// SupersedeTask transitions a BLOCKED, REJECTED, or READY task to SUPERSEDED,
// linking it to one or more replacement task IDs. No terminal I/O.
func SupersedeTask(projectRoot, taskID string, replacementIDs []string, reason, agentID string) (*SupersedeResult, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID is required")
	}
	if len(replacementIDs) == 0 {
		return nil, fmt.Errorf("at least one replacement task ID is required")
	}
	if reason == "" {
		return nil, fmt.Errorf("rescope reason is required")
	}
	if agentID == "" {
		return nil, fmt.Errorf("orchestrator agent ID is required")
	}

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())

	pb, err := loadPipelineBundle(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load pipeline config: %w", err)
	}

	// Phase 1: Read and Validate (no lock held)
	_, task, err := readTaskState(bb, taskID)
	if err != nil {
		return nil, err
	}

	originalStatus := task.Status
	if originalStatus != models.TaskStatusBlocked &&
		originalStatus != models.TaskStatusRejected &&
		originalStatus != models.TaskStatusReady {
		return nil, fmt.Errorf("cannot supersede task %s in status %s (must be BLOCKED, REJECTED, or READY)", taskID, originalStatus)
	}

	// Phase 2: Atomic State Update
	hadWorktree := task.Worktree != nil
	err = bb.Modify(func(state *models.State) error {
		currentTask := state.FindTask(taskID)
		if currentTask == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		if currentTask.Status != originalStatus {
			return fmt.Errorf("cannot supersede task %s: status changed from %s to %s", taskID, originalStatus, currentTask.Status)
		}

		if err := currentTask.TransitionWith(models.TaskStatusSuperseded, pb.transitions); err != nil {
			return err
		}
		currentTask.SupersededBy = replacementIDs
		currentTask.RescopeReason = &reason

		currentTask.AssignedTo = nil
		currentTask.LeaseExpires = nil
		currentTask.ReviewingBy = nil
		currentTask.ReviewLeaseExpires = nil
		currentTask.Worktree = nil

		now := time.Now().UTC()
		note := fmt.Sprintf("replaced by: %s", strings.Join(replacementIDs, ", "))
		currentTask.History = append(currentTask.History, models.TaskHistoryEntry{
			Time:   now,
			Event:  models.TaskEventSuperseded,
			Agent:  &agentID,
			Reason: &reason,
			Note:   &note,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to supersede task: %w", err)
	}

	// Best-effort worktree cleanup (after state commit — safe to lose worktree now)
	var warnings []string
	if hadWorktree {
		gw := git.New(projectRoot)
		if rmErr := gw.RemoveWorktree(taskID); rmErr != nil {
			warnings = append(warnings, fmt.Sprintf("failed to remove worktree: %v", rmErr))
		}
		taskBranch := paths.TaskBranchPrefix + taskID
		if exists, brErr := gw.BranchExists(taskBranch); brErr != nil {
			warnings = append(warnings, fmt.Sprintf("failed to check branch %s: %v", taskBranch, brErr))
		} else if exists {
			if delErr := gw.DeleteBranch(taskBranch); delErr != nil {
				warnings = append(warnings, fmt.Sprintf("failed to delete branch %s: %v", taskBranch, delErr))
			}
		}
	}

	return &SupersedeResult{
		TaskID:         taskID,
		OriginalStatus: originalStatus,
		ReplacementIDs: replacementIDs,
		Warnings:       warnings,
	}, nil
}
