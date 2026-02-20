package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestSubmitVerdict_Validation(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		verdict     string
		reason      string
		agentID     string
		errContains string
	}{
		{
			name: "empty task ID", verdict: "APPROVED", agentID: "r1",
			errContains: "task ID is required",
		},
		{
			name: "empty verdict", taskID: "t1", agentID: "r1",
			errContains: "verdict is required",
		},
		{
			name: "empty agent ID", taskID: "t1", verdict: "APPROVED",
			errContains: "LIZA_AGENT_ID is required",
		},
		{
			name: "invalid verdict", taskID: "t1", verdict: "MAYBE", agentID: "r1",
			errContains: "must be APPROVED or REJECTED",
		},
		{
			name: "rejection without reason", taskID: "t1", verdict: "REJECTED", agentID: "r1",
			errContains: "rejection reason is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SubmitVerdict("/nonexistent", tt.taskID, tt.verdict, tt.reason, tt.agentID)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Error = %q, want to contain %q", err.Error(), tt.errContains)
			}
		})
	}
}

func TestSubmitVerdict_VerdictNormalization(t *testing.T) {
	// Lowercase "approved" should be accepted and normalized
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now),
	}
	state.Agents["reviewer-1"] = models.Agent{
		Role:   "reviewer",
		Status: models.AgentStatusWorking,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SubmitVerdict(tmpDir, "task-1", "approved", "", "reviewer-1")
	if err != nil {
		t.Fatalf("SubmitVerdict() error: %v", err)
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "APPROVED")
	}
}

func TestSubmitVerdict_Approved(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now),
	}
	state.Agents["reviewer-1"] = models.Agent{
		Role:   "reviewer",
		Status: models.AgentStatusWorking,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SubmitVerdict(tmpDir, "task-1", "APPROVED", "", "reviewer-1")
	if err != nil {
		t.Fatalf("SubmitVerdict() error: %v", err)
	}

	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "APPROVED")
	}

	// Verify state
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := readState.FindTask("task-1")
	if task == nil {
		t.Fatal("Task not found")
	}
	if task.Status != models.TaskStatusApproved {
		t.Errorf("Status = %v, want APPROVED", task.Status)
	}
	if task.ApprovedBy == nil || *task.ApprovedBy != "reviewer-1" {
		t.Error("ApprovedBy should be reviewer-1")
	}
	if task.RejectionReason != nil {
		t.Error("RejectionReason should be nil after approval")
	}
	if task.ReviewingBy != nil {
		t.Error("ReviewingBy should be cleared")
	}
	if task.ReviewLeaseExpires != nil {
		t.Error("ReviewLeaseExpires should be cleared")
	}

	lastHistory := task.History[len(task.History)-1]
	if lastHistory.Event != "approved" {
		t.Errorf("History event = %q, want %q", lastHistory.Event, "approved")
	}
}

func TestSubmitVerdict_Rejected(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now),
	}
	state.Agents["reviewer-1"] = models.Agent{
		Role:   "reviewer",
		Status: models.AgentStatusWorking,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SubmitVerdict(tmpDir, "task-1", "REJECTED", "Missing error handling", "reviewer-1")
	if err != nil {
		t.Fatalf("SubmitVerdict() error: %v", err)
	}

	if result.Verdict != "REJECTED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "REJECTED")
	}
	if result.Reason != "Missing error handling" {
		t.Errorf("Reason = %q, want %q", result.Reason, "Missing error handling")
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := readState.FindTask("task-1")
	if task == nil {
		t.Fatal("Task not found")
	}
	if task.Status != models.TaskStatusRejected {
		t.Errorf("Status = %v, want REJECTED", task.Status)
	}
	if task.RejectionReason == nil || *task.RejectionReason != "Missing error handling" {
		t.Error("RejectionReason not set correctly")
	}
	if task.ReviewCyclesCurrent != 1 {
		t.Errorf("ReviewCyclesCurrent = %d, want 1", task.ReviewCyclesCurrent)
	}
	if task.ReviewCyclesTotal != 1 {
		t.Errorf("ReviewCyclesTotal = %d, want 1", task.ReviewCyclesTotal)
	}

	lastHistory := task.History[len(task.History)-1]
	if lastHistory.Event != "rejected" {
		t.Errorf("History event = %q, want %q", lastHistory.Event, "rejected")
	}
}

func TestSubmitVerdict_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitVerdict(tmpDir, "nonexistent", "APPROVED", "", "reviewer-1")
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !strings.Contains(err.Error(), "task not found") {
		t.Errorf("Error = %q, want to contain 'task not found'", err.Error())
	}
}

func TestSubmitVerdict_WrongStatus(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitVerdict(tmpDir, "task-1", "APPROVED", "", "reviewer-1")
	if err == nil {
		t.Fatal("Expected error for non-REVIEWING task")
	}
	if !strings.Contains(err.Error(), "not REVIEWING") {
		t.Errorf("Error = %q, want to contain 'not REVIEWING'", err.Error())
	}
}

func TestSubmitVerdict_AgentReleased(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now),
	}
	taskRef := "task-1"
	state.Agents["reviewer-1"] = models.Agent{
		Role:        "reviewer",
		Status:      models.AgentStatusWorking,
		CurrentTask: &taskRef,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitVerdict(tmpDir, "task-1", "APPROVED", "", "reviewer-1")
	if err != nil {
		t.Fatalf("SubmitVerdict() error: %v", err)
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	agent := readState.Agents["reviewer-1"]
	if agent.Status != models.AgentStatusIdle {
		t.Errorf("Agent status = %v, want idle", agent.Status)
	}
	if agent.CurrentTask != nil {
		t.Error("Agent CurrentTask should be nil after verdict")
	}
}
