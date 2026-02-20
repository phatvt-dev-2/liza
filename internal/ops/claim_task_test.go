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

func TestClaimTask_Validation(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		agentID     string
		errContains string
	}{
		{
			name:        "empty task ID",
			taskID:      "",
			agentID:     "coder-1",
			errContains: "task ID is required",
		},
		{
			name:        "empty agent ID",
			taskID:      "task-1",
			agentID:     "",
			errContains: "agent ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ClaimTask("/nonexistent", tt.taskID, tt.agentID)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Error = %q, want to contain %q", err.Error(), tt.errContains)
			}
		})
	}
}

func TestClaimTask_ReadyTask(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err != nil {
		t.Fatalf("ClaimTask() error: %v", err)
	}

	// Verify result fields
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
	if result.AgentID != "coder-1" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "coder-1")
	}
	if result.SourceStatus != models.TaskStatusReady {
		t.Errorf("SourceStatus = %v, want READY", result.SourceStatus)
	}
	if result.BaseCommit == "" {
		t.Error("BaseCommit should not be empty")
	}
	if result.IntegrationFix {
		t.Error("IntegrationFix should be false for READY task")
	}

	// Verify state updated
	readState := readClaimStateForTest(t, stateFile)
	task := readState.FindTask("task-1")
	if task == nil {
		t.Fatal("Task not found in state")
	}
	if task.Status != models.TaskStatusImplementing {
		t.Errorf("Task status = %v, want IMPLEMENTING", task.Status)
	}
	if task.AssignedTo == nil || *task.AssignedTo != "coder-1" {
		t.Error("AssignedTo should be coder-1")
	}
	if task.Iteration != 1 {
		t.Errorf("Iteration = %d, want 1", task.Iteration)
	}
	if task.Worktree == nil {
		t.Error("Worktree should be set")
	}

	// Verify worktree was created on disk
	wtDir := filepath.Join(tmpDir, ".worktrees", "task-1")
	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		t.Errorf("Worktree directory should exist at %s", wtDir)
	}

	// Verify agent registered
	agent, exists := readState.Agents["coder-1"]
	if !exists {
		t.Fatal("Agent not found in state")
	}
	if agent.CurrentTask == nil || *agent.CurrentTask != "task-1" {
		t.Error("Agent CurrentTask should be task-1")
	}
	if agent.Status != models.AgentStatusWorking {
		t.Errorf("Agent Status = %v, want working", agent.Status)
	}
}

func TestClaimTask_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ClaimTask(tmpDir, "nonexistent", "coder-1")
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !strings.Contains(err.Error(), "task not found") {
		t.Errorf("Error = %q, want to contain 'task not found'", err.Error())
	}
}

func TestClaimTask_WrongStatus(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ClaimTask(tmpDir, "task-1", "coder-2")
	if err == nil {
		t.Fatal("Expected error for IMPLEMENTING task")
	}
	if !strings.Contains(err.Error(), "not READY") {
		t.Errorf("Error = %q, want to contain 'not READY'", err.Error())
	}
}

func TestClaimTask_AgentBusy(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	// Agent is busy with another task
	otherTask := "task-other"
	state.Agents["coder-1"] = models.Agent{
		Status:      models.AgentStatusWorking,
		CurrentTask: &otherTask,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err == nil {
		t.Fatal("Expected error for busy agent")
	}
	if !strings.Contains(err.Error(), "already working") {
		t.Errorf("Error = %q, want to contain 'already working'", err.Error())
	}
}

func TestClaimTask_UnmetDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	depTask := testhelpers.BuildTaskByStatus("dep-1", models.TaskStatusReady, now)
	mainTask := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	mainTask.DependsOn = []string{"dep-1"}
	state.Tasks = []models.Task{depTask, mainTask}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err == nil {
		t.Fatal("Expected error for unmet dependencies")
	}
	if !strings.Contains(err.Error(), "unmet dependencies") {
		t.Errorf("Error = %q, want to contain 'unmet dependencies'", err.Error())
	}
}

func TestClaimTask_MetDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	depTask := testhelpers.BuildTaskByStatus("dep-1", models.TaskStatusMerged, now)
	mainTask := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	mainTask.DependsOn = []string{"dep-1"}
	state.Tasks = []models.Task{depTask, mainTask}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err != nil {
		t.Fatalf("ClaimTask() error: %v", err)
	}
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
}

func TestClaimTask_IntegrationFailed(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now)
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	// Create the worktree directory that IntegrationFailed expects to exist
	wtDir := filepath.Join(tmpDir, ".worktrees", "task-1")
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		t.Fatalf("Failed to create worktree dir: %v", err)
	}

	result, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err != nil {
		t.Fatalf("ClaimTask() error: %v", err)
	}

	if !result.IntegrationFix {
		t.Error("IntegrationFix should be true for INTEGRATION_FAILED task")
	}
	if result.SourceStatus != models.TaskStatusIntegrationFailed {
		t.Errorf("SourceStatus = %v, want INTEGRATION_FAILED", result.SourceStatus)
	}

	// Verify task state
	readState := readClaimStateForTest(t, stateFile)
	claimedTask := readState.FindTask("task-1")
	if claimedTask == nil {
		t.Fatal("Task not found")
	}
	if !claimedTask.IntegrationFix {
		t.Error("IntegrationFix flag should be set in state")
	}
}

func TestClaimTask_RejectedSameCoder(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
	// task.AssignedTo is already "coder-1" from BuildTaskByStatus
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	// Create the worktree that same-coder reclaim expects
	wtDir := filepath.Join(tmpDir, ".worktrees", "task-1")
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		t.Fatalf("Failed to create worktree dir: %v", err)
	}

	result, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err != nil {
		t.Fatalf("ClaimTask() error: %v", err)
	}

	if result.PreviousAssignee != "coder-1" {
		t.Errorf("PreviousAssignee = %q, want %q", result.PreviousAssignee, "coder-1")
	}
	if result.WorktreeRecreated {
		t.Error("WorktreeRecreated should be false for same coder reclaim")
	}
}

// readClaimStateForTest reads state for claim test verification.
func readClaimStateForTest(t *testing.T, stateFile string) *models.State {
	t.Helper()
	bb := db.New(stateFile)
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	return state
}
