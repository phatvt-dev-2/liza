package ops

import (
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestReleaseClaim_Validation(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		role        string
		errContains string
	}{
		{
			name: "empty task ID", role: "coder",
			errContains: "task ID is required",
		},
		{
			name: "invalid role", taskID: "t1", role: "invalid",
			errContains: "role must be reviewer, coder, or both",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReleaseClaim("/nonexistent", tt.taskID, tt.role, false, "", "human")
			testhelpers.RequireErrorContains(t, err, tt.errContains)
		})
	}
}

func TestReleaseClaim_CoderClaim(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
	state.Tasks = []models.Task{task}
	// Register the assigned agent
	state.Agents["coder-1"] = models.Agent{
		Role:        "coder",
		Status:      models.AgentStatusWorking,
		CurrentTask: testhelpers.StringPtr("task-1"),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ReleaseClaim(tmpDir, "task-1", "coder", true, "manual cleanup", "human")
	if err != nil {
		t.Fatalf("ReleaseClaim() error: %v", err)
	}

	if !result.ReleasedCoder {
		t.Error("ReleasedCoder should be true")
	}
	if result.ReleasedReviewer {
		t.Error("ReleasedReviewer should be false")
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	readTask := readState.FindTask("task-1")
	if readTask == nil {
		t.Fatal("Task not found")
	}
	if readTask.Status != models.TaskStatusReady {
		t.Errorf("Status = %v, want READY", readTask.Status)
	}
	if readTask.AssignedTo != nil {
		t.Error("AssignedTo should be nil")
	}
	if readTask.LeaseExpires != nil {
		t.Error("LeaseExpires should be nil")
	}

	// Verify agent released
	agent := readState.Agents["coder-1"]
	if agent.Status != models.AgentStatusIdle {
		t.Errorf("Agent status = %v, want idle", agent.Status)
	}

	// Verify history entry
	lastHistory := readTask.History[len(readTask.History)-1]
	if lastHistory.Event != "coder_claim_released" {
		t.Errorf("History event = %q, want %q", lastHistory.Event, "coder_claim_released")
	}
}

func TestReleaseClaim_ReviewerClaim(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	state.Tasks = []models.Task{task}
	state.Agents["reviewer-1"] = models.Agent{
		Role:        "reviewer",
		Status:      models.AgentStatusWorking,
		CurrentTask: testhelpers.StringPtr("task-1"),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ReleaseClaim(tmpDir, "task-1", "reviewer", true, "timeout", "human")
	if err != nil {
		t.Fatalf("ReleaseClaim() error: %v", err)
	}

	if !result.ReleasedReviewer {
		t.Error("ReleasedReviewer should be true")
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	readTask := readState.FindTask("task-1")
	if readTask == nil {
		t.Fatal("Task not found")
	}
	if readTask.Status != models.TaskStatusReadyForReview {
		t.Errorf("Status = %v, want READY_FOR_REVIEW", readTask.Status)
	}
	if readTask.ReviewingBy != nil {
		t.Error("ReviewingBy should be nil")
	}
	if readTask.ReviewLeaseExpires != nil {
		t.Error("ReviewLeaseExpires should be nil")
	}

	lastHistory := readTask.History[len(readTask.History)-1]
	if lastHistory.Event != "review_claim_released" {
		t.Errorf("History event = %q, want %q", lastHistory.Event, "review_claim_released")
	}
}

func TestReleaseClaim_BothClaims(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	state.Tasks = []models.Task{task}
	state.Agents["coder-1"] = models.Agent{
		Role:        "coder",
		Status:      models.AgentStatusWorking,
		CurrentTask: testhelpers.StringPtr("task-1"),
	}
	state.Agents["reviewer-1"] = models.Agent{
		Role:        "reviewer",
		Status:      models.AgentStatusWorking,
		CurrentTask: testhelpers.StringPtr("task-1"),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ReleaseClaim(tmpDir, "task-1", "both", true, "full reset", "human")
	if err != nil {
		t.Fatalf("ReleaseClaim() error: %v", err)
	}

	if !result.ReleasedCoder {
		t.Error("ReleasedCoder should be true")
	}
	if !result.ReleasedReviewer {
		t.Error("ReleasedReviewer should be true")
	}
}

func TestReleaseClaim_NoClaims(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ReleaseClaim(tmpDir, "task-1", "coder", true, "reason", "human")
	testhelpers.RequireErrorContains(t, err, "no claims to release")
}

func TestReleaseClaim_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ReleaseClaim(tmpDir, "nonexistent", "coder", false, "", "human")
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestReleaseClaim_ActiveLease_NoForce(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
	// Ensure lease is in the future
	futureLease := now.Add(30 * time.Minute)
	task.LeaseExpires = &futureLease
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ReleaseClaim(tmpDir, "task-1", "coder", false, "", "human")
	testhelpers.RequireErrorContains(t, err, "lease still valid")
}

func TestReleaseClaim_DefaultAgentAndReason(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
	state.Tasks = []models.Task{task}
	state.Agents["coder-1"] = models.Agent{
		Role:        "coder",
		Status:      models.AgentStatusWorking,
		CurrentTask: testhelpers.StringPtr("task-1"),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	// Empty agentID and reason should get defaults
	_, err := ReleaseClaim(tmpDir, "task-1", "coder", true, "", "")
	if err != nil {
		t.Fatalf("ReleaseClaim() error: %v", err)
	}
}
