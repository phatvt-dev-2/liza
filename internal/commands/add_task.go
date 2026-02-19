package commands

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/log"
	"github.com/liza-mas/liza/internal/models"
	"gopkg.in/yaml.v3"
)

// TaskInput represents the input parameters for adding a task.
// Can be loaded from a YAML file or constructed from CLI flags.
type TaskInput struct {
	ID          string   `yaml:"id"`
	Type        string   `yaml:"type,omitempty"`
	Description string   `yaml:"description"`
	SpecRef     string   `yaml:"spec_ref"`
	DoneWhen    string   `yaml:"done_when"`
	Scope       string   `yaml:"scope"`
	Priority    int      `yaml:"priority"`
	DependsOn   []string `yaml:"depends_on,omitempty"`
}

// LoadTaskInputFromFile loads task input from a YAML file.
func LoadTaskInputFromFile(path string) (*TaskInput, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read task file: %w", err)
	}

	var input TaskInput
	if err := yaml.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("failed to parse task file: %w", err)
	}

	return &input, nil
}

// AddTaskCommand adds a new task to the state.yaml file.
// It validates inputs, checks for duplicate task IDs, atomically adds the task,
// updates sprint.scope.planned and goal.alignment_history, logs the action,
// and runs validation.
func AddTaskCommand(statePath, logPath string, input *TaskInput, plannerID string) error {
	if plannerID == "" {
		plannerID = "planner-1"
	}
	// Validate required fields
	if input.ID == "" {
		return fmt.Errorf("task ID is required")
	}
	if input.Description == "" {
		return fmt.Errorf("description is required")
	}
	if input.SpecRef == "" {
		return fmt.Errorf("spec_ref is required")
	}
	if input.DoneWhen == "" {
		return fmt.Errorf("done_when is required")
	}
	if input.Scope == "" {
		return fmt.Errorf("scope is required")
	}
	if input.Priority < 1 {
		return fmt.Errorf("priority must be positive, got %d", input.Priority)
	}

	// Default task type to "coding" if empty
	if input.Type == "" {
		input.Type = string(models.TaskTypeCoding)
	}

	// Validate task type
	taskType := models.TaskType(input.Type)
	if !taskType.IsValid() {
		return fmt.Errorf("unknown task type %q", input.Type)
	}

	// Normalize dependencies (trim whitespace, remove empty strings)
	normalizedDeps := []string{}
	for _, dep := range input.DependsOn {
		trimmed := strings.TrimSpace(dep)
		if trimmed != "" {
			normalizedDeps = append(normalizedDeps, trimmed)
		}
	}

	// Get current time
	now := time.Now().UTC()

	// Use provided planner ID (already has default of "planner-1")
	agentID := plannerID

	// Create blackboard instance
	bb := db.New(statePath)

	// Check for duplicate task ID
	existingTask, err := bb.GetTask(input.ID)
	if err != nil {
		return fmt.Errorf("failed to check for existing task: %w", err)
	}
	if existingTask != nil {
		return fmt.Errorf("task '%s' already exists in %s", input.ID, statePath)
	}

	// Create new task
	newTask := models.Task{
		ID:          input.ID,
		Type:        taskType,
		Description: input.Description,
		Status:      models.TaskStatusReady,
		Priority:    input.Priority,
		SpecRef:     input.SpecRef,
		DoneWhen:    input.DoneWhen,
		Scope:       input.Scope,
		DependsOn:   normalizedDeps,
		Created:     now,
		History:     []models.TaskHistoryEntry{},
	}

	// Atomically add task and update state
	err = bb.Modify(func(state *models.State) error {
		// Add task to tasks list
		state.Tasks = append(state.Tasks, newTask)

		// Update sprint.scope.planned (add taskID if not already present)
		if !slices.Contains(state.Sprint.Scope.Planned, input.ID) {
			state.Sprint.Scope.Planned = append(state.Sprint.Scope.Planned, input.ID)
		}

		// Add to goal.alignment_history
		alignmentEntry := models.AlignmentHistory{
			Timestamp: now,
			Event:     "planning",
			Summary:   fmt.Sprintf("Added task %s: %s", input.ID, input.Description),
		}
		state.Goal.AlignmentHistory = append(state.Goal.AlignmentHistory, alignmentEntry)

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to add task: %w", err)
	}

	// Log the action
	logger := log.New(logPath)
	logEntry := log.Entry{
		Timestamp: now,
		Agent:     agentID,
		Action:    "task_added",
		Task:      &input.ID,
		Detail:    input.Description,
	}

	if err := logger.Append(logEntry); err != nil {
		// Log error but don't fail the command
		fmt.Fprintf(os.Stderr, "Warning: failed to log action: %v\n", err)
	}

	// Run validation
	if err := ValidateCommand(statePath, false); err != nil {
		return fmt.Errorf("validation failed after adding task: %w", err)
	}

	return nil
}
