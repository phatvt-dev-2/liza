package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestCreateWorktree_Validation(t *testing.T) {
	_, err := CreateWorktree("/nonexistent", "", false)
	if err == nil {
		t.Fatal("Expected error for empty task ID")
	}
	if !strings.Contains(err.Error(), "task ID is required") {
		t.Errorf("Error = %q, want to contain 'task ID is required'", err.Error())
	}
}

func TestCreateWorktree_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := CreateWorktree(tmpDir, "nonexistent", false)
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestCreateWorktree_WrongStatus(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := CreateWorktree(tmpDir, "task-1", false)
	if err == nil {
		t.Fatal("Expected error for non-IMPLEMENTING task")
	}
	if !strings.Contains(err.Error(), "not IMPLEMENTING") {
		t.Errorf("Error = %q, want to contain 'not IMPLEMENTING'", err.Error())
	}
}

func TestCreateWorktree_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	// Create the worktree directory manually
	testhelpers.CreateTestWorktree(t, tmpDir, "task-1")

	result, err := CreateWorktree(tmpDir, "task-1", false)
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}

	if !result.AlreadyExisted {
		t.Error("AlreadyExisted should be true")
	}
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
}
