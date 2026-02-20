package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestDeleteWorktree_Validation(t *testing.T) {
	_, err := DeleteWorktree("/nonexistent", "")
	if err == nil {
		t.Fatal("Expected error for empty task ID")
	}
	if !strings.Contains(err.Error(), "task ID is required") {
		t.Errorf("Error = %q, want to contain 'task ID is required'", err.Error())
	}
}

func TestDeleteWorktree_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := DeleteWorktree(tmpDir, "nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !strings.Contains(err.Error(), "task not found") {
		t.Errorf("Error = %q, want to contain 'task not found'", err.Error())
	}
}

func TestDeleteWorktree_WrongStatus(t *testing.T) {
	statuses := []struct {
		name   string
		status models.TaskStatus
	}{
		{"IMPLEMENTING", models.TaskStatusImplementing},
		{"READY_FOR_REVIEW", models.TaskStatusReadyForReview},
		{"REVIEWING", models.TaskStatusReviewing},
		{"APPROVED", models.TaskStatusApproved},
		{"READY", models.TaskStatusReady},
	}

	for _, tt := range statuses {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

			now := time.Now().UTC()
			state := testhelpers.CreateValidState()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-1", tt.status, now),
			}
			testhelpers.WriteInitialState(t, stateFile, state)

			_, err := DeleteWorktree(tmpDir, "task-1")
			if err == nil {
				t.Fatalf("Expected error for %s task", tt.name)
			}
			if !strings.Contains(err.Error(), "cannot delete worktree") {
				t.Errorf("Error = %q, want to contain 'cannot delete worktree'", err.Error())
			}
		})
	}
}

func TestDeleteWorktree_AllowedStatuses(t *testing.T) {
	statuses := []struct {
		name   string
		status models.TaskStatus
	}{
		{"BLOCKED", models.TaskStatusBlocked},
		{"ABANDONED", models.TaskStatusAbandoned},
		{"SUPERSEDED", models.TaskStatusSuperseded},
	}

	for _, tt := range statuses {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

			now := time.Now().UTC()
			state := testhelpers.CreateValidState()
			task := testhelpers.BuildTaskByStatus("task-1", tt.status, now)
			task.Worktree = nil // No worktree to clean up
			state.Tasks = []models.Task{task}
			testhelpers.WriteInitialState(t, stateFile, state)

			result, err := DeleteWorktree(tmpDir, "task-1")
			if err != nil {
				t.Fatalf("DeleteWorktree() error: %v", err)
			}
			if result.Existed {
				t.Error("Existed should be false when no worktree set")
			}
			if result.PreviousStatus != tt.status {
				t.Errorf("PreviousStatus = %v, want %v", result.PreviousStatus, tt.status)
			}
		})
	}
}

func TestDeleteWorktree_MergedWithWarning(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
	task.Worktree = nil // Already cleaned
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := DeleteWorktree(tmpDir, "task-1")
	if err != nil {
		t.Fatalf("DeleteWorktree() error: %v", err)
	}
	if result.Existed {
		t.Error("Existed should be false")
	}
}
