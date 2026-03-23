package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// buildMergedPlanningTask creates a merged planning task with unconsumed output,
// suitable for replan tests.
func buildMergedPlanningTask(id string, now time.Time) models.Task {
	task := testhelpers.BuildTaskByStatus(id, models.TaskStatusMerged, now)
	task.RolePair = "code-planning-pair"
	task.PlanRef = "specs/plan.md"
	task.Output = []models.OutputEntry{
		{Desc: "implement feature X", DoneWhen: "tests pass", Scope: "internal/", SpecRef: "README.md"},
	}
	return task
}

func setupReplanTest(t *testing.T) (string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n")
	testhelpers.CreateSpecFile(t, tmpDir, "plan.md", "# Plan\n")
	// BuildTaskByStatus uses SpecRef: "README.md" (at project root, not specs/)
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# README\n"), 0644); err != nil {
		t.Fatalf("Failed to create README.md: %v", err)
	}
	return tmpDir, stateFile
}

func TestReplan_Validation(t *testing.T) {
	tests := []struct {
		name    string
		input   ReplanInput
		wantErr string
	}{
		{
			name:    "empty changed_by",
			input:   ReplanInput{ChangedBy: ""},
			wantErr: "changed_by is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Replan("/nonexistent", &tt.input)
			testhelpers.RequireErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestReplan_HappyPath(t *testing.T) {
	tmpDir, stateFile := setupReplanTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint
	state.Sprint.CheckpointTrigger = models.CheckpointTriggerPlanningComplete

	planningTask := buildMergedPlanningTask("code-planning-1", now)
	state.Tasks = []models.Task{planningTask}
	state.Sprint.Scope.Planned = []string{"code-planning-1"}

	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Replan(tmpDir, &ReplanInput{
		TaskID:    "code-planning-1",
		ChangedBy: "human",
	})
	if err != nil {
		t.Fatalf("Replan() error: %v", err)
	}

	if result.OriginalTaskID != "code-planning-1" {
		t.Errorf("OriginalTaskID = %q, want %q", result.OriginalTaskID, "code-planning-1")
	}
	if result.NewTaskID != "code-planning-1-replan-1" {
		t.Errorf("NewTaskID = %q, want %q", result.NewTaskID, "code-planning-1-replan-1")
	}
	if result.RolePair != "code-planning-pair" {
		t.Errorf("RolePair = %q, want %q", result.RolePair, "code-planning-pair")
	}

	// Verify persisted state
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	// Old task invalidated
	oldTask := readState.FindTask("code-planning-1")
	if oldTask == nil {
		t.Fatal("Old task not found")
	}
	if !oldTask.TransitionsExecuted["replanned"] {
		t.Error("Old task TransitionsExecuted should have replanned=true")
	}
	if len(oldTask.History) == 0 {
		t.Fatal("Old task should have history entries")
	}
	lastEntry := oldTask.History[len(oldTask.History)-1]
	if lastEntry.Event != models.TaskEventReplanned {
		t.Errorf("Last history event = %q, want %q", lastEntry.Event, models.TaskEventReplanned)
	}

	// New task created
	newTask := readState.FindTask("code-planning-1-replan-1")
	if newTask == nil {
		t.Fatal("New task not found")
	}
	if newTask.RolePair != "code-planning-pair" {
		t.Errorf("New task RolePair = %q, want %q", newTask.RolePair, "code-planning-pair")
	}
	if newTask.SpecRef != oldTask.SpecRef {
		t.Errorf("New task SpecRef = %q, want %q", newTask.SpecRef, oldTask.SpecRef)
	}
	if newTask.PlanRef != oldTask.PlanRef {
		t.Errorf("New task PlanRef = %q, want %q", newTask.PlanRef, oldTask.PlanRef)
	}
	if newTask.DoneWhen != oldTask.DoneWhen {
		t.Errorf("New task DoneWhen = %q, want %q", newTask.DoneWhen, oldTask.DoneWhen)
	}
	if newTask.Scope != oldTask.Scope {
		t.Errorf("New task Scope = %q, want %q", newTask.Scope, oldTask.Scope)
	}
	if newTask.Supersedes == nil || *newTask.Supersedes != "code-planning-1" {
		t.Errorf("New task Supersedes = %v, want code-planning-1", newTask.Supersedes)
	}

	// Sprint resumed
	if readState.Sprint.Status != models.SprintStatusInProgress {
		t.Errorf("Sprint.Status = %v, want IN_PROGRESS", readState.Sprint.Status)
	}
	if readState.Sprint.CheckpointTrigger != "" {
		t.Errorf("Sprint.CheckpointTrigger = %q, want empty", readState.Sprint.CheckpointTrigger)
	}

	// New task in sprint scope
	found := false
	for _, id := range readState.Sprint.Scope.Planned {
		if id == "code-planning-1-replan-1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Sprint.Scope.Planned = %v, should include code-planning-1-replan-1", readState.Sprint.Scope.Planned)
	}

	// Alignment history
	if len(readState.Goal.AlignmentHistory) == 0 {
		t.Fatal("AlignmentHistory should have entries")
	}
	lastAlignment := readState.Goal.AlignmentHistory[len(readState.Goal.AlignmentHistory)-1]
	if lastAlignment.Event != "replan" {
		t.Errorf("Last alignment event = %q, want %q", lastAlignment.Event, "replan")
	}
}

func TestReplan_AutoDetection_SingleMatch(t *testing.T) {
	tmpDir, stateFile := setupReplanTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint

	planningTask := buildMergedPlanningTask("code-planning-1", now)
	state.Tasks = []models.Task{planningTask}
	state.Sprint.Scope.Planned = []string{"code-planning-1"}

	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Replan(tmpDir, &ReplanInput{ChangedBy: "human"})
	if err != nil {
		t.Fatalf("Replan() error: %v", err)
	}
	if result.OriginalTaskID != "code-planning-1" {
		t.Errorf("OriginalTaskID = %q, want %q", result.OriginalTaskID, "code-planning-1")
	}
}

func TestReplan_AutoDetection_MultipleMatches(t *testing.T) {
	tmpDir, stateFile := setupReplanTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint

	task1 := buildMergedPlanningTask("code-planning-1", now)
	task2 := buildMergedPlanningTask("code-planning-2", now)
	state.Tasks = []models.Task{task1, task2}
	state.Sprint.Scope.Planned = []string{"code-planning-1", "code-planning-2"}

	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Replan(tmpDir, &ReplanInput{ChangedBy: "human"})
	testhelpers.RequireErrorContains(t, err, "multiple planning tasks found")
}

func TestReplan_AutoDetection_ZeroMatches(t *testing.T) {
	tmpDir, stateFile := setupReplanTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint

	// Merged coding task (not a planning pair) — should not match
	codingTask := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
	state.Tasks = []models.Task{codingTask}
	state.Sprint.Scope.Planned = []string{"task-1"}

	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Replan(tmpDir, &ReplanInput{ChangedBy: "human"})
	testhelpers.RequireErrorContains(t, err, "no planning task with unconsumed output")
}

func TestReplan_WrongSprintStatus(t *testing.T) {
	tmpDir, stateFile := setupReplanTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusInProgress

	planningTask := buildMergedPlanningTask("code-planning-1", now)
	state.Tasks = []models.Task{planningTask}
	state.Sprint.Scope.Planned = []string{"code-planning-1"}

	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Replan(tmpDir, &ReplanInput{TaskID: "code-planning-1", ChangedBy: "human"})
	testhelpers.RequireErrorContains(t, err, "sprint must be at CHECKPOINT")
}

func TestReplan_TaskNotMerged(t *testing.T) {
	tmpDir, stateFile := setupReplanTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint

	task := testhelpers.BuildTaskByStatus("code-planning-1", models.TaskStatusBlocked, now)
	task.RolePair = "code-planning-pair"
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{"code-planning-1"}

	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Replan(tmpDir, &ReplanInput{TaskID: "code-planning-1", ChangedBy: "human"})
	testhelpers.RequireErrorContains(t, err, "must be MERGED")
}

func TestReplan_TransitionsAlreadyExecuted(t *testing.T) {
	tmpDir, stateFile := setupReplanTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint

	planningTask := buildMergedPlanningTask("code-planning-1", now)
	planningTask.TransitionsExecuted = map[string]bool{"some-transition": true}
	state.Tasks = []models.Task{planningTask}
	state.Sprint.Scope.Planned = []string{"code-planning-1"}

	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Replan(tmpDir, &ReplanInput{TaskID: "code-planning-1", ChangedBy: "human"})
	testhelpers.RequireErrorContains(t, err, "child tasks already created")
}

func TestReplan_NoOutput(t *testing.T) {
	tmpDir, stateFile := setupReplanTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint

	task := testhelpers.BuildTaskByStatus("code-planning-1", models.TaskStatusMerged, now)
	task.RolePair = "code-planning-pair"
	// No output set
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{"code-planning-1"}

	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Replan(tmpDir, &ReplanInput{TaskID: "code-planning-1", ChangedBy: "human"})
	testhelpers.RequireErrorContains(t, err, "has no output to replan")
}

func TestReplan_NotPlanningPair(t *testing.T) {
	tmpDir, stateFile := setupReplanTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint

	// Merged coding task with output — not a planning pair
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
	task.Output = []models.OutputEntry{
		{Desc: "something", DoneWhen: "done", Scope: ".", SpecRef: "README.md"},
	}
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{"task-1"}

	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Replan(tmpDir, &ReplanInput{TaskID: "task-1", ChangedBy: "human"})
	testhelpers.RequireErrorContains(t, err, "is not a planning pair")
}

func TestReplan_SecondReplanIncrementsCounter(t *testing.T) {
	tmpDir, stateFile := setupReplanTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint

	planningTask := buildMergedPlanningTask("code-planning-1", now)

	// Simulate an existing replan-1 task (already replanned once)
	replan1 := testhelpers.BuildTaskByStatus("code-planning-1-replan-1", models.TaskStatusMerged, now)
	replan1.RolePair = "code-planning-pair"
	replan1.TransitionsExecuted = map[string]bool{"replanned": true}

	state.Tasks = []models.Task{planningTask, replan1}
	state.Sprint.Scope.Planned = []string{"code-planning-1"}

	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Replan(tmpDir, &ReplanInput{TaskID: "code-planning-1", ChangedBy: "human"})
	if err != nil {
		t.Fatalf("Replan() error: %v", err)
	}
	if result.NewTaskID != "code-planning-1-replan-2" {
		t.Errorf("NewTaskID = %q, want %q", result.NewTaskID, "code-planning-1-replan-2")
	}
}

func TestReplan_TaskNotFound(t *testing.T) {
	tmpDir, stateFile := setupReplanTest(t)

	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Replan(tmpDir, &ReplanInput{TaskID: "nonexistent", ChangedBy: "human"})
	testhelpers.RequireErrorContains(t, err, "not found")
}

func TestReplan_PreservesDependsOn(t *testing.T) {
	tmpDir, stateFile := setupReplanTest(t)
	now := time.Now().UTC()

	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint

	// plan-1 is the upstream phase (already merged)
	plan1 := buildMergedPlanningTask("plan-1", now)
	plan1.TransitionsExecuted = map[string]bool{"replanned": true} // prevent transition check

	// plan-2 depends on plan-1
	plan2 := buildMergedPlanningTask("plan-2", now)
	plan2.DependsOn = []string{"plan-1"}

	state.Tasks = append(state.Tasks, plan1, plan2)
	state.Sprint.Scope.Planned = []string{"plan-1", "plan-2"}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Replan(tmpDir, &ReplanInput{TaskID: "plan-2", ChangedBy: "human"})
	if err != nil {
		t.Fatalf("Replan: %v", err)
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("read state: %v", err)
	}

	newTask := readState.FindTask(result.NewTaskID)
	if newTask == nil {
		t.Fatal("new task not found")
	}

	// DependsOn should be cloned from original
	if len(newTask.DependsOn) != 1 || newTask.DependsOn[0] != "plan-1" {
		t.Errorf("new task DependsOn = %v, want [plan-1]", newTask.DependsOn)
	}
}

func TestReplan_RetargetsDownstream(t *testing.T) {
	tmpDir, stateFile := setupReplanTest(t)
	now := time.Now().UTC()

	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint

	plan1 := buildMergedPlanningTask("plan-1", now)

	// plan-2 depends on plan-1 (non-terminal, should get retargeted)
	plan2 := testhelpers.BuildTaskByStatus("plan-2", models.TaskStatus("DRAFT_CODING_PLAN"), now)
	plan2.RolePair = "code-planning-pair"
	plan2.DependsOn = []string{"plan-1"}

	state.Tasks = append(state.Tasks, plan1, plan2)
	state.Sprint.Scope.Planned = []string{"plan-1", "plan-2"}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Replan(tmpDir, &ReplanInput{TaskID: "plan-1", ChangedBy: "human"})
	if err != nil {
		t.Fatalf("Replan: %v", err)
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("read state: %v", err)
	}

	// plan-2 should now depend on the new task, not plan-1
	downstream := readState.FindTask("plan-2")
	if downstream == nil {
		t.Fatal("plan-2 not found")
	}
	if len(downstream.DependsOn) != 1 || downstream.DependsOn[0] != result.NewTaskID {
		t.Errorf("plan-2 DependsOn = %v, want [%s]", downstream.DependsOn, result.NewTaskID)
	}
}

func TestReplan_WarnsTerminalDownstream(t *testing.T) {
	tmpDir, stateFile := setupReplanTest(t)
	now := time.Now().UTC()

	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint

	plan1 := buildMergedPlanningTask("plan-1", now)

	// plan-merged is MERGED and depends on plan-1 (terminal, should warn)
	planMerged := testhelpers.BuildTaskByStatus("plan-merged", models.TaskStatusMerged, now)
	planMerged.RolePair = "code-planning-pair"
	planMerged.DependsOn = []string{"plan-1"}

	state.Tasks = append(state.Tasks, plan1, planMerged)
	state.Sprint.Scope.Planned = []string{"plan-1", "plan-merged"}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Replan(tmpDir, &ReplanInput{TaskID: "plan-1", ChangedBy: "human"})
	if err != nil {
		t.Fatalf("Replan: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Fatal("expected warning about terminal downstream task")
	}
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "plan-merged") && strings.Contains(w, "replanned") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning mentioning plan-merged; warnings = %v", result.Warnings)
	}
}

func TestReplan_RetargetDedupes(t *testing.T) {
	tmpDir, stateFile := setupReplanTest(t)
	now := time.Now().UTC()

	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint

	plan1 := buildMergedPlanningTask("plan-1", now)

	// plan-2 has duplicate DependsOn (edge case from prior replan)
	plan2 := testhelpers.BuildTaskByStatus("plan-2", models.TaskStatus("DRAFT_CODING_PLAN"), now)
	plan2.RolePair = "code-planning-pair"
	plan2.DependsOn = []string{"plan-1", "plan-1"}

	state.Tasks = append(state.Tasks, plan1, plan2)
	state.Sprint.Scope.Planned = []string{"plan-1", "plan-2"}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Replan(tmpDir, &ReplanInput{TaskID: "plan-1", ChangedBy: "human"})
	if err != nil {
		t.Fatalf("Replan: %v", err)
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("read state: %v", err)
	}

	downstream := readState.FindTask("plan-2")
	if len(downstream.DependsOn) != 1 {
		t.Errorf("plan-2 DependsOn = %v, want exactly 1 entry (deduped)", downstream.DependsOn)
	}
	if downstream.DependsOn[0] != result.NewTaskID {
		t.Errorf("plan-2 DependsOn[0] = %q, want %q", downstream.DependsOn[0], result.NewTaskID)
	}
}
