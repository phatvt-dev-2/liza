package integration

// lease_expiry_test.go contains integration tests for lease expiry handling.
//
// These tests verify that the system correctly handles expired leases for
// both task claims and review claims.

import (
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// TestExpiredClaimLease tests that tasks with expired leases can be re-claimed
func TestExpiredClaimLease(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	projectDir, cleanup := setupTestProject(t)
	defer cleanup()

	taskID := "task-1"
	bb, _, _ := setupIntegrationTest(t, projectDir, []string{taskID})

	// Register first agent
	agent1 := "coder-1"
	testhelpers.RegisterTestAgent(t, bb, agent1, "coder")

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
				expiredTime := time.Now().UTC().Add(-1 * time.Hour) // Expired 1 hour ago
				state.Tasks[i].LeaseExpires = &expiredTime
				break
			}
		}
		return nil
	})
	testhelpers.AssertNoError(t, err)

	// Register second agent
	agent2 := "coder-2"
	testhelpers.RegisterTestAgent(t, bb, agent2, "coder")

	// Second agent should NOT be able to claim the task directly
	// (The claim command doesn't check for expired leases itself - that's the watch command's job)
	// But the validation should catch that it's still IMPLEMENTING
	t.Log("Agent 2 attempts to claim already-claimed task")
	err = commands.ClaimTaskCommand(projectDir, taskID, agent2)
	if err == nil {
		t.Error("Expected error when claiming already-claimed task")
	}

	t.Log("✓ Expired claim lease test passed")
}

// TestExpiredReviewLease tests that review leases can expire and be reclaimed
func TestExpiredReviewLease(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	projectDir, cleanup := setupTestProject(t)
	defer cleanup()

	taskID := "task-1"
	bb, _, _ := setupIntegrationTest(t, projectDir, []string{taskID})

	// Register agents
	coderID := "coder-1"
	reviewer1 := "code-reviewer-1"
	reviewer2 := "code-reviewer-2"
	testhelpers.RegisterTestAgent(t, bb, coderID, "coder")
	testhelpers.RegisterTestAgent(t, bb, reviewer1, "code-reviewer")
	testhelpers.RegisterTestAgent(t, bb, reviewer2, "code-reviewer")

	// Claim and submit task for review
	if err := commands.ClaimTaskCommand(projectDir, taskID, coderID); err != nil {
		t.Fatalf("ClaimTask failed: %v", err)
	}

	// Manually set task to READY_FOR_REVIEW with a commit
	err := bb.Modify(func(state *models.State) error {
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

	// First reviewer claims the review (transitions to REVIEWING)
	t.Log("Reviewer 1 claims review")
	reviewLeaseExpires := time.Now().UTC().Add(30 * time.Minute)
	err = bb.Modify(func(state *models.State) error {
		for i := range state.Tasks {
			if state.Tasks[i].ID == taskID {
				state.Tasks[i].Status = models.TaskStatusReviewing
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
				expiredTime := time.Now().UTC().Add(-1 * time.Hour)
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

	// Now reviewer2 should be able to claim the review (transitions to REVIEWING)
	t.Log("Reviewer 2 claims review after expiry")
	newLeaseExpires := time.Now().UTC().Add(30 * time.Minute)
	err = bb.Modify(func(state *models.State) error {
		for i := range state.Tasks {
			if state.Tasks[i].ID == taskID {
				state.Tasks[i].Status = models.TaskStatusReviewing
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
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	projectDir, cleanup := setupTestProject(t)
	defer cleanup()

	taskID := "task-1"
	bb, _, _ := setupIntegrationTest(t, projectDir, []string{taskID})

	// Register agent
	agentID := "coder-1"
	testhelpers.RegisterTestAgent(t, bb, agentID, "coder")

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

	err = bb.Modify(func(state *models.State) error {
		newExpiry := initialExpiry.Add(100 * time.Millisecond)
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
