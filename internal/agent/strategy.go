package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/roles"
)

// RoleStrategy encapsulates all role-specific behavior in the supervisor loop.
type RoleStrategy interface {
	// DefaultTimeout returns the execution timeout when SupervisorConfig.ExecutionTimeout is unset.
	DefaultTimeout() time.Duration

	// WaitConfig returns the poll interval and max wait for this role,
	// resolved from state configuration with role-appropriate defaults.
	WaitConfig(state *models.State) (pollInterval, maxWait time.Duration)

	// PreWork runs actions before waiting for work (e.g., merge handling for reviewers).
	// Returns shouldContinue=true to restart the loop iteration (e.g., pending merge retry).
	PreWork(ctx context.Context, bb *db.Blackboard, config SupervisorConfig) (shouldContinue bool, err error)

	// WaitForWork blocks until role-specific work is available.
	WaitForWork(ctx context.Context, bb *db.Blackboard, config SupervisorConfig, pollInterval, maxWait time.Duration) (bool, error)

	// ClaimTask claims a task for execution.
	// Returns (taskID, claimedTaskID, err). Orchestrator returns ("", "", nil).
	ClaimTask(config SupervisorConfig, bb *db.Blackboard) (taskID, claimedTaskID string, err error)

	// PreExecution runs setup after claiming but before execution (e.g., orchestrator PLANNING status).
	PreExecution(bb *db.Blackboard, config SupervisorConfig) error

	// BuildPrompt constructs the role-specific prompt.
	BuildPrompt(state *models.State, config SupervisorConfig, taskID string) (string, error)

	// PostExecution runs actions after exit code 0 (e.g., submission logging, state verification).
	PostExecution(bb *db.Blackboard, config SupervisorConfig, taskID, claimedTaskID string, stateBefore *models.State) error
}

// NewRoleStrategy creates the appropriate strategy for the given runtime role.
func NewRoleStrategy(role string) (RoleStrategy, error) {
	workflowRole, err := roles.ToWorkflow(role)
	if err != nil {
		return nil, fmt.Errorf("unknown role %q: %w", role, err)
	}

	switch role {
	// Doer roles
	case roles.RuntimeCoder:
		return &doerStrategy{role: role, workflowRole: workflowRole, buildContext: coderContext}, nil
	case roles.RuntimeCodePlanner:
		return &doerStrategy{role: role, workflowRole: workflowRole, buildContext: codePlannerContext}, nil
	case roles.RuntimeEpicPlanner:
		return &doerStrategy{role: role, workflowRole: workflowRole, buildContext: epicPlannerContext}, nil
	case roles.RuntimeUSWriter:
		return &doerStrategy{role: role, workflowRole: workflowRole, buildContext: usWriterContext}, nil

	// Reviewer roles
	case roles.RuntimeCodeReviewer:
		return &reviewerStrategy{role: role, workflowRole: workflowRole, buildContext: codeReviewerContext}, nil
	case roles.RuntimeCodePlanReviewer:
		return &reviewerStrategy{role: role, workflowRole: workflowRole, buildContext: codePlanReviewerContext}, nil
	case roles.RuntimeEpicPlanReviewer:
		return &reviewerStrategy{role: role, workflowRole: workflowRole, buildContext: epicPlanReviewerContext}, nil
	case roles.RuntimeUSReviewer:
		return &reviewerStrategy{role: role, workflowRole: workflowRole, buildContext: usReviewerContext}, nil

	// Orchestrator
	case roles.RuntimeOrchestrator:
		return &orchestratorStrategy{}, nil

	default:
		return nil, fmt.Errorf("no strategy for role %q", role)
	}
}
