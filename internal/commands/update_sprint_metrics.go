package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// UpdateSprintMetricsCommand recomputes sprint metrics from current task and agent state
func UpdateSprintMetricsCommand(projectRoot string) error {
	// Get state path
	statePath := paths.New(projectRoot).StatePath()

	// Create blackboard
	blackboard := db.New(statePath)

	// Read current state
	state, err := blackboard.Read()
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	// Compute metrics from state
	metrics := computeSprintMetrics(state)

	// Update state atomically
	err = blackboard.Modify(func(s *models.State) error {
		s.Sprint.Metrics = metrics
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to update sprint metrics: %w", err)
	}

	// Output summary
	fmt.Println("Sprint metrics updated:")
	fmt.Printf("  Tasks done: %d\n", metrics.TasksDone)
	fmt.Printf("  Tasks in progress: %d\n", metrics.TasksInProgress)
	fmt.Printf("  Tasks blocked: %d\n", metrics.TasksBlocked)
	fmt.Printf("  Total iterations: %d\n", metrics.IterationsTotal)
	fmt.Printf("  Total review cycles: %d\n", metrics.ReviewCyclesTotal)
	fmt.Printf("  Review verdicts: %d approvals, %d rejections (%d%%)\n",
		metrics.ReviewVerdictApprovals,
		metrics.ReviewVerdictRejections,
		metrics.ReviewVerdictApprovalRatePercent)
	fmt.Printf("  Task outcomes: %d submitted, %d%% approval rate\n",
		metrics.TaskSubmittedForReviewCount,
		metrics.TaskOutcomeApprovalRatePercent)

	// Check for suspicious approval rates (>95%)
	warnings := checkSuspiciousRates(metrics)
	if len(warnings) > 0 {
		fmt.Println()
		for _, warning := range warnings {
			fmt.Println(warning)
		}
	}

	return nil
}

// computeSprintMetrics calculates all sprint metrics from current state
func computeSprintMetrics(state *models.State) models.SprintMetrics {
	metrics := models.SprintMetrics{}

	// Count tasks by status
	for _, task := range state.Tasks {
		// Tasks done: terminal states (MERGED, ABANDONED, SUPERSEDED)
		if task.Status.IsTerminal() {
			metrics.TasksDone++
		}

		// Tasks in progress: CLAIMED, READY_FOR_REVIEW, REJECTED, INTEGRATION_FAILED
		if task.Status == models.TaskStatusClaimed ||
			task.Status == models.TaskStatusReadyForReview ||
			task.Status == models.TaskStatusRejected ||
			task.Status == models.TaskStatusIntegrationFailed {
			metrics.TasksInProgress++
		}

		// Tasks blocked
		if task.Status == models.TaskStatusBlocked {
			metrics.TasksBlocked++
		}

		// Aggregate review cycles
		metrics.ReviewCyclesTotal += task.ReviewCyclesTotal

		// Count review verdicts and task submissions from history
		hasSubmitted := false
		for _, entry := range task.History {
			if entry.Event == "submitted_for_review" {
				hasSubmitted = true
			}
			if entry.Event == "review_verdict_approved" {
				metrics.ReviewVerdictApprovals++
				metrics.ReviewVerdictCount++
			}
			if entry.Event == "review_verdict_rejected" {
				metrics.ReviewVerdictRejections++
				metrics.ReviewVerdictCount++
			}
		}

		// Count tasks that have been submitted for review
		if hasSubmitted {
			metrics.TaskSubmittedForReviewCount++
		}
	}

	// Aggregate iterations from agents
	for _, agent := range state.Agents {
		metrics.IterationsTotal += agent.IterationsTotal
	}

	// Calculate review verdict approval rate
	if metrics.ReviewVerdictCount > 0 {
		metrics.ReviewVerdictApprovalRatePercent = (metrics.ReviewVerdictApprovals * 100) / metrics.ReviewVerdictCount
	}

	// Calculate task outcome approval rate
	// Tasks that ended up approved or merged out of tasks submitted for review
	if metrics.TaskSubmittedForReviewCount > 0 {
		approvedOrMerged := 0
		for _, task := range state.Tasks {
			hasSubmitted := false
			for _, entry := range task.History {
				if entry.Event == "submitted_for_review" {
					hasSubmitted = true
					break
				}
			}
			if hasSubmitted && (task.Status == models.TaskStatusApproved || task.Status == models.TaskStatusMerged) {
				approvedOrMerged++
			}
		}
		metrics.TaskOutcomeApprovalRatePercent = (approvedOrMerged * 100) / metrics.TaskSubmittedForReviewCount
	}

	return metrics
}

// checkSuspiciousRates returns warnings if approval rates are suspiciously high (>95%)
func checkSuspiciousRates(metrics models.SprintMetrics) []string {
	var warnings []string

	// Check review verdict approval rate
	if metrics.ReviewVerdictCount >= 3 && metrics.ReviewVerdictApprovalRatePercent > 95 {
		warnings = append(warnings, fmt.Sprintf(
			"⚠️  Review verdict approval rate is %d%% (%d/%d) - suspiciously high",
			metrics.ReviewVerdictApprovalRatePercent,
			metrics.ReviewVerdictApprovals,
			metrics.ReviewVerdictCount,
		))
	}

	// Check task outcome approval rate
	if metrics.TaskSubmittedForReviewCount >= 3 && metrics.TaskOutcomeApprovalRatePercent > 95 {
		// Calculate approved/merged count for the message
		approvedOrMergedCount := (metrics.TaskOutcomeApprovalRatePercent * metrics.TaskSubmittedForReviewCount) / 100
		warnings = append(warnings, fmt.Sprintf(
			"⚠️  Task outcome approval rate is %d%% (%d/%d) - suspiciously high",
			metrics.TaskOutcomeApprovalRatePercent,
			approvedOrMergedCount,
			metrics.TaskSubmittedForReviewCount,
		))
	}

	return warnings
}
