package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// setupPipelineTest creates a test directory with a frozen pipeline config and a valid state.
// Returns (projectRoot, stateFile) paths.
func setupPipelineTest(t *testing.T) (string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	// Copy the valid pipeline YAML to .liza/pipeline.yaml (frozen config).
	src, err := os.ReadFile(filepath.Join(findRepoRoot(t), "internal", "pipeline", "testdata", "valid-coding-subpipeline.yaml"))
	if err != nil {
		t.Fatalf("Failed to read pipeline testdata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".liza", "pipeline.yaml"), src, 0644); err != nil {
		t.Fatalf("Failed to write frozen pipeline config: %v", err)
	}

	return tmpDir, stateFile
}

// findRepoRoot walks up from the current working dir to find the repo root.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (go.mod)")
		}
		dir = parent
	}
}

func TestLoadResolver_PipelineGoal(t *testing.T) {
	tmpDir, _ := setupPipelineTest(t)

	resolver, err := loadResolver(tmpDir)
	if err != nil {
		t.Fatalf("loadResolver() error: %v", err)
	}
	if resolver == nil {
		t.Fatal("expected non-nil resolver for pipeline goal")
	}
}

func TestLoadResolver_LegacyGoal(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	testhelpers.SetupLizaDir(t, tmpDir)
	// No pipeline.yaml → legacy goal

	resolver, err := loadResolver(tmpDir)
	if err != nil {
		t.Fatalf("loadResolver() error: %v", err)
	}
	if resolver != nil {
		t.Fatal("expected nil resolver for legacy goal")
	}
}

// --- ClaimTask pipeline tests ---

func TestClaimTask_PipelineCodingPair(t *testing.T) {
	tmpDir, stateFile := setupPipelineTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	task := models.Task{
		ID:          "task-1",
		Type:        models.TaskTypeCoding,
		RolePair:    "coding-pair",
		Description: "Pipeline coding task",
		Status:      models.TaskStatus("DRAFT_CODE"),
		Priority:    1,
		Created:     now,
		SpecRef:     "README.md",
		DoneWhen:    "done",
		Scope:       "scope",
		History:     []models.TaskHistoryEntry{},
	}
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{"task-1"}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err != nil {
		t.Fatalf("ClaimTask() error: %v", err)
	}
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}

	// Verify the task transitioned to the pipeline executing state, not hardcoded IMPLEMENTING
	bb := db.For(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	readTask := readState.FindTask("task-1")
	if readTask == nil {
		t.Fatal("Task not found")
	}
	if readTask.Status != models.TaskStatus("IMPLEMENTING_CODE") {
		t.Errorf("Task status = %v, want IMPLEMENTING_CODE", readTask.Status)
	}
}

func TestClaimTask_PipelineCodePlanningPair(t *testing.T) {
	tmpDir, stateFile := setupPipelineTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2
	task := models.Task{
		ID:          "plan-1",
		Type:        models.TaskTypeCoding,
		RolePair:    "code-planning-pair",
		Description: "Pipeline planning task",
		Status:      models.TaskStatus("DRAFT_CODING_PLAN"),
		Priority:    1,
		Created:     now,
		SpecRef:     "README.md",
		DoneWhen:    "done",
		Scope:       "scope",
		History:     []models.TaskHistoryEntry{},
	}
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{"plan-1"}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ClaimTask(tmpDir, "plan-1", "code-planner-1")
	if err != nil {
		t.Fatalf("ClaimTask() error: %v", err)
	}
	if result.TaskID != "plan-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "plan-1")
	}

	// Verify pipeline executing state
	bb := db.For(stateFile)
	readState, _ := bb.Read()
	readTask := readState.FindTask("plan-1")
	if readTask.Status != models.TaskStatus("CODE_PLANNING") {
		t.Errorf("Task status = %v, want CODE_PLANNING", readTask.Status)
	}
}

func TestClaimTask_LegacyGoalStillWorks(t *testing.T) {
	// No pipeline.yaml → legacy path
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	state.Sprint.Scope.Planned = []string{"task-1"}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ClaimTask(tmpDir, "task-1", "coder-1")
	if err != nil {
		t.Fatalf("ClaimTask() error: %v", err)
	}
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}

	bb := db.For(stateFile)
	readState, _ := bb.Read()
	readTask := readState.FindTask("task-1")
	if readTask.Status != models.TaskStatusImplementing {
		t.Errorf("Legacy task status = %v, want IMPLEMENTING", readTask.Status)
	}
}

// --- AddTask pipeline tests ---

func TestInitialTaskStatus_PipelineGoal(t *testing.T) {
	tmpDir, _ := setupPipelineTest(t)

	resolver, err := loadResolver(tmpDir)
	if err != nil {
		t.Fatalf("loadResolver error: %v", err)
	}

	// Pipeline goal: coding-pair → DRAFT_CODE
	status := initialTaskStatusWithResolver("coding-pair", resolver)
	if status != models.TaskStatus("DRAFT_CODE") {
		t.Errorf("initialTaskStatus(coding-pair) = %v, want DRAFT_CODE", status)
	}

	// Pipeline goal: code-planning-pair → DRAFT_CODING_PLAN
	status = initialTaskStatusWithResolver("code-planning-pair", resolver)
	if status != models.TaskStatus("DRAFT_CODING_PLAN") {
		t.Errorf("initialTaskStatus(code-planning-pair) = %v, want DRAFT_CODING_PLAN", status)
	}
}

func TestInitialTaskStatus_LegacyGoal(t *testing.T) {
	// Legacy: no resolver
	status := initialTaskStatusWithResolver("", nil)
	if status != models.TaskStatusReady {
		t.Errorf("initialTaskStatus('', nil) = %v, want READY", status)
	}

	// Legacy: code-planning-pair without resolver
	status = initialTaskStatusWithResolver("code-planning-pair", nil)
	if status != models.TaskStatusDraftCodingPlan {
		t.Errorf("initialTaskStatus(code-planning-pair, nil) = %v, want DRAFT_CODING_PLAN", status)
	}
}

// --- SubmitForReview pipeline tests ---
// (These require git worktrees, so we test the status resolution logic)

func TestSubmitForReview_PipelineCodingPairStatusCheck(t *testing.T) {
	tmpDir, stateFile := setupPipelineTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2

	agent := "coder-1"
	leaseExpires := now.Add(30 * time.Minute)
	baseCommit := "abc1234"
	worktree := ".worktrees/task-1"
	task := models.Task{
		ID:           "task-1",
		Type:         models.TaskTypeCoding,
		RolePair:     "coding-pair",
		Description:  "Pipeline coding task",
		Status:       models.TaskStatus("IMPLEMENTING_CODE"),
		Priority:     1,
		AssignedTo:   &agent,
		LeaseExpires: &leaseExpires,
		BaseCommit:   &baseCommit,
		Worktree:     &worktree,
		Created:      now,
		SpecRef:      "README.md",
		DoneWhen:     "done",
		Scope:        "scope",
		History: []models.TaskHistoryEntry{
			{Time: now, Event: "pre_execution_checkpoint", Agent: &agent},
		},
	}
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{"task-1"}
	testhelpers.WriteInitialState(t, stateFile, state)

	// SubmitForReview will fail because worktree doesn't exist as git worktree,
	// but the status check should pass (it checks IMPLEMENTING_CODE, not IMPLEMENTING)
	_, err := SubmitForReview(tmpDir, "task-1", "abc1234", "coder-1")
	if err == nil {
		t.Fatal("Expected error (worktree not real git worktree)")
	}
	// The error should be about the worktree, not about wrong status
	if strings.Contains(err.Error(), "not IMPLEMENTING") {
		t.Errorf("Should accept IMPLEMENTING_CODE as valid executing status, got: %v", err)
	}
}

// --- SubmitVerdict pipeline tests ---

func TestSubmitVerdict_PipelineApproved(t *testing.T) {
	tmpDir, stateFile := setupPipelineTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2

	reviewingBy := "code-plan-reviewer-1"
	reviewLeaseExpires := now.Add(30 * time.Minute)
	reviewCommit := "review123"
	agent := "code-planner-1"
	worktree := ".worktrees/plan-1"
	task := models.Task{
		ID:                 "plan-1",
		Type:               models.TaskTypeCoding,
		RolePair:           "code-planning-pair",
		Description:        "Pipeline planning task",
		Status:             models.TaskStatus("REVIEWING_CODING_PLAN"),
		Priority:           1,
		AssignedTo:         &agent,
		ReviewingBy:        &reviewingBy,
		ReviewLeaseExpires: &reviewLeaseExpires,
		ReviewCommit:       &reviewCommit,
		Worktree:           &worktree,
		Created:            now,
		SpecRef:            "README.md",
		DoneWhen:           "done",
		Scope:              "scope",
		History:            []models.TaskHistoryEntry{},
	}
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{"plan-1"}
	state.Agents = map[string]models.Agent{
		"code-plan-reviewer-1": {
			Role:   "code-plan-reviewer",
			Status: models.AgentStatusReviewing,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SubmitVerdict(tmpDir, "plan-1", "APPROVED", "", "code-plan-reviewer-1")
	if err != nil {
		t.Fatalf("SubmitVerdict() error: %v", err)
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("Verdict = %q, want APPROVED", result.Verdict)
	}

	bb := db.For(stateFile)
	readState, _ := bb.Read()
	readTask := readState.FindTask("plan-1")
	if readTask.Status != models.TaskStatus("CODING_PLAN_APPROVED") {
		t.Errorf("Task status = %v, want CODING_PLAN_APPROVED", readTask.Status)
	}
}

func TestSubmitVerdict_PipelineCodingPairApproved(t *testing.T) {
	tmpDir, stateFile := setupPipelineTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2

	reviewingBy := "code-reviewer-1"
	reviewLeaseExpires := now.Add(30 * time.Minute)
	reviewCommit := "review123"
	agent := "coder-1"
	worktree := ".worktrees/task-1"
	task := models.Task{
		ID:                 "task-1",
		Type:               models.TaskTypeCoding,
		RolePair:           "coding-pair",
		Description:        "Pipeline coding task",
		Status:             models.TaskStatus("REVIEWING_CODE"),
		Priority:           1,
		AssignedTo:         &agent,
		ReviewingBy:        &reviewingBy,
		ReviewLeaseExpires: &reviewLeaseExpires,
		ReviewCommit:       &reviewCommit,
		Worktree:           &worktree,
		Created:            now,
		SpecRef:            "README.md",
		DoneWhen:           "done",
		Scope:              "scope",
		History:            []models.TaskHistoryEntry{},
	}
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{"task-1"}
	state.Agents = map[string]models.Agent{
		"code-reviewer-1": {
			Role:   "code-reviewer",
			Status: models.AgentStatusReviewing,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SubmitVerdict(tmpDir, "task-1", "APPROVED", "", "code-reviewer-1")
	if err != nil {
		t.Fatalf("SubmitVerdict() error: %v", err)
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("Verdict = %q, want APPROVED", result.Verdict)
	}

	bb := db.For(stateFile)
	readState, _ := bb.Read()
	readTask := readState.FindTask("task-1")
	if readTask.Status != models.TaskStatus("CODE_APPROVED") {
		t.Errorf("Task status = %v, want CODE_APPROVED", readTask.Status)
	}
}

// --- ResumeHandoff pipeline tests ---

func TestResumeHandoff_PipelineExecutingState(t *testing.T) {
	tmpDir, stateFile := setupPipelineTest(t)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.PipelineVersion = 2

	agent := "coder-1"
	leaseExpires := now.Add(30 * time.Minute)
	worktree := ".worktrees/task-1"
	baseCommit := "abc1234"
	task := models.Task{
		ID:             "task-1",
		Type:           models.TaskTypeCoding,
		RolePair:       "coding-pair",
		Description:    "Pipeline coding task",
		Status:         models.TaskStatus("IMPLEMENTING_CODE"),
		Priority:       1,
		AssignedTo:     &agent,
		LeaseExpires:   &leaseExpires,
		BaseCommit:     &baseCommit,
		Worktree:       &worktree,
		HandoffPending: true,
		Created:        now,
		SpecRef:        "README.md",
		DoneWhen:       "done",
		Scope:          "scope",
		History:        []models.TaskHistoryEntry{},
	}
	state.Tasks = []models.Task{task}
	state.Sprint.Scope.Planned = []string{"task-1"}
	state.Agents = map[string]models.Agent{
		"coder-1": {
			Role:         "coder",
			Status:       models.AgentStatusHandoff,
			CurrentTask:  &task.ID,
			LeaseExpires: &leaseExpires,
			Heartbeat:    now,
		},
	}

	// Create the worktree directory on disk
	testhelpers.CreateTestWorktree(t, tmpDir, "task-1")

	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ResumeHandoff(ResumeHandoffInput{
		ProjectRoot: tmpDir,
		AgentID:     "coder-1",
	})
	if err != nil {
		t.Fatalf("ResumeHandoff() error: %v", err)
	}
	if !result.Found {
		t.Fatal("Expected to find resumable handoff")
	}
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
}
