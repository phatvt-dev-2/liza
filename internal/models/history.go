package models

import (
	"slices"
	"time"
)

// GoalStatus represents the state of a goal
type GoalStatus string

const (
	GoalStatusInProgress GoalStatus = "IN_PROGRESS"
	GoalStatusCompleted  GoalStatus = "COMPLETED"
	GoalStatusAborted    GoalStatus = "ABORTED"
)

// IsValid checks if the goal status is valid
func (gs GoalStatus) IsValid() bool {
	return gs == GoalStatusInProgress || gs == GoalStatusCompleted || gs == GoalStatusAborted
}

// Goal represents the high-level goal spanning one or more sprints
type Goal struct {
	ID               string             `yaml:"id"`
	Description      string             `yaml:"description"`
	SpecRef          string             `yaml:"spec_ref"`
	EntryPoint       string             `yaml:"entry_point,omitempty"`
	Created          time.Time          `yaml:"created"`
	Status           GoalStatus         `yaml:"status"`
	AlignmentHistory []AlignmentHistory `yaml:"alignment_history"`
	Extra            map[string]any     `yaml:",inline"`
}

// AlignmentHistory tracks goal alignment events
type AlignmentHistory struct {
	Timestamp time.Time      `yaml:"timestamp"`
	Event     string         `yaml:"event"`
	Summary   string         `yaml:"summary"`
	Extra     map[string]any `yaml:",inline"`
}

// TaskHistoryEntry represents a single event in a task's history
type TaskHistoryEntry struct {
	Time             time.Time      `yaml:"time"`
	Event            string         `yaml:"event"`
	Agent            *string        `yaml:"agent,omitempty"`
	PreviousAssignee *string        `yaml:"previous_assignee,omitempty"`
	Reason           *string        `yaml:"reason,omitempty"`
	Commit           *string        `yaml:"commit,omitempty"`
	Note             *string        `yaml:"note,omitempty"`
	Extra            map[string]any `yaml:",inline"`
}

// Discovery represents a finding by an agent during work
type Discovery struct {
	ID              string         `yaml:"id"`
	By              string         `yaml:"by"`
	During          string         `yaml:"during"`
	Description     string         `yaml:"description"`
	Severity        string         `yaml:"severity"`
	Urgency         string         `yaml:"urgency"`
	Recommendation  string         `yaml:"recommendation"`
	Created         time.Time      `yaml:"created"`
	ConvertedToTask *string        `yaml:"converted_to_task,omitempty"`
	Extra           map[string]any `yaml:",inline"`
}

// IsValidSeverity checks if the discovery severity is valid
func (d *Discovery) IsValidSeverity() bool {
	return d.Severity == "critical" || d.Severity == "high" ||
		d.Severity == "medium" || d.Severity == "low"
}

// IsValidUrgency checks if the discovery urgency is valid
func (d *Discovery) IsValidUrgency() bool {
	return d.Urgency == "immediate" || d.Urgency == "deferred"
}

// HandoffNote represents context handoff between agents
type HandoffNote struct {
	Agent         string         `yaml:"agent"`
	ContextUsed   int            `yaml:"context_used"`
	Timestamp     time.Time      `yaml:"timestamp"`
	Summary       string         `yaml:"summary"`
	NextAction    string         `yaml:"next_action"`
	Approach      *string        `yaml:"approach,omitempty"`
	Blockers      *string        `yaml:"blockers,omitempty"`
	FilesModified []string       `yaml:"files_modified,omitempty"`
	NextSteps     []string       `yaml:"next_steps,omitempty"`
	Extra         map[string]any `yaml:",inline"`
}

// HumanNote represents a note from a human to agents
type HumanNote struct {
	Timestamp time.Time      `yaml:"timestamp"`
	Message   string         `yaml:"message"`
	For       string         `yaml:"for"`
	Extra     map[string]any `yaml:",inline"`
}

// SpecChange tracks modifications to specification documents
type SpecChange struct {
	Timestamp   time.Time      `yaml:"timestamp"`
	Spec        string         `yaml:"spec"`
	Change      string         `yaml:"change"`
	TriggeredBy string         `yaml:"triggered_by"`
	Extra       map[string]any `yaml:",inline"`
}

// Anomaly represents an execution anomaly that may trigger circuit breaker
type Anomaly struct {
	Timestamp time.Time      `yaml:"timestamp"`
	Task      string         `yaml:"task"`
	Reporter  string         `yaml:"reporter"`
	Type      string         `yaml:"type"`
	Details   map[string]any `yaml:"details"`
	Extra     map[string]any `yaml:",inline"`
}

// IsValidType checks if the anomaly type is valid
func (a *Anomaly) IsValidType() bool {
	validTypes := []string{
		"retry_loop", "trade_off", "spec_ambiguity", "external_blocker",
		"assumption_violated", "scope_deviation", "workaround", "debt_created",
		"spec_changed", "hypothesis_exhaustion", "spec_gap", "review_budget_exhausted",
		"review_exhaustion", "reviewer_loop", "system_ambiguity",
	}
	return slices.Contains(validTypes, a.Type)
}

// CircuitBreaker tracks circuit breaker status and history
type CircuitBreaker struct {
	LastCheck      time.Time               `yaml:"last_check"`
	Status         string                  `yaml:"status"` // "OK" or "TRIGGERED"
	CurrentTrigger *CircuitBreakerTrigger  `yaml:"current_trigger,omitempty"`
	History        []CircuitBreakerHistory `yaml:"history"`
	Extra          map[string]any          `yaml:",inline"`
}

// IsValidStatus checks if the circuit breaker status is valid
func (cb *CircuitBreaker) IsValidStatus() bool {
	return cb.Status == "OK" || cb.Status == "TRIGGERED"
}

// CircuitBreakerTrigger represents an active circuit breaker trigger
type CircuitBreakerTrigger struct {
	Timestamp  time.Time      `yaml:"timestamp"`
	Pattern    string         `yaml:"pattern"`
	Severity   string         `yaml:"severity"`
	ReportFile string         `yaml:"report_file"`
	Extra      map[string]any `yaml:",inline"`
}

// CircuitBreakerHistory tracks historical circuit breaker checks
type CircuitBreakerHistory struct {
	Timestamp  time.Time      `yaml:"timestamp"`
	Pattern    *string        `yaml:"pattern,omitempty"`
	Severity   *string        `yaml:"severity,omitempty"`
	Result     string         `yaml:"result"` // "OK" or "TRIGGERED"
	Resolution *string        `yaml:"resolution,omitempty"`
	ResolvedAt *time.Time     `yaml:"resolved_at,omitempty"`
	Extra      map[string]any `yaml:",inline"`
}
