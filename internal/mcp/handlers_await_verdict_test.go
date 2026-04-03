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

func TestHandleAwaitVerdictMissingTaskID(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	_, err := server.handleAwaitVerdict(map[string]any{
		"agent_id": "coder-1",
	})
	if err == nil {
		t.Fatal("Expected error when task_id is missing")
	}
	if !strings.Contains(err.Error(), "task_id") {
		t.Errorf("Expected error about task_id, got: %v", err)
	}
}

func TestHandleAwaitVerdictMissingAgentID(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	_, err := server.handleAwaitVerdict(map[string]any{
		"task_id": "task-1",
	})
	if err == nil {
		t.Fatal("Expected error when agent_id is missing")
	}
	if !strings.Contains(err.Error(), "agent_id") {
		t.Errorf("Expected error about agent_id, got: %v", err)
	}
}

func TestHandleAwaitVerdictTaskNotInAwaitableStatus(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	// task-1 is in READY status (not submitted/reviewing), so AwaitVerdict
	// should fail precondition check immediately without blocking.
	_, err := server.handleAwaitVerdict(map[string]any{
		"task_id":         "task-1",
		"agent_id":        "coder-1",
		"timeout_seconds": float64(5),
	})
	if err == nil {
		t.Fatal("Expected error for task not in awaitable status")
	}
	if !strings.Contains(err.Error(), "await verdict failed") {
		t.Errorf("Expected 'await verdict failed' error, got: %v", err)
	}
}

func TestHandleAwaitVerdict_PollWithBudget(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
	task.History = append(task.History, models.TaskHistoryEntry{
		Time:  now,
		Event: models.TaskEventSubmittedForReview,
		Agent: strPtr("coder-1"),
	})
	state.Tasks = []models.Task{task}
	state.Agents = map[string]models.Agent{
		"coder-1": {Role: "coder", Status: models.AgentStatusIdle, Heartbeat: now},
	}
	testhelpers.WriteInitialState(t, statePath, state)
	logPath := filepath.Join(tmpDir, ".liza", "log.yaml")
	_ = os.WriteFile(logPath, []byte("[]\n"), 0644)

	server := NewServer(tmpDir, logPath)

	result, err := server.handleAwaitVerdict(map[string]any{
		"task_id":           "task-1",
		"agent_id":          "coder-1",
		"timeout_seconds":   float64(10),
		"max_block_seconds": float64(1),
	})
	if err != nil {
		t.Fatalf("handleAwaitVerdict error: %v", err)
	}
	text := fmt.Sprintf("%v", result)
	if !strings.Contains(text, "POLL") {
		t.Errorf("expected POLL verdict, got: %s", text)
	}
	if !strings.Contains(text, "timeout_seconds=9") {
		t.Errorf("expected remaining budget hint, got: %s", text)
	}
}

func TestHandleAwaitVerdict_TimeoutWhenBudgetExhausted(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
	task.History = append(task.History, models.TaskHistoryEntry{
		Time:  now,
		Event: models.TaskEventSubmittedForReview,
		Agent: strPtr("coder-1"),
	})
	state.Tasks = []models.Task{task}
	state.Agents = map[string]models.Agent{
		"coder-1": {Role: "coder", Status: models.AgentStatusIdle, Heartbeat: now},
	}
	testhelpers.WriteInitialState(t, statePath, state)
	logPath := filepath.Join(tmpDir, ".liza", "log.yaml")
	_ = os.WriteFile(logPath, []byte("[]\n"), 0644)

	server := NewServer(tmpDir, logPath)

	result, err := server.handleAwaitVerdict(map[string]any{
		"task_id":           "task-1",
		"agent_id":          "coder-1",
		"timeout_seconds":   float64(1),
		"max_block_seconds": float64(1),
	})
	if err != nil {
		t.Fatalf("handleAwaitVerdict error: %v", err)
	}
	text := fmt.Sprintf("%v", result)
	if !strings.Contains(text, "TIMEOUT") {
		t.Errorf("expected TIMEOUT verdict when budget exhausted, got: %s", text)
	}
	if strings.Contains(text, "POLL") {
		t.Errorf("should not return POLL when budget is exhausted, got: %s", text)
	}
}
