package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// SubmitVerdictCommand atomically submits a review verdict (APPROVED or REJECTED).
// Used by reviewer agents to approve or reject work.
func SubmitVerdictCommand(projectRoot, taskID, verdict, reason, agentID string) error {
	// Validate input
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}
	if verdict == "" {
		return fmt.Errorf("verdict is required")
	}
	if agentID == "" {
		return fmt.Errorf("LIZA_AGENT_ID is required")
	}

	// Validate verdict
	verdict = strings.ToUpper(verdict)
	if verdict != "APPROVED" && verdict != "REJECTED" {
		return fmt.Errorf("verdict must be APPROVED or REJECTED, got: %s", verdict)
	}

	// Validate rejection reason
	if verdict == "REJECTED" && reason == "" {
		return fmt.Errorf("rejection reason is required for REJECTED verdict")
	}

	// Setup paths
	lp := paths.New(projectRoot)

	// Get database instance
	bb := db.New(lp.StatePath())

	// Atomic update
	now := time.Now().UTC()

	err := bb.Modify(func(state *models.State) error {
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

		// Validate task is in READY_FOR_REVIEW status
		if task.Status != models.TaskStatusReadyForReview {
			return fmt.Errorf("task %s is not READY_FOR_REVIEW (current status: %s)", taskID, task.Status)
		}

		// Update based on verdict
		if verdict == "APPROVED" {
			task.Status = models.TaskStatusApproved
			task.ApprovedBy = &agentID
			task.RejectionReason = nil

			// Add history entry for approval
			agentPtr := &agentID
			historyEntry := models.TaskHistoryEntry{
				Time:  now,
				Event: "approved",
				Agent: agentPtr,
			}
			task.History = append(task.History, historyEntry)
		} else {
			// REJECTED
			task.Status = models.TaskStatusRejected
			task.RejectionReason = &reason

			// Increment review cycles
			task.ReviewCyclesCurrent++
			task.ReviewCyclesTotal++

			// Add history entry for rejection (including reason for tracking)
			agentPtr := &agentID
			reasonPtr := &reason
			historyEntry := models.TaskHistoryEntry{
				Time:   now,
				Event:  "rejected",
				Agent:  agentPtr,
				Reason: reasonPtr,
			}
			task.History = append(task.History, historyEntry)
		}

		// Clear review claims (both APPROVED and REJECTED)
		task.ReviewingBy = nil
		task.ReviewLeaseExpires = nil

		// Update reviewer agent status: no longer actively reviewing
		if agent, ok := state.Agents[agentID]; ok {
			agent.Status = models.AgentStatusIdle
			agent.CurrentTask = nil
			agent.LeaseExpires = nil
			state.Agents[agentID] = agent
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to submit verdict: %w", err)
	}

	// Success output
	if verdict == "APPROVED" {
		fmt.Printf("APPROVED: %s\n", taskID)
		fmt.Printf("  approved_by: %s\n", agentID)
	} else {
		fmt.Printf("REJECTED: %s\n", taskID)
		fmt.Printf("  rejection_reason: %s\n", reason)
		fmt.Printf("  reviewed_by: %s\n", agentID)
	}

	return nil
}
