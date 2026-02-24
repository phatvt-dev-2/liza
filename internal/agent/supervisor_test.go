package agent

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// MockCLIExecutor for testing CLI execution
type MockCLIExecutor struct {
	mu               sync.Mutex
	Calls            []MockCLICall
	InteractiveCalls []MockCLICall
	ExitCode         int
	ExitError        error
}

type MockCLICall struct {
	CLIName string
	Prompt  string
}

func (m *MockCLIExecutor) Execute(ctx context.Context, cliName string, prompt string, projectRoot string) (int, error) {
	m.mu.Lock()
	m.Calls = append(m.Calls, MockCLICall{CLIName: cliName, Prompt: prompt})
	m.mu.Unlock()
	return m.ExitCode, m.ExitError
}

func (m *MockCLIExecutor) ExecuteInteractive(ctx context.Context, cliName string, projectRoot string) (int, error) {
	m.mu.Lock()
	m.InteractiveCalls = append(m.InteractiveCalls, MockCLICall{CLIName: cliName})
	m.mu.Unlock()
	return m.ExitCode, m.ExitError
}

// GetCalls returns a copy of the calls slice in a thread-safe manner
func (m *MockCLIExecutor) GetCalls() []MockCLICall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]MockCLICall, len(m.Calls))
	copy(calls, m.Calls)
	return calls
}

// GetInteractiveCalls returns a copy of the interactive calls slice in a thread-safe manner
func (m *MockCLIExecutor) GetInteractiveCalls() []MockCLICall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]MockCLICall, len(m.InteractiveCalls))
	copy(calls, m.InteractiveCalls)
	return calls
}

// TestMockCLIExecution tests CLI executor mock
func TestMockCLIExecution(t *testing.T) {
	mock := &MockCLIExecutor{
		ExitCode: 0,
	}

	ctx := context.Background()
	exitCode, err := mock.Execute(ctx, "claude", "test prompt", "/tmp/test-project")

	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}

	if exitCode != 0 {
		t.Errorf("Execute() exitCode = %d, want 0", exitCode)
	}

	calls := mock.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(calls))
	}

	call := calls[0]
	if call.CLIName != "claude" {
		t.Errorf("CLIName = %s, want claude", call.CLIName)
	}
	if call.Prompt != "test prompt" {
		t.Errorf("Prompt = %s, want 'test prompt'", call.Prompt)
	}
}

// TestSupervisorBasicLoop tests basic supervisor operation
// Uses planner role to avoid git repository requirements
func TestSupervisorBasicLoop(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	// No tasks initially - planner will detect INITIAL_PLANNING trigger
	state.Tasks = []models.Task{}
	// Set short poll intervals for fast test
	state.Config.CoderPollInterval = 1 // 1 second
	state.Config.CoderMaxWait = 5      // 5 seconds

	testhelpers.WriteInitialState(t, statePath, state)

	mock := &MockCLIExecutor{
		ExitCode: 0, // Exit successfully after first iteration
	}

	config := SupervisorConfig{
		AgentID:     "planner-1",
		Role:        "planner",
		ProjectRoot: tmpDir,
		StatePath:   statePath,
		SpecsDir:    filepath.Join(tmpDir, "specs"),
		CLIName:     "claude",
		Executor:    mock,
	}

	// Create required directories
	os.MkdirAll(config.SpecsDir, 0755)

	// Set STOPPED mode after first execution to exit the loop
	bb := db.New(statePath)
	go func() {
		// Wait for first execution
		waitTicker := time.NewTicker(10 * time.Millisecond)
		defer waitTicker.Stop()
		for len(mock.GetCalls()) == 0 {
			<-waitTicker.C
		}
		// Set STOPPED mode
		bb.Modify(func(s *models.State) error {
			s.Config.Mode = models.SystemModeStopped
			return nil
		})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := RunSupervisor(ctx, config)

	if err != nil && err != context.DeadlineExceeded {
		t.Errorf("RunSupervisor() error = %v", err)
	}

	// Verify agent was registered and unregistered
	finalState, err := db.New(statePath).Read()
	if err != nil {
		t.Fatalf("Failed to read final state: %v", err)
	}

	if _, exists := finalState.Agents[config.AgentID]; exists {
		t.Error("Agent should be unregistered after exit")
	}
}

// TestInteractiveMode tests that interactive mode launches CLI interactively (no -p)
func TestInteractiveMode(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, statePath, state)

	mock := &MockCLIExecutor{}

	config := SupervisorConfig{
		AgentID:     "planner-1",
		Role:        "planner",
		ProjectRoot: tmpDir,
		StatePath:   statePath,
		SpecsDir:    filepath.Join(tmpDir, "specs"),
		CLIName:     "claude",
		Interactive: true,
		Executor:    mock,
	}

	os.MkdirAll(config.SpecsDir, 0755)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// In interactive mode, should launch CLI via ExecuteInteractive (not Execute)
	err := RunSupervisor(ctx, config)

	// Should exit cleanly (no error) or timeout waiting for next planner wake
	if err != nil && err != context.DeadlineExceeded {
		t.Errorf("RunSupervisor() error = %v, want nil or DeadlineExceeded", err)
	}

	if len(mock.GetCalls()) > 0 {
		t.Error("Interactive mode should not call Execute (non-interactive)")
	}

	interactiveCalls := mock.GetInteractiveCalls()
	if len(interactiveCalls) == 0 {
		t.Error("Interactive mode should call ExecuteInteractive")
	} else if interactiveCalls[0].CLIName != "claude" {
		t.Errorf("ExecuteInteractive called with CLI %q, want %q", interactiveCalls[0].CLIName, "claude")
	}
}

// TestSupervisorAbortsQuickly tests end-to-end ABORT behavior
func TestSupervisorAbortsQuickly(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{} // No work for coder
	state.Config.CoderPollInterval = 1
	state.Config.CoderMaxWait = 1800 // 30 minutes
	testhelpers.WriteInitialState(t, statePath, state)

	mock := &MockCLIExecutor{ExitCode: 0}

	config := SupervisorConfig{
		AgentID:     "coder-1",
		Role:        "coder",
		ProjectRoot: tmpDir,
		StatePath:   statePath,
		SpecsDir:    filepath.Join(tmpDir, "specs"),
		CLIName:     "claude",
		Executor:    mock,
	}

	os.MkdirAll(config.SpecsDir, 0755)

	// Send ABORT signal after supervisor has registered itself.
	go func() {
		stopSystem := func() {
			if err := db.New(statePath).Modify(func(s *models.State) error {
				s.Config.Mode = models.SystemModeStopped
				return nil
			}); err != nil {
				t.Logf("Failed to set STOPPED mode: %v", err)
			}
		}

		waitTicker := time.NewTicker(10 * time.Millisecond)
		defer waitTicker.Stop()
		waitDeadline := time.After(2 * time.Second)

		for {
			select {
			case <-waitTicker.C:
				snapshot, err := db.New(statePath).Read()
				if err == nil {
					if _, exists := snapshot.Agents[config.AgentID]; exists {
						stopSystem()
						return
					}
				}
			case <-waitDeadline:
				stopSystem()
				return
			}
		}
	}()

	ctx := context.Background()
	startTime := time.Now()

	err := RunSupervisor(ctx, config)
	elapsed := time.Since(startTime)

	if err != nil {
		t.Errorf("RunSupervisor() error = %v", err)
	}

	// Should exit within 7 seconds (1s delay + 5s ticker + margin)
	if elapsed > 7*time.Second {
		t.Errorf("Supervisor took %v to exit, expected < 7s", elapsed)
	}

	// Verify no CLI execution happened (no work available for coder)
	if len(mock.GetCalls()) > 0 {
		t.Error("CLI should not be executed when ABORT is sent before work")
	}
}

func TestResumeHandoffTask_SuccessRenewsLeaseAndUpdatesAgent(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	agentID := "coder-1"
	taskID := "task-1"
	worktree := ".worktrees/task-1"
	expiredLease := time.Now().UTC().Add(-2 * time.Minute)

	state := testhelpers.CreateValidState()
	state.Config.LeaseDuration = 120
	state.Tasks = []models.Task{
		{
			ID:             taskID,
			Status:         models.TaskStatusImplementing,
			AssignedTo:     &agentID,
			Worktree:       &worktree,
			HandoffPending: true,
			LeaseExpires:   &expiredLease,
			History:        []models.TaskHistoryEntry{},
		},
	}

	bb := testhelpers.WriteInitialState(t, statePath, state)
	snapshot, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read snapshot: %v", err)
	}

	callStart := time.Now().UTC()
	gotTaskID, gotWorktree, found, err := resumeHandoffTask(bb, snapshot, agentID)
	if err != nil {
		t.Fatalf("resumeHandoffTask() error = %v", err)
	}
	if !found {
		t.Fatal("resumeHandoffTask() found = false, want true")
	}
	if gotTaskID != taskID {
		t.Fatalf("resumeHandoffTask() taskID = %q, want %q", gotTaskID, taskID)
	}
	if gotWorktree != worktree {
		t.Fatalf("resumeHandoffTask() worktree = %q, want %q", gotWorktree, worktree)
	}

	updated, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read updated state: %v", err)
	}

	task := updated.FindTask(taskID)
	if task == nil {
		t.Fatalf("Task %q not found after resume", taskID)
	}
	if task.HandoffPending {
		t.Fatal("task.handoff_pending = true, want false")
	}
	if task.LeaseExpires == nil || !task.LeaseExpires.After(callStart) {
		t.Fatalf("task.lease_expires = %v, want renewed lease after %v", task.LeaseExpires, callStart)
	}
	if len(task.History) == 0 || task.History[len(task.History)-1].Event != "handoff_resumed" {
		t.Fatalf("last history event = %v, want handoff_resumed", task.History)
	}

	agent, ok := updated.Agents[agentID]
	if !ok {
		t.Fatalf("agent %q not found after resume", agentID)
	}
	if agent.Status != models.AgentStatusWorking {
		t.Fatalf("agent status = %q, want %q", agent.Status, models.AgentStatusWorking)
	}
	if agent.CurrentTask == nil || *agent.CurrentTask != taskID {
		t.Fatalf("agent current_task = %v, want %q", agent.CurrentTask, taskID)
	}
	if agent.LeaseExpires == nil || !agent.LeaseExpires.Equal(*task.LeaseExpires) {
		t.Fatalf("agent lease_expires = %v, want %v", agent.LeaseExpires, task.LeaseExpires)
	}
}

func TestResumeHandoffTask_FailsWhenWorktreeMissing(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	agentID := "coder-1"
	state := testhelpers.CreateValidState()
	// IMPLEMENTING task without Worktree field is invalid state that validateAgentInvariants
	// would catch upstream. This tests resumeHandoffTask's own guard against the condition.
	state.Tasks = []models.Task{
		{
			ID:             "task-1",
			Status:         models.TaskStatusImplementing,
			AssignedTo:     &agentID,
			HandoffPending: true,
		},
	}

	bb := testhelpers.WriteInitialState(t, statePath, state)
	snapshot, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read snapshot: %v", err)
	}

	_, _, found, err := resumeHandoffTask(bb, snapshot, agentID)
	if err == nil {
		t.Fatal("resumeHandoffTask() error = nil, want missing worktree error")
	}
	testhelpers.AssertErrorContains(t, err, "missing worktree")
	if found {
		t.Fatal("resumeHandoffTask() found = true, want false")
	}

	updated, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read updated state: %v", err)
	}
	task := updated.FindTask("task-1")
	if task == nil {
		t.Fatal("task-1 not found")
	}
	if !task.HandoffPending {
		t.Fatal("task.handoff_pending changed on failure, want true")
	}
}

func TestResumeHandoffTask_EdgeCases(t *testing.T) {
	t.Run("no resumable handoff returns not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

		agentID := "coder-1"
		worktree := ".worktrees/task-1"
		state := testhelpers.CreateValidState()
		state.Tasks = []models.Task{
			{
				ID:             "task-1",
				Status:         models.TaskStatusImplementing,
				AssignedTo:     testhelpers.StringPtr("coder-2"),
				Worktree:       &worktree,
				HandoffPending: true,
			},
		}

		bb := testhelpers.WriteInitialState(t, statePath, state)
		snapshot, err := bb.Read()
		if err != nil {
			t.Fatalf("Failed to read snapshot: %v", err)
		}

		_, _, found, err := resumeHandoffTask(bb, snapshot, agentID)
		if err != nil {
			t.Fatalf("resumeHandoffTask() error = %v, want nil", err)
		}
		if found {
			t.Fatal("resumeHandoffTask() found = true, want false")
		}
	})

	t.Run("stale candidate is skipped and next handoff resumes", func(t *testing.T) {
		tmpDir := t.TempDir()
		statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

		agentID := "coder-1"
		worktree1 := ".worktrees/task-1"
		worktree2 := ".worktrees/task-2"

		state := testhelpers.CreateValidState()
		state.Tasks = []models.Task{
			{
				ID:             "task-1",
				Status:         models.TaskStatusImplementing,
				AssignedTo:     &agentID,
				Worktree:       &worktree1,
				HandoffPending: true,
			},
			{
				ID:             "task-2",
				Status:         models.TaskStatusImplementing,
				AssignedTo:     &agentID,
				Worktree:       &worktree2,
				HandoffPending: true,
			},
		}

		bb := testhelpers.WriteInitialState(t, statePath, state)
		snapshot, err := bb.Read()
		if err != nil {
			t.Fatalf("Failed to read snapshot: %v", err)
		}

		// Make the first task stale after snapshot to exercise conflict handling.
		if err := bb.Modify(func(s *models.State) error {
			t1 := s.FindTask("task-1")
			if t1 == nil {
				t.Fatal("task-1 not found")
			}
			t1.Status = models.TaskStatusReady
			return nil
		}); err != nil {
			t.Fatalf("Failed to mutate stale candidate: %v", err)
		}

		gotTaskID, gotWorktree, found, err := resumeHandoffTask(bb, snapshot, agentID)
		if err != nil {
			t.Fatalf("resumeHandoffTask() error = %v", err)
		}
		if !found {
			t.Fatal("resumeHandoffTask() found = false, want true")
		}
		if gotTaskID != "task-2" || gotWorktree != worktree2 {
			t.Fatalf("resumeHandoffTask() = (%q, %q), want (%q, %q)", gotTaskID, gotWorktree, "task-2", worktree2)
		}
	})
}
