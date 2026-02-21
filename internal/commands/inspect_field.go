package commands

import (
	"strings"

	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
)

// GetField accesses direct state fields using dot notation
// Examples: "config.mode", "sprint.status", "sprint.metrics.tasks_done"
func getField(state *models.State, fieldPath string) (any, error) {
	parts := strings.Split(fieldPath, ".")
	if len(parts) == 0 {
		return nil, &errors.NotFoundError{Entity: "field", Field: fieldPath}
	}

	entity := parts[0]

	switch entity {
	case "config":
		return getConfigField(state, parts[1:])
	case "sprint":
		return getSprintField(state, parts[1:])
	case "version":
		if len(parts) == 1 {
			return state.Version, nil
		}
		return nil, &errors.NotFoundError{Entity: "state", Field: fieldPath}
	default:
		return nil, &errors.NotFoundError{Entity: entity, Field: ""}
	}
}

// getConfigField accesses config fields
func getConfigField(state *models.State, parts []string) (any, error) {
	if len(parts) == 0 {
		return nil, &errors.NotFoundError{Entity: "config", Field: ""}
	}

	field := parts[0]
	config := state.Config

	switch field {
	case "mode":
		return string(config.Mode), nil
	case "max_coder_iterations":
		return config.MaxCoderIterations, nil
	case "max_review_cycles":
		return config.MaxReviewCycles, nil
	case "heartbeat_interval":
		return config.HeartbeatInterval, nil
	case "lease_duration":
		return config.LeaseDuration, nil
	case "coder_poll_interval":
		return config.CoderPollInterval, nil
	case "coder_max_wait":
		return config.CoderMaxWait, nil
	case "integration_branch":
		return config.IntegrationBranch, nil
	case "escalation_webhook":
		if config.EscalationWebhook != nil {
			return *config.EscalationWebhook, nil
		}
		return nil, nil
	default:
		return nil, &errors.NotFoundError{Entity: "config", Field: field}
	}
}

// getSprintField accesses sprint fields
func getSprintField(state *models.State, parts []string) (any, error) {
	if len(parts) == 0 {
		return nil, &errors.NotFoundError{Entity: "sprint", Field: ""}
	}

	field := parts[0]
	sprint := state.Sprint

	switch field {
	case "id":
		return sprint.ID, nil
	case "status":
		return string(sprint.Status), nil
	case "goal_ref":
		return sprint.GoalRef, nil
	case "metrics":
		if len(parts) > 1 {
			return getSprintMetricsField(state, parts[1:])
		}
		return sprint.Metrics, nil
	case "timeline":
		if len(parts) > 1 {
			return getSprintTimelineField(state, parts[1:])
		}
		return nil, &errors.NotFoundError{Entity: "sprint.timeline", Field: ""}
	default:
		return nil, &errors.NotFoundError{Entity: "sprint", Field: field}
	}
}

// getSprintMetricsField accesses sprint metrics fields
func getSprintMetricsField(state *models.State, parts []string) (any, error) {
	if len(parts) == 0 {
		return nil, &errors.NotFoundError{Entity: "sprint.metrics", Field: ""}
	}

	field := parts[0]
	metrics := state.Sprint.Metrics

	switch field {
	case "tasks_done":
		return metrics.TasksDone, nil
	case "tasks_in_progress":
		return metrics.TasksInProgress, nil
	case "tasks_blocked":
		return metrics.TasksBlocked, nil
	case "iterations_total":
		return metrics.IterationsTotal, nil
	case "review_cycles_total":
		return metrics.ReviewCyclesTotal, nil
	case "review_verdict_approvals":
		return metrics.ReviewVerdictApprovals, nil
	case "review_verdict_rejections":
		return metrics.ReviewVerdictRejections, nil
	case "review_verdict_count":
		return metrics.ReviewVerdictCount, nil
	case "review_verdict_approval_rate_percent":
		return metrics.ReviewVerdictApprovalRatePercent, nil
	case "task_submitted_for_review_count":
		return metrics.TaskSubmittedForReviewCount, nil
	case "task_outcome_approval_rate_percent":
		return metrics.TaskOutcomeApprovalRatePercent, nil
	default:
		return nil, &errors.NotFoundError{Entity: "sprint.metrics", Field: field}
	}
}

// getSprintTimelineField accesses sprint timeline fields
func getSprintTimelineField(state *models.State, parts []string) (any, error) {
	if len(parts) == 0 {
		return nil, &errors.NotFoundError{Entity: "sprint.timeline", Field: ""}
	}

	field := parts[0]
	timeline := state.Sprint.Timeline

	switch field {
	case "started":
		return timeline.Started, nil
	case "deadline":
		return timeline.Deadline, nil
	case "checkpoint_at":
		if timeline.CheckpointAt != nil {
			return *timeline.CheckpointAt, nil
		}
		return nil, nil
	case "ended":
		if timeline.Ended != nil {
			return *timeline.Ended, nil
		}
		return nil, nil
	default:
		return nil, &errors.NotFoundError{Entity: "sprint.timeline", Field: field}
	}
}

// getComputedField calculates derived data from state
// Supports computed fields like agents.active_count, sprint.elapsed, etc.
func getComputedField(state *models.State, fieldPath string) (any, error) {
	parts := strings.Split(fieldPath, ".")
	if len(parts) < 2 {
		return nil, &errors.NotFoundError{Entity: "computed", Field: fieldPath}
	}

	entity := parts[0]

	switch entity {
	case "agents":
		return getAgentsComputedField(state, parts[1])
	case "tasks":
		return getTasksComputedField(state, parts[1])
	case "sprint":
		return getSprintComputedField(state, parts[1])
	case "agent":
		if len(parts) < 3 {
			return nil, &errors.NotFoundError{Entity: "agent", Field: "id required"}
		}
		agentID := parts[1]
		field := parts[2]
		return getAgentComputedField(state, agentID, field)
	case "task":
		if len(parts) < 3 {
			return nil, &errors.NotFoundError{Entity: "task", Field: "id required"}
		}
		taskID := parts[1]
		field := parts[2]
		return getTaskComputedField(state, taskID, field)
	default:
		return nil, &errors.NotFoundError{Entity: entity, Field: fieldPath}
	}
}

// getAgentsComputedField calculates aggregate agent metrics
func getAgentsComputedField(state *models.State, field string) (any, error) {
	switch field {
	case "active_count":
		count := 0
		for _, agent := range state.Agents {
			if agent.Status != models.AgentStatusIdle {
				count++
			}
		}
		return count, nil
	case "utilization":
		if len(state.Agents) == 0 {
			return 0.0, nil
		}
		active := 0
		for _, agent := range state.Agents {
			if agent.Status == models.AgentStatusWorking || agent.Status == models.AgentStatusReviewing {
				active++
			}
		}
		return float64(active) / float64(len(state.Agents)) * 100, nil
	default:
		return nil, &errors.NotFoundError{Entity: "agents", Field: field}
	}
}

// getTasksComputedField calculates aggregate task metrics
func getTasksComputedField(state *models.State, field string) (any, error) {
	switch field {
	case "completion_rate":
		if len(state.Tasks) == 0 {
			return 0.0, nil
		}
		done := 0
		for _, task := range state.Tasks {
			if task.Status == models.TaskStatusMerged {
				done++
			}
		}
		return float64(done) / float64(len(state.Tasks)) * 100, nil
	case "avg_iteration_count":
		if len(state.Tasks) == 0 {
			return 0.0, nil
		}
		total := 0
		for _, task := range state.Tasks {
			total += task.Iteration
		}
		return float64(total) / float64(len(state.Tasks)), nil
	default:
		return nil, &errors.NotFoundError{Entity: "tasks", Field: field}
	}
}

// getSprintComputedField calculates sprint-related computed values
func getSprintComputedField(state *models.State, field string) (any, error) {
	switch field {
	case "elapsed":
		duration := calculateSprintElapsed(&state.Sprint)
		return formatDuration(duration), nil
	case "remaining":
		duration := calculateSprintRemaining(&state.Sprint)
		return formatDuration(duration), nil
	case "progress_percent":
		if len(state.Tasks) == 0 {
			return 0.0, nil
		}
		done := 0
		for _, task := range state.Tasks {
			if task.Status == models.TaskStatusMerged {
				done++
			}
		}
		return float64(done) / float64(len(state.Tasks)) * 100, nil
	default:
		return nil, &errors.NotFoundError{Entity: "sprint", Field: field}
	}
}

// getAgentComputedField calculates agent-specific computed values
func getAgentComputedField(state *models.State, agentID, field string) (any, error) {
	agent, ok := state.Agents[agentID]
	if !ok {
		return nil, &errors.NotFoundError{Entity: "agent", ID: agentID}
	}

	switch field {
	case "time_since_heartbeat":
		duration := calculateTimeSinceHeartbeat(&agent)
		return formatDuration(duration), nil
	case "time_on_task":
		// Find the task assigned to this agent
		for _, task := range state.Tasks {
			if task.AssignedTo != nil && *task.AssignedTo == agentID {
				duration := calculateTimeOnTask(&task)
				return formatDuration(duration), nil
			}
		}
		return "0s", nil
	default:
		return nil, &errors.NotFoundError{Entity: "agent", ID: agentID, Field: field}
	}
}

// getTaskComputedField calculates task-specific computed values
func getTaskComputedField(state *models.State, taskID, field string) (any, error) {
	task := state.FindTask(taskID)
	if task == nil {
		return nil, &errors.NotFoundError{Entity: "task", ID: taskID}
	}

	switch field {
	case "age":
		duration := calculateTaskAge(task)
		return formatDuration(duration), nil
	case "time_in_status":
		// Find the most recent status change in history
		duration := calculateTimeOnTask(task)
		return formatDuration(duration), nil
	default:
		return nil, &errors.NotFoundError{Entity: "task", ID: taskID, Field: field}
	}
}
