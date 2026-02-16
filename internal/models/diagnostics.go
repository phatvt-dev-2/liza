package models

import (
	"fmt"
	"strings"
	"time"
)

// CountClaimableTasks counts tasks that coders can claim.
// A task is claimable if:
//   - Status is UNCLAIMED, REJECTED, or INTEGRATION_FAILED
//   - All dependencies (depends_on) are MERGED
func CountClaimableTasks(state *State) int {
	// Build set of merged task IDs
	mergedIDs := make(map[string]bool)
	for _, task := range state.Tasks {
		if task.Status == TaskStatusMerged {
			mergedIDs[task.ID] = true
		}
	}

	count := 0
	for _, task := range state.Tasks {
		if task.Status != TaskStatusUnclaimed &&
			task.Status != TaskStatusRejected &&
			task.Status != TaskStatusIntegrationFailed {
			continue
		}

		allDepsSatisfied := true
		for _, depID := range task.DependsOn {
			if !mergedIDs[depID] {
				allDepsSatisfied = false
				break
			}
		}

		if allDepsSatisfied {
			count++
		}
	}

	return count
}

// CountReviewableTasks counts tasks that reviewers can review.
// A task is reviewable if:
//   - Status is READY_FOR_REVIEW
//   - AND one of:
//   - reviewing_by is null (no one assigned)
//   - reviewing_by is set AND review_lease_expires is set AND expired
//
// Note: reviewing_by set with nil review_lease_expires is malformed and NOT reviewable
func CountReviewableTasks(state *State) int {
	now := time.Now().UTC()
	count := 0

	for _, task := range state.Tasks {
		if task.Status != TaskStatusReadyForReview {
			continue
		}

		if task.ReviewingBy == nil {
			count++
			continue
		}

		if task.ReviewLeaseExpires != nil && task.ReviewLeaseExpires.Before(now) {
			count++
		}
	}

	return count
}

// GetCoderWorkDiagnostics returns detailed diagnostic information about task availability for coders.
func GetCoderWorkDiagnostics(state *State) string {
	claimable := CountClaimableTasks(state)

	if claimable > 0 {
		return fmt.Sprintf("Found %d claimable task(s)", claimable)
	}

	blockedByDeps := 0
	inProgress := 0

	mergedIDs := make(map[string]bool)
	for _, task := range state.Tasks {
		if task.Status == TaskStatusMerged {
			mergedIDs[task.ID] = true
		}
	}

	for _, task := range state.Tasks {
		if task.Status == TaskStatusUnclaimed ||
			task.Status == TaskStatusRejected ||
			task.Status == TaskStatusIntegrationFailed {
			hasUnsatisfiedDeps := false
			for _, depID := range task.DependsOn {
				if !mergedIDs[depID] {
					hasUnsatisfiedDeps = true
					break
				}
			}
			if hasUnsatisfiedDeps {
				blockedByDeps++
			}
		}

		if task.Status == TaskStatusClaimed ||
			task.Status == TaskStatusReadyForReview ||
			task.Status == TaskStatusApproved {
			inProgress++
		}
	}

	parts := []string{"No claimable tasks"}
	if blockedByDeps > 0 {
		parts = append(parts, fmt.Sprintf("%d blocked by dependencies", blockedByDeps))
	}
	if inProgress > 0 {
		parts = append(parts, fmt.Sprintf("%d in progress", inProgress))
	}

	return strings.Join(parts, "; ")
}

// GetReviewerWorkDiagnostics returns detailed diagnostic information about review availability.
func GetReviewerWorkDiagnostics(state *State) string {
	now := time.Now().UTC()

	unassigned := 0
	expiredLeases := 0
	activelyReviewing := 0

	for _, task := range state.Tasks {
		if task.Status == TaskStatusReadyForReview {
			if task.ReviewingBy == nil {
				unassigned++
			} else if task.ReviewLeaseExpires != nil && task.ReviewLeaseExpires.Before(now) {
				expiredLeases++
			} else if task.ReviewLeaseExpires != nil {
				activelyReviewing++
			}
		}
	}

	reviewable := unassigned + expiredLeases
	if reviewable > 0 {
		parts := []string{fmt.Sprintf("Found %d reviewable task(s)", reviewable)}
		details := []string{}
		if unassigned > 0 {
			details = append(details, fmt.Sprintf("%d unassigned", unassigned))
		}
		if expiredLeases > 0 {
			details = append(details, fmt.Sprintf("%d with expired leases", expiredLeases))
		}
		if len(details) > 0 {
			parts = append(parts, strings.Join(details, ", "))
		}
		return strings.Join(parts, ": ")
	}

	if activelyReviewing > 0 {
		return fmt.Sprintf("No reviewable tasks; %d actively being reviewed", activelyReviewing)
	}

	return "No reviewable tasks"
}
