package ops

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/statevalidate"
	"github.com/liza-mas/liza/internal/testhelpers"
	"gopkg.in/yaml.v3"
)

func setupAdvanceTest(t *testing.T) (tmpDir, stateFile string) {
	t.Helper()
	tmpDir = t.TempDir()
	stateFile, _ = testhelpers.SetupLizaDir(t, tmpDir)
	archiveDir := filepath.Join(tmpDir, ".liza", "archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		t.Fatalf("Failed to create archive dir: %v", err)
	}
	return tmpDir, stateFile
}

func TestAdvanceSprint(t *testing.T) {
	tmpDir, stateFile := setupAdvanceTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint
	state.Sprint.Number = 1
	state.Sprint.Metrics.TasksDone = 2

	// Merged task (terminal) in planned scope
	mergedTask := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
	// Non-terminal task not in planned scope
	readyTask := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReady, now)
	state.Tasks = []models.Task{mergedTask, readyTask}
	state.Sprint.Scope.Planned = []string{"task-1"}

	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := AdvanceSprint(tmpDir)
	if err != nil {
		t.Fatalf("AdvanceSprint() error: %v", err)
	}

	if result.ArchivedSprintID != "sprint-1" {
		t.Errorf("ArchivedSprintID = %q, want %q", result.ArchivedSprintID, "sprint-1")
	}
	if result.NewSprintID != "sprint-2" {
		t.Errorf("NewSprintID = %q, want %q", result.NewSprintID, "sprint-2")
	}
	if result.NewSprintNumber != 2 {
		t.Errorf("NewSprintNumber = %d, want 2", result.NewSprintNumber)
	}
	if len(result.CarriedTasks) != 1 || result.CarriedTasks[0] != "task-2" {
		t.Errorf("CarriedTasks = %v, want [task-2]", result.CarriedTasks)
	}

	// Verify persisted state
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	if readState.Sprint.ID != "sprint-2" {
		t.Errorf("Sprint.ID = %q, want %q", readState.Sprint.ID, "sprint-2")
	}
	if readState.Sprint.Number != 2 {
		t.Errorf("Sprint.Number = %d, want 2", readState.Sprint.Number)
	}
	if readState.Sprint.Status != models.SprintStatusInProgress {
		t.Errorf("Sprint.Status = %v, want IN_PROGRESS", readState.Sprint.Status)
	}
	if len(readState.Sprint.Scope.Planned) != 1 || readState.Sprint.Scope.Planned[0] != "task-2" {
		t.Errorf("Sprint.Scope.Planned = %v, want [task-2]", readState.Sprint.Scope.Planned)
	}

	// Verify sprint history
	if len(readState.SprintHistory) != 1 {
		t.Fatalf("SprintHistory length = %d, want 1", len(readState.SprintHistory))
	}
	summary := readState.SprintHistory[0]
	if summary.ID != "sprint-1" {
		t.Errorf("SprintHistory[0].ID = %q, want %q", summary.ID, "sprint-1")
	}
	if summary.Number != 1 {
		t.Errorf("SprintHistory[0].Number = %d, want 1", summary.Number)
	}
	if summary.Status != models.SprintStatusCompleted {
		t.Errorf("SprintHistory[0].Status = %v, want COMPLETED", summary.Status)
	}
	if summary.TasksDone != 2 {
		t.Errorf("SprintHistory[0].TasksDone = %d, want 2", summary.TasksDone)
	}
}

func TestAdvanceSprint_ArchiveFileCreated(t *testing.T) {
	tmpDir, stateFile := setupAdvanceTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint
	state.Sprint.Number = 1
	state.Sprint.Metrics.TasksDone = 1

	mergedTask := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
	state.Tasks = []models.Task{mergedTask}
	state.Sprint.Scope.Planned = []string{"task-1"}

	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := AdvanceSprint(tmpDir)
	if err != nil {
		t.Fatalf("AdvanceSprint() error: %v", err)
	}

	// Verify archive file exists
	archivePath := result.ArchivePath
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Fatalf("Archive file not created at %s", archivePath)
	}

	// Verify archive content
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("Failed to read archive file: %v", err)
	}
	var archived models.Sprint
	if err := yaml.Unmarshal(data, &archived); err != nil {
		t.Fatalf("Failed to parse archive YAML: %v", err)
	}
	if archived.ID != "sprint-1" {
		t.Errorf("Archived sprint ID = %q, want %q", archived.ID, "sprint-1")
	}
	if archived.Number != 1 {
		t.Errorf("Archived sprint Number = %d, want 1", archived.Number)
	}
	// Archive should store COMPLETED status, not CHECKPOINT
	if archived.Status != models.SprintStatusCompleted {
		t.Errorf("Archived sprint Status = %v, want COMPLETED", archived.Status)
	}
	if archived.Timeline.Ended == nil {
		t.Error("Archived sprint Timeline.Ended should be set")
	}
}

func TestAdvanceSprint_NotCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusInProgress
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := AdvanceSprint(tmpDir)
	if err == nil {
		t.Fatal("Expected error when sprint is not at CHECKPOINT")
	}
}

func TestAdvanceSprint_NotAllTerminal(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint
	state.Sprint.Number = 1

	readyTask := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	state.Tasks = []models.Task{readyTask}
	state.Sprint.Scope.Planned = []string{"task-1"}

	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := AdvanceSprint(tmpDir)
	if err == nil {
		t.Fatal("Expected error when not all planned tasks terminal")
	}
}

func TestAdvanceSprint_LegacyZeroNumber(t *testing.T) {
	tmpDir, stateFile := setupAdvanceTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint
	state.Sprint.Number = 0 // legacy state

	mergedTask := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
	state.Tasks = []models.Task{mergedTask}
	state.Sprint.Scope.Planned = []string{"task-1"}

	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := AdvanceSprint(tmpDir)
	if err != nil {
		t.Fatalf("AdvanceSprint() error: %v", err)
	}

	// Legacy Number=0 should produce sprint-2 (normalized to 1, then +1)
	if result.NewSprintNumber != 2 {
		t.Errorf("NewSprintNumber = %d, want 2 (legacy zero guard)", result.NewSprintNumber)
	}

	// Verify history entry has normalized Number=1, not 0
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if len(readState.SprintHistory) != 1 {
		t.Fatalf("SprintHistory length = %d, want 1", len(readState.SprintHistory))
	}
	if readState.SprintHistory[0].Number != 1 {
		t.Errorf("SprintHistory[0].Number = %d, want 1 (normalized from legacy 0)", readState.SprintHistory[0].Number)
	}
}

// TestAdvanceSprint_LegacyPassesValidation is a regression test: advancing a
// legacy sprint (Number=0) must produce state that passes full validation.
func TestAdvanceSprint_LegacyPassesValidation(t *testing.T) {
	tmpDir, stateFile := setupAdvanceTest(t)

	// Create spec files referenced by state and tasks
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specsDir, "vision.md"), []byte("# Vision\n"), 0644); err != nil {
		t.Fatalf("Failed to create spec file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# README\n"), 0644); err != nil {
		t.Fatalf("Failed to create README.md: %v", err)
	}

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint
	state.Sprint.Number = 0 // legacy

	mergedTask := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
	state.Tasks = []models.Task{mergedTask}
	state.Sprint.Scope.Planned = []string{"task-1"}

	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := AdvanceSprint(tmpDir)
	if err != nil {
		t.Fatalf("AdvanceSprint() error: %v", err)
	}

	// Full validation should pass on the resulting state
	if err := statevalidate.ValidateStateFile(stateFile, false, nil); err != nil {
		t.Fatalf("Validation failed after legacy advance: %v", err)
	}
}

func TestResumeWithSprintAdvance(t *testing.T) {
	tmpDir, stateFile := setupAdvanceTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModeRunning
	state.Sprint.Status = models.SprintStatusCheckpoint
	state.Sprint.Number = 1

	// All planned tasks are terminal
	mergedTask := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
	state.Tasks = []models.Task{mergedTask}
	state.Sprint.Scope.Planned = []string{"task-1"}

	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Resume(tmpDir, "human")
	if err != nil {
		t.Fatalf("Resume() error: %v", err)
	}

	if result.SprintAdvanced == nil {
		t.Fatal("Expected SprintAdvanced to be non-nil")
	}
	if result.SprintAdvanced.NewSprintID != "sprint-2" {
		t.Errorf("SprintAdvanced.NewSprintID = %q, want %q", result.SprintAdvanced.NewSprintID, "sprint-2")
	}

	// Verify persisted state — both mode and sprint changed atomically
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if readState.Sprint.ID != "sprint-2" {
		t.Errorf("Sprint.ID = %q, want %q", readState.Sprint.ID, "sprint-2")
	}
	if readState.Sprint.Number != 2 {
		t.Errorf("Sprint.Number = %d, want 2", readState.Sprint.Number)
	}
	if readState.Sprint.Status != models.SprintStatusInProgress {
		t.Errorf("Sprint.Status = %v, want IN_PROGRESS", readState.Sprint.Status)
	}
	if len(readState.SprintHistory) != 1 {
		t.Errorf("SprintHistory length = %d, want 1", len(readState.SprintHistory))
	}
}

func TestResumeWithoutSprintAdvance(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModeRunning
	state.Sprint.Status = models.SprintStatusCheckpoint
	state.Sprint.Number = 1

	// Not all planned tasks are terminal (mid-sprint checkpoint)
	readyTask := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	state.Tasks = []models.Task{readyTask}
	state.Sprint.Scope.Planned = []string{"task-1"}

	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Resume(tmpDir, "human")
	if err != nil {
		t.Fatalf("Resume() error: %v", err)
	}

	if result.SprintAdvanced != nil {
		t.Error("Expected SprintAdvanced to be nil for mid-sprint checkpoint")
	}

	// Verify sprint stays the same
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if readState.Sprint.ID != "sprint-1" {
		t.Errorf("Sprint.ID = %q, want %q (should stay same)", readState.Sprint.ID, "sprint-1")
	}
	if readState.Sprint.Status != models.SprintStatusInProgress {
		t.Errorf("Sprint.Status = %v, want IN_PROGRESS", readState.Sprint.Status)
	}
}
