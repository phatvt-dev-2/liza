package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestHandleAwaitResubmissionMissingTaskID(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	_, err := server.handleAwaitResubmission(map[string]any{
		"agent_id": "code-reviewer-1",
	})
	if err == nil {
		t.Fatal("Expected error when task_id is missing")
	}
	if !strings.Contains(err.Error(), "task_id") {
		t.Errorf("Expected error about task_id, got: %v", err)
	}
}

func TestHandleAwaitResubmissionMissingAgentID(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	_, err := server.handleAwaitResubmission(map[string]any{
		"task_id": "task-1",
	})
	if err == nil {
		t.Fatal("Expected error when agent_id is missing")
	}
	if !strings.Contains(err.Error(), "agent_id") {
		t.Errorf("Expected error about agent_id, got: %v", err)
	}
}

func TestHandleAwaitResubmissionTaskNotInAwaitableStatus(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	// task-1 is in READY status (not rejected/submitted), so AwaitResubmission
	// should fail precondition check immediately without blocking.
	_, err := server.handleAwaitResubmission(map[string]any{
		"task_id":         "task-1",
		"agent_id":        "code-reviewer-1",
		"timeout_seconds": float64(5),
	})
	if err == nil {
		t.Fatal("Expected error for task not in awaitable status")
	}
	if !strings.Contains(err.Error(), "await resubmission failed") {
		t.Errorf("Expected 'await resubmission failed' error, got: %v", err)
	}
}

func TestHandleAwaitResubmission_PollWithBudget(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
	task.History = append(task.History, models.TaskHistoryEntry{
		Time:  now,
		Event: models.TaskEventRejected,
		Agent: strPtr("code-reviewer-1"),
	})
	state.Tasks = []models.Task{task}
	state.Agents = map[string]models.Agent{
		"code-reviewer-1": {Role: "code-reviewer", Status: models.AgentStatusIdle, Heartbeat: now},
	}
	testhelpers.WriteInitialState(t, statePath, state)
	logPath := filepath.Join(tmpDir, ".liza", "log.yaml")
	_ = os.WriteFile(logPath, []byte("[]\n"), 0644)

	server := NewServer(tmpDir, logPath)

	// Short block (1s), budget 10s — should return POLL with remaining budget.
	result, err := server.handleAwaitResubmission(map[string]any{
		"task_id":           "task-1",
		"agent_id":          "code-reviewer-1",
		"timeout_seconds":   float64(10),
		"max_block_seconds": float64(1),
	})
	if err != nil {
		t.Fatalf("handleAwaitResubmission error: %v", err)
	}
	text := fmt.Sprintf("%v", result)
	if !strings.Contains(text, "POLL") {
		t.Errorf("expected POLL verdict, got: %s", text)
	}
	if !strings.Contains(text, "timeout_seconds=9") {
		t.Errorf("expected remaining budget hint, got: %s", text)
	}
}

func TestHandleAwaitResubmission_TimeoutWhenBudgetExhausted(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
	task.History = append(task.History, models.TaskHistoryEntry{
		Time:  now,
		Event: models.TaskEventRejected,
		Agent: strPtr("code-reviewer-1"),
	})
	state.Tasks = []models.Task{task}
	state.Agents = map[string]models.Agent{
		"code-reviewer-1": {Role: "code-reviewer", Status: models.AgentStatusIdle, Heartbeat: now},
	}
	testhelpers.WriteInitialState(t, statePath, state)
	logPath := filepath.Join(tmpDir, ".liza", "log.yaml")
	_ = os.WriteFile(logPath, []byte("[]\n"), 0644)

	server := NewServer(tmpDir, logPath)

	// Budget equals block size — should return TIMEOUT (not POLL).
	result, err := server.handleAwaitResubmission(map[string]any{
		"task_id":           "task-1",
		"agent_id":          "code-reviewer-1",
		"timeout_seconds":   float64(1),
		"max_block_seconds": float64(1),
	})
	if err != nil {
		t.Fatalf("handleAwaitResubmission error: %v", err)
	}
	text := fmt.Sprintf("%v", result)
	if !strings.Contains(text, "TIMEOUT") {
		t.Errorf("expected TIMEOUT verdict when budget exhausted, got: %s", text)
	}
	if strings.Contains(text, "POLL") {
		t.Errorf("should not return POLL when budget is exhausted, got: %s", text)
	}
}

func strPtr(s string) *string { return &s }
