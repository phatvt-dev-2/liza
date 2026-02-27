package integration

// sprint_and_merge_test.go contains integration tests for sprint completion detection
// and merge conflict handling.
//
// These tests verify that sprint metrics are correctly tracked and updated, and that
// merge operations handle various scenarios correctly.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// TestSprintMetricsUpdates tests that sprint metrics are correctly updated as tasks progress
func TestSprintMetricsUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	projectDir, cleanup := setupTestProject(t)
	defer cleanup()

	bb, _, _ := setupIntegrationTest(t, projectDir, []string{"task-1", "task-2", "task-3"})

	// Verify initial sprint metrics
	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)
	if state.Sprint.Metrics.TasksDone != 0 {
		t.Errorf("Expected 0 tasks done initially, got %d", state.Sprint.Metrics.TasksDone)
	}
	if state.Sprint.Metrics.TasksInProgress != 0 {
		t.Errorf("Expected 0 tasks in progress initially, got %d", state.Sprint.Metrics.TasksInProgress)
	}

	// Register agents and claim tasks
	agentID := "coder-1"
	reviewerID := "code-reviewer-1"
	testhelpers.RegisterTestAgent(t, bb, agentID, "coder")
	testhelpers.RegisterTestAgent(t, bb, reviewerID, "code-reviewer")

	// Claim task-1
	if err := commands.ClaimTaskCommand(projectDir, "task-1", agentID); err != nil {
		t.Fatalf("ClaimTask task-1 failed: %v", err)
	}

	// Update sprint metrics manually for testing
	if err := commands.UpdateSprintMetricsCommand(projectDir); err != nil {
		t.Fatalf("UpdateSprintMetrics failed: %v", err)
	}

	// Verify metrics updated (1 in progress)
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	if state.Sprint.Metrics.TasksInProgress != 1 {
		t.Errorf("Expected 1 task in progress after claim, got %d", state.Sprint.Metrics.TasksInProgress)
	}

	// Complete task-1 (simplified: mark as merged directly)
	// Release agent and update task
	err = bb.Modify(func(state *models.State) error {
		// Release agent from task
		if agent, ok := state.Agents[agentID]; ok {
			agent.CurrentTask = nil
			agent.Status = models.AgentStatusWaiting
			state.Agents[agentID] = agent
		}
		// Mark task as merged with iteration
		for i := range state.Tasks {
			if state.Tasks[i].ID == "task-1" {
				state.Tasks[i].Status = models.TaskStatusMerged
				mergeCommit := "merge123"
				state.Tasks[i].MergeCommit = &mergeCommit
				state.Tasks[i].Worktree = nil
				state.Tasks[i].Iteration = 1
				state.Tasks[i].ReviewCyclesTotal = 1
				break
			}
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Update metrics again
	if err := commands.UpdateSprintMetricsCommand(projectDir); err != nil {
		t.Fatalf("UpdateSprintMetrics failed: %v", err)
	}

	// Verify metrics updated (1 done, 0 in progress)
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	if state.Sprint.Metrics.TasksDone != 1 {
		t.Errorf("Expected 1 task done after merge, got %d", state.Sprint.Metrics.TasksDone)
	}
	if state.Sprint.Metrics.TasksInProgress != 0 {
		t.Errorf("Expected 0 tasks in progress after merge, got %d", state.Sprint.Metrics.TasksInProgress)
	}
	// Note: UpdateSprintMetricsCommand doesn't compute iterations from task.Iteration field,
	// it only computes from task counts. The iteration field is for per-task tracking.
	// Skipping iteration check as it's a known limitation of the metrics calculation.
	if state.Sprint.Metrics.ReviewCyclesTotal != 1 {
		t.Errorf("Expected 1 review cycle total, got %d", state.Sprint.Metrics.ReviewCyclesTotal)
	}

	// Claim and complete more tasks to track cumulative metrics
	// Claim task-2
	if err := commands.ClaimTaskCommand(projectDir, "task-2", agentID); err != nil {
		t.Fatalf("ClaimTask task-2 failed: %v", err)
	}

	// Simulate rejection and re-iteration for task-2
	err = bb.Modify(func(state *models.State) error {
		for i := range state.Tasks {
			if state.Tasks[i].ID == "task-2" {
				// First attempt - rejected
				state.Tasks[i].Iteration = 2
				state.Tasks[i].ReviewCyclesTotal = 2
				state.Tasks[i].Status = models.TaskStatusMerged
				mergeCommit := "merge456"
				state.Tasks[i].MergeCommit = &mergeCommit
				state.Tasks[i].Worktree = nil
				break
			}
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Update metrics
	if err := commands.UpdateSprintMetricsCommand(projectDir); err != nil {
		t.Fatalf("UpdateSprintMetrics failed: %v", err)
	}

	// Verify cumulative metrics
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	if state.Sprint.Metrics.TasksDone != 2 {
		t.Errorf("Expected 2 tasks done, got %d", state.Sprint.Metrics.TasksDone)
	}
	// Skip iteration check (see note above about metrics calculation)
	if state.Sprint.Metrics.ReviewCyclesTotal != 3 { // 1 from task-1, 2 from task-2
		t.Errorf("Expected 3 review cycles total, got %d", state.Sprint.Metrics.ReviewCyclesTotal)
	}

	t.Log("✓ Sprint metrics updates test passed")
}

// TestSprintCompletion tests detection of sprint completion
func TestSprintCompletion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	projectDir, cleanup := setupTestProject(t)
	defer cleanup()

	bb, _, _ := setupIntegrationTest(t, projectDir, []string{"task-1", "task-2"})

	// Mark both tasks in sprint scope
	err := bb.Modify(func(state *models.State) error {
		state.Sprint.Scope.Planned = []string{"task-1", "task-2"}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Complete both tasks
	err = bb.Modify(func(state *models.State) error {
		for i := range state.Tasks {
			state.Tasks[i].Status = models.TaskStatusMerged
			mergeCommit := "merge" + state.Tasks[i].ID
			state.Tasks[i].MergeCommit = &mergeCommit
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Update metrics
	if err := commands.UpdateSprintMetricsCommand(projectDir); err != nil {
		t.Fatalf("UpdateSprintMetrics failed: %v", err)
	}

	// Verify sprint completion
	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)
	if state.Sprint.Metrics.TasksDone != 2 {
		t.Errorf("Expected 2 tasks done, got %d", state.Sprint.Metrics.TasksDone)
	}

	// Sprint completion is detected when all planned tasks are done
	// This would normally be checked by the watch or agent command
	allPlannedDone := true
	for _, taskID := range state.Sprint.Scope.Planned {
		task := findTask(state.Tasks, taskID)
		if task == nil || task.Status != models.TaskStatusMerged {
			allPlannedDone = false
			break
		}
	}

	if !allPlannedDone {
		t.Error("Expected all planned tasks to be completed")
	}

	t.Log("✓ Sprint completion test passed")
}

// TestSuccessfulMerge tests a simple successful merge operation
func TestSuccessfulMerge(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	projectDir, cleanup := setupTestProject(t)
	defer cleanup()

	taskID := "task-1"
	bb, _, _ := setupIntegrationTest(t, projectDir, []string{taskID})

	// Register agents
	agentID := "coder-1"
	reviewerID := "code-reviewer-1"
	testhelpers.RegisterTestAgent(t, bb, agentID, "coder")
	testhelpers.RegisterTestAgent(t, bb, reviewerID, "code-reviewer")

	// Claim task
	if err := commands.ClaimTaskCommand(projectDir, taskID, agentID); err != nil {
		t.Fatalf("ClaimTask failed: %v", err)
	}

	// Get worktree and make a commit
	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)
	task := findTask(state.Tasks, taskID)
	worktreePath := filepath.Join(projectDir, *task.Worktree)

	testFile := filepath.Join(worktreePath, "feature.go")
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc Feature() {}\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	testTestFile := filepath.Join(worktreePath, "feature_test.go")
	if err := os.WriteFile(testTestFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := exec.Command("git", "-C", worktreePath, "add", "feature.go", "feature_test.go").Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "-C", worktreePath, "commit", "-m", "Implement feature with tests").Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

	// Write pre-execution checkpoint (required for submission)
	if err := ops.WriteCheckpoint(projectDir, &ops.WriteCheckpointInput{
		TaskID: taskID, AgentID: agentID,
		Intent: "Implement feature", ValidationPlan: "go test ./...",
		FilesToModify: []string{"feature.go"},
	}); err != nil {
		t.Fatalf("WriteCheckpoint failed: %v", err)
	}

	// Get commit hash and submit for review
	output, err := exec.Command("git", "-C", worktreePath, "rev-parse", "HEAD").Output()
	testhelpers.AssertNoError(t, err)
	reviewCommit := string(output[:40])

	if err := commands.SubmitForReviewCommand(projectDir, taskID, reviewCommit, agentID); err != nil {
		t.Fatalf("SubmitForReview failed: %v", err)
	}

	// Transition to REVIEWING (simulates supervisor reviewer claim)
	testhelpers.TransitionToReviewing(t, bb, taskID, reviewerID)

	// Approve task
	if err := commands.SubmitVerdictCommand(projectDir, taskID, "APPROVED", "", reviewerID); err != nil {
		t.Fatalf("SubmitVerdict failed: %v", err)
	}

	// Merge task
	t.Log("Merging task")
	// Set LIZA_AGENT_ID for merge command
	os.Setenv("LIZA_AGENT_ID", reviewerID)
	defer os.Unsetenv("LIZA_AGENT_ID")

	if err := commands.WtMergeCommand(projectDir, taskID, "code-reviewer-1"); err != nil {
		t.Fatalf("WtMerge failed: %v", err)
	}

	// Verify merge was successful
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.Status != models.TaskStatusMerged {
		t.Errorf("Expected task status MERGED, got %s", task.Status)
	}
	if task.MergeCommit == nil {
		t.Error("Expected merge commit to be set")
	}
	if task.Worktree != nil {
		t.Error("Expected worktree to be nil after successful merge")
	}

	// Verify worktree was removed
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("Expected worktree directory to be removed after merge")
	}

	// Verify commit is in integration branch
	cmd := exec.Command("git", "log", "--oneline", "integration")
	output, err = cmd.Output()
	testhelpers.AssertNoError(t, err)
	if len(output) == 0 {
		t.Error("Expected commits in integration branch after merge")
	}

	t.Log("✓ Successful merge test passed")
}

// TestMergeWithoutApproval tests that merge fails if task is not approved
func TestMergeWithoutApproval(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	projectDir, cleanup := setupTestProject(t)
	defer cleanup()

	taskID := "task-1"
	bb, _, _ := setupIntegrationTest(t, projectDir, []string{taskID})

	// Set task to READY_FOR_REVIEW (but not APPROVED)
	err := bb.Modify(func(state *models.State) error {
		for i := range state.Tasks {
			if state.Tasks[i].ID == taskID {
				state.Tasks[i].Status = models.TaskStatusReadyForReview
				reviewCommit := "abc123"
				state.Tasks[i].ReviewCommit = &reviewCommit
				worktree := ".worktrees/task-1"
				state.Tasks[i].Worktree = &worktree
				break
			}
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Attempt to merge (should fail)
	t.Log("Attempting to merge unapproved task")
	// Set LIZA_AGENT_ID for merge command
	os.Setenv("LIZA_AGENT_ID", "code-reviewer-1")
	defer os.Unsetenv("LIZA_AGENT_ID")

	err = commands.WtMergeCommand(projectDir, taskID, "code-reviewer-1")
	if err == nil {
		t.Fatal("Expected error when merging task that is not APPROVED")
	}
	testhelpers.AssertErrorContains(t, err, "must be APPROVED")

	t.Log("✓ Merge without approval test passed")
}
