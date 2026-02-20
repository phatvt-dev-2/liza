package commands

import (
	"fmt"
	"sort"
	"time"

	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
)

// inspectAgentsOptions contains options for agent inspection
type inspectAgentsOptions struct {
	Format       string // Output format: json, yaml, table, value
	RoleFilter   string // Filter by role
	StatusFilter string // Filter by status
	Internal     bool   // Return structured data for composition
}

// agentInfo represents agent information with computed fields
type agentInfo struct {
	ID                 string  `json:"id" yaml:"id"`
	Role               string  `json:"role" yaml:"role"`
	Status             string  `json:"status" yaml:"status"`
	PID                int     `json:"pid" yaml:"pid"`
	ProcessStatus      string  `json:"process_status" yaml:"process_status"`
	CurrentTask        *string `json:"current_task,omitempty" yaml:"current_task,omitempty"`
	TimeOnTask         string  `json:"time_on_task,omitempty" yaml:"time_on_task,omitempty"`   // Computed
	TimeSinceHeartbeat string  `json:"time_since_heartbeat" yaml:"time_since_heartbeat"`       // Computed
	LeaseExpires       *string `json:"lease_expires,omitempty" yaml:"lease_expires,omitempty"` // Computed (formatted)
	Terminal           string  `json:"terminal" yaml:"terminal"`
	IterationsTotal    int     `json:"iterations_total" yaml:"iterations_total"`
	ContextPercent     int     `json:"context_percent" yaml:"context_percent"`
}

// inspectAgents lists all agents or filters by criteria
func inspectAgents(state *models.State, opts inspectAgentsOptions) (any, error) {
	// Get all agents as a slice (agents are stored in a map)
	agents := make([]agentInfo, 0, len(state.Agents))
	for agentID, agent := range state.Agents {
		// Apply filters
		if opts.RoleFilter != "" && agent.Role != opts.RoleFilter {
			continue
		}
		if opts.StatusFilter != "" && string(agent.Status) != opts.StatusFilter {
			continue
		}

		// Find the task this agent is working on (if any)
		var currentTask *models.Task
		if agent.CurrentTask != nil {
			currentTask = state.FindTask(*agent.CurrentTask)
		}

		// Build agentInfo with computed fields
		info := buildagentInfo(agentID, &agent, currentTask)
		agents = append(agents, info)
	}

	// Sort agents by ID for consistent output
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].ID < agents[j].ID
	})

	// If called internally (for composition), return structured data
	if opts.Internal {
		return agents, nil
	}

	// Otherwise, format for output
	return formatAgentsOutput(agents, opts.Format)
}

// inspectAgent shows details for a single agent
func inspectAgent(state *models.State, agentID string, opts inspectAgentsOptions) (any, error) {
	// Find the agent
	agent, exists := state.Agents[agentID]
	if !exists {
		return nil, &errors.NotFoundError{Entity: fmt.Sprintf("agent %s", agentID)}
	}

	// Find the task this agent is working on (if any)
	var currentTask *models.Task
	if agent.CurrentTask != nil {
		currentTask = state.FindTask(*agent.CurrentTask)
	}

	// Build agentInfo with computed fields
	info := buildagentInfo(agentID, &agent, currentTask)

	// If called internally, return structured data
	if opts.Internal {
		return info, nil
	}

	// Otherwise, format for output
	return formatAgentOutput(info, opts.Format)
}

// buildagentInfo converts an Agent to agentInfo with computed fields
func buildagentInfo(agentID string, agent *models.Agent, currentTask *models.Task) agentInfo {
	info := agentInfo{
		ID:              agentID,
		Role:            agent.Role,
		Status:          string(agent.Status),
		CurrentTask:     agent.CurrentTask,
		Terminal:        agent.Terminal,
		IterationsTotal: agent.IterationsTotal,
		ContextPercent:  agent.ContextPercent,
	}

	// Copy PID and determine process status
	info.PID = agent.PID
	if agent.PID == 0 {
		info.ProcessStatus = "n/a"
	} else if ops.IsProcessAlive(agent.PID) {
		info.ProcessStatus = "running"
	} else {
		info.ProcessStatus = "not found"
	}

	// Compute time since last heartbeat
	timeSinceHeartbeat := calculateTimeSinceHeartbeat(agent)
	info.TimeSinceHeartbeat = formatDuration(timeSinceHeartbeat)

	// Compute time on task (if agent is working on a task)
	if currentTask != nil {
		timeOnTask := calculateTimeOnTask(currentTask)
		info.TimeOnTask = formatDuration(timeOnTask)
	}

	// Format lease expiry if present
	if agent.LeaseExpires != nil {
		remaining := time.Until(*agent.LeaseExpires)
		formatted := formatDuration(remaining)
		info.LeaseExpires = &formatted
	}

	return info
}

// formatAgentsOutput formats a list of agents for output
func formatAgentsOutput(agents []agentInfo, format string) (string, error) {
	// Default to table format
	if format == "" {
		format = "table"
	}

	switch format {
	case "json":
		return formatJSON(agents)
	case "yaml":
		return formatYAML(agents)
	case "table":
		return formatAgentsTable(agents), nil
	case "value":
		// Value format doesn't make sense for multiple agents
		return "", fmt.Errorf("value format not supported for agent lists (use json, yaml, or table)")
	default:
		return "", fmt.Errorf("invalid format: %s", format)
	}
}

// formatAgentOutput formats a single agent for output
func formatAgentOutput(agent agentInfo, format string) (string, error) {
	// Default to value format for single agent
	if format == "" {
		format = "value"
	}

	switch format {
	case "json":
		return formatJSON(agent)
	case "yaml":
		return formatYAML(agent)
	case "value":
		return formatAgentValue(agent), nil
	case "table":
		// Single agent in table format
		return formatAgentsTable([]agentInfo{agent}), nil
	default:
		return "", fmt.Errorf("invalid format: %s", format)
	}
}

// formatAgentsTable formats agents as a table
func formatAgentsTable(agents []agentInfo) string {
	if len(agents) == 0 {
		return "No agents found"
	}

	headers := []string{"ID", "ROLE", "STATUS", "PID", "CURRENT_TASK", "TIME_ON_TASK", "HEARTBEAT", "CONTEXT"}
	var rows [][]string

	for _, agent := range agents {
		currentTask := "-"
		if agent.CurrentTask != nil {
			currentTask = *agent.CurrentTask
		}

		timeOnTask := "-"
		if agent.TimeOnTask != "" {
			timeOnTask = agent.TimeOnTask
		}

		// Format PID with status indicator
		pidDisplay := "-"
		if agent.PID == 0 {
			pidDisplay = "- n/a"
		} else if agent.ProcessStatus == "running" {
			pidDisplay = fmt.Sprintf("%d ✓", agent.PID)
		} else {
			pidDisplay = fmt.Sprintf("%d ✗", agent.PID)
		}

		rows = append(rows, []string{
			agent.ID,
			agent.Role,
			agent.Status,
			pidDisplay,
			currentTask,
			timeOnTask,
			agent.TimeSinceHeartbeat + " ago",
			fmt.Sprintf("%d%%", agent.ContextPercent),
		})
	}

	return formatTable(headers, rows)
}

// formatAgentValue formats a single agent as key-value pairs
func formatAgentValue(agent agentInfo) string {
	return executeCommandTemplate("agent_value", agent)
}
