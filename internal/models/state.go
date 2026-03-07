package models

import (
	"fmt"
	"slices"
	"time"

	"github.com/liza-mas/liza/internal/roles"
)

// State represents the complete Liza state.yaml structure
type State struct {
	Version         int                    `yaml:"version"`
	PipelineVersion int                    `yaml:"pipeline_version,omitempty"`
	Goal            Goal                   `yaml:"goal"`
	Tasks           []Task                 `yaml:"tasks"`
	Agents          map[string]Agent       `yaml:"agents"`
	Discovered      []Discovery            `yaml:"discovered"`
	Handoff         map[string]HandoffNote `yaml:"handoff"`
	HumanNotes      []HumanNote            `yaml:"human_notes"`
	SpecChanges     []SpecChange           `yaml:"spec_changes"`
	Anomalies       []Anomaly              `yaml:"anomalies"`
	Sprint          Sprint                 `yaml:"sprint"`
	SprintHistory   []SprintSummary        `yaml:"sprint_history,omitempty"`
	CircuitBreaker  CircuitBreaker         `yaml:"circuit_breaker"`
	Config          Config                 `yaml:"config"`
	Extra           map[string]any         `yaml:",inline"`
}

// TaskType represents the kind of task, determining which roles participate in its lifecycle.
type TaskType string

const (
	TaskTypeCoding TaskType = "coding"
)

// Role name constants used in task workflow definitions.
// These are aliases for the canonical definitions in the roles package.
const (
	RoleCoder            = roles.WorkflowCoder
	RoleCodeReviewer     = roles.WorkflowCodeReviewer
	RoleOrchestrator     = roles.WorkflowOrchestrator
	RoleCodePlanner      = roles.WorkflowCodePlanner
	RoleCodePlanReviewer = roles.WorkflowCodePlanReviewer
	RoleEpicPlanner      = roles.WorkflowEpicPlanner
	RoleEpicPlanReviewer = roles.WorkflowEpicPlanReviewer
	RoleUSWriter         = roles.WorkflowUSWriter
	RoleUSReviewer       = roles.WorkflowUSReviewer
)

// taskWorkflows maps each TaskType to its ordered role sequence.
// This is the single source of truth for which roles participate in a task type's lifecycle.
// Access via RoleWorkflow(), HasRole(), and IsValid() — not directly.
var taskWorkflows = map[TaskType][]string{
	TaskTypeCoding: {RoleCoder, RoleCodeReviewer},
}

// IsValid checks if the task type is known.
func (tt TaskType) IsValid() bool {
	_, ok := taskWorkflows[tt]
	return ok
}

// ValidTaskTypeNames returns sorted names of all valid task types.
func ValidTaskTypeNames() []string {
	names := make([]string, 0, len(taskWorkflows))
	for tt := range taskWorkflows {
		names = append(names, string(tt))
	}
	slices.Sort(names)
	return names
}

// RoleWorkflow returns a copy of the ordered role sequence for this task type.
func (tt TaskType) RoleWorkflow() []string {
	wf := taskWorkflows[tt]
	if wf == nil {
		return nil
	}
	out := make([]string, len(wf))
	copy(out, wf)
	return out
}

// HasRole checks if the given role participates in this task type's workflow.
func (tt TaskType) HasRole(role string) bool {
	return slices.Contains(taskWorkflows[tt], role)
}

// TaskStatus represents the state of a task
type TaskStatus string

const (
	TaskStatusDraft             TaskStatus = "DRAFT"
	TaskStatusReady             TaskStatus = "READY"
	TaskStatusImplementing      TaskStatus = "IMPLEMENTING"
	TaskStatusReadyForReview    TaskStatus = "READY_FOR_REVIEW"
	TaskStatusReviewing         TaskStatus = "REVIEWING"
	TaskStatusRejected          TaskStatus = "REJECTED"
	TaskStatusApproved          TaskStatus = "APPROVED"
	TaskStatusMerged            TaskStatus = "MERGED"
	TaskStatusBlocked           TaskStatus = "BLOCKED"
	TaskStatusAbandoned         TaskStatus = "ABANDONED"
	TaskStatusSuperseded        TaskStatus = "SUPERSEDED"
	TaskStatusIntegrationFailed TaskStatus = "INTEGRATION_FAILED"

	// Code-planning pair states
	TaskStatusDraftCodingPlan     TaskStatus = "DRAFT_CODING_PLAN"
	TaskStatusCodePlanning        TaskStatus = "CODE_PLANNING"
	TaskStatusCodingPlanToReview  TaskStatus = "CODING_PLAN_TO_REVIEW"
	TaskStatusReviewingCodingPlan TaskStatus = "REVIEWING_CODING_PLAN"
	TaskStatusCodingPlanApproved  TaskStatus = "CODING_PLAN_APPROVED"
	TaskStatusCodingPlanRejected  TaskStatus = "CODING_PLAN_REJECTED"
)

// IsValid checks if the task status is valid
func (ts TaskStatus) IsValid() bool {
	switch ts {
	case TaskStatusDraft, TaskStatusReady, TaskStatusImplementing,
		TaskStatusReadyForReview, TaskStatusReviewing, TaskStatusRejected,
		TaskStatusApproved, TaskStatusMerged, TaskStatusBlocked,
		TaskStatusAbandoned, TaskStatusSuperseded, TaskStatusIntegrationFailed,
		TaskStatusDraftCodingPlan, TaskStatusCodePlanning,
		TaskStatusCodingPlanToReview, TaskStatusReviewingCodingPlan,
		TaskStatusCodingPlanApproved, TaskStatusCodingPlanRejected:
		return true
	}
	return false
}

// IsTerminal checks if the task status is terminal (no further transitions)
func (ts TaskStatus) IsTerminal() bool {
	return ts == TaskStatusMerged || ts == TaskStatusAbandoned || ts == TaskStatusSuperseded
}

// taskTransitions defines the complete, explicit task state machine.
// Every valid status transition is declared here. Terminal states have empty target lists.
var taskTransitions = map[TaskStatus][]TaskStatus{
	TaskStatusDraft:             {TaskStatusReady, TaskStatusAbandoned},
	TaskStatusReady:             {TaskStatusImplementing, TaskStatusSuperseded, TaskStatusAbandoned},
	TaskStatusImplementing:      {TaskStatusReadyForReview, TaskStatusBlocked, TaskStatusReady, TaskStatusIntegrationFailed},
	TaskStatusReadyForReview:    {TaskStatusReviewing},
	TaskStatusReviewing:         {TaskStatusApproved, TaskStatusRejected, TaskStatusReadyForReview},
	TaskStatusRejected:          {TaskStatusImplementing, TaskStatusBlocked, TaskStatusSuperseded, TaskStatusAbandoned},
	TaskStatusApproved:          {TaskStatusMerged, TaskStatusIntegrationFailed},
	TaskStatusBlocked:           {TaskStatusSuperseded, TaskStatusAbandoned},
	TaskStatusIntegrationFailed: {TaskStatusImplementing, TaskStatusAbandoned},
	TaskStatusMerged:            {},
	TaskStatusAbandoned:         {},
	TaskStatusSuperseded:        {},

	// Code-planning pair transitions
	TaskStatusDraftCodingPlan:     {TaskStatusCodePlanning, TaskStatusAbandoned},
	TaskStatusCodePlanning:        {TaskStatusCodingPlanToReview, TaskStatusBlocked, TaskStatusDraftCodingPlan, TaskStatusIntegrationFailed},
	TaskStatusCodingPlanToReview:  {TaskStatusReviewingCodingPlan},
	TaskStatusReviewingCodingPlan: {TaskStatusCodingPlanApproved, TaskStatusCodingPlanRejected, TaskStatusCodingPlanToReview},
	TaskStatusCodingPlanRejected:  {TaskStatusDraftCodingPlan, TaskStatusBlocked, TaskStatusSuperseded, TaskStatusAbandoned},
	TaskStatusCodingPlanApproved:  {TaskStatusMerged, TaskStatusIntegrationFailed},
}

// CanTransition reports whether a transition from ts to the given target status is valid.
func (ts TaskStatus) CanTransition(to TaskStatus) bool {
	return slices.Contains(taskTransitions[ts], to)
}

// Transition validates and applies a status transition on the task.
// Returns a descriptive error if the transition is invalid.
func (t *Task) Transition(to TaskStatus) error {
	if !t.Status.CanTransition(to) {
		return fmt.Errorf("invalid task transition: %s → %s (task %s)", t.Status, to, t.ID)
	}
	t.Status = to
	return nil
}

// TransitionWith validates and applies a status transition using a custom transition map.
// This supports pipeline-defined states that aren't in the hardcoded transition map.
// The target status must exist as a key in the transition map (i.e., be a declared state).
func (t *Task) TransitionWith(to TaskStatus, transitions map[TaskStatus][]TaskStatus) error {
	if !slices.Contains(transitions[t.Status], to) {
		return fmt.Errorf("invalid task transition: %s → %s (task %s)", t.Status, to, t.ID)
	}
	if _, declared := transitions[to]; !declared {
		return fmt.Errorf("target status %s is not a declared state in the transition map (task %s)", to, t.ID)
	}
	t.Status = to
	return nil
}

// OutputEntry represents a structured subtask definition produced by a doer role.
// When a task completes with output[], each entry defines a downstream child task.
type OutputEntry struct {
	Desc     string `yaml:"desc"`
	DoneWhen string `yaml:"done_when"`
	Scope    string `yaml:"scope"`
	SpecRef  string `yaml:"spec_ref"`
}

// PipelineResolver provides pipeline state resolution for tasks with role-pairs.
// Implemented by pipeline.Resolver. Pass nil for legacy projects.
type PipelineResolver interface {
	DoerRole(rolePair string) (string, error)
	ReviewerRole(rolePair string) (string, error)
	InitialStatus(rolePair string) (TaskStatus, error)
	RejectedStatus(rolePair string) (TaskStatus, error)
	SubmittedStatus(rolePair string) (TaskStatus, error)
	ReviewingStatus(rolePair string) (TaskStatus, error)
	ExecutingStatus(rolePair string) (TaskStatus, error)
	ApprovedStatus(rolePair string) (TaskStatus, error)
}

// IsApprovedForMerge checks if a task is in an approved state eligible for merge.
// Uses the pipeline resolver for pipeline tasks (non-empty RolePair); falls back
// to legacy statuses (APPROVED, CODING_PLAN_APPROVED) otherwise.
func IsApprovedForMerge(task *Task, pr PipelineResolver) bool {
	if task.RolePair != "" && pr != nil {
		approved, err := pr.ApprovedStatus(task.RolePair)
		return err == nil && task.Status == approved
	}
	return task.Status == TaskStatusApproved || task.Status == TaskStatusCodingPlanApproved
}

// IsSubmittedStatus checks if a task is in a submitted state (pipeline-aware).
func IsSubmittedStatus(task *Task, pr PipelineResolver) bool {
	if task.RolePair != "" && pr != nil {
		submitted, err := pr.SubmittedStatus(task.RolePair)
		return err == nil && task.Status == submitted
	}
	return task.Status == TaskStatusReadyForReview || task.Status == TaskStatusCodingPlanToReview
}

// IsExecutingStatus checks if a task is in an executing state (pipeline-aware).
func IsExecutingStatus(task *Task, pr PipelineResolver) bool {
	if task.RolePair != "" && pr != nil {
		executing, err := pr.ExecutingStatus(task.RolePair)
		return err == nil && task.Status == executing
	}
	return task.Status == TaskStatusImplementing
}

// Task represents a single task in the Liza system
type Task struct {
	ID                  string             `yaml:"id"`
	Type                TaskType           `yaml:"type,omitempty"`
	RolePair            string             `yaml:"role_pair,omitempty"`
	Description         string             `yaml:"description"`
	Status              TaskStatus         `yaml:"status"`
	Priority            int                `yaml:"priority"`
	AssignedTo          *string            `yaml:"assigned_to,omitempty"`
	Worktree            *string            `yaml:"worktree,omitempty"`
	BaseCommit          *string            `yaml:"base_commit,omitempty"`
	Iteration           int                `yaml:"iteration,omitempty"`
	Output              []OutputEntry      `yaml:"output,omitempty"`
	ParentTask          *string            `yaml:"parent_task,omitempty"`
	TransitionsExecuted map[string]bool    `yaml:"transitions_executed,omitempty"`
	Exit42RestartCount  int                `yaml:"exit42_restart_count,omitempty"`
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
	Extra               map[string]any     `yaml:",inline"`
}

// EffectiveType returns the task's type, defaulting to TaskTypeCoding when empty (backward compat).
func (t *Task) EffectiveType() TaskType {
	if t.Type == "" {
		return TaskTypeCoding
	}
	return t.Type
}

// IsClaimable checks if a task is claimable by the given role based on its type, status, and dependencies.
// The role parameter uses workflow form (e.g. "code_reviewer").
// When pr is non-nil and the task has a RolePair, pipeline-defined states are used.
func (t *Task) IsClaimable(role string, allTasks []Task, pr PipelineResolver) bool {
	// Pipeline path: use resolver for tasks with role-pairs.
	if t.RolePair != "" && pr != nil {
		if !t.isClaimablePipeline(role, pr) {
			return false
		}
		return checkDependencies(t, allTasks)
	}

	// Legacy path: hardcoded status checks.
	switch role {
	case RoleCoder:
		if !t.EffectiveType().HasRole(role) {
			return false
		}
		if !t.Status.CanTransition(TaskStatusImplementing) {
			return false
		}
	case RoleCodeReviewer:
		if !t.EffectiveType().HasRole(role) {
			return false
		}
		if !t.Status.CanTransition(TaskStatusReviewing) {
			return false
		}
	case RoleCodePlanner:
		if !t.Status.CanTransition(TaskStatusCodePlanning) {
			return false
		}
	case RoleCodePlanReviewer:
		if !t.Status.CanTransition(TaskStatusReviewingCodingPlan) {
			return false
		}
	case RoleOrchestrator:
		// Orchestrator does not participate in task claiming workflows.
		return false
	default:
		return false
	}

	return checkDependencies(t, allTasks)
}

// isClaimablePipeline checks claimability using pipeline-resolved states.
func (t *Task) isClaimablePipeline(role string, pr PipelineResolver) bool {
	// Convert workflow role to runtime form for comparison with pipeline roles.
	runtimeRole, err := roles.ToRuntime(role)
	if err != nil {
		return false
	}

	doerRole, err := pr.DoerRole(t.RolePair)
	if err != nil {
		return false
	}
	reviewerRole, err := pr.ReviewerRole(t.RolePair)
	if err != nil {
		return false
	}

	switch runtimeRole {
	case doerRole:
		initial, err := pr.InitialStatus(t.RolePair)
		if err != nil {
			return false
		}
		rejected, err := pr.RejectedStatus(t.RolePair)
		if err != nil {
			return false
		}
		return t.Status == initial || t.Status == rejected || t.Status == TaskStatusIntegrationFailed

	case reviewerRole:
		submitted, err := pr.SubmittedStatus(t.RolePair)
		if err != nil {
			return false
		}
		return t.Status == submitted

	default:
		return false
	}
}

// checkDependencies returns true if all dependencies of the task are satisfied (MERGED).
func checkDependencies(t *Task, allTasks []Task) bool {
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
	Time             time.Time      `yaml:"time"`
	Event            string         `yaml:"event"`
	Agent            *string        `yaml:"agent,omitempty"`
	PreviousAssignee *string        `yaml:"previous_assignee,omitempty"`
	Reason           *string        `yaml:"reason,omitempty"`
	Commit           *string        `yaml:"commit,omitempty"`
	Note             *string        `yaml:"note,omitempty"`
	Extra            map[string]any `yaml:",inline"`
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

// FindTask returns a pointer to the task with the given ID, or nil if not found.
// The returned pointer refers to the element within s.Tasks, so mutations are
// reflected in the state (useful inside Blackboard.Modify closures).
func (s *State) FindTask(taskID string) *Task {
	for i := range s.Tasks {
		if s.Tasks[i].ID == taskID {
			return &s.Tasks[i]
		}
	}
	return nil
}

// FindTaskIndex returns the index of the task with the given ID, or -1 if not found.
// Use when you need to remove a task from the slice.
func (s *State) FindTaskIndex(taskID string) int {
	for i := range s.Tasks {
		if s.Tasks[i].ID == taskID {
			return i
		}
	}
	return -1
}

// IsSprintTerminal checks if the task status is terminal for sprint completion purposes.
// MERGED is the universal sprint-terminal state for all role-pairs.
func (ts TaskStatus) IsSprintTerminal() bool {
	return ts.IsTerminal()
}

// IsPipelineValid checks if the status is valid in a pipeline context.
// A status is valid if it is a hardcoded valid status (legacy) OR appears in
// the provided set of pipeline-declared states OR is a cross-cutting meta-state.
func (ts TaskStatus) IsPipelineValid(declaredStates []TaskStatus) bool {
	if ts.IsValid() {
		return true
	}
	return slices.Contains(declaredStates, ts)
}

// CanPipelineTransition checks if a transition from ts to the given target is
// valid using the provided transition map (built from pipeline config).
func (ts TaskStatus) CanPipelineTransition(to TaskStatus, transitions map[TaskStatus][]TaskStatus) bool {
	return slices.Contains(transitions[ts], to)
}

// IsPipelineSprintTerminal checks if the status is terminal for sprint purposes
// using pipeline-defined terminal states. Universal terminals (MERGED, ABANDONED,
// SUPERSEDED) are always considered sprint-terminal.
func (ts TaskStatus) IsPipelineSprintTerminal(terminalStates []TaskStatus) bool {
	return ts.IsTerminal() || slices.Contains(terminalStates, ts)
}

// AllPlannedTasksTerminal returns true if the sprint has planned tasks and all of
// them are in a sprint-terminal state. Returns false if the planned list is empty
// or any planned task is not found/not sprint-terminal.
// Equivalent to AllPlannedTasksTerminalWith(nil).
func (s *State) AllPlannedTasksTerminal() bool {
	return s.AllPlannedTasksTerminalWith(nil)
}

// AllPlannedTasksTerminalWith checks if all planned tasks are sprint-terminal.
// For tasks with a role_pair, uses pipeline-defined terminal states in addition to
// universal terminals (MERGED, ABANDONED, SUPERSEDED). Legacy tasks (no role_pair)
// always use the standard IsSprintTerminal check.
//
// When pipelineTerminals is nil, only universal terminal states are recognized —
// this is the correct fallback for legacy projects with no pipeline config.
func (s *State) AllPlannedTasksTerminalWith(pipelineTerminals []TaskStatus) bool {
	if len(s.Sprint.Scope.Planned) == 0 {
		return false
	}
	for _, taskID := range s.Sprint.Scope.Planned {
		task := s.FindTask(taskID)
		if task == nil {
			return false
		}
		if task.RolePair != "" {
			if !task.Status.IsPipelineSprintTerminal(pipelineTerminals) {
				return false
			}
		} else {
			if !task.Status.IsSprintTerminal() {
				return false
			}
		}
	}
	return true
}

// SprintStalled returns true if the sprint has planned tasks and every planned
// task is either sprint-terminal or BLOCKED, with at least one BLOCKED. This indicates
// no agent can make progress — the sprint is stuck and needs human intervention.
func (s *State) SprintStalled() bool {
	if len(s.Sprint.Scope.Planned) == 0 {
		return false
	}
	hasBlocked := false
	for _, taskID := range s.Sprint.Scope.Planned {
		task := s.FindTask(taskID)
		if task == nil {
			return false
		}
		if task.Status == TaskStatusBlocked {
			hasBlocked = true
		} else if !task.Status.IsSprintTerminal() {
			return false
		}
	}
	return hasBlocked
}

// Agent represents an agent (coder, reviewer, orchestrator) in the system
type Agent struct {
	Role            string         `yaml:"role"`
	Status          AgentStatus    `yaml:"status"`
	CurrentTask     *string        `yaml:"current_task,omitempty"`
	LeaseExpires    *time.Time     `yaml:"lease_expires,omitempty"`
	Heartbeat       time.Time      `yaml:"heartbeat"`
	Terminal        string         `yaml:"terminal"`
	IterationsTotal int            `yaml:"iterations_total"`
	ContextPercent  int            `yaml:"context_percent"`
	PID             int            `yaml:"pid,omitempty"`
	Extra           map[string]any `yaml:",inline"`
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
	Number        int            `yaml:"number"`
	GoalRef       string         `yaml:"goal_ref"`
	Scope         SprintScope    `yaml:"scope"`
	Timeline      SprintTimeline `yaml:"timeline"`
	Status        SprintStatus   `yaml:"status"`
	Metrics       SprintMetrics  `yaml:"metrics"`
	Retrospective *string        `yaml:"retrospective,omitempty"`
	Extra         map[string]any `yaml:",inline"`
}

// SprintSummary is a lightweight record of a completed sprint kept in state.yaml.
// Full sprint data (metrics, scope, retrospective) is archived to .liza/archive/sprint-N.yaml.
type SprintSummary struct {
	ID        string       `yaml:"id"`
	Number    int          `yaml:"number"`
	Status    SprintStatus `yaml:"status"`
	Started   time.Time    `yaml:"started"`
	Ended     time.Time    `yaml:"ended"`
	TasksDone int          `yaml:"tasks_done"`
}

// SprintScope defines planned and stretch tasks
type SprintScope struct {
	Planned []string       `yaml:"planned"`
	Stretch []string       `yaml:"stretch"`
	Extra   map[string]any `yaml:",inline"`
}

// SprintTimeline defines sprint timing
type SprintTimeline struct {
	Started      time.Time      `yaml:"started"`
	Deadline     time.Time      `yaml:"deadline"`
	CheckpointAt *time.Time     `yaml:"checkpoint_at,omitempty"`
	Ended        *time.Time     `yaml:"ended,omitempty"`
	Extra        map[string]any `yaml:",inline"`
}

// SprintMetrics tracks sprint progress and quality
type SprintMetrics struct {
	TasksDone                        int            `yaml:"tasks_done"`
	TasksInProgress                  int            `yaml:"tasks_in_progress"`
	TasksBlocked                     int            `yaml:"tasks_blocked"`
	IterationsTotal                  int            `yaml:"iterations_total"`
	ReviewCyclesTotal                int            `yaml:"review_cycles_total"`
	ReviewVerdictApprovals           int            `yaml:"review_verdict_approvals"`
	ReviewVerdictRejections          int            `yaml:"review_verdict_rejections"`
	ReviewVerdictCount               int            `yaml:"review_verdict_count"`
	ReviewVerdictApprovalRatePercent int            `yaml:"review_verdict_approval_rate_percent"`
	TaskSubmittedForReviewCount      int            `yaml:"task_submitted_for_review_count"`
	TaskOutcomeApprovalRatePercent   int            `yaml:"task_outcome_approval_rate_percent"`
	Extra                            map[string]any `yaml:",inline"`
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
		"spec_changed", "hypothesis_exhaustion", "spec_gap", "review_deadlock",
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

// systemModeTransition defines allowed source modes and rejection messages for a target mode.
type systemModeTransition struct {
	AllowedFrom []SystemMode
	Rejections  map[SystemMode]string
}

// systemModeTransitions declares the valid mode transition graph, keyed by target mode.
// Callers say "transition TO X"; the table says which source modes are valid and what
// error message to return for known-invalid sources.
var systemModeTransitions = map[SystemMode]systemModeTransition{
	SystemModeRunning: {
		AllowedFrom: []SystemMode{SystemModeStopped},
		Rejections: map[SystemMode]string{
			SystemModeRunning: "system is already RUNNING",
			SystemModePaused:  "system is PAUSED - use 'liza resume' instead",
		},
	},
	SystemModeStopped: {
		AllowedFrom: []SystemMode{SystemModeRunning, SystemModePaused, SystemModeCircuitBreakerTripped},
		Rejections: map[SystemMode]string{
			SystemModeStopped: "system is already STOPPED",
		},
	},
	SystemModePaused: {
		AllowedFrom: []SystemMode{SystemModeRunning, SystemModeCircuitBreakerTripped},
		Rejections: map[SystemMode]string{
			SystemModePaused:  "system is already PAUSED",
			SystemModeStopped: "cannot pause: system is STOPPED (use resume only from PAUSED state)",
		},
	},
}

// ValidateTransition checks whether transitioning from sm to the target mode is valid.
// Returns nil if allowed, or a descriptive error for known rejections / unknown sources.
func (sm SystemMode) ValidateTransition(to SystemMode) error {
	tr, ok := systemModeTransitions[to]
	if !ok {
		return fmt.Errorf("unknown target mode: %s", to)
	}

	if msg, rejected := tr.Rejections[sm]; rejected {
		return fmt.Errorf("%s", msg)
	}

	if slices.Contains(tr.AllowedFrom, sm) {
		return nil
	}

	return fmt.Errorf("can only transition to %s from %v (current: %s)", to, tr.AllowedFrom, sm)
}

// Default configuration values (seconds) used as fallbacks when config fields are unset.
const (
	DefaultMaxCoderIterations       = 10
	DefaultMaxReviewCycles          = 5
	DefaultLeaseDurationSeconds     = 1800 // 30 minutes
	DefaultCoderPollInterval        = 30
	DefaultCoderMaxWait             = 1800 // 30 minutes
	DefaultOrchestratorPollInterval = 60
	DefaultOrchestratorMaxWait      = 1800 // 30 minutes
	DefaultReviewerPollInterval     = 30
	DefaultReviewerMaxWait          = 1800 // 30 minutes
	DefaultExit42MaxBackoffSec      = 60
	DefaultExit42RestartLimit       = 5
)

// Bounds for heartbeat interval validation.
const (
	MinHeartbeatIntervalSeconds = 1
	MaxHeartbeatIntervalSeconds = 300 // 5 minutes
	DefaultHeartbeatIntervalSec = 60
)

// NormalizeHeartbeatInterval validates and normalizes a heartbeat interval value.
// Returns the normalized duration or the default if the value is invalid.
// Invalid values: ≤ 0, > MaxHeartbeatIntervalSeconds (300s / 5min)
func NormalizeHeartbeatInterval(interval int) time.Duration {
	if interval <= 0 || interval > MaxHeartbeatIntervalSeconds {
		return DefaultHeartbeatIntervalSec * time.Second
	}
	return time.Duration(interval) * time.Second
}

// Config holds system configuration parameters
type Config struct {
	MaxCoderIterations       int `yaml:"max_coder_iterations"`
	MaxReviewCycles          int `yaml:"max_review_cycles"`
	HeartbeatInterval        int `yaml:"heartbeat_interval"`
	LeaseDuration            int `yaml:"lease_duration"`
	CoderPollInterval        int `yaml:"coder_poll_interval"`
	CoderMaxWait             int `yaml:"coder_max_wait"`
	OrchestratorPollInterval int `yaml:"orchestrator_poll_interval"`
	// OrchestratorMaxWait is the maximum time an orchestrator agent will wait for work
	// before exiting. When 0, defaults to DefaultOrchestratorMaxWait (30 minutes).
	// The orchestrator will exit earlier if STOPPED mode is detected or context is cancelled.
	OrchestratorMaxWait     int            `yaml:"orchestrator_max_wait"`
	ReviewerPollInterval    int            `yaml:"reviewer_poll_interval"`
	ReviewerMaxWait         int            `yaml:"reviewer_max_wait"`
	Exit42RestartThreshold  int            `yaml:"exit42_restart_threshold,omitempty"`
	Exit42MaxBackoffSeconds int            `yaml:"exit42_max_backoff_seconds,omitempty"`
	IntegrationBranch       string         `yaml:"integration_branch"`
	EscalationWebhook       *string        `yaml:"escalation_webhook,omitempty"`
	Mode                    SystemMode     `yaml:"mode,omitempty"`
	ModeChangedAt           *time.Time     `yaml:"mode_changed_at,omitempty"`
	ModeChangedBy           *string        `yaml:"mode_changed_by,omitempty"`
	DiagnosticLogging       bool           `yaml:"diagnostic_logging,omitempty"`
	PostWorktreeCmd         *string        `yaml:"post_worktree_cmd,omitempty"`
	Extra                   map[string]any `yaml:",inline"`
}
