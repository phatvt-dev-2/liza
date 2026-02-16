package integration

// concurrent_operations_test.go contains integration tests for concurrent operations.
//
// These tests verify that the blackboard locking mechanism correctly handles
// race conditions when multiple agents operate simultaneously.

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// TestConcurrentClaimAttempts tests that only one agent can claim a task
// when multiple agents try to claim simultaneously.
func TestConcurrentClaimAttempts(t *testing.T) {
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

	// Add a single task
	taskID := "task-1"
	taskInput := &commands.TaskInput{
		ID:          taskID,
		Description: "Test feature",
		DoneWhen:    "Done",
		Scope:       "Feature",
		Priority:    1,
		SpecRef:     "specs/feature.md",
		DependsOn:   []string{},
	}
	if err := commands.AddTaskCommand(statePath, logPath, taskInput, "planner-1"); err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}

	// Register multiple agents
	now := time.Now().UTC()
	numAgents := 5
	agentIDs := make([]string, numAgents)
	agentLeaseExpires := now.Add(30 * time.Minute)

	err := bb.Modify(func(state *models.State) error {
		for i := 0; i < numAgents; i++ {
			agentID := "coder-" + string(rune('a'+i))
			agentIDs[i] = agentID
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
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Have all agents try to claim the same task concurrently
	var wg sync.WaitGroup
	results := make([]error, numAgents)

	t.Log("Starting concurrent claim attempts")
	for i := 0; i < numAgents; i++ {
		wg.Add(1)
		go func(agentIndex int) {
			defer wg.Done()
			agentID := agentIDs[agentIndex]
			results[agentIndex] = commands.ClaimTaskCommand(projectDir, taskID, agentID)
		}(i)
	}

	wg.Wait()

	// Verify that exactly one claim succeeded
	successCount := 0
	var successfulAgent string
	for i, err := range results {
		if err == nil {
			successCount++
			successfulAgent = agentIDs[i]
			t.Logf("Agent %s successfully claimed task", agentIDs[i])
		} else {
			t.Logf("Agent %s failed to claim: %v", agentIDs[i], err)
		}
	}

	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful claim, got %d", successCount)
	}

	// Verify final state
	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)

	task := findTask(state.Tasks, taskID)
	if task == nil {
		t.Fatal("Task not found")
	}

	if task.Status != models.TaskStatusClaimed {
		t.Errorf("Expected task status CLAIMED, got %s", task.Status)
	}

	if task.AssignedTo == nil {
		t.Error("Expected task to be assigned")
	} else if *task.AssignedTo != successfulAgent {
		t.Errorf("Expected task assigned to %s, got %s", successfulAgent, *task.AssignedTo)
	}

	t.Log("✓ Concurrent claim test passed")
}

// TestConcurrentClaimsOfDifferentTasks tests that multiple agents can
// claim different tasks simultaneously without interference.
func TestConcurrentClaimsOfDifferentTasks(t *testing.T) {
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

	// Add multiple tasks
	numTasks := 5
	taskIDs := make([]string, numTasks)
	for i := 0; i < numTasks; i++ {
		taskID := "task-" + string(rune('a'+i))
		taskIDs[i] = taskID
		taskInput := &commands.TaskInput{
			ID:          taskID,
			Description: "Test feature " + taskID,
			DoneWhen:    "Done",
			Scope:       "Feature",
			Priority:    1,
			SpecRef:     "specs/feature.md",
			DependsOn:   []string{},
		}
		if err := commands.AddTaskCommand(statePath, logPath, taskInput, "planner-1"); err != nil {
			t.Fatalf("AddTask %s failed: %v", taskID, err)
		}
	}

	// Register multiple agents (one per task)
	now := time.Now().UTC()
	agentIDs := make([]string, numTasks)
	agentLeaseExpires := now.Add(30 * time.Minute)

	err := bb.Modify(func(state *models.State) error {
		for i := 0; i < numTasks; i++ {
			agentID := "coder-" + string(rune('a'+i))
			agentIDs[i] = agentID
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
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Have each agent claim a different task concurrently
	var wg sync.WaitGroup
	results := make([]error, numTasks)

	t.Log("Starting concurrent claims of different tasks")
	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index] = commands.ClaimTaskCommand(projectDir, taskIDs[index], agentIDs[index])
		}(i)
	}

	wg.Wait()

	// Verify that all claims succeeded
	for i, err := range results {
		if err != nil {
			t.Errorf("Agent %s failed to claim task %s: %v", agentIDs[i], taskIDs[i], err)
		}
	}

	// Verify final state - all tasks should be claimed by their respective agents
	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)

	for i := 0; i < numTasks; i++ {
		task := findTask(state.Tasks, taskIDs[i])
		if task == nil {
			t.Errorf("Task %s not found", taskIDs[i])
			continue
		}

		if task.Status != models.TaskStatusClaimed {
			t.Errorf("Expected task %s status CLAIMED, got %s", taskIDs[i], task.Status)
		}

		if task.AssignedTo == nil {
			t.Errorf("Expected task %s to be assigned", taskIDs[i])
		} else if *task.AssignedTo != agentIDs[i] {
			t.Errorf("Expected task %s assigned to %s, got %s", taskIDs[i], agentIDs[i], *task.AssignedTo)
		}
	}

	t.Log("✓ Concurrent claims of different tasks test passed")
}

// TestConcurrentStateModifications tests that concurrent modifications to state
// are properly serialized and no updates are lost.
func TestConcurrentStateModifications(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup minimal state
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	bb := testhelpers.WriteInitialState(t, statePath, state)

	// Perform concurrent modifications that increment a counter
	numGoroutines := 10
	modificationsPerGoroutine := 5

	var wg sync.WaitGroup
	errors := make([]error, numGoroutines*modificationsPerGoroutine)
	errorIndex := 0
	var errorMutex sync.Mutex

	t.Log("Starting concurrent state modifications")
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < modificationsPerGoroutine; j++ {
				err := bb.Modify(func(state *models.State) error {
					// Increment a counter in config (using HeartbeatInterval as test counter)
					state.Config.HeartbeatInterval++
					return nil
				})

				errorMutex.Lock()
				errors[errorIndex] = err
				errorIndex++
				errorMutex.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Verify no errors occurred
	for i, err := range errors {
		if err != nil {
			t.Errorf("Modification %d failed: %v", i, err)
		}
	}

	// Verify final counter value
	finalState, err := bb.Read()
	testhelpers.AssertNoError(t, err)

	expectedCount := 60 + (numGoroutines * modificationsPerGoroutine) // 60 is initial value from CreateValidState
	if finalState.Config.HeartbeatInterval != expectedCount {
		t.Errorf("Expected counter to be %d, got %d (some updates were lost)", expectedCount, finalState.Config.HeartbeatInterval)
	}

	t.Log("✓ Concurrent state modifications test passed")
}

// TestConcurrentReadsDuringWrite tests that reads are blocked during writes
// (verifying lock behavior).
func TestConcurrentReadsDuringWrite(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	bb := testhelpers.WriteInitialState(t, statePath, state)

	// Start a long-running write
	var wg sync.WaitGroup
	writeStarted := make(chan bool)
	writeCompleted := make(chan bool)

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := bb.Modify(func(state *models.State) error {
			close(writeStarted)
			time.Sleep(100 * time.Millisecond) // Simulate slow operation
			state.Config.HeartbeatInterval = 999
			return nil
		})
		if err != nil {
			t.Errorf("Write failed: %v", err)
		}
		close(writeCompleted)
	}()

	// Wait for write to start
	<-writeStarted

	// Try to read during the write - should be blocked until write completes
	readStart := time.Now()
	_, err := bb.Read()
	readDuration := time.Since(readStart)

	testhelpers.AssertNoError(t, err)

	// Verify read was blocked (took at least as long as the write sleep)
	if readDuration < 50*time.Millisecond {
		t.Error("Read completed too quickly, may not have been properly blocked by write lock")
	}

	// Wait for write to complete
	<-writeCompleted
	wg.Wait()

	// Verify the write succeeded
	finalState, err := bb.Read()
	testhelpers.AssertNoError(t, err)
	if finalState.Config.HeartbeatInterval != 999 {
		t.Errorf("Expected HeartbeatInterval to be 999, got %d", finalState.Config.HeartbeatInterval)
	}

	t.Log("✓ Concurrent reads during write test passed")
}

// TestConcurrentClaimWithWorktreeConflict tests the scenario where multiple
// coders try to claim the same task and create worktrees simultaneously,
// ensuring only one succeeds and invalid state is never created.
func TestConcurrentClaimWithWorktreeConflict(t *testing.T) {
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

	// Add a single task
	taskID := "test-task"
	taskInput := &commands.TaskInput{
		ID:          taskID,
		Description: "Test task",
		DoneWhen:    "Tests pass",
		Scope:       "Test scope",
		Priority:    1,
		SpecRef:     "specs/feature.md",
		DependsOn:   []string{},
	}
	if err := commands.AddTaskCommand(statePath, logPath, taskInput, "planner-1"); err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}

	// Register multiple agents
	now := time.Now().UTC()
	numCoders := 5
	agentIDs := make([]string, numCoders)
	agentLeaseExpires := now.Add(30 * time.Minute)

	err := bb.Modify(func(state *models.State) error {
		for i := 0; i < numCoders; i++ {
			agentID := "coder-" + string(rune('1'+i))
			agentIDs[i] = agentID
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
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Launch concurrent claim attempts
	var wg sync.WaitGroup
	results := make([]error, numCoders)

	t.Log("Starting concurrent claim attempts with worktree creation")
	for i := 0; i < numCoders; i++ {
		wg.Add(1)
		go func(coderNum int) {
			defer wg.Done()
			agentID := agentIDs[coderNum]
			// All coders try to claim the same task
			results[coderNum] = commands.ClaimTaskCommand(projectDir, taskID, agentID)
		}(i)
	}

	wg.Wait()

	// Verify results
	successCount := 0
	failureCount := 0
	var successfulAgent string

	for i, err := range results {
		if err == nil {
			successCount++
			successfulAgent = agentIDs[i]
			t.Logf("Agent %s successfully claimed task", agentIDs[i])
		} else {
			failureCount++
			t.Logf("Agent %s failed to claim: %v", agentIDs[i], err)
			// Should fail with one of the expected race condition errors
			errMsg := err.Error()
			if !strings.Contains(errMsg, "race condition") &&
				!strings.Contains(errMsg, "worktree") &&
				!strings.Contains(errMsg, "branch") &&
				!strings.Contains(errMsg, "is CLAIMED") {
				t.Errorf("Agent %s: unexpected error type: %v", agentIDs[i], err)
			}
		}
	}

	// Exactly one should succeed
	if successCount != 1 {
		t.Errorf("Expected exactly 1 success, got %d", successCount)
	}
	if failureCount != numCoders-1 {
		t.Errorf("Expected %d failures, got %d", numCoders-1, failureCount)
	}

	// CRITICAL: Verify state is valid
	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)

	task := findTask(state.Tasks, taskID)
	if task == nil {
		t.Fatal("Task disappeared from state")
	}

	// Task should be CLAIMED
	if task.Status != models.TaskStatusClaimed {
		t.Errorf("Task status should be CLAIMED, got %s", task.Status)
	}

	// Should be assigned to the successful agent
	if task.AssignedTo == nil {
		t.Fatal("Task marked CLAIMED but AssignedTo is nil")
	}
	if *task.AssignedTo != successfulAgent {
		t.Errorf("Task assigned to %s, expected %s", *task.AssignedTo, successfulAgent)
	}

	// Worktree must be set
	if task.Worktree == nil {
		t.Fatal("Task marked CLAIMED but worktree is nil")
	}

	// CRITICAL CHECK: Worktree directory must exist on disk
	worktreePath := filepath.Join(projectDir, *task.Worktree)
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Fatalf("INVALID STATE: Task marked CLAIMED with worktree=%s but directory does not exist",
			*task.Worktree)
	}

	t.Logf("✓ Worktree exists at %s", worktreePath)
	t.Log("✓ Concurrent claim with worktree conflict test passed")
}

// TestConcurrentClaimIntegrationFailedTask tests that when multiple agents
// attempt to claim the same INTEGRATION_FAILED task concurrently, only one
// succeeds. This validates the fix for the task coordination race condition
// where ReadCached() was allowing multiple agents to select the same task.
func TestConcurrentClaimIntegrationFailedTask(t *testing.T) {
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

	// Add a task and set it to INTEGRATION_FAILED with IntegrationFix=true
	taskID := "task-failed"
	taskInput := &commands.TaskInput{
		ID:          taskID,
		Description: "Test feature with merge conflict",
		DoneWhen:    "Done",
		Scope:       "Feature",
		Priority:    1,
		SpecRef:     "specs/feature.md",
		DependsOn:   []string{},
	}
	if err := commands.AddTaskCommand(statePath, logPath, taskInput, "planner-1"); err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}

	// Set task to INTEGRATION_FAILED with a worktree (simulating failed merge)
	worktreeName := ".worktrees/task-failed"
	integrationFix := true
	err := bb.Modify(func(state *models.State) error {
		for i := range state.Tasks {
			if state.Tasks[i].ID == taskID {
				state.Tasks[i].Status = models.TaskStatusIntegrationFailed
				state.Tasks[i].IntegrationFix = integrationFix
				state.Tasks[i].Worktree = &worktreeName
				// Create dummy worktree directory
				worktreePath := filepath.Join(projectDir, worktreeName)
				if err := os.MkdirAll(worktreePath, 0755); err != nil {
					return err
				}
				break
			}
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Register multiple coder agents
	now := time.Now().UTC()
	numAgents := 3
	agentIDs := make([]string, numAgents)
	agentLeaseExpires := now.Add(30 * time.Minute)

	err = bb.Modify(func(state *models.State) error {
		for i := 0; i < numAgents; i++ {
			agentID := "coder-" + string(rune('1'+i))
			agentIDs[i] = agentID
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
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Have all agents try to claim the same INTEGRATION_FAILED task concurrently
	var wg sync.WaitGroup
	results := make([]error, numAgents)

	t.Log("Starting concurrent claim attempts for INTEGRATION_FAILED task")
	for i := 0; i < numAgents; i++ {
		wg.Add(1)
		go func(agentIndex int) {
			defer wg.Done()
			agentID := agentIDs[agentIndex]
			results[agentIndex] = commands.ClaimTaskCommand(projectDir, taskID, agentID)
		}(i)
	}

	wg.Wait()

	// Verify that exactly one claim succeeded
	successCount := 0
	var successfulAgent string
	for i, err := range results {
		if err == nil {
			successCount++
			successfulAgent = agentIDs[i]
			t.Logf("Agent %s successfully claimed INTEGRATION_FAILED task", agentIDs[i])
		} else {
			// Expected errors: either "race condition", "not claimable", or "is CLAIMED"
			if !strings.Contains(err.Error(), "race condition") &&
				!strings.Contains(err.Error(), "not claimable") &&
				!strings.Contains(err.Error(), "no claimable tasks") &&
				!strings.Contains(err.Error(), "is CLAIMED") {
				t.Logf("Agent %s failed with unexpected error: %v", agentIDs[i], err)
			} else {
				t.Logf("Agent %s failed as expected: %v", agentIDs[i], err)
			}
		}
	}

	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful claim, got %d", successCount)
	}

	// Verify final state
	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)

	task := findTask(state.Tasks, taskID)
	if task == nil {
		t.Fatal("Task not found")
	}

	// Task should be CLAIMED
	if task.Status != models.TaskStatusClaimed {
		t.Errorf("Expected task status CLAIMED, got %s", task.Status)
	}

	// Should be assigned to exactly one agent
	if task.AssignedTo == nil {
		t.Error("Expected task to be assigned")
	} else if *task.AssignedTo != successfulAgent {
		t.Errorf("Expected task assigned to %s, got %s", successfulAgent, *task.AssignedTo)
	}

	// IntegrationFix flag should still be set
	if !task.IntegrationFix {
		t.Error("Expected IntegrationFix=true to be preserved")
	}

	// Worktree should be preserved (not recreated)
	if task.Worktree == nil {
		t.Error("Expected worktree to be preserved")
	} else if *task.Worktree != worktreeName {
		t.Errorf("Expected worktree=%s, got %s", worktreeName, *task.Worktree)
	}

	// Verify worktree still exists
	worktreePath := filepath.Join(projectDir, worktreeName)
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Errorf("Worktree directory should still exist at %s", worktreePath)
	}

	t.Log("✓ Concurrent INTEGRATION_FAILED claim test passed")
}
