package testhelpers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
)

func TestCreateValidState(t *testing.T) {
	state := CreateValidState()

	// Verify state structure
	if state == nil {
		t.Fatal("CreateValidState returned nil")
	}

	// Verify version
	if state.Version != 1 {
		t.Errorf("Expected version=1, got %d", state.Version)
	}

	// Verify goal
	if state.Goal.ID == "" {
		t.Error("Goal ID is empty")
	}
	if state.Goal.Description == "" {
		t.Error("Goal description is empty")
	}
	if state.Goal.Status != models.GoalStatusInProgress {
		t.Errorf("Expected goal status=IN_PROGRESS, got %s", state.Goal.Status)
	}
	if len(state.Goal.AlignmentHistory) == 0 {
		t.Error("Goal alignment history is empty")
	}

	// Verify tasks slice is initialized
	if state.Tasks == nil {
		t.Error("Tasks slice is nil, should be empty slice")
	}

	// Verify agents map is initialized
	if state.Agents == nil {
		t.Error("Agents map is nil, should be empty map")
	}

	// Verify other slices are initialized
	if state.Discovered == nil {
		t.Error("Discovered slice is nil")
	}
	if state.HumanNotes == nil {
		t.Error("HumanNotes slice is nil")
	}
	if state.SpecChanges == nil {
		t.Error("SpecChanges slice is nil")
	}
	if state.Anomalies == nil {
		t.Error("Anomalies slice is nil")
	}

	// Verify sprint
	if state.Sprint.ID == "" {
		t.Error("Sprint ID is empty")
	}
	if state.Sprint.Status != models.SprintStatusInProgress {
		t.Errorf("Expected sprint status=IN_PROGRESS, got %s", state.Sprint.Status)
	}

	// Verify circuit breaker
	if state.CircuitBreaker.Status != "OK" {
		t.Errorf("Expected circuit breaker status=OK, got %s", state.CircuitBreaker.Status)
	}

	// Verify config has reasonable values
	if state.Config.MaxCoderIterations <= 0 {
		t.Error("MaxCoderIterations should be positive")
	}
	if state.Config.MaxReviewCycles <= 0 {
		t.Error("MaxReviewCycles should be positive")
	}
	if state.Config.IntegrationBranch == "" {
		t.Error("IntegrationBranch should not be empty")
	}
}

func TestCreateValidState_Customizable(t *testing.T) {
	// Verify that the returned state can be customized
	state := CreateValidState()

	// Customize goal
	state.Goal.Description = "Custom test goal"

	// Add tasks
	state.Tasks = append(state.Tasks, models.Task{
		ID:          "task-1",
		Description: "Custom task",
		Status:      models.TaskStatusReady,
		Priority:    1,
		Created:     time.Now().UTC(),
		SpecRef:     "specs/custom.md",
		DoneWhen:    "Done",
		Scope:       "Scope",
		History:     []models.TaskHistoryEntry{},
	})

	// Verify customizations took effect
	if state.Goal.Description != "Custom test goal" {
		t.Error("Failed to customize goal description")
	}
	if len(state.Tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(state.Tasks))
	}
}

func TestWriteInitialState(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup liza directory
	statePath, _ := SetupLizaDir(t, tmpDir)

	// Create and write state
	state := CreateValidState()
	bb := WriteInitialState(t, statePath, state)

	// Verify blackboard was returned
	if bb == nil {
		t.Fatal("WriteInitialState returned nil blackboard")
	}

	// Verify state file exists
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Error("State file was not created")
	}

	// Read back and verify state was written correctly
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state back: %v", err)
	}

	if readState.Goal.ID != state.Goal.ID {
		t.Errorf("Expected goal ID=%s, got %s", state.Goal.ID, readState.Goal.ID)
	}
}

func TestWriteInitialState_WithCustomState(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := SetupLizaDir(t, tmpDir)

	// Create custom state with tasks
	state := CreateValidState()
	state.Goal.Description = "Custom goal"
	state.Tasks = []models.Task{
		{
			ID:          "task-1",
			Description: "Test task",
			Status:      models.TaskStatusReady,
			Priority:    1,
			Created:     time.Now().UTC(),
			SpecRef:     "README.md",
			DoneWhen:    "Done",
			Scope:       "Scope",
			History:     []models.TaskHistoryEntry{},
		},
	}

	bb := WriteInitialState(t, statePath, state)

	// Read back and verify custom data
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	if readState.Goal.Description != "Custom goal" {
		t.Errorf("Custom goal not preserved")
	}
	if len(readState.Tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(readState.Tasks))
	}
}

func TestBuildTaskByStatus_Unclaimed(t *testing.T) {
	now := time.Now().UTC()
	task := BuildTaskByStatus("task-1", models.TaskStatusReady, now)

	if task.ID != "task-1" {
		t.Errorf("Expected ID=task-1, got %s", task.ID)
	}
	if task.Status != models.TaskStatusReady {
		t.Errorf("Expected status=READY, got %s", task.Status)
	}
	if task.AssignedTo != nil {
		t.Error("READY task should not have AssignedTo")
	}
	if task.LeaseExpires != nil {
		t.Error("READY task should not have LeaseExpires")
	}
}

func TestBuildTaskByStatus_Claimed(t *testing.T) {
	now := time.Now().UTC()
	task := BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)

	if task.Status != models.TaskStatusImplementing {
		t.Errorf("Expected status=IMPLEMENTING, got %s", task.Status)
	}
	if task.AssignedTo == nil {
		t.Error("IMPLEMENTING task should have AssignedTo")
	} else if *task.AssignedTo == "" {
		t.Error("AssignedTo should not be empty")
	}
	if task.LeaseExpires == nil {
		t.Error("IMPLEMENTING task should have LeaseExpires")
	}
	if task.BaseCommit == nil {
		t.Error("IMPLEMENTING task should have BaseCommit")
	}
	if task.Worktree == nil {
		t.Error("IMPLEMENTING task should have Worktree")
	}

	// Verify worktree path format
	expectedWorktree := ".worktrees/task-1"
	if *task.Worktree != expectedWorktree {
		t.Errorf("Expected worktree=%s, got %s", expectedWorktree, *task.Worktree)
	}
}

func TestBuildTaskByStatus_ReadyForReview(t *testing.T) {
	now := time.Now().UTC()
	task := BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)

	if task.Status != models.TaskStatusReadyForReview {
		t.Errorf("Expected status=READY_FOR_REVIEW, got %s", task.Status)
	}
	if task.AssignedTo == nil {
		t.Error("READY_FOR_REVIEW task should have AssignedTo")
	}
	if task.ReviewCommit == nil {
		t.Error("READY_FOR_REVIEW task should have ReviewCommit")
	}
	if task.BaseCommit == nil {
		t.Error("READY_FOR_REVIEW task should have BaseCommit")
	}
	if task.Worktree == nil {
		t.Error("READY_FOR_REVIEW task should have Worktree")
	}
}

func TestBuildTaskByStatus_Reviewing(t *testing.T) {
	now := time.Now().UTC()
	task := BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)

	if task.Status != models.TaskStatusReviewing {
		t.Errorf("Expected status=REVIEWING, got %s", task.Status)
	}
	if task.AssignedTo == nil {
		t.Error("REVIEWING task should have AssignedTo")
	}
	if task.ReviewCommit == nil {
		t.Error("REVIEWING task should have ReviewCommit")
	}
	if task.ReviewingBy == nil {
		t.Error("REVIEWING task should have ReviewingBy")
	}
	if task.ReviewLeaseExpires == nil {
		t.Error("REVIEWING task should have ReviewLeaseExpires")
	}
	if task.BaseCommit == nil {
		t.Error("REVIEWING task should have BaseCommit")
	}
	if task.Worktree == nil {
		t.Error("REVIEWING task should have Worktree")
	}
}

func TestBuildTaskByStatus_Rejected(t *testing.T) {
	now := time.Now().UTC()
	task := BuildTaskByStatus("task-1", models.TaskStatusRejected, now)

	if task.Status != models.TaskStatusRejected {
		t.Errorf("Expected status=REJECTED, got %s", task.Status)
	}
	if task.RejectionReason == nil {
		t.Error("REJECTED task should have RejectionReason")
	}
	if task.ReviewCommit == nil {
		t.Error("REJECTED task should have ReviewCommit")
	}
	if task.Iteration != 1 {
		t.Errorf("Expected iteration=1, got %d", task.Iteration)
	}
}

func TestBuildTaskByStatus_Approved(t *testing.T) {
	now := time.Now().UTC()
	task := BuildTaskByStatus("task-1", models.TaskStatusApproved, now)

	if task.Status != models.TaskStatusApproved {
		t.Errorf("Expected status=APPROVED, got %s", task.Status)
	}
	if task.ApprovedBy == nil {
		t.Error("APPROVED task should have ApprovedBy")
	}
	if task.ReviewCommit == nil {
		t.Error("APPROVED task should have ReviewCommit")
	}
}

func TestBuildTaskByStatus_Merged(t *testing.T) {
	now := time.Now().UTC()
	task := BuildTaskByStatus("task-1", models.TaskStatusMerged, now)

	if task.Status != models.TaskStatusMerged {
		t.Errorf("Expected status=MERGED, got %s", task.Status)
	}
	if task.MergeCommit == nil {
		t.Error("MERGED task should have MergeCommit")
	}
	if task.ApprovedBy == nil {
		t.Error("MERGED task should have ApprovedBy")
	}
}

func TestBuildTaskByStatus_Blocked(t *testing.T) {
	now := time.Now().UTC()
	task := BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)

	if task.Status != models.TaskStatusBlocked {
		t.Errorf("Expected status=BLOCKED, got %s", task.Status)
	}
	if task.BlockedReason == nil {
		t.Error("BLOCKED task should have BlockedReason")
	}
	if len(task.BlockedQuestions) == 0 {
		t.Error("BLOCKED task should have BlockedQuestions")
	}
}

func TestBuildTaskByStatus_IntegrationFailed(t *testing.T) {
	now := time.Now().UTC()
	task := BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now)

	if task.Status != models.TaskStatusIntegrationFailed {
		t.Errorf("Expected status=INTEGRATION_FAILED, got %s", task.Status)
	}
	if task.AssignedTo == nil {
		t.Error("INTEGRATION_FAILED task should have AssignedTo")
	}
}

func TestBuildTaskByStatus_Abandoned(t *testing.T) {
	now := time.Now().UTC()
	task := BuildTaskByStatus("task-1", models.TaskStatusAbandoned, now)

	if task.Status != models.TaskStatusAbandoned {
		t.Errorf("Expected status=ABANDONED, got %s", task.Status)
	}
	// Abandoned tasks don't need extra fields, just verify basic structure
	if task.ID != "task-1" {
		t.Errorf("Expected ID=task-1, got %s", task.ID)
	}
}

func TestBuildTaskByStatus_Superseded(t *testing.T) {
	now := time.Now().UTC()
	task := BuildTaskByStatus("task-1", models.TaskStatusSuperseded, now)

	if task.Status != models.TaskStatusSuperseded {
		t.Errorf("Expected status=SUPERSEDED, got %s", task.Status)
	}
	if len(task.SupersededBy) == 0 {
		t.Error("SUPERSEDED task should have SupersededBy")
	}
}

func TestBuildTaskByStatus_Draft(t *testing.T) {
	now := time.Now().UTC()
	task := BuildTaskByStatus("task-1", models.TaskStatusDraft, now)

	if task.Status != models.TaskStatusDraft {
		t.Errorf("Expected status=DRAFT, got %s", task.Status)
	}
	// Draft tasks are basic, just verify core fields
	if task.ID != "task-1" {
		t.Errorf("Expected ID=task-1, got %s", task.ID)
	}
}

func TestBuildTaskByStatus_CommonFields(t *testing.T) {
	// Verify all tasks have common required fields
	now := time.Now().UTC()
	statuses := []models.TaskStatus{
		models.TaskStatusReady,
		models.TaskStatusImplementing,
		models.TaskStatusReadyForReview,
		models.TaskStatusReviewing,
		models.TaskStatusRejected,
		models.TaskStatusApproved,
		models.TaskStatusMerged,
		models.TaskStatusBlocked,
	}

	for _, status := range statuses {
		task := BuildTaskByStatus("task-test", status, now)

		if task.ID == "" {
			t.Errorf("Task with status %s has empty ID", status)
		}
		if task.Description == "" {
			t.Errorf("Task with status %s has empty Description", status)
		}
		if task.Priority == 0 {
			t.Errorf("Task with status %s has zero Priority", status)
		}
		if task.SpecRef == "" {
			t.Errorf("Task with status %s has empty SpecRef", status)
		}
		if task.DoneWhen == "" {
			t.Errorf("Task with status %s has empty DoneWhen", status)
		}
		if task.Scope == "" {
			t.Errorf("Task with status %s has empty Scope", status)
		}
		if task.History == nil {
			t.Errorf("Task with status %s has nil History", status)
		}
	}
}

func TestStringPtr(t *testing.T) {
	s := "test string"
	ptr := StringPtr(s)

	if ptr == nil {
		t.Fatal("StringPtr returned nil")
	}
	if *ptr != s {
		t.Errorf("Expected *ptr=%s, got %s", s, *ptr)
	}

	// Verify it's actually a pointer to the value
	*ptr = "modified"
	if *ptr != "modified" {
		t.Error("Failed to modify through pointer")
	}
}

func TestTimePtr(t *testing.T) {
	now := time.Now().UTC()
	ptr := TimePtr(now)

	if ptr == nil {
		t.Fatal("TimePtr returned nil")
	}
	if !ptr.Equal(now) {
		t.Errorf("Expected *ptr=%v, got %v", now, *ptr)
	}

	// Verify it's actually a pointer
	newTime := now.Add(1 * time.Hour)
	*ptr = newTime
	if !ptr.Equal(newTime) {
		t.Error("Failed to modify through pointer")
	}
}

func TestIntegration_FullWorkflow(t *testing.T) {
	// Test a complete workflow using all helpers together
	tmpDir := t.TempDir()

	// Setup git repo and liza directory
	SetupTestGitRepo(t, tmpDir)
	statePath, _ := SetupLizaDir(t, tmpDir)
	CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n")

	// Create state with tasks in various statuses
	state := CreateValidState()
	now := time.Now().UTC()
	state.Tasks = []models.Task{
		BuildTaskByStatus("task-1", models.TaskStatusReady, now),
		BuildTaskByStatus("task-2", models.TaskStatusImplementing, now),
		BuildTaskByStatus("task-3", models.TaskStatusReadyForReview, now),
	}

	// Write state
	bb := WriteInitialState(t, statePath, state)

	// Read back and verify
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	if len(readState.Tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(readState.Tasks))
	}

	// Verify git repo exists
	gitDir := filepath.Join(tmpDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Error("Git directory should exist")
	}

	// Verify spec file exists
	specPath := filepath.Join(tmpDir, "specs", "vision.md")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		t.Error("Spec file should exist")
	}
}
