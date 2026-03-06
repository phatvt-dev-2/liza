package ops

import (
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestSetTaskOutput_Validation(t *testing.T) {
	tests := []struct {
		name        string
		input       SetTaskOutputInput
		errContains string
	}{
		{
			name:        "empty task ID",
			input:       SetTaskOutputInput{AgentID: "coder-1", Output: []models.OutputEntry{{Desc: "d", DoneWhen: "dw", Scope: "s"}}},
			errContains: "task_id is required",
		},
		{
			name:        "empty agent ID",
			input:       SetTaskOutputInput{TaskID: "t1", Output: []models.OutputEntry{{Desc: "d", DoneWhen: "dw", Scope: "s"}}},
			errContains: "agent_id is required",
		},
		{
			name:        "empty output",
			input:       SetTaskOutputInput{TaskID: "t1", AgentID: "coder-1"},
			errContains: "output is required",
		},
		{
			name:        "output entry missing desc",
			input:       SetTaskOutputInput{TaskID: "t1", AgentID: "coder-1", Output: []models.OutputEntry{{DoneWhen: "dw", Scope: "s"}}},
			errContains: "output[0].desc is required",
		},
		{
			name:        "output entry missing done_when",
			input:       SetTaskOutputInput{TaskID: "t1", AgentID: "coder-1", Output: []models.OutputEntry{{Desc: "d", Scope: "s"}}},
			errContains: "output[0].done_when is required",
		},
		{
			name:        "output entry missing scope",
			input:       SetTaskOutputInput{TaskID: "t1", AgentID: "coder-1", Output: []models.OutputEntry{{Desc: "d", DoneWhen: "dw"}}},
			errContains: "output[0].scope is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetTaskOutput("/nonexistent", &tt.input)
			testhelpers.RequireErrorContains(t, err, tt.errContains)
		})
	}
}

func TestSetTaskOutput_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	err := SetTaskOutput(tmpDir, &SetTaskOutputInput{
		TaskID:  "nonexistent",
		AgentID: "coder-1",
		Output:  []models.OutputEntry{{Desc: "d", DoneWhen: "dw", Scope: "s"}},
	})
	testhelpers.RequireErrorContains(t, err, "task nonexistent not found")
}

func TestSetTaskOutput_WrongStatus(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	now := time.Now().UTC()

	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	err := SetTaskOutput(tmpDir, &SetTaskOutputInput{
		TaskID:  "task-1",
		AgentID: "coder-1",
		Output:  []models.OutputEntry{{Desc: "d", DoneWhen: "dw", Scope: "s"}},
	})
	testhelpers.RequireErrorContains(t, err, "not in an executing state")
}

func TestSetTaskOutput_WrongAgent(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	now := time.Now().UTC()

	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	err := SetTaskOutput(tmpDir, &SetTaskOutputInput{
		TaskID:  "task-1",
		AgentID: "coder-99",
		Output:  []models.OutputEntry{{Desc: "d", DoneWhen: "dw", Scope: "s"}},
	})
	testhelpers.RequireErrorContains(t, err, "not assigned to agent coder-99")
}

func TestSetTaskOutput_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	now := time.Now().UTC()

	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	output := []models.OutputEntry{
		{Desc: "implement feature X", DoneWhen: "tests pass", Scope: "pkg/x", SpecRef: "specs/x.md"},
		{Desc: "implement feature Y", DoneWhen: "linter green", Scope: "pkg/y", SpecRef: "specs/y.md"},
	}

	err := SetTaskOutput(tmpDir, &SetTaskOutputInput{
		TaskID:  "task-1",
		AgentID: "coder-1",
		Output:  output,
	})
	if err != nil {
		t.Fatalf("SetTaskOutput() unexpected error: %v", err)
	}

	// Verify output was persisted
	bb := db.For(stateFile)
	stateAfter, err := bb.ReadCached()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := stateAfter.FindTask("task-1")
	if task == nil {
		t.Fatal("task-1 not found after SetTaskOutput")
	}
	if len(task.Output) != 2 {
		t.Fatalf("Expected 2 output entries, got %d", len(task.Output))
	}
	if task.Output[0].Desc != "implement feature X" {
		t.Errorf("Output[0].Desc = %q, want %q", task.Output[0].Desc, "implement feature X")
	}
	if task.Output[1].SpecRef != "specs/y.md" {
		t.Errorf("Output[1].SpecRef = %q, want %q", task.Output[1].SpecRef, "specs/y.md")
	}
}

func TestSetTaskOutput_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	now := time.Now().UTC()

	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	first := []models.OutputEntry{
		{Desc: "old task", DoneWhen: "old", Scope: "old"},
	}
	second := []models.OutputEntry{
		{Desc: "new task A", DoneWhen: "new A", Scope: "scope A"},
		{Desc: "new task B", DoneWhen: "new B", Scope: "scope B"},
	}

	// First call
	if err := SetTaskOutput(tmpDir, &SetTaskOutputInput{TaskID: "task-1", AgentID: "coder-1", Output: first}); err != nil {
		t.Fatalf("First SetTaskOutput() error: %v", err)
	}

	// Second call overwrites
	if err := SetTaskOutput(tmpDir, &SetTaskOutputInput{TaskID: "task-1", AgentID: "coder-1", Output: second}); err != nil {
		t.Fatalf("Second SetTaskOutput() error: %v", err)
	}

	bb := db.For(stateFile)
	stateAfter, err := bb.ReadCached()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := stateAfter.FindTask("task-1")
	if len(task.Output) != 2 {
		t.Fatalf("Expected 2 output entries (overwritten), got %d", len(task.Output))
	}
	if task.Output[0].Desc != "new task A" {
		t.Errorf("Output[0].Desc = %q, want %q", task.Output[0].Desc, "new task A")
	}
}

func TestSetTaskOutput_CodePlanningStatus(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	now := time.Now().UTC()

	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusCodePlanning, now)
	agent := "code-planner-1"
	task.AssignedTo = &agent
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	err := SetTaskOutput(tmpDir, &SetTaskOutputInput{
		TaskID:  "task-1",
		AgentID: "code-planner-1",
		Output:  []models.OutputEntry{{Desc: "d", DoneWhen: "dw", Scope: "s"}},
	})
	if err != nil {
		t.Fatalf("SetTaskOutput() for CODE_PLANNING task: unexpected error: %v", err)
	}
}
