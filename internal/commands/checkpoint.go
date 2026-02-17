package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// CheckpointCommand creates a checkpoint for the sprint
// This pauses agents and generates a sprint summary report
func CheckpointCommand(projectRoot string) error {
	// Setup paths
	lizaPaths := paths.New(projectRoot)
	statePath := lizaPaths.StatePath()
	reportPath := lizaPaths.SprintSummaryPath()

	// Create blackboard
	blackboard := db.New(statePath)

	// Read current state
	state, err := blackboard.Read()
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	// Validate sprint status
	if state.Sprint.Status == models.SprintStatusCheckpoint {
		return fmt.Errorf("sprint is already at CHECKPOINT")
	}

	if state.Sprint.Status == models.SprintStatusCompleted {
		return fmt.Errorf("cannot checkpoint: sprint is COMPLETED")
	}

	if state.Sprint.Status == models.SprintStatusAborted {
		return fmt.Errorf("cannot checkpoint: sprint is ABORTED")
	}

	// Gather metrics
	timestamp := time.Now()
	report := generateSprintSummary(state, timestamp)

	// Write report
	if err := os.WriteFile(reportPath, []byte(report), 0644); err != nil {
		return fmt.Errorf("failed to write sprint summary: %w", err)
	}

	// Update sprint status atomically
	err = blackboard.Modify(func(s *models.State) error {
		s.Sprint.Status = models.SprintStatusCheckpoint
		s.Sprint.Timeline.CheckpointAt = &timestamp
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to update sprint status: %w", err)
	}

	// Output success message
	fmt.Println("Sprint checkpoint created")
	fmt.Printf("  Status: %s → %s\n", models.SprintStatusInProgress, models.SprintStatusCheckpoint)
	fmt.Printf("  Checkpoint at: %s\n", timestamp.Format(time.RFC3339))
	fmt.Println()
	fmt.Printf("Sprint summary written to: %s\n", reportPath)
	fmt.Println()
	fmt.Println("Agents will pause at their next check.")
	fmt.Println("Review the sprint summary, then use 'liza resume' to continue.")

	return nil
}

// generateSprintSummary creates a markdown report of the sprint status
func generateSprintSummary(state *models.State, timestamp time.Time) string {
	var report string

	report += "# Sprint Summary\n\n"
	report += fmt.Sprintf("**Generated:** %s\n\n", timestamp.Format(time.RFC3339))

	// Sprint Status
	report += "## Sprint Status\n\n"
	report += fmt.Sprintf("- **Sprint ID:** %s\n", state.Sprint.ID)
	report += fmt.Sprintf("- **Status:** %s → %s\n", state.Sprint.Status, models.SprintStatusCheckpoint)
	report += fmt.Sprintf("- **Started:** %s\n", state.Sprint.Timeline.Started.Format(time.RFC3339))
	report += fmt.Sprintf("- **Deadline:** %s\n", state.Sprint.Timeline.Deadline.Format(time.RFC3339))

	// Calculate elapsed time
	elapsed := timestamp.Sub(state.Sprint.Timeline.Started)
	report += fmt.Sprintf("- **Elapsed:** %s\n", formatCheckpointDuration(elapsed))

	// Calculate time until deadline
	remaining := state.Sprint.Timeline.Deadline.Sub(timestamp)
	if remaining > 0 {
		report += fmt.Sprintf("- **Remaining:** %s\n", formatCheckpointDuration(remaining))
	} else {
		report += fmt.Sprintf("- **Overdue by:** %s\n", formatCheckpointDuration(-remaining))
	}

	report += "\n"

	// Task Status
	report += "## Task Status\n\n"

	tasksByStatus := make(map[models.TaskStatus][]models.Task)
	for _, task := range state.Tasks {
		tasksByStatus[task.Status] = append(tasksByStatus[task.Status], task)
	}

	report += "| Status | Count |\n"
	report += "|--------|-------|\n"
	report += fmt.Sprintf("| MERGED | %d |\n", len(tasksByStatus[models.TaskStatusMerged]))
	report += fmt.Sprintf("| APPROVED | %d |\n", len(tasksByStatus[models.TaskStatusApproved]))
	report += fmt.Sprintf("| READY_FOR_REVIEW | %d |\n", len(tasksByStatus[models.TaskStatusReadyForReview]))
	report += fmt.Sprintf("| IMPLEMENTING | %d |\n", len(tasksByStatus[models.TaskStatusImplementing]))
	report += fmt.Sprintf("| REJECTED | %d |\n", len(tasksByStatus[models.TaskStatusRejected]))
	report += fmt.Sprintf("| BLOCKED | %d |\n", len(tasksByStatus[models.TaskStatusBlocked]))
	report += fmt.Sprintf("| READY | %d |\n", len(tasksByStatus[models.TaskStatusReady]))
	report += fmt.Sprintf("| ABANDONED | %d |\n", len(tasksByStatus[models.TaskStatusAbandoned]))
	report += fmt.Sprintf("| SUPERSEDED | %d |\n", len(tasksByStatus[models.TaskStatusSuperseded]))
	report += fmt.Sprintf("| INTEGRATION_FAILED | %d |\n", len(tasksByStatus[models.TaskStatusIntegrationFailed]))

	report += "\n"

	// Sprint Metrics
	report += "## Sprint Metrics\n\n"
	report += fmt.Sprintf("- **Tasks Done:** %d\n", state.Sprint.Metrics.TasksDone)
	report += fmt.Sprintf("- **Tasks In Progress:** %d\n", state.Sprint.Metrics.TasksInProgress)
	report += fmt.Sprintf("- **Tasks Blocked:** %d\n", state.Sprint.Metrics.TasksBlocked)
	report += fmt.Sprintf("- **Total Iterations:** %d\n", state.Sprint.Metrics.IterationsTotal)
	report += fmt.Sprintf("- **Total Review Cycles:** %d\n", state.Sprint.Metrics.ReviewCyclesTotal)
	report += fmt.Sprintf("- **Review Verdict Approvals:** %d\n", state.Sprint.Metrics.ReviewVerdictApprovals)
	report += fmt.Sprintf("- **Review Verdict Rejections:** %d\n", state.Sprint.Metrics.ReviewVerdictRejections)
	report += fmt.Sprintf("- **Review Verdict Approval Rate:** %d%%\n", state.Sprint.Metrics.ReviewVerdictApprovalRatePercent)
	report += fmt.Sprintf("- **Task Outcome Approval Rate:** %d%%\n", state.Sprint.Metrics.TaskOutcomeApprovalRatePercent)

	report += "\n"

	// Active Agents
	report += "## Active Agents\n\n"
	if len(state.Agents) == 0 {
		report += "*No active agents*\n\n"
	} else {
		report += "| Agent ID | Role | Status | Current Task |\n"
		report += "|----------|------|--------|-------------|\n"
		for agentID, agent := range state.Agents {
			currentTask := "—"
			if agent.CurrentTask != nil {
				currentTask = *agent.CurrentTask
			}
			report += fmt.Sprintf("| %s | %s | %s | %s |\n", agentID, agent.Role, agent.Status, currentTask)
		}
		report += "\n"
	}

	// Anomalies
	if len(state.Anomalies) > 0 {
		report += "## Anomalies\n\n"
		report += fmt.Sprintf("**Total:** %d anomalies detected\n\n", len(state.Anomalies))

		anomaliesByType := make(map[string]int)
		for _, anomaly := range state.Anomalies {
			anomaliesByType[anomaly.Type]++
		}

		report += "| Type | Count |\n"
		report += "|------|-------|\n"
		for anomalyType, count := range anomaliesByType {
			report += fmt.Sprintf("| %s | %d |\n", anomalyType, count)
		}
		report += "\n"
	}

	// Circuit Breaker Status
	report += "## Circuit Breaker\n\n"
	report += fmt.Sprintf("- **Status:** %s\n", state.CircuitBreaker.Status)
	if state.CircuitBreaker.Status == "TRIGGERED" && state.CircuitBreaker.CurrentTrigger != nil {
		report += fmt.Sprintf("- **Pattern:** %s\n", state.CircuitBreaker.CurrentTrigger.Pattern)
		report += fmt.Sprintf("- **Severity:** %s\n", state.CircuitBreaker.CurrentTrigger.Severity)
	}
	report += "\n"

	// Next Steps
	report += "## Next Steps\n\n"
	report += "- [ ] Review sprint progress and metrics\n"
	report += "- [ ] Address any blocked tasks\n"
	report += "- [ ] Review anomalies and circuit breaker status\n"
	report += "- [ ] Adjust sprint scope if needed\n"
	report += "- [ ] Resume sprint with `liza resume`\n"
	report += "\n"

	return report
}

// formatCheckpointDuration formats a duration in a human-readable format
func formatCheckpointDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours >= 24 {
		days := hours / 24
		hours = hours % 24
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}

	return fmt.Sprintf("%dm", minutes)
}
