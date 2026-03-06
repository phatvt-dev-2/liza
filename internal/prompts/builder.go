package prompts

import (
	"fmt"
	"sort"
	"strings"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/pipeline"
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

// EpicPlannerContextConfig contains configuration for building epic-planner context
type EpicPlannerContextConfig struct {
	ProjectRoot string
	AgentID     string
}

// USWriterContextConfig contains configuration for building us-writer context
type USWriterContextConfig struct {
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

	wakeData := buildWakeTemplateData(state.Goal.SpecRef, state.Goal.EntryPoint, config.ProjectRoot)

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

// epicPlannerContextData is the template data for epic_planner_context.tmpl
type epicPlannerContextData struct {
	Task         *models.Task
	Config       EpicPlannerContextConfig
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

// EpicPlanReviewerContextConfig contains configuration for building epic-plan-reviewer context
type EpicPlanReviewerContextConfig struct {
	ProjectRoot string
	AgentID     string
}

// epicPlanReviewerContextData is the template data for epic_plan_reviewer_context.tmpl
type epicPlanReviewerContextData struct {
	Task         *models.Task
	Config       EpicPlanReviewerContextConfig
	WorktreePath string
	ReviewCommit string
	AssignedTo   string
}

// usWriterContextData is the template data for us_writer_context.tmpl
type usWriterContextData struct {
	Task         *models.Task
	Config       USWriterContextConfig
	WorktreePath string
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

// BuildEpicPlanReviewerContext creates epic-plan-reviewer-specific context with review details
func BuildEpicPlanReviewerContext(task *models.Task, config EpicPlanReviewerContextConfig) (string, error) {
	worktreePath := ""
	if task.Worktree != nil {
		worktreePath = fmt.Sprintf("%s/%s", config.ProjectRoot, *task.Worktree)
	}

	data := epicPlanReviewerContextData{
		Task:         task,
		Config:       config,
		WorktreePath: worktreePath,
		ReviewCommit: derefString(task.ReviewCommit),
		AssignedTo:   derefString(task.AssignedTo),
	}
	return executeTemplate("epic_plan_reviewer_context", data)
}

// BuildEpicPlannerContext creates epic-planner-specific context with task details and decomposition guidance
func BuildEpicPlannerContext(task *models.Task, config EpicPlannerContextConfig) (string, error) {
	worktreePath := ""
	if task.Worktree != nil {
		worktreePath = fmt.Sprintf("%s/%s", config.ProjectRoot, *task.Worktree)
	}

	data := epicPlannerContextData{
		Task:         task,
		Config:       config,
		WorktreePath: worktreePath,
	}
	return executeTemplate("epic_planner_context", data)
}

// BuildUSWriterContext creates us-writer-specific context with task details
func BuildUSWriterContext(task *models.Task, config USWriterContextConfig) (string, error) {
	worktreePath := ""
	if task.Worktree != nil {
		worktreePath = fmt.Sprintf("%s/%s", config.ProjectRoot, *task.Worktree)
	}

	data := usWriterContextData{
		Task:         task,
		Config:       config,
		WorktreePath: worktreePath,
	}
	return executeTemplate("us_writer_context", data)
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

// wakeEntryPointData describes an available entry-point for the orchestrator template.
type wakeEntryPointData struct {
	Name        string // e.g., "general-objective"
	RolePair    string // e.g., "epic-planning-pair"
	DisplayName string // doer's display name, e.g., "Epic Planner"
}

// wakeTemplateData is used by wake trigger templates that need GoalSpecRef
type wakeTemplateData struct {
	GoalSpecRef          string
	GoalEntryPoint       string               // set if --entry-point was specified
	ResolvedRolePair     string               // role-pair resolved from GoalEntryPoint
	ResolvedDisplayName  string               // display name of the resolved role-pair's doer
	ResolvedTaskIDPrefix string               // task ID prefix, e.g., "epic-planning" (role-pair without "-pair" suffix)
	EntryPoints          []wakeEntryPointData // available entry-points for LLM classification
}

// buildWakeTemplateData constructs entry-point-aware template data for the
// INITIAL_PLANNING wake trigger. When a pipeline config exists, it resolves
// entry-points to role-pairs and display names. Without a pipeline config
// (legacy goals), the template falls back to hardcoded code-planning-pair.
func buildWakeTemplateData(goalSpecRef, goalEntryPoint, projectRoot string) wakeTemplateData {
	data := wakeTemplateData{
		GoalSpecRef:    goalSpecRef,
		GoalEntryPoint: goalEntryPoint,
	}

	cfg, err := pipeline.LoadFrozen(projectRoot)
	if err != nil || cfg == nil {
		return data // legacy goal — no pipeline config
	}

	// Build sorted entry-point list for deterministic template output.
	var eps []wakeEntryPointData
	for epName, epValue := range cfg.Pipeline.EntryPoints {
		parts := strings.SplitN(epValue, ".", 2)
		if len(parts) != 2 {
			continue
		}
		rolePair := parts[1]
		displayName := resolveDoerDisplayName(cfg, rolePair)
		eps = append(eps, wakeEntryPointData{
			Name:        epName,
			RolePair:    rolePair,
			DisplayName: displayName,
		})
	}
	sort.Slice(eps, func(i, j int) bool { return eps[i].Name < eps[j].Name })
	data.EntryPoints = eps

	// If entry-point is explicitly set, resolve it.
	if goalEntryPoint != "" {
		if epValue, ok := cfg.Pipeline.EntryPoints[goalEntryPoint]; ok {
			parts := strings.SplitN(epValue, ".", 2)
			if len(parts) == 2 {
				data.ResolvedRolePair = parts[1]
				data.ResolvedDisplayName = resolveDoerDisplayName(cfg, parts[1])
				data.ResolvedTaskIDPrefix = strings.TrimSuffix(parts[1], "-pair")
			}
		}
	}

	return data
}

// resolveDoerDisplayName looks up the doer's display name for a role-pair.
func resolveDoerDisplayName(cfg *pipeline.PipelineConfig, rolePair string) string {
	rp, ok := cfg.Pipeline.RolePairs[rolePair]
	if !ok {
		return rolePair
	}
	if name, ok := cfg.Pipeline.AgentRoles[rp.Doer]; ok {
		return name
	}
	return rp.Doer
}

// wakePlanningCompleteData is used by the PLANNING_COMPLETE wake template
type wakePlanningCompleteData struct {
	PlanningTasks []planningTaskData
}

// buildInstructionsForWakeTrigger returns trigger-specific instructions
func buildInstructionsForWakeTrigger(wakeTrigger string, wakeData wakeTemplateData, planningTasks []planningTaskData) (string, error) {
	switch wakeTrigger {
	case "INITIAL_PLANNING":
		return executeTemplate("wake_initial_planning", wakeData)
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
