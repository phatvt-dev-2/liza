package ops

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestRecoverAgent_Validation(t *testing.T) {
	_, err := RecoverAgent("/nonexistent", "", false, "reason")
	if err == nil {
		t.Fatal("Expected error for empty agent ID")
	}
	if !strings.Contains(err.Error(), "agent ID required") {
		t.Errorf("Error = %q, want to contain 'agent ID required'", err.Error())
	}
}

func TestRecoverAgent_NotFound_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := RecoverAgent(tmpDir, "nonexistent", false, "reason")
	if err != nil {
		t.Fatalf("RecoverAgent() error: %v", err)
	}
	if !result.AlreadyClean {
		t.Error("Expected AlreadyClean=true for nonexistent agent")
	}
	if result.AgentDeleted {
		t.Error("AgentDeleted should be false for nonexistent agent")
	}
}

func TestRecoverAgent_CoderWithImplementingTask(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	taskRef := "task-1"
	now := time.Now().UTC()
	leaseExpires := now.Add(-10 * time.Minute) // expired
	worktreeRef := ".worktrees/task-1"
	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:         "coder",
		Status:       models.AgentStatusWorking,
		CurrentTask:  &taskRef,
		LeaseExpires: &leaseExpires,
		PID:          999999, // dead PID
	}
	state.Tasks = append(state.Tasks, models.Task{
		ID:           "task-1",
		Description:  "test task",
		Status:       models.TaskStatusImplementing,
		RolePair:     "coding-pair",
		Priority:     1,
		AssignedTo:   strPtr("coder-1"),
		Worktree:     &worktreeRef,
		LeaseExpires: &leaseExpires,
		SpecRef:      "spec.md",
		DoneWhen:     "tests pass",
		Scope:        "small",
	})
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := RecoverAgent(tmpDir, "coder-1", false, "crashed")
	if err != nil {
		t.Fatalf("RecoverAgent() error: %v", err)
	}

	if result.AlreadyClean {
		t.Error("Expected AlreadyClean=false")
	}
	if result.Role != "coder" {
		t.Errorf("Role = %q, want %q", result.Role, "coder")
	}
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
	if !result.ClaimReleased {
		t.Error("Expected ClaimReleased=true")
	}
	if !result.AgentDeleted {
		t.Error("Expected AgentDeleted=true")
	}

	// Verify state
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	// Agent should be gone
	if _, exists := readState.Agents["coder-1"]; exists {
		t.Error("Agent should be removed from state")
	}

	// Task should be READY with no assignment
	task := readState.FindTask("task-1")
	if task == nil {
		t.Fatal("Task should still exist")
	}
	if task.Status != models.TaskStatusReady {
		t.Errorf("Task status = %s, want %s", task.Status, models.TaskStatusReady)
	}
	if task.AssignedTo != nil {
		t.Error("Task AssignedTo should be nil")
	}
	if task.LeaseExpires != nil {
		t.Error("Task LeaseExpires should be nil")
	}
	if task.Worktree != nil {
		t.Error("Task Worktree should be nil")
	}

	// Human note should exist
	if len(readState.HumanNotes) == 0 {
		t.Fatal("Expected human note to be added")
	}
	lastNote := readState.HumanNotes[len(readState.HumanNotes)-1]
	if !strings.Contains(lastNote.Message, "coder-1") {
		t.Errorf("Note message = %q, want to contain agent ID", lastNote.Message)
	}
}

func TestRecoverAgent_ReviewerWithReviewingTask(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	taskRef := "task-2"
	now := time.Now().UTC()
	leaseExpires := now.Add(-10 * time.Minute)
	state := testhelpers.CreateValidState()
	state.Agents["code-reviewer-1"] = models.Agent{
		Role:         "code-reviewer",
		Status:       models.AgentStatusReviewing,
		CurrentTask:  &taskRef,
		LeaseExpires: &leaseExpires,
	}
	state.Tasks = append(state.Tasks, models.Task{
		ID:                 "task-2",
		Description:        "test task",
		Status:             models.TaskStatusReviewing,
		RolePair:           "coding-pair",
		Priority:           1,
		ReviewingBy:        strPtr("code-reviewer-1"),
		ReviewLeaseExpires: &leaseExpires,
		SpecRef:            "spec.md",
		DoneWhen:           "tests pass",
		Scope:              "small",
	})
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := RecoverAgent(tmpDir, "code-reviewer-1", false, "crashed")
	if err != nil {
		t.Fatalf("RecoverAgent() error: %v", err)
	}

	if result.Role != "code-reviewer" {
		t.Errorf("Role = %q, want %q", result.Role, "code-reviewer")
	}
	if !result.ClaimReleased {
		t.Error("Expected ClaimReleased=true")
	}
	if result.WorktreeRemoved {
		t.Error("Expected WorktreeRemoved=false for reviewer")
	}
	if !result.AgentDeleted {
		t.Error("Expected AgentDeleted=true")
	}

	// Verify state
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	// Agent should be gone
	if _, exists := readState.Agents["code-reviewer-1"]; exists {
		t.Error("Agent should be removed from state")
	}

	// Task should be READY_FOR_REVIEW
	task := readState.FindTask("task-2")
	if task == nil {
		t.Fatal("Task should still exist")
	}
	if task.Status != models.TaskStatusReadyForReview {
		t.Errorf("Task status = %s, want %s", task.Status, models.TaskStatusReadyForReview)
	}
	if task.ReviewingBy != nil {
		t.Error("Task ReviewingBy should be nil")
	}
	if task.ReviewLeaseExpires != nil {
		t.Error("Task ReviewLeaseExpires should be nil")
	}
}

func TestRecoverAgent_NoCurrentTask(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:   "coder",
		Status: models.AgentStatusIdle,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := RecoverAgent(tmpDir, "coder-1", false, "cleanup")
	if err != nil {
		t.Fatalf("RecoverAgent() error: %v", err)
	}

	if !result.AgentDeleted {
		t.Error("Expected AgentDeleted=true")
	}
	if result.ClaimReleased {
		t.Error("Expected ClaimReleased=false (no task)")
	}
	if result.WorktreeRemoved {
		t.Error("Expected WorktreeRemoved=false (no task)")
	}
	if result.TaskID != "" {
		t.Errorf("TaskID = %q, want empty", result.TaskID)
	}

	// Verify agent removed
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if _, exists := readState.Agents["coder-1"]; exists {
		t.Error("Agent should be removed from state")
	}
}

func TestRecoverAgent_PIDAlive_NoForce(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:   "coder",
		Status: models.AgentStatusWorking,
		PID:    os.Getpid(), // alive PID
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := RecoverAgent(tmpDir, "coder-1", false, "reason")
	if err == nil {
		t.Fatal("Expected error for alive PID without force")
	}
	if !strings.Contains(err.Error(), "still running") {
		t.Errorf("Error = %q, want to contain 'still running'", err.Error())
	}
}

func TestRecoverAgent_PIDAlive_WithForce(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:   "coder",
		Status: models.AgentStatusIdle,
		PID:    os.Getpid(), // alive PID
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := RecoverAgent(tmpDir, "coder-1", true, "forced recovery")
	if err != nil {
		t.Fatalf("RecoverAgent() with force error: %v", err)
	}
	if !result.AgentDeleted {
		t.Error("Expected AgentDeleted=true with force")
	}
}

func TestRecoverAgent_DoubleRecover(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:   "coder",
		Status: models.AgentStatusIdle,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	// First recover
	result1, err := RecoverAgent(tmpDir, "coder-1", false, "first recovery")
	if err != nil {
		t.Fatalf("First RecoverAgent() error: %v", err)
	}
	if result1.AlreadyClean {
		t.Error("First recovery should not be AlreadyClean")
	}
	if !result1.AgentDeleted {
		t.Error("First recovery should delete agent")
	}

	// Second recover — idempotent
	result2, err := RecoverAgent(tmpDir, "coder-1", false, "second recovery")
	if err != nil {
		t.Fatalf("Second RecoverAgent() error: %v", err)
	}
	if !result2.AlreadyClean {
		t.Error("Second recovery should be AlreadyClean")
	}
	if result2.AgentDeleted {
		t.Error("Second recovery should not report AgentDeleted")
	}
}

func strPtr(s string) *string {
	return &s
}
