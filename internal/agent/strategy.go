package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/pipeline"
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

// contextBuilders maps known role names to their prompt context builder functions.
// Roles not in this map (e.g., user-defined YAML roles) get nil buildContext —
// strategy creation succeeds, but BuildPrompt will return an error until
// Phase 2 introduces composable prompt sections.
var contextBuilders = map[string]contextBuilderFunc{
	"coder":              coderContext,
	"code-planner":       codePlannerContext,
	"epic-planner":       epicPlannerContext,
	"us-writer":          usWriterContext,
	"code-reviewer":      codeReviewerContext,
	"code-plan-reviewer": codePlanReviewerContext,
	"epic-plan-reviewer": epicPlanReviewerContext,
	"us-reviewer":        usReviewerContext,
}

// NewRoleStrategy creates the appropriate strategy for the given runtime role.
// The resolver determines the role's type (doer/reviewer/orchestrator) from the
// pipeline YAML, enabling custom YAML-defined roles to get the correct strategy
// without modifying this function.
func NewRoleStrategy(role string, resolver *pipeline.Resolver) (RoleStrategy, error) {
	roleType, err := resolver.RoleType(role)
	if err != nil {
		return nil, fmt.Errorf("unknown role %q: %w", role, err)
	}

	// Derive workflowRole for backward compatibility (Phase 4 removes this).
	// For known roles, use the canonical mapping; for custom roles, use the
	// role name itself as the workflow name.
	workflowRole, wfErr := roles.ToWorkflow(role)
	if wfErr != nil {
		workflowRole = role
	}

	// Look up per-role context builder. nil is acceptable for custom roles —
	// strategy creation succeeds, BuildPrompt will error if called without
	// a registered builder (Phase 2 will add composable prompt sections).
	ctxBuilder := contextBuilders[role]

	switch roleType {
	case "doer":
		return &doerStrategy{role: role, workflowRole: workflowRole, buildContext: ctxBuilder}, nil
	case "reviewer":
		return &reviewerStrategy{role: role, workflowRole: workflowRole, buildContext: ctxBuilder}, nil
	case "orchestrator":
		return &orchestratorStrategy{}, nil
	default:
		return nil, fmt.Errorf("unsupported role type %q for role %q", roleType, role)
	}
}
