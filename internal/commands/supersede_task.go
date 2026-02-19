package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// SupersedeTaskCommand marks a task as SUPERSEDED by one or more replacement tasks.
// This is used by the planner when rescoping blocked, rejected, or problematic tasks.
//
// Requirements:
// - Task must be in BLOCKED, REJECTED, or READY status
// - At least one replacement task ID must be provided
// - Rescope reason must be non-empty
//
// The command follows the three-phase pattern:
// 1. Read and validate (no lock)
// 2. Atomic state update (via bb.Modify with lock)
// 3. Logging (after successful commit)
func SupersedeTaskCommand(projectRoot, taskID string, replacementIDs []string, reason, agentID string) error {
	// Validate inputs
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}
	if len(replacementIDs) == 0 {
		return fmt.Errorf("at least one replacement task ID is required")
	}
	if reason == "" {
		return fmt.Errorf("rescope reason is required")
	}
	if agentID == "" {
		agentID = "planner-1" // Default to planner-1 if not specified
	}

	// Setup paths
	lp := paths.New(projectRoot)

	// Get database instance
	bb := db.New(lp.StatePath())

	// Phase 1: Read and Validate (no lock held)
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

	// Validate source status - only allow superseding from certain statuses
	originalStatus := task.Status
	if originalStatus != models.TaskStatusBlocked &&
		originalStatus != models.TaskStatusRejected &&
		originalStatus != models.TaskStatusReady {
		return fmt.Errorf("cannot supersede task %s in status %s (must be BLOCKED, REJECTED, or READY)", taskID, originalStatus)
	}

	// Phase 2: Atomic State Update (via bb.Modify with lock)
	err = bb.Modify(func(state *models.State) error {
		// Re-find task in state
		taskIndex := -1
		for i := range state.Tasks {
			if state.Tasks[i].ID == taskID {
				taskIndex = i
				break
			}
		}

		if taskIndex == -1 {
			return fmt.Errorf("task not found: %s", taskID)
		}

		currentTask := &state.Tasks[taskIndex]

		// Re-validate status hasn't changed (TOCTOU protection)
		if currentTask.Status != originalStatus {
			return fmt.Errorf("cannot supersede task %s: status changed from %s to %s", taskID, originalStatus, currentTask.Status)
		}

		// Update task fields
		if err := currentTask.Transition(models.TaskStatusSuperseded); err != nil {
			return err
		}
		currentTask.SupersededBy = replacementIDs
		currentTask.RescopeReason = &reason

		// Clear lease/claim fields
		currentTask.AssignedTo = nil
		currentTask.LeaseExpires = nil
		currentTask.ReviewingBy = nil
		currentTask.ReviewLeaseExpires = nil

		// Add history entry
		now := time.Now().UTC()
		note := fmt.Sprintf("replaced by: %s", strings.Join(replacementIDs, ", "))
		historyEntry := models.TaskHistoryEntry{
			Time:   now,
			Event:  "superseded",
			Agent:  &agentID,
			Reason: &reason,
			Note:   &note,
		}
		currentTask.History = append(currentTask.History, historyEntry)

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to supersede task: %w", err)
	}

	// Phase 3: Logging (after successful commit)
	fmt.Printf("Superseded task %s (was %s) with replacements: %s\n", taskID, originalStatus, strings.Join(replacementIDs, ", "))

	return nil
}
