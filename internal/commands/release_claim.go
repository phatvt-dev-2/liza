package commands

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// ReleaseClaimCommand manually releases claims on a task (reviewer, coder, or both).
// Used to release task claims manually when needed.
func ReleaseClaimCommand(projectRoot, taskID, role string, force bool, reason, agentID string) error {
	// Validate input
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}

	// Validate role
	if role != "reviewer" && role != "coder" && role != "both" {
		return fmt.Errorf("role must be reviewer, coder, or both, got: %s", role)
	}

	// Default agent ID if not provided
	if agentID == "" {
		agentID = "human"
	}

	// Default reason if not provided
	if reason == "" {
		reason = "manual release"
	}

	// Setup paths
	lp := paths.New(projectRoot)

	// Get database instance
	bb := db.New(lp.StatePath())

	// Track what we released
	releasedReviewer := false
	releasedCoder := false

	// Atomic update
	now := time.Now().UTC()

	err := bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return fmt.Errorf("task not found: %s", taskID)
		}

		// Release reviewer claim if requested
		if role == "reviewer" || role == "both" {
			hasReviewerClaim := task.ReviewingBy != nil || task.ReviewLeaseExpires != nil

			if !hasReviewerClaim {
				// No reviewer claim to release - this is just a warning, not an error
				// We'll track this and handle it after processing both roles
			} else {
				// Check if lease is missing but reviewer is assigned
				if task.ReviewingBy != nil && task.ReviewLeaseExpires == nil && !force {
					return fmt.Errorf("review lease expires missing for task %s, use --force to clear", taskID)
				}

				// Check if lease is still valid
				if task.ReviewLeaseExpires != nil && !force {
					if task.ReviewLeaseExpires.After(now) {
						return fmt.Errorf("review lease still valid until %s, use --force to clear", task.ReviewLeaseExpires.Format(time.RFC3339))
					}
				}

				// Transition REVIEWING back to READY_FOR_REVIEW
				if task.Status == models.TaskStatusReviewing {
					if err := task.Transition(models.TaskStatusReadyForReview); err != nil {
						return err
					}
				}

				// Release reviewer agent state
				if task.ReviewingBy != nil {
					state.ReleaseAgent(*task.ReviewingBy)
				}

				// Release reviewer claim
				task.ReviewingBy = nil
				task.ReviewLeaseExpires = nil

				// Add history entry
				agentPtr := &agentID
				reasonPtr := &reason
				historyEntry := models.TaskHistoryEntry{
					Time:   now,
					Event:  "review_claim_released",
					Agent:  agentPtr,
					Reason: reasonPtr,
				}
				task.History = append(task.History, historyEntry)
				releasedReviewer = true
			}
		}

		// Release coder claim if requested
		if role == "coder" || role == "both" {
			hasCoderClaim := task.AssignedTo != nil || task.LeaseExpires != nil

			if !hasCoderClaim {
				// No coder claim to release - this is just a warning, not an error
				// We'll track this and handle it after processing both roles
			} else {
				// Check if lease is missing but coder is assigned
				if task.AssignedTo != nil && task.LeaseExpires == nil && !force {
					return fmt.Errorf("lease expires missing for task %s, use --force to clear", taskID)
				}

				// Check if lease is still valid
				if task.LeaseExpires != nil && !force {
					if task.LeaseExpires.After(now) {
						return fmt.Errorf("coder lease still valid until %s, use --force to clear", task.LeaseExpires.Format(time.RFC3339))
					}
				}

				// Change status if CLAIMED
				if task.Status == models.TaskStatusImplementing {
					if err := task.Transition(models.TaskStatusReady); err != nil {
						return err
					}
				}

				// Release coder agent state
				if task.AssignedTo != nil {
					state.ReleaseAgent(*task.AssignedTo)
				}

				// Release coder claim
				task.AssignedTo = nil
				task.LeaseExpires = nil

				// Add history entry
				agentPtr := &agentID
				reasonPtr := &reason
				historyEntry := models.TaskHistoryEntry{
					Time:   now,
					Event:  "coder_claim_released",
					Agent:  agentPtr,
					Reason: reasonPtr,
				}
				task.History = append(task.History, historyEntry)
				releasedCoder = true
			}
		}

		// Check if any claims were actually released
		if !releasedReviewer && !releasedCoder {
			return fmt.Errorf("no claims to release for task %s", taskID)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to release claim: %w", err)
	}

	// Success output
	if releasedReviewer {
		fmt.Printf("Released review claim for %s\n", taskID)
	}
	if releasedCoder {
		fmt.Printf("Released coder claim for %s\n", taskID)
	}

	return nil
}
