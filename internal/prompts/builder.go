package prompts

import (
	"fmt"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
)

// BasePromptConfig contains configuration for building the base prompt
type BasePromptConfig struct {
	Role        string
	AgentID     string
	SpecsDir    string
	ProjectRoot string
	StatePath   string
	GoalDesc    string
	GoalSpecRef string
}

// OrchestratorContextConfig contains configuration for building orchestrator context
type OrchestratorContextConfig struct {
	ProjectRoot string
}

// SiblingTaskSummary provides minimal context about sibling tasks in the same sprint
type SiblingTaskSummary struct {
	ID          string
	Description string
	Status      string
}

// CoderContextConfig contains configuration for building coder context
type CoderContextConfig struct {
	ProjectRoot       string
	AgentID           string
	IntegrationBranch string
	HandoffNote       *models.HandoffNote
	GoalSpecRef       string
	SiblingTasks      []SiblingTaskSummary
	TotalPlanTasks    int
	TaskOrdinal       int // 1-based position in sprint plan
}

// ReviewerContextConfig contains configuration for building reviewer context
type ReviewerContextConfig struct {
	ProjectRoot    string
	AgentID        string
	GoalSpecRef    string
	SiblingTasks   []SiblingTaskSummary
	TotalPlanTasks int
	TaskOrdinal    int // 1-based position in sprint plan
}

// CodePlannerContextConfig contains configuration for building code-planner context
type CodePlannerContextConfig struct {
	ProjectRoot string
	AgentID     string
}

// CodePlanReviewerContextConfig contains configuration for building code-plan-reviewer context
type CodePlanReviewerContextConfig struct {
	ProjectRoot string
	AgentID     string
}

// BuildBasePrompt creates the base bootstrap prompt for all agents
func BuildBasePrompt(config BasePromptConfig) (string, error) {
	return executeTemplate("base_prompt", config)
}

// planningTaskData holds a merged planning task's output for the PLANNING_COMPLETE template.
type planningTaskData struct {
	TaskID string
	Output []models.OutputEntry
}

// orchestratorContextData is the template data for orchestrator_context.tmpl
type orchestratorContextData struct {
	WakeTrigger          string
	SprintNumber         int
	SprintHistory        []models.SprintSummary
	TotalTasks           int
	Merged               int
	InProgress           int
	Unclaimed            int
	Blocked              int
	IntegrationFailed    int
	HypothesisExhausted  int
	ImmediateDiscoveries int
	WakeInstructions     string
}

// BuildOrchestratorContext creates orchestrator-specific context with sprint state
func BuildOrchestratorContext(state *models.State, config OrchestratorContextConfig) (string, error) {
	totalTasks := len(state.Tasks)
	merged := countTasksByStatus(state.Tasks, models.TaskStatusMerged)
	blocked := countTasksByStatus(state.Tasks, models.TaskStatusBlocked)
	integrationFailed := countTasksByStatus(state.Tasks, models.TaskStatusIntegrationFailed)
	unclaimed := countTasksByStatus(state.Tasks, models.TaskStatusReady)

	inProgress := countTasksByStatus(state.Tasks, models.TaskStatusImplementing) +
		countTasksByStatus(state.Tasks, models.TaskStatusReadyForReview) +
		countTasksByStatus(state.Tasks, models.TaskStatusApproved)

	hypothesisExhausted := 0
	for _, task := range state.Tasks {
		if len(task.FailedBy) >= 2 && !task.Status.IsTerminal() {
			hypothesisExhausted++
		}
	}

	immediateDiscoveries := 0
	for _, disc := range state.Discovered {
		if disc.Urgency == "immediate" && disc.ConvertedToTask == nil {
			immediateDiscoveries++
		}
	}

	sprintComplete := state.AllPlannedTasksTerminalWith(ops.SprintTerminalStates(config.ProjectRoot))

	// Collect merged planning tasks with output[] for PLANNING_COMPLETE detection.
	// Only code-planning-pair tasks qualify — coding tasks with output[] are ignored.
	var planningTasks []planningTaskData
	if sprintComplete {
		for _, taskID := range state.Sprint.Scope.Planned {
			task := state.FindTask(taskID)
			if task != nil && task.Status == models.TaskStatusMerged && len(task.Output) > 0 && task.RolePair == "code-planning-pair" {
				planningTasks = append(planningTasks, planningTaskData{
					TaskID: task.ID,
					Output: task.Output,
				})
			}
		}
	}

	wakeTrigger := determineWakeTrigger(totalTasks, blocked, integrationFailed, hypothesisExhausted, immediateDiscoveries, sprintComplete, planningTasks)

	wakeInstructions, err := buildInstructionsForWakeTrigger(wakeTrigger, state.Goal.SpecRef, planningTasks)
	if err != nil {
		return "", fmt.Errorf("building wake instructions: %w", err)
	}

	data := orchestratorContextData{
		WakeTrigger:          wakeTrigger,
		SprintNumber:         state.Sprint.Number,
		SprintHistory:        state.SprintHistory,
		TotalTasks:           totalTasks,
		Merged:               merged,
		InProgress:           inProgress,
		Unclaimed:            unclaimed,
		Blocked:              blocked,
		IntegrationFailed:    integrationFailed,
		HypothesisExhausted:  hypothesisExhausted,
		ImmediateDiscoveries: immediateDiscoveries,
		WakeInstructions:     wakeInstructions,
	}
	return executeTemplate("orchestrator_context", data)
}

// coderContextData is the template data for coder_context.tmpl
type coderContextData struct {
	Task              *models.Task
	Config            CoderContextConfig
	WorktreePath      string
	HasPriorRejection bool
}

// BuildCoderContext creates coder-specific context with task details
func BuildCoderContext(task *models.Task, config CoderContextConfig) (string, error) {
	worktreePath := ""
	if task.Worktree != nil {
		worktreePath = fmt.Sprintf("%s/%s", config.ProjectRoot, *task.Worktree)
	}

	data := coderContextData{
		Task:              task,
		Config:            config,
		WorktreePath:      worktreePath,
		HasPriorRejection: hasPriorRejection(task),
	}
	return executeTemplate("coder_context", data)
}

// reviewerContextData is the template data for reviewer_context.tmpl
type reviewerContextData struct {
	Task              *models.Task
	Config            ReviewerContextConfig
	WorktreePath      string
	BaseCommit        string
	ReviewCommit      string
	AssignedTo        string
	HasPriorRejection bool
	ScopeExtensions   []map[string]string
}

// codePlannerContextData is the template data for code_planner_context.tmpl
type codePlannerContextData struct {
	Task         *models.Task
	Config       CodePlannerContextConfig
	WorktreePath string
}

// codePlanReviewerContextData is the template data for code_plan_reviewer_context.tmpl
type codePlanReviewerContextData struct {
	Task         *models.Task
	Config       CodePlanReviewerContextConfig
	WorktreePath string
	ReviewCommit string
	AssignedTo   string
}

// BuildReviewerContext creates reviewer-specific context with review details
func BuildReviewerContext(task *models.Task, config ReviewerContextConfig) (string, error) {
	worktreePath := ""
	if task.Worktree != nil {
		worktreePath = fmt.Sprintf("%s/%s", config.ProjectRoot, *task.Worktree)
	}

	var scopeExtensions []map[string]string
	if task.AssignedTo != nil {
		scopeExtensions = ops.GetLatestScopeExtensions(task.History, *task.AssignedTo)
	}

	data := reviewerContextData{
		Task:              task,
		Config:            config,
		WorktreePath:      worktreePath,
		BaseCommit:        derefString(task.BaseCommit),
		ReviewCommit:      derefString(task.ReviewCommit),
		AssignedTo:        derefString(task.AssignedTo),
		HasPriorRejection: hasPriorRejection(task),
		ScopeExtensions:   scopeExtensions,
	}
	return executeTemplate("reviewer_context", data)
}

// BuildCodePlannerContext creates code-planner-specific context with task details
func BuildCodePlannerContext(task *models.Task, config CodePlannerContextConfig) (string, error) {
	worktreePath := ""
	if task.Worktree != nil {
		worktreePath = fmt.Sprintf("%s/%s", config.ProjectRoot, *task.Worktree)
	}

	data := codePlannerContextData{
		Task:         task,
		Config:       config,
		WorktreePath: worktreePath,
	}
	return executeTemplate("code_planner_context", data)
}

// BuildCodePlanReviewerContext creates code-plan-reviewer-specific context with review details
func BuildCodePlanReviewerContext(task *models.Task, config CodePlanReviewerContextConfig) (string, error) {
	worktreePath := ""
	if task.Worktree != nil {
		worktreePath = fmt.Sprintf("%s/%s", config.ProjectRoot, *task.Worktree)
	}

	data := codePlanReviewerContextData{
		Task:         task,
		Config:       config,
		WorktreePath: worktreePath,
		ReviewCommit: derefString(task.ReviewCommit),
		AssignedTo:   derefString(task.AssignedTo),
	}
	return executeTemplate("code_plan_reviewer_context", data)
}

// derefString returns the value pointed to by s, or "" if s is nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// hasPriorRejection reports whether the task has actionable rejection feedback from a prior iteration
func hasPriorRejection(task *models.Task) bool {
	return task.Iteration > 1 && task.RejectionReason != nil && *task.RejectionReason != "" && *task.RejectionReason != "null"
}

// countTasksByStatus counts tasks with a specific status
func countTasksByStatus(tasks []models.Task, status models.TaskStatus) int {
	count := 0
	for _, task := range tasks {
		if task.Status == status {
			count++
		}
	}
	return count
}

// determineWakeTrigger determines what triggered the orchestrator to wake
func determineWakeTrigger(totalTasks, blocked, integrationFailed, hypothesisExhausted, immediateDiscoveries int, sprintComplete bool, planningTasks []planningTaskData) string {
	if totalTasks == 0 {
		return "INITIAL_PLANNING"
	}
	if blocked > 0 {
		return "BLOCKED_TASKS"
	}
	if integrationFailed > 0 {
		return "INTEGRATION_FAILED"
	}
	if hypothesisExhausted > 0 {
		return "HYPOTHESIS_EXHAUSTED"
	}
	if immediateDiscoveries > 0 {
		return "IMMEDIATE_DISCOVERY"
	}
	if sprintComplete && len(planningTasks) > 0 {
		return "PLANNING_COMPLETE"
	}
	if sprintComplete {
		return "SPRINT_COMPLETE"
	}
	return "UNKNOWN"
}

// wakeTemplateData is used by wake trigger templates that need GoalSpecRef
type wakeTemplateData struct {
	GoalSpecRef string
}

// wakePlanningCompleteData is used by the PLANNING_COMPLETE wake template
type wakePlanningCompleteData struct {
	PlanningTasks []planningTaskData
}

// buildInstructionsForWakeTrigger returns trigger-specific instructions
func buildInstructionsForWakeTrigger(wakeTrigger, goalSpecRef string, planningTasks []planningTaskData) (string, error) {
	switch wakeTrigger {
	case "INITIAL_PLANNING":
		return executeTemplate("wake_initial_planning", wakeTemplateData{GoalSpecRef: goalSpecRef})
	case "BLOCKED_TASKS":
		return executeTemplate("wake_blocked_tasks", nil)
	case "INTEGRATION_FAILED":
		return executeTemplate("wake_integration_failed", nil)
	case "HYPOTHESIS_EXHAUSTED":
		return executeTemplate("wake_hypothesis_exhausted", nil)
	case "IMMEDIATE_DISCOVERY":
		return executeTemplate("wake_immediate_discovery", nil)
	case "PLANNING_COMPLETE":
		return executeTemplate("wake_planning_complete", wakePlanningCompleteData{
			PlanningTasks: planningTasks,
		})
	case "SPRINT_COMPLETE":
		return executeTemplate("wake_sprint_complete", nil)
	default:
		return "", nil
	}
}
