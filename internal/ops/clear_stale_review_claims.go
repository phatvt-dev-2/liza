package ops

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/log"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// reviewMatch captures the detection result for a task in a reviewing state,
// including the revert target and whether pipeline transitions are needed.
type reviewMatch struct {
	revertStatus models.TaskStatus
	usePipeline  bool
}

// ClearStaleReviewClaims finds and clears expired review leases on reviewing tasks.
// For legacy tasks, checks TaskStatusReviewing / TaskStatusReviewingCodingPlan and
// reverts to their corresponding submitted states. For pipeline tasks, also checks
// pipeline-defined reviewing states and reverts to pipeline-defined submitted states.
// Returns the number of claims cleared.
func ClearStaleReviewClaims(projectRoot string) (int, error) {
	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	logger := log.New(lp.LogPath())

	// Load pipeline config once for both detection and transition.
	pb := loadPipelineBundle(projectRoot)

	cleared := 0
	now := time.Now().UTC()

	err := bb.Modify(func(state *models.State) error {
		for i := range state.Tasks {
			task := &state.Tasks[i]

			// Determine if this task is in a reviewing state.
			match, err := detectReviewingState(task, pb)
			if err != nil {
				return err
			}
			if match == nil {
				continue
			}

			// Skip if no reviewing_by
			if task.ReviewingBy == nil {
				continue
			}

			// Determine expiry. If lease is nil but reviewing_by is set,
			// treat as malformed/expired.
			var expiredAt string
			switch {
			case task.ReviewLeaseExpires == nil:
				expiredAt = "unknown (lease missing)"
			case !task.ReviewLeaseExpires.After(now):
				expiredAt = task.ReviewLeaseExpires.Format(time.RFC3339)
			default:
				continue
			}

			// Capture reviewer before clearing the claim.
			staleReviewer := *task.ReviewingBy

			// Revert to submitted state and clear the stale claim.
			if match.usePipeline {
				if err := task.TransitionWith(match.revertStatus, pb.transitions); err != nil {
					return err
				}
			} else {
				if err := task.Transition(match.revertStatus); err != nil {
					return err
				}
			}
			task.ReviewingBy = nil
			task.ReviewLeaseExpires = nil

			detail := fmt.Sprintf("Review claim expired at %s (reviewer: %s)", expiredAt, staleReviewer)
			logEntry := log.Entry{
				Timestamp: now,
				Agent:     "system",
				Action:    "stale_review_cleared",
				Task:      &task.ID,
				Detail:    detail,
			}
			if err := logger.Append(logEntry); err != nil {
				return fmt.Errorf("failed to log stale review cleanup for %s: %w", task.ID, err)
			}

			cleared++
		}

		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to clear stale review claims: %w", err)
	}

	return cleared, nil
}

// detectReviewingState checks whether a task is in any reviewing state (legacy or pipeline).
// Returns (nil, nil) if the task is not in a reviewing state.
// Returns a non-nil error if the task IS in a pipeline reviewing state but the
// submitted status cannot be resolved — callers should surface this rather than
// silently skipping, as it would leave the task stuck.
func detectReviewingState(task *models.Task, pb *pipelineBundle) (*reviewMatch, error) {
	// Legacy reviewing states.
	switch task.Status {
	case models.TaskStatusReviewing:
		return &reviewMatch{revertStatus: models.TaskStatusReadyForReview, usePipeline: false}, nil
	case models.TaskStatusReviewingCodingPlan:
		return &reviewMatch{revertStatus: models.TaskStatusCodingPlanToReview, usePipeline: false}, nil
	}

	// Pipeline reviewing states.
	if task.RolePair != "" && pb != nil {
		reviewing, err := pb.pr.ReviewingStatus(task.RolePair)
		if err == nil && task.Status == reviewing {
			submitted, err := pb.pr.SubmittedStatus(task.RolePair)
			if err != nil {
				return nil, fmt.Errorf("task %s is in pipeline reviewing state %s but submitted status resolution failed for role-pair %q: %w",
					task.ID, task.Status, task.RolePair, err)
			}
			return &reviewMatch{revertStatus: submitted, usePipeline: true}, nil
		}
	}

	return nil, nil
}
