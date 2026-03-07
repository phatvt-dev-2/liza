package agent

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestEnsureReviewerWorktree_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	state.Tasks = []models.Task{task}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	// Create the worktree directory so it "exists"
	testhelpers.CreateTestWorktree(t, tmpDir, "task-1")

	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", "code-reviewer-1")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if recovered {
		t.Error("Expected recovered=false when worktree exists")
	}
}

func TestEnsureReviewerWorktree_MissingRecoverable(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	state.Tasks = []models.Task{task}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	// Create the branch (task/task-1) so recovery can find it.
	branchName := paths.TaskBranchPrefix + "task-1"
	cmd := exec.Command("git", "branch", branchName)
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to create branch: %v\n%s", err, out)
	}

	// No worktree directory — recovery should recreate it.
	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", "code-reviewer-1")
	if err != nil {
		t.Fatalf("Expected successful recovery, got error: %v", err)
	}
	if !recovered {
		t.Error("Expected recovered=true")
	}

	// Verify worktree was created.
	wtPath := filepath.Join(tmpDir, paths.WorktreesDirName, "task-1")
	if _, statErr := os.Stat(wtPath); os.IsNotExist(statErr) {
		t.Error("Expected worktree directory to exist after recovery")
	}

	// Verify history entry was added.
	readState, _ := bb.Read()
	readTask := readState.FindTask("task-1")
	if readTask == nil {
		t.Fatal("Task not found in state")
	}
	found := false
	for _, h := range readTask.History {
		if h.Event == "worktree_recovered" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'worktree_recovered' history entry")
	}
}

func TestEnsureReviewerWorktree_MissingRecoverable_RunsPostWorktreeCmd(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	now := time.Now().UTC()

	// Configure a post-worktree command that creates a marker file.
	postCmd := "touch .post-worktree-ran"
	state.Config.PostWorktreeCmd = &postCmd
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	state.Tasks = []models.Task{task}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	// Create the branch so recovery can find it.
	branchName := paths.TaskBranchPrefix + "task-1"
	cmd := exec.Command("git", "branch", branchName)
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to create branch: %v\n%s", err, out)
	}

	// No worktree directory — recovery should recreate it.
	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", "code-reviewer-1")
	if err != nil {
		t.Fatalf("Expected successful recovery, got error: %v", err)
	}
	if !recovered {
		t.Error("Expected recovered=true")
	}

	// Verify the post-worktree command ran in the recovered worktree.
	markerPath := filepath.Join(tmpDir, paths.WorktreesDirName, "task-1", ".post-worktree-ran")
	if _, statErr := os.Stat(markerPath); os.IsNotExist(statErr) {
		t.Error("Post-worktree command did not run after worktree recovery: marker file missing")
	}
}

func TestEnsureReviewerWorktree_MissingAlreadyRecovered(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	// Simulate reviewer claim.
	reviewerID := "code-reviewer-1"
	task.ReviewingBy = &reviewerID
	// Simulate a prior recovery.
	task.History = append(task.History, models.TaskHistoryEntry{
		Time:  now.Add(-5 * time.Minute),
		Event: "worktree_recovered",
		Agent: &reviewerID,
	})
	state.Tasks = []models.Task{task}
	// Register the reviewer agent so ReleaseAgent can reset it.
	taskPtr := "task-1"
	state.Agents[reviewerID] = models.Agent{
		Role:        models.RoleCodeReviewer,
		Status:      models.AgentStatusReviewing,
		CurrentTask: &taskPtr,
	}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	// No worktree and already recovered once — should block.
	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", reviewerID)
	if err == nil {
		t.Fatal("Expected error for already-recovered task")
	}
	if recovered {
		t.Error("Expected recovered=false")
	}
	if !errors.Is(err, errTaskBlocked) {
		t.Errorf("Expected errTaskBlocked, got: %v", err)
	}

	// Verify task was blocked in state.
	readState, _ := bb.Read()
	readTask := readState.FindTask("task-1")
	if readTask == nil {
		t.Fatal("Task not found")
	}
	if readTask.Status != models.TaskStatusBlocked {
		t.Errorf("Expected BLOCKED status, got %s", readTask.Status)
	}

	// Verify reviewer agent was released to IDLE.
	agent, exists := readState.Agents[reviewerID]
	if !exists {
		t.Fatal("Agent not found in state")
	}
	if agent.Status != models.AgentStatusIdle {
		t.Errorf("Expected agent status IDLE, got %s", agent.Status)
	}
	if agent.CurrentTask != nil {
		t.Errorf("Expected agent CurrentTask nil, got %v", agent.CurrentTask)
	}
}

func TestEnsureReviewerWorktree_MissingBranchGone(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	reviewerID := "code-reviewer-1"
	task.ReviewingBy = &reviewerID
	state.Tasks = []models.Task{task}
	taskPtr := "task-1"
	state.Agents[reviewerID] = models.Agent{
		Role:        models.RoleCodeReviewer,
		Status:      models.AgentStatusReviewing,
		CurrentTask: &taskPtr,
	}
	bb := testhelpers.WriteInitialState(t, statePath, state)

	// No worktree AND no branch — unrecoverable.
	recovered, err := ensureReviewerWorktree(tmpDir, bb, "task-1", reviewerID)
	if err == nil {
		t.Fatal("Expected error when branch is missing")
	}
	if recovered {
		t.Error("Expected recovered=false")
	}
	if !errors.Is(err, errTaskBlocked) {
		t.Errorf("Expected errTaskBlocked, got: %v", err)
	}

	// Verify task was blocked.
	readState, _ := bb.Read()
	readTask := readState.FindTask("task-1")
	if readTask == nil {
		t.Fatal("Task not found")
	}
	if readTask.Status != models.TaskStatusBlocked {
		t.Errorf("Expected BLOCKED status, got %s", readTask.Status)
	}

	// Verify reviewer agent was released to IDLE.
	agent, exists := readState.Agents[reviewerID]
	if !exists {
		t.Fatal("Agent not found in state")
	}
	if agent.Status != models.AgentStatusIdle {
		t.Errorf("Expected agent status IDLE, got %s", agent.Status)
	}
}
