package models

import (
	"fmt"
	"strings"
	"time"
)

// CountClaimableTasks counts tasks that coders can claim.
// A task is claimable if:
//   - Status is READY, REJECTED, or INTEGRATION_FAILED
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
		if task.Status != TaskStatusReady &&
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

// CountReviewableTasks counts tasks that are immediately claimable by a reviewer.
// Only READY_FOR_REVIEW tasks qualify — REVIEWING tasks with expired leases require
// ClearStaleReviewClaimsCommand to revert them to READY_FOR_REVIEW first.
func CountReviewableTasks(state *State) int {
	count := 0
	for _, task := range state.Tasks {
		if task.Status == TaskStatusReadyForReview {
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
		if task.Status == TaskStatusReady ||
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

		if task.Status == TaskStatusImplementing ||
			task.Status == TaskStatusReadyForReview ||
			task.Status == TaskStatusReviewing ||
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
			unassigned++
		}
		if task.Status == TaskStatusReviewing {
			if task.ReviewLeaseExpires != nil && task.ReviewLeaseExpires.Before(now) {
				expiredLeases++
			} else {
				activelyReviewing++
			}
		}
	}

	if unassigned > 0 {
		parts := []string{fmt.Sprintf("Found %d reviewable task(s)", unassigned)}
		if expiredLeases > 0 {
			parts = append(parts, fmt.Sprintf("%d with stale leases (pending reclamation)", expiredLeases))
		}
		return strings.Join(parts, "; ")
	}

	parts := []string{"No reviewable tasks"}
	if expiredLeases > 0 {
		parts = append(parts, fmt.Sprintf("%d with stale leases (pending reclamation)", expiredLeases))
	}
	if activelyReviewing > 0 {
		parts = append(parts, fmt.Sprintf("%d actively being reviewed", activelyReviewing))
	}

	return strings.Join(parts, "; ")
}
