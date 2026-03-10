package agent

import (
	"fmt"

	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/prompts"
)

// contextBuilderFunc returns the role-specific context string for a task-based role.
// It does NOT include the base prompt or InitialTask suffix — those are handled by
// buildPromptWithContext.
type contextBuilderFunc func(task *models.Task, state *models.State, config SupervisorConfig) (string, error)

// baseConfigFrom constructs the BasePromptConfig shared by all roles.
func baseConfigFrom(state *models.State, config SupervisorConfig, taskID string) prompts.BasePromptConfig {
	return prompts.BasePromptConfig{
		Role:        config.Role,
		AgentID:     config.AgentID,
		TaskID:      taskID,
		SpecsDir:    config.SpecsDir,
		ProjectRoot: config.ProjectRoot,
		StatePath:   config.StatePath,
		GoalDesc:    state.Goal.Description,
		GoalSpecRef: state.Goal.SpecRef,
	}
}

// buildPromptWithContext builds a complete prompt for any task-based role:
// base prompt + task lookup + role-specific context + InitialTask suffix.
func buildPromptWithContext(state *models.State, config SupervisorConfig, taskID string, ctxFn contextBuilderFunc) (string, error) {
	prompt, err := prompts.BuildBasePrompt(baseConfigFrom(state, config, taskID))
	if err != nil {
		return "", fmt.Errorf("building base prompt: %w", err)
	}

	task := state.FindTask(taskID)
	if task == nil {
		return "", &errors.NotFoundError{Entity: "task", ID: taskID}
	}

	context, err := ctxFn(task, state, config)
	if err != nil {
		return "", err
	}
	prompt += context

	if config.InitialTask != "" {
		prompt += fmt.Sprintf("\n\n=== RESUME CONTEXT ===\nResuming task: %s\n", config.InitialTask)
	}

	return prompt, nil
}

// buildOrchestratorPromptContext builds the complete prompt for the orchestrator role.
// Unlike task-based roles, the orchestrator has no task to look up.
func buildOrchestratorPromptContext(state *models.State, config SupervisorConfig) (string, error) {
	prompt, err := prompts.BuildBasePrompt(baseConfigFrom(state, config, ""))
	if err != nil {
		return "", fmt.Errorf("building base prompt: %w", err)
	}

	context, err := orchestratorContext(state, config)
	if err != nil {
		return "", err
	}
	prompt += context

	if config.InitialTask != "" {
		prompt += fmt.Sprintf("\n\n=== RESUME CONTEXT ===\nResuming task: %s\n", config.InitialTask)
	}

	return prompt, nil
}

// --- Named context builder functions (one per task-based role) ---

func coderContext(task *models.Task, state *models.State, config SupervisorConfig) (string, error) {
	var handoffNote *models.HandoffNote
	if note, ok := state.Handoff[task.ID]; ok {
		noteCopy := note
		handoffNote = &noteCopy
	}

	siblingTasks, totalPlanTasks, taskOrdinal := collectSiblingTasks(state, task.ID)

	coderConfig := prompts.CoderContextConfig{
		ProjectRoot:       config.ProjectRoot,
		AgentID:           config.AgentID,
		IntegrationBranch: state.Config.IntegrationBranch,
		HandoffNote:       handoffNote,
		GoalSpecRef:       state.Goal.SpecRef,
		SiblingTasks:      siblingTasks,
		TotalPlanTasks:    totalPlanTasks,
		TaskOrdinal:       taskOrdinal,
	}
	return prompts.BuildCoderContext(task, coderConfig)
}

func codePlannerContext(task *models.Task, state *models.State, config SupervisorConfig) (string, error) {
	siblingTasks, totalPlanTasks, taskOrdinal := collectSiblingTasks(state, task.ID)

	plannerConfig := prompts.CodePlannerContextConfig{
		ProjectRoot:    config.ProjectRoot,
		AgentID:        config.AgentID,
		GoalSpecRef:    state.Goal.SpecRef,
		SiblingTasks:   siblingTasks,
		TotalPlanTasks: totalPlanTasks,
		TaskOrdinal:    taskOrdinal,
	}
	return prompts.BuildCodePlannerContext(task, plannerConfig)
}

func codeReviewerContext(task *models.Task, state *models.State, config SupervisorConfig) (string, error) {
	siblingTasks, totalPlanTasks, taskOrdinal := collectSiblingTasks(state, task.ID)

	reviewerConfig := prompts.ReviewerContextConfig{
		ProjectRoot:    config.ProjectRoot,
		AgentID:        config.AgentID,
		GoalSpecRef:    state.Goal.SpecRef,
		SiblingTasks:   siblingTasks,
		TotalPlanTasks: totalPlanTasks,
		TaskOrdinal:    taskOrdinal,
	}
	return prompts.BuildReviewerContext(task, reviewerConfig)
}

func codePlanReviewerContext(task *models.Task, state *models.State, config SupervisorConfig) (string, error) {
	siblingTasks, totalPlanTasks, taskOrdinal := collectSiblingTasks(state, task.ID)

	reviewerConfig := prompts.CodePlanReviewerContextConfig{
		ProjectRoot:    config.ProjectRoot,
		AgentID:        config.AgentID,
		GoalSpecRef:    state.Goal.SpecRef,
		SiblingTasks:   siblingTasks,
		TotalPlanTasks: totalPlanTasks,
		TaskOrdinal:    taskOrdinal,
	}
	return prompts.BuildCodePlanReviewerContext(task, reviewerConfig)
}

func epicPlannerContext(task *models.Task, _ *models.State, config SupervisorConfig) (string, error) {
	epicPlannerConfig := prompts.EpicPlannerContextConfig{
		ProjectRoot: config.ProjectRoot,
		AgentID:     config.AgentID,
	}
	return prompts.BuildEpicPlannerContext(task, epicPlannerConfig)
}

func epicPlanReviewerContext(task *models.Task, _ *models.State, config SupervisorConfig) (string, error) {
	epicPlanReviewerConfig := prompts.EpicPlanReviewerContextConfig{
		ProjectRoot: config.ProjectRoot,
		AgentID:     config.AgentID,
	}
	return prompts.BuildEpicPlanReviewerContext(task, epicPlanReviewerConfig)
}

func usWriterContext(task *models.Task, state *models.State, config SupervisorConfig) (string, error) {
	siblingTasks, totalPlanTasks, taskOrdinal := collectSiblingTasks(state, task.ID)

	usWriterConfig := prompts.USWriterContextConfig{
		ProjectRoot:    config.ProjectRoot,
		AgentID:        config.AgentID,
		GoalSpecRef:    state.Goal.SpecRef,
		SiblingTasks:   siblingTasks,
		TotalPlanTasks: totalPlanTasks,
		TaskOrdinal:    taskOrdinal,
	}
	return prompts.BuildUSWriterContext(task, usWriterConfig)
}

func usReviewerContext(task *models.Task, state *models.State, config SupervisorConfig) (string, error) {
	siblingTasks, totalPlanTasks, taskOrdinal := collectSiblingTasks(state, task.ID)

	usReviewerConfig := prompts.USReviewerContextConfig{
		ProjectRoot:    config.ProjectRoot,
		AgentID:        config.AgentID,
		GoalSpecRef:    state.Goal.SpecRef,
		SiblingTasks:   siblingTasks,
		TotalPlanTasks: totalPlanTasks,
		TaskOrdinal:    taskOrdinal,
	}
	return prompts.BuildUSReviewerContext(task, usReviewerConfig)
}

func orchestratorContext(state *models.State, config SupervisorConfig) (string, error) {
	orchestratorConfig := prompts.OrchestratorContextConfig{
		ProjectRoot: config.ProjectRoot,
	}
	return prompts.BuildOrchestratorContext(state, orchestratorConfig)
}

// collectSiblingTasks returns summaries of sibling tasks in the sprint plan (excluding currentTaskID),
// the total count of planned tasks, and the 1-based ordinal position of currentTaskID in the plan.
// Returns nil, 0, 0 if no planned tasks or if currentTaskID is not in the planned list
// (e.g. mid-sprint replacement tasks created outside the original plan).
//
// Note: tasks not found by FindTask are silently skipped. This assumes the orchestrator keeps
// Sprint.Scope.Planned in sync with the task list (archived/removed tasks are pruned from planned[]).
func collectSiblingTasks(state *models.State, currentTaskID string) ([]prompts.SiblingTaskSummary, int, int) {
	planned := state.Sprint.Scope.Planned
	if len(planned) == 0 {
		return nil, 0, 0
	}

	ordinal := 0
	var siblings []prompts.SiblingTaskSummary
	for i, id := range planned {
		if id == currentTaskID {
			ordinal = i + 1 // 1-based
			continue
		}
		task := state.FindTask(id)
		if task != nil {
			siblings = append(siblings, prompts.SiblingTaskSummary{
				ID:          task.ID,
				Description: task.Description,
				Status:      string(task.Status),
			})
		}
	}

	// Suppress scoping for tasks not in the plan (mid-sprint replacements).
	// Returning 0 for totalPlanTasks ensures the template condition is false.
	if ordinal == 0 {
		return nil, 0, 0
	}

	return siblings, len(planned), ordinal
}

func savePrompt(promptDir, agentID, prompt string) (string, error) {
	return saveTimestampedFile(promptDir, agentID, "txt", prompt)
}
