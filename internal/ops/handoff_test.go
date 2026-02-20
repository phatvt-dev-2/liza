package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestHandoff_Validation(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		summary     string
		nextAction  string
		agentID     string
		errContains string
	}{
		{
			name: "empty task ID", summary: "s", nextAction: "n", agentID: "a",
			errContains: "task ID is required",
		},
		{
			name: "empty summary", taskID: "t1", nextAction: "n", agentID: "a",
			errContains: "summary is required",
		},
		{
			name: "empty next action", taskID: "t1", summary: "s", agentID: "a",
			errContains: "next action is required",
		},
		{
			name: "empty agent ID", taskID: "t1", summary: "s", nextAction: "n",
			errContains: "LIZA_AGENT_ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Handoff("/nonexistent", tt.taskID, tt.summary, tt.nextAction, tt.agentID)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Error = %q, want to contain %q", err.Error(), tt.errContains)
			}
		})
	}
}

func TestHandoff_Success(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Handoff(tmpDir, "task-1", "Context at 80%", "Continue from function X", "coder-1")
	if err != nil {
		t.Fatalf("Handoff() error: %v", err)
	}

	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
	if result.AgentID != "coder-1" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "coder-1")
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
	if !task.HandoffPending {
		t.Error("HandoffPending should be true")
	}

	// Verify handoff note
	note, exists := readState.Handoff["task-1"]
	if !exists {
		t.Fatal("Handoff note not found")
	}
	if note.Agent != "coder-1" {
		t.Errorf("Handoff agent = %q, want %q", note.Agent, "coder-1")
	}
	if note.Summary != "Context at 80%" {
		t.Errorf("Handoff summary = %q, want %q", note.Summary, "Context at 80%")
	}
	if note.NextAction != "Continue from function X" {
		t.Errorf("Handoff nextAction = %q, want %q", note.NextAction, "Continue from function X")
	}

	// Verify agent status
	agent, exists := readState.Agents["coder-1"]
	if !exists {
		t.Fatal("Agent not found")
	}
	if agent.Status != models.AgentStatusHandoff {
		t.Errorf("Agent status = %v, want HANDOFF", agent.Status)
	}
	if agent.CurrentTask == nil || *agent.CurrentTask != "task-1" {
		t.Error("Agent CurrentTask should be task-1")
	}

	// Verify history
	lastHistory := task.History[len(task.History)-1]
	if lastHistory.Event != "handoff_initiated" {
		t.Errorf("History event = %q, want %q", lastHistory.Event, "handoff_initiated")
	}
}

func TestHandoff_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Handoff(tmpDir, "nonexistent", "s", "n", "coder-1")
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !strings.Contains(err.Error(), "task not found") {
		t.Errorf("Error = %q, want to contain 'task not found'", err.Error())
	}
}

func TestHandoff_WrongStatus(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Handoff(tmpDir, "task-1", "s", "n", "coder-1")
	if err == nil {
		t.Fatal("Expected error for non-IMPLEMENTING task")
	}
	if !strings.Contains(err.Error(), "not IMPLEMENTING") {
		t.Errorf("Error = %q, want to contain 'not IMPLEMENTING'", err.Error())
	}
}

func TestHandoff_WrongAgent(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Handoff(tmpDir, "task-1", "s", "n", "coder-2")
	if err == nil {
		t.Fatal("Expected error for wrong agent")
	}
	if !strings.Contains(err.Error(), "not assigned to agent") {
		t.Errorf("Error = %q, want to contain 'not assigned to agent'", err.Error())
	}
}
