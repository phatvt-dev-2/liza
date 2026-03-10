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
  agent-roles:
    code-planner: "Code Planner"
    code-plan-reviewer: "Code Plan Reviewer"
    coder: "Coder"
    code-reviewer: "Code Reviewer"

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

	// Children must NOT be in Sprint.Scope.Planned
	for _, id := range readState.Sprint.Scope.Planned {
		if id == childID {
			t.Errorf("Child %q should NOT be in Sprint.Scope.Planned", childID)
		}
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
		// Must NOT be in sprint scope
		for _, id := range readState.Sprint.Scope.Planned {
			if id == childID {
				t.Errorf("Child %q should NOT be in Sprint.Scope.Planned", childID)
			}
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
