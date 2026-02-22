package ops

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestCheckpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusInProgress
	state.Sprint.Timeline.Started = now.Add(-2 * time.Hour)
	state.Sprint.Timeline.Deadline = now.Add(6 * time.Hour)
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
		testhelpers.BuildTaskByStatus("task-2", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Checkpoint(tmpDir)
	if err != nil {
		t.Fatalf("Checkpoint() error: %v", err)
	}

	if result.CheckpointAt.IsZero() {
		t.Error("CheckpointAt should not be zero")
	}
	if result.ReportPath == "" {
		t.Error("ReportPath should not be empty")
	}

	// Verify report file was written
	content, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatalf("Failed to read report: %v", err)
	}
	if !strings.Contains(string(content), "Sprint Summary") {
		t.Error("Report should contain 'Sprint Summary'")
	}
	if !strings.Contains(string(content), "MERGED") {
		t.Error("Report should contain task status table")
	}

	// Verify state updated
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if readState.Sprint.Status != models.SprintStatusCheckpoint {
		t.Errorf("Sprint status = %v, want CHECKPOINT", readState.Sprint.Status)
	}
	if readState.Sprint.Timeline.CheckpointAt == nil {
		t.Error("CheckpointAt should be set in state")
	}
}

func TestCheckpoint_AlreadyAtCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCheckpoint
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Checkpoint(tmpDir)
	if err == nil {
		t.Fatal("Expected error when already at CHECKPOINT")
	}
	if !errors.Is(err, ErrSprintAlreadyCheckpoint) {
		t.Fatalf("error = %v, want errors.Is(..., ErrSprintAlreadyCheckpoint)", err)
	}
}

func TestCheckpoint_CompletedSprint(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusCompleted
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Checkpoint(tmpDir)
	if err == nil {
		t.Fatal("Expected error for COMPLETED sprint")
	}
	if !strings.Contains(err.Error(), "COMPLETED") {
		t.Errorf("Error = %q, want to contain 'COMPLETED'", err.Error())
	}
}

func TestCheckpoint_AbortedSprint(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusAborted
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Checkpoint(tmpDir)
	if err == nil {
		t.Fatal("Expected error for ABORTED sprint")
	}
	if !strings.Contains(err.Error(), "ABORTED") {
		t.Errorf("Error = %q, want to contain 'ABORTED'", err.Error())
	}
}

func TestFormatCheckpointDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{name: "minutes only", duration: 45 * time.Minute, expected: "45m"},
		{name: "hours and minutes", duration: 3*time.Hour + 15*time.Minute, expected: "3h 15m"},
		{name: "days hours minutes", duration: 50 * time.Hour, expected: "2d 2h 0m"},
		{name: "zero", duration: 0, expected: "0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCheckpointDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatCheckpointDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestGenerateSprintSummary_WithAnomalies(t *testing.T) {
	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Timeline.Started = now.Add(-1 * time.Hour)
	state.Sprint.Timeline.Deadline = now.Add(5 * time.Hour)
	state.Anomalies = []models.Anomaly{
		{Type: "retry_loop", Task: "task-1", Reporter: "coder-1", Details: map[string]any{"error_pattern": "test anomaly"}},
		{Type: "retry_loop", Task: "task-1", Reporter: "coder-1", Details: map[string]any{"error_pattern": "another"}},
	}

	report := generateSprintSummary(state, now)

	if !strings.Contains(report, "Anomalies") {
		t.Error("Report should contain Anomalies section")
	}
	if !strings.Contains(report, "retry_loop") {
		t.Error("Report should contain anomaly type")
	}
}

func TestGenerateSprintSummary_Overdue(t *testing.T) {
	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Timeline.Started = now.Add(-10 * time.Hour)
	state.Sprint.Timeline.Deadline = now.Add(-2 * time.Hour) // Overdue

	report := generateSprintSummary(state, now)

	if !strings.Contains(report, "Overdue") {
		t.Error("Report should contain 'Overdue' for past deadlines")
	}
}

func TestGenerateSprintSummary_WithAgents(t *testing.T) {
	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Timeline.Started = now.Add(-1 * time.Hour)
	state.Sprint.Timeline.Deadline = now.Add(5 * time.Hour)
	taskRef := "task-1"
	state.Agents["coder-1"] = models.Agent{
		Role:        "coder",
		Status:      models.AgentStatusWorking,
		CurrentTask: &taskRef,
	}

	report := generateSprintSummary(state, now)

	if !strings.Contains(report, "coder-1") {
		t.Error("Report should contain agent ID")
	}
	if !strings.Contains(report, "task-1") {
		t.Error("Report should contain agent's current task")
	}
}

func TestGenerateSprintSummary_CircuitBreakerTriggered(t *testing.T) {
	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Sprint.Timeline.Started = now.Add(-1 * time.Hour)
	state.Sprint.Timeline.Deadline = now.Add(5 * time.Hour)
	state.CircuitBreaker.Status = "TRIGGERED"
	state.CircuitBreaker.CurrentTrigger = &models.CircuitBreakerTrigger{
		Pattern:  "retry_cluster",
		Severity: "HIGH",
	}

	report := generateSprintSummary(state, now)

	if !strings.Contains(report, "TRIGGERED") {
		t.Error("Report should contain circuit breaker status")
	}
	if !strings.Contains(report, "retry_cluster") {
		t.Error("Report should contain trigger pattern")
	}
}
