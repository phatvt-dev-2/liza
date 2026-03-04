// Package roles provides role name constants and mapping between runtime
// role names (used in agent config/CLI) and workflow role names (used in task definitions).
package roles

import "fmt"

// Runtime role names used in agent configuration, CLI, and supervisor.
// These are the hyphenated forms that appear in agent IDs like "code-reviewer-1".
const (
	RuntimeCoder            = "coder"
	RuntimeCodeReviewer     = "code-reviewer"
	RuntimeOrchestrator     = "orchestrator"
	RuntimeCodePlanner      = "code-planner"
	RuntimeCodePlanReviewer = "code-plan-reviewer"
)

// Workflow role names used in task workflow definitions.
// These are the underscore forms stored in models.
const (
	WorkflowCoder            = "coder"
	WorkflowCodeReviewer     = "code_reviewer"
	WorkflowOrchestrator     = "orchestrator"
	WorkflowCodePlanner      = "code_planner"
	WorkflowCodePlanReviewer = "code_plan_reviewer"
)

// runtimeToWorkflow maps runtime role names to workflow role names.
var runtimeToWorkflow = map[string]string{
	RuntimeCoder:            WorkflowCoder,
	RuntimeCodeReviewer:     WorkflowCodeReviewer,
	RuntimeOrchestrator:     WorkflowOrchestrator,
	RuntimeCodePlanner:      WorkflowCodePlanner,
	RuntimeCodePlanReviewer: WorkflowCodePlanReviewer,
}

// workflowToRuntime maps workflow role names to runtime role names.
var workflowToRuntime = map[string]string{
	WorkflowCoder:            RuntimeCoder,
	WorkflowCodeReviewer:     RuntimeCodeReviewer,
	WorkflowOrchestrator:     RuntimeOrchestrator,
	WorkflowCodePlanner:      RuntimeCodePlanner,
	WorkflowCodePlanReviewer: RuntimeCodePlanReviewer,
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

// IsValidRuntime checks if the given role is a valid runtime role.
func IsValidRuntime(role string) bool {
	_, ok := runtimeToWorkflow[role]
	return ok
}

// IsValidWorkflow checks if the given role is a valid workflow role.
func IsValidWorkflow(role string) bool {
	_, ok := workflowToRuntime[role]
	return ok
}

// AllRuntime returns all valid runtime role names.
func AllRuntime() []string {
	return []string{RuntimeCoder, RuntimeCodeReviewer, RuntimeOrchestrator, RuntimeCodePlanner, RuntimeCodePlanReviewer}
}

// AllWorkflow returns all valid workflow role names.
func AllWorkflow() []string {
	return []string{WorkflowCoder, WorkflowCodeReviewer, WorkflowOrchestrator, WorkflowCodePlanner, WorkflowCodePlanReviewer}
}
