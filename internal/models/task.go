package models

import (
	"fmt"
	"slices"
	"time"

	"github.com/liza-mas/liza/internal/roles"
)

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

// EffectiveType returns the task's type, defaulting to TaskTypeCoding when empty (backward compat).
func (t *Task) EffectiveType() TaskType {
	if t.Type == "" {
		return TaskTypeCoding
	}
	return t.Type
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
	return task.Status == TaskStatusImplementing || task.Status == TaskStatusCodePlanning
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

// TopPriorityTier returns all candidates that share the highest priority
// (lowest number). Returns nil if candidates is empty.
func TopPriorityTier(candidates []*Task) []*Task {
	if len(candidates) == 0 {
		return nil
	}

	bestPriority := candidates[0].Priority
	for _, t := range candidates[1:] {
		if t.Priority < bestPriority {
			bestPriority = t.Priority
		}
	}

	var tier []*Task
	for _, t := range candidates {
		if t.Priority == bestPriority {
			tier = append(tier, t)
		}
	}

	return tier
}
