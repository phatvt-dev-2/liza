package ops

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// MarkBlockedResult contains the outcome of marking a task as blocked.
type MarkBlockedResult struct {
	TaskID string
	Reason string
}

// MarkBlocked transitions a task from IMPLEMENTING to BLOCKED. Only the
// assigned agent can block its own task. Requires reason and 1-3 clarifying
// questions per the blocking protocol. No terminal I/O.
func MarkBlocked(projectRoot, taskID, reason string, questions []string, agentID string) (*MarkBlockedResult, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID is required")
	}
	if reason == "" {
		return nil, fmt.Errorf("reason is required")
	}
	if agentID == "" {
		return nil, fmt.Errorf("agent ID is required")
	}
	if len(questions) == 0 {
		return nil, fmt.Errorf("at least 1 question is required")
	}
	if len(questions) > 3 {
		return nil, fmt.Errorf("maximum 3 questions allowed per blocking protocol")
	}

	lp := paths.New(projectRoot)
	bb := db.New(lp.StatePath())
	now := time.Now().UTC()

	err := bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return fmt.Errorf("task not found: %s", taskID)
		}

		if task.Status != models.TaskStatusImplementing {
			return fmt.Errorf("task must be in IMPLEMENTING status to be marked blocked, current status: %s", task.Status)
		}

		if task.AssignedTo == nil || *task.AssignedTo != agentID {
			return fmt.Errorf("only the assigned agent can mark task as blocked")
		}

		if err := task.Transition(models.TaskStatusBlocked); err != nil {
			return err
		}
		task.BlockedReason = &reason
		task.BlockedQuestions = questions
		task.AssignedTo = nil
		task.LeaseExpires = nil

		task.History = append(task.History, models.TaskHistoryEntry{
			Time:   now,
			Event:  "blocked",
			Agent:  &agentID,
			Reason: &reason,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to mark task as blocked: %w", err)
	}

	return &MarkBlockedResult{
		TaskID: taskID,
		Reason: reason,
	}, nil
}
