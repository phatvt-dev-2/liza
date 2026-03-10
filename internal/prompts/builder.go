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
	TaskID      string // empty for orchestrator
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
	ProjectRoot    string
	AgentID        string
	GoalSpecRef    string
	SiblingTasks   []SiblingTaskSummary
	TotalPlanTasks int
	TaskOrdinal    int
}

// CodePlanReviewerContextConfig contains configuration for building code-plan-reviewer context
type CodePlanReviewerContextConfig struct {
	ProjectRoot    string
	AgentID        string
	GoalSpecRef    string
	SiblingTasks   []SiblingTaskSummary
	TotalPlanTasks int
	TaskOrdinal    int
}

// EpicPlannerContextConfig contains configuration for building epic-planner context
type EpicPlannerContextConfig struct {
	ProjectRoot string
	AgentID     string
}

// USWriterContextConfig contains configuration for building us-writer context
type USWriterContextConfig struct {
	ProjectRoot    string
	AgentID        string
	GoalSpecRef    string
	SiblingTasks   []SiblingTaskSummary
	TotalPlanTasks int
	TaskOrdinal    int
}

// USReviewerContextConfig contains configuration for building us-reviewer context
type USReviewerContextConfig struct {
	ProjectRoot    string
	AgentID        string
	GoalSpecRef    string
	SiblingTasks   []SiblingTaskSummary
	TotalPlanTasks int
	TaskOrdinal    int
}

func resolveWorktreePath(projectRoot string, worktree *string) string {
	if worktree == nil {
		return ""
	}
	return fmt.Sprintf("%s/%s", projectRoot, *worktree)
}

// BuildBasePrompt creates the base bootstrap prompt for all agents
func BuildBasePrompt(config BasePromptConfig) (string, error) {
	return executeTemplate("base_prompt", config)
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

	hypothesisExhausted := countHypothesisExhausted(state.Tasks)
	immediateDiscoveries := countImmediateDiscoveries(state.Discovered)

	detCtx, detErr := ops.LoadDetectionContext(config.ProjectRoot)
	var sprintTerminals []models.TaskStatus
	var planningPairs map[string]bool
	if detErr == nil {
		sprintTerminals = detCtx.SprintTerminals
		planningPairs = detCtx.PlanningPairs
	}

	sprintComplete := state.AllPlannedTasksTerminalWith(sprintTerminals)

	var planningTasks []planningTaskData
	if sprintComplete {
		planningTasks = collectMergedPlanningTasks(state, planningPairs)
	}

	wakeTrigger := determineWakeTrigger(totalTasks, blocked, integrationFailed, hypothesisExhausted, immediateDiscoveries, sprintComplete, planningTasks)

	wakeData, err := buildWakeTemplateData(state.Goal.SpecRef, state.Goal.EntryPoint, config.ProjectRoot)
	if err != nil {
		return "", fmt.Errorf("building wake template data: %w", err)
	}

	wakeInstructions, err := buildInstructionsForWakeTrigger(wakeTrigger, wakeData, planningTasks)
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
	data := coderContextData{
		Task:              task,
		Config:            config,
		WorktreePath:      resolveWorktreePath(config.ProjectRoot, task.Worktree),
		HasPriorRejection: hasPriorRejection(task),
	}
	return executeTemplate("coder_context", data)
}

// reviewerContextData is the template data for code_reviewer_context.tmpl
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
	Task              *models.Task
	Config            CodePlannerContextConfig
	WorktreePath      string
	HasPriorRejection bool
}

// epicPlannerContextData is the template data for epic_planner_context.tmpl
type epicPlannerContextData struct {
	Task              *models.Task
	Config            EpicPlannerContextConfig
	WorktreePath      string
	HasPriorRejection bool
}

// codePlanReviewerContextData is the template data for code_plan_reviewer_context.tmpl
type codePlanReviewerContextData struct {
	Task              *models.Task
	Config            CodePlanReviewerContextConfig
	WorktreePath      string
	BaseCommit        string
	ReviewCommit      string
	AssignedTo        string
	HasPriorRejection bool
}

// EpicPlanReviewerContextConfig contains configuration for building epic-plan-reviewer context
type EpicPlanReviewerContextConfig struct {
	ProjectRoot string
	AgentID     string
}

// epicPlanReviewerContextData is the template data for epic_plan_reviewer_context.tmpl
type epicPlanReviewerContextData struct {
	Task              *models.Task
	Config            EpicPlanReviewerContextConfig
	WorktreePath      string
	BaseCommit        string
	ReviewCommit      string
	AssignedTo        string
	HasPriorRejection bool
}

// usWriterContextData is the template data for us_writer_context.tmpl
type usWriterContextData struct {
	Task              *models.Task
	Config            USWriterContextConfig
	WorktreePath      string
	HasPriorRejection bool
}

// usReviewerContextData is the template data for us_reviewer_context.tmpl
type usReviewerContextData struct {
	Task              *models.Task
	Config            USReviewerContextConfig
	WorktreePath      string
	BaseCommit        string
	ReviewCommit      string
	AssignedTo        string
	HasPriorRejection bool
}

// BuildReviewerContext creates reviewer-specific context with review details
func BuildReviewerContext(task *models.Task, config ReviewerContextConfig) (string, error) {
	var scopeExtensions []map[string]string
	if task.AssignedTo != nil {
		scopeExtensions = ops.GetLatestScopeExtensions(task.History, *task.AssignedTo)
	}

	data := reviewerContextData{
		Task:              task,
		Config:            config,
		WorktreePath:      resolveWorktreePath(config.ProjectRoot, task.Worktree),
		BaseCommit:        derefString(task.BaseCommit),
		ReviewCommit:      derefString(task.ReviewCommit),
		AssignedTo:        derefString(task.AssignedTo),
		HasPriorRejection: hasPriorRejection(task),
		ScopeExtensions:   scopeExtensions,
	}
	return executeTemplate("code_reviewer_context", data)
}

// BuildCodePlannerContext creates code-planner-specific context with task details
func BuildCodePlannerContext(task *models.Task, config CodePlannerContextConfig) (string, error) {
	data := codePlannerContextData{
		Task:              task,
		Config:            config,
		WorktreePath:      resolveWorktreePath(config.ProjectRoot, task.Worktree),
		HasPriorRejection: hasPriorRejection(task),
	}
	return executeTemplate("code_planner_context", data)
}

// BuildCodePlanReviewerContext creates code-plan-reviewer-specific context with review details
func BuildCodePlanReviewerContext(task *models.Task, config CodePlanReviewerContextConfig) (string, error) {
	data := codePlanReviewerContextData{
		Task:              task,
		Config:            config,
		WorktreePath:      resolveWorktreePath(config.ProjectRoot, task.Worktree),
		BaseCommit:        derefString(task.BaseCommit),
		ReviewCommit:      derefString(task.ReviewCommit),
		AssignedTo:        derefString(task.AssignedTo),
		HasPriorRejection: hasPriorRejection(task),
	}
	return executeTemplate("code_plan_reviewer_context", data)
}

// BuildEpicPlanReviewerContext creates epic-plan-reviewer-specific context with review details
func BuildEpicPlanReviewerContext(task *models.Task, config EpicPlanReviewerContextConfig) (string, error) {
	data := epicPlanReviewerContextData{
		Task:              task,
		Config:            config,
		WorktreePath:      resolveWorktreePath(config.ProjectRoot, task.Worktree),
		BaseCommit:        derefString(task.BaseCommit),
		ReviewCommit:      derefString(task.ReviewCommit),
		AssignedTo:        derefString(task.AssignedTo),
		HasPriorRejection: hasPriorRejection(task),
	}
	return executeTemplate("epic_plan_reviewer_context", data)
}

// BuildEpicPlannerContext creates epic-planner-specific context with task details and decomposition guidance
func BuildEpicPlannerContext(task *models.Task, config EpicPlannerContextConfig) (string, error) {
	data := epicPlannerContextData{
		Task:              task,
		Config:            config,
		WorktreePath:      resolveWorktreePath(config.ProjectRoot, task.Worktree),
		HasPriorRejection: hasPriorRejection(task),
	}
	return executeTemplate("epic_planner_context", data)
}

// BuildUSWriterContext creates us-writer-specific context with task details
func BuildUSWriterContext(task *models.Task, config USWriterContextConfig) (string, error) {
	data := usWriterContextData{
		Task:              task,
		Config:            config,
		WorktreePath:      resolveWorktreePath(config.ProjectRoot, task.Worktree),
		HasPriorRejection: hasPriorRejection(task),
	}
	return executeTemplate("us_writer_context", data)
}

// BuildUSReviewerContext creates us-reviewer-specific context with review details
func BuildUSReviewerContext(task *models.Task, config USReviewerContextConfig) (string, error) {
	data := usReviewerContextData{
		Task:              task,
		Config:            config,
		WorktreePath:      resolveWorktreePath(config.ProjectRoot, task.Worktree),
		BaseCommit:        derefString(task.BaseCommit),
		ReviewCommit:      derefString(task.ReviewCommit),
		AssignedTo:        derefString(task.AssignedTo),
		HasPriorRejection: hasPriorRejection(task),
	}
	return executeTemplate("us_reviewer_context", data)
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

func countTasksByStatus(tasks []models.Task, status models.TaskStatus) int {
	count := 0
	for _, task := range tasks {
		if task.Status == status {
			count++
		}
	}
	return count
}

// countHypothesisExhausted counts non-terminal tasks that have been failed by 2+ reviewers.
func countHypothesisExhausted(tasks []models.Task) int {
	count := 0
	for _, task := range tasks {
		if len(task.FailedBy) >= 2 && !task.Status.IsTerminal() {
			count++
		}
	}
	return count
}

// countImmediateDiscoveries counts unresolved discoveries with "immediate" urgency.
func countImmediateDiscoveries(discovered []models.Discovery) int {
	count := 0
	for _, disc := range discovered {
		if disc.Urgency == "immediate" && disc.ConvertedToTask == nil {
			count++
		}
	}
	return count
}
