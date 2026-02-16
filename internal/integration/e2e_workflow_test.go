package integration

// e2e_workflow_test.go contains end-to-end integration tests for complete task workflows.
//
// These tests verify that entire workflows function correctly when commands are
// used in sequence.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// Test setup helper that returns a project directory and cleanup function
func setupTestProject(t *testing.T) (projectDir string, cleanup func()) {
	t.Helper()

	tmpDir := t.TempDir()

	// Initialize git repository
	testhelpers.SetupTestGitRepo(t, tmpDir)

	// Store original directory to restore later
	originalDir, err := os.Getwd()
	testhelpers.AssertNoError(t, err)

	// Change to tmpDir
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	cleanup = func() {
		os.Chdir(originalDir)
	}

	return tmpDir, cleanup
}

// TestSimpleWorkflow tests: init -> add-task -> claim -> submit-for-review -> approve -> merge
func TestSimpleWorkflow(t *testing.T) {
	projectDir, cleanup := setupTestProject(t)
	defer cleanup()

	// Create spec file
	testhelpers.CreateSpecFile(t, projectDir, "feature.md", "# Feature\nImplement feature")

	// Step 1: Initialize liza
	t.Log("Step 1: Initialize liza")
	if err := commands.InitCommand("Test goal", "specs/feature.md"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	statePath := filepath.Join(projectDir, ".liza", "state.yaml")
	logPath := filepath.Join(projectDir, ".liza", "log.yaml")
	bb := db.New(statePath)

	// Verify initialization
	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)
	if state.Goal.Description != "Test goal" {
		t.Errorf("Expected goal description 'Test goal', got %s", state.Goal.Description)
	}

	// Step 2: Add a task
	t.Log("Step 2: Add task")
	taskID := "task-1"
	taskInput := &commands.TaskInput{
		ID:          taskID,
		Description: "Implement feature X",
		DoneWhen:    "Feature is implemented",
		Scope:       "Feature",
		Priority:    1,
		SpecRef:     "specs/feature.md",
		DependsOn:   []string{},
	}

	if err := commands.AddTaskCommand(statePath, logPath, taskInput, "planner-1"); err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}

	// Verify task was added
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	if len(state.Tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(state.Tasks))
	}
	if state.Tasks[0].Status != models.TaskStatusUnclaimed {
		t.Errorf("Expected task status UNCLAIMED, got %s", state.Tasks[0].Status)
	}

	// Step 3: Register an agent
	t.Log("Step 3: Register agent")
	agentID := "coder-1"
	now := time.Now().UTC()
	agentLeaseExpires := now.Add(30 * time.Minute)

	err = bb.Modify(func(state *models.State) error {
		state.Agents[agentID] = models.Agent{
			Role:            "coder",
			Status:          models.AgentStatusWaiting,
			Heartbeat:       now,
			LeaseExpires:    &agentLeaseExpires,
			CurrentTask:     nil,
			Terminal:        "test",
			IterationsTotal: 0,
			ContextPercent:  0,
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Step 4: Claim the task
	t.Log("Step 4: Claim task")
	if err := commands.ClaimTaskCommand(projectDir, taskID, agentID); err != nil {
		t.Fatalf("ClaimTask failed: %v", err)
	}

	// Verify task was claimed
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task := findTask(state.Tasks, taskID)
	if task == nil {
		t.Fatal("Task not found after claim")
	}
	if task.Status != models.TaskStatusClaimed {
		t.Errorf("Expected task status CLAIMED, got %s", task.Status)
	}
	if task.AssignedTo == nil || *task.AssignedTo != agentID {
		t.Errorf("Expected task assigned to %s", agentID)
	}
	if task.Worktree == nil {
		t.Error("Expected worktree to be set after claim")
	}

	// Step 5: Make a commit and submit for review
	t.Log("Step 5: Make commit and submit for review")
	worktreePath := filepath.Join(projectDir, *task.Worktree)
	testFile := filepath.Join(worktreePath, "feature.go")
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc Feature() {}\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := exec.Command("git", "-C", worktreePath, "add", "feature.go").Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "-C", worktreePath, "commit", "-m", "Implement feature").Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

	// Get commit hash
	output, err := exec.Command("git", "-C", worktreePath, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("Failed to get commit hash: %v", err)
	}
	reviewCommit := string(output[:40])

	if err := commands.SubmitForReviewCommand(projectDir, taskID, reviewCommit, agentID); err != nil {
		t.Fatalf("SubmitForReview failed: %v", err)
	}

	// Verify task is ready for review
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.Status != models.TaskStatusReadyForReview {
		t.Errorf("Expected task status READY_FOR_REVIEW, got %s", task.Status)
	}

	// Step 6: Register reviewer and approve
	t.Log("Step 6: Register reviewer and approve")
	reviewerID := "reviewer-1"
	err = bb.Modify(func(state *models.State) error {
		state.Agents[reviewerID] = models.Agent{
			Role:            "reviewer",
			Status:          models.AgentStatusWaiting,
			Heartbeat:       now,
			LeaseExpires:    &agentLeaseExpires,
			CurrentTask:     nil,
			Terminal:        "test",
			IterationsTotal: 0,
			ContextPercent:  0,
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	if err := commands.SubmitVerdictCommand(projectDir, taskID, "APPROVED", "", reviewerID); err != nil {
		t.Fatalf("SubmitVerdict failed: %v", err)
	}

	// Verify task is approved
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.Status != models.TaskStatusApproved {
		t.Errorf("Expected task status APPROVED, got %s", task.Status)
	}

	// Step 7: Merge the task
	t.Log("Step 7: Merge task")
	// Set LIZA_AGENT_ID for merge command
	os.Setenv("LIZA_AGENT_ID", reviewerID)
	defer os.Unsetenv("LIZA_AGENT_ID")

	if err := commands.WtMergeCommand(projectDir, taskID, "code-reviewer-1"); err != nil {
		t.Fatalf("WtMerge failed: %v", err)
	}

	// Verify task is merged
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.Status != models.TaskStatusMerged {
		t.Errorf("Expected task status MERGED, got %s", task.Status)
	}
	if task.MergeCommit == nil {
		t.Error("Expected merge commit to be set")
	}

	t.Log("✓ Simple workflow test passed")
}

// TestTaskDependencyWorkflow tests that task dependencies are enforced across the workflow
func TestTaskDependencyWorkflow(t *testing.T) {
	projectDir, cleanup := setupTestProject(t)
	defer cleanup()

	testhelpers.CreateSpecFile(t, projectDir, "feature.md", "# Feature")

	// Initialize
	if err := commands.InitCommand("Test goal", "specs/feature.md"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	statePath := filepath.Join(projectDir, ".liza", "state.yaml")
	logPath := filepath.Join(projectDir, ".liza", "log.yaml")
	bb := db.New(statePath)

	// Add task-1 (no dependencies)
	task1Input := &commands.TaskInput{
		ID:          "task-1",
		Description: "Base feature",
		DoneWhen:    "Done",
		Scope:       "Base",
		Priority:    1,
		SpecRef:     "specs/feature.md",
		DependsOn:   []string{},
	}
	if err := commands.AddTaskCommand(statePath, logPath, task1Input, "planner-1"); err != nil {
		t.Fatalf("AddTask task-1 failed: %v", err)
	}

	// Add task-2 (depends on task-1)
	task2Input := &commands.TaskInput{
		ID:          "task-2",
		Description: "Dependent feature",
		DoneWhen:    "Done",
		Scope:       "Dependent",
		Priority:    1,
		SpecRef:     "specs/feature.md",
		DependsOn:   []string{"task-1"},
	}
	if err := commands.AddTaskCommand(statePath, logPath, task2Input, "planner-1"); err != nil {
		t.Fatalf("AddTask task-2 failed: %v", err)
	}

	// Register agent
	agentID := "coder-1"
	now := time.Now().UTC()
	agentLeaseExpires := now.Add(30 * time.Minute)

	err := bb.Modify(func(state *models.State) error {
		state.Agents[agentID] = models.Agent{
			Role:            "coder",
			Status:          models.AgentStatusWaiting,
			Heartbeat:       now,
			LeaseExpires:    &agentLeaseExpires,
			CurrentTask:     nil,
			Terminal:        "test",
			IterationsTotal: 0,
			ContextPercent:  0,
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Try to claim task-2 (should fail because task-1 not merged)
	t.Log("Attempting to claim task-2 before task-1 is done")
	err = commands.ClaimTaskCommand(projectDir, "task-2", agentID)
	if err == nil {
		t.Fatal("Expected error when claiming task with unmet dependencies")
	}
	testhelpers.AssertErrorContains(t, err, "unmet dependencies")

	// Claim task-1
	t.Log("Claiming task-1")
	if err := commands.ClaimTaskCommand(projectDir, "task-1", agentID); err != nil {
		t.Fatalf("ClaimTask task-1 failed: %v", err)
	}

	// Release agent from task-1 and mark it as merged (simplified for test)
	err = bb.Modify(func(state *models.State) error {
		// Release agent from task
		if agent, ok := state.Agents[agentID]; ok {
			agent.CurrentTask = nil
			agent.Status = models.AgentStatusWaiting
			state.Agents[agentID] = agent
		}
		// Mark task as merged
		for i := range state.Tasks {
			if state.Tasks[i].ID == "task-1" {
				state.Tasks[i].Status = models.TaskStatusMerged
				mergeCommit := "merge123"
				state.Tasks[i].MergeCommit = &mergeCommit
				state.Tasks[i].Worktree = nil
				break
			}
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Now try to claim task-2 (should succeed)
	t.Log("Claiming task-2 after task-1 is merged")
	if err := commands.ClaimTaskCommand(projectDir, "task-2", agentID); err != nil {
		t.Fatalf("ClaimTask task-2 failed after dependency was met: %v", err)
	}

	// Verify task-2 is claimed
	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)
	task := findTask(state.Tasks, "task-2")
	if task.Status != models.TaskStatusClaimed {
		t.Errorf("Expected task-2 status CLAIMED, got %s", task.Status)
	}

	t.Log("✓ Task dependency workflow test passed")
}

// findTask is a helper function to find a task by ID in a slice of tasks
func findTask(tasks []models.Task, taskID string) *models.Task {
	for i := range tasks {
		if tasks[i].ID == taskID {
			return &tasks[i]
		}
	}
	return nil
}
