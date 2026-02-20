package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestMarkBlocked_Validation(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		reason      string
		questions   []string
		agentID     string
		errContains string
	}{
		{
			name:   "empty task ID",
			reason: "blocked", questions: []string{"q1"}, agentID: "coder-1",
			errContains: "task ID is required",
		},
		{
			name:   "empty reason",
			taskID: "t1", questions: []string{"q1"}, agentID: "coder-1",
			errContains: "reason is required",
		},
		{
			name:   "empty agent ID",
			taskID: "t1", reason: "blocked", questions: []string{"q1"},
			errContains: "agent ID is required",
		},
		{
			name:   "no questions",
			taskID: "t1", reason: "blocked", questions: []string{}, agentID: "coder-1",
			errContains: "at least 1 question",
		},
		{
			name:   "too many questions",
			taskID: "t1", reason: "blocked", questions: []string{"q1", "q2", "q3", "q4"}, agentID: "coder-1",
			errContains: "maximum 3 questions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := MarkBlocked("/nonexistent", tt.taskID, tt.reason, tt.questions, tt.agentID)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Error = %q, want to contain %q", err.Error(), tt.errContains)
			}
		})
	}
}

func TestMarkBlocked_Success(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	questions := []string{"What is the API format?", "Where is the config?"}
	result, err := MarkBlocked(tmpDir, "task-1", "Missing API spec", questions, "coder-1")
	if err != nil {
		t.Fatalf("MarkBlocked() error: %v", err)
	}

	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
	if result.Reason != "Missing API spec" {
		t.Errorf("Reason = %q, want %q", result.Reason, "Missing API spec")
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
	if task.Status != models.TaskStatusBlocked {
		t.Errorf("Status = %v, want BLOCKED", task.Status)
	}
	if task.BlockedReason == nil || *task.BlockedReason != "Missing API spec" {
		t.Error("BlockedReason not set correctly")
	}
	if len(task.BlockedQuestions) != 2 {
		t.Errorf("BlockedQuestions len = %d, want 2", len(task.BlockedQuestions))
	}
	if task.AssignedTo != nil {
		t.Error("AssignedTo should be nil after blocking")
	}
	if task.LeaseExpires != nil {
		t.Error("LeaseExpires should be nil after blocking")
	}

	// Verify history entry
	lastHistory := task.History[len(task.History)-1]
	if lastHistory.Event != "blocked" {
		t.Errorf("History event = %q, want %q", lastHistory.Event, "blocked")
	}
}

func TestMarkBlocked_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := MarkBlocked(tmpDir, "nonexistent", "reason", []string{"q1"}, "coder-1")
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !strings.Contains(err.Error(), "task not found") {
		t.Errorf("Error = %q, want to contain 'task not found'", err.Error())
	}
}

func TestMarkBlocked_WrongStatus(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := MarkBlocked(tmpDir, "task-1", "reason", []string{"q1"}, "coder-1")
	if err == nil {
		t.Fatal("Expected error for non-IMPLEMENTING task")
	}
	if !strings.Contains(err.Error(), "IMPLEMENTING") {
		t.Errorf("Error = %q, want to contain 'IMPLEMENTING'", err.Error())
	}
}

func TestMarkBlocked_WrongAgent(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := MarkBlocked(tmpDir, "task-1", "reason", []string{"q1"}, "coder-2")
	if err == nil {
		t.Fatal("Expected error for wrong agent")
	}
	if !strings.Contains(err.Error(), "assigned agent") {
		t.Errorf("Error = %q, want to contain 'assigned agent'", err.Error())
	}
}
