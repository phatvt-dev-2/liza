package models

import (
	"slices"
	"time"
)

// State represents the complete Liza state.yaml structure
type State struct {
	Version        int                    `yaml:"version"`
	Goal           Goal                   `yaml:"goal"`
	Tasks          []Task                 `yaml:"tasks"`
	Agents         map[string]Agent       `yaml:"agents"`
	Discovered     []Discovery            `yaml:"discovered"`
	Handoff        map[string]HandoffNote `yaml:"handoff"`
	HumanNotes     []HumanNote            `yaml:"human_notes"`
	SpecChanges    []SpecChange           `yaml:"spec_changes"`
	Anomalies      []Anomaly              `yaml:"anomalies"`
	Sprint         Sprint                 `yaml:"sprint"`
	CircuitBreaker CircuitBreaker         `yaml:"circuit_breaker"`
	Config         Config                 `yaml:"config"`
}

// TaskStatus represents the state of a task
type TaskStatus string

const (
	TaskStatusDraft             TaskStatus = "DRAFT"
	TaskStatusUnclaimed         TaskStatus = "UNCLAIMED"
	TaskStatusClaimed           TaskStatus = "CLAIMED"
	TaskStatusReadyForReview    TaskStatus = "READY_FOR_REVIEW"
	TaskStatusRejected          TaskStatus = "REJECTED"
	TaskStatusApproved          TaskStatus = "APPROVED"
	TaskStatusMerged            TaskStatus = "MERGED"
	TaskStatusBlocked           TaskStatus = "BLOCKED"
	TaskStatusAbandoned         TaskStatus = "ABANDONED"
	TaskStatusSuperseded        TaskStatus = "SUPERSEDED"
	TaskStatusIntegrationFailed TaskStatus = "INTEGRATION_FAILED"
)

// IsValid checks if the task status is valid
func (ts TaskStatus) IsValid() bool {
	switch ts {
	case TaskStatusDraft, TaskStatusUnclaimed, TaskStatusClaimed,
		TaskStatusReadyForReview, TaskStatusRejected, TaskStatusApproved,
		TaskStatusMerged, TaskStatusBlocked, TaskStatusAbandoned,
		TaskStatusSuperseded, TaskStatusIntegrationFailed:
		return true
	}
	return false
}

// IsTerminal checks if the task status is terminal (no further transitions)
func (ts TaskStatus) IsTerminal() bool {
	return ts == TaskStatusMerged || ts == TaskStatusAbandoned || ts == TaskStatusSuperseded
}

// Task represents a single task in the Liza system
type Task struct {
	ID                  string             `yaml:"id"`
	Description         string             `yaml:"description"`
	Status              TaskStatus         `yaml:"status"`
	Priority            int                `yaml:"priority"`
	AssignedTo          *string            `yaml:"assigned_to,omitempty"`
	Worktree            *string            `yaml:"worktree,omitempty"`
	BaseCommit          *string            `yaml:"base_commit,omitempty"`
	Iteration           int                `yaml:"iteration,omitempty"`
	ReviewCyclesCurrent int                `yaml:"review_cycles_current,omitempty"`
	ReviewCyclesTotal   int                `yaml:"review_cycles_total,omitempty"`
	ReviewCommit        *string            `yaml:"review_commit,omitempty"`
	ReviewingBy         *string            `yaml:"reviewing_by,omitempty"`
	ReviewLeaseExpires  *time.Time         `yaml:"review_lease_expires,omitempty"`
	ApprovedBy          *string            `yaml:"approved_by,omitempty"`
	MergeCommit         *string            `yaml:"merge_commit,omitempty"`
	LeaseExpires        *time.Time         `yaml:"lease_expires,omitempty"`
	SpecRef             string             `yaml:"spec_ref"`
	DoneWhen            string             `yaml:"done_when"`
	Scope               string             `yaml:"scope"`
	RejectionReason     *string            `yaml:"rejection_reason,omitempty"`
	BlockedReason       *string            `yaml:"blocked_reason,omitempty"`
	BlockedQuestions    []string           `yaml:"blocked_questions,omitempty"`
	Attempted           []string           `yaml:"attempted,omitempty"`
	SupersededBy        []string           `yaml:"superseded_by,omitempty"`
	Supersedes          *string            `yaml:"supersedes,omitempty"`
	RescopeReason       *string            `yaml:"rescope_reason,omitempty"`
	FailedBy            []string           `yaml:"failed_by,omitempty"`
	DependsOn           []string           `yaml:"depends_on,omitempty"`
	IntegrationFix      bool               `yaml:"integration_fix,omitempty"`
	HandoffPending      bool               `yaml:"handoff_pending,omitempty"`
	MaxIterations       int                `yaml:"max_iterations,omitempty"`
	Created             time.Time          `yaml:"created"`
	History             []TaskHistoryEntry `yaml:"history"`
}

// IsClaimable checks if a task is claimable based on its status and dependencies
func (t *Task) IsClaimable(allTasks []Task) bool {
	// Check if status allows claiming
	if t.Status != TaskStatusUnclaimed &&
		t.Status != TaskStatusRejected &&
		t.Status != TaskStatusIntegrationFailed {
		return false
	}

	// Check dependencies if allTasks is provided
	if allTasks != nil && len(t.DependsOn) > 0 {
		for _, depID := range t.DependsOn {
			depSatisfied := false
			for _, task := range allTasks {
				if task.ID == depID && task.Status == TaskStatusMerged {
					depSatisfied = true
					break
				}
			}
			if !depSatisfied {
				return false
			}
		}
	}

	return true
}

// TaskHistoryEntry represents a single event in a task's history
type TaskHistoryEntry struct {
	Time             time.Time `yaml:"time"`
	Event            string    `yaml:"event"`
	Agent            *string   `yaml:"agent,omitempty"`
	PreviousAssignee *string   `yaml:"previous_assignee,omitempty"`
	Reason           *string   `yaml:"reason,omitempty"`
	Commit           *string   `yaml:"commit,omitempty"`
	Note             *string   `yaml:"note,omitempty"`
}

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

// ReleaseAgent resets an agent to idle with no task assignment.
func (s *State) ReleaseAgent(agentID string) {
	if agent, ok := s.Agents[agentID]; ok {
		agent.Status = AgentStatusIdle
		agent.CurrentTask = nil
		agent.LeaseExpires = nil
		s.Agents[agentID] = agent
	}
}

// Agent represents an agent (coder, reviewer, planner) in the system
type Agent struct {
	Role            string      `yaml:"role"`
	Status          AgentStatus `yaml:"status"`
	CurrentTask     *string     `yaml:"current_task,omitempty"`
	LeaseExpires    *time.Time  `yaml:"lease_expires,omitempty"`
	Heartbeat       time.Time   `yaml:"heartbeat"`
	Terminal        string      `yaml:"terminal"`
	IterationsTotal int         `yaml:"iterations_total"`
	ContextPercent  int         `yaml:"context_percent"`
	PID             int         `yaml:"pid,omitempty"`
}

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
	Created          time.Time          `yaml:"created"`
	Status           GoalStatus         `yaml:"status"`
	AlignmentHistory []AlignmentHistory `yaml:"alignment_history"`
}

// AlignmentHistory tracks goal alignment events
type AlignmentHistory struct {
	Timestamp time.Time `yaml:"timestamp"`
	Event     string    `yaml:"event"`
	Summary   string    `yaml:"summary"`
}

// SprintStatus represents the state of a sprint
type SprintStatus string

const (
	SprintStatusInProgress SprintStatus = "IN_PROGRESS"
	SprintStatusCheckpoint SprintStatus = "CHECKPOINT"
	SprintStatusCompleted  SprintStatus = "COMPLETED"
	SprintStatusAborted    SprintStatus = "ABORTED"
)

// IsValid checks if the sprint status is valid
func (ss SprintStatus) IsValid() bool {
	switch ss {
	case SprintStatusInProgress, SprintStatusCheckpoint, SprintStatusCompleted, SprintStatusAborted:
		return true
	}
	return false
}

// Sprint represents a sprint with scope, timeline, and metrics
type Sprint struct {
	ID            string         `yaml:"id"`
	GoalRef       string         `yaml:"goal_ref"`
	Scope         SprintScope    `yaml:"scope"`
	Timeline      SprintTimeline `yaml:"timeline"`
	Status        SprintStatus   `yaml:"status"`
	Metrics       SprintMetrics  `yaml:"metrics"`
	Retrospective *string        `yaml:"retrospective,omitempty"`
}

// SprintScope defines planned and stretch tasks
type SprintScope struct {
	Planned []string `yaml:"planned"`
	Stretch []string `yaml:"stretch"`
}

// SprintTimeline defines sprint timing
type SprintTimeline struct {
	Started      time.Time  `yaml:"started"`
	Deadline     time.Time  `yaml:"deadline"`
	CheckpointAt *time.Time `yaml:"checkpoint_at,omitempty"`
	Ended        *time.Time `yaml:"ended,omitempty"`
}

// SprintMetrics tracks sprint progress and quality
type SprintMetrics struct {
	TasksDone                        int `yaml:"tasks_done"`
	TasksInProgress                  int `yaml:"tasks_in_progress"`
	TasksBlocked                     int `yaml:"tasks_blocked"`
	IterationsTotal                  int `yaml:"iterations_total"`
	ReviewCyclesTotal                int `yaml:"review_cycles_total"`
	ReviewVerdictApprovals           int `yaml:"review_verdict_approvals"`
	ReviewVerdictRejections          int `yaml:"review_verdict_rejections"`
	ReviewVerdictCount               int `yaml:"review_verdict_count"`
	ReviewVerdictApprovalRatePercent int `yaml:"review_verdict_approval_rate_percent"`
	TaskSubmittedForReviewCount      int `yaml:"task_submitted_for_review_count"`
	TaskOutcomeApprovalRatePercent   int `yaml:"task_outcome_approval_rate_percent"`
}

// Discovery represents a finding by an agent during work
type Discovery struct {
	ID              string    `yaml:"id"`
	By              string    `yaml:"by"`
	During          string    `yaml:"during"`
	Description     string    `yaml:"description"`
	Severity        string    `yaml:"severity"`
	Urgency         string    `yaml:"urgency"`
	Recommendation  string    `yaml:"recommendation"`
	Created         time.Time `yaml:"created"`
	ConvertedToTask *string   `yaml:"converted_to_task,omitempty"`
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
	Agent         string    `yaml:"agent"`
	ContextUsed   int       `yaml:"context_used"`
	Timestamp     time.Time `yaml:"timestamp"`
	Summary       string    `yaml:"summary"`
	NextAction    string    `yaml:"next_action"`
	Approach      *string   `yaml:"approach,omitempty"`
	Blockers      *string   `yaml:"blockers,omitempty"`
	FilesModified []string  `yaml:"files_modified,omitempty"`
	NextSteps     []string  `yaml:"next_steps,omitempty"`
}

// HumanNote represents a note from a human to agents
type HumanNote struct {
	Timestamp time.Time `yaml:"timestamp"`
	Message   string    `yaml:"message"`
	For       string    `yaml:"for"`
}

// SpecChange tracks modifications to specification documents
type SpecChange struct {
	Timestamp   time.Time `yaml:"timestamp"`
	Spec        string    `yaml:"spec"`
	Change      string    `yaml:"change"`
	TriggeredBy string    `yaml:"triggered_by"`
}

// Anomaly represents an execution anomaly that may trigger circuit breaker
type Anomaly struct {
	Timestamp time.Time      `yaml:"timestamp"`
	Task      string         `yaml:"task"`
	Reporter  string         `yaml:"reporter"`
	Type      string         `yaml:"type"`
	Details   map[string]any `yaml:"details"`
}

// IsValidType checks if the anomaly type is valid
func (a *Anomaly) IsValidType() bool {
	validTypes := []string{
		"retry_loop", "trade_off", "spec_ambiguity", "external_blocker",
		"assumption_violated", "scope_deviation", "workaround", "debt_created",
		"spec_changed", "hypothesis_exhaustion", "spec_gap", "review_deadlock",
		"system_ambiguity",
	}
	return slices.Contains(validTypes, a.Type)
}

// CircuitBreaker tracks circuit breaker status and history
type CircuitBreaker struct {
	LastCheck      time.Time               `yaml:"last_check"`
	Status         string                  `yaml:"status"` // "OK" or "TRIGGERED"
	CurrentTrigger *CircuitBreakerTrigger  `yaml:"current_trigger,omitempty"`
	History        []CircuitBreakerHistory `yaml:"history"`
}

// IsValidStatus checks if the circuit breaker status is valid
func (cb *CircuitBreaker) IsValidStatus() bool {
	return cb.Status == "OK" || cb.Status == "TRIGGERED"
}

// CircuitBreakerTrigger represents an active circuit breaker trigger
type CircuitBreakerTrigger struct {
	Timestamp  time.Time `yaml:"timestamp"`
	Pattern    string    `yaml:"pattern"`
	Severity   string    `yaml:"severity"`
	ReportFile string    `yaml:"report_file"`
}

// CircuitBreakerHistory tracks historical circuit breaker checks
type CircuitBreakerHistory struct {
	Timestamp  time.Time  `yaml:"timestamp"`
	Pattern    *string    `yaml:"pattern,omitempty"`
	Severity   *string    `yaml:"severity,omitempty"`
	Result     string     `yaml:"result"` // "OK" or "TRIGGERED"
	Resolution *string    `yaml:"resolution,omitempty"`
	ResolvedAt *time.Time `yaml:"resolved_at,omitempty"`
}

// SystemMode represents the operational mode of the Liza system
type SystemMode string

const (
	SystemModeRunning               SystemMode = "RUNNING"
	SystemModePaused                SystemMode = "PAUSED"
	SystemModeStopped               SystemMode = "STOPPED"
	SystemModeCircuitBreakerTripped SystemMode = "CIRCUIT_BREAKER_TRIPPED"
)

// IsValid checks if the system mode is valid
func (sm SystemMode) IsValid() bool {
	return sm == SystemModeRunning || sm == SystemModePaused || sm == SystemModeStopped || sm == SystemModeCircuitBreakerTripped
}

// Config holds system configuration parameters
type Config struct {
	MaxCoderIterations   int        `yaml:"max_coder_iterations"`
	MaxReviewCycles      int        `yaml:"max_review_cycles"`
	HeartbeatInterval    int        `yaml:"heartbeat_interval"`
	LeaseDuration        int        `yaml:"lease_duration"`
	CoderPollInterval    int        `yaml:"coder_poll_interval"`
	CoderMaxWait         int        `yaml:"coder_max_wait"`
	PlannerPollInterval  int        `yaml:"planner_poll_interval"`
	PlannerMaxWait       int        `yaml:"planner_max_wait"`
	ReviewerPollInterval int        `yaml:"reviewer_poll_interval"`
	ReviewerMaxWait      int        `yaml:"reviewer_max_wait"`
	IntegrationBranch    string     `yaml:"integration_branch"`
	EscalationWebhook    *string    `yaml:"escalation_webhook,omitempty"`
	Mode                 SystemMode `yaml:"mode,omitempty"`
	ModeChangedAt        *time.Time `yaml:"mode_changed_at,omitempty"`
	ModeChangedBy        *string    `yaml:"mode_changed_by,omitempty"`
	DiagnosticLogging    bool       `yaml:"diagnostic_logging,omitempty"`
}
