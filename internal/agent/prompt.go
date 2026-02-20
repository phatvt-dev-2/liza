package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/prompts"
)

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

	prompt := prompts.BuildBasePrompt(baseConfig)

	// Add role-specific context
	switch config.Role {
	case "coder":
		task := state.FindTask(taskID)
		if task == nil {
			return "", fmt.Errorf("task not found: %s", taskID)
		}

		var handoffNote *models.HandoffNote
		if note, ok := state.Handoff[task.ID]; ok {
			noteCopy := note
			handoffNote = &noteCopy
		}

		coderConfig := prompts.CoderContextConfig{
			ProjectRoot:       config.ProjectRoot,
			AgentID:           config.AgentID,
			IntegrationBranch: state.Config.IntegrationBranch,
			HandoffNote:       handoffNote,
		}
		prompt += prompts.BuildCoderContext(task, coderConfig)

	case "code-reviewer":
		task := state.FindTask(taskID)
		if task == nil {
			return "", fmt.Errorf("task not found: %s", taskID)
		}

		reviewerConfig := prompts.ReviewerContextConfig{
			ProjectRoot: config.ProjectRoot,
			AgentID:     config.AgentID,
		}
		prompt += prompts.BuildReviewerContext(task, reviewerConfig)

	case "planner":
		plannerConfig := prompts.PlannerContextConfig{}
		prompt += prompts.BuildPlannerContext(state, plannerConfig)
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
