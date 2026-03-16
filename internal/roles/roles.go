// Package roles provides role name constants and mapping between runtime
// role names (used in agent config/CLI) and workflow role names (used in task definitions).
package roles

import (
	"fmt"
)

// Claim-type selectors used by ReleaseClaim to indicate which claim slot to release.
// These are NOT role names — a code-planner releases its claim with ClaimDoer,
// a code-plan-reviewer with ClaimReviewer.
const (
	ClaimDoer     = "doer"
	ClaimReviewer = "reviewer"
	ClaimBoth     = "both"
)

// Workflow role names used in task workflow definitions.
// These are the underscore forms stored in models.
const (
	WorkflowCoder            = "coder"
	WorkflowCodeReviewer     = "code_reviewer"
	WorkflowOrchestrator     = "orchestrator"
	WorkflowCodePlanner      = "code_planner"
	WorkflowCodePlanReviewer = "code_plan_reviewer"
	WorkflowEpicPlanner      = "epic_planner"
	WorkflowEpicPlanReviewer = "epic_plan_reviewer"
	WorkflowUSWriter         = "us_writer"
	WorkflowUSReviewer       = "us_reviewer"
)

// runtimeToWorkflow maps runtime role names to workflow role names.
var runtimeToWorkflow = map[string]string{
	"coder":              WorkflowCoder,
	"code-reviewer":      WorkflowCodeReviewer,
	"orchestrator":       WorkflowOrchestrator,
	"code-planner":       WorkflowCodePlanner,
	"code-plan-reviewer": WorkflowCodePlanReviewer,
	"epic-planner":       WorkflowEpicPlanner,
	"epic-plan-reviewer": WorkflowEpicPlanReviewer,
	"us-writer":          WorkflowUSWriter,
	"us-reviewer":        WorkflowUSReviewer,
}

// workflowToRuntime maps workflow role names to runtime role names.
var workflowToRuntime = map[string]string{
	WorkflowCoder:            "coder",
	WorkflowCodeReviewer:     "code-reviewer",
	WorkflowOrchestrator:     "orchestrator",
	WorkflowCodePlanner:      "code-planner",
	WorkflowCodePlanReviewer: "code-plan-reviewer",
	WorkflowEpicPlanner:      "epic-planner",
	WorkflowEpicPlanReviewer: "epic-plan-reviewer",
	WorkflowUSWriter:         "us-writer",
	WorkflowUSReviewer:       "us-reviewer",
}

// ToWorkflow converts a runtime role name to its workflow equivalent.
// Returns error if the role is not recognized.
func ToWorkflow(runtimeRole string) (string, error) {
	if workflow, ok := runtimeToWorkflow[runtimeRole]; ok {
		return workflow, nil
	}
	return "", fmt.Errorf("unknown runtime role: %s", runtimeRole)
}

// ToRuntime converts a workflow role name to its runtime equivalent.
// Returns error if the role is not recognized.
func ToRuntime(workflowRole string) (string, error) {
	if runtime, ok := workflowToRuntime[workflowRole]; ok {
		return runtime, nil
	}
	return "", fmt.Errorf("unknown workflow role: %s", workflowRole)
}

// IsValidWorkflow checks if the given role is a valid workflow role.
func IsValidWorkflow(role string) bool {
	_, ok := workflowToRuntime[role]
	return ok
}

// AllWorkflow returns all valid workflow role names.
func AllWorkflow() []string {
	return []string{
		WorkflowCoder, WorkflowCodeReviewer, WorkflowOrchestrator,
		WorkflowCodePlanner, WorkflowCodePlanReviewer,
		WorkflowEpicPlanner, WorkflowEpicPlanReviewer,
		WorkflowUSWriter, WorkflowUSReviewer,
	}
}

// NormalizeRoleName converts a known underscore-form role name to its
// canonical hyphenated form. Unknown names are returned unchanged.
func NormalizeRoleName(name string) string {
	if runtime, ok := workflowToRuntime[name]; ok {
		return runtime
	}
	return name
}
