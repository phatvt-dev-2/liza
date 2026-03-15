package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestClaimReviewerTask_Validation(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupLizaDir(t, tmpDir)

	tests := []struct {
		name        string
		input       ClaimReviewerTaskInput
		errContains string
	}{
		{
			name:        "empty agent ID",
			input:       ClaimReviewerTaskInput{ProjectRoot: tmpDir, LeaseDuration: 1800},
			errContains: "agent ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ClaimReviewerTask(tt.input)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Error = %q, want to contain %q", err.Error(), tt.errContains)
			}
		})
	}
}

func TestClaimReviewerTask_DefaultLeaseDuration(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	worktree := ".worktrees/task-1"
	reviewCommit := "abc123"
	state.Tasks = []models.Task{
		{
			ID:           "task-1",
			Status:       models.TaskStatusReadyForReview,
			RolePair:     "coding-pair",
			Worktree:     &worktree,
			ReviewCommit: &reviewCommit,
			Created:      now,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	// When LeaseDuration is 0, should use default (1800 seconds)
	start := time.Now()
	result, err := ClaimReviewerTask(ClaimReviewerTaskInput{
		ProjectRoot:   tmpDir,
		AgentID:       "code-reviewer-1",
		LeaseDuration: 0, // Should use default
	})
	if err != nil {
		t.Fatalf("ClaimReviewerTask() error: %v", err)
	}

	// Verify lease was set using default duration (1800s = 30m)
	expectedLeaseMin := start.Add(1700 * time.Second) // Allow some tolerance
	expectedLeaseMax := start.Add(1900 * time.Second)
	if result.LeaseExpires.Before(expectedLeaseMin) || result.LeaseExpires.After(expectedLeaseMax) {
		t.Errorf("LeaseExpires = %v, expected between %v and %v", result.LeaseExpires, expectedLeaseMin, expectedLeaseMax)
	}
}

func TestClaimReviewerTask_NoReviewableTasks(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	// Only create tasks in non-reviewable states
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-ready", models.TaskStatusReady, now),
		testhelpers.BuildTaskByStatus("task-implementing", models.TaskStatusImplementing, now),
		testhelpers.BuildTaskByStatus("task-merged", models.TaskStatusMerged, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ClaimReviewerTask(ClaimReviewerTaskInput{
		ProjectRoot:   tmpDir,
		AgentID:       "code-reviewer-1",
		LeaseDuration: 1800,
	})
	if err == nil {
		t.Fatal("Expected error when no reviewable tasks, got nil")
	}
	if !strings.Contains(err.Error(), "no reviewable tasks found") {
		t.Errorf("Error = %q, want to contain 'no reviewable tasks found'", err.Error())
	}
}

func TestClaimReviewerTask_Success(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	worktree := ".worktrees/task-1"
	reviewCommit := "abc123"
	state.Tasks = []models.Task{
		{
			ID:           "task-1",
			Status:       models.TaskStatusReadyForReview,
			RolePair:     "coding-pair",
			Priority:     1,
			Worktree:     &worktree,
			ReviewCommit: &reviewCommit,
			History:      []models.TaskHistoryEntry{},
			Created:      now,
		},
	}
	state.Agents["code-reviewer-1"] = models.Agent{
		Role:   models.RoleCodeReviewer,
		Status: models.AgentStatusIdle,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ClaimReviewerTask(ClaimReviewerTaskInput{
		ProjectRoot:   tmpDir,
		AgentID:       "code-reviewer-1",
		LeaseDuration: 1800,
	})
	if err != nil {
		t.Fatalf("ClaimReviewerTask() error: %v", err)
	}

	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
	if result.Worktree != worktree {
		t.Errorf("Worktree = %q, want %q", result.Worktree, worktree)
	}
	if result.ReviewCommit != reviewCommit {
		t.Errorf("ReviewCommit = %q, want %q", result.ReviewCommit, reviewCommit)
	}
	if result.LeaseExpires.IsZero() {
		t.Error("LeaseExpires should not be zero")
	}

	// Verify state was updated
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := readState.FindTask("task-1")
	if task == nil {
		t.Fatal("Task not found")
	}
	if task.Status != models.TaskStatusReviewing {
		t.Errorf("Task status = %v, want REVIEWING_CODE", task.Status)
	}
	if task.ReviewingBy == nil || *task.ReviewingBy != "code-reviewer-1" {
		t.Error("Task ReviewingBy should be code-reviewer-1")
	}
	if task.ReviewLeaseExpires == nil {
		t.Error("Task ReviewLeaseExpires should be set")
	}

	agent, exists := readState.Agents["code-reviewer-1"]
	if !exists {
		t.Fatal("Agent not found")
	}
	if agent.Status != models.AgentStatusReviewing {
		t.Errorf("Agent status = %v, want REVIEWING_CODE", agent.Status)
	}
	if agent.CurrentTask == nil || *agent.CurrentTask != "task-1" {
		t.Error("Agent CurrentTask should be task-1")
	}
}

func TestClaimReviewerTask_PrioritySelection(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	worktree1 := ".worktrees/task-low"
	worktree2 := ".worktrees/task-high"
	reviewCommit1 := "abc123"
	reviewCommit2 := "def456"
	state.Tasks = []models.Task{
		{
			ID:           "task-low",
			Status:       models.TaskStatusReadyForReview,
			RolePair:     "coding-pair",
			Priority:     3,
			Worktree:     &worktree1,
			ReviewCommit: &reviewCommit1,
			Created:      now.Add(-1 * time.Minute),
		},
		{
			ID:           "task-high",
			Status:       models.TaskStatusReadyForReview,
			RolePair:     "coding-pair",
			Priority:     1,
			Worktree:     &worktree2,
			ReviewCommit: &reviewCommit2,
			Created:      now,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ClaimReviewerTask(ClaimReviewerTaskInput{
		ProjectRoot:   tmpDir,
		AgentID:       "code-reviewer-1",
		LeaseDuration: 1800,
	})
	if err != nil {
		t.Fatalf("ClaimReviewerTask() error: %v", err)
	}

	// Should claim the high-priority task (lower number = higher priority)
	if result.TaskID != "task-high" {
		t.Errorf("TaskID = %q, want %q (higher priority)", result.TaskID, "task-high")
	}
}

func TestClaimReviewerTask_TieBreaking(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	worktree1 := ".worktrees/task-old"
	worktree2 := ".worktrees/task-new"
	reviewCommit1 := "abc123"
	reviewCommit2 := "def456"
	state.Tasks = []models.Task{
		{
			ID:           "task-new",
			Status:       models.TaskStatusReadyForReview,
			RolePair:     "coding-pair",
			Priority:     2,
			Worktree:     &worktree2,
			ReviewCommit: &reviewCommit2,
			Created:      now,
		},
		{
			ID:           "task-old",
			Status:       models.TaskStatusReadyForReview,
			RolePair:     "coding-pair",
			Priority:     2,
			Worktree:     &worktree1,
			ReviewCommit: &reviewCommit1,
			Created:      now.Add(-1 * time.Minute),
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ClaimReviewerTask(ClaimReviewerTaskInput{
		ProjectRoot:   tmpDir,
		AgentID:       "code-reviewer-1",
		LeaseDuration: 1800,
	})
	if err != nil {
		t.Fatalf("ClaimReviewerTask() error: %v", err)
	}

	// With randomized selection, either task is valid (same priority tier)
	validIDs := map[string]bool{"task-old": true, "task-new": true}
	if !validIDs[result.TaskID] {
		t.Errorf("TaskID = %q, want one of %v", result.TaskID, validIDs)
	}
}

func TestClaimReviewerTask_MissingReviewCommit(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	worktree := ".worktrees/task-1"
	// Task is READY_FOR_REVIEW but missing ReviewCommit (corrupted state)
	state.Tasks = []models.Task{
		{
			ID:       "task-1",
			Status:   models.TaskStatusReadyForReview,
			RolePair: "coding-pair",
			Priority: 1,
			Worktree: &worktree,
			// ReviewCommit intentionally nil
			History: []models.TaskHistoryEntry{},
			Created: now,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ClaimReviewerTask(ClaimReviewerTaskInput{
		ProjectRoot:   tmpDir,
		AgentID:       "code-reviewer-1",
		LeaseDuration: 1800,
	})
	if err == nil {
		t.Fatal("Expected error for missing review_commit, got nil")
	}
	if !strings.Contains(err.Error(), "no review_commit") {
		t.Errorf("Error = %q, want to contain 'no review_commit'", err.Error())
	}
}

func TestClaimReviewerTask_CodePlanReviewerInference(t *testing.T) {
	// Verifies that a code-plan-reviewer agent (identified by agent ID only,
	// no explicit WorkflowRole) gets correct workflow role inference via roles.ToWorkflow.
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	worktree := ".worktrees/plan-task-1"
	reviewCommit := "abc123"
	state.Tasks = []models.Task{
		{
			ID:           "plan-task-1",
			Status:       models.TaskStatusCodingPlanToReview,
			RolePair:     "code-planning-pair",
			Priority:     1,
			Worktree:     &worktree,
			ReviewCommit: &reviewCommit,
			History:      []models.TaskHistoryEntry{},
			Created:      now,
		},
	}
	state.Agents["code-plan-reviewer-1"] = models.Agent{
		Role:   models.RoleCodePlanReviewer,
		Status: models.AgentStatusIdle,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	// No explicit WorkflowRole — inference from agent ID must use roles.ToWorkflow
	result, err := ClaimReviewerTask(ClaimReviewerTaskInput{
		ProjectRoot:   tmpDir,
		AgentID:       "code-plan-reviewer-1",
		LeaseDuration: 1800,
	})
	if err != nil {
		t.Fatalf("ClaimReviewerTask() error: %v", err)
	}

	if result.TaskID != "plan-task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "plan-task-1")
	}

	// Verify the task transitioned to reviewing state
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	task := readState.FindTask("plan-task-1")
	if task == nil {
		t.Fatal("Task not found")
	}
	if task.Status != models.TaskStatusReviewingCodingPlan {
		t.Errorf("Task status = %v, want REVIEWING_CODING_PLAN", task.Status)
	}
	if task.ReviewingBy == nil || *task.ReviewingBy != "code-plan-reviewer-1" {
		t.Error("Task ReviewingBy should be code-plan-reviewer-1")
	}
}

func TestClaimReviewerTask_SkipsAlreadyReviewing(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	worktree1 := ".worktrees/task-reviewing"
	worktree2 := ".worktrees/task-available"
	reviewer := "code-reviewer-99"
	leaseExpires := now.Add(1 * time.Hour)
	reviewCommit1 := "abc123"
	reviewCommit2 := "def456"
	state.Tasks = []models.Task{
		{
			ID:                 "task-reviewing",
			Status:             models.TaskStatusReviewing,
			RolePair:           "coding-pair",
			Priority:           1, // High priority but already claimed
			Worktree:           &worktree1,
			ReviewCommit:       &reviewCommit1,
			ReviewingBy:        &reviewer,
			ReviewLeaseExpires: &leaseExpires,
			Created:            now,
		},
		{
			ID:           "task-available",
			Status:       models.TaskStatusReadyForReview,
			RolePair:     "coding-pair",
			Priority:     3, // Lower priority but available
			Worktree:     &worktree2,
			ReviewCommit: &reviewCommit2,
			Created:      now,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ClaimReviewerTask(ClaimReviewerTaskInput{
		ProjectRoot:   tmpDir,
		AgentID:       "code-reviewer-1",
		LeaseDuration: 1800,
	})
	if err != nil {
		t.Fatalf("ClaimReviewerTask() error: %v", err)
	}

	// Should skip the REVIEWING task and claim the available one
	if result.TaskID != "task-available" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-available")
	}
}
