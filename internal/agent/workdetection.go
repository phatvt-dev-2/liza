package agent

import (
	"fmt"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/models"
)

// PlannerWakeTrigger represents what triggered the planner to wake
type PlannerWakeTrigger string

const (
	WakeTriggerInitialPlanning     PlannerWakeTrigger = "INITIAL_PLANNING"
	WakeTriggerBlocked             PlannerWakeTrigger = "BLOCKED_TASKS"
	WakeTriggerIntegrationFailed   PlannerWakeTrigger = "INTEGRATION_FAILED"
	WakeTriggerHypothesisExhausted PlannerWakeTrigger = "HYPOTHESIS_EXHAUSTED"
	WakeTriggerImmediateDiscovery  PlannerWakeTrigger = "IMMEDIATE_DISCOVERY"
	WakeTriggerNone                PlannerWakeTrigger = "NONE"
)

// PlannerWakeResult contains the wake trigger and count
type PlannerWakeResult struct {
	Trigger PlannerWakeTrigger
	Count   int
}

// CountClaimableTasks counts tasks that coders can claim
// A task is claimable if:
// - Status is UNCLAIMED, REJECTED, or INTEGRATION_FAILED
// - All dependencies (depends_on) are MERGED
func CountClaimableTasks(state *models.State) int {
	// Build set of merged task IDs
	mergedIDs := make(map[string]bool)
	for _, task := range state.Tasks {
		if task.Status == models.TaskStatusMerged {
			mergedIDs[task.ID] = true
		}
	}

	count := 0
	for _, task := range state.Tasks {
		// Check status
		if task.Status != models.TaskStatusUnclaimed &&
			task.Status != models.TaskStatusRejected &&
			task.Status != models.TaskStatusIntegrationFailed {
			continue
		}

		// Check dependencies
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

// CountReviewableTasks counts tasks that reviewers can review
// A task is reviewable if:
// - Status is READY_FOR_REVIEW
// - AND one of:
//   - reviewing_by is null (no one assigned)
//   - reviewing_by is set AND review_lease_expires is set AND expired
//
// Note: reviewing_by set with nil review_lease_expires is malformed and NOT reviewable
func CountReviewableTasks(state *models.State) int {
	now := time.Now().UTC()
	count := 0

	for _, task := range state.Tasks {
		if task.Status != models.TaskStatusReadyForReview {
			continue
		}

		// Case 1: No reviewer assigned
		if task.ReviewingBy == nil {
			count++
			continue
		}

		// Case 2: Reviewer assigned with expired lease
		// (malformed state with reviewing_by but no lease is NOT reviewable)
		if task.ReviewLeaseExpires != nil && task.ReviewLeaseExpires.Before(now) {
			count++
		}
	}

	return count
}

// DetectPlannerWakeTriggers detects conditions that should wake the planner
// Returns the highest-priority trigger and count of items for that trigger
// Priority order (per bash script):
// 1. No tasks (initial planning)
// 2. Blocked tasks
// 3. Integration failed
// 4. Hypothesis exhausted (2+ failed_by)
// 5. Immediate discoveries (not yet converted to tasks)
func DetectPlannerWakeTriggers(state *models.State) PlannerWakeResult {
	// Check for initial planning
	if len(state.Tasks) == 0 {
		return PlannerWakeResult{
			Trigger: WakeTriggerInitialPlanning,
			Count:   1,
		}
	}

	// Count blocked tasks
	blocked := 0
	for _, task := range state.Tasks {
		if task.Status == models.TaskStatusBlocked {
			blocked++
		}
	}
	if blocked > 0 {
		return PlannerWakeResult{
			Trigger: WakeTriggerBlocked,
			Count:   blocked,
		}
	}

	// Count integration failures
	integrationFailed := 0
	for _, task := range state.Tasks {
		if task.Status == models.TaskStatusIntegrationFailed {
			integrationFailed++
		}
	}
	if integrationFailed > 0 {
		return PlannerWakeResult{
			Trigger: WakeTriggerIntegrationFailed,
			Count:   integrationFailed,
		}
	}

	// Count hypothesis exhaustion (2+ failed coders on non-terminal tasks)
	hypothesisExhausted := 0
	for _, task := range state.Tasks {
		if len(task.FailedBy) >= 2 && !task.Status.IsTerminal() {
			hypothesisExhausted++
		}
	}
	if hypothesisExhausted > 0 {
		return PlannerWakeResult{
			Trigger: WakeTriggerHypothesisExhausted,
			Count:   hypothesisExhausted,
		}
	}

	// Count immediate discoveries not yet converted to tasks
	immediateDiscoveries := 0
	for _, disc := range state.Discovered {
		if disc.Urgency == "immediate" && disc.ConvertedToTask == nil {
			immediateDiscoveries++
		}
	}
	if immediateDiscoveries > 0 {
		return PlannerWakeResult{
			Trigger: WakeTriggerImmediateDiscovery,
			Count:   immediateDiscoveries,
		}
	}

	// No triggers
	return PlannerWakeResult{
		Trigger: WakeTriggerNone,
		Count:   0,
	}
}

// getCoderWorkDiagnostics returns detailed diagnostic information about task availability for coders
func getCoderWorkDiagnostics(state *models.State) string {
	claimable := CountClaimableTasks(state)

	// If there are claimable tasks, report that
	if claimable > 0 {
		return fmt.Sprintf("Found %d claimable task(s)", claimable)
	}

	// Otherwise, explain why there are no claimable tasks
	blockedByDeps := 0
	inProgress := 0

	// Build set of merged task IDs for dependency checking
	mergedIDs := make(map[string]bool)
	for _, task := range state.Tasks {
		if task.Status == models.TaskStatusMerged {
			mergedIDs[task.ID] = true
		}
	}

	// Count blocked and in-progress tasks
	for _, task := range state.Tasks {
		// Check if task is potentially claimable by status
		if task.Status == models.TaskStatusUnclaimed ||
			task.Status == models.TaskStatusRejected ||
			task.Status == models.TaskStatusIntegrationFailed {
			// Check if blocked by dependencies
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

		// Count in-progress tasks
		if task.Status == models.TaskStatusClaimed ||
			task.Status == models.TaskStatusReadyForReview ||
			task.Status == models.TaskStatusApproved {
			inProgress++
		}
	}

	// Build diagnostic message
	parts := []string{"No claimable tasks"}
	if blockedByDeps > 0 {
		parts = append(parts, fmt.Sprintf("%d blocked by dependencies", blockedByDeps))
	}
	if inProgress > 0 {
		parts = append(parts, fmt.Sprintf("%d in progress", inProgress))
	}

	return strings.Join(parts, "; ")
}

// getReviewerWorkDiagnostics returns detailed diagnostic information about review availability
func getReviewerWorkDiagnostics(state *models.State) string {
	now := time.Now().UTC()

	unassigned := 0
	expiredLeases := 0
	activelyReviewing := 0

	for _, task := range state.Tasks {
		if task.Status == models.TaskStatusReadyForReview {
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
