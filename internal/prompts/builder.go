package prompts

import (
	"fmt"
	"strings"

	"github.com/liza-mas/liza/internal/models"
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

// PlannerContextConfig contains configuration for building planner context
type PlannerContextConfig struct {
}

// CoderContextConfig contains configuration for building coder context
type CoderContextConfig struct {
	ProjectRoot       string
	AgentID           string
	IntegrationBranch string
	HandoffNote       *models.HandoffNote
}

// ReviewerContextConfig contains configuration for building reviewer context
type ReviewerContextConfig struct {
	ProjectRoot string
	AgentID     string
}

// basePromptData is the template data for base_prompt.tmpl
type basePromptData struct {
	BasePromptConfig
	RoleTitle string
}

// BuildBasePrompt creates the base bootstrap prompt for all agents
func BuildBasePrompt(config BasePromptConfig) string {
	data := basePromptData{
		BasePromptConfig: config,
		RoleTitle:        formatRoleTitle(config.Role),
	}
	return executeTemplate("base_prompt", data)
}

// plannerContextData is the template data for planner_context.tmpl
type plannerContextData struct {
	WakeTrigger          string
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

// BuildPlannerContext creates planner-specific context with sprint state
func BuildPlannerContext(state *models.State, config PlannerContextConfig) string {
	totalTasks := len(state.Tasks)
	merged := countTasksByStatus(state.Tasks, models.TaskStatusMerged)
	blocked := countTasksByStatus(state.Tasks, models.TaskStatusBlocked)
	integrationFailed := countTasksByStatus(state.Tasks, models.TaskStatusIntegrationFailed)
	unclaimed := countTasksByStatus(state.Tasks, models.TaskStatusUnclaimed)

	inProgress := countTasksByStatus(state.Tasks, models.TaskStatusClaimed) +
		countTasksByStatus(state.Tasks, models.TaskStatusReadyForReview) +
		countTasksByStatus(state.Tasks, models.TaskStatusApproved)

	hypothesisExhausted := 0
	for _, task := range state.Tasks {
		if len(task.FailedBy) >= 2 {
			hypothesisExhausted++
		}
	}

	immediateDiscoveries := 0
	for _, disc := range state.Discovered {
		if disc.Urgency == "immediate" && disc.ConvertedToTask == nil {
			immediateDiscoveries++
		}
	}

	wakeTrigger := determineWakeTrigger(totalTasks, blocked, integrationFailed, hypothesisExhausted, immediateDiscoveries)

	data := plannerContextData{
		WakeTrigger:          wakeTrigger,
		TotalTasks:           totalTasks,
		Merged:               merged,
		InProgress:           inProgress,
		Unclaimed:            unclaimed,
		Blocked:              blocked,
		IntegrationFailed:    integrationFailed,
		HypothesisExhausted:  hypothesisExhausted,
		ImmediateDiscoveries: immediateDiscoveries,
		WakeInstructions:     buildInstructionsForWakeTrigger(wakeTrigger, state.Goal.SpecRef),
	}
	return executeTemplate("planner_context", data)
}

// coderContextData is the template data for coder_context.tmpl
type coderContextData struct {
	Task              *models.Task
	Config            CoderContextConfig
	WorktreePath      string
	HasPriorRejection bool
}

// BuildCoderContext creates coder-specific context with task details
func BuildCoderContext(task *models.Task, config CoderContextConfig) string {
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
}

// BuildReviewerContext creates reviewer-specific context with review details
func BuildReviewerContext(task *models.Task, config ReviewerContextConfig) string {
	worktreePath := ""
	if task.Worktree != nil {
		worktreePath = fmt.Sprintf("%s/%s", config.ProjectRoot, *task.Worktree)
	}

	data := reviewerContextData{
		Task:              task,
		Config:            config,
		WorktreePath:      worktreePath,
		BaseCommit:        derefString(task.BaseCommit),
		ReviewCommit:      derefString(task.ReviewCommit),
		AssignedTo:        derefString(task.AssignedTo),
		HasPriorRejection: hasPriorRejection(task),
	}
	return executeTemplate("reviewer_context", data)
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

// formatRoleTitle converts kebab-case role names to Title Case
func formatRoleTitle(role string) string {
	words := strings.Split(role, "-")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
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

// determineWakeTrigger determines what triggered the planner to wake
func determineWakeTrigger(totalTasks, blocked, integrationFailed, hypothesisExhausted, immediateDiscoveries int) string {
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
	return "UNKNOWN"
}

// wakeTemplateData is used by wake trigger templates that need GoalSpecRef
type wakeTemplateData struct {
	GoalSpecRef string
}

// buildInstructionsForWakeTrigger returns trigger-specific instructions
func buildInstructionsForWakeTrigger(wakeTrigger, goalSpecRef string) string {
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
	default:
		return ""
	}
}
