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

// TestHandleGetTasksSlashID verifies liza_get with "tasks/<id>" query format
func TestHandleGetTasksSlashID(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	// "tasks/task-1" should resolve identically to bare "task-1"
	result, err := server.handleGet(map[string]any{
		"query": "tasks/task-1",
	})

	if err != nil {
		t.Fatalf("handleGet with tasks/<id> failed: %v", err)
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
		"code-reviewer-1": {
			Role:      "code-reviewer",
			Status:    models.AgentStatusIdle,
			Heartbeat: time.Now().UTC(),
		},
		"orchestrator-1": {
			Role:      "orchestrator",
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

// TestHandleAddTasks verifies liza_add_tasks tool (single task)
func TestHandleAddTasks(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleAddTasks(map[string]any{
		"tasks": []any{
			map[string]any{
				"id":       "task-new",
				"desc":     "New test task",
				"spec":     "specs/test-spec.md",
				"done":     "Task is complete",
				"scope":    "Add new feature",
				"priority": 1,
			},
		},
		"agent_id": "orchestrator-1",
	})

	if err != nil {
		t.Fatalf("handleAddTasks failed: %v", err)
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

// TestHandleAddTasksWithInvalidParams verifies error handling
func TestHandleAddTasksWithInvalidParams(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	// Missing required field in task
	_, err := server.handleAddTasks(map[string]any{
		"tasks": []any{
			map[string]any{
				"id": "task-new",
				// missing desc
			},
		},
		"agent_id": "orchestrator-1",
	})

	if err == nil {
		t.Error("Expected error for missing required field")
	}
}

// TestHandleAddTasksBatch verifies adding multiple tasks in one call
func TestHandleAddTasksBatch(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleAddTasks(map[string]any{
		"tasks": []any{
			map[string]any{
				"id":    "batch-1",
				"desc":  "First batch task",
				"spec":  "specs/test-spec.md",
				"done":  "Done",
				"scope": "scope",
			},
			map[string]any{
				"id":    "batch-2",
				"desc":  "Second batch task",
				"spec":  "specs/test-spec.md",
				"done":  "Done",
				"scope": "scope",
			},
		},
		"agent_id": "orchestrator-1",
	})

	if err != nil {
		t.Fatalf("handleAddTasks failed: %v", err)
	}

	// Verify result format
	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}
	contentArr, ok := content["content"].([]any)
	if !ok || len(contentArr) == 0 {
		t.Fatal("Expected content array")
	}
	textMap, ok := contentArr[0].(map[string]any)
	if !ok {
		t.Fatal("Expected text content map")
	}
	text, ok := textMap["text"].(string)
	if !ok {
		t.Fatal("Expected text string")
	}
	if !strings.Contains(text, "Added 2/2 tasks") {
		t.Errorf("Expected 'Added 2/2 tasks' in result, got %q", text)
	}

	// Verify both tasks exist in state
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if state.FindTask("batch-1") == nil {
		t.Error("batch-1 not found in state")
	}
	if state.FindTask("batch-2") == nil {
		t.Error("batch-2 not found in state")
	}
}

// TestHandleAddTasksBatchPartialFailure verifies partial failure reporting
func TestHandleAddTasksBatchPartialFailure(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	// First add a task to create a duplicate
	_, err := server.handleAddTasks(map[string]any{
		"tasks": []any{
			map[string]any{
				"id":    "existing-task",
				"desc":  "Pre-existing",
				"spec":  "specs/test-spec.md",
				"done":  "Done",
				"scope": "scope",
			},
		},
		"agent_id": "orchestrator-1",
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Now batch with a good task and a duplicate
	result, err := server.handleAddTasks(map[string]any{
		"tasks": []any{
			map[string]any{
				"id":    "new-task",
				"desc":  "A new task",
				"spec":  "specs/test-spec.md",
				"done":  "Done",
				"scope": "scope",
			},
			map[string]any{
				"id":    "existing-task",
				"desc":  "Duplicate",
				"spec":  "specs/test-spec.md",
				"done":  "Done",
				"scope": "scope",
			},
		},
		"agent_id": "orchestrator-1",
	})
	if err != nil {
		t.Fatalf("handleAddTasks failed: %v", err)
	}

	content := result.(map[string]any)
	text := content["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "Added 1/2 tasks") {
		t.Errorf("Expected 'Added 1/2 tasks', got %q", text)
	}
	if !strings.Contains(text, "error:") {
		t.Errorf("Expected error line for duplicate, got %q", text)
	}
}

// TestHandleAddTasksEmptyArray verifies empty tasks array is rejected
func TestHandleAddTasksEmptyArray(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	_, err := server.handleAddTasks(map[string]any{
		"tasks":    []any{},
		"agent_id": "orchestrator-1",
	})
	if err == nil {
		t.Fatal("Expected error for empty tasks array")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("Expected 'must not be empty' error, got: %v", err)
	}
}

// TestHandleAddTasksMalformedEntry verifies non-object in array produces indexed error
func TestHandleAddTasksMalformedEntry(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	_, err := server.handleAddTasks(map[string]any{
		"tasks":    []any{"not-an-object"},
		"agent_id": "orchestrator-1",
	})
	if err == nil {
		t.Fatal("Expected error for malformed entry")
	}
	if !strings.Contains(err.Error(), "tasks[0]") {
		t.Errorf("Expected indexed error 'tasks[0]', got: %v", err)
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

	// Make commits in the worktree (including a test file for TDD enforcement)
	implFile := filepath.Join(wtPath, "feature.go")
	if err := os.WriteFile(implFile, []byte("package feature\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(wtPath, "feature_test.go")
	if err := os.WriteFile(testFile, []byte("package feature\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, wtPath, "add", "feature.go", "feature_test.go")
	testhelpers.MustGit(t, wtPath, "commit", "-m", "Add feature with tests")

	// Get the commit SHA using git package
	commitSHA, err := g.GetWorktreeHEAD(taskID)
	if err != nil {
		t.Fatalf("Failed to get commit SHA: %v", err)
	}

	// Setup: Claim task with the worktree and add checkpoint
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	err = bb.Modify(func(state *models.State) error {
		state.Tasks[0].Status = models.TaskStatusImplementing
		assignedTo := "coder-1"
		state.Tasks[0].AssignedTo = &assignedTo
		worktree := g.GetWorktreeRelPath(taskID)
		state.Tasks[0].Worktree = &worktree
		state.Tasks[0].BaseCommit = &baseCommit
		// Add pre-execution checkpoint (required for submission)
		agent := "coder-1"
		state.Tasks[0].History = append(state.Tasks[0].History, models.TaskHistoryEntry{
			Time:  time.Now().UTC(),
			Event: "pre_execution_checkpoint",
			Agent: &agent,
			Extra: map[string]any{"intent": "test", "validation_plan": "test", "files_to_modify": []string{"feature.go"}},
		})
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

func TestHandleSubmitForReviewCommitMismatch(t *testing.T) {
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

	// Make commits in the worktree (including test file for TDD enforcement)
	implFile := filepath.Join(wtPath, "feature.go")
	if err := os.WriteFile(implFile, []byte("package feature\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(wtPath, "feature_test.go")
	if err := os.WriteFile(testFile, []byte("package feature\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, wtPath, "add", "feature.go", "feature_test.go")
	testhelpers.MustGit(t, wtPath, "commit", "-m", "Add feature with tests")

	// Use integration HEAD as an intentionally wrong commit SHA
	wrongCommit := testhelpers.MustGit(t, projectRoot, "rev-parse", "integration")

	// Setup: Claim task with the worktree and add checkpoint
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	err = bb.Modify(func(state *models.State) error {
		state.Tasks[0].Status = models.TaskStatusImplementing
		assignedTo := "coder-1"
		state.Tasks[0].AssignedTo = &assignedTo
		worktree := g.GetWorktreeRelPath(taskID)
		state.Tasks[0].Worktree = &worktree
		state.Tasks[0].BaseCommit = &baseCommit
		// Add pre-execution checkpoint (required for submission)
		agent := "coder-1"
		state.Tasks[0].History = append(state.Tasks[0].History, models.TaskHistoryEntry{
			Time:  time.Now().UTC(),
			Event: "pre_execution_checkpoint",
			Agent: &agent,
			Extra: map[string]any{"intent": "test", "validation_plan": "test", "files_to_modify": []string{"test-file.txt"}},
		})
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to modify state: %v", err)
	}

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	_, err = server.handleSubmitForReview(map[string]any{
		"task_id":    taskID,
		"commit_sha": wrongCommit,
		"agent_id":   "coder-1",
	})
	if err == nil {
		t.Fatal("Expected commit mismatch error")
	}
	if !strings.Contains(err.Error(), "does not match worktree HEAD") {
		t.Fatalf("Expected mismatch error, got: %v", err)
	}

	// Verify task remains unchanged
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := state.Tasks[0]
	if task.Status != models.TaskStatusImplementing {
		t.Errorf("Expected status IMPLEMENTING after mismatch, got %s", task.Status)
	}
	if task.ReviewCommit != nil {
		t.Errorf("Expected review_commit to remain nil after mismatch, got %v", task.ReviewCommit)
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
		reviewingBy := "code-reviewer-1"
		state.Tasks[0].ReviewingBy = &reviewingBy
		worktree := ".worktrees/task-1"
		state.Tasks[0].Worktree = &worktree
		currentTask := "task-1"
		reviewLease := now.Add(30 * time.Minute)
		state.Tasks[0].ReviewLeaseExpires = &reviewLease
		state.Agents["code-reviewer-1"] = models.Agent{
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
		"agent_id": "code-reviewer-1",
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

	agent := state.Agents["code-reviewer-1"]
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

// TestHandleReleaseClaim_RoleValidation verifies agents can only release their own role's claims
func TestHandleReleaseClaim_RoleValidation(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	// Setup: task claimed by coder with reviewer also assigned
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

	tests := []struct {
		name    string
		agentID string
		role    string
		wantErr string
	}{
		{
			name:    "coder releasing coder claim succeeds",
			agentID: "coder-1",
			role:    "coder",
			wantErr: "", // no error
		},
		{
			name:    "coder releasing reviewer claim rejected",
			agentID: "coder-1",
			role:    "code-reviewer",
			wantErr: "can only release coder claims",
		},
		{
			name:    "coder releasing both rejected",
			agentID: "coder-1",
			role:    "both",
			wantErr: "can only release coder claims",
		},
		{
			name:    "reviewer releasing coder claim rejected",
			agentID: "code-reviewer-1",
			role:    "coder",
			wantErr: "can only release code-reviewer claims",
		},
		{
			name:    "orchestrator releasing coder claim succeeds",
			agentID: "orchestrator-1",
			role:    "coder",
			wantErr: "", // no error
		},
		{
			name:    "orchestrator releasing both claims succeeds",
			agentID: "orchestrator-1",
			role:    "both",
			wantErr: "", // no error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.handleReleaseClaim(map[string]any{
				"task_id":  "task-1",
				"role":     tt.role,
				"agent_id": tt.agentID,
				"force":    true,
			})

			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
				// Re-setup state for next test that expects success
				_ = bb.Modify(func(state *models.State) error {
					state.Tasks[0].Status = models.TaskStatusImplementing
					assignedTo := "coder-1"
					state.Tasks[0].AssignedTo = &assignedTo
					worktree := ".worktrees/task-1"
					state.Tasks[0].Worktree = &worktree
					leaseExpires := time.Now().UTC().Add(30 * time.Minute)
					state.Tasks[0].LeaseExpires = &leaseExpires
					return nil
				})
			} else {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got: %v", tt.wantErr, err)
				}
			}
		})
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
		"agent_id":        "orchestrator-1",
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
	_, err := server.handleAddTasks(map[string]any{
		"tasks": []any{
			map[string]any{
				"id":       "task-logged",
				"desc":     "Task for log test",
				"spec":     "specs/test-spec.md",
				"done":     "Task is complete",
				"scope":    "Test logging",
				"priority": 1,
			},
		},
		"agent_id": "orchestrator-1",
	})

	if err != nil {
		t.Fatalf("handleAddTasks failed: %v", err)
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
		reviewingBy := "code-reviewer-1"
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

// TestHandleSprintCheckpoint verifies liza_sprint_checkpoint tool
func TestHandleSprintCheckpoint(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleSprintCheckpoint(map[string]any{})

	if err != nil {
		t.Fatalf("handleSprintCheckpoint failed: %v", err)
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

// ============================================================================
// Pre-Execution Checkpoint Tests
// ============================================================================

// TestHandleWriteCheckpoint verifies liza_write_checkpoint tool
func TestHandleWriteCheckpoint(t *testing.T) {
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
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to modify state: %v", err)
	}

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleWriteCheckpoint(map[string]any{
		"task_id":         "task-1",
		"agent_id":        "coder-1",
		"intent":          "Implement greeting function",
		"validation_plan": "go test ./...",
		"files_to_modify": []any{"main.go", "main_test.go"},
	})

	if err != nil {
		t.Fatalf("handleWriteCheckpoint failed: %v", err)
	}

	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}
	if content["content"] == nil {
		t.Error("Expected content field in result")
	}

	// Verify checkpoint was written to task history
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := state.Tasks[0]
	found := false
	for _, entry := range task.History {
		if entry.Event == "pre_execution_checkpoint" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected pre_execution_checkpoint in task history")
	}
}

// TestHandleWriteCheckpointWithTDDWaiver verifies tdd_not_required param is stored
func TestHandleWriteCheckpointWithTDDWaiver(t *testing.T) {
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
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to modify state: %v", err)
	}

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleWriteCheckpoint(map[string]any{
		"task_id":          "task-1",
		"agent_id":         "coder-1",
		"intent":           "Fix comment typo",
		"validation_plan":  "go build ./...",
		"files_to_modify":  []any{"main.go"},
		"tdd_not_required": "cosmetic-only: comment typo fix",
	})

	if err != nil {
		t.Fatalf("handleWriteCheckpoint failed: %v", err)
	}

	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}
	if content["content"] == nil {
		t.Error("Expected content field in result")
	}

	// Verify tdd_not_required was stored in checkpoint Extra
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := state.Tasks[0]
	var found bool
	for _, entry := range task.History {
		if entry.Event == "pre_execution_checkpoint" {
			val, ok := entry.Extra["tdd_not_required"].(string)
			if ok && val == "cosmetic-only: comment typo fix" {
				found = true
			}
			break
		}
	}
	if !found {
		t.Error("Expected tdd_not_required in checkpoint history entry")
	}
}

func TestExtractScopeExtensions(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]any
		want   int
	}{
		{
			name:   "missing key returns nil",
			params: map[string]any{},
			want:   0,
		},
		{
			name:   "wrong type returns nil",
			params: map[string]any{"scope_extensions": "not an array"},
			want:   0,
		},
		{
			name: "well-formed entries",
			params: map[string]any{
				"scope_extensions": []any{
					map[string]any{"file": "pkg/util.go", "justification": "shared helper"},
					map[string]any{"file": "pkg/types.go", "justification": "new type needed"},
				},
			},
			want: 2,
		},
		{
			name: "malformed entries skipped",
			params: map[string]any{
				"scope_extensions": []any{
					"not a map",
					map[string]any{"file": "valid.go", "justification": "ok"},
					map[string]any{"file": "", "justification": "missing file"}, // skipped: empty file
					map[string]any{"file": "no-justification.go"},               // skipped: no justification
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractScopeExtensions(tt.params, "scope_extensions")
			if len(got) != tt.want {
				t.Errorf("extractScopeExtensions() returned %d entries, want %d", len(got), tt.want)
			}
		})
	}
}

func TestHandleWriteCheckpointWithScopeExtensions(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

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

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	result, err := server.handleWriteCheckpoint(map[string]any{
		"task_id":         "task-1",
		"agent_id":        "coder-1",
		"intent":          "Add helper to shared package",
		"validation_plan": "go test ./...",
		"files_to_modify": []any{"internal/ops/main.go"},
		"scope_extensions": []any{
			map[string]any{"file": "internal/utils/helpers.go", "justification": "Need shared helper"},
		},
	})

	if err != nil {
		t.Fatalf("handleWriteCheckpoint failed: %v", err)
	}

	content, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}
	if content["content"] == nil {
		t.Error("Expected content field in result")
	}

	// Verify scope_extensions was stored in checkpoint Extra
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := state.Tasks[0]
	var found bool
	for _, entry := range task.History {
		if entry.Event == "pre_execution_checkpoint" {
			if _, ok := entry.Extra["scope_extensions"]; ok {
				found = true
			}
			break
		}
	}
	if !found {
		t.Error("Expected scope_extensions in checkpoint history entry")
	}
}

// TestHandleSubmitForReviewWithoutCheckpoint verifies submission is rejected without checkpoint
func TestHandleSubmitForReviewWithoutCheckpoint(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	// Setup: Claim task but DON'T write checkpoint
	statePath := filepath.Join(projectRoot, ".liza", "state.yaml")
	bb := db.New(statePath)
	err := bb.Modify(func(state *models.State) error {
		state.Tasks[0].Status = models.TaskStatusImplementing
		assignedTo := "coder-1"
		state.Tasks[0].AssignedTo = &assignedTo
		worktree := ".worktrees/task-1"
		state.Tasks[0].Worktree = &worktree
		baseCommit := "abc123"
		state.Tasks[0].BaseCommit = &baseCommit
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to modify state: %v", err)
	}

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	_, err = server.handleSubmitForReview(map[string]any{
		"task_id":    "task-1",
		"commit_sha": "abc123",
		"agent_id":   "coder-1",
	})

	if err == nil {
		t.Fatal("Expected error for submission without checkpoint")
	}
	if !strings.Contains(err.Error(), "pre-execution checkpoint required") {
		t.Errorf("Expected checkpoint required error, got: %v", err)
	}
}

// ============================================================================
// Role Enforcement Tests
// ============================================================================

// TestHandleRoleEnforcement verifies that handlers reject agents with the wrong role.
func TestHandleRoleEnforcement(t *testing.T) {
	projectRoot, cleanup := setupTestWorkspaceWithGit(t)
	defer cleanup()

	server := NewServer(projectRoot, filepath.Join(projectRoot, ".liza", "log.yaml"))

	tests := []struct {
		name    string
		handler func(map[string]any) (any, error)
		params  map[string]any
		wantErr string
	}{
		{
			name:    "claim_task rejects reviewer",
			handler: server.handleClaimTask,
			params:  map[string]any{"task_id": "task-1", "agent_id": "code-reviewer-1"},
			wantErr: "requires one of [coder code-planner] roles",
		},
		{
			name:    "submit_for_review rejects reviewer",
			handler: server.handleSubmitForReview,
			params:  map[string]any{"task_id": "task-1", "commit_sha": "abc123", "agent_id": "code-reviewer-1"},
			wantErr: "requires one of [coder code-planner] roles",
		},
		{
			name:    "handoff rejects reviewer",
			handler: server.handleHandoff,
			params:  map[string]any{"task_id": "task-1", "summary": "s", "next_action": "n", "agent_id": "code-reviewer-1"},
			wantErr: "requires one of [coder code-planner] roles",
		},
		{
			name:    "submit_verdict rejects coder",
			handler: server.handleSubmitVerdict,
			params:  map[string]any{"task_id": "task-1", "verdict": "APPROVED", "agent_id": "coder-1"},
			wantErr: "requires one of [code-reviewer code-plan-reviewer] roles",
		},
		{
			name:    "wt_merge rejects coder",
			handler: server.handleWtMerge,
			params:  map[string]any{"task_id": "task-1", "agent_id": "coder-1"},
			wantErr: "requires one of [code-reviewer code-plan-reviewer] roles",
		},
		{
			name:    "add_tasks rejects coder",
			handler: server.handleAddTasks,
			params:  map[string]any{"tasks": []any{map[string]any{"id": "t-new", "desc": "d", "spec": "specs/test-spec.md", "done": "d", "scope": "s"}}, "agent_id": "coder-1"},
			wantErr: "requires orchestrator role",
		},
		{
			name:    "supersede rejects coder",
			handler: server.handleSupersede,
			params:  map[string]any{"task_id": "task-1", "reason": "r", "agent_id": "coder-1"},
			wantErr: "requires orchestrator role",
		},
		{
			name:    "write_checkpoint rejects reviewer",
			handler: server.handleWriteCheckpoint,
			params:  map[string]any{"task_id": "task-1", "agent_id": "code-reviewer-1", "intent": "i", "validation_plan": "v", "files_to_modify": []any{"f"}},
			wantErr: "requires one of [coder code-planner] roles",
		},
		// Malformed agent ID cases
		{
			name:    "claim_task rejects malformed ID (no number)",
			handler: server.handleClaimTask,
			params:  map[string]any{"task_id": "task-1", "agent_id": "coder"},
			wantErr: "invalid agent ID",
		},
		{
			name:    "claim_task rejects malformed ID (non-numeric suffix)",
			handler: server.handleClaimTask,
			params:  map[string]any{"task_id": "task-1", "agent_id": "coder-abc"},
			wantErr: "invalid agent ID",
		},
		{
			name:    "submit_verdict rejects unknown role",
			handler: server.handleSubmitVerdict,
			params:  map[string]any{"task_id": "task-1", "verdict": "APPROVED", "agent_id": "foobar-1"},
			wantErr: "requires one of [code-reviewer code-plan-reviewer] roles",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.handler(tt.params)
			if err == nil {
				t.Fatal("Expected role enforcement error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}
