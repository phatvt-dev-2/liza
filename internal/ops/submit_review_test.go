package ops

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestSubmitForReview_Validation(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		commitSHA   string
		agentID     string
		errContains string
	}{
		{
			name: "empty task ID", commitSHA: "abc123", agentID: "coder-1",
			errContains: "task ID is required",
		},
		{
			name: "empty commit SHA", taskID: "t1", agentID: "coder-1",
			errContains: "commit SHA is required",
		},
		{
			name: "empty agent ID", taskID: "t1", commitSHA: "abc123",
			errContains: "LIZA_AGENT_ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SubmitForReview("/nonexistent", tt.taskID, tt.commitSHA, tt.agentID)
			testhelpers.RequireErrorContains(t, err, tt.errContains)
		})
	}
}

func TestSubmitForReview_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitForReview(tmpDir, "nonexistent", "abc123", "coder-1")
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestSubmitForReview_WrongStatus(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitForReview(tmpDir, "task-1", "abc123", "coder-1")
	testhelpers.RequireErrorContains(t, err, "not IMPLEMENTING")
}

func TestSubmitForReview_WrongAgent(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitForReview(tmpDir, "task-1", "abc123", "coder-2")
	testhelpers.RequireErrorContains(t, err, "not assigned to agent")
}

func TestSubmitForReview_NoWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
	task.Worktree = nil // No worktree
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitForReview(tmpDir, "task-1", "abc123", "coder-1")
	testhelpers.RequireErrorContains(t, err, "no worktree")
}

func TestSubmitForReview_TDDWaiverBypassesTestRequirement(t *testing.T) {
	// Unit test: verify that GetTDDWaiver check in SubmitForReview
	// allows submission without test files when waiver is declared.
	// This tests the waiver logic at the data level since the full
	// SubmitForReview path requires a real git worktree.
	agent := "coder-1"
	history := []models.TaskHistoryEntry{
		{
			Event: "pre_execution_checkpoint",
			Agent: &agent,
			Extra: map[string]any{
				"intent":           "Fix comment typo",
				"tdd_not_required": "cosmetic-only: comment fix, no behavior change",
			},
		},
	}

	// With waiver, GetTDDWaiver should return non-empty
	waiver := GetTDDWaiver(history, "coder-1")
	if waiver == "" {
		t.Fatal("Expected non-empty waiver from checkpoint with tdd_not_required")
	}
	if waiver != "cosmetic-only: comment fix, no behavior change" {
		t.Errorf("Unexpected waiver value: %q", waiver)
	}

	// Without waiver, GetTDDWaiver should return empty
	historyNoWaiver := []models.TaskHistoryEntry{
		{
			Event: "pre_execution_checkpoint",
			Agent: &agent,
			Extra: map[string]any{
				"intent": "Add feature",
			},
		},
	}
	if GetTDDWaiver(historyNoWaiver, "coder-1") != "" {
		t.Fatal("Expected empty waiver from checkpoint without tdd_not_required")
	}
}

func TestSubmitForReview_NoCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	// Task has worktree but no checkpoint in history
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitForReview(tmpDir, "task-1", "abc123", "coder-1")
	testhelpers.RequireErrorContains(t, err, "pre-execution checkpoint required")
}

// setupRebaseConflictScenario creates a git repo with a worktree whose branch
// conflicts with integration. Returns (tmpDir, taskID, worktreeCommitSHA, agentID, blackboard).
func setupRebaseConflictScenario(t *testing.T) (string, string, string, string, *db.Blackboard) {
	t.Helper()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	testhelpers.MustGit(t, tmpDir, "checkout", "integration")

	g := git.New(tmpDir)
	taskID := "task-rebase-conflict"
	baseCommit, err := g.CreateWorktree(taskID, "integration")
	if err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}
	wtPath := g.GetWorktreePath(taskID)

	// Modify README in worktree (will conflict) and add test file for TDD
	if err := os.WriteFile(filepath.Join(wtPath, "README.md"), []byte("# Task version\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "task_test.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, wtPath, "add", "README.md", "task_test.go")
	testhelpers.MustGit(t, wtPath, "commit", "-m", "Task commit")
	wtCommit := testhelpers.MustGit(t, wtPath, "rev-parse", "HEAD")

	// Create conflicting change on integration branch
	testhelpers.MustGit(t, tmpDir, "checkout", "integration")
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Integration version\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, tmpDir, "add", "README.md")
	testhelpers.MustGit(t, tmpDir, "commit", "-m", "Integration commit")

	agentID := "coder-1"
	leaseExpires := time.Now().UTC().Add(30 * time.Minute)
	worktree := g.GetWorktreeRelPath(taskID)
	initialState := &models.State{
		Config: models.Config{
			IntegrationBranch: "integration",
			LeaseDuration:     1800,
		},
		Tasks: []models.Task{
			{
				ID:           taskID,
				Description:  "Task with rebase conflict",
				Status:       models.TaskStatusImplementing,
				AssignedTo:   &agentID,
				LeaseExpires: &leaseExpires,
				Worktree:     &worktree,
				BaseCommit:   &baseCommit,
				Iteration:    1,
				Created:      time.Now().UTC(),
				History: []models.TaskHistoryEntry{
					{
						Time:  time.Now().UTC(),
						Event: "pre_execution_checkpoint",
						Agent: &agentID,
						Extra: map[string]any{
							"intent":          "test",
							"validation_plan": "test",
							"files_to_modify": []string{"README.md"},
						},
					},
				},
			},
		},
		Agents: map[string]models.Agent{
			agentID: {Status: models.AgentStatusWorking, CurrentTask: &taskID},
		},
	}

	bb := testhelpers.WriteInitialState(t, statePath, initialState)
	return tmpDir, taskID, wtCommit, agentID, bb
}

func TestSubmitForReview_RebaseConflict_TransitionsToIntegrationFailed(t *testing.T) {
	tmpDir, taskID, wtCommit, agentID, bb := setupRebaseConflictScenario(t)

	_, err := SubmitForReview(tmpDir, taskID, wtCommit, agentID)
	if err == nil {
		t.Fatal("expected error due to rebase conflict, got nil")
	}

	// Should return IntegrationFailedError
	var ifErr *IntegrationFailedError
	if !stderrors.As(err, &ifErr) {
		t.Fatalf("expected *IntegrationFailedError, got %T: %v", err, err)
	}
	if ifErr.Reason != IntegrationReasonMergeConflict {
		t.Errorf("expected reason %q, got %q", IntegrationReasonMergeConflict, ifErr.Reason)
	}

	// Task should be INTEGRATION_FAILED
	state, err := bb.Read()
	testhelpers.AssertNoError(t, err)
	task := state.FindTask(taskID)
	if task.Status != models.TaskStatusIntegrationFailed {
		t.Errorf("expected INTEGRATION_FAILED, got %s", task.Status)
	}

	// Agent should be released
	if task.AssignedTo != nil {
		t.Errorf("expected agent released (AssignedTo nil), got %v", *task.AssignedTo)
	}
	agent := state.Agents[agentID]
	if agent.Status != models.AgentStatusWaiting {
		t.Errorf("expected agent status WAITING, got %s", agent.Status)
	}
	if agent.CurrentTask != nil {
		t.Errorf("expected agent CurrentTask nil, got %v", *agent.CurrentTask)
	}

	// FailedBy should include the agent
	if len(task.FailedBy) == 0 || task.FailedBy[0] != agentID {
		t.Errorf("expected FailedBy to include %s, got %v", agentID, task.FailedBy)
	}

	// History should have integration_failed entry
	found := false
	for _, h := range task.History {
		if h.Event == "integration_failed" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected integration_failed history entry")
	}

	// Worktree should be clean (rebase aborted) — verify by checking branch
	g := git.New(tmpDir)
	wtPath := g.GetWorktreePath(taskID)
	branch, err := g.GetWorktreeBranch(wtPath)
	if err != nil {
		t.Fatalf("failed to get worktree branch: %v", err)
	}
	if branch == "" {
		t.Error("worktree in detached HEAD state — rebase was not aborted")
	}

	// ReviewCommit should NOT be set
	if task.ReviewCommit != nil {
		t.Errorf("expected ReviewCommit nil, got %v", *task.ReviewCommit)
	}
}
