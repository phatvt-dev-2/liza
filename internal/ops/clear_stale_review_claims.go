package ops

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/log"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// ClearStaleReviewClaims finds and clears expired review leases on REVIEWING tasks.
// When a lease expires, the task reverts to READY_FOR_REVIEW.
// Returns the number of claims cleared.
// TODO: make pipeline-aware — currently hardcodes TaskStatusReviewing and
// TaskStatusReadyForReview, which won't match pipeline-defined reviewing states.
func ClearStaleReviewClaims(projectRoot string) (int, error) {
	// Setup paths
	lp := paths.New(projectRoot)

	// Get database and logger instances
	bb := db.For(lp.StatePath())
	logger := log.New(lp.LogPath())

	// Track cleared claims
	cleared := 0
	now := time.Now().UTC()

	// Atomic update
	err := bb.Modify(func(state *models.State) error {
		// Find REVIEWING tasks with expired review leases
		for i := range state.Tasks {
			task := &state.Tasks[i]

			// Skip if not REVIEWING
			if task.Status != models.TaskStatusReviewing {
				continue
			}

			// Skip if no reviewing_by
			if task.ReviewingBy == nil {
				continue
			}

			// Check if lease is expired
			// If lease is nil but reviewing_by is set, treat as malformed/expired
			isExpired := false
			var staleReviewer string
			var expiredAt string

			if task.ReviewLeaseExpires == nil {
				// Malformed state: reviewing_by set but no lease
				isExpired = true
				staleReviewer = *task.ReviewingBy
				expiredAt = "unknown (lease missing)"
			} else if task.ReviewLeaseExpires.Before(now) || task.ReviewLeaseExpires.Equal(now) {
				// Lease has expired
				isExpired = true
				staleReviewer = *task.ReviewingBy
				expiredAt = task.ReviewLeaseExpires.Format(time.RFC3339)
			}

			if !isExpired {
				continue
			}

			// Revert to READY_FOR_REVIEW and clear the stale claim
			if err := task.Transition(models.TaskStatusReadyForReview); err != nil {
				return err
			}
			task.ReviewingBy = nil
			task.ReviewLeaseExpires = nil

			// Log the cleanup
			detail := fmt.Sprintf("Review claim expired at %s (reviewer: %s)", expiredAt, staleReviewer)

			logEntry := log.Entry{
				Timestamp: now,
				Agent:     "system",
				Action:    "stale_review_cleared",
				Task:      &task.ID,
				Detail:    detail,
			}

			// Append log entry (this will acquire its own lock)
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
