package ops

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/log"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/statevalidate"
)

// AddTaskInput represents the input parameters for adding a task.
type AddTaskInput struct {
	ID          string
	Type        string
	RolePair    string
	Description string
	SpecRef     string
	DoneWhen    string
	Scope       string
	Priority    int
	DependsOn   []string
}

// AddTaskResult contains the outcome of adding a task.
type AddTaskResult struct {
	TaskID   string
	Warnings []string
}

// PostWriteValidationError indicates the mutation succeeded but state
// validation failed immediately afterward.
type PostWriteValidationError struct {
	Err error
}

func (e *PostWriteValidationError) Error() string {
	return fmt.Sprintf("task added but state validation failed: %v", e.Err)
}

func (e *PostWriteValidationError) Unwrap() error {
	return e.Err
}

// AddTask atomically persists a new task after validating inputs and checking
// for duplicates. Also updates sprint.scope.planned, goal.alignment_history,
// and appends to the activity log. No terminal I/O.
func AddTask(statePath, logPath string, input *AddTaskInput, orchestratorID string) (*AddTaskResult, error) {
	if orchestratorID == "" {
		orchestratorID = "orchestrator-1"
	}
	if err := paths.ValidateTaskID(input.ID); err != nil {
		return nil, fmt.Errorf("invalid task ID: %w", err)
	}
	if input.Description == "" {
		return nil, fmt.Errorf("description is required")
	}
	if input.SpecRef == "" {
		return nil, fmt.Errorf("spec_ref is required")
	}
	if input.DoneWhen == "" {
		return nil, fmt.Errorf("done_when is required")
	}
	if input.Scope == "" {
		return nil, fmt.Errorf("scope is required")
	}
	if input.Priority < 1 {
		return nil, fmt.Errorf("priority must be positive, got %d", input.Priority)
	}

	if input.Type == "" {
		input.Type = string(models.TaskTypeCoding)
	}

	// Derive project root from state path (.liza/state.yaml → project root)
	projectRoot := filepath.Dir(filepath.Dir(statePath))
	resolver, _, err := loadResolver(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load pipeline config: %w", err)
	}

	taskType := models.TaskType(input.Type)
	if !taskType.IsValid() {
		msg := fmt.Sprintf("unknown task type %q; valid types: %s", input.Type, strings.Join(models.ValidTaskTypeNames(), ", "))
		if resolver != nil {
			msg += fmt.Sprintf(". For pipeline workflow customization, use role_pair (available: %s)",
				strings.Join(resolver.RolePairNames(), ", "))
		}
		return nil, &PreconditionError{Reason: msg}
	}

	if resolver != nil && input.RolePair != "" {
		if _, rpErr := resolver.RolePair(input.RolePair); rpErr != nil {
			return nil, &PreconditionError{
				Reason: fmt.Sprintf("unknown role_pair %q; available role_pairs: %s",
					input.RolePair, strings.Join(resolver.RolePairNames(), ", ")),
			}
		}
	} else if resolver != nil && input.RolePair == "" {
		return nil, &PreconditionError{
			Reason: fmt.Sprintf("role_pair is required for pipeline-configured goals; available: %s",
				strings.Join(resolver.RolePairNames(), ", ")),
		}
	}

	normalizedDeps := []string{}
	for _, dep := range input.DependsOn {
		trimmed := strings.TrimSpace(dep)
		if trimmed != "" {
			normalizedDeps = append(normalizedDeps, trimmed)
		}
	}

	now := time.Now().UTC()
	agentID := orchestratorID

	bb := db.For(statePath)

	newTask := models.Task{
		ID:          input.ID,
		Type:        taskType,
		RolePair:    input.RolePair,
		Description: input.Description,
		Status:      initialTaskStatusWithResolver(input.RolePair, resolver),
		Priority:    input.Priority,
		SpecRef:     input.SpecRef,
		DoneWhen:    input.DoneWhen,
		Scope:       input.Scope,
		DependsOn:   normalizedDeps,
		Created:     now,
		History:     []models.TaskHistoryEntry{},
	}

	err = bb.Modify(func(state *models.State) error {
		if state.FindTask(input.ID) != nil {
			return fmt.Errorf("task '%s' already exists in %s", input.ID, statePath)
		}
		state.Tasks = append(state.Tasks, newTask)

		if !slices.Contains(state.Sprint.Scope.Planned, input.ID) {
			state.Sprint.Scope.Planned = append(state.Sprint.Scope.Planned, input.ID)
		}

		alignmentEntry := models.AlignmentHistory{
			Timestamp: now,
			Event:     "planning",
			Summary:   fmt.Sprintf("Added task %s: %s", input.ID, input.Description),
		}
		state.Goal.AlignmentHistory = append(state.Goal.AlignmentHistory, alignmentEntry)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to add task: %w", err)
	}

	result := &AddTaskResult{TaskID: input.ID}

	logger := log.New(logPath)
	logEntry := log.Entry{
		Timestamp: now,
		Agent:     agentID,
		Action:    "task_added",
		Task:      &input.ID,
		Detail:    input.Description,
	}

	if err := logger.Append(logEntry); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("activity log write failed: %v", err))
	}

	if err := statevalidate.ValidateStateFile(statePath, false, io.Discard); err != nil {
		return nil, &PostWriteValidationError{Err: err}
	}

	return result, nil
}

// AddTasksInput represents the input for batch task creation.
type AddTasksInput struct {
	Tasks          []AddTaskInput
	OrchestratorID string
}

// AddTasksResult contains the outcome of batch task creation.
type AddTasksResult struct {
	Results []AddTaskItemResult
}

// AddTaskItemResult contains the outcome of adding a single task in a batch.
type AddTaskItemResult struct {
	TaskID   string
	Success  bool
	Error    string // empty on success
	Warnings []string
}

// AddTasks adds multiple tasks in a single call. Each task is added
// independently; failed tasks don't block subsequent ones.
func AddTasks(statePath, logPath string, input *AddTasksInput) (*AddTasksResult, error) {
	if len(input.Tasks) == 0 {
		return nil, fmt.Errorf("at least one task is required")
	}
	orchestratorID := input.OrchestratorID
	if orchestratorID == "" {
		orchestratorID = "orchestrator-1"
	}
	result := &AddTasksResult{Results: make([]AddTaskItemResult, 0, len(input.Tasks))}
	for i := range input.Tasks {
		r, err := AddTask(statePath, logPath, &input.Tasks[i], orchestratorID)
		item := AddTaskItemResult{TaskID: input.Tasks[i].ID}
		if err != nil {
			// PostWriteValidationError means the task was persisted but state
			// validation failed. State is suspect — halt the batch and propagate
			// as a top-level error so the MCP layer can classify it properly.
			var postWriteErr *PostWriteValidationError
			if errors.As(err, &postWriteErr) {
				item.Error = err.Error()
				result.Results = append(result.Results, item)
				return result, err
			}
			item.Error = err.Error()
		} else {
			item.Success = true
			item.TaskID = r.TaskID
			item.Warnings = r.Warnings
		}
		result.Results = append(result.Results, item)
	}
	return result, nil
}

func initialTaskStatus(rolePair string) models.TaskStatus {
	if rolePair == "code-planning-pair" {
		return models.TaskStatusDraftCodingPlan
	}
	return models.TaskStatusReady
}
