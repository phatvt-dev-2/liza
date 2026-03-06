package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/prompts"
	"github.com/liza-mas/liza/internal/roles"
)

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

// buildPrompt creates the complete prompt for the agent
func buildPrompt(state *models.State, config SupervisorConfig, taskID string) (string, error) {
	// Build base prompt
	baseConfig := prompts.BasePromptConfig{
		Role:        config.Role,
		AgentID:     config.AgentID,
		SpecsDir:    config.SpecsDir,
		ProjectRoot: config.ProjectRoot,
		StatePath:   config.StatePath,
		GoalDesc:    state.Goal.Description,
		GoalSpecRef: state.Goal.SpecRef,
	}

	prompt, err := prompts.BuildBasePrompt(baseConfig)
	if err != nil {
		return "", fmt.Errorf("building base prompt: %w", err)
	}

	// Add role-specific context
	switch config.Role {
	case roles.RuntimeCoder:
		task := state.FindTask(taskID)
		if task == nil {
			return "", &errors.NotFoundError{Entity: "task", ID: taskID}
		}

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
		context, err := prompts.BuildCoderContext(task, coderConfig)
		if err != nil {
			return "", fmt.Errorf("building coder context: %w", err)
		}
		prompt += context

	case roles.RuntimeCodeReviewer:
		task := state.FindTask(taskID)
		if task == nil {
			return "", &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		siblingTasks, totalPlanTasks, taskOrdinal := collectSiblingTasks(state, task.ID)

		reviewerConfig := prompts.ReviewerContextConfig{
			ProjectRoot:    config.ProjectRoot,
			AgentID:        config.AgentID,
			GoalSpecRef:    state.Goal.SpecRef,
			SiblingTasks:   siblingTasks,
			TotalPlanTasks: totalPlanTasks,
			TaskOrdinal:    taskOrdinal,
		}
		context, err := prompts.BuildReviewerContext(task, reviewerConfig)
		if err != nil {
			return "", fmt.Errorf("building reviewer context: %w", err)
		}
		prompt += context

	case roles.RuntimeOrchestrator:
		orchestratorConfig := prompts.OrchestratorContextConfig{
			ProjectRoot: config.ProjectRoot,
		}
		context, err := prompts.BuildOrchestratorContext(state, orchestratorConfig)
		if err != nil {
			return "", fmt.Errorf("building orchestrator context: %w", err)
		}
		prompt += context

	case roles.RuntimeCodePlanner:
		task := state.FindTask(taskID)
		if task == nil {
			return "", &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		plannerConfig := prompts.CodePlannerContextConfig{
			ProjectRoot: config.ProjectRoot,
			AgentID:     config.AgentID,
		}
		context, err := prompts.BuildCodePlannerContext(task, plannerConfig)
		if err != nil {
			return "", fmt.Errorf("building code-planner context: %w", err)
		}
		prompt += context

	case roles.RuntimeCodePlanReviewer:
		task := state.FindTask(taskID)
		if task == nil {
			return "", &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		reviewerConfig := prompts.CodePlanReviewerContextConfig{
			ProjectRoot: config.ProjectRoot,
			AgentID:     config.AgentID,
		}
		context, err := prompts.BuildCodePlanReviewerContext(task, reviewerConfig)
		if err != nil {
			return "", fmt.Errorf("building code-plan-reviewer context: %w", err)
		}
		prompt += context

	case roles.RuntimeEpicPlanner:
		task := state.FindTask(taskID)
		if task == nil {
			return "", &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		epicPlannerConfig := prompts.EpicPlannerContextConfig{
			ProjectRoot: config.ProjectRoot,
			AgentID:     config.AgentID,
		}
		context, err := prompts.BuildEpicPlannerContext(task, epicPlannerConfig)
		if err != nil {
			return "", fmt.Errorf("building epic-planner context: %w", err)
		}
		prompt += context

	case roles.RuntimeEpicPlanReviewer:
		task := state.FindTask(taskID)
		if task == nil {
			return "", &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		epicPlanReviewerConfig := prompts.EpicPlanReviewerContextConfig{
			ProjectRoot: config.ProjectRoot,
			AgentID:     config.AgentID,
		}
		context, err := prompts.BuildEpicPlanReviewerContext(task, epicPlanReviewerConfig)
		if err != nil {
			return "", fmt.Errorf("building epic-plan-reviewer context: %w", err)
		}
		prompt += context

	case roles.RuntimeUSWriter:
		task := state.FindTask(taskID)
		if task == nil {
			return "", &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		usWriterConfig := prompts.USWriterContextConfig{
			ProjectRoot: config.ProjectRoot,
			AgentID:     config.AgentID,
		}
		context, err := prompts.BuildUSWriterContext(task, usWriterConfig)
		if err != nil {
			return "", fmt.Errorf("building us-writer context: %w", err)
		}
		prompt += context

	case roles.RuntimeUSReviewer:
		task := state.FindTask(taskID)
		if task == nil {
			return "", &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		usReviewerConfig := prompts.USReviewerContextConfig{
			ProjectRoot: config.ProjectRoot,
			AgentID:     config.AgentID,
		}
		context, err := prompts.BuildUSReviewerContext(task, usReviewerConfig)
		if err != nil {
			return "", fmt.Errorf("building us-reviewer context: %w", err)
		}
		prompt += context
	}

	// Add resume context if initial task
	if config.InitialTask != "" {
		prompt += fmt.Sprintf("\n\n=== RESUME CONTEXT ===\nResuming task: %s\n", config.InitialTask)
	}

	return prompt, nil
}

// savePrompt saves the prompt to a file and returns the path
func savePrompt(promptDir, agentID, prompt string) (string, error) {
	// Create prompt directory if missing
	if err := os.MkdirAll(promptDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create prompt directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.txt", agentID, timestamp)
	filePath := filepath.Join(promptDir, filename)

	// Write prompt
	if err := os.WriteFile(filePath, []byte(prompt), 0644); err != nil {
		return "", fmt.Errorf("failed to write prompt file: %w", err)
	}

	return filePath, nil
}
