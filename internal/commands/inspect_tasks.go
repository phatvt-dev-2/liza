package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/render"
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
	ID              string               `json:"id" yaml:"id"`
	Description     string               `json:"description" yaml:"description"`
	Status          string               `json:"status" yaml:"status"`
	Priority        int                  `json:"priority" yaml:"priority"`
	AssignedTo      *string              `json:"assigned_to,omitempty" yaml:"assigned_to,omitempty"`
	ReviewingBy     *string              `json:"reviewing_by,omitempty" yaml:"reviewing_by,omitempty"`
	DependsOn       []string             `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	Age             string               `json:"age" yaml:"age"`                       // Computed: time since created
	TimeInStatus    string               `json:"time_in_status" yaml:"time_in_status"` // Computed: time in current status
	BlockedReason   *string              `json:"blocked_reason,omitempty" yaml:"blocked_reason,omitempty"`
	Iteration       int                  `json:"iteration,omitempty" yaml:"iteration,omitempty"`
	ReviewCycles    int                  `json:"review_cycles,omitempty" yaml:"review_cycles,omitempty"`
	LeaseExpires    *string              `json:"lease_expires,omitempty" yaml:"lease_expires,omitempty"`
	Worktree        *string              `json:"worktree,omitempty" yaml:"worktree,omitempty"`
	DoneWhen        string               `json:"done_when,omitempty" yaml:"done_when,omitempty"`
	Scope           string               `json:"scope,omitempty" yaml:"scope,omitempty"`
	SpecRef         string               `json:"spec_ref,omitempty" yaml:"spec_ref,omitempty"`
	RejectionReason *string              `json:"rejection_reason,omitempty" yaml:"rejection_reason,omitempty"`
	Output          []models.OutputEntry `json:"output,omitempty" yaml:"output,omitempty"`
}

// inspectTasks lists all tasks or filters by criteria
func inspectTasks(state *models.State, opts inspectTasksOptions) (any, error) {
	filtered := filterTasks(state.Tasks, opts)

	taskInfos := make([]taskInfo, len(filtered))
	for i, task := range filtered {
		taskInfos[i] = buildTaskInfo(&task)
	}

	if opts.Internal {
		return taskInfos, nil
	}
	return formatTasksOutput(taskInfos, opts.Format)
}

// inspectTask shows details for a single task
func inspectTask(state *models.State, taskID string, opts inspectTasksOptions) (any, error) {
	foundTask := state.FindTask(taskID)
	if foundTask == nil {
		return nil, &errors.NotFoundError{Entity: "task", ID: taskID}
	}

	info := buildTaskInfo(foundTask)
	if opts.Internal {
		return info, nil
	}
	return formatTaskOutput(info, opts.Format)
}

// buildTaskInfo converts a Task to taskInfo with computed fields
func buildTaskInfo(task *models.Task) taskInfo {
	info := taskInfo{
		ID:              task.ID,
		Description:     task.Description,
		Status:          string(task.Status),
		Priority:        task.Priority,
		AssignedTo:      task.AssignedTo,
		ReviewingBy:     task.ReviewingBy,
		DependsOn:       task.DependsOn,
		BlockedReason:   task.BlockedReason,
		Iteration:       task.Iteration,
		ReviewCycles:    task.ReviewCyclesCurrent,
		Worktree:        task.Worktree,
		DoneWhen:        task.DoneWhen,
		Scope:           task.Scope,
		SpecRef:         task.SpecRef,
		RejectionReason: task.RejectionReason,
		Output:          task.Output,
	}

	info.Age = render.FormatDuration(calculateTaskAge(task))
	info.TimeInStatus = render.FormatDuration(calculateTimeInStatus(task))

	if task.LeaseExpires != nil {
		remaining := time.Until(*task.LeaseExpires)
		formatted := render.FormatDuration(remaining)
		info.LeaseExpires = &formatted
	}

	return info
}

// calculateTimeInStatus calculates how long the task has been in its current status
func calculateTimeInStatus(task *models.Task) time.Duration {
	for i := len(task.History) - 1; i >= 0; i-- {
		entry := task.History[i]
		switch entry.Event {
		case "claimed", "submitted_for_review", "rejected", "approved",
			"merged", "blocked", "abandoned", "superseded", "integration_failed":
			return time.Since(entry.Time)
		}
	}

	return time.Since(task.Created)
}

// filterTasks applies filters to task list
func filterTasks(tasks []models.Task, opts inspectTasksOptions) []models.Task {
	var filtered []models.Task

	for _, task := range tasks {
		if opts.StatusFilter != "" && string(task.Status) != opts.StatusFilter {
			continue
		}
		if opts.AssignedToFilter != "" {
			if task.AssignedTo == nil || *task.AssignedTo != opts.AssignedToFilter {
				continue
			}
		}
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
	if format == "" {
		format = "table"
	}

	switch format {
	case "json":
		return render.FormatJSON(tasks)
	case "yaml":
		return render.FormatYAML(tasks)
	case "table":
		return formatTasksTable(tasks), nil
	case "value":
		return "", fmt.Errorf("value format not supported for task lists (use json, yaml, or table)")
	default:
		return "", fmt.Errorf("invalid format: %s", format)
	}
}

// formatTaskOutput formats a single task for output
func formatTaskOutput(task taskInfo, format string) (string, error) {
	if format == "" {
		format = "value"
	}

	switch format {
	case "json":
		return render.FormatJSON(task)
	case "yaml":
		return render.FormatYAML(task)
	case "value":
		return formatTaskValue(task), nil
	case "table":
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

	return render.FormatTable(headers, rows)
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

	if task.DoneWhen != "" {
		lines = append(lines, fmt.Sprintf("Done When: %s", task.DoneWhen))
	}

	if task.Scope != "" {
		lines = append(lines, fmt.Sprintf("Scope: %s", task.Scope))
	}

	if task.SpecRef != "" {
		lines = append(lines, fmt.Sprintf("Spec Ref: %s", task.SpecRef))
	}

	if task.RejectionReason != nil {
		lines = append(lines, fmt.Sprintf("Rejection Reason: %s", *task.RejectionReason))
	}

	if len(task.Output) > 0 {
		lines = append(lines, fmt.Sprintf("Output: %d entries", len(task.Output)))
	}

	var result strings.Builder
	for _, line := range lines {
		result.WriteString(line)
		result.WriteString("\n")
	}
	return result.String()
}
