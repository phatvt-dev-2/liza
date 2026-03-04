package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// --- Start ---

func TestStart_FromStopped(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModeStopped
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Start(tmpDir, "resuming work", "human")
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	if result.Previous != models.SystemModeStopped {
		t.Errorf("Previous = %v, want STOPPED", result.Previous)
	}
	if result.New != models.SystemModeRunning {
		t.Errorf("New = %v, want RUNNING", result.New)
	}
	if result.ChangedBy != "human" {
		t.Errorf("ChangedBy = %q, want %q", result.ChangedBy, "human")
	}

	// Verify persisted state
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if readState.Config.Mode != models.SystemModeRunning {
		t.Errorf("Persisted mode = %v, want RUNNING", readState.Config.Mode)
	}
	if readState.Config.ModeChangedBy == nil || *readState.Config.ModeChangedBy != "human" {
		t.Error("ModeChangedBy not set")
	}
}

func TestStart_AlreadyRunning(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModeRunning
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Start(tmpDir, "reason", "human")
	if err == nil {
		t.Fatal("Expected error when already RUNNING")
	}
	if !strings.Contains(err.Error(), "already RUNNING") {
		t.Errorf("Error = %q, want to contain 'already RUNNING'", err.Error())
	}
}

func TestStart_FromPaused(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModePaused
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Start(tmpDir, "reason", "human")
	if err == nil {
		t.Fatal("Expected error when PAUSED")
	}
	if !strings.Contains(err.Error(), "PAUSED") {
		t.Errorf("Error = %q, want to contain 'PAUSED'", err.Error())
	}
}

// --- Stop ---

func TestStop_FromRunning(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModeRunning
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Stop(tmpDir, "end of day", "human")
	if err != nil {
		t.Fatalf("Stop() error: %v", err)
	}

	if result.Previous != models.SystemModeRunning {
		t.Errorf("Previous = %v, want RUNNING", result.Previous)
	}
	if result.New != models.SystemModeStopped {
		t.Errorf("New = %v, want STOPPED", result.New)
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if readState.Config.Mode != models.SystemModeStopped {
		t.Errorf("Persisted mode = %v, want STOPPED", readState.Config.Mode)
	}
}

func TestStop_AlreadyStopped(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModeStopped
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Stop(tmpDir, "reason", "human")
	if err == nil {
		t.Fatal("Expected error when already STOPPED")
	}
	if !strings.Contains(err.Error(), "already STOPPED") {
		t.Errorf("Error = %q, want to contain 'already STOPPED'", err.Error())
	}
}

// --- Pause ---

func TestPause_FromRunning(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModeRunning
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Pause(tmpDir, "lunch break", "human")
	if err != nil {
		t.Fatalf("Pause() error: %v", err)
	}

	if result.Previous != models.SystemModeRunning {
		t.Errorf("Previous = %v, want RUNNING", result.Previous)
	}
	if result.New != models.SystemModePaused {
		t.Errorf("New = %v, want PAUSED", result.New)
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if readState.Config.Mode != models.SystemModePaused {
		t.Errorf("Persisted mode = %v, want PAUSED", readState.Config.Mode)
	}
}

func TestPause_AlreadyPaused(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModePaused
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Pause(tmpDir, "reason", "human")
	if err == nil {
		t.Fatal("Expected error when already PAUSED")
	}
	if !strings.Contains(err.Error(), "already PAUSED") {
		t.Errorf("Error = %q, want to contain 'already PAUSED'", err.Error())
	}
}

func TestPause_FromStopped(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModeStopped
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Pause(tmpDir, "reason", "human")
	if err == nil {
		t.Fatal("Expected error when STOPPED")
	}
	if !strings.Contains(err.Error(), "STOPPED") {
		t.Errorf("Error = %q, want to contain 'STOPPED'", err.Error())
	}
}

// --- Resume ---

func TestResume_FromPaused(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModePaused
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Resume(tmpDir, "human")
	if err != nil {
		t.Fatalf("Resume() error: %v", err)
	}

	if result.ResumedFrom != "PAUSED mode" {
		t.Errorf("ResumedFrom = %q, want %q", result.ResumedFrom, "PAUSED mode")
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if readState.Config.Mode != models.SystemModeRunning {
		t.Errorf("Persisted mode = %v, want RUNNING", readState.Config.Mode)
	}
}

func TestResume_FromCircuitBreaker(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModeCircuitBreakerTripped
	state.CircuitBreaker.Status = "TRIGGERED"
	trigger := &models.CircuitBreakerTrigger{Pattern: "retry_cluster", Severity: "HIGH"}
	state.CircuitBreaker.CurrentTrigger = trigger
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Resume(tmpDir, "human")
	if err != nil {
		t.Fatalf("Resume() error: %v", err)
	}

	if result.ResumedFrom != "CIRCUIT_BREAKER_TRIPPED mode" {
		t.Errorf("ResumedFrom = %q, want %q", result.ResumedFrom, "CIRCUIT_BREAKER_TRIPPED mode")
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if readState.Config.Mode != models.SystemModeRunning {
		t.Errorf("Persisted mode = %v, want RUNNING", readState.Config.Mode)
	}
	if readState.CircuitBreaker.Status != "OK" {
		t.Errorf("CircuitBreaker status = %q, want %q", readState.CircuitBreaker.Status, "OK")
	}
	if readState.CircuitBreaker.CurrentTrigger != nil {
		t.Error("CircuitBreaker trigger should be cleared")
	}
}

func TestResume_FromCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModeRunning
	state.Sprint.Status = models.SprintStatusCheckpoint
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Resume(tmpDir, "human")
	if err != nil {
		t.Fatalf("Resume() error: %v", err)
	}

	if result.ResumedFrom != "CHECKPOINT" {
		t.Errorf("ResumedFrom = %q, want %q", result.ResumedFrom, "CHECKPOINT")
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if readState.Sprint.Status != models.SprintStatusInProgress {
		t.Errorf("Sprint status = %v, want IN_PROGRESS", readState.Sprint.Status)
	}
}

func TestResume_PausedAndCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModePaused
	state.Sprint.Status = models.SprintStatusCheckpoint
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Resume(tmpDir, "human")
	if err != nil {
		t.Fatalf("Resume() error: %v", err)
	}

	if !strings.Contains(result.ResumedFrom, "PAUSED") || !strings.Contains(result.ResumedFrom, "CHECKPOINT") {
		t.Errorf("ResumedFrom = %q, want to contain both PAUSED and CHECKPOINT", result.ResumedFrom)
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if readState.Config.Mode != models.SystemModeRunning {
		t.Errorf("Persisted mode = %v, want RUNNING", readState.Config.Mode)
	}
	if readState.Sprint.Status != models.SprintStatusInProgress {
		t.Errorf("Sprint status = %v, want IN_PROGRESS", readState.Sprint.Status)
	}
}

func TestResume_CheckpointAllTerminal_MarksCompleted(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModeRunning
	state.Sprint.Status = models.SprintStatusCheckpoint

	// Add a sprint-terminal task
	now := time.Now().UTC()
	reviewCommit := "abc123"
	task := models.Task{
		ID:           "plan-1",
		Type:         models.TaskTypeCoding,
		Description:  "Plan task",
		Status:       models.TaskStatusCodingPlanApproved,
		Priority:     1,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "Approved",
		Scope:        "scope",
		ReviewCommit: &reviewCommit,
		History:      []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, task)
	state.Sprint.Scope.Planned = []string{"plan-1"}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Resume(tmpDir, "human")
	if err != nil {
		t.Fatalf("Resume() error: %v", err)
	}

	if result.ResumedFrom != "CHECKPOINT" {
		t.Errorf("ResumedFrom = %q, want %q", result.ResumedFrom, "CHECKPOINT")
	}
	// Should NOT advance (no SprintAdvanced)
	if result.SprintAdvanced != nil {
		t.Error("Expected no sprint advance when transitioning to COMPLETED")
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if readState.Sprint.Status != models.SprintStatusCompleted {
		t.Errorf("Sprint status = %v, want COMPLETED", readState.Sprint.Status)
	}
}

func TestResume_FromCompleted_AdvancesSprint(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModeRunning
	state.Sprint.Status = models.SprintStatusCompleted

	// Parent task at sprint-terminal, plus child at DRAFT
	now := time.Now().UTC()
	parentID := "plan-1"
	reviewCommit := "abc123"
	parentTask := models.Task{
		ID:                  parentID,
		Type:                models.TaskTypeCoding,
		Description:         "Plan task",
		Status:              models.TaskStatusCodingPlanApproved,
		Priority:            1,
		Created:             now,
		SpecRef:             "README.md",
		DoneWhen:            "Approved",
		Scope:               "scope",
		ReviewCommit:        &reviewCommit,
		TransitionsExecuted: map[string]bool{"code-plan-to-coding": true},
		History:             []models.TaskHistoryEntry{},
	}
	childTask := models.Task{
		ID:          "plan-1-code-plan-to-coding-0",
		Type:        models.TaskTypeCoding,
		Description: "Child task",
		Status:      models.TaskStatusDraft,
		Priority:    1,
		Created:     now,
		ParentTask:  &parentID,
		History:     []models.TaskHistoryEntry{},
	}
	state.Tasks = append(state.Tasks, parentTask, childTask)
	state.Sprint.Scope.Planned = []string{parentID}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Resume(tmpDir, "human")
	if err != nil {
		t.Fatalf("Resume() error: %v", err)
	}

	if result.ResumedFrom != "COMPLETED sprint" {
		t.Errorf("ResumedFrom = %q, want %q", result.ResumedFrom, "COMPLETED sprint")
	}
	if result.SprintAdvanced == nil {
		t.Fatal("Expected sprint advance")
	}
	if result.SprintAdvanced.NewSprintNumber != 2 {
		t.Errorf("NewSprintNumber = %d, want 2", result.SprintAdvanced.NewSprintNumber)
	}

	// Child task should be in new sprint's planned scope
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if readState.Sprint.Status != models.SprintStatusInProgress {
		t.Errorf("Sprint status = %v, want IN_PROGRESS", readState.Sprint.Status)
	}
	if readState.Sprint.Number != 2 {
		t.Errorf("Sprint number = %d, want 2", readState.Sprint.Number)
	}
	// The child task (DRAFT, non-terminal) should be carried forward
	found := false
	for _, id := range readState.Sprint.Scope.Planned {
		if id == "plan-1-code-plan-to-coding-0" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Child task not in new sprint planned scope: %v", readState.Sprint.Scope.Planned)
	}
}

func TestResume_FromStopped(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModeStopped
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Resume(tmpDir, "human")
	if err == nil {
		t.Fatal("Expected error when STOPPED")
	}
	if !strings.Contains(err.Error(), "STOPPED") {
		t.Errorf("Error = %q, want to contain 'STOPPED'", err.Error())
	}
}

func TestResume_NothingToResume(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModeRunning
	state.Sprint.Status = models.SprintStatusInProgress
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Resume(tmpDir, "human")
	if err == nil {
		t.Fatal("Expected error when nothing to resume")
	}
}
