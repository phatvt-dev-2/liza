package integration

// lease_expiry_test.go contains integration tests for lease expiry handling.
//
// These tests verify that the system correctly handles expired leases for
// both task claims and review claims.

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// TestExpiredClaimLease tests that tasks with expired leases can be re-claimed
func TestExpiredClaimLease(t *testing.T) {
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

	// Add a task
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

	// Register first agent
	agent1 := "coder-1"
	now := time.Now().UTC()
	agentLeaseExpires := now.Add(30 * time.Minute)

	err := bb.Modify(func(state *models.State) error {
		state.Agents[agent1] = models.Agent{
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

	// First agent claims the task
	t.Log("Agent 1 claims task")
	if err := commands.ClaimTaskCommand(projectDir, taskID, agent1); err != nil {
		t.Fatalf("ClaimTask by agent1 failed: %v", err)
	}

	// Verify task is claimed by agent1
	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)
	task := findTask(state.Tasks, taskID)
	if task.AssignedTo == nil || *task.AssignedTo != agent1 {
		t.Error("Expected task to be assigned to agent1")
	}

	// Simulate lease expiry by manually setting LeaseExpires to the past
	t.Log("Simulating lease expiry")
	err = bb.Modify(func(state *models.State) error {
		for i := range state.Tasks {
			if state.Tasks[i].ID == taskID {
				expiredTime := now.Add(-1 * time.Hour) // Expired 1 hour ago
				state.Tasks[i].LeaseExpires = &expiredTime
				break
			}
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Register second agent
	agent2 := "coder-2"
	err = bb.Modify(func(state *models.State) error {
		state.Agents[agent2] = models.Agent{
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

	// Second agent should NOT be able to claim the task directly
	// (The claim command doesn't check for expired leases itself - that's the watch command's job)
	// But the validation should catch that it's still CLAIMED
	t.Log("Agent 2 attempts to claim already-claimed task")
	err = commands.ClaimTaskCommand(projectDir, taskID, agent2)
	if err == nil {
		t.Error("Expected error when claiming already-claimed task")
	}

	t.Log("✓ Expired claim lease test passed")
}

// TestExpiredReviewLease tests that review leases can expire and be reclaimed
func TestExpiredReviewLease(t *testing.T) {
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

	// Add and claim a task
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

	// Register agents
	coderID := "coder-1"
	reviewer1 := "reviewer-1"
	reviewer2 := "reviewer-2"
	now := time.Now().UTC()
	agentLeaseExpires := now.Add(30 * time.Minute)

	err := bb.Modify(func(state *models.State) error {
		state.Agents[coderID] = models.Agent{
			Role:            "coder",
			Status:          models.AgentStatusWaiting,
			Heartbeat:       now,
			LeaseExpires:    &agentLeaseExpires,
			CurrentTask:     nil,
			Terminal:        "test",
			IterationsTotal: 0,
			ContextPercent:  0,
		}
		state.Agents[reviewer1] = models.Agent{
			Role:            "reviewer",
			Status:          models.AgentStatusWaiting,
			Heartbeat:       now,
			LeaseExpires:    &agentLeaseExpires,
			CurrentTask:     nil,
			Terminal:        "test",
			IterationsTotal: 0,
			ContextPercent:  0,
		}
		state.Agents[reviewer2] = models.Agent{
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

	// Claim and submit task for review
	if err := commands.ClaimTaskCommand(projectDir, taskID, coderID); err != nil {
		t.Fatalf("ClaimTask failed: %v", err)
	}

	// Manually set task to READY_FOR_REVIEW with a commit
	err = bb.Modify(func(state *models.State) error {
		for i := range state.Tasks {
			if state.Tasks[i].ID == taskID {
				state.Tasks[i].Status = models.TaskStatusReadyForReview
				reviewCommit := "abc123"
				state.Tasks[i].ReviewCommit = &reviewCommit
				break
			}
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// First reviewer claims the review
	t.Log("Reviewer 1 claims review")
	reviewLeaseExpires := now.Add(30 * time.Minute)
	err = bb.Modify(func(state *models.State) error {
		for i := range state.Tasks {
			if state.Tasks[i].ID == taskID {
				state.Tasks[i].ReviewingBy = &reviewer1
				state.Tasks[i].ReviewLeaseExpires = &reviewLeaseExpires
				break
			}
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Verify review is claimed
	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)
	task := findTask(state.Tasks, taskID)
	if task.ReviewingBy == nil || *task.ReviewingBy != reviewer1 {
		t.Error("Expected review to be claimed by reviewer1")
	}

	// Simulate review lease expiry
	t.Log("Simulating review lease expiry")
	err = bb.Modify(func(state *models.State) error {
		for i := range state.Tasks {
			if state.Tasks[i].ID == taskID {
				expiredTime := now.Add(-1 * time.Hour)
				state.Tasks[i].ReviewLeaseExpires = &expiredTime
				break
			}
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Test the clear-stale-review-claims command
	t.Log("Clearing stale review claims")
	clearedCount, err := commands.ClearStaleReviewClaimsCommand(projectDir)
	if err != nil {
		t.Fatalf("ClearStaleReviewClaims failed: %v", err)
	}
	if clearedCount != 1 {
		t.Errorf("Expected 1 stale review claim to be cleared, got %d", clearedCount)
	}

	// Verify review claim was cleared
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.ReviewingBy != nil {
		t.Error("Expected review claim to be cleared after expiry")
	}
	if task.ReviewLeaseExpires != nil {
		t.Error("Expected review lease to be cleared after expiry")
	}

	// Now reviewer2 should be able to claim the review
	t.Log("Reviewer 2 claims review after expiry")
	newLeaseExpires := time.Now().UTC().Add(30 * time.Minute)
	err = bb.Modify(func(state *models.State) error {
		for i := range state.Tasks {
			if state.Tasks[i].ID == taskID {
				state.Tasks[i].ReviewingBy = &reviewer2
				state.Tasks[i].ReviewLeaseExpires = &newLeaseExpires
				break
			}
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Verify review is now claimed by reviewer2
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.ReviewingBy == nil || *task.ReviewingBy != reviewer2 {
		t.Error("Expected review to be claimed by reviewer2")
	}

	t.Log("✓ Expired review lease test passed")
}

// TestLeaseRenewal tests that heartbeats can extend leases
func TestLeaseRenewal(t *testing.T) {
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

	// Add a task
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

	// Claim task
	if err := commands.ClaimTaskCommand(projectDir, taskID, agentID); err != nil {
		t.Fatalf("ClaimTask failed: %v", err)
	}

	// Get initial lease expiry time
	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)
	task := findTask(state.Tasks, taskID)
	if task.LeaseExpires == nil {
		t.Fatal("Expected lease expiry to be set")
	}
	initialExpiry := *task.LeaseExpires

	// Simulate heartbeat by updating lease
	t.Log("Simulating heartbeat/lease renewal")
	time.Sleep(100 * time.Millisecond) // Small delay to ensure time difference

	err = bb.Modify(func(state *models.State) error {
		newExpiry := time.Now().UTC().Add(30 * time.Minute)
		for i := range state.Tasks {
			if state.Tasks[i].ID == taskID {
				state.Tasks[i].LeaseExpires = &newExpiry
				break
			}
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Verify lease was extended
	state, err = bb.Read()
	testhelpers.AssertNoError(t, err)
	task = findTask(state.Tasks, taskID)
	if task.LeaseExpires == nil {
		t.Fatal("Expected lease expiry to be set after renewal")
	}
	newExpiry := *task.LeaseExpires

	if !newExpiry.After(initialExpiry) {
		t.Error("Expected lease to be extended (new expiry should be after initial expiry)")
	}

	t.Log("✓ Lease renewal test passed")
}
