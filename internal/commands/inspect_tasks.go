package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
)

// inspectTasksOptions contains options for task inspection
type inspectTasksOptions struct {
	Format           string // Output format: json, yaml, table, value
	StatusFilter     string // Filter by status
	AssignedToFilter string // Filter by assignee
	BlockedFilter    bool   // Show only blocked tasks
	Internal         bool   // Return structured data for composition
}

// taskInfo represents task information with computed fields
type taskInfo struct {
	ID            string   `json:"id" yaml:"id"`
	Description   string   `json:"description" yaml:"description"`
	Status        string   `json:"status" yaml:"status"`
	Priority      int      `json:"priority" yaml:"priority"`
	AssignedTo    *string  `json:"assigned_to,omitempty" yaml:"assigned_to,omitempty"`
	ReviewingBy   *string  `json:"reviewing_by,omitempty" yaml:"reviewing_by,omitempty"`
	DependsOn     []string `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	Age           string   `json:"age" yaml:"age"`                       // Computed: time since created
	TimeInStatus  string   `json:"time_in_status" yaml:"time_in_status"` // Computed: time in current status
	BlockedReason *string  `json:"blocked_reason,omitempty" yaml:"blocked_reason,omitempty"`
	Iteration     int      `json:"iteration,omitempty" yaml:"iteration,omitempty"`
	ReviewCycles  int      `json:"review_cycles,omitempty" yaml:"review_cycles,omitempty"`
	LeaseExpires  *string  `json:"lease_expires,omitempty" yaml:"lease_expires,omitempty"`
	Worktree      *string  `json:"worktree,omitempty" yaml:"worktree,omitempty"`
}

// inspectTasks lists all tasks or filters by criteria
func inspectTasks(state *models.State, opts inspectTasksOptions) (any, error) {
	// Get all tasks
	tasks := state.Tasks

	// Apply filters
	filtered := filterTasks(tasks, opts)

	// Build taskInfo with computed fields
	taskInfos := make([]taskInfo, len(filtered))
	for i, task := range filtered {
		taskInfos[i] = buildtaskInfo(&task)
	}

	// If called internally (for composition), return structured data
	if opts.Internal {
		return taskInfos, nil
	}

	// Otherwise, format for output
	return formatTasksOutput(taskInfos, opts.Format)
}

// inspectTask shows details for a single task
func inspectTask(state *models.State, taskID string, opts inspectTasksOptions) (any, error) {
	// Find the task
	var foundTask *models.Task
	for i := range state.Tasks {
		if state.Tasks[i].ID == taskID {
			foundTask = &state.Tasks[i]
			break
		}
	}

	if foundTask == nil {
		return nil, &errors.NotFoundError{Entity: fmt.Sprintf("task %s", taskID)}
	}

	// Build taskInfo with computed fields
	info := buildtaskInfo(foundTask)

	// If called internally, return structured data
	if opts.Internal {
		return info, nil
	}

	// Otherwise, format for output
	return formatTaskOutput(info, opts.Format)
}

// buildtaskInfo converts a Task to taskInfo with computed fields
func buildtaskInfo(task *models.Task) taskInfo {
	info := taskInfo{
		ID:            task.ID,
		Description:   task.Description,
		Status:        string(task.Status),
		Priority:      task.Priority,
		AssignedTo:    task.AssignedTo,
		ReviewingBy:   task.ReviewingBy,
		DependsOn:     task.DependsOn,
		BlockedReason: task.BlockedReason,
		Iteration:     task.Iteration,
		ReviewCycles:  task.ReviewCyclesCurrent,
		Worktree:      task.Worktree,
	}

	// Compute age (time since created)
	age := calculateTaskAge(task)
	info.Age = formatDuration(age)

	// Compute time in current status
	timeInStatus := calculateTimeInStatus(task)
	info.TimeInStatus = formatDuration(timeInStatus)

	// Format lease expiry if present
	if task.LeaseExpires != nil {
		remaining := time.Until(*task.LeaseExpires)
		formatted := formatDuration(remaining)
		info.LeaseExpires = &formatted
	}

	return info
}

// calculateTimeInStatus calculates how long the task has been in its current status
func calculateTimeInStatus(task *models.Task) time.Duration {
	// Find the most recent status transition in history
	// (iterate backwards to find most recent first)
	for i := len(task.History) - 1; i >= 0; i-- {
		entry := task.History[i]
		switch entry.Event {
		case "claimed", "submitted_for_review", "rejected", "approved",
			"merged", "blocked", "abandoned", "superseded", "integration_failed":
			// Found the most recent status change - return duration since then
			return time.Since(entry.Time)
		}
	}

	// If no status change event found, use creation time as fallback
	return time.Since(task.Created)
}

// filterTasks applies filters to task list
func filterTasks(tasks []models.Task, opts inspectTasksOptions) []models.Task {
	var filtered []models.Task

	for _, task := range tasks {
		// Apply status filter
		if opts.StatusFilter != "" && string(task.Status) != opts.StatusFilter {
			continue
		}

		// Apply assigned_to filter
		if opts.AssignedToFilter != "" {
			if task.AssignedTo == nil || *task.AssignedTo != opts.AssignedToFilter {
				continue
			}
		}

		// Apply blocked filter
		if opts.BlockedFilter {
			if task.Status != models.TaskStatusBlocked {
				continue
			}
		}

		filtered = append(filtered, task)
	}

	return filtered
}

// formatTasksOutput formats a list of tasks for output
func formatTasksOutput(tasks []taskInfo, format string) (string, error) {
	// Default to table format
	if format == "" {
		format = "table"
	}

	switch format {
	case "json":
		return formatJSON(tasks)
	case "yaml":
		return formatYAML(tasks)
	case "table":
		return formatTasksTable(tasks), nil
	case "value":
		// Value format doesn't make sense for multiple tasks
		return "", fmt.Errorf("value format not supported for task lists (use json, yaml, or table)")
	default:
		return "", fmt.Errorf("invalid format: %s", format)
	}
}

// formatTaskOutput formats a single task for output
func formatTaskOutput(task taskInfo, format string) (string, error) {
	// Default to value format for single task
	if format == "" {
		format = "value"
	}

	switch format {
	case "json":
		return formatJSON(task)
	case "yaml":
		return formatYAML(task)
	case "value":
		return formatTaskValue(task), nil
	case "table":
		// Single task in table format
		return formatTasksTable([]taskInfo{task}), nil
	default:
		return "", fmt.Errorf("invalid format: %s", format)
	}
}

// formatTasksTable formats tasks as a table
func formatTasksTable(tasks []taskInfo) string {
	if len(tasks) == 0 {
		return "No tasks found"
	}

	headers := []string{"ID", "STATUS", "PRIORITY", "ASSIGNED_TO", "REVIEWING_BY", "DEPS", "AGE", "TIME_IN_STATUS", "DESCRIPTION"}
	var rows [][]string

	for _, task := range tasks {
		assignedTo := "-"
		if task.AssignedTo != nil {
			assignedTo = *task.AssignedTo
		}

		reviewingBy := "-"
		if task.ReviewingBy != nil {
			reviewingBy = *task.ReviewingBy
		}

		deps := "-"
		if len(task.DependsOn) > 0 {
			deps = fmt.Sprintf("%d", len(task.DependsOn))
		}

		desc := task.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}

		rows = append(rows, []string{
			task.ID,
			task.Status,
			fmt.Sprintf("%d", task.Priority),
			assignedTo,
			reviewingBy,
			deps,
			task.Age,
			task.TimeInStatus,
			desc,
		})
	}

	return formatTable(headers, rows)
}

// formatTaskValue formats a single task as key-value pairs
func formatTaskValue(task taskInfo) string {
	lines := []string{
		fmt.Sprintf("ID: %s", task.ID),
		fmt.Sprintf("Description: %s", task.Description),
		fmt.Sprintf("Status: %s", task.Status),
		fmt.Sprintf("Priority: %d", task.Priority),
	}

	if task.AssignedTo != nil {
		lines = append(lines, fmt.Sprintf("Assigned To: %s", *task.AssignedTo))
	} else {
		lines = append(lines, "Assigned To: -")
	}

	if task.ReviewingBy != nil {
		lines = append(lines, fmt.Sprintf("Reviewing By: %s", *task.ReviewingBy))
	} else {
		lines = append(lines, "Reviewing By: -")
	}

	lines = append(lines, fmt.Sprintf("Age: %s", task.Age))
	lines = append(lines, fmt.Sprintf("Time in Status: %s", task.TimeInStatus))

	if len(task.DependsOn) > 0 {
		lines = append(lines, fmt.Sprintf("Dependencies: %s", strings.Join(task.DependsOn, ", ")))
	} else {
		lines = append(lines, "Dependencies: none")
	}

	if task.BlockedReason != nil {
		lines = append(lines, fmt.Sprintf("Blocked Reason: %s", *task.BlockedReason))
	}

	if task.Iteration > 0 {
		lines = append(lines, fmt.Sprintf("Iteration: %d", task.Iteration))
	}

	if task.ReviewCycles > 0 {
		lines = append(lines, fmt.Sprintf("Review Cycles: %d", task.ReviewCycles))
	}

	if task.LeaseExpires != nil {
		lines = append(lines, fmt.Sprintf("Lease Expires: %s", *task.LeaseExpires))
	}

	if task.Worktree != nil {
		lines = append(lines, fmt.Sprintf("Worktree: %s", *task.Worktree))
	}

	var result strings.Builder
	for _, line := range lines {
		result.WriteString(line)
		result.WriteString("\n")
	}
	return result.String()
}
