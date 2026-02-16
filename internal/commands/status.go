package commands

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// StatusOptions contains options for the status command
type StatusOptions struct {
	Format      string // "dashboard", "json", "yaml"
	Detailed    bool   // Include anomalies and circuit breaker
	ProjectRoot string
}

// statusData contains all status information
type statusData struct {
	Goal           goalStatus            `json:"goal"`
	Sprint         sprintStatus          `json:"sprint"`
	Config         configStatus          `json:"config"`
	Tasks          taskStatus            `json:"tasks"`
	Agents         []agentStatus         `json:"agents"`
	PlannerState   plannerStatus         `json:"planner_state"`
	WorkQueues     workQueuesStatus      `json:"work_queues"`
	Anomalies      *[]string             `json:"anomalies,omitempty"`
	CircuitBreaker *circuitBreakerStatus `json:"circuit_breaker,omitempty"`
}

type goalStatus struct {
	Description string `json:"description"`
	Status      string `json:"status"`
	SpecRef     string `json:"spec_ref"`
}

type sprintStatus struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	StartTime  string `json:"start_time"`
	TasksDone  int    `json:"tasks_done"`
	TasksTotal int    `json:"tasks_total"`
}

type configStatus struct {
	Mode        string  `json:"mode"`
	PausedBy    *string `json:"paused_by,omitempty"`
	PauseReason *string `json:"pause_reason,omitempty"`
}

type taskStatus struct {
	Total         int            `json:"total"`
	Active        int            `json:"active"`
	Terminal      int            `json:"terminal"`
	ByStatus      map[string]int `json:"by_status"`
	Claimable     int            `json:"claimable"`
	Reviewable    int            `json:"reviewable"`
	BlockedByDeps int            `json:"blocked_by_deps"`
}

type agentStatus struct {
	ID                 string `json:"id"`
	Role               string `json:"role"`
	Status             string `json:"status"`
	PID                int    `json:"pid"`
	CurrentTask        string `json:"current_task"`
	TimeSinceHeartbeat string `json:"time_since_heartbeat"`
	ProcessStatus      string `json:"process_status"`
}

type plannerStatus struct {
	Trigger      string `json:"trigger"`
	TriggerCount int    `json:"trigger_count"`
	Reason       string `json:"reason"`
}

type workQueuesStatus struct {
	Coder    queueStatus `json:"coder"`
	Reviewer queueStatus `json:"reviewer"`
}

type queueStatus struct {
	Available int    `json:"available"`
	Reason    string `json:"reason"`
}

type circuitBreakerStatus struct {
	Status   string   `json:"status"`
	Triggers []string `json:"triggers,omitempty"`
}

// StatusCommand returns a comprehensive system status
func StatusCommand(opts StatusOptions) (string, error) {
	// Setup paths
	statePath := paths.New(opts.ProjectRoot).StatePath()

	// Read state
	bb := db.New(statePath)
	state, err := bb.Read()
	if err != nil {
		return "", fmt.Errorf("failed to read state: %w", err)
	}

	// Build status data
	status := buildStatusData(state, opts.Detailed)

	// Format output
	switch opts.Format {
	case "json":
		return formatJSON(status)
	case "yaml":
		return formatYAML(status)
	default: // "dashboard" or empty
		return formatStatusDashboard(status)
	}
}

// buildStatusData populates the statusData structure from state
func buildStatusData(state *models.State, detailed bool) statusData {
	data := statusData{}

	// Populate goal information
	data.Goal = goalStatus{
		Description: state.Goal.Description,
		Status:      string(state.Goal.Status),
		SpecRef:     state.Goal.SpecRef,
	}

	// Populate sprint information
	data.Sprint = sprintStatus{
		ID:         state.Sprint.ID,
		Status:     string(state.Sprint.Status),
		StartTime:  state.Sprint.Timeline.Started.Format(time.RFC3339),
		TasksDone:  state.Sprint.Metrics.TasksDone,
		TasksTotal: len(state.Tasks),
	}

	// Populate config/system mode
	data.Config = configStatus{
		Mode: string(state.Config.Mode),
	}
	if state.Config.Mode == models.SystemModePaused {
		data.Config.PausedBy = state.Config.ModeChangedBy
		// Could add reason if we had a field for it
	}

	// Populate task statistics
	data.Tasks = buildTaskStatus(state)

	// Populate agent information
	data.Agents = buildAgentStatuses(state)

	// Populate planner state
	data.PlannerState = buildPlannerStatus(state)

	// Populate work queues
	data.WorkQueues = buildWorkQueuesStatus(state)

	// Optionally include detailed information
	if detailed {
		if len(state.Anomalies) > 0 {
			anomalies := make([]string, len(state.Anomalies))
			for i, anomaly := range state.Anomalies {
				anomalies[i] = fmt.Sprintf("[%s] %s by %s: %s",
					anomaly.Timestamp.Format("2006-01-02 15:04"),
					anomaly.Type,
					anomaly.Reporter,
					anomaly.Task)
			}
			data.Anomalies = &anomalies
		}

		if state.CircuitBreaker.Status != "" && state.CircuitBreaker.Status != "OK" {
			cb := &circuitBreakerStatus{
				Status: state.CircuitBreaker.Status,
			}
			if state.CircuitBreaker.CurrentTrigger != nil {
				cb.Triggers = []string{
					fmt.Sprintf("%s (severity: %s)",
						state.CircuitBreaker.CurrentTrigger.Pattern,
						state.CircuitBreaker.CurrentTrigger.Severity),
				}
			}
			data.CircuitBreaker = cb
		}
	}

	return data
}

// buildTaskStatus calculates task statistics
func buildTaskStatus(state *models.State) taskStatus {
	ts := taskStatus{
		Total:    len(state.Tasks),
		ByStatus: make(map[string]int),
	}

	// Build merged task IDs for dependency checking
	mergedIDs := make(map[string]bool)
	for _, task := range state.Tasks {
		if task.Status == models.TaskStatusMerged {
			mergedIDs[task.ID] = true
		}
	}

	// Count tasks by status and categories
	for _, task := range state.Tasks {
		// Count by status
		ts.ByStatus[string(task.Status)]++

		// Count active vs terminal
		if task.Status.IsTerminal() {
			ts.Terminal++
		} else {
			ts.Active++
		}

		// Check if blocked by dependencies
		if task.Status == models.TaskStatusUnclaimed ||
			task.Status == models.TaskStatusRejected ||
			task.Status == models.TaskStatusIntegrationFailed {
			hasUnsatisfiedDeps := false
			for _, depID := range task.DependsOn {
				if !mergedIDs[depID] {
					hasUnsatisfiedDeps = true
					break
				}
			}
			if hasUnsatisfiedDeps {
				ts.BlockedByDeps++
			}
		}
	}

	// Count work availability
	ts.Claimable = countClaimableTasks(state)
	ts.Reviewable = countReviewableTasks(state)

	return ts
}

// buildAgentStatuses converts agent map to agent status list
func buildAgentStatuses(state *models.State) []agentStatus {
	agents := make([]agentStatus, 0, len(state.Agents))
	now := time.Now().UTC()

	for id, agent := range state.Agents {
		as := agentStatus{
			ID:          id,
			Role:        agent.Role,
			Status:      string(agent.Status),
			CurrentTask: "",
		}

		if agent.CurrentTask != nil {
			as.CurrentTask = *agent.CurrentTask
		}

		// Calculate time since heartbeat
		timeSince := now.Sub(agent.Heartbeat)
		as.TimeSinceHeartbeat = formatDuration(timeSince)

		// Check process status
		as.ProcessStatus = getProcessStatus(agent.PID)
		as.PID = agent.PID

		agents = append(agents, as)
	}

	return agents
}

// buildPlannerStatus determines planner state
func buildPlannerStatus(state *models.State) plannerStatus {
	trigger, count := detectPlannerWakeTriggers(state)

	ps := plannerStatus{
		Trigger:      trigger,
		TriggerCount: count,
	}

	// Build human-readable reason
	switch trigger {
	case "INITIAL_PLANNING":
		ps.Reason = "No tasks exist; initial planning needed"
	case "BLOCKED_TASKS":
		ps.Reason = fmt.Sprintf("%d task(s) are blocked and need attention", count)
	case "INTEGRATION_FAILED":
		ps.Reason = fmt.Sprintf("%d task(s) failed integration", count)
	case "HYPOTHESIS_EXHAUSTED":
		ps.Reason = fmt.Sprintf("%d task(s) exhausted hypotheses (2+ failures)", count)
	case "IMMEDIATE_DISCOVERY":
		ps.Reason = fmt.Sprintf("%d immediate discovery(ies) need to be converted to tasks", count)
	case "NONE":
		ps.Reason = "No triggers; planner is idle"
	default:
		ps.Reason = "Unknown trigger"
	}

	return ps
}

// buildWorkQueuesStatus calculates work queue availability
func buildWorkQueuesStatus(state *models.State) workQueuesStatus {
	coderCount := countClaimableTasks(state)
	reviewerCount := countReviewableTasks(state)

	return workQueuesStatus{
		Coder: queueStatus{
			Available: coderCount,
			Reason:    getCoderWorkDiagnostics(state),
		},
		Reviewer: queueStatus{
			Available: reviewerCount,
			Reason:    getReviewerWorkDiagnostics(state),
		},
	}
}

// countClaimableTasks counts tasks that coders can claim
func countClaimableTasks(state *models.State) int {
	mergedIDs := make(map[string]bool)
	for _, task := range state.Tasks {
		if task.Status == models.TaskStatusMerged {
			mergedIDs[task.ID] = true
		}
	}

	count := 0
	for _, task := range state.Tasks {
		if task.Status != models.TaskStatusUnclaimed &&
			task.Status != models.TaskStatusRejected &&
			task.Status != models.TaskStatusIntegrationFailed {
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

// countReviewableTasks counts tasks that reviewers can review
func countReviewableTasks(state *models.State) int {
	now := time.Now().UTC()
	count := 0

	for _, task := range state.Tasks {
		if task.Status != models.TaskStatusReadyForReview {
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

// detectPlannerWakeTriggers detects conditions that should wake the planner
func detectPlannerWakeTriggers(state *models.State) (trigger string, count int) {
	if len(state.Tasks) == 0 {
		return "INITIAL_PLANNING", 1
	}

	blocked := 0
	for _, task := range state.Tasks {
		if task.Status == models.TaskStatusBlocked {
			blocked++
		}
	}
	if blocked > 0 {
		return "BLOCKED_TASKS", blocked
	}

	integrationFailed := 0
	for _, task := range state.Tasks {
		if task.Status == models.TaskStatusIntegrationFailed {
			integrationFailed++
		}
	}
	if integrationFailed > 0 {
		return "INTEGRATION_FAILED", integrationFailed
	}

	hypothesisExhausted := 0
	for _, task := range state.Tasks {
		if len(task.FailedBy) >= 2 {
			hypothesisExhausted++
		}
	}
	if hypothesisExhausted > 0 {
		return "HYPOTHESIS_EXHAUSTED", hypothesisExhausted
	}

	immediateDiscoveries := 0
	for _, disc := range state.Discovered {
		if disc.Urgency == "immediate" && disc.ConvertedToTask == nil {
			immediateDiscoveries++
		}
	}
	if immediateDiscoveries > 0 {
		return "IMMEDIATE_DISCOVERY", immediateDiscoveries
	}

	return "NONE", 0
}

// getCoderWorkDiagnostics returns diagnostic message for coder work
func getCoderWorkDiagnostics(state *models.State) string {
	claimable := countClaimableTasks(state)

	if claimable > 0 {
		return fmt.Sprintf("Found %d claimable task(s)", claimable)
	}

	blockedByDeps := 0
	inProgress := 0

	mergedIDs := make(map[string]bool)
	for _, task := range state.Tasks {
		if task.Status == models.TaskStatusMerged {
			mergedIDs[task.ID] = true
		}
	}

	for _, task := range state.Tasks {
		if task.Status == models.TaskStatusUnclaimed ||
			task.Status == models.TaskStatusRejected ||
			task.Status == models.TaskStatusIntegrationFailed {
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

		if task.Status == models.TaskStatusClaimed ||
			task.Status == models.TaskStatusReadyForReview ||
			task.Status == models.TaskStatusApproved {
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

// getReviewerWorkDiagnostics returns diagnostic message for reviewer work
// This is a copy of the internal agent function to avoid import cycles
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

// getProcessStatus checks if a process is running
func getProcessStatus(pid int) string {
	if pid == 0 {
		return "unknown"
	}

	// Try to find the process
	process, err := os.FindProcess(pid)
	if err != nil {
		return "not found"
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return "running"
	}

	return "stopped"
}

// formatStatusDashboard renders the status as a dashboard
func formatStatusDashboard(data statusData) (string, error) {
	var b strings.Builder

	// Goal section
	b.WriteString("=== GOAL ===\n")
	b.WriteString(fmt.Sprintf("Description: %s\n", data.Goal.Description))
	b.WriteString(fmt.Sprintf("Status: %s\n", data.Goal.Status))
	b.WriteString(fmt.Sprintf("Spec: %s\n\n", data.Goal.SpecRef))

	// Sprint section
	b.WriteString("=== SPRINT ===\n")
	b.WriteString(fmt.Sprintf("ID: %s\n", data.Sprint.ID))
	b.WriteString(fmt.Sprintf("Status: %s\n", data.Sprint.Status))
	b.WriteString(fmt.Sprintf("Started: %s\n", data.Sprint.StartTime))
	b.WriteString(fmt.Sprintf("Progress: %d/%d tasks complete\n\n",
		data.Sprint.TasksDone, data.Sprint.TasksTotal))

	// System section
	b.WriteString("=== SYSTEM ===\n")
	b.WriteString(fmt.Sprintf("Mode: %s\n", data.Config.Mode))
	if data.Config.Mode == "PAUSED" && data.Config.PausedBy != nil {
		b.WriteString(fmt.Sprintf("Paused By: %s\n", *data.Config.PausedBy))
		if data.Config.PauseReason != nil {
			b.WriteString(fmt.Sprintf("Reason: %s\n", *data.Config.PauseReason))
		}
	}
	b.WriteString("\n")

	// Tasks section
	b.WriteString("=== TASKS ===\n")
	b.WriteString(fmt.Sprintf("Total: %d (%d active, %d terminal)\n",
		data.Tasks.Total, data.Tasks.Active, data.Tasks.Terminal))

	if len(data.Tasks.ByStatus) > 0 {
		b.WriteString("\nBy Status:\n")
		// Sort status names for consistent output
		statuses := make([]string, 0, len(data.Tasks.ByStatus))
		for status := range data.Tasks.ByStatus {
			statuses = append(statuses, status)
		}
		// Simple bubble sort for consistent ordering
		for i := 0; i < len(statuses); i++ {
			for j := i + 1; j < len(statuses); j++ {
				if statuses[i] > statuses[j] {
					statuses[i], statuses[j] = statuses[j], statuses[i]
				}
			}
		}
		for _, status := range statuses {
			count := data.Tasks.ByStatus[status]
			b.WriteString(fmt.Sprintf("  %s: %d\n", status, count))
		}
	}

	b.WriteString(fmt.Sprintf("\nClaimable: %d tasks\n", data.Tasks.Claimable))
	b.WriteString(fmt.Sprintf("Reviewable: %d tasks\n", data.Tasks.Reviewable))
	if data.Tasks.BlockedByDeps > 0 {
		b.WriteString(fmt.Sprintf("Blocked by dependencies: %d tasks\n", data.Tasks.BlockedByDeps))
	}
	b.WriteString("\n")

	// Agents section
	b.WriteString("=== AGENTS ===\n")
	if len(data.Agents) == 0 {
		b.WriteString("No active agents\n\n")
	} else {
		headers := []string{"ID", "Role", "Status", "PID", "Task", "Heartbeat", "Process"}
		rows := make([][]string, len(data.Agents))
		for i, agent := range data.Agents {
			pidStr := "-"
			if agent.PID != 0 {
				pidStr = fmt.Sprintf("%d", agent.PID)
			}
			rows[i] = []string{
				agent.ID,
				agent.Role,
				agent.Status,
				pidStr,
				agent.CurrentTask,
				agent.TimeSinceHeartbeat,
				agent.ProcessStatus,
			}
		}
		b.WriteString(formatTable(headers, rows))
		b.WriteString("\n\n")
	}

	// Planner section
	b.WriteString("=== PLANNER ===\n")
	b.WriteString(fmt.Sprintf("Wake Trigger: %s", data.PlannerState.Trigger))
	if data.PlannerState.TriggerCount > 0 {
		b.WriteString(fmt.Sprintf(" (count: %d)", data.PlannerState.TriggerCount))
	}
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Explanation: %s\n\n", data.PlannerState.Reason))

	// Work queues section
	b.WriteString("=== WORK QUEUES ===\n")
	b.WriteString(fmt.Sprintf("Coder: %d available - %s\n",
		data.WorkQueues.Coder.Available, data.WorkQueues.Coder.Reason))
	b.WriteString(fmt.Sprintf("Reviewer: %d available - %s\n\n",
		data.WorkQueues.Reviewer.Available, data.WorkQueues.Reviewer.Reason))

	// Optional detailed sections
	if data.Anomalies != nil && len(*data.Anomalies) > 0 {
		b.WriteString("=== ANOMALIES ===\n")
		for _, anomaly := range *data.Anomalies {
			b.WriteString(fmt.Sprintf("⚠  %s\n", anomaly))
		}
		b.WriteString("\n")
	}

	if data.CircuitBreaker != nil {
		b.WriteString("=== CIRCUIT BREAKER ===\n")
		b.WriteString(fmt.Sprintf("Status: %s\n", data.CircuitBreaker.Status))
		if len(data.CircuitBreaker.Triggers) > 0 {
			b.WriteString("Triggers:\n")
			for _, trigger := range data.CircuitBreaker.Triggers {
				b.WriteString(fmt.Sprintf("  - %s\n", trigger))
			}
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}
