package integration

// e2e_workflow_test.go contains end-to-end integration tests for complete task workflows.
//
// These tests verify that entire workflows function correctly when commands are
// used in sequence.

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// Test setup helper that returns a project directory and cleanup function
func setupTestProject(t *testing.T) (projectDir string, cleanup func()) {
	t.Helper()

	testhelpers.SetupGlobalLiza(t)

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

// setupIntegrationTest performs the common integration test setup:
// init, create spec, add tasks, and return the blackboard.
func setupIntegrationTest(t *testing.T, projectDir string, taskIDs []string) (*db.Blackboard, string, string) {
	t.Helper()

	testhelpers.CreateSpecFile(t, projectDir, "feature.md", "# Feature")

	if err := commands.InitCommand("Test goal", "specs/feature.md", nil); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	statePath := filepath.Join(projectDir, ".liza", "state.yaml")
	logPath := filepath.Join(projectDir, ".liza", "log.yaml")
	bb := db.New(statePath)

	for _, taskID := range taskIDs {
		taskInput := &commands.TaskInput{
			ID:          taskID,
			RolePair:    "coding-pair",
			Description: "Test feature " + taskID,
			DoneWhen:    "Done",
			Scope:       "Feature",
			Priority:    1,
			SpecRef:     "specs/feature.md",
			DependsOn:   []string{},
		}
		if err := commands.AddTaskCommand(statePath, logPath, taskInput, "orchestrator-1"); err != nil {
			t.Fatalf("AddTask %s failed: %v", taskID, err)
		}
	}

	return bb, statePath, logPath
}

// TestSimpleWorkflow tests: init -> add-task -> claim -> submit-for-review -> approve -> merge
func TestSimpleWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	projectDir, cleanup := setupTestProject(t)
	defer cleanup()

	// Steps 1-2: Initialize liza and add task
	t.Log("Steps 1-2: Initialize liza and add task")
	taskID := "task-1"
	bb, _, _ := setupIntegrationTest(t, projectDir, []string{taskID})

	// Verify initialization
	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)
	if state.Goal.Description != "Test goal" {
		t.Errorf("Expected goal description 'Test goal', got %s", state.Goal.Description)
	}

	// Verify task was added
	if len(state.Tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(state.Tasks))
	}
	if state.Tasks[0].Status != models.TaskStatusReady {
		t.Errorf("Expected task status READY, got %s", state.Tasks[0].Status)
	}

	// Step 3: Register an agent
	t.Log("Step 3: Register agent")
	agentID := "coder-1"
	testhelpers.RegisterTestAgent(t, bb, agentID, "coder")

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
	if task.Status != models.TaskStatusImplementing {
		t.Errorf("Expected task status IMPLEMENTING, got %s", task.Status)
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
	reviewerID := "code-reviewer-1"
	testhelpers.RegisterTestAgent(t, bb, reviewerID, "code-reviewer")

	// Transition to REVIEWING (simulates supervisor reviewer claim)
	testhelpers.TransitionToReviewing(t, bb, taskID, reviewerID)

	if err := commands.SubmitVerdictCommand(projectDir, taskID, "APPROVED", "", reviewerID, ""); err != nil {
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
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	projectDir, cleanup := setupTestProject(t)
	defer cleanup()

	// Setup: init + add task-1 (no deps)
	bb, statePath, logPath := setupIntegrationTest(t, projectDir, []string{"task-1"})

	// Add task-2 (depends on task-1) — custom deps, can't use helper
	task2Input := &commands.TaskInput{
		ID:          "task-2",
		RolePair:    "coding-pair",
		Description: "Dependent feature",
		DoneWhen:    "Done",
		Scope:       "Dependent",
		Priority:    1,
		SpecRef:     "specs/feature.md",
		DependsOn:   []string{"task-1"},
	}
	if err := commands.AddTaskCommand(statePath, logPath, task2Input, "orchestrator-1"); err != nil {
		t.Fatalf("AddTask task-2 failed: %v", err)
	}

	// Register agent
	agentID := "coder-1"
	testhelpers.RegisterTestAgent(t, bb, agentID, "coder")

	// Try to claim task-2 (should fail because task-1 not merged)
	t.Log("Attempting to claim task-2 before task-1 is done")
	err := commands.ClaimTaskCommand(projectDir, "task-2", agentID)
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
	if task.Status != models.TaskStatusImplementing {
		t.Errorf("Expected task-2 status IMPLEMENTING, got %s", task.Status)
	}

	t.Log("✓ Task dependency workflow test passed")
}

// TestIntegrationPipelineWithFindings tests the integration sub-pipeline when
// the analyst produces findings: init -> add integration task -> analyst claims ->
// sets output -> submits -> reviewer approves -> auto-transition creates fix tasks.
func TestIntegrationPipelineWithFindings(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	projectDir, cleanup := setupTestProject(t)
	defer cleanup()

	// Step 1: Initialize project
	t.Log("Step 1: Initialize project")
	testhelpers.CreateSpecFile(t, projectDir, "feature.md", "# Feature")

	if err := commands.InitCommand("Integration test goal", "specs/feature.md", nil); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	statePath := filepath.Join(projectDir, ".liza", "state.yaml")
	logPath := filepath.Join(projectDir, ".liza", "log.yaml")
	bb := db.New(statePath)

	// Step 2: Add an integration-pair task
	t.Log("Step 2: Add integration-pair task")
	taskID := "integ-1"
	taskInput := &commands.TaskInput{
		ID:          taskID,
		Type:        "integration",
		RolePair:    "integration-pair",
		Description: "Integration analysis for branch",
		DoneWhen:    "All integration issues identified",
		Scope:       "Full branch diff",
		Priority:    1,
		SpecRef:     "specs/feature.md",
		DependsOn:   []string{},
	}
	if err := commands.AddTaskCommand(statePath, logPath, taskInput, "orchestrator-1"); err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}

	// Verify task was added with correct type and role-pair
	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)
	task := findTask(state.Tasks, taskID)
	if task == nil {
		t.Fatal("Integration task not found")
	}
	if task.RolePair != "integration-pair" {
		t.Errorf("Expected role_pair integration-pair, got %s", task.RolePair)
	}
	if task.Type != models.TaskTypeIntegration {
		t.Errorf("Expected type integration, got %s", task.Type)
	}

	// Step 3: Register integration-analyst and claim
	t.Log("Step 3: Register integration-analyst and claim")
	analystID := "integration-analyst-1"
	testhelpers.RegisterTestAgent(t, bb, analystID, "integration-analyst")

	if err := commands.ClaimTaskCommand(projectDir, taskID, analystID); err != nil {
		t.Fatalf("ClaimTask failed: %v", err)
	}

	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.Status != "ANALYZING_INTEGRATION" {
		t.Errorf("Expected ANALYZING_INTEGRATION, got %s", task.Status)
	}
	if task.Worktree == nil {
		t.Fatal("Expected worktree to be set after claim")
	}

	// Step 4: Set output (findings) and submit for review
	t.Log("Step 4: Set output and submit for review")
	if err := ops.SetTaskOutput(projectDir, &ops.SetTaskOutputInput{
		TaskID:  taskID,
		AgentID: analystID,
		Output: []models.OutputEntry{
			{
				Desc:     "Fix type alignment in handler.go",
				DoneWhen: "Types match across handler and service layer",
				Scope:    "internal/handler.go",
				SpecRef:  "specs/feature.md",
			},
			{
				Desc:     "Add missing serialization tag",
				DoneWhen: "JSON tag present on Response.Status field",
				Scope:    "internal/models/response.go",
				SpecRef:  "specs/feature.md",
			},
		},
	}); err != nil {
		t.Fatalf("SetTaskOutput failed: %v", err)
	}

	// Write checkpoint (required before submission).
	// Integration analyst doesn't modify code files but checkpoint requires at least one entry.
	// Checkpoint doesn't validate file existence — it records intent, not actuality.
	// TODO: WriteCheckpoint's files_to_modify requirement assumes coding tasks;
	// integration/planning tasks that produce findings rather than code changes
	// shouldn't need this field. Consider relaxing for non-coding task types.
	if err := ops.WriteCheckpoint(projectDir, &ops.WriteCheckpointInput{
		TaskID: taskID, AgentID: analystID,
		Intent: "Integration analysis", ValidationPlan: "Review findings",
		FilesToModify: []string{"integration-analysis.md"},
	}); err != nil {
		t.Fatalf("WriteCheckpoint failed: %v", err)
	}

	// Get worktree HEAD for submit (analyst doesn't commit new code)
	worktreePath := filepath.Join(projectDir, *task.Worktree)
	output, err := exec.Command("git", "-C", worktreePath, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("Failed to get commit hash: %v", err)
	}
	reviewCommit := string(output[:len(output)-1]) // trim newline

	if err := commands.SubmitForReviewCommand(projectDir, taskID, reviewCommit, analystID); err != nil {
		t.Fatalf("SubmitForReview failed: %v", err)
	}

	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.Status != "INTEGRATION_ANALYSIS_TO_REVIEW" {
		t.Errorf("Expected INTEGRATION_ANALYSIS_TO_REVIEW, got %s", task.Status)
	}

	// Step 5: Register integration-reviewer and approve
	t.Log("Step 5: Register integration-reviewer and approve")
	reviewerID := "integration-reviewer-1"
	testhelpers.RegisterTestAgent(t, bb, reviewerID, "integration-reviewer")

	// Transition to REVIEWING_INTEGRATION_ANALYSIS
	leaseExpires := state.Agents[reviewerID].LeaseExpires
	err = bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return nil
		}
		task.Status = "REVIEWING_INTEGRATION_ANALYSIS"
		task.ReviewingBy = &reviewerID
		task.ReviewLeaseExpires = leaseExpires

		if agent, ok := state.Agents[reviewerID]; ok {
			agent.Status = models.AgentStatusWorking
			agent.CurrentTask = &taskID
			state.Agents[reviewerID] = agent
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	if err := commands.SubmitVerdictCommand(projectDir, taskID, "APPROVED", "", reviewerID, ""); err != nil {
		t.Fatalf("SubmitVerdict failed: %v", err)
	}

	// Verify task is approved (not clean — has output)
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.Status != "INTEGRATION_ANALYSIS_APPROVED" {
		t.Errorf("Expected INTEGRATION_ANALYSIS_APPROVED, got %s", task.Status)
	}

	// Step 6: Execute auto-transitions — should create coding-pair fix tasks
	t.Log("Step 6: Execute auto-transitions")
	results, err := ops.ExecuteAvailableTransitions(projectDir, "auto")
	if err != nil {
		t.Fatalf("ExecuteAvailableTransitions failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 transition result, got %d", len(results))
	}
	result := results[0]
	if result.SourceTaskID != taskID {
		t.Errorf("Expected source task %s, got %s", taskID, result.SourceTaskID)
	}
	if result.TransitionName != "integration-to-fix" {
		t.Errorf("Expected transition integration-to-fix, got %s", result.TransitionName)
	}
	if len(result.ChildTaskIDs) != 2 {
		t.Fatalf("Expected 2 child tasks (one per finding), got %d", len(result.ChildTaskIDs))
	}

	// Verify child tasks are coding-pair tasks in DRAFT_CODE state
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	for _, childID := range result.ChildTaskIDs {
		child := findTask(state.Tasks, childID)
		if child == nil {
			t.Fatalf("Child task %s not found", childID)
		}
		if child.RolePair != "coding-pair" {
			t.Errorf("Child %s: expected role_pair coding-pair, got %s", childID, child.RolePair)
		}
		if child.Status != "DRAFT_CODE" {
			t.Errorf("Child %s: expected DRAFT_CODE, got %s", childID, child.Status)
		}
		if !slices.Contains(child.ParentTasks, taskID) {
			t.Errorf("Child %s: expected parent_tasks to contain %s, got %v", childID, taskID, child.ParentTasks)
		}
	}

	// Verify Goal.BaseCommit was snapshotted when coding-pair children were created.
	// This is the diff base the integration analyst uses (goal.base_commit..HEAD).
	if state.Goal.BaseCommit == nil {
		t.Error("Expected Goal.BaseCommit to be set after coding-pair children created")
	}
}

// TestIntegrationPipelineCleanScan tests the integration sub-pipeline when
// the analyst finds no issues: analyst submits without output -> reviewer
// approves -> routes to INTEGRATION_ANALYSIS_CLEAN terminal state.
func TestIntegrationPipelineCleanScan(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	projectDir, cleanup := setupTestProject(t)
	defer cleanup()

	// Setup: init + spec + integration task
	testhelpers.CreateSpecFile(t, projectDir, "feature.md", "# Feature")

	if err := commands.InitCommand("Clean scan goal", "specs/feature.md", nil); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	statePath := filepath.Join(projectDir, ".liza", "state.yaml")
	logPath := filepath.Join(projectDir, ".liza", "log.yaml")
	bb := db.New(statePath)

	taskID := "integ-clean-1"
	taskInput := &commands.TaskInput{
		ID:          taskID,
		Type:        "integration",
		RolePair:    "integration-pair",
		Description: "Integration analysis — expecting clean",
		DoneWhen:    "All integration issues identified",
		Scope:       "Full branch diff",
		Priority:    1,
		SpecRef:     "specs/feature.md",
		DependsOn:   []string{},
	}
	if err := commands.AddTaskCommand(statePath, logPath, taskInput, "orchestrator-1"); err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}

	// Claim with integration-analyst
	analystID := "integration-analyst-1"
	testhelpers.RegisterTestAgent(t, bb, analystID, "integration-analyst")

	if err := commands.ClaimTaskCommand(projectDir, taskID, analystID); err != nil {
		t.Fatalf("ClaimTask failed: %v", err)
	}

	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)
	task := findTask(state.Tasks, taskID)
	if task.Worktree == nil {
		t.Fatal("Expected worktree after claim")
	}

	// Submit without setting output (clean scan — no findings).
	// See TODO in TestIntegrationPipelineWithFindings re: files_to_modify for non-coding tasks.
	if err := ops.WriteCheckpoint(projectDir, &ops.WriteCheckpointInput{
		TaskID: taskID, AgentID: analystID,
		Intent: "Integration analysis", ValidationPlan: "Review findings",
		FilesToModify: []string{"integration-analysis.md"},
	}); err != nil {
		t.Fatalf("WriteCheckpoint failed: %v", err)
	}

	worktreePath := filepath.Join(projectDir, *task.Worktree)
	output, err := exec.Command("git", "-C", worktreePath, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("Failed to get commit hash: %v", err)
	}
	reviewCommit := string(output[:len(output)-1])

	if err := commands.SubmitForReviewCommand(projectDir, taskID, reviewCommit, analystID); err != nil {
		t.Fatalf("SubmitForReview failed: %v", err)
	}

	// Reviewer approves — should route to CLEAN (empty output)
	reviewerID := "integration-reviewer-1"
	testhelpers.RegisterTestAgent(t, bb, reviewerID, "integration-reviewer")

	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	leaseExpires := state.Agents[reviewerID].LeaseExpires
	err = bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return nil
		}
		task.Status = "REVIEWING_INTEGRATION_ANALYSIS"
		task.ReviewingBy = &reviewerID
		task.ReviewLeaseExpires = leaseExpires

		if agent, ok := state.Agents[reviewerID]; ok {
			agent.Status = models.AgentStatusWorking
			agent.CurrentTask = &taskID
			state.Agents[reviewerID] = agent
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	if err := commands.SubmitVerdictCommand(projectDir, taskID, "APPROVED", "", reviewerID, ""); err != nil {
		t.Fatalf("SubmitVerdict failed: %v", err)
	}

	// Verify task routed to CLEAN terminal state
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.Status != "INTEGRATION_ANALYSIS_CLEAN" {
		t.Errorf("Expected INTEGRATION_ANALYSIS_CLEAN, got %s", task.Status)
	}

	// Auto-transitions should produce nothing (clean is terminal)
	results, err := ops.ExecuteAvailableTransitions(projectDir, "auto")
	if err != nil {
		t.Fatalf("ExecuteAvailableTransitions failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 transition results for clean scan, got %d", len(results))
	}
}

// TestArchitecturePairWorkflow tests the architecture-pair lifecycle end-to-end:
// add task → claim → set output → submit → review → reject → reclaim →
// resubmit → approve → proceed (architecture-to-code-plan transition).
// Verifies the architecture task visits all 6 declared states and that
// the per-subtask transition creates correct code-planning child tasks.
func TestArchitecturePairWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	projectDir, cleanup := setupTestProject(t)
	defer cleanup()

	// Step 1: Initialize project
	t.Log("Step 1: Initialize project")
	testhelpers.CreateSpecFile(t, projectDir, "arch-feature.md", "# Architecture Feature")

	if err := commands.InitCommand("Architecture test goal", "specs/arch-feature.md", nil); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	statePath := filepath.Join(projectDir, ".liza", "state.yaml")
	logPath := filepath.Join(projectDir, ".liza", "log.yaml")
	bb := db.New(statePath)

	// Step 2: Add architecture task
	t.Log("Step 2: Add architecture task")
	taskID := "arch-1"
	taskInput := &commands.TaskInput{
		ID:          taskID,
		Type:        "architecture",
		RolePair:    "architecture-pair",
		Description: "Architecture for feature",
		DoneWhen:    "Architecture defined",
		Scope:       "Full feature",
		Priority:    1,
		SpecRef:     "specs/arch-feature.md",
		DependsOn:   []string{},
	}
	if err := commands.AddTaskCommand(statePath, logPath, taskInput, "orchestrator-1"); err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}

	// Verify: state 1 — DRAFT_ARCHITECTURE
	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)
	task := findTask(state.Tasks, taskID)
	if task == nil {
		t.Fatal("Architecture task not found")
	}
	if task.Status != "DRAFT_ARCHITECTURE" {
		t.Errorf("Expected DRAFT_ARCHITECTURE, got %s", task.Status)
	}
	if task.RolePair != "architecture-pair" {
		t.Errorf("Expected role_pair architecture-pair, got %s", task.RolePair)
	}
	if task.Type != models.TaskTypeArchitecture {
		t.Errorf("Expected type architecture, got %s", task.Type)
	}

	// Step 3: Register architect and claim
	t.Log("Step 3: Register architect and claim")
	architectID := "architect-1"
	testhelpers.RegisterTestAgent(t, bb, architectID, "architect")

	if err := commands.ClaimTaskCommand(projectDir, taskID, architectID); err != nil {
		t.Fatalf("ClaimTask failed: %v", err)
	}

	// Verify: state 2 — ARCHITECTING
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.Status != "ARCHITECTING" {
		t.Errorf("Expected ARCHITECTING, got %s", task.Status)
	}
	if task.Worktree == nil {
		t.Fatal("Expected worktree to be set after claim")
	}

	// Step 4: Set output with 2 code-planning entries
	t.Log("Step 4: Set output entries")
	if err := ops.SetTaskOutput(projectDir, &ops.SetTaskOutputInput{
		TaskID:  taskID,
		AgentID: architectID,
		Output: []models.OutputEntry{
			{
				Desc:     "Implement auth module",
				DoneWhen: "Auth module complete",
				Scope:    "internal/auth/",
				SpecRef:  "specs/arch-feature.md",
			},
			{
				Desc:     "Implement storage layer",
				DoneWhen: "Storage layer complete",
				Scope:    "internal/storage/",
				SpecRef:  "specs/arch-feature.md",
			},
		},
	}); err != nil {
		t.Fatalf("SetTaskOutput failed: %v", err)
	}

	// Step 5: Write checkpoint, make commit, submit for review
	t.Log("Step 5: Submit for review")
	if err := ops.WriteCheckpoint(projectDir, &ops.WriteCheckpointInput{
		TaskID: taskID, AgentID: architectID,
		Intent: "Architecture definition", ValidationPlan: "Review architecture",
		FilesToModify: []string{"specs/arch-plan/feature.md"},
	}); err != nil {
		t.Fatalf("WriteCheckpoint failed: %v", err)
	}

	worktreePath := filepath.Join(projectDir, *task.Worktree)
	archDoc := filepath.Join(worktreePath, "arch-plan.md")
	if err := os.WriteFile(archDoc, []byte("# Architecture Plan\n"), 0644); err != nil {
		t.Fatalf("Failed to create arch doc: %v", err)
	}
	if err := exec.Command("git", "-C", worktreePath, "add", "arch-plan.md").Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "-C", worktreePath, "commit", "-m", "Define architecture").Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

	output, err := exec.Command("git", "-C", worktreePath, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("Failed to get commit hash: %v", err)
	}
	reviewCommit := string(output[:len(output)-1])

	if err := commands.SubmitForReviewCommand(projectDir, taskID, reviewCommit, architectID); err != nil {
		t.Fatalf("SubmitForReview failed: %v", err)
	}

	// Verify: state 3 — ARCHITECTURE_TO_REVIEW
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.Status != "ARCHITECTURE_TO_REVIEW" {
		t.Errorf("Expected ARCHITECTURE_TO_REVIEW, got %s", task.Status)
	}

	// Step 6: Register architecture-reviewer and transition to reviewing
	t.Log("Step 6: Reviewer claims for review")
	reviewerID := "architecture-reviewer-1"
	testhelpers.RegisterTestAgent(t, bb, reviewerID, "architecture-reviewer")

	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	leaseExpires := state.Agents[reviewerID].LeaseExpires
	err = bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return nil
		}
		task.Status = "REVIEWING_ARCHITECTURE"
		task.ReviewingBy = &reviewerID
		task.ReviewLeaseExpires = leaseExpires

		if agent, ok := state.Agents[reviewerID]; ok {
			agent.Status = models.AgentStatusWorking
			agent.CurrentTask = &taskID
			state.Agents[reviewerID] = agent
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Verify: state 4 — REVIEWING_ARCHITECTURE
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.Status != "REVIEWING_ARCHITECTURE" {
		t.Errorf("Expected REVIEWING_ARCHITECTURE, got %s", task.Status)
	}

	// Step 7: Reject — covers state 5 (ARCHITECTURE_REJECTED)
	t.Log("Step 7: Reject architecture")
	if err := commands.SubmitVerdictCommand(projectDir, taskID, "REJECTED", "Needs more detail on interfaces", reviewerID, ""); err != nil {
		t.Fatalf("SubmitVerdict (REJECTED) failed: %v", err)
	}

	// Verify: state 5 — ARCHITECTURE_REJECTED
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.Status != "ARCHITECTURE_REJECTED" {
		t.Errorf("Expected ARCHITECTURE_REJECTED, got %s", task.Status)
	}

	// Step 8: Re-claim after rejection
	t.Log("Step 8: Re-claim after rejection")
	if err := commands.ClaimTaskCommand(projectDir, taskID, architectID); err != nil {
		t.Fatalf("Re-claim after rejection failed: %v", err)
	}

	// Verify: back to ARCHITECTING
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.Status != "ARCHITECTING" {
		t.Errorf("Expected ARCHITECTING after re-claim, got %s", task.Status)
	}

	// Step 9: Write new checkpoint and re-submit
	t.Log("Step 9: Re-submit for review")
	if err := ops.WriteCheckpoint(projectDir, &ops.WriteCheckpointInput{
		TaskID: taskID, AgentID: architectID,
		Intent: "Revised architecture with interface details", ValidationPlan: "Review architecture",
		FilesToModify: []string{"specs/arch-plan/feature.md"},
	}); err != nil {
		t.Fatalf("WriteCheckpoint (round 2) failed: %v", err)
	}

	// Make another commit in the worktree
	revisedDoc := filepath.Join(worktreePath, "arch-plan.md")
	if err := os.WriteFile(revisedDoc, []byte("# Architecture Plan\n\n## Interfaces\n"), 0644); err != nil {
		t.Fatalf("Failed to update arch doc: %v", err)
	}
	if err := exec.Command("git", "-C", worktreePath, "add", "arch-plan.md").Run(); err != nil {
		t.Fatalf("Failed to git add (round 2): %v", err)
	}
	if err := exec.Command("git", "-C", worktreePath, "commit", "-m", "Add interface details").Run(); err != nil {
		t.Fatalf("Failed to git commit (round 2): %v", err)
	}

	output, err = exec.Command("git", "-C", worktreePath, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("Failed to get commit hash (round 2): %v", err)
	}
	reviewCommit = string(output[:len(output)-1])

	if err := commands.SubmitForReviewCommand(projectDir, taskID, reviewCommit, architectID); err != nil {
		t.Fatalf("SubmitForReview (round 2) failed: %v", err)
	}

	// Verify: ARCHITECTURE_TO_REVIEW again
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.Status != "ARCHITECTURE_TO_REVIEW" {
		t.Errorf("Expected ARCHITECTURE_TO_REVIEW (round 2), got %s", task.Status)
	}

	// Step 10: Reviewer approves this time
	t.Log("Step 10: Approve architecture")
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	leaseExpires = state.Agents[reviewerID].LeaseExpires
	err = bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return nil
		}
		task.Status = "REVIEWING_ARCHITECTURE"
		task.ReviewingBy = &reviewerID
		task.ReviewLeaseExpires = leaseExpires

		if agent, ok := state.Agents[reviewerID]; ok {
			agent.Status = models.AgentStatusWorking
			agent.CurrentTask = &taskID
			state.Agents[reviewerID] = agent
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	if err := commands.SubmitVerdictCommand(projectDir, taskID, "APPROVED", "", reviewerID, ""); err != nil {
		t.Fatalf("SubmitVerdict (APPROVED) failed: %v", err)
	}

	// Verify: state 6 — ARCHITECTURE_APPROVED
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.Status != "ARCHITECTURE_APPROVED" {
		t.Errorf("Expected ARCHITECTURE_APPROVED, got %s", task.Status)
	}

	// Step 11: Merge the task so Proceed can execute
	t.Log("Step 11: Merge architecture task")
	os.Setenv("LIZA_AGENT_ID", reviewerID)
	defer os.Unsetenv("LIZA_AGENT_ID")

	if err := commands.WtMergeCommand(projectDir, taskID, reviewerID); err != nil {
		t.Fatalf("WtMerge failed: %v", err)
	}

	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.Status != models.TaskStatusMerged {
		t.Errorf("Expected MERGED after merge, got %s", task.Status)
	}

	// Step 12: Execute architecture-to-code-plan transition via ExecuteAvailableTransitions.
	// architecture-to-code-plan is a manual transition; ExecuteAvailableTransitions handles
	// MERGED tasks by overriding requiredStatus (unlike Proceed which expects the from-phase status).
	t.Log("Step 12: Execute architecture-to-code-plan transition")
	results, err := ops.ExecuteAvailableTransitions(projectDir, "manual")
	if err != nil {
		t.Fatalf("ExecuteAvailableTransitions failed: %v", err)
	}

	// Step 13: Assert child tasks
	t.Log("Step 13: Verify child tasks")
	if len(results) != 1 {
		t.Fatalf("Expected 1 transition result, got %d", len(results))
	}
	result := results[0]
	if result.SourceTaskID != taskID {
		t.Errorf("Expected source task %s, got %s", taskID, result.SourceTaskID)
	}
	if result.TransitionName != "architecture-to-code-plan" {
		t.Errorf("Expected transition name architecture-to-code-plan, got %s", result.TransitionName)
	}
	if len(result.ChildTaskIDs) != 2 {
		t.Fatalf("Expected 2 child tasks, got %d", len(result.ChildTaskIDs))
	}

	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)

	// Verify transitions_executed on the architecture task
	task = findTask(state.Tasks, taskID)
	if !task.TransitionsExecuted["architecture-to-code-plan"] {
		t.Error("Expected transitions_executed to contain architecture-to-code-plan")
	}

	// Verify each child task
	for _, childID := range result.ChildTaskIDs {
		child := findTask(state.Tasks, childID)
		if child == nil {
			t.Fatalf("Child task %s not found", childID)
		}
		if child.RolePair != "code-planning-pair" {
			t.Errorf("Child %s: expected role_pair code-planning-pair, got %s", childID, child.RolePair)
		}
		if child.Status != "DRAFT_CODING_PLAN" {
			t.Errorf("Child %s: expected DRAFT_CODING_PLAN, got %s", childID, child.Status)
		}
		if !slices.Contains(child.ParentTasks, taskID) {
			t.Errorf("Child %s: expected ParentTasks to contain %s, got %v", childID, taskID, child.ParentTasks)
		}
		if child.SpecRef != "specs/arch-feature.md" {
			t.Errorf("Child %s: expected spec_ref specs/arch-feature.md, got %s", childID, child.SpecRef)
		}
	}

	t.Log("TestArchitecturePairWorkflow passed — all 6 states visited, child tasks verified")
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
