package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestSubmitForReview_Validation(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		commitSHA   string
		agentID     string
		errContains string
	}{
		{
			name: "empty task ID", commitSHA: "abc123", agentID: "coder-1",
			errContains: "task ID is required",
		},
		{
			name: "empty commit SHA", taskID: "t1", agentID: "coder-1",
			errContains: "commit SHA is required",
		},
		{
			name: "empty agent ID", taskID: "t1", commitSHA: "abc123",
			errContains: "LIZA_AGENT_ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SubmitForReview("/nonexistent", tt.taskID, tt.commitSHA, tt.agentID)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Error = %q, want to contain %q", err.Error(), tt.errContains)
			}
		})
	}
}

func TestSubmitForReview_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitForReview(tmpDir, "nonexistent", "abc123", "coder-1")
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestSubmitForReview_WrongStatus(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitForReview(tmpDir, "task-1", "abc123", "coder-1")
	if err == nil {
		t.Fatal("Expected error for non-IMPLEMENTING task")
	}
	if !strings.Contains(err.Error(), "not IMPLEMENTING") {
		t.Errorf("Error = %q, want to contain 'not IMPLEMENTING'", err.Error())
	}
}

func TestSubmitForReview_WrongAgent(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitForReview(tmpDir, "task-1", "abc123", "coder-2")
	if err == nil {
		t.Fatal("Expected error for wrong agent")
	}
	if !strings.Contains(err.Error(), "not assigned to agent") {
		t.Errorf("Error = %q, want to contain 'not assigned to agent'", err.Error())
	}
}

func TestSubmitForReview_NoWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
	task.Worktree = nil // No worktree
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitForReview(tmpDir, "task-1", "abc123", "coder-1")
	if err == nil {
		t.Fatal("Expected error for missing worktree")
	}
	if !strings.Contains(err.Error(), "no worktree") {
		t.Errorf("Error = %q, want to contain 'no worktree'", err.Error())
	}
}
