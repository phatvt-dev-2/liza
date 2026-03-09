//go:build e2e

package integration

// full_sprint_test.go contains an end-to-end integration test that exercises
// the full sprint pipeline (epic-planning → US-writing → code-planning → coding)
// using the real RunSupervisor loop with a mock CLI executor.
//
// The SmartMockCLIExecutor replaces the LLM CLI by calling ops.* functions
// directly to simulate what each agent role does, without any real LLM calls.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/agent"
	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/db"
	gitpkg "github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/identity"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/roles"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// ---------------------------------------------------------------------------
// SmartMockCLIExecutor — implements agent.CLIExecutor
// ---------------------------------------------------------------------------

// MockExecution records what the mock did for a single Execute call.
type MockExecution struct {
	AgentID string
	TaskID  string
	Role    string
	Action  string // "doer" or "reviewer"
}

// SmartMockCLIExecutor replaces the real LLM CLI. It reads the blackboard to
// find which task is assigned to the calling agent, then performs the expected
// ops calls (checkpoint, output, commit, submit/verdict) directly.
type SmartMockCLIExecutor struct {
	mu    sync.Mutex
	calls []MockExecution
}

func (m *SmartMockCLIExecutor) Execute(ctx context.Context, cliName, agentID, prompt, projectRoot string) (int, error) {
	runtimeRole, err := identity.ExtractRole(agentID)
	if err != nil {
		return 1, fmt.Errorf("extract role from %s: %w", agentID, err)
	}

	// Find the task assigned to this agent.
	// Doers use AssignedTo; reviewers use ReviewingBy.
	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	state, err := bb.Read()
	if err != nil {
		return 1, fmt.Errorf("read state: %w", err)
	}

	var taskID string
	isReviewer := roles.IsReviewerRole(runtimeRole)
	for i := range state.Tasks {
		task := &state.Tasks[i]
		if isReviewer {
			if task.ReviewingBy != nil && *task.ReviewingBy == agentID {
				taskID = task.ID
				break
			}
		} else {
			if task.AssignedTo != nil && *task.AssignedTo == agentID {
				taskID = task.ID
				break
			}
		}
	}
	if taskID == "" {
		return 1, fmt.Errorf("no task assigned to %s (reviewer=%v)", agentID, isReviewer)
	}

	if roles.IsDoerRole(runtimeRole) {
		if err := m.executeDoer(ctx, projectRoot, agentID, taskID, runtimeRole); err != nil {
			return 1, err
		}
	} else if roles.IsReviewerRole(runtimeRole) {
		if err := m.executeReviewer(projectRoot, agentID, taskID, runtimeRole); err != nil {
			return 1, err
		}
	} else {
		return 1, fmt.Errorf("unsupported role: %s", runtimeRole)
	}

	return 0, nil
}

func (m *SmartMockCLIExecutor) ExecuteInteractive(_ context.Context, _, _ string) (int, error) {
	return 0, fmt.Errorf("interactive mode not supported in mock")
}

// executeDoer simulates what a doer agent (planner or coder) does:
//  1. Write a pre-execution checkpoint
//  2. Set task output (planners only — needed for per-subtask transitions)
//  3. Create a file and commit in the worktree
//  4. Submit the task for review
func (m *SmartMockCLIExecutor) executeDoer(ctx context.Context, projectRoot, agentID, taskID, role string) error {
	// 1. Write checkpoint
	if err := ops.WriteCheckpoint(projectRoot, &ops.WriteCheckpointInput{
		TaskID:         taskID,
		AgentID:        agentID,
		Intent:         fmt.Sprintf("Mock %s work on %s", role, taskID),
		ValidationPlan: "mock validation passes",
		FilesToModify:  []string{"mock.txt"},
		TDDNotRequired: "integration test mock — no real code changes",
	}); err != nil {
		return fmt.Errorf("WriteCheckpoint: %w", err)
	}

	// 2. Set output for planner roles (needed for per-subtask transitions).
	// epic-planner and code-planner produce output[] entries that become child
	// tasks. us-writer uses a one-to-one transition, but setting output is
	// harmless and realistic.
	if role == roles.RuntimeEpicPlanner ||
		role == roles.RuntimeUSWriter ||
		role == roles.RuntimeCodePlanner {
		if err := ops.SetTaskOutput(projectRoot, &ops.SetTaskOutputInput{
			TaskID:  taskID,
			AgentID: agentID,
			Output: []models.OutputEntry{{
				Desc:     fmt.Sprintf("Child task from %s", taskID),
				DoneWhen: "Implementation complete",
				Scope:    "Full scope",
				SpecRef:  "specs/feature.md",
			}},
		}); err != nil {
			return fmt.Errorf("SetTaskOutput: %w", err)
		}
	}

	// 3. Create a file and commit in the worktree.
	g := gitpkg.New(projectRoot)
	wtPath := g.GetWorktreePath(taskID)

	mockFile := filepath.Join(wtPath, "mock.txt")
	content := fmt.Sprintf("Work by %s on %s at %s\n", agentID, taskID, time.Now().Format(time.RFC3339Nano))
	if err := os.WriteFile(mockFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("write mock file: %w", err)
	}

	if err := exec.CommandContext(ctx, "git", "-C", wtPath, "add", "mock.txt").Run(); err != nil {
		return fmt.Errorf("git add in worktree %s: %w", wtPath, err)
	}
	if err := exec.CommandContext(ctx, "git", "-C", wtPath, "commit", "-m", fmt.Sprintf("Mock work by %s", agentID)).Run(); err != nil {
		return fmt.Errorf("git commit in worktree %s: %w", wtPath, err)
	}

	// 4. Get HEAD SHA and submit for review.
	headOutput, err := exec.CommandContext(ctx, "git", "-C", wtPath, "rev-parse", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	commitSHA := strings.TrimSpace(string(headOutput))

	if _, err := ops.SubmitForReview(projectRoot, taskID, commitSHA, agentID); err != nil {
		return fmt.Errorf("SubmitForReview: %w", err)
	}

	// Record the call for assertions.
	m.mu.Lock()
	m.calls = append(m.calls, MockExecution{
		AgentID: agentID,
		TaskID:  taskID,
		Role:    role,
		Action:  "doer",
	})
	m.mu.Unlock()

	return nil
}

// executeReviewer simulates what a reviewer agent does: approve the task.
func (m *SmartMockCLIExecutor) executeReviewer(projectRoot, agentID, taskID, role string) error {
	if _, err := ops.SubmitVerdict(projectRoot, taskID, "APPROVED", "", agentID); err != nil {
		return fmt.Errorf("SubmitVerdict: %w", err)
	}

	m.mu.Lock()
	m.calls = append(m.calls, MockExecution{
		AgentID: agentID,
		TaskID:  taskID,
		Role:    role,
		Action:  "reviewer",
	})
	m.mu.Unlock()

	return nil
}

// ---------------------------------------------------------------------------
// TestFullSprintSequence
// ---------------------------------------------------------------------------

// TestFullSprintSequence runs the real RunSupervisor loop through a complete
// pipeline: epic-planning → US-writing → code-planning → coding.
//
// Each supervisor is run sequentially. Each claims one task, the mock executor
// does the work (checkpoint, commit, submit/verdict), the supervisor loops
// back, finds no more work, and exits after a short timeout.
//
// The test verifies:
//   - All 4 tasks reach MERGED status
//   - 3 child tasks are created by pipeline transitions
//   - Each mock CLI was called exactly once per role
func TestFullSprintSequence(t *testing.T) {
	// ── Setup ──────────────────────────────────────────────────────────

	// Clean any leftover db singletons from other tests.
	db.ResetInstances()
	t.Cleanup(db.ResetInstances)

	testhelpers.SetupGlobalLiza(t)

	projectDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectDir)

	// Save original CWD — InitCommandWithConfig resolves paths relative to CWD.
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	// Create spec file (required by AddTask validation).
	testhelpers.CreateSpecFile(t, projectDir, "feature.md", "# E2E Sprint Feature\nTest feature for full sprint sequence.")

	// Write the production pipeline.yaml to a temp location so
	// InitCommandWithConfig can read it.
	// go test CWD is internal/integration/; go up one level to reach internal/embedded/.
	pipelineSrc := filepath.Join(originalDir, "..", "embedded", "pipeline.yaml")
	pipelineData, err := os.ReadFile(pipelineSrc)
	if err != nil {
		t.Fatalf("Failed to read pipeline.yaml from %s: %v", pipelineSrc, err)
	}
	pipelineDst := filepath.Join(projectDir, "pipeline.yaml")
	if err := os.WriteFile(pipelineDst, pipelineData, 0644); err != nil {
		t.Fatalf("Failed to write pipeline config: %v", err)
	}

	// Initialize workspace with pipeline config.
	if err := commands.InitCommandWithConfig(commands.InitParams{
		Description: "E2E sprint test",
		SpecRef:     "specs/feature.md",
		ConfigPath:  pipelineDst,
		EntryPoint:  "general-objective",
	}); err != nil {
		t.Fatalf("InitCommandWithConfig failed: %v", err)
	}

	statePath := filepath.Join(projectDir, ".liza", "state.yaml")
	logPath := filepath.Join(projectDir, ".liza", "log.yaml")
	specsDir := filepath.Join(projectDir, "specs")

	// Add the initial epic-planning task.
	if _, err := ops.AddTask(statePath, logPath, &ops.AddTaskInput{
		ID:          "epic-1",
		Description: "Plan the epic feature",
		DoneWhen:    "Epic plan approved",
		Scope:       "Full feature",
		Priority:    1,
		SpecRef:     "specs/feature.md",
		RolePair:    "epic-planning-pair",
	}, "orchestrator-1"); err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}

	// Configure fast poll/wait times for quick test execution.
	bb := db.New(statePath)
	if err := bb.Modify(func(s *models.State) error {
		s.Config.CoderPollInterval = 1
		s.Config.CoderMaxWait = 5
		s.Config.ReviewerPollInterval = 1
		s.Config.ReviewerMaxWait = 5
		s.Config.OrchestratorPollInterval = 1
		s.Config.OrchestratorMaxWait = 3
		s.Config.LeaseDuration = 300 // 5 min — generous for test
		return nil
	}); err != nil {
		t.Fatalf("Failed to set config: %v", err)
	}

	mock := &SmartMockCLIExecutor{}

	// Helper: run a supervisor for a specific agent, blocking until it exits.
	runSupervisor := func(agentID, role string) {
		t.Helper()
		t.Logf("▶ Running supervisor: %s (role: %s)", agentID, role)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		cfg := agent.SupervisorConfig{
			AgentID:          agentID,
			Role:             role,
			ProjectRoot:      projectDir,
			StatePath:        statePath,
			LogPath:          logPath,
			SpecsDir:         specsDir,
			CLIName:          "claude",
			Executor:         mock,
			ExecutionTimeout: 60 * time.Second,
		}

		if err := agent.RunSupervisor(ctx, cfg); err != nil {
			t.Fatalf("RunSupervisor(%s) failed: %v", agentID, err)
		}
		t.Logf("  ✓ %s exited normally", agentID)
	}

	// ── Phase 1: Epic Planning ─────────────────────────────────────────
	t.Log("=== Phase 1: Epic Planning ===")
	runSupervisor("epic-planner-1", roles.RuntimeEpicPlanner)
	runSupervisor("epic-plan-reviewer-1", roles.RuntimeEpicPlanReviewer)

	// ── Phase 2: US Writing ────────────────────────────────────────────
	t.Log("=== Phase 2: US Writing ===")
	runSupervisor("us-writer-1", roles.RuntimeUSWriter)
	runSupervisor("us-reviewer-1", roles.RuntimeUSReviewer)

	// ── Phase 3: Code Planning ─────────────────────────────────────────
	t.Log("=== Phase 3: Code Planning ===")
	runSupervisor("code-planner-1", roles.RuntimeCodePlanner)
	runSupervisor("code-plan-reviewer-1", roles.RuntimeCodePlanReviewer)

	// ── Phase 4: Coding ────────────────────────────────────────────────
	t.Log("=== Phase 4: Coding ===")
	runSupervisor("coder-1", roles.RuntimeCoder)
	runSupervisor("code-reviewer-1", roles.RuntimeCodeReviewer)

	// ── Assertions ─────────────────────────────────────────────────────
	t.Log("=== Assertions ===")

	state, err := bb.Read()
	if err != nil {
		t.Fatal(err)
	}

	// Log all tasks for debugging.
	for _, task := range state.Tasks {
		t.Logf("  Task %-55s  status=%-25s  role_pair=%s", task.ID, task.Status, task.RolePair)
	}

	// Expected tasks (1 original + 3 created by transitions):
	//   epic-1                                                    (epic-planning-pair)
	//   epic-1-epic-to-us-0                                       (us-writing-pair)
	//   epic-1-epic-to-us-0-us-to-coding                          (code-planning-pair)
	//   epic-1-epic-to-us-0-us-to-coding-code-plan-to-coding-0    (coding-pair)
	if len(state.Tasks) != 4 {
		t.Errorf("Expected 4 tasks, got %d", len(state.Tasks))
	}

	// All tasks should be MERGED.
	mergedCount := 0
	for _, task := range state.Tasks {
		if task.Status == models.TaskStatusMerged {
			mergedCount++
		}
	}
	if mergedCount != 4 {
		t.Errorf("Expected 4 MERGED tasks, got %d", mergedCount)
	}

	// Verify transitions_executed on source tasks.
	assertTransitionExecuted(t, state, "epic-1", "epic-to-us")

	usTaskID := "epic-1-epic-to-us-0"
	assertTransitionExecuted(t, state, usTaskID, "us-to-coding")

	codePlanTaskID := usTaskID + "-us-to-coding"
	assertTransitionExecuted(t, state, codePlanTaskID, "code-plan-to-coding")

	// Verify mock call count and role coverage.
	mock.mu.Lock()
	defer mock.mu.Unlock()

	if len(mock.calls) != 8 {
		t.Errorf("Expected 8 mock calls, got %d", len(mock.calls))
		for i, call := range mock.calls {
			t.Logf("  Call %d: %s (%s) on %s [%s]", i, call.AgentID, call.Role, call.TaskID, call.Action)
		}
	}

	expectedRoles := []string{
		roles.RuntimeEpicPlanner, roles.RuntimeEpicPlanReviewer,
		roles.RuntimeUSWriter, roles.RuntimeUSReviewer,
		roles.RuntimeCodePlanner, roles.RuntimeCodePlanReviewer,
		roles.RuntimeCoder, roles.RuntimeCodeReviewer,
	}
	roleCounts := make(map[string]int)
	for _, call := range mock.calls {
		roleCounts[call.Role]++
	}
	for _, role := range expectedRoles {
		if roleCounts[role] != 1 {
			t.Errorf("Expected role %s called exactly once, got %d", role, roleCounts[role])
		}
	}
}

// assertTransitionExecuted verifies that the given task has the named
// transition in its transitions_executed map.
func assertTransitionExecuted(t *testing.T, state *models.State, taskID, transitionName string) {
	t.Helper()
	task := state.FindTask(taskID)
	if task == nil {
		t.Errorf("task %s not found", taskID)
		return
	}
	if !task.TransitionsExecuted[transitionName] {
		t.Errorf("task %s missing transition %s in transitions_executed", taskID, transitionName)
	}
}
