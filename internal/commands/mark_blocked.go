package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// MarkBlockedCommand marks a task as BLOCKED when work cannot proceed.
// Only the assigned agent can mark a task as blocked, and only from IMPLEMENTING status.
// Per blocking protocol: requires reason and 1-3 clarifying questions.
func MarkBlockedCommand(projectRoot, taskID, reason string, questions []string, agentID string) error {
	// Input validation
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}
	if reason == "" {
		return fmt.Errorf("reason is required")
	}
	if agentID == "" {
		return fmt.Errorf("agent ID is required")
	}
	if len(questions) == 0 {
		return fmt.Errorf("at least 1 question is required")
	}
	if len(questions) > 3 {
		return fmt.Errorf("maximum 3 questions allowed per blocking protocol")
	}

	// Setup paths
	lp := paths.New(projectRoot)

	// Get database instance
	bb := db.New(lp.StatePath())

	// Atomic state update
	now := time.Now().UTC()

	err := bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return fmt.Errorf("task not found: %s", taskID)
		}

		// Validate task status
		if task.Status != models.TaskStatusImplementing {
			return fmt.Errorf("task must be in IMPLEMENTING status to be marked blocked, current status: %s", task.Status)
		}

		// Validate agent is assigned to task
		if task.AssignedTo == nil || *task.AssignedTo != agentID {
			return fmt.Errorf("only the assigned agent can mark task as blocked")
		}

		// Update task state
		if err := task.Transition(models.TaskStatusBlocked); err != nil {
			return err
		}
		task.BlockedReason = &reason
		task.BlockedQuestions = questions
		task.AssignedTo = nil   // Clear assignment
		task.LeaseExpires = nil // Clear lease

		// Add history entry
		historyEntry := models.TaskHistoryEntry{
			Time:   now,
			Event:  "blocked",
			Agent:  &agentID,
			Reason: &reason,
		}
		task.History = append(task.History, historyEntry)

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to mark task as blocked: %w", err)
	}

	// Success output
	fmt.Fprintf(os.Stdout, "Task %s marked as BLOCKED\nReason: %s\n", taskID, reason)

	return nil
}
