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

// --- Proceed: happy path ---

func TestProceed_CreatesChildTasks(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	parentID := "plan-task-1"
	reviewCommit := "abc123"
	task := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "code-planning-pair",
		Description:  "Plan the auth module",
		Status:       models.TaskStatus("CODING_PLAN_APPROVED"),
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Plan approved",
		Scope:        "auth module",
		ReviewCommit: &reviewCommit,
		Output: []models.OutputEntry{
			{Desc: "Implement login", DoneWhen: "POST /login works", Scope: "auth", SpecRef: "specs/auth.md#login"},
			{Desc: "Implement refresh", DoneWhen: "POST /refresh works", Scope: "auth", SpecRef: "specs/auth.md#refresh"},
		},
		History: []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Proceed(tmpDir, parentID, "code-plan-to-coding")
	if err != nil {
		t.Fatalf("Proceed() error: %v", err)
	}

	if result.SourceTaskID != parentID {
		t.Errorf("SourceTaskID = %q, want %q", result.SourceTaskID, parentID)
	}
	if result.TransitionName != "code-plan-to-coding" {
		t.Errorf("TransitionName = %q, want %q", result.TransitionName, "code-plan-to-coding")
	}
	if len(result.ChildTaskIDs) != 2 {
		t.Fatalf("ChildTaskIDs count = %d, want 2", len(result.ChildTaskIDs))
	}

	expectedID0 := "plan-task-1-code-plan-to-coding-0"
	expectedID1 := "plan-task-1-code-plan-to-coding-1"
	if result.ChildTaskIDs[0] != expectedID0 {
		t.Errorf("ChildTaskIDs[0] = %q, want %q", result.ChildTaskIDs[0], expectedID0)
	}
	if result.ChildTaskIDs[1] != expectedID1 {
		t.Errorf("ChildTaskIDs[1] = %q, want %q", result.ChildTaskIDs[1], expectedID1)
	}

	// Verify persisted state
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	// Source task unchanged status, transitions_executed set
	srcTask := readState.FindTask(parentID)
	if srcTask == nil {
		t.Fatal("Source task not found")
	}
	if srcTask.Status != models.TaskStatus("CODING_PLAN_APPROVED") {
		t.Errorf("Source status = %v, want CODING_PLAN_APPROVED", srcTask.Status)
	}
	if !srcTask.TransitionsExecuted["code-plan-to-coding"] {
		t.Error("transitions_executed should contain code-plan-to-coding")
	}

	// Child task 0
	child0 := readState.FindTask(expectedID0)
	if child0 == nil {
		t.Fatal("Child task 0 not found")
	}
	if child0.Status != models.TaskStatus("DRAFT_CODE") {
		t.Errorf("Child 0 status = %v, want DRAFT_CODE", child0.Status)
	}
	if child0.ParentTask == nil || *child0.ParentTask != parentID {
		t.Errorf("Child 0 parent_task = %v, want %q", child0.ParentTask, parentID)
	}
	if child0.Description != "Implement login" {
		t.Errorf("Child 0 desc = %q, want %q", child0.Description, "Implement login")
	}
	if child0.DoneWhen != "POST /login works" {
		t.Errorf("Child 0 done_when = %q", child0.DoneWhen)
	}
	if child0.Scope != "auth" {
		t.Errorf("Child 0 scope = %q", child0.Scope)
	}
	if child0.SpecRef != "specs/auth.md#login" {
		t.Errorf("Child 0 spec_ref = %q", child0.SpecRef)
	}

	// Child task 1
	child1 := readState.FindTask(expectedID1)
	if child1 == nil {
		t.Fatal("Child task 1 not found")
	}
	if child1.Status != models.TaskStatus("DRAFT_CODE") {
		t.Errorf("Child 1 status = %v, want DRAFT_CODE", child1.Status)
	}
	if child1.ParentTask == nil || *child1.ParentTask != parentID {
		t.Errorf("Child 1 parent_task = %v, want %q", child1.ParentTask, parentID)
	}
}

// --- Proceed: idempotency rejection ---

func TestProceed_RejectsRepeatedTransition(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	parentID := "plan-1"
	reviewCommit := "abc123"
	task := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "code-planning-pair",
		Description:  "Plan task",
		Status:       models.TaskStatus("CODING_PLAN_APPROVED"),
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Approved",
		Scope:        "scope",
		ReviewCommit: &reviewCommit,
		Output: []models.OutputEntry{
			{Desc: "Child", DoneWhen: "Done", Scope: "s", SpecRef: "README.md"},
		},
		TransitionsExecuted: map[string]bool{"code-plan-to-coding": true},
		History:             []models.TaskHistoryEntry{},
	}
	// Child already exists — transition was fully completed
	child := models.Task{
		ID:          "plan-1-code-plan-to-coding-0",
		Type:        models.TaskTypeCoding,
		RolePair:    "coding-pair",
		Description: "Child",
		Status:      models.TaskStatus("DRAFT_CODE"),
		Priority:    1,
		Created:     now,
		ParentTask:  &parentID,
		SpecRef:     "README.md",
		DoneWhen:    "Done",
		Scope:       "s",
		History:     []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task, child)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Proceed(tmpDir, parentID, "code-plan-to-coding")
	if err == nil {
		t.Fatal("Expected error for repeated transition")
	}
	if !strings.Contains(err.Error(), "already executed") {
		t.Errorf("Error = %q, want to contain 'already executed'", err.Error())
	}
}

// --- Proceed: sprint not COMPLETED ---

func TestProceed_RejectsIfSprintNotCompleted(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusInProgress

	now := time.Now().UTC()
	reviewCommit := "abc123"
	task := models.Task{
		ID:           "plan-1",
		Type:         models.TaskTypeCoding,
		Description:  "Plan task",
		Status:       models.TaskStatusMerged,
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Approved",
		Scope:        "scope",
		ReviewCommit: &reviewCommit,
		Output: []models.OutputEntry{
			{Desc: "Child", DoneWhen: "Done", Scope: "s", SpecRef: "README.md"},
		},
		History: []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{"plan-1"}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Proceed(tmpDir, "plan-1", "code-plan-to-coding")
	if err == nil {
		t.Fatal("Expected error for non-COMPLETED sprint")
	}
	if !strings.Contains(err.Error(), "COMPLETED") {
		t.Errorf("Error = %q, want to contain 'COMPLETED'", err.Error())
	}
}

// --- Proceed: unknown transition ---

func TestProceed_RejectsUnknownTransition(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	reviewCommit := "abc123"
	task := models.Task{
		ID:           "plan-1",
		Type:         models.TaskTypeCoding,
		Description:  "Plan task",
		Status:       models.TaskStatusMerged,
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Approved",
		Scope:        "scope",
		ReviewCommit: &reviewCommit,
		Output: []models.OutputEntry{
			{Desc: "Child", DoneWhen: "Done", Scope: "s", SpecRef: "README.md"},
		},
		History: []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{"plan-1"}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Proceed(tmpDir, "plan-1", "unknown-transition")
	if err == nil {
		t.Fatal("Expected error for unknown transition")
	}
	if !strings.Contains(err.Error(), "unknown transition") {
		t.Errorf("Error = %q, want to contain 'unknown transition'", err.Error())
	}
}

// --- Proceed: task not found ---

func TestProceed_RejectsNonexistentTask(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCompleted
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Proceed(tmpDir, "nonexistent", "code-plan-to-coding")
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error = %q, want to contain 'not found'", err.Error())
	}
}

// --- Proceed: wrong status ---

func TestProceed_RejectsWrongStatus(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	task := models.Task{
		ID:           "plan-1",
		Type:         models.TaskTypeCoding,
		RolePair:     "code-planning-pair",
		Description:  "Plan task",
		Status:       models.TaskStatus("CODE_PLANNING"),
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Done",
		Scope:        "scope",
		AssignedTo:   testhelpers.StringPtr("coder-1"),
		Worktree:     testhelpers.StringPtr(".worktrees/plan-1"),
		BaseCommit:   testhelpers.StringPtr("abc123"),
		LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
		History:      []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{"plan-1"}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Proceed(tmpDir, "plan-1", "code-plan-to-coding")
	if err == nil {
		t.Fatal("Expected error for wrong status")
	}
	if !strings.Contains(err.Error(), "CODING_PLAN_APPROVED") {
		t.Errorf("Error = %q, want to contain 'CODING_PLAN_APPROVED'", err.Error())
	}
}

// --- Proceed: empty output ---

func TestProceed_RejectsEmptyOutput(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	reviewCommit := "abc123"
	task := models.Task{
		ID:           "plan-1",
		Type:         models.TaskTypeCoding,
		RolePair:     "code-planning-pair",
		Description:  "Plan task",
		Status:       models.TaskStatus("CODING_PLAN_APPROVED"),
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Approved",
		Scope:        "scope",
		ReviewCommit: &reviewCommit,
		Output:       []models.OutputEntry{},
		History:      []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{"plan-1"}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Proceed(tmpDir, "plan-1", "code-plan-to-coding")
	if err == nil {
		t.Fatal("Expected error for empty output")
	}
	if !strings.Contains(err.Error(), "output") {
		t.Errorf("Error = %q, want to contain 'output'", err.Error())
	}
}

// --- Proceed: crash recovery ---

func TestProceed_CrashRecovery_CreatesMissingChildren(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	parentID := "plan-1"
	reviewCommit := "abc123"
	parentTask := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "code-planning-pair",
		Description:  "Plan task",
		Status:       models.TaskStatus("CODING_PLAN_APPROVED"),
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Approved",
		Scope:        "scope",
		ReviewCommit: &reviewCommit,
		Output: []models.OutputEntry{
			{Desc: "Child 0", DoneWhen: "Done 0", Scope: "s0", SpecRef: "README.md"},
			{Desc: "Child 1", DoneWhen: "Done 1", Scope: "s1", SpecRef: "README.md"},
		},
		TransitionsExecuted: map[string]bool{"code-plan-to-coding": true},
		History:             []models.TaskHistoryEntry{},
	}

	// Simulate crash: transition marked but only first child created
	child0 := models.Task{
		ID:          "plan-1-code-plan-to-coding-0",
		Type:        models.TaskTypeCoding,
		RolePair:    "coding-pair",
		Description: "Child 0",
		Status:      models.TaskStatus("DRAFT_CODE"),
		Priority:    1,
		Created:     now,
		ParentTask:  &parentID,
		SpecRef:     "README.md",
		DoneWhen:    "Done 0",
		Scope:       "s0",
		History:     []models.TaskHistoryEntry{},
	}

	state.Tasks = append(state.Tasks, parentTask, child0)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Proceed(tmpDir, parentID, "code-plan-to-coding")
	if err != nil {
		t.Fatalf("Proceed() error (crash recovery): %v", err)
	}

	// Should only create the missing child
	if len(result.ChildTaskIDs) != 1 {
		t.Fatalf("ChildTaskIDs count = %d, want 1 (only missing)", len(result.ChildTaskIDs))
	}
	if result.ChildTaskIDs[0] != "plan-1-code-plan-to-coding-1" {
		t.Errorf("ChildTaskIDs[0] = %q, want %q", result.ChildTaskIDs[0], "plan-1-code-plan-to-coding-1")
	}

	// Verify child 1 now exists
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	child1 := readState.FindTask("plan-1-code-plan-to-coding-1")
	if child1 == nil {
		t.Fatal("Child task 1 not found after crash recovery")
	}
	if child1.Description != "Child 1" {
		t.Errorf("Child 1 desc = %q, want %q", child1.Description, "Child 1")
	}
}

// --- Proceed: crash recovery with all children present ---

func TestProceed_CrashRecovery_AllChildrenExist(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	parentID := "plan-1"
	reviewCommit := "abc123"
	parentTask := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "code-planning-pair",
		Description:  "Plan task",
		Status:       models.TaskStatus("CODING_PLAN_APPROVED"),
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Approved",
		Scope:        "scope",
		ReviewCommit: &reviewCommit,
		Output: []models.OutputEntry{
			{Desc: "Child 0", DoneWhen: "Done 0", Scope: "s0", SpecRef: "README.md"},
		},
		TransitionsExecuted: map[string]bool{"code-plan-to-coding": true},
		History:             []models.TaskHistoryEntry{},
	}

	child0 := models.Task{
		ID:          "plan-1-code-plan-to-coding-0",
		Type:        models.TaskTypeCoding,
		RolePair:    "coding-pair",
		Description: "Child 0",
		Status:      models.TaskStatus("DRAFT_CODE"),
		Priority:    1,
		Created:     now,
		ParentTask:  &parentID,
		SpecRef:     "README.md",
		DoneWhen:    "Done 0",
		Scope:       "s0",
		History:     []models.TaskHistoryEntry{},
	}

	state.Tasks = append(state.Tasks, parentTask, child0)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Proceed(tmpDir, parentID, "code-plan-to-coding")
	if err == nil {
		t.Fatal("Expected error when all children already exist")
	}
	if !strings.Contains(err.Error(), "already executed") {
		t.Errorf("Error = %q, want to contain 'already executed'", err.Error())
	}
}

// --- Proceed: output entry validation ---

func TestProceed_RejectsOutputMissingFields(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	reviewCommit := "abc123"
	task := models.Task{
		ID:           "plan-1",
		Type:         models.TaskTypeCoding,
		RolePair:     "code-planning-pair",
		Description:  "Plan task",
		Status:       models.TaskStatus("CODING_PLAN_APPROVED"),
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Approved",
		Scope:        "scope",
		ReviewCommit: &reviewCommit,
		Output: []models.OutputEntry{
			{Desc: "", DoneWhen: "Done", Scope: "s", SpecRef: "README.md"}, // missing desc
		},
		History: []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{"plan-1"}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Proceed(tmpDir, "plan-1", "code-plan-to-coding")
	if err == nil {
		t.Fatal("Expected error for output entry missing desc")
	}
	if !strings.Contains(err.Error(), "output") {
		t.Errorf("Error = %q, want to contain 'output'", err.Error())
	}
}

// --- Pipeline-aware Proceed tests ---

// setupPipelineProceedTest creates a test dir with frozen pipeline config.
func setupPipelineProceedTest(t *testing.T) (string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	// Copy the valid pipeline YAML to .liza/pipeline.yaml (frozen config).
	src, err := os.ReadFile(filepath.Join(testhelpers.FindRepoRoot(t), "internal", "pipeline", "testdata", "valid-coding-subpipeline.yaml"))
	if err != nil {
		t.Fatalf("Failed to read pipeline testdata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".liza", "pipeline.yaml"), src, 0o644); err != nil {
		t.Fatalf("Failed to write frozen pipeline config: %v", err)
	}

	return tmpDir, stateFile
}

func TestProceed_PipelineCreatesChildTasksWithRolePair(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	parentID := "plan-task-1"
	reviewCommit := "abc123"
	task := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "code-planning-pair",
		Description:  "Plan the auth module",
		Status:       models.TaskStatus("CODING_PLAN_APPROVED"),
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Plan approved",
		Scope:        "auth module",
		ReviewCommit: &reviewCommit,
		Output: []models.OutputEntry{
			{Desc: "Implement login", DoneWhen: "POST /login works", Scope: "auth", SpecRef: "specs/auth.md#login"},
			{Desc: "Implement refresh", DoneWhen: "POST /refresh works", Scope: "auth", SpecRef: "specs/auth.md#refresh"},
		},
		History: []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Proceed(tmpDir, parentID, "code-plan-to-coding")
	if err != nil {
		t.Fatalf("Proceed() error: %v", err)
	}

	if len(result.ChildTaskIDs) != 2 {
		t.Fatalf("ChildTaskIDs count = %d, want 2", len(result.ChildTaskIDs))
	}

	// Verify persisted state
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	// Child task 0: should have pipeline status and role_pair
	child0 := readState.FindTask(result.ChildTaskIDs[0])
	if child0 == nil {
		t.Fatal("Child task 0 not found")
	}
	if child0.Status != models.TaskStatus("DRAFT_CODE") {
		t.Errorf("Child 0 status = %v, want DRAFT_CODE", child0.Status)
	}
	if child0.RolePair != "coding-pair" {
		t.Errorf("Child 0 role_pair = %q, want %q", child0.RolePair, "coding-pair")
	}
	if child0.ParentTask == nil || *child0.ParentTask != parentID {
		t.Errorf("Child 0 parent_task = %v, want %q", child0.ParentTask, parentID)
	}
	if child0.Description != "Implement login" {
		t.Errorf("Child 0 desc = %q, want %q", child0.Description, "Implement login")
	}

	// Child task 1
	child1 := readState.FindTask(result.ChildTaskIDs[1])
	if child1 == nil {
		t.Fatal("Child task 1 not found")
	}
	if child1.Status != models.TaskStatus("DRAFT_CODE") {
		t.Errorf("Child 1 status = %v, want DRAFT_CODE", child1.Status)
	}
	if child1.RolePair != "coding-pair" {
		t.Errorf("Child 1 role_pair = %q, want %q", child1.RolePair, "coding-pair")
	}
	if child1.ParentTask == nil || *child1.ParentTask != parentID {
		t.Errorf("Child 1 parent_task = %v, want %q", child1.ParentTask, parentID)
	}

	// Source task status should be unchanged
	srcTask := readState.FindTask(parentID)
	if srcTask.Status != models.TaskStatus("CODING_PLAN_APPROVED") {
		t.Errorf("Source status = %v, want CODING_PLAN_APPROVED", srcTask.Status)
	}
	if !srcTask.TransitionsExecuted["code-plan-to-coding"] {
		t.Error("transitions_executed should contain code-plan-to-coding")
	}
}

func TestAvailableTransitions_PipelineTask(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	now := time.Now().UTC()
	task := models.Task{
		ID:          "plan-1",
		Type:        models.TaskTypeCoding,
		RolePair:    "code-planning-pair",
		Description: "Plan task",
		Status:      models.TaskStatus("CODING_PLAN_APPROVED"),
		Priority:    1,
		Created:     now,
		SpecRef:     "README.md",
		DoneWhen:    "Done",
		Scope:       "scope",
		History:     []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	testhelpers.WriteInitialState(t, stateFile, state)

	avail := AvailableTransitions(&state.Tasks[len(state.Tasks)-1], tmpDir)
	if len(avail) != 1 || avail[0] != "code-plan-to-coding" {
		t.Errorf("AvailableTransitions = %v, want [code-plan-to-coding]", avail)
	}
}

func TestProceed_PipelineRejectsAutoTransition(t *testing.T) {
	// Create a pipeline config with an auto transition to verify proceed rejects it.
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	autoYAML := `pipeline:
  roles:
    code-planner:
      type: doer
      display-name: "Code Planner"
    code-plan-reviewer:
      type: reviewer
      display-name: "Code Plan Reviewer"
    coder:
      type: doer
      display-name: "Coder"
    code-reviewer:
      type: reviewer
      display-name: "Code Reviewer"

  role-pairs:
    code-planning-pair:
      doer: code-planner
      reviewer: code-plan-reviewer
      states:
        initial: DRAFT_CODING_PLAN
        executing: CODE_PLANNING
        submitted: CODING_PLAN_TO_REVIEW
        reviewing: REVIEWING_CODING_PLAN
        approved: CODING_PLAN_APPROVED
        rejected: CODING_PLAN_REJECTED

    coding-pair:
      doer: coder
      reviewer: code-reviewer
      states:
        initial: DRAFT_CODE
        executing: IMPLEMENTING_CODE
        submitted: CODE_READY_FOR_REVIEW
        reviewing: REVIEWING_CODE
        approved: CODE_APPROVED
        rejected: CODE_REJECTED

  sub-pipelines:
    coding-subpipeline:
      steps:
        - code-planning-pair
        - coding-pair
      transitions:
        - name: code-plan-to-coding
          from: code-planning-pair.approved
          to: coding-pair.initial
          trigger: auto
          cardinality: per-subtask

  entry-points:
    detailed-spec: coding-subpipeline.code-planning-pair
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".liza", "pipeline.yaml"), []byte(autoYAML), 0o644); err != nil {
		t.Fatalf("Failed to write pipeline config: %v", err)
	}

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	parentID := "plan-1"
	reviewCommit := "abc123"
	task := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "code-planning-pair",
		Description:  "Plan task",
		Status:       models.TaskStatus("CODING_PLAN_APPROVED"),
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Plan approved",
		Scope:        "scope",
		ReviewCommit: &reviewCommit,
		Output: []models.OutputEntry{
			{Desc: "Child", DoneWhen: "Done", Scope: "s", SpecRef: "README.md"},
		},
		History: []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Proceed(tmpDir, parentID, "code-plan-to-coding")
	if err == nil {
		t.Fatal("Expected error for auto transition via proceed")
	}
	if !strings.Contains(err.Error(), "manual") {
		t.Errorf("Error = %q, want to mention 'manual'", err.Error())
	}
	if !strings.Contains(err.Error(), "auto") {
		t.Errorf("Error = %q, want to mention 'auto'", err.Error())
	}
}

// --- Phase 2 pipeline test helpers ---

// setupPhase2PipelineProceedTest creates a test dir with the full Phase 2 pipeline config
// (including pipeline-transitions with us-to-coding one-to-one transition).
func setupPhase2PipelineProceedTest(t *testing.T) (string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	// Copy the full Phase 2 pipeline YAML to .liza/pipeline.yaml (frozen config).
	src, err := os.ReadFile(filepath.Join(testhelpers.FindRepoRoot(t), "internal", "pipeline", "testdata", "valid-phase2-full.yaml"))
	if err != nil {
		t.Fatalf("Failed to read pipeline testdata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".liza", "pipeline.yaml"), src, 0o644); err != nil {
		t.Fatalf("Failed to write frozen pipeline config: %v", err)
	}

	return tmpDir, stateFile
}

// --- Proceed: one-to-one cardinality ---

func TestProceed_OneToOne_CreatesSingleChild(t *testing.T) {
	tmpDir, stateFile := setupPhase2PipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	parentID := "us-task-1"
	reviewCommit := "abc123"
	task := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "us-writing-pair",
		Description:  "User authentication story",
		Status:       models.TaskStatus("US_APPROVED"),
		Priority:     1,
		Created:      now,
		SpecRef:      "specs/auth.md",
		DoneWhen:     "US approved",
		Scope:        "auth module",
		ReviewCommit: &reviewCommit,
		History:      []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Proceed(tmpDir, parentID, "us-to-coding")
	if err != nil {
		t.Fatalf("Proceed() error: %v", err)
	}

	// Should create exactly one child
	if len(result.ChildTaskIDs) != 1 {
		t.Fatalf("ChildTaskIDs count = %d, want 1", len(result.ChildTaskIDs))
	}

	// Child ID follows <parent>-<transition-name> pattern (no index)
	expectedChildID := "us-task-1-us-to-coding"
	if result.ChildTaskIDs[0] != expectedChildID {
		t.Errorf("ChildTaskIDs[0] = %q, want %q", result.ChildTaskIDs[0], expectedChildID)
	}

	// Verify persisted state
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	child := readState.FindTask(expectedChildID)
	if child == nil {
		t.Fatal("Child task not found")
	}

	// Child has correct status (target pair's initial state)
	if child.Status != models.TaskStatus("DRAFT_CODING_PLAN") {
		t.Errorf("Child status = %v, want DRAFT_CODING_PLAN", child.Status)
	}
	// Child has correct role_pair (target pair)
	if child.RolePair != "code-planning-pair" {
		t.Errorf("Child role_pair = %q, want %q", child.RolePair, "code-planning-pair")
	}
	// Child has parent_task set
	if child.ParentTask == nil || *child.ParentTask != parentID {
		t.Errorf("Child parent_task = %v, want %q", child.ParentTask, parentID)
	}
	// Child desc contains doer display name and parent description
	if !strings.Contains(child.Description, "Code Planner") {
		t.Errorf("Child desc = %q, want to contain 'Code Planner'", child.Description)
	}
	if !strings.Contains(child.Description, "User authentication story") {
		t.Errorf("Child desc = %q, want to contain parent description", child.Description)
	}
	// Child spec_ref is parent's spec_ref
	if child.SpecRef != "specs/auth.md" {
		t.Errorf("Child spec_ref = %q, want %q", child.SpecRef, "specs/auth.md")
	}
	// Child scope references parent
	if !strings.Contains(child.Scope, parentID) {
		t.Errorf("Child scope = %q, want to contain parent ID %q", child.Scope, parentID)
	}

	// Source task unchanged status, transitions_executed set
	srcTask := readState.FindTask(parentID)
	if srcTask.Status != models.TaskStatus("US_APPROVED") {
		t.Errorf("Source status = %v, want US_APPROVED", srcTask.Status)
	}
	if !srcTask.TransitionsExecuted["us-to-coding"] {
		t.Error("transitions_executed should contain us-to-coding")
	}
}

func TestProceed_OneToOne_CrashRecovery_ChildExists(t *testing.T) {
	tmpDir, stateFile := setupPhase2PipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	parentID := "us-task-1"
	reviewCommit := "abc123"
	parentTask := models.Task{
		ID:                  parentID,
		Type:                models.TaskTypeCoding,
		RolePair:            "us-writing-pair",
		Description:         "User authentication story",
		Status:              models.TaskStatus("US_APPROVED"),
		Priority:            1,
		Created:             now,
		SpecRef:             "specs/auth.md",
		DoneWhen:            "US approved",
		Scope:               "auth module",
		ReviewCommit:        &reviewCommit,
		TransitionsExecuted: map[string]bool{"us-to-coding": true},
		History:             []models.TaskHistoryEntry{},
	}

	// Child already exists — transition was fully completed
	childID := "us-task-1-us-to-coding"
	child := models.Task{
		ID:          childID,
		Type:        models.TaskTypeCoding,
		RolePair:    "code-planning-pair",
		Description: "Code Planner task for: User authentication story",
		Status:      models.TaskStatus("DRAFT_CODING_PLAN"),
		Priority:    1,
		Created:     now,
		ParentTask:  &parentID,
		SpecRef:     "specs/auth.md",
		DoneWhen:    "done",
		Scope:       "scope",
		History:     []models.TaskHistoryEntry{},
	}

	state.Tasks = append(state.Tasks, parentTask, child)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Proceed(tmpDir, parentID, "us-to-coding")
	if err == nil {
		t.Fatal("Expected error for repeated one-to-one transition")
	}
	if !strings.Contains(err.Error(), "already executed") {
		t.Errorf("Error = %q, want to contain 'already executed'", err.Error())
	}
}

func TestProceed_OneToOne_CrashRecovery_ChildMissing(t *testing.T) {
	tmpDir, stateFile := setupPhase2PipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	parentID := "us-task-1"
	reviewCommit := "abc123"
	parentTask := models.Task{
		ID:                  parentID,
		Type:                models.TaskTypeCoding,
		RolePair:            "us-writing-pair",
		Description:         "User authentication story",
		Status:              models.TaskStatus("US_APPROVED"),
		Priority:            1,
		Created:             now,
		SpecRef:             "specs/auth.md",
		DoneWhen:            "US approved",
		Scope:               "auth module",
		ReviewCommit:        &reviewCommit,
		TransitionsExecuted: map[string]bool{"us-to-coding": true},
		History:             []models.TaskHistoryEntry{},
	}

	// Crash scenario: transition marked but child NOT created
	state.Tasks = append(state.Tasks, parentTask)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Proceed(tmpDir, parentID, "us-to-coding")
	if err != nil {
		t.Fatalf("Proceed() error (crash recovery): %v", err)
	}

	// Should create the missing child
	if len(result.ChildTaskIDs) != 1 {
		t.Fatalf("ChildTaskIDs count = %d, want 1", len(result.ChildTaskIDs))
	}
	expectedChildID := "us-task-1-us-to-coding"
	if result.ChildTaskIDs[0] != expectedChildID {
		t.Errorf("ChildTaskIDs[0] = %q, want %q", result.ChildTaskIDs[0], expectedChildID)
	}

	// Verify child now exists
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	child := readState.FindTask(expectedChildID)
	if child == nil {
		t.Fatal("Child task not found after crash recovery")
	}
}

// --- resolvePhaseRef: 3-part refs ---

func TestResolvePhaseRef_3PartRef(t *testing.T) {
	tmpDir, _ := setupPhase2PipelineProceedTest(t)

	resolver, _, err := loadResolver(tmpDir)
	if err != nil {
		t.Fatalf("loadResolver error: %v", err)
	}

	// 3-part ref: epic-spec-subpipeline.us-writing-pair.approved → US_APPROVED
	status, err := resolvePhaseRef(resolver, "epic-spec-subpipeline.us-writing-pair.approved")
	if err != nil {
		t.Fatalf("resolvePhaseRef error: %v", err)
	}
	if status != models.TaskStatus("US_APPROVED") {
		t.Errorf("resolvePhaseRef = %v, want US_APPROVED", status)
	}
}

func TestResolvePhaseRef_3PartRef_Initial(t *testing.T) {
	tmpDir, _ := setupPhase2PipelineProceedTest(t)

	resolver, _, err := loadResolver(tmpDir)
	if err != nil {
		t.Fatalf("loadResolver error: %v", err)
	}

	// 3-part ref: coding-subpipeline.code-planning-pair.initial → DRAFT_CODING_PLAN
	status, err := resolvePhaseRef(resolver, "coding-subpipeline.code-planning-pair.initial")
	if err != nil {
		t.Fatalf("resolvePhaseRef error: %v", err)
	}
	if status != models.TaskStatus("DRAFT_CODING_PLAN") {
		t.Errorf("resolvePhaseRef = %v, want DRAFT_CODING_PLAN", status)
	}
}

func TestResolvePhaseRef_2PartRef_StillWorks(t *testing.T) {
	tmpDir, _ := setupPhase2PipelineProceedTest(t)

	resolver, _, err := loadResolver(tmpDir)
	if err != nil {
		t.Fatalf("loadResolver error: %v", err)
	}

	// 2-part ref: code-planning-pair.approved → CODING_PLAN_APPROVED
	status, err := resolvePhaseRef(resolver, "code-planning-pair.approved")
	if err != nil {
		t.Fatalf("resolvePhaseRef error: %v", err)
	}
	if status != models.TaskStatus("CODING_PLAN_APPROVED") {
		t.Errorf("resolvePhaseRef = %v, want CODING_PLAN_APPROVED", status)
	}
}

// --- AvailableTransitions: pipeline-transitions ---

func TestAvailableTransitions_PipelineTransition_USApproved(t *testing.T) {
	tmpDir, stateFile := setupPhase2PipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	now := time.Now().UTC()
	task := models.Task{
		ID:          "us-1",
		Type:        models.TaskTypeCoding,
		RolePair:    "us-writing-pair",
		Description: "US task",
		Status:      models.TaskStatus("US_APPROVED"),
		Priority:    1,
		Created:     now,
		SpecRef:     "README.md",
		DoneWhen:    "Done",
		Scope:       "scope",
		History:     []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	testhelpers.WriteInitialState(t, stateFile, state)

	avail := AvailableTransitions(&state.Tasks[len(state.Tasks)-1], tmpDir)
	if len(avail) != 1 || avail[0] != "us-to-coding" {
		t.Errorf("AvailableTransitions = %v, want [us-to-coding]", avail)
	}
}

func TestAvailableTransitions_PipelineExcludesExecuted(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	now := time.Now().UTC()
	task := models.Task{
		ID:                  "plan-1",
		Type:                models.TaskTypeCoding,
		RolePair:            "code-planning-pair",
		Description:         "Plan task",
		Status:              models.TaskStatus("CODING_PLAN_APPROVED"),
		Priority:            1,
		Created:             now,
		SpecRef:             "README.md",
		DoneWhen:            "Done",
		Scope:               "scope",
		TransitionsExecuted: map[string]bool{"code-plan-to-coding": true},
		History:             []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	testhelpers.WriteInitialState(t, stateFile, state)

	avail := AvailableTransitions(&state.Tasks[len(state.Tasks)-1], tmpDir)
	if len(avail) != 0 {
		t.Errorf("AvailableTransitions = %v, want [] (already executed)", avail)
	}
}

// --- ExecuteAvailableTransitions tests ---

func TestExecuteAvailableTransitions_CreatesChildrenForMergedTasks(t *testing.T) {
	tmpDir, stateFile := setupPhase2PipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusInProgress

	now := time.Now().UTC()
	parentID := "us-task-1"
	reviewCommit := "abc123"
	mergeCommit := "def456"
	task := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "us-writing-pair",
		Description:  "User authentication story",
		Status:       models.TaskStatusMerged,
		Priority:     1,
		Created:      now,
		SpecRef:      "specs/auth.md",
		DoneWhen:     "US approved",
		Scope:        "auth module",
		ReviewCommit: &reviewCommit,
		MergeCommit:  &mergeCommit,
		History:      []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	results, err := ExecuteAvailableTransitions(tmpDir)
	if err != nil {
		t.Fatalf("ExecuteAvailableTransitions() error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}
	if results[0].SourceTaskID != parentID {
		t.Errorf("SourceTaskID = %q, want %q", results[0].SourceTaskID, parentID)
	}
	if results[0].TransitionName != "us-to-coding" {
		t.Errorf("TransitionName = %q, want %q", results[0].TransitionName, "us-to-coding")
	}
	if len(results[0].ChildTaskIDs) != 1 {
		t.Fatalf("ChildTaskIDs count = %d, want 1", len(results[0].ChildTaskIDs))
	}

	// Verify child exists in state
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	childID := results[0].ChildTaskIDs[0]
	child := readState.FindTask(childID)
	if child == nil {
		t.Fatal("Child task not found in state.Tasks")
	}
	if child.RolePair != "code-planning-pair" {
		t.Errorf("Child role_pair = %q, want %q", child.RolePair, "code-planning-pair")
	}
	if child.Status != models.TaskStatus("DRAFT_CODING_PLAN") {
		t.Errorf("Child status = %v, want DRAFT_CODING_PLAN", child.Status)
	}

	// Children MUST be in Sprint.Scope.Planned
	found := false
	for _, id := range readState.Sprint.Scope.Planned {
		if id == childID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Child %q should be in Sprint.Scope.Planned", childID)
	}

	// Source task should have transition marked
	srcTask := readState.FindTask(parentID)
	if !srcTask.TransitionsExecuted["us-to-coding"] {
		t.Error("transitions_executed should contain us-to-coding")
	}
}

func TestExecuteAvailableTransitions_PerSubtask(t *testing.T) {
	tmpDir, stateFile := setupPhase2PipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusInProgress

	now := time.Now().UTC()
	parentID := "plan-task-1"
	reviewCommit := "abc123"
	mergeCommit := "def456"
	task := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "code-planning-pair",
		Description:  "Plan the auth module",
		Status:       models.TaskStatusMerged,
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Plan approved",
		Scope:        "auth module",
		ReviewCommit: &reviewCommit,
		MergeCommit:  &mergeCommit,
		Output: []models.OutputEntry{
			{Desc: "Implement login", DoneWhen: "POST /login works", Scope: "auth", SpecRef: "specs/auth.md#login"},
			{Desc: "Implement refresh", DoneWhen: "POST /refresh works", Scope: "auth", SpecRef: "specs/auth.md#refresh"},
		},
		History: []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	results, err := ExecuteAvailableTransitions(tmpDir)
	if err != nil {
		t.Fatalf("ExecuteAvailableTransitions() error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}
	if len(results[0].ChildTaskIDs) != 2 {
		t.Fatalf("ChildTaskIDs count = %d, want 2", len(results[0].ChildTaskIDs))
	}

	// Verify children exist
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	for _, childID := range results[0].ChildTaskIDs {
		child := readState.FindTask(childID)
		if child == nil {
			t.Fatalf("Child task %q not found in state.Tasks", childID)
		}
		if child.RolePair != "coding-pair" {
			t.Errorf("Child %q role_pair = %q, want %q", childID, child.RolePair, "coding-pair")
		}
		// Must be in sprint scope
		childFound := false
		for _, id := range readState.Sprint.Scope.Planned {
			if id == childID {
				childFound = true
				break
			}
		}
		if !childFound {
			t.Errorf("Child %q should be in Sprint.Scope.Planned", childID)
		}
	}
}

func TestExecuteAvailableTransitions_NoopWhenNoTransitions(t *testing.T) {
	tmpDir, stateFile := setupPhase2PipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusInProgress

	now := time.Now().UTC()
	reviewCommit := "abc123"
	mergeCommit := "def456"
	// coding-pair has no outgoing transitions — should produce no results
	task := models.Task{
		ID:           "code-task-1",
		Type:         models.TaskTypeCoding,
		RolePair:     "coding-pair",
		Description:  "Implement login",
		Status:       models.TaskStatusMerged,
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Done",
		Scope:        "scope",
		ReviewCommit: &reviewCommit,
		MergeCommit:  &mergeCommit,
		History:      []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	testhelpers.WriteInitialState(t, stateFile, state)

	results, err := ExecuteAvailableTransitions(tmpDir)
	if err != nil {
		t.Fatalf("ExecuteAvailableTransitions() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("results count = %d, want 0 (no transitions for coding-pair)", len(results))
	}
}

func TestExecuteAvailableTransitions_Idempotent(t *testing.T) {
	tmpDir, stateFile := setupPhase2PipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusInProgress

	now := time.Now().UTC()
	parentID := "us-task-1"
	reviewCommit := "abc123"
	mergeCommit := "def456"
	childID := "us-task-1-us-to-coding"
	task := models.Task{
		ID:                  parentID,
		Type:                models.TaskTypeCoding,
		RolePair:            "us-writing-pair",
		Description:         "User authentication story",
		Status:              models.TaskStatusMerged,
		Priority:            1,
		Created:             now,
		SpecRef:             "specs/auth.md",
		DoneWhen:            "US approved",
		Scope:               "auth module",
		ReviewCommit:        &reviewCommit,
		MergeCommit:         &mergeCommit,
		TransitionsExecuted: map[string]bool{"us-to-coding": true},
		History:             []models.TaskHistoryEntry{},
	}
	child := models.Task{
		ID:          childID,
		Type:        models.TaskTypeCoding,
		RolePair:    "code-planning-pair",
		Description: "Code Planner task for: User authentication story",
		Status:      models.TaskStatus("DRAFT_CODING_PLAN"),
		Priority:    1,
		Created:     now,
		ParentTask:  &parentID,
		SpecRef:     "specs/auth.md",
		DoneWhen:    "done",
		Scope:       "scope",
		History:     []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task, child)
	testhelpers.WriteInitialState(t, stateFile, state)

	results, err := ExecuteAvailableTransitions(tmpDir)
	if err != nil {
		t.Fatalf("ExecuteAvailableTransitions() error: %v", err)
	}
	// Transition already executed and child exists → skipped
	if len(results) != 0 {
		t.Errorf("results count = %d, want 0 (idempotent)", len(results))
	}
}

func TestExecuteAvailableTransitions_NoSprintGate(t *testing.T) {
	// Verify ExecuteAvailableTransitions works regardless of sprint status
	tmpDir, stateFile := setupPhase2PipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusInProgress // NOT COMPLETED

	now := time.Now().UTC()
	parentID := "us-task-1"
	reviewCommit := "abc123"
	mergeCommit := "def456"
	task := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "us-writing-pair",
		Description:  "User authentication story",
		Status:       models.TaskStatusMerged,
		Priority:     1,
		Created:      now,
		SpecRef:      "specs/auth.md",
		DoneWhen:     "US approved",
		Scope:        "auth module",
		ReviewCommit: &reviewCommit,
		MergeCommit:  &mergeCommit,
		History:      []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	// Should succeed even though sprint is IN_PROGRESS (not COMPLETED)
	results, err := ExecuteAvailableTransitions(tmpDir)
	if err != nil {
		t.Fatalf("ExecuteAvailableTransitions() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}
}

// TestExecuteAvailableTransitions_ScopeDedupGuard verifies the defensive dedup
// guard when adding children to Sprint.Scope.Planned.
//
// Note: crash recovery through ExecuteAvailableTransitions is unreachable because
// AvailableTransitions filters out transitions already in TransitionsExecuted.
// The recoverCrashedTransition path is only reachable through direct Proceed calls.
// This test validates the dedup guard against a pre-seeded scope entry.
func TestExecuteAvailableTransitions_ScopeDedupGuard(t *testing.T) {
	tmpDir, stateFile := setupPhase2PipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusInProgress

	now := time.Now().UTC()
	parentID := "plan-task-1"
	reviewCommit := "abc123"
	mergeCommit := "def456"

	// Predictable child IDs based on the naming convention
	child0ID := "plan-task-1-code-plan-to-coding-0"
	child1ID := "plan-task-1-code-plan-to-coding-1"

	task := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "code-planning-pair",
		Description:  "Plan the auth module",
		Status:       models.TaskStatusMerged,
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Plan approved",
		Scope:        "auth module",
		ReviewCommit: &reviewCommit,
		MergeCommit:  &mergeCommit,
		Output: []models.OutputEntry{
			{Desc: "Implement login", DoneWhen: "POST /login works", Scope: "auth", SpecRef: "specs/auth.md#login"},
			{Desc: "Implement refresh", DoneWhen: "POST /refresh works", Scope: "auth", SpecRef: "specs/auth.md#refresh"},
		},
		History: []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	// Artificially pre-seed a child ID in scope to validate the dedup guard
	state.Sprint.Scope.Planned = []string{parentID, child0ID}
	testhelpers.WriteInitialState(t, stateFile, state)

	results, err := ExecuteAvailableTransitions(tmpDir)
	if err != nil {
		t.Fatalf("ExecuteAvailableTransitions() error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}
	if len(results[0].ChildTaskIDs) != 2 {
		t.Fatalf("ChildTaskIDs count = %d, want 2", len(results[0].ChildTaskIDs))
	}

	// Verify sprint scope: each child ID exactly once, no duplicates
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	child0Count := 0
	child1Count := 0
	for _, id := range readState.Sprint.Scope.Planned {
		if id == child0ID {
			child0Count++
		}
		if id == child1ID {
			child1Count++
		}
	}
	if child0Count != 1 {
		t.Errorf("child %q appears %d times in Sprint.Scope.Planned, want 1 (dedup failed)", child0ID, child0Count)
	}
	if child1Count != 1 {
		t.Errorf("child %q appears %d times in Sprint.Scope.Planned, want 1", child1ID, child1Count)
	}
}

// --- Proceed: sprint scope update ---

func TestProceed_AddsChildrenToSprintScope(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	parentID := "plan-task-1"
	reviewCommit := "abc123"
	task := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "code-planning-pair",
		Description:  "Plan the auth module",
		Status:       models.TaskStatus("CODING_PLAN_APPROVED"),
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Plan approved",
		Scope:        "auth module",
		ReviewCommit: &reviewCommit,
		Output: []models.OutputEntry{
			{Desc: "Implement login", DoneWhen: "POST /login works", Scope: "auth", SpecRef: "specs/auth.md#login"},
			{Desc: "Implement refresh", DoneWhen: "POST /refresh works", Scope: "auth", SpecRef: "specs/auth.md#refresh"},
		},
		History: []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Proceed(tmpDir, parentID, "code-plan-to-coding")
	if err != nil {
		t.Fatalf("Proceed() error: %v", err)
	}

	// Verify children are in sprint scope
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	for _, childID := range result.ChildTaskIDs {
		found := false
		for _, id := range readState.Sprint.Scope.Planned {
			if id == childID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Child %q not found in Sprint.Scope.Planned", childID)
		}
	}
}

// --- DependsOn tests ---

func TestProceed_DependsOnResolvesToChildTaskIDs(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	parentID := "plan-deps-1"
	reviewCommit := "abc123"
	task := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "code-planning-pair",
		Description:  "Plan with dependencies",
		Status:       models.TaskStatus("CODING_PLAN_APPROVED"),
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Plan approved",
		Scope:        "deps test",
		ReviewCommit: &reviewCommit,
		Output: []models.OutputEntry{
			{Desc: "Setup DB", DoneWhen: "DB ready", Scope: "db", SpecRef: "specs/db.md"},
			{Desc: "Build API", DoneWhen: "API works", Scope: "api", SpecRef: "specs/api.md", DependsOn: []string{"0"}},
			{Desc: "Build UI", DoneWhen: "UI works", Scope: "ui", SpecRef: "specs/ui.md", DependsOn: []string{"0", "1"}},
		},
		History: []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Proceed(tmpDir, parentID, "code-plan-to-coding")
	if err != nil {
		t.Fatalf("Proceed() error: %v", err)
	}

	if len(result.ChildTaskIDs) != 3 {
		t.Fatalf("ChildTaskIDs count = %d, want 3", len(result.ChildTaskIDs))
	}

	// Verify persisted child tasks have correct DependsOn
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	child0ID := parentID + "-code-plan-to-coding-0"
	child1ID := parentID + "-code-plan-to-coding-1"
	child2ID := parentID + "-code-plan-to-coding-2"

	child0 := readState.FindTask(child0ID)
	child1 := readState.FindTask(child1ID)
	child2 := readState.FindTask(child2ID)

	if child0 == nil || child1 == nil || child2 == nil {
		t.Fatal("One or more child tasks not found")
	}

	// Child 0: no dependencies
	if len(child0.DependsOn) != 0 {
		t.Errorf("child0.DependsOn = %v, want empty", child0.DependsOn)
	}

	// Child 1: depends on child 0
	if len(child1.DependsOn) != 1 || child1.DependsOn[0] != child0ID {
		t.Errorf("child1.DependsOn = %v, want [%s]", child1.DependsOn, child0ID)
	}

	// Child 2: depends on child 0 and child 1
	if len(child2.DependsOn) != 2 || child2.DependsOn[0] != child0ID || child2.DependsOn[1] != child1ID {
		t.Errorf("child2.DependsOn = %v, want [%s, %s]", child2.DependsOn, child0ID, child1ID)
	}
}

func TestProceed_DependsOnInvalidIndex(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	parentID := "plan-bad-dep"
	reviewCommit := "abc123"
	task := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "code-planning-pair",
		Description:  "Plan with bad dep",
		Status:       models.TaskStatus("CODING_PLAN_APPROVED"),
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Plan approved",
		Scope:        "bad dep test",
		ReviewCommit: &reviewCommit,
		Output: []models.OutputEntry{
			{Desc: "Task A", DoneWhen: "Done", Scope: "s", SpecRef: "specs/a.md"},
			{Desc: "Task B", DoneWhen: "Done", Scope: "s", SpecRef: "specs/b.md", DependsOn: []string{"5"}},
		},
		History: []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Proceed(tmpDir, parentID, "code-plan-to-coding")
	if err == nil {
		t.Fatal("Expected error for out-of-range DependsOn index")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("Error = %q, want to contain 'out of range'", err.Error())
	}
}

func TestProceed_DependsOnNonNumeric(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	parentID := "plan-nan-dep"
	reviewCommit := "abc123"
	task := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "code-planning-pair",
		Description:  "Plan with non-numeric dep",
		Status:       models.TaskStatus("CODING_PLAN_APPROVED"),
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Plan approved",
		Scope:        "nan dep test",
		ReviewCommit: &reviewCommit,
		Output: []models.OutputEntry{
			{Desc: "Task A", DoneWhen: "Done", Scope: "s", SpecRef: "specs/a.md", DependsOn: []string{"abc"}},
		},
		History: []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Proceed(tmpDir, parentID, "code-plan-to-coding")
	if err == nil {
		t.Fatal("Expected error for non-numeric DependsOn")
	}
	if !strings.Contains(err.Error(), "non-numeric") {
		t.Errorf("Error = %q, want to contain 'non-numeric'", err.Error())
	}
}

func TestProceed_DependsOnSelfReference(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	parentID := "plan-self-dep"
	reviewCommit := "abc123"
	task := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "code-planning-pair",
		Description:  "Plan with self dep",
		Status:       models.TaskStatus("CODING_PLAN_APPROVED"),
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Plan approved",
		Scope:        "self dep test",
		ReviewCommit: &reviewCommit,
		Output: []models.OutputEntry{
			{Desc: "Task A", DoneWhen: "Done", Scope: "s", SpecRef: "specs/a.md", DependsOn: []string{"0"}},
		},
		History: []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Proceed(tmpDir, parentID, "code-plan-to-coding")
	if err == nil {
		t.Fatal("Expected error for self-referencing DependsOn")
	}
	if !strings.Contains(err.Error(), "references itself") {
		t.Errorf("Error = %q, want to contain 'references itself'", err.Error())
	}
}

func TestProceed_ChildTasksGetPlanRefFromOutputEntry(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	parentID := "plan-planref-1"
	reviewCommit := "abc123"
	task := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "code-planning-pair",
		Description:  "Plan with plan_ref",
		Status:       models.TaskStatus("CODING_PLAN_APPROVED"),
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Plan approved",
		Scope:        "auth module",
		ReviewCommit: &reviewCommit,
		Output: []models.OutputEntry{
			{Desc: "Task A", DoneWhen: "A works", Scope: "a", SpecRef: "specs/a.md", PlanRef: "specs/plans/20260317-plan.md"},
			{Desc: "Task B", DoneWhen: "B works", Scope: "b", SpecRef: "specs/b.md", PlanRef: "specs/plans/20260317-plan.md"},
		},
		History: []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Proceed(tmpDir, parentID, "code-plan-to-coding")
	if err != nil {
		t.Fatalf("Proceed() error: %v", err)
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	for i, childID := range result.ChildTaskIDs {
		child := readState.FindTask(childID)
		if child == nil {
			t.Fatalf("Child task %d not found", i)
		}
		if child.PlanRef != "specs/plans/20260317-plan.md" {
			t.Errorf("Child %d plan_ref = %q, want %q", i, child.PlanRef, "specs/plans/20260317-plan.md")
		}
	}
}

func TestProceed_OneToOne_InheritsPlanRefFromParent(t *testing.T) {
	tmpDir, stateFile := setupPhase2PipelineProceedTest(t)

	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusCompleted

	now := time.Now().UTC()
	parentID := "us-planref-1"
	reviewCommit := "abc123"
	task := models.Task{
		ID:           parentID,
		Type:         models.TaskTypeCoding,
		RolePair:     "us-writing-pair",
		Description:  "US with plan_ref",
		Status:       models.TaskStatus("US_APPROVED"),
		Priority:     1,
		Created:      now,
		SpecRef:      "specs/auth-epic.md",
		PlanRef:      "specs/auth-epic.md",
		DoneWhen:     "US approved",
		Scope:        "auth module",
		ReviewCommit: &reviewCommit,
		History:      []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Proceed(tmpDir, parentID, "us-to-coding")
	if err != nil {
		t.Fatalf("Proceed() error: %v", err)
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	child := readState.FindTask(result.ChildTaskIDs[0])
	if child == nil {
		t.Fatal("Child task not found")
	}
	if child.PlanRef != "specs/auth-epic.md" {
		t.Errorf("Child plan_ref = %q, want %q", child.PlanRef, "specs/auth-epic.md")
	}
}

// --- computeInheritedDeps tests ---

func TestComputeInheritedDeps_UpstreamExecutedSameTransition(t *testing.T) {
	tmpDir, _ := setupPipelineProceedTest(t)
	resolver, _, err := loadResolver(tmpDir)
	if err != nil {
		t.Fatalf("loadResolver: %v", err)
	}

	// Upstream plan-1 executed code-plan-to-coding with 3 output entries → 3 children exist
	upstreamID := "plan-1"
	transitionName := "code-plan-to-coding"
	s := &models.State{
		Tasks: []models.Task{
			{
				ID:       upstreamID,
				Status:   models.TaskStatusMerged,
				RolePair: "code-planning-pair",
				Output: []models.OutputEntry{
					{Desc: "a", DoneWhen: "a", Scope: "a", SpecRef: "s.md"},
					{Desc: "b", DoneWhen: "b", Scope: "b", SpecRef: "s.md"},
					{Desc: "c", DoneWhen: "c", Scope: "c", SpecRef: "s.md"},
				},
				TransitionsExecuted: map[string]bool{transitionName: true},
			},
			// The 3 children that were created by the transition
			{ID: perSubtaskChildID(upstreamID, transitionName, 0), Status: models.TaskStatus("DRAFT_CODE")},
			{ID: perSubtaskChildID(upstreamID, transitionName, 1), Status: models.TaskStatus("DRAFT_CODE")},
			{ID: perSubtaskChildID(upstreamID, transitionName, 2), Status: models.TaskStatus("DRAFT_CODE")},
			// Downstream plan-2 depends on plan-1
			{
				ID:        "plan-2",
				Status:    models.TaskStatusMerged,
				RolePair:  "code-planning-pair",
				DependsOn: []string{upstreamID},
			},
		},
	}

	downstream := s.FindTask("plan-2")
	inherited, err := computeInheritedDeps(s, downstream, transitionName, resolver)
	if err != nil {
		t.Fatalf("computeInheritedDeps: %v", err)
	}

	if len(inherited) != 3 {
		t.Fatalf("inherited count = %d, want 3", len(inherited))
	}
	for i := 0; i < 3; i++ {
		expected := perSubtaskChildID(upstreamID, transitionName, i)
		if inherited[i] != expected {
			t.Errorf("inherited[%d] = %q, want %q", i, inherited[i], expected)
		}
	}
}

func TestComputeInheritedDeps_UpstreamDidNotExecuteTransition(t *testing.T) {
	tmpDir, _ := setupPipelineProceedTest(t)
	resolver, _, err := loadResolver(tmpDir)
	if err != nil {
		t.Fatalf("loadResolver: %v", err)
	}

	s := &models.State{
		Tasks: []models.Task{
			{
				ID:       "plan-1",
				Status:   models.TaskStatusMerged,
				RolePair: "code-planning-pair",
				// No TransitionsExecuted — hasn't been transitioned yet
			},
			{
				ID:        "plan-2",
				Status:    models.TaskStatusMerged,
				RolePair:  "code-planning-pair",
				DependsOn: []string{"plan-1"},
			},
		},
	}

	downstream := s.FindTask("plan-2")
	inherited, err := computeInheritedDeps(s, downstream, "code-plan-to-coding", resolver)
	if err != nil {
		t.Fatalf("computeInheritedDeps: %v", err)
	}
	if len(inherited) != 0 {
		t.Errorf("inherited count = %d, want 0 (upstream hasn't executed transition)", len(inherited))
	}
}

func TestComputeInheritedDeps_UpstreamExecutedDifferentTransition(t *testing.T) {
	tmpDir, _ := setupPipelineProceedTest(t)
	resolver, _, err := loadResolver(tmpDir)
	if err != nil {
		t.Fatalf("loadResolver: %v", err)
	}

	// Upstream executed auto-code-plan-to-coding (one-to-one), not code-plan-to-coding
	s := &models.State{
		Tasks: []models.Task{
			{
				ID:                  "plan-1",
				Status:              models.TaskStatusMerged,
				RolePair:            "code-planning-pair",
				TransitionsExecuted: map[string]bool{"auto-code-plan-to-coding": true},
			},
			{ID: oneToOneChildID("plan-1", "auto-code-plan-to-coding"), Status: models.TaskStatus("DRAFT_CODE")},
			{
				ID:        "plan-2",
				Status:    models.TaskStatusMerged,
				RolePair:  "code-planning-pair",
				DependsOn: []string{"plan-1"},
			},
		},
	}

	downstream := s.FindTask("plan-2")
	inherited, err := computeInheritedDeps(s, downstream, "code-plan-to-coding", resolver)
	if err != nil {
		t.Fatalf("computeInheritedDeps: %v", err)
	}
	if len(inherited) != 0 {
		t.Errorf("inherited count = %d, want 0 (upstream executed different transition)", len(inherited))
	}
}

func TestComputeInheritedDeps_MissingUpstreamChild_ReturnsError(t *testing.T) {
	tmpDir, _ := setupPipelineProceedTest(t)
	resolver, _, err := loadResolver(tmpDir)
	if err != nil {
		t.Fatalf("loadResolver: %v", err)
	}

	// Upstream says transition executed with 2 output entries, but child-1 is missing
	s := &models.State{
		Tasks: []models.Task{
			{
				ID:       "plan-1",
				Status:   models.TaskStatusMerged,
				RolePair: "code-planning-pair",
				Output: []models.OutputEntry{
					{Desc: "a", DoneWhen: "a", Scope: "a", SpecRef: "s.md"},
					{Desc: "b", DoneWhen: "b", Scope: "b", SpecRef: "s.md"},
				},
				TransitionsExecuted: map[string]bool{"code-plan-to-coding": true},
			},
			// Only child-0 exists, child-1 is missing (crash)
			{ID: perSubtaskChildID("plan-1", "code-plan-to-coding", 0), Status: models.TaskStatus("DRAFT_CODE")},
			{
				ID:        "plan-2",
				Status:    models.TaskStatusMerged,
				RolePair:  "code-planning-pair",
				DependsOn: []string{"plan-1"},
			},
		},
	}

	downstream := s.FindTask("plan-2")
	_, err = computeInheritedDeps(s, downstream, "code-plan-to-coding", resolver)
	if err == nil {
		t.Fatal("expected error for missing upstream child")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error = %q, want to contain 'missing'", err.Error())
	}
}

// --- Topological ordering tests ---

func TestEAT_TopoOrdering_UpstreamBeforeDownstream(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusInProgress

	// plan-2 depends on plan-1, but plan-2 appears FIRST in state.Tasks
	plan2 := testhelpers.BuildTaskByStatus("plan-2", models.TaskStatusMerged, now)
	plan2.RolePair = "code-planning-pair"
	plan2.Output = []models.OutputEntry{
		{Desc: "d", DoneWhen: "d", Scope: "d", SpecRef: "s.md"},
	}
	plan2.DependsOn = []string{"plan-1"}

	plan1 := testhelpers.BuildTaskByStatus("plan-1", models.TaskStatusMerged, now)
	plan1.RolePair = "code-planning-pair"
	plan1.Output = []models.OutputEntry{
		{Desc: "a", DoneWhen: "a", Scope: "a", SpecRef: "s.md"},
		{Desc: "b", DoneWhen: "b", Scope: "b", SpecRef: "s.md"},
	}

	state.Tasks = append(state.Tasks, plan2, plan1)
	state.Sprint.Scope.Planned = []string{"plan-2", "plan-1"}
	testhelpers.WriteInitialState(t, stateFile, state)

	results, err := ExecuteAvailableTransitions(tmpDir)
	if err != nil {
		t.Fatalf("ExecuteAvailableTransitions: %v", err)
	}

	// plan-1 should execute first despite appearing second in array
	if len(results) < 2 {
		t.Fatalf("results count = %d, want >= 2", len(results))
	}
	if results[0].SourceTaskID != "plan-1" {
		t.Errorf("results[0].SourceTaskID = %q, want plan-1 (upstream first)", results[0].SourceTaskID)
	}
	if results[1].SourceTaskID != "plan-2" {
		t.Errorf("results[1].SourceTaskID = %q, want plan-2 (downstream second)", results[1].SourceTaskID)
	}
}

func TestEAT_StableOrdering_UnrelatedTasksPreserveArrayOrder(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusInProgress

	// Two unrelated tasks — no dep relationship, should preserve original order
	planA := testhelpers.BuildTaskByStatus("plan-a", models.TaskStatusMerged, now)
	planA.RolePair = "code-planning-pair"
	planA.Output = []models.OutputEntry{
		{Desc: "a", DoneWhen: "a", Scope: "a", SpecRef: "s.md"},
	}

	planB := testhelpers.BuildTaskByStatus("plan-b", models.TaskStatusMerged, now)
	planB.RolePair = "code-planning-pair"
	planB.Output = []models.OutputEntry{
		{Desc: "b", DoneWhen: "b", Scope: "b", SpecRef: "s.md"},
	}

	state.Tasks = append(state.Tasks, planA, planB)
	state.Sprint.Scope.Planned = []string{"plan-a", "plan-b"}
	testhelpers.WriteInitialState(t, stateFile, state)

	results, err := ExecuteAvailableTransitions(tmpDir)
	if err != nil {
		t.Fatalf("ExecuteAvailableTransitions: %v", err)
	}

	if len(results) < 2 {
		t.Fatalf("results count = %d, want >= 2", len(results))
	}
	// plan-a appears first in Tasks array, should execute first
	if results[0].SourceTaskID != "plan-a" {
		t.Errorf("results[0].SourceTaskID = %q, want plan-a (stable order)", results[0].SourceTaskID)
	}
	if results[1].SourceTaskID != "plan-b" {
		t.Errorf("results[1].SourceTaskID = %q, want plan-b (stable order)", results[1].SourceTaskID)
	}
}

func TestEAT_CycleDetection_CyclicTasksGetHistoryEvent(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusInProgress

	// Circular dependency: plan-a depends on plan-b, plan-b depends on plan-a
	planA := testhelpers.BuildTaskByStatus("plan-a", models.TaskStatusMerged, now)
	planA.RolePair = "code-planning-pair"
	planA.Output = []models.OutputEntry{
		{Desc: "a", DoneWhen: "a", Scope: "a", SpecRef: "s.md"},
	}
	planA.DependsOn = []string{"plan-b"}

	planB := testhelpers.BuildTaskByStatus("plan-b", models.TaskStatusMerged, now)
	planB.RolePair = "code-planning-pair"
	planB.Output = []models.OutputEntry{
		{Desc: "b", DoneWhen: "b", Scope: "b", SpecRef: "s.md"},
	}
	planB.DependsOn = []string{"plan-a"}

	state.Tasks = append(state.Tasks, planA, planB)
	state.Sprint.Scope.Planned = []string{"plan-a", "plan-b"}
	testhelpers.WriteInitialState(t, stateFile, state)

	results, err := ExecuteAvailableTransitions(tmpDir)
	if err != nil {
		t.Fatalf("ExecuteAvailableTransitions: %v", err)
	}

	// Cyclic tasks should NOT execute
	if len(results) != 0 {
		t.Errorf("results count = %d, want 0 (cyclic tasks skipped)", len(results))
	}

	// Verify history events on both tasks
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	for _, taskID := range []string{"plan-a", "plan-b"} {
		task := readState.FindTask(taskID)
		if task == nil {
			t.Fatalf("task %s not found", taskID)
		}
		found := false
		for _, h := range task.History {
			if h.Event == models.TaskEventTransitionCycleBlocked {
				found = true
				if trans, ok := h.Extra["transition"].(string); !ok || trans != "code-plan-to-coding" {
					t.Errorf("task %s cycle event transition = %v, want code-plan-to-coding", taskID, h.Extra["transition"])
				}
			}
		}
		if !found {
			t.Errorf("task %s missing transition_cycle_blocked history event", taskID)
		}
	}
}

func TestEAT_CycleDetection_Idempotent(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusInProgress

	planA := testhelpers.BuildTaskByStatus("plan-a", models.TaskStatusMerged, now)
	planA.RolePair = "code-planning-pair"
	planA.Output = []models.OutputEntry{
		{Desc: "a", DoneWhen: "a", Scope: "a", SpecRef: "s.md"},
	}
	planA.DependsOn = []string{"plan-b"}

	planB := testhelpers.BuildTaskByStatus("plan-b", models.TaskStatusMerged, now)
	planB.RolePair = "code-planning-pair"
	planB.Output = []models.OutputEntry{
		{Desc: "b", DoneWhen: "b", Scope: "b", SpecRef: "s.md"},
	}
	planB.DependsOn = []string{"plan-a"}

	state.Tasks = append(state.Tasks, planA, planB)
	state.Sprint.Scope.Planned = []string{"plan-a", "plan-b"}
	testhelpers.WriteInitialState(t, stateFile, state)

	// Run twice
	_, _ = ExecuteAvailableTransitions(tmpDir)
	_, _ = ExecuteAvailableTransitions(tmpDir)

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	for _, taskID := range []string{"plan-a", "plan-b"} {
		task := readState.FindTask(taskID)
		count := 0
		for _, h := range task.History {
			if h.Event == models.TaskEventTransitionCycleBlocked {
				count++
			}
		}
		if count != 1 {
			t.Errorf("task %s has %d cycle-blocked events, want 1 (idempotent)", taskID, count)
		}
	}
}

func TestEAT_CrashRecoveryThroughEAT(t *testing.T) {
	tmpDir, stateFile := setupPipelineProceedTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	state.Sprint.Status = models.SprintStatusInProgress

	// Upstream task with TransitionsExecuted set but child-1 missing (crash)
	plan1 := testhelpers.BuildTaskByStatus("plan-1", models.TaskStatusMerged, now)
	plan1.RolePair = "code-planning-pair"
	plan1.Output = []models.OutputEntry{
		{Desc: "a", DoneWhen: "a", Scope: "a", SpecRef: "s.md"},
		{Desc: "b", DoneWhen: "b", Scope: "b", SpecRef: "s.md"},
	}
	plan1.TransitionsExecuted = map[string]bool{"code-plan-to-coding": true}

	// Only child-0 exists
	child0 := testhelpers.BuildTaskByStatus(perSubtaskChildID("plan-1", "code-plan-to-coding", 0), models.TaskStatus("DRAFT_CODE"), now)
	child0.RolePair = "coding-pair"

	state.Tasks = append(state.Tasks, plan1, child0)
	state.Sprint.Scope.Planned = []string{"plan-1", child0.ID}
	testhelpers.WriteInitialState(t, stateFile, state)

	results, err := ExecuteAvailableTransitions(tmpDir)
	if err != nil {
		t.Fatalf("ExecuteAvailableTransitions: %v", err)
	}

	// Should recover the missing child
	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1 (crash recovery)", len(results))
	}
	if len(results[0].ChildTaskIDs) != 1 {
		t.Fatalf("recovered children = %d, want 1", len(results[0].ChildTaskIDs))
	}
	expectedChild1 := perSubtaskChildID("plan-1", "code-plan-to-coding", 1)
	if results[0].ChildTaskIDs[0] != expectedChild1 {
		t.Errorf("recovered child = %q, want %q", results[0].ChildTaskIDs[0], expectedChild1)
	}
}
