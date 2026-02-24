package ops

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestClaimTask_Validation(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		agentID     string
		errContains string
	}{
		{
			name:        "empty task ID",
			taskID:      "",
			agentID:     "coder-1",
			errContains: "task ID is required",
		},
		{
			name:        "empty agent ID",
			taskID:      "task-1",
			agentID:     "",
			errContains: "agent ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ClaimTask("/nonexistent", tt.taskID, tt.agentID)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Error = %q, want to contain %q", err.Error(), tt.errContains)
			}
		})
	}
}

func TestClaimTask_ReadyTask(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err != nil {
		t.Fatalf("ClaimTask() error: %v", err)
	}

	// Verify result fields
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
	if result.AgentID != "coder-1" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "coder-1")
	}
	if result.SourceStatus != models.TaskStatusReady {
		t.Errorf("SourceStatus = %v, want READY", result.SourceStatus)
	}
	if result.BaseCommit == "" {
		t.Error("BaseCommit should not be empty")
	}
	if result.IntegrationFix {
		t.Error("IntegrationFix should be false for READY task")
	}

	// Verify state updated
	readState := readClaimStateForTest(t, stateFile)
	task := readState.FindTask("task-1")
	if task == nil {
		t.Fatal("Task not found in state")
	}
	if task.Status != models.TaskStatusImplementing {
		t.Errorf("Task status = %v, want IMPLEMENTING", task.Status)
	}
	if task.AssignedTo == nil || *task.AssignedTo != "coder-1" {
		t.Error("AssignedTo should be coder-1")
	}
	if task.Iteration != 1 {
		t.Errorf("Iteration = %d, want 1", task.Iteration)
	}
	if task.Worktree == nil {
		t.Error("Worktree should be set")
	}

	// Verify worktree was created on disk
	wtDir := filepath.Join(tmpDir, ".worktrees", "task-1")
	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		t.Errorf("Worktree directory should exist at %s", wtDir)
	}

	// Verify agent registered
	agent, exists := readState.Agents["coder-1"]
	if !exists {
		t.Fatal("Agent not found in state")
	}
	if agent.CurrentTask == nil || *agent.CurrentTask != "task-1" {
		t.Error("Agent CurrentTask should be task-1")
	}
	if agent.Status != models.AgentStatusWorking {
		t.Errorf("Agent Status = %v, want working", agent.Status)
	}
}

func TestClaimTask_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ClaimTask(tmpDir, "nonexistent", "coder-1")
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestClaimTask_WrongStatus(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ClaimTask(tmpDir, "task-1", "coder-2")
	if err == nil {
		t.Fatal("Expected error for IMPLEMENTING task")
	}
	if !strings.Contains(err.Error(), "not READY") {
		t.Errorf("Error = %q, want to contain 'not READY'", err.Error())
	}
}

func TestClaimTask_AgentBusy(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	// Agent is busy with another task
	otherTask := "task-other"
	state.Agents["coder-1"] = models.Agent{
		Status:      models.AgentStatusWorking,
		CurrentTask: &otherTask,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err == nil {
		t.Fatal("Expected error for busy agent")
	}
	if !strings.Contains(err.Error(), "already working") {
		t.Errorf("Error = %q, want to contain 'already working'", err.Error())
	}
}

func TestClaimTask_UnmetDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	depTask := testhelpers.BuildTaskByStatus("dep-1", models.TaskStatusReady, now)
	mainTask := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	mainTask.DependsOn = []string{"dep-1"}
	state.Tasks = []models.Task{depTask, mainTask}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err == nil {
		t.Fatal("Expected error for unmet dependencies")
	}
	if !strings.Contains(err.Error(), "unmet dependencies") {
		t.Errorf("Error = %q, want to contain 'unmet dependencies'", err.Error())
	}
}

func TestClaimTask_MetDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	depTask := testhelpers.BuildTaskByStatus("dep-1", models.TaskStatusMerged, now)
	mainTask := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	mainTask.DependsOn = []string{"dep-1"}
	state.Tasks = []models.Task{depTask, mainTask}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err != nil {
		t.Fatalf("ClaimTask() error: %v", err)
	}
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
}

func TestUnmetDependencies(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name      string
		dependsOn []string
		tasks     []models.Task
		want      []string
	}{
		{
			name:      "no dependencies",
			dependsOn: nil,
			tasks:     nil,
			want:      nil,
		},
		{
			name:      "all dependencies merged",
			dependsOn: []string{"dep-1", "dep-2"},
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("dep-1", models.TaskStatusMerged, now),
				testhelpers.BuildTaskByStatus("dep-2", models.TaskStatusMerged, now),
			},
			want: nil,
		},
		{
			name:      "includes missing and non-merged dependencies",
			dependsOn: []string{"dep-1", "dep-missing", "dep-2"},
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("dep-1", models.TaskStatusMerged, now),
				testhelpers.BuildTaskByStatus("dep-2", models.TaskStatusReady, now),
			},
			want: []string{"dep-missing", "dep-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := testhelpers.CreateValidState()
			state.Tasks = append([]models.Task(nil), tt.tasks...)

			task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
			task.DependsOn = tt.dependsOn

			got := unmetDependencies(&task, state)
			if !slices.Equal(got, tt.want) {
				t.Errorf("unmetDependencies() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClaimTask_IntegrationFailed(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now)
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	// Create the worktree directory that IntegrationFailed expects to exist
	wtDir := filepath.Join(tmpDir, ".worktrees", "task-1")
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		t.Fatalf("Failed to create worktree dir: %v", err)
	}

	result, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err != nil {
		t.Fatalf("ClaimTask() error: %v", err)
	}

	if !result.IntegrationFix {
		t.Error("IntegrationFix should be true for INTEGRATION_FAILED task")
	}
	if result.SourceStatus != models.TaskStatusIntegrationFailed {
		t.Errorf("SourceStatus = %v, want INTEGRATION_FAILED", result.SourceStatus)
	}

	// Verify task state
	readState := readClaimStateForTest(t, stateFile)
	claimedTask := readState.FindTask("task-1")
	if claimedTask == nil {
		t.Fatal("Task not found")
	}
	if !claimedTask.IntegrationFix {
		t.Error("IntegrationFix flag should be set in state")
	}
}

func TestClaimTask_RejectedSameCoder(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
	// task.AssignedTo is already "coder-1" from BuildTaskByStatus
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	// Create the real git worktree/branch that same-coder reclaim expects.
	gitWrapper := git.New(tmpDir)
	if _, err := gitWrapper.CreateWorktree("task-1", "integration"); err != nil {
		t.Fatalf("Failed to create initial rejected worktree: %v", err)
	}

	result, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err != nil {
		t.Fatalf("ClaimTask() error: %v", err)
	}

	if result.PreviousAssignee != "coder-1" {
		t.Errorf("PreviousAssignee = %q, want %q", result.PreviousAssignee, "coder-1")
	}
	if result.WorktreeRecreated {
		t.Error("WorktreeRecreated should be false for same coder reclaim")
	}
}

func TestClaimTask_RejectedDifferentCoderReassignmentRecreatesValidWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	gitWrapper := git.New(tmpDir)
	if _, err := gitWrapper.CreateWorktree("task-1", "integration"); err != nil {
		t.Fatalf("Failed to create initial rejected worktree: %v", err)
	}

	wtDir := filepath.Join(tmpDir, ".worktrees", "task-1")
	changesFile := filepath.Join(wtDir, "rejected-change.txt")
	if err := os.WriteFile(changesFile, []byte("rejected change\n"), 0644); err != nil {
		t.Fatalf("Failed to write rejected-change.txt: %v", err)
	}
	if err := exec.Command("git", "-C", wtDir, "add", "rejected-change.txt").Run(); err != nil {
		t.Fatalf("Failed to add rejected worktree file: %v", err)
	}
	if err := exec.Command("git", "-C", wtDir, "commit", "-m", "Rejected work").Run(); err != nil {
		t.Fatalf("Failed to commit rejected worktree file: %v", err)
	}

	oldBranchSHA, err := gitWrapper.GetCommitSHA("task/task-1")
	if err != nil {
		t.Fatalf("Failed to read old task branch SHA: %v", err)
	}
	integrationSHA, err := gitWrapper.GetCommitSHA("integration")
	if err != nil {
		t.Fatalf("Failed to read integration SHA: %v", err)
	}
	if oldBranchSHA == integrationSHA {
		t.Fatal("Expected rejected task branch to diverge from integration before reassignment")
	}

	result, err := ClaimTask(tmpDir, "task-1", "coder-2")
	if err != nil {
		t.Fatalf("ClaimTask() error: %v", err)
	}

	if result.PreviousAssignee != "coder-1" {
		t.Errorf("PreviousAssignee = %q, want %q", result.PreviousAssignee, "coder-1")
	}
	if !result.WorktreeRecreated {
		t.Error("WorktreeRecreated should be true for different coder reassignment")
	}

	readState := readClaimStateForTest(t, stateFile)
	claimedTask := readState.FindTask("task-1")
	if claimedTask == nil {
		t.Fatal("Task not found in state")
	}
	if claimedTask.Status != models.TaskStatusImplementing {
		t.Errorf("Task status = %v, want IMPLEMENTING", claimedTask.Status)
	}
	if claimedTask.AssignedTo == nil || *claimedTask.AssignedTo != "coder-2" {
		t.Errorf("AssignedTo = %v, want coder-2", claimedTask.AssignedTo)
	}

	currentTaskBranchSHA, err := gitWrapper.GetCommitSHA("task/task-1")
	if err != nil {
		t.Fatalf("Failed to read reassigned task branch SHA: %v", err)
	}
	if currentTaskBranchSHA != result.BaseCommit {
		t.Errorf("Task branch SHA = %s, want result.BaseCommit %s", currentTaskBranchSHA, result.BaseCommit)
	}
	if currentTaskBranchSHA != integrationSHA {
		t.Errorf("Task branch SHA = %s, want integration SHA %s", currentTaskBranchSHA, integrationSHA)
	}
	if currentTaskBranchSHA == oldBranchSHA {
		t.Error("Task branch should not retain previous rejected coder commit after reassignment")
	}

	worktreeHead, err := gitWrapper.GetWorktreeHEAD("task-1")
	if err != nil {
		t.Fatalf("Expected valid reassigned worktree HEAD, got error: %v", err)
	}
	if worktreeHead != currentTaskBranchSHA {
		t.Errorf("Worktree HEAD = %s, want %s", worktreeHead, currentTaskBranchSHA)
	}
}

func TestClaimTask_RejectedDifferentCoderCreateFailureRestoresValidPreviousWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	// Force replacement creation to fail after teardown: this ref exists before teardown,
	// but is deleted with the old task branch/worktree during reassignment.
	state.Config.IntegrationBranch = "task/task-1"
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	gitWrapper := git.New(tmpDir)
	if _, err := gitWrapper.CreateWorktree("task-1", "integration"); err != nil {
		t.Fatalf("Failed to create initial rejected worktree: %v", err)
	}
	originalTaskBranchSHA, err := gitWrapper.GetCommitSHA("task/task-1")
	if err != nil {
		t.Fatalf("Failed to read original task branch SHA: %v", err)
	}

	_, err = ClaimTask(tmpDir, "task-1", "coder-2")
	if err == nil {
		t.Fatal("Expected claim to fail when replacement create ref disappears")
	}
	if !strings.Contains(err.Error(), "failed to create replacement worktree (previous worktree restored)") {
		t.Errorf("Error = %q, want to contain 'failed to create replacement worktree (previous worktree restored)'", err.Error())
	}

	readState := readClaimStateForTest(t, stateFile)
	taskAfterFailure := readState.FindTask("task-1")
	if taskAfterFailure == nil {
		t.Fatal("Task not found in state")
	}
	if taskAfterFailure.Status != models.TaskStatusRejected {
		t.Errorf("Task status = %v, want REJECTED", taskAfterFailure.Status)
	}
	if taskAfterFailure.AssignedTo == nil || *taskAfterFailure.AssignedTo != "coder-1" {
		t.Errorf("AssignedTo = %v, want coder-1", taskAfterFailure.AssignedTo)
	}

	restoredTaskBranchSHA, err := gitWrapper.GetCommitSHA("task/task-1")
	if err != nil {
		t.Fatalf("Expected task branch to be restored, got error: %v", err)
	}
	if restoredTaskBranchSHA != originalTaskBranchSHA {
		t.Errorf("Restored task branch SHA = %s, want %s", restoredTaskBranchSHA, originalTaskBranchSHA)
	}

	restoredWorktreeHead, err := gitWrapper.GetWorktreeHEAD("task-1")
	if err != nil {
		t.Fatalf("Expected restored worktree HEAD, got error: %v", err)
	}
	if restoredWorktreeHead != restoredTaskBranchSHA {
		t.Errorf("Restored worktree HEAD = %s, want %s", restoredWorktreeHead, restoredTaskBranchSHA)
	}
}

func TestClaimTask_RejectedAtIterationLimitTransitionsToBlocked(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Config.MaxCoderIterations = 3

	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
	task.Iteration = 3
	state.Tasks = []models.Task{task}

	taskRef := "task-1"
	state.Agents["coder-1"] = models.Agent{
		Role:        "coder",
		Status:      models.AgentStatusWaiting,
		CurrentTask: &taskRef,
	}

	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err == nil {
		t.Fatal("Expected iteration-limit error")
	}
	if !strings.Contains(err.Error(), "transitioned to BLOCKED") {
		t.Errorf("Error = %q, want to contain 'transitioned to BLOCKED'", err.Error())
	}

	readState := readClaimStateForTest(t, stateFile)
	blockedTask := readState.FindTask("task-1")
	if blockedTask == nil {
		t.Fatal("Task not found in state")
	}
	if blockedTask.Status != models.TaskStatusBlocked {
		t.Errorf("Task status = %v, want BLOCKED", blockedTask.Status)
	}
	if blockedTask.AssignedTo != nil {
		t.Error("AssignedTo should be cleared when task is blocked")
	}
	if blockedTask.BlockedReason == nil || !strings.Contains(*blockedTask.BlockedReason, "max iterations") {
		t.Errorf("BlockedReason = %v, want max-iterations reason", blockedTask.BlockedReason)
	}
	if len(blockedTask.BlockedQuestions) == 0 {
		t.Error("BlockedQuestions should be populated")
	}

	agent := readState.Agents["coder-1"]
	if agent.Status != models.AgentStatusIdle {
		t.Errorf("Agent status = %v, want IDLE", agent.Status)
	}
	if agent.CurrentTask != nil {
		t.Error("Agent CurrentTask should be cleared after limit-based block")
	}
}

func TestClaimTask_ReadyWithStaleBranchAndWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	// Create a stale worktree and branch (simulating orphaned resources from
	// a previous claim that was released without cleanup).
	gitWrapper := git.New(tmpDir)
	if _, err := gitWrapper.CreateWorktree("task-1", "integration"); err != nil {
		t.Fatalf("Failed to create stale worktree: %v", err)
	}

	// Verify stale resources exist before claim
	wtDir := filepath.Join(tmpDir, ".worktrees", "task-1")
	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		t.Fatal("Stale worktree should exist before claim")
	}
	branchExists, err := gitWrapper.BranchExists("task/task-1")
	if err != nil {
		t.Fatalf("Failed to check branch: %v", err)
	}
	if !branchExists {
		t.Fatal("Stale branch should exist before claim")
	}

	// Claim should succeed despite stale resources
	result, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err != nil {
		t.Fatalf("ClaimTask() error: %v", err)
	}

	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
	if result.SourceStatus != models.TaskStatusReady {
		t.Errorf("SourceStatus = %v, want READY", result.SourceStatus)
	}

	// Verify worktree exists (freshly created)
	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		t.Error("Worktree should exist after successful claim")
	}

	// Verify state is correct
	readState := readClaimStateForTest(t, stateFile)
	task := readState.FindTask("task-1")
	if task == nil {
		t.Fatal("Task not found")
	}
	if task.Status != models.TaskStatusImplementing {
		t.Errorf("Status = %v, want IMPLEMENTING", task.Status)
	}
	if task.Worktree == nil {
		t.Error("Worktree should be set")
	}
}

// readClaimStateForTest reads state for claim test verification.
func readClaimStateForTest(t *testing.T, stateFile string) *models.State {
	t.Helper()
	bb := db.New(stateFile)
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	return state
}
