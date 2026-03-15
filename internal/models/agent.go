package models

import "time"

// AgentStatus represents the state of an agent
type AgentStatus string

const (
	AgentStatusStarting  AgentStatus = "STARTING"
	AgentStatusIdle      AgentStatus = "IDLE"
	AgentStatusWorking   AgentStatus = "WORKING"
	AgentStatusReviewing AgentStatus = "REVIEWING"
	AgentStatusWaiting   AgentStatus = "WAITING"
	AgentStatusHandoff   AgentStatus = "HANDOFF"
	AgentStatusPlanning  AgentStatus = "PLANNING"
)

// IsValid checks if the agent status is valid
func (as AgentStatus) IsValid() bool {
	switch as {
	case AgentStatusStarting, AgentStatusIdle, AgentStatusWorking,
		AgentStatusReviewing, AgentStatusWaiting, AgentStatusHandoff,
		AgentStatusPlanning:
		return true
	}
	return false
}

// Agent represents an agent (coder, reviewer, orchestrator) in the system
type Agent struct {
	Role            string         `yaml:"role"`
	Status          AgentStatus    `yaml:"status"`
	CurrentTask     *string        `yaml:"current_task,omitempty"`
	LeaseExpires    *time.Time     `yaml:"lease_expires,omitempty"`
	Heartbeat       time.Time      `yaml:"heartbeat"`
	Terminal        string         `yaml:"terminal"`
	Provider        string         `yaml:"provider,omitempty"`
	IterationsTotal int            `yaml:"iterations_total"`
	ContextPercent  int            `yaml:"context_percent"`
	PID             int            `yaml:"pid,omitempty"`
	Extra           map[string]any `yaml:",inline"`
}

// ReleaseAgent resets an agent to idle with no task assignment.
func (s *State) ReleaseAgent(agentID string) {
	if agent, ok := s.Agents[agentID]; ok {
		agent.Status = AgentStatusIdle
		agent.CurrentTask = nil
		agent.LeaseExpires = nil
		s.Agents[agentID] = agent
	}
}
