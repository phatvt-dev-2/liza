package ops

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestAddTask_Validation(t *testing.T) {
	tests := []struct {
		name        string
		input       AddTaskInput
		errContains string
	}{
		{
			name:        "empty task ID",
			input:       AddTaskInput{Description: "d", SpecRef: "s", DoneWhen: "w", Scope: "sc", Priority: 1},
			errContains: "task ID is required",
		},
		{
			name:        "empty description",
			input:       AddTaskInput{ID: "t1", SpecRef: "s", DoneWhen: "w", Scope: "sc", Priority: 1},
			errContains: "description is required",
		},
		{
			name:        "empty spec_ref",
			input:       AddTaskInput{ID: "t1", Description: "d", DoneWhen: "w", Scope: "sc", Priority: 1},
			errContains: "spec_ref is required",
		},
		{
			name:        "empty done_when",
			input:       AddTaskInput{ID: "t1", Description: "d", SpecRef: "s", Scope: "sc", Priority: 1},
			errContains: "done_when is required",
		},
		{
			name:        "empty scope",
			input:       AddTaskInput{ID: "t1", Description: "d", SpecRef: "s", DoneWhen: "w", Priority: 1},
			errContains: "scope is required",
		},
		{
			name:        "zero priority",
			input:       AddTaskInput{ID: "t1", Description: "d", SpecRef: "s", DoneWhen: "w", Scope: "sc", Priority: 0},
			errContains: "priority must be positive",
		},
		{
			name:        "negative priority",
			input:       AddTaskInput{ID: "t1", Description: "d", SpecRef: "s", DoneWhen: "w", Scope: "sc", Priority: -1},
			errContains: "priority must be positive",
		},
		{
			name:        "invalid task type",
			input:       AddTaskInput{ID: "t1", Description: "d", SpecRef: "s", DoneWhen: "w", Scope: "sc", Priority: 1, Type: "invalid"},
			errContains: "unknown task type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := AddTask("/nonexistent", "/dev/null", &tt.input, "orchestrator-1")
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Error = %q, want to contain %q", err.Error(), tt.errContains)
			}
		})
	}
}

func TestAddTask_Success(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	logFile := filepath.Join(tmpDir, ".liza", "log.jsonl")
	testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n")
	testhelpers.CreateSpecFile(t, tmpDir, "feature-x.md", "# Feature X\n")

	state := testhelpers.CreateValidState()
	now := time.Now().UTC()
	state.Tasks = append(state.Tasks,
		models.Task{
			ID:          "dep-1",
			Description: "Dependency 1",
			Status:      models.TaskStatusMerged,
			Priority:    1,
			SpecRef:     "specs/vision.md",
			DoneWhen:    "done",
			Scope:       "scope",
			Created:     now,
			History:     []models.TaskHistoryEntry{},
		},
		models.Task{
			ID:          "dep-2",
			Description: "Dependency 2",
			Status:      models.TaskStatusMerged,
			Priority:    1,
			SpecRef:     "specs/vision.md",
			DoneWhen:    "done",
			Scope:       "scope",
			Created:     now,
			History:     []models.TaskHistoryEntry{},
		},
	)
	testhelpers.WriteInitialState(t, stateFile, state)

	input := &AddTaskInput{
		ID:          "task-1",
		Description: "Implement feature X",
		SpecRef:     "specs/feature-x.md",
		DoneWhen:    "Tests pass",
		Scope:       "internal/ops",
		Priority:    2,
		DependsOn:   []string{"dep-1", " dep-2 ", ""},
	}

	result, err := AddTask(stateFile, logFile, input, "orchestrator-1")
	if err != nil {
		t.Fatalf("AddTask() error: %v", err)
	}

	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}

	// Verify state
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := readState.FindTask("task-1")
	if task == nil {
		t.Fatal("Task not found in state")
	}
	if task.Status != models.TaskStatusReady {
		t.Errorf("Task status = %v, want READY", task.Status)
	}
	if task.Priority != 2 {
		t.Errorf("Priority = %d, want 2", task.Priority)
	}
	if task.Type != models.TaskTypeCoding {
		t.Errorf("Type = %v, want coding", task.Type)
	}
	// Verify deps normalized (trimmed, empty removed)
	if len(task.DependsOn) != 2 {
		t.Errorf("DependsOn len = %d, want 2", len(task.DependsOn))
	}
	if task.DependsOn[1] != "dep-2" {
		t.Errorf("DependsOn[1] = %q, want %q", task.DependsOn[1], "dep-2")
	}

	// Verify sprint scope updated
	found := false
	for _, id := range readState.Sprint.Scope.Planned {
		if id == "task-1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Task ID not added to Sprint.Scope.Planned")
	}

	// Verify alignment history updated
	lastAlignment := readState.Goal.AlignmentHistory[len(readState.Goal.AlignmentHistory)-1]
	if !strings.Contains(lastAlignment.Summary, "task-1") {
		t.Errorf("Alignment history summary should mention task ID, got %q", lastAlignment.Summary)
	}
}

func TestAddTask_DefaultOrchestratorID(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	logFile := filepath.Join(tmpDir, ".liza", "log.jsonl")
	testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n")

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	input := &AddTaskInput{
		ID: "task-1", Description: "d", SpecRef: "specs/vision.md",
		DoneWhen: "w", Scope: "sc", Priority: 1,
	}

	// Empty orchestratorID should default to "orchestrator-1"
	result, err := AddTask(stateFile, logFile, input, "")
	if err != nil {
		t.Fatalf("AddTask() error: %v", err)
	}
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
}

func TestAddTask_DuplicateID(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	logFile := filepath.Join(tmpDir, ".liza", "log.jsonl")

	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		{ID: "task-1", Description: "existing", Status: models.TaskStatusReady},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	input := &AddTaskInput{
		ID: "task-1", Description: "d", SpecRef: "s",
		DoneWhen: "w", Scope: "sc", Priority: 1,
	}

	_, err := AddTask(stateFile, logFile, input, "orchestrator-1")
	if err == nil {
		t.Fatal("Expected error for duplicate task ID")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Error = %q, want to contain 'already exists'", err.Error())
	}
}

func TestAddTask_PostWriteValidationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	logFile := filepath.Join(tmpDir, ".liza", "log.jsonl")
	testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n")
	testhelpers.CreateSpecFile(t, tmpDir, "feature-x.md", "# Feature X\n")

	state := testhelpers.CreateValidState()
	state.Tasks = append(state.Tasks, models.Task{
		ID:          "invalid-existing-task",
		Description: "Invalid existing task",
		Status:      models.TaskStatusImplementing, // missing assigned_to/worktree/base_commit
		Priority:    1,
		SpecRef:     "specs/vision.md",
		DoneWhen:    "done",
		Scope:       "scope",
		Created:     time.Now().UTC(),
		History:     []models.TaskHistoryEntry{},
	})
	testhelpers.WriteInitialState(t, stateFile, state)

	input := &AddTaskInput{
		ID:          "task-added-before-validation-failure",
		Description: "Task to trigger post-write validation",
		SpecRef:     "specs/feature-x.md",
		DoneWhen:    "tests pass",
		Scope:       "internal/ops",
		Priority:    1,
	}

	_, err := AddTask(stateFile, logFile, input, "orchestrator-1")
	if err == nil {
		t.Fatal("expected post-write validation error")
	}

	var validationErr *PostWriteValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected PostWriteValidationError, got %T: %v", err, err)
	}

	if !strings.Contains(err.Error(), "state validation failed") {
		t.Fatalf("error = %q, want to contain %q", err.Error(), "state validation failed")
	}
}
