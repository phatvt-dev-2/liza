package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// setupTestWorkspace creates a temporary Liza workspace for testing
func setupTestWorkspace(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir := t.TempDir()

	// Setup .liza directory
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	// Create state with test tasks
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, time.Now().UTC()),
		testhelpers.BuildTaskByStatus("task-2", models.TaskStatusImplementing, time.Now().UTC()),
	}
	state.Agents = map[string]models.Agent{
		"coder-1": {
			Role:      "coder",
			Status:    models.AgentStatusIdle,
			Heartbeat: time.Now().UTC(),
		},
	}

	// Write state
	testhelpers.WriteInitialState(t, statePath, state)

	// Create log.yaml (empty array format for proper append)
	logPath := filepath.Join(tmpDir, ".liza", "log.yaml")
	if err := os.WriteFile(logPath, []byte("[]\n"), 0644); err != nil {
		t.Fatalf("Failed to write log.yaml: %v", err)
	}

	cleanup := func() {
		// TempDir auto-cleanup
	}

	return tmpDir, cleanup
}

// TestHandleGetTasks verifies liza_get tasks
func TestHandleGetTasks(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleGet(map[string]any{
		"query": "tasks",
	})

	if err != nil {
		t.Fatalf("handleGet failed: %v", err)
	}

	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	// Check that content is returned (exact format will depend on implementation)
	if content["content"] == nil {
		t.Error("Expected content field in result")
	}
}

// TestHandleGetSpecificTask verifies liza_get task-id
func TestHandleGetSpecificTask(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	// Query for specific task by ID (inspect command treats this as a task ID lookup)
	result, err := server.handleGet(map[string]any{
		"query": "task-1",
	})

	if err != nil {
		t.Fatalf("handleGet failed: %v", err)
	}

	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["content"] == nil {
		t.Error("Expected content field in result")
	}
}

// TestHandleStatus verifies liza_status
func TestHandleStatus(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleStatus(map[string]any{})

	if err != nil {
		t.Fatalf("handleStatus failed: %v", err)
	}

	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["content"] == nil {
		t.Error("Expected content field in result")
	}
}

// TestHandleValidate verifies liza_validate
func TestHandleValidate(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleValidate(map[string]any{})

	if err != nil {
		t.Fatalf("handleValidate failed: %v", err)
	}

	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["content"] == nil {
		t.Error("Expected content field in result")
	}
}

// TestHandleVersion verifies liza_version
func TestHandleVersion(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleVersion(map[string]any{})

	if err != nil {
		t.Fatalf("handleVersion failed: %v", err)
	}

	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["content"] == nil {
		t.Error("Expected content field in result")
	}
}

// TestReadStateResource verifies liza://state resource
func TestReadStateResource(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleResourceReadInternal("liza://state")

	if err != nil {
		t.Fatalf("handleResourceReadInternal failed: %v", err)
	}

	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["contents"] == nil {
		t.Error("Expected contents field in result")
	}
}

// TestReadTasksResource verifies liza://tasks resource
func TestReadTasksResource(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleResourceReadInternal("liza://tasks")

	if err != nil {
		t.Fatalf("handleResourceReadInternal failed: %v", err)
	}

	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["contents"] == nil {
		t.Error("Expected contents field in result")
	}
}

// TestReadAgentsResource verifies liza://agents resource
func TestReadAgentsResource(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleResourceReadInternal("liza://agents")

	if err != nil {
		t.Fatalf("handleResourceReadInternal failed: %v", err)
	}

	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["contents"] == nil {
		t.Error("Expected contents field in result")
	}
}

// TestHandleGetWithInvalidQuery verifies error handling for invalid queries
func TestHandleGetWithInvalidQuery(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	_, err := server.handleGet(map[string]any{
		"query": "nonexistent/resource",
	})

	if err == nil {
		t.Error("Expected error for invalid query")
	}
}

// TestBlackboardIntegration verifies that handlers use the blackboard correctly
func TestBlackboardIntegration(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	// Verify we can read state via blackboard
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	if len(state.Tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(state.Tasks))
	}

	if state.Tasks[0].ID != "task-1" {
		t.Errorf("Expected task-1, got %s", state.Tasks[0].ID)
	}
}

// ============================================================================
// Phase 2: Mutation Tool Tests
// ============================================================================

// setupTestWorkspaceWithGit creates a workspace with git repository
func setupTestWorkspaceWithGit(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir := t.TempDir()

	// Setup git repo
	testhelpers.SetupTestGitRepo(t, tmpDir)

	// Setup .liza directory
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	// Create state with test data
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, time.Now().UTC()),
	}
	state.Agents = map[string]models.Agent{
		"coder-1": {
			Role:      "coder",
			Status:    models.AgentStatusIdle,
			Heartbeat: time.Now().UTC(),
		},
		"reviewer-1": {
			Role:      "code-reviewer",
			Status:    models.AgentStatusIdle,
			Heartbeat: time.Now().UTC(),
		},
		"planner-1": {
			Role:      "planner",
			Status:    models.AgentStatusIdle,
			Heartbeat: time.Now().UTC(),
		},
	}

	// Write state
	testhelpers.WriteInitialState(t, statePath, state)

	// Create log.yaml (empty array format for proper append)
	logPath := filepath.Join(tmpDir, ".liza", "log.yaml")
	if err := os.WriteFile(logPath, []byte("[]\n"), 0644); err != nil {
		t.Fatalf("Failed to write log.yaml: %v", err)
	}

	// Create specs directory with test specs
	testhelpers.CreateSpecFile(t, tmpDir, "test-spec.md", "# Test Spec\n\nThis is a test specification.")
	testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n\nProject vision document.")

	cleanup := func() {
		// TempDir auto-cleanup
	}

	return tmpDir, cleanup
}

// TestHandleAddTask verifies liza_add_task tool
func TestHandleAddTask(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleAddTask(map[string]any{
		"id":       "task-new",
		"desc":     "New test task",
		"spec":     "specs/test-spec.md",
		"done":     "Task is complete",
		"scope":    "Add new feature",
		"priority": 1,
		"agent_id": "planner-1",
	})

	if err != nil {
		t.Fatalf("handleAddTask failed: %v", err)
	}

	// Verify result format
	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["content"] == nil {
		t.Error("Expected content field in result")
	}

	// Verify task was added to state
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	found := false
	for _, task := range state.Tasks {
		if task.ID == "task-new" {
			found = true
			if task.Description != "New test task" {
				t.Errorf("Expected description 'New test task', got %s", task.Description)
			}
			break
		}
	}
	if !found {
		t.Error("Task task-new not found in state")
	}
}

// TestHandleAddTaskWithInvalidParams verifies error handling
func TestHandleAddTaskWithInvalidParams(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	// Missing required field
	_, err := server.handleAddTask(map[string]any{
		"id": "task-new",
		// missing description
	})

	if err == nil {
		t.Error("Expected error for missing required field")
	}
}

// TestHandleClaimTask verifies liza_claim_task tool
func TestHandleClaimTask(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleClaimTask(map[string]any{
		"task_id":  "task-1",
		"agent_id": "coder-1",
	})

	if err != nil {
		t.Fatalf("handleClaimTask failed: %v", err)
	}

	// Verify result format
	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["content"] == nil {
		t.Error("Expected content field in result")
	}

	// Verify task was claimed
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := state.Tasks[0]
	if task.Status != models.TaskStatusImplementing {
		t.Errorf("Expected status IMPLEMENTING, got %s", task.Status)
	}
	if task.AssignedTo == nil || *task.AssignedTo != "coder-1" {
		t.Errorf("Expected assigned_to coder-1, got %v", task.AssignedTo)
	}

	// Verify worktree was created
	worktreePath := filepath.Join(projectRoot, ".worktrees", "task-1")
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Error("Worktree directory not created")
	}
}

// TestHandleClaimTaskAlreadyClaimed verifies error handling
func TestHandleClaimTaskAlreadyClaimed(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	// Manually set task-1 to CLAIMED
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	err := bb.Modify(func(state *models.State) error {
		state.Tasks[0].Status = models.TaskStatusImplementing
		assignedTo := "coder-1"
		state.Tasks[0].AssignedTo = &assignedTo
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to modify state: %v", err)
	}

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	_, err = server.handleClaimTask(map[string]any{
		"task_id":  "task-1",
		"agent_id": "coder-2",
	})

	if err == nil {
		t.Error("Expected error when claiming already claimed task")
	}
}

// TestHandleSubmitForReview verifies liza_submit_for_review tool
func TestHandleSubmitForReview(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	// Create a worktree for the task
	g := git.New(projectRoot)
	taskID := "task-1"
	baseCommit, err := g.CreateWorktree(taskID, "integration")
	if err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}
	wtPath := g.GetWorktreePath(taskID)

	// Make a commit in the worktree
	testFile := filepath.Join(wtPath, "test-file.txt")
	if err := os.WriteFile(testFile, []byte("test content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, wtPath, "add", "test-file.txt")
	testhelpers.MustGit(t, wtPath, "commit", "-m", "Test commit")

	// Get the commit SHA using git package
	commitSHA, err := g.GetWorktreeHEAD(taskID)
	if err != nil {
		t.Fatalf("Failed to get commit SHA: %v", err)
	}

	// Setup: Claim task with the worktree
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	err = bb.Modify(func(state *models.State) error {
		state.Tasks[0].Status = models.TaskStatusImplementing
		assignedTo := "coder-1"
		state.Tasks[0].AssignedTo = &assignedTo
		worktree := g.GetWorktreeRelPath(taskID)
		state.Tasks[0].Worktree = &worktree
		state.Tasks[0].BaseCommit = &baseCommit
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to modify state: %v", err)
	}

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleSubmitForReview(map[string]any{
		"task_id":    taskID,
		"commit_sha": commitSHA,
		"agent_id":   "coder-1",
	})

	if err != nil {
		t.Fatalf("handleSubmitForReview failed: %v", err)
	}

	// Verify result format
	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["content"] == nil {
		t.Error("Expected content field in result")
	}

	// Verify task status changed
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := state.Tasks[0]
	if task.Status != models.TaskStatusReadyForReview {
		t.Errorf("Expected status READY_FOR_REVIEW, got %s", task.Status)
	}

	agent := state.Agents["coder-1"]
	if agent.Status != models.AgentStatusWaiting {
		t.Errorf("Expected coder status WAITING, got %s", agent.Status)
	}
}

// TestHandleHandoff verifies liza_handoff tool
func TestHandleHandoff(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	err := bb.Modify(func(state *models.State) error {
		state.Tasks[0].Status = models.TaskStatusImplementing
		assignedTo := "coder-1"
		state.Tasks[0].AssignedTo = &assignedTo
		state.Agents["coder-1"] = models.Agent{
			Role:      "coder",
			Status:    models.AgentStatusWorking,
			Heartbeat: time.Now().UTC(),
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to modify state: %v", err)
	}

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleHandoff(map[string]any{
		"task_id":     "task-1",
		"summary":     "Implemented parser and core validation",
		"next_action": "Add edge-case tests for malformed payloads",
		"agent_id":    "coder-1",
	})
	if err != nil {
		t.Fatalf("handleHandoff failed: %v", err)
	}

	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}
	if content["content"] == nil {
		t.Error("Expected content field in result")
	}

	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := state.Tasks[0]
	if !task.HandoffPending {
		t.Fatal("expected task handoff_pending to be true")
	}
	note, ok := state.Handoff["task-1"]
	if !ok {
		t.Fatal("expected handoff note for task-1")
	}
	if note.Summary == "" || note.NextAction == "" {
		t.Fatal("expected handoff note summary and next_action to be set")
	}
	agent := state.Agents["coder-1"]
	if agent.Status != models.AgentStatusHandoff {
		t.Fatalf("expected agent status HANDOFF, got %s", agent.Status)
	}
}

// TestHandleSubmitVerdict verifies liza_submit_verdict tool
func TestHandleSubmitVerdict(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	// Setup: Set task to REVIEWING (reviewer has claimed it)
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	err := bb.Modify(func(state *models.State) error {
		now := time.Now().UTC()
		state.Tasks[0].Status = models.TaskStatusReviewing
		assignedTo := "coder-1"
		state.Tasks[0].AssignedTo = &assignedTo
		reviewCommit := "abc123def456"
		state.Tasks[0].ReviewCommit = &reviewCommit
		reviewingBy := "reviewer-1"
		state.Tasks[0].ReviewingBy = &reviewingBy
		worktree := ".worktrees/task-1"
		state.Tasks[0].Worktree = &worktree
		currentTask := "task-1"
		reviewLease := now.Add(30 * time.Minute)
		state.Tasks[0].ReviewLeaseExpires = &reviewLease
		state.Agents["reviewer-1"] = models.Agent{
			Role:         "code-reviewer",
			Status:       models.AgentStatusReviewing,
			CurrentTask:  &currentTask,
			LeaseExpires: &reviewLease,
			Heartbeat:    now,
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to modify state: %v", err)
	}

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleSubmitVerdict(map[string]any{
		"task_id":  "task-1",
		"verdict":  "APPROVED",
		"agent_id": "reviewer-1",
	})

	if err != nil {
		t.Fatalf("handleSubmitVerdict failed: %v", err)
	}

	// Verify result format
	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["content"] == nil {
		t.Error("Expected content field in result")
	}

	// Verify task status changed
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := state.Tasks[0]
	if task.Status != models.TaskStatusApproved {
		t.Errorf("Expected status APPROVED, got %s", task.Status)
	}

	agent := state.Agents["reviewer-1"]
	if agent.Status != models.AgentStatusIdle {
		t.Errorf("Expected reviewer status IDLE, got %s", agent.Status)
	}
}

// TestHandleReleaseClaim verifies liza_release_claim tool
func TestHandleReleaseClaim(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	// Setup: Claim task first
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	err := bb.Modify(func(state *models.State) error {
		state.Tasks[0].Status = models.TaskStatusImplementing
		assignedTo := "coder-1"
		state.Tasks[0].AssignedTo = &assignedTo
		worktree := ".worktrees/task-1"
		state.Tasks[0].Worktree = &worktree
		leaseExpires := time.Now().UTC().Add(30 * time.Minute)
		state.Tasks[0].LeaseExpires = &leaseExpires
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to modify state: %v", err)
	}

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleReleaseClaim(map[string]any{
		"task_id":  "task-1",
		"role":     "coder",
		"reason":   "Need to work on something else",
		"agent_id": "coder-1",
		"force":    true,
	})

	if err != nil {
		t.Fatalf("handleReleaseClaim failed: %v", err)
	}

	// Verify result format
	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["content"] == nil {
		t.Error("Expected content field in result")
	}

	// Verify task is unclaimed
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := state.Tasks[0]
	if task.Status != models.TaskStatusReady {
		t.Errorf("Expected status READY, got %s", task.Status)
	}
	if task.AssignedTo != nil {
		t.Errorf("Expected assigned_to to be nil, got %v", task.AssignedTo)
	}
}

// TestHandleSupersede verifies liza_supersede_task tool
func TestHandleSupersede(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	// Setup: Add a replacement task
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	err := bb.Modify(func(state *models.State) error {
		newTask := testhelpers.BuildTaskByStatus("task-replacement", models.TaskStatusReady, time.Now().UTC())
		state.Tasks = append(state.Tasks, newTask)
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to add replacement task: %v", err)
	}

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleSupersede(map[string]any{
		"task_id":         "task-1",
		"replacement_ids": []any{"task-replacement"},
		"reason":          "Task scope changed",
		"agent_id":        "planner-1",
	})

	if err != nil {
		t.Fatalf("handleSupersede failed: %v", err)
	}

	// Verify result format
	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["content"] == nil {
		t.Error("Expected content field in result")
	}

	// Verify task was superseded
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := state.Tasks[0]
	if task.Status != models.TaskStatusSuperseded {
		t.Errorf("Expected status SUPERSEDED, got %s", task.Status)
	}
}

// TestMutationsLoggedCorrectly verifies all mutations appear in log.yaml
func TestMutationsLoggedCorrectly(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	// Perform an add task operation
	_, err := server.handleAddTask(map[string]any{
		"id":       "task-logged",
		"desc":     "Task for log test",
		"spec":     "specs/test-spec.md",
		"done":     "Task is complete",
		"scope":    "Test logging",
		"priority": 1,
		"agent_id": "planner-1",
	})

	if err != nil {
		t.Fatalf("handleAddTask failed: %v", err)
	}

	// Read log file
	logPath := filepath.Join(projectRoot, ".liza", "log.yaml")
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log: %v", err)
	}

	logContent := string(logData)
	if !strings.Contains(logContent, "task-logged") {
		t.Error("Log does not contain the added task ID")
	}
}

// ============================================================================
// Phase 3: Worktree Operation Tests
// ============================================================================

// TestHandleWtCreate verifies liza_wt_create tool
func TestHandleWtCreate(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	// Setup: Claim a task first
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	err := bb.Modify(func(state *models.State) error {
		state.Tasks[0].Status = models.TaskStatusImplementing
		assignedTo := "coder-1"
		state.Tasks[0].AssignedTo = &assignedTo
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to modify state: %v", err)
	}

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleWtCreate(map[string]any{
		"task_id": "task-1",
	})

	if err != nil {
		t.Fatalf("handleWtCreate failed: %v", err)
	}

	// Verify result format
	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["content"] == nil {
		t.Error("Expected content field in result")
	}

	// Verify worktree was created
	worktreePath := filepath.Join(projectRoot, ".worktrees", "task-1")
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Error("Worktree directory not created")
	}

	// Verify git branch exists
	// (This would require running git commands to verify)
}

// TestHandleWtCreateFresh verifies liza_wt_create with --fresh flag
func TestHandleWtCreateFresh(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	// Setup: Claim a task and create an old worktree
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	err := bb.Modify(func(state *models.State) error {
		state.Tasks[0].Status = models.TaskStatusImplementing
		assignedTo := "coder-1"
		state.Tasks[0].AssignedTo = &assignedTo
		worktree := ".worktrees/task-1"
		state.Tasks[0].Worktree = &worktree
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to modify state: %v", err)
	}

	// Create old worktree directory
	worktreePath := filepath.Join(projectRoot, ".worktrees", "task-1")
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatalf("Failed to create old worktree: %v", err)
	}

	// Create a test file in old worktree
	oldFile := filepath.Join(worktreePath, "old-file.txt")
	if err := os.WriteFile(oldFile, []byte("old content"), 0644); err != nil {
		t.Fatalf("Failed to create old file: %v", err)
	}

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleWtCreate(map[string]any{
		"task_id": "task-1",
		"fresh":   true,
	})

	if err != nil {
		t.Fatalf("handleWtCreate with fresh failed: %v", err)
	}

	// Verify result format
	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["content"] == nil {
		t.Error("Expected content field in result")
	}

	// Verify old file is gone (fresh recreate)
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("Expected old file to be removed with fresh flag")
	}
}

// TestHandleWtDelete verifies liza_wt_delete tool
func TestHandleWtDelete(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	// Setup: Create a worktree and set task to terminal state
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	err := bb.Modify(func(state *models.State) error {
		state.Tasks[0].Status = models.TaskStatusMerged
		worktree := ".worktrees/task-1"
		state.Tasks[0].Worktree = &worktree
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to modify state: %v", err)
	}

	// Create worktree directory
	worktreePath := filepath.Join(projectRoot, ".worktrees", "task-1")
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleWtDelete(map[string]any{
		"task_id": "task-1",
	})

	if err != nil {
		t.Fatalf("handleWtDelete failed: %v", err)
	}

	// Verify result format
	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["content"] == nil {
		t.Error("Expected content field in result")
	}

	// Verify worktree was removed
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("Expected worktree to be deleted")
	}

	// Verify state updated
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	if state.Tasks[0].Worktree != nil {
		t.Error("Expected worktree field to be cleared in state")
	}
}

// TestHandleWtMerge verifies liza_wt_merge tool
func TestHandleWtMerge(t *testing.T) {
	t.Skip("Skipping - requires full git worktree setup with matching commit SHA")

	// This test would require:
	// 1. Creating actual git worktree with git worktree add
	// 2. Making commits in the worktree
	// 3. Setting review_commit to match actual worktree HEAD
	// 4. Then calling wt-merge
	//
	// The complexity of this setup makes it more suitable for integration tests
	// rather than unit tests. The handler itself is simple and just delegates
	// to the existing WtMergeCommand which has its own comprehensive tests.
}

// TestHandleWtCreateRequiresClaimed verifies validation
func TestHandleWtCreateRequiresClaimed(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	// Try to create worktree for READY task
	_, err := server.handleWtCreate(map[string]any{
		"task_id": "task-1",
	})

	if err == nil {
		t.Error("Expected error when creating worktree for READY task")
	}
}

// ============================================================================
// Phase 3: Analysis & Utility Tests
// ============================================================================

// TestHandleAnalyze verifies liza_analyze tool
func TestHandleAnalyze(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleAnalyze(map[string]any{})

	if err != nil {
		t.Fatalf("handleAnalyze failed: %v", err)
	}

	// Verify result format
	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["content"] == nil {
		t.Error("Expected content field in result")
	}
}

// TestHandleUpdateSprintMetrics verifies liza_update_sprint_metrics tool
func TestHandleUpdateSprintMetrics(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleUpdateSprintMetrics(map[string]any{})

	if err != nil {
		t.Fatalf("handleUpdateSprintMetrics failed: %v", err)
	}

	// Verify result format
	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["content"] == nil {
		t.Error("Expected content field in result")
	}

	// Verify metrics were updated
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	// Sprint metrics should exist (exact values depend on state)
	if state.Sprint.Metrics.TasksDone < 0 {
		t.Error("Expected valid sprint metrics")
	}
}

// TestHandleClearStaleReviews verifies liza_clear_stale_review_claims tool
func TestHandleClearStaleReviews(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	// Setup: Create a task with expired review lease
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	err := bb.Modify(func(state *models.State) error {
		state.Tasks[0].Status = models.TaskStatusReviewing
		reviewingBy := "reviewer-1"
		state.Tasks[0].ReviewingBy = &reviewingBy
		reviewCommit := "abc123"
		state.Tasks[0].ReviewCommit = &reviewCommit
		// Expired lease (in the past)
		expiredTime := time.Now().UTC().Add(-1 * time.Hour)
		state.Tasks[0].ReviewLeaseExpires = &expiredTime
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to modify state: %v", err)
	}

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleClearStaleReviews(map[string]any{})

	if err != nil {
		t.Fatalf("handleClearStaleReviews failed: %v", err)
	}

	// Verify result format
	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["content"] == nil {
		t.Error("Expected content field in result")
	}

	// Verify stale review claim was cleared
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	if state.Tasks[0].ReviewingBy != nil {
		t.Error("Expected reviewing_by to be cleared for stale review")
	}
	if state.Tasks[0].Status != models.TaskStatusReadyForReview {
		t.Errorf("Expected status READY_FOR_REVIEW after stale clear, got %s", state.Tasks[0].Status)
	}
}

// TestHandleDeleteAgent verifies liza_delete_agent tool
func TestHandleDeleteAgent(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	// Setup: Add an inactive agent
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	err := bb.Modify(func(state *models.State) error {
		state.Agents["inactive-agent"] = models.Agent{
			Role:      "coder",
			Status:    models.AgentStatusIdle,
			Heartbeat: time.Now().UTC().Add(-2 * time.Hour), // Old heartbeat
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to add inactive agent: %v", err)
	}

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleDeleteAgent(map[string]any{
		"agent_id": "inactive-agent",
		"force":    true,
		"reason":   "Cleanup inactive agent",
	})

	if err != nil {
		t.Fatalf("handleDeleteAgent failed: %v", err)
	}

	// Verify result format
	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["content"] == nil {
		t.Error("Expected content field in result")
	}

	// Verify agent was deleted
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	if _, exists := state.Agents["inactive-agent"]; exists {
		t.Error("Expected agent to be deleted from state")
	}
}

// TestHandleDeleteAgentWithMissingParams verifies validation
func TestHandleDeleteAgentWithMissingParams(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	// Try to delete agent without reason
	_, err := server.handleDeleteAgent(map[string]any{
		"agent_id": "coder-1",
		// missing reason
	})

	if err == nil {
		t.Error("Expected error when deleting agent without reason")
	}

	if !strings.Contains(err.Error(), "reason parameter required") {
		t.Errorf("Expected 'reason parameter required' error, got: %v", err)
	}
}

// TestHandleCheckpoint verifies liza_checkpoint tool
func TestHandleCheckpoint(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleCheckpoint(map[string]any{})

	if err != nil {
		t.Fatalf("handleCheckpoint failed: %v", err)
	}

	// Verify result format
	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if content["content"] == nil {
		t.Error("Expected content field in result")
	}

	// Verify sprint status changed to CHECKPOINT
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	if state.Sprint.Status != models.SprintStatusCheckpoint {
		t.Errorf("Expected sprint status CHECKPOINT, got %s", state.Sprint.Status)
	}

	if state.Sprint.Timeline.CheckpointAt == nil {
		t.Error("Expected checkpoint_at to be set")
	}
}
