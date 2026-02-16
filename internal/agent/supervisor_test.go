package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// MockCLIExecutor for testing CLI execution
type MockCLIExecutor struct {
	mu        sync.Mutex
	Calls     []MockCLICall
	ExitCode  int
	ExitError error
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

// GetCalls returns a copy of the calls slice in a thread-safe manner
func (m *MockCLIExecutor) GetCalls() []MockCLICall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]MockCLICall, len(m.Calls))
	copy(calls, m.Calls)
	return calls
}

// TestValidateIdentity tests agent ID format validation
func TestValidateIdentity(t *testing.T) {
	tests := []struct {
		name    string
		agentID string
		role    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid coder ID",
			agentID: "coder-1",
			role:    "coder",
			wantErr: false,
		},
		{
			name:    "valid reviewer ID",
			agentID: "code-reviewer-1",
			role:    "code-reviewer",
			wantErr: false,
		},
		{
			name:    "valid planner ID",
			agentID: "planner-1",
			role:    "planner",
			wantErr: false,
		},
		{
			name:    "valid multi-digit number",
			agentID: "coder-42",
			role:    "coder",
			wantErr: false,
		},
		{
			name:    "empty agent ID",
			agentID: "",
			role:    "coder",
			wantErr: true,
			errMsg:  "agent ID required",
		},
		{
			name:    "missing number",
			agentID: "coder",
			role:    "coder",
			wantErr: true,
			errMsg:  "format",
		},
		{
			name:    "non-numeric suffix",
			agentID: "coder-abc",
			role:    "coder",
			wantErr: true,
			errMsg:  "numeric",
		},
		{
			name:    "role mismatch",
			agentID: "coder-1",
			role:    "planner",
			wantErr: true,
			errMsg:  "mismatch",
		},
		{
			name:    "invalid prefix",
			agentID: "invalid-1",
			role:    "coder",
			wantErr: true,
			errMsg:  "mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIdentity(tt.agentID, tt.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIdentity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Error should contain %q, got %v", tt.errMsg, err)
			}
		})
	}
}

// TestRegisterAgent tests agent registration
func TestRegisterAgent(t *testing.T) {
	tests := []struct {
		name           string
		agentID        string
		role           string
		existingAgent  *models.Agent
		expectRegister bool
		wantErr        bool
		errMsg         string
	}{
		{
			name:           "new agent registration",
			agentID:        "coder-1",
			role:           "coder",
			existingAgent:  nil,
			expectRegister: true,
			wantErr:        false,
		},
		{
			name:    "collision with valid lease",
			agentID: "coder-1",
			role:    "coder",
			existingAgent: &models.Agent{
				Role:         "coder",
				Status:       models.AgentStatusWorking,
				LeaseExpires: testhelpers.TimePtr(time.Now().UTC().Add(10 * time.Minute)),
				Heartbeat:    time.Now().UTC(),
			},
			expectRegister: false,
			wantErr:        true,
			errMsg:         "collision",
		},
		{
			name:    "takeover expired lease",
			agentID: "coder-1",
			role:    "coder",
			existingAgent: &models.Agent{
				Role:         "coder",
				Status:       models.AgentStatusWorking,
				LeaseExpires: testhelpers.TimePtr(time.Now().UTC().Add(-10 * time.Minute)),
				Heartbeat:    time.Now().UTC().Add(-10 * time.Minute),
			},
			expectRegister: true,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

			state := testhelpers.CreateValidState()
			if tt.existingAgent != nil {
				state.Agents[tt.agentID] = *tt.existingAgent
			}

			bb := testhelpers.WriteInitialState(t, statePath, state)

			err := registerAgent(bb, tmpDir, tt.agentID, tt.role, "terminal-1", 1800)

			if (err != nil) != tt.wantErr {
				t.Errorf("registerAgent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Error should contain %q, got %v", tt.errMsg, err)
			}

			if tt.expectRegister {
				// Verify agent was registered with correct status
				state, err := bb.Read()
				if err != nil {
					t.Fatalf("Failed to read state: %v", err)
				}

				agent, exists := state.Agents[tt.agentID]
				if !exists {
					t.Errorf("Agent %s not registered", tt.agentID)
					return
				}

				if agent.Status != models.AgentStatusIdle {
					t.Errorf("Expected status IDLE, got %s", agent.Status)
				}

				if agent.Role != tt.role {
					t.Errorf("Expected role %s, got %s", tt.role, agent.Role)
				}

				// Verify PID is stored
				if agent.PID == 0 {
					t.Error("Expected PID to be set (non-zero)")
				}
			}
		})
	}
}

// TestRegisterAgentConcurrent tests concurrent registration race condition
func TestRegisterAgentConcurrent(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	bb := testhelpers.WriteInitialState(t, statePath, state)

	agentID := "coder-1"
	role := "coder"
	numGoroutines := 5

	// Track success/failure counts
	successes := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines)

	// Launch multiple goroutines trying to register the same agent ID
	for range numGoroutines {
		go func() {
			err := registerAgent(bb, tmpDir, agentID, role, "terminal-1", 1800)
			if err != nil {
				errors <- err
			} else {
				successes <- true
			}
		}()
	}

	// Collect results
	successCount := 0
	errorCount := 0
	for range numGoroutines {
		select {
		case <-successes:
			successCount++
		case err := <-errors:
			errorCount++
			// Verify error is about collision
			if !strings.Contains(err.Error(), "collision") && !strings.Contains(err.Error(), "already registered") {
				t.Errorf("Expected collision error, got: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for registration results")
		}
	}

	// Exactly one should succeed, rest should fail with collision
	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful registration, got %d", successCount)
	}

	if errorCount != numGoroutines-1 {
		t.Errorf("Expected %d collision errors, got %d", numGoroutines-1, errorCount)
	}

	// Verify only one agent exists in state
	finalState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read final state: %v", err)
	}

	agent, exists := finalState.Agents[agentID]
	if !exists {
		t.Fatal("Agent should be registered")
	}

	if agent.Status != models.AgentStatusIdle {
		t.Errorf("Agent status should be IDLE, got %s", agent.Status)
	}
}

// TestUnregisterAgent tests agent cleanup
func TestUnregisterAgent(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	agentID := "coder-1"
	state.Agents[agentID] = models.Agent{
		Role:      "coder",
		Status:    models.AgentStatusWorking,
		Heartbeat: time.Now().UTC(),
	}

	bb := testhelpers.WriteInitialState(t, statePath, state)

	unregisterAgent(bb, agentID)

	// Verify agent was removed
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	if _, exists := state.Agents[agentID]; exists {
		t.Errorf("Agent %s should be unregistered", agentID)
	}
}

// TestRegisterAgentStoresPID tests that agent registration stores the process PID
func TestRegisterAgentStoresPID(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	bb := testhelpers.WriteInitialState(t, statePath, state)

	agentID := "coder-1"
	role := "coder"

	err := registerAgent(bb, tmpDir, agentID, role, "terminal-1", 1800)
	if err != nil {
		t.Fatalf("registerAgent() error = %v", err)
	}

	// Verify PID is stored and matches current process
	state, err = bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	agent, exists := state.Agents[agentID]
	if !exists {
		t.Fatal("Agent should be registered")
	}

	// Verify PID is non-zero
	if agent.PID == 0 {
		t.Error("PID should be set (non-zero)")
	}

	// Verify PID matches current process (in test, it will be the test process PID)
	currentPID := os.Getpid()
	if agent.PID != currentPID {
		t.Errorf("Expected PID to be %d (current process), got %d", currentPID, agent.PID)
	}
}

// TestWaitForCoderWork tests coder work detection
func TestWaitForCoderWork(t *testing.T) {
	tests := []struct {
		name     string
		tasks    []models.Task
		wantWork bool
	}{
		{
			name: "claimable task available",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusUnclaimed, time.Now().UTC()),
			},
			wantWork: true,
		},
		{
			name: "rejected task available",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, time.Now().UTC()),
			},
			wantWork: true,
		},
		{
			name: "integration failed task available",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, time.Now().UTC()),
			},
			wantWork: true,
		},
		{
			name: "no claimable tasks",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusClaimed, time.Now().UTC()),
			},
			wantWork: false,
		},
		{
			name: "task waiting on dependency",
			tasks: []models.Task{
				{
					ID:        "task-1",
					Status:    models.TaskStatusUnclaimed,
					DependsOn: []string{"task-2"},
				},
				testhelpers.BuildTaskByStatus("task-2", models.TaskStatusClaimed, time.Now().UTC()),
			},
			wantWork: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
			lizaDir := filepath.Dir(statePath)

			state := testhelpers.CreateValidState()
			state.Tasks = tt.tasks

			testhelpers.WriteInitialState(t, statePath, state)

			config := SupervisorConfig{
				StatePath: statePath,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()

			hasWork, err := waitForWork(ctx, db.New(statePath), lizaDir, "coder", config, 10*time.Millisecond, 100*time.Millisecond)

			if err != nil {
				t.Fatalf("waitForWork() error = %v", err)
			}

			if hasWork != tt.wantWork {
				t.Errorf("waitForWork() = %v, want %v", hasWork, tt.wantWork)
			}
		})
	}
}

// TestWaitForReviewerWork tests reviewer work detection
func TestWaitForReviewerWork(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name     string
		tasks    []models.Task
		wantWork bool
	}{
		{
			name: "reviewable task available",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now),
			},
			wantWork: true,
		},
		{
			name: "task with expired review lease",
			tasks: []models.Task{
				{
					ID:                 "task-1",
					Status:             models.TaskStatusReadyForReview,
					ReviewingBy:        testhelpers.StringPtr("reviewer-1"),
					ReviewLeaseExpires: testhelpers.TimePtr(now.Add(-10 * time.Minute)),
				},
			},
			wantWork: true,
		},
		{
			name: "no reviewable tasks",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusClaimed, now),
			},
			wantWork: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
			lizaDir := filepath.Dir(statePath)

			state := testhelpers.CreateValidState()
			state.Tasks = tt.tasks

			testhelpers.WriteInitialState(t, statePath, state)

			config := SupervisorConfig{
				StatePath: statePath,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()

			hasWork, err := waitForWork(ctx, db.New(statePath), lizaDir, "code-reviewer", config, 10*time.Millisecond, 100*time.Millisecond)

			if err != nil {
				t.Fatalf("waitForWork() error = %v", err)
			}

			if hasWork != tt.wantWork {
				t.Errorf("waitForWork() = %v, want %v", hasWork, tt.wantWork)
			}
		})
	}
}

// TestWaitForPlannerWork tests planner work detection
func TestWaitForPlannerWork(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name     string
		tasks    []models.Task
		wantWork bool
	}{
		{
			name:     "initial planning - no tasks",
			tasks:    []models.Task{},
			wantWork: true,
		},
		{
			name: "blocked tasks",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
			},
			wantWork: true,
		},
		{
			name: "integration failed tasks",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now),
			},
			wantWork: true,
		},
		{
			name: "no planner work needed",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusClaimed, now),
			},
			wantWork: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
			lizaDir := filepath.Dir(statePath)

			state := testhelpers.CreateValidState()
			state.Tasks = tt.tasks

			testhelpers.WriteInitialState(t, statePath, state)

			config := SupervisorConfig{
				StatePath: statePath,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()

			hasWork, err := waitForWork(ctx, db.New(statePath), lizaDir, "planner", config, 10*time.Millisecond, 100*time.Millisecond)

			// For "no planner work needed", planner waits indefinitely until context cancellation
			// So we expect context.DeadlineExceeded error
			if tt.name == "no planner work needed" {
				if err == nil {
					t.Fatalf("waitForWork() expected context deadline exceeded, got nil error")
				}
				if err != context.DeadlineExceeded {
					t.Fatalf("waitForWork() expected context.DeadlineExceeded, got %v", err)
				}
				// hasWork should be false when context times out
				if hasWork != false {
					t.Errorf("waitForWork() = %v, want false", hasWork)
				}
			} else {
				if err != nil {
					t.Fatalf("waitForWork() error = %v", err)
				}

				if hasWork != tt.wantWork {
					t.Errorf("waitForWork() = %v, want %v", hasWork, tt.wantWork)
				}
			}
		})
	}
}

// TestWaitForWorkEventDriven tests that agents wake quickly on state changes
func TestWaitForWorkEventDriven(t *testing.T) {
	tests := []struct {
		name        string
		role        string
		setupState  func() *models.State
		modifyState func(*models.State)
		wantWork    bool
	}{
		{
			name: "coder wakes on new claimable task",
			role: "coder",
			setupState: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusClaimed, time.Now().UTC()),
				}
				return state
			},
			modifyState: func(s *models.State) {
				s.Tasks = append(s.Tasks, testhelpers.BuildTaskByStatus("task-2", models.TaskStatusUnclaimed, time.Now().UTC()))
			},
			wantWork: true,
		},
		{
			name: "reviewer wakes on reviewable task",
			role: "code-reviewer",
			setupState: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{}
				return state
			},
			modifyState: func(s *models.State) {
				s.Tasks = append(s.Tasks, testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, time.Now().UTC()))
			},
			wantWork: true,
		},
		{
			name: "planner wakes on wake trigger",
			role: "planner",
			setupState: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{}
				return state
			},
			modifyState: func(s *models.State) {
				// Add 5 unclaimed tasks to trigger planner wake
				for i := 1; i <= 5; i++ {
					taskID := "task-" + string(rune('0'+i))
					s.Tasks = append(s.Tasks, testhelpers.BuildTaskByStatus(taskID, models.TaskStatusUnclaimed, time.Now().UTC()))
				}
			},
			wantWork: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
			lizaDir := filepath.Dir(statePath)

			// Write initial state with no work
			state := tt.setupState()
			testhelpers.WriteInitialState(t, statePath, state)

			config := SupervisorConfig{
				StatePath: statePath,
			}

			bb := db.New(statePath)

			// Start waiting in goroutine
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			startTime := time.Now()
			resultCh := make(chan bool, 1)
			errCh := make(chan error, 1)

			go func() {
				hasWork, err := waitForWork(ctx, bb, lizaDir, tt.role, config, 10*time.Millisecond, 5*time.Second)
				if err != nil {
					errCh <- err
					return
				}
				resultCh <- hasWork
			}()

			// Wait a bit for watcher to start
			time.Sleep(50 * time.Millisecond)

			// Modify state to create work
			if err := bb.Modify(func(s *models.State) error {
				tt.modifyState(s)
				return nil
			}); err != nil {
				t.Fatalf("Failed to modify state: %v", err)
			}

			// Wait for result
			select {
			case err := <-errCh:
				t.Fatalf("waitForWork() error = %v", err)
			case hasWork := <-resultCh:
				elapsed := time.Since(startTime)

				if hasWork != tt.wantWork {
					t.Errorf("waitForWork() = %v, want %v", hasWork, tt.wantWork)
				}

				// Verify wake time is quick (under 500ms including setup)
				if hasWork && elapsed > 500*time.Millisecond {
					t.Errorf("Agent took %v to wake, expected < 500ms", elapsed)
				}
			case <-time.After(6 * time.Second):
				t.Fatal("Timeout waiting for waitForWork result")
			}
		})
	}
}

// TestWaitForWorkCancellation tests context cancellation during wait
func TestWaitForWorkCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	lizaDir := filepath.Dir(statePath)

	// Create state with no work
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{}
	testhelpers.WriteInitialState(t, statePath, state)

	config := SupervisorConfig{
		StatePath: statePath,
	}

	bb := db.New(statePath)
	ctx, cancel := context.WithCancel(context.Background())

	// Start waiting in goroutine
	errCh := make(chan error, 1)
	go func() {
		_, err := waitForWork(ctx, bb, lizaDir, "coder", config, 10*time.Millisecond, 10*time.Second)
		errCh <- err
	}()

	// Cancel after short delay
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Should return context.Canceled error quickly
	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for cancellation")
	}
}

// TestWaitForWorkTimeout tests deadline expiration
func TestWaitForWorkTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	lizaDir := filepath.Dir(statePath)

	// Create state with no work
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{}
	testhelpers.WriteInitialState(t, statePath, state)

	config := SupervisorConfig{
		StatePath: statePath,
	}

	bb := db.New(statePath)
	ctx := context.Background()

	startTime := time.Now()
	hasWork, err := waitForWork(ctx, bb, lizaDir, "coder", config, 10*time.Millisecond, 200*time.Millisecond)
	elapsed := time.Since(startTime)

	if err != nil {
		t.Fatalf("waitForWork() error = %v", err)
	}

	if hasWork {
		t.Error("Expected no work after timeout")
	}

	// Should wait approximately the maxWait duration
	if elapsed < 200*time.Millisecond || elapsed > 300*time.Millisecond {
		t.Errorf("Expected timeout around 200ms, got %v", elapsed)
	}
}

// TestCheckAbort tests ABORT signal detection
// TestPromptSaving tests prompt file creation
func TestPromptSaving(t *testing.T) {
	tmpDir := t.TempDir()
	promptDir := filepath.Join(tmpDir, "agent-prompts")

	prompt := "Test prompt content"
	agentID := "coder-1"

	filePath, err := savePrompt(promptDir, agentID, prompt)
	if err != nil {
		t.Fatalf("savePrompt() error = %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(promptDir); os.IsNotExist(err) {
		t.Error("Prompt directory should be created")
	}

	// Verify file exists and has correct content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read prompt file: %v", err)
	}

	if string(content) != prompt {
		t.Errorf("Prompt content = %q, want %q", string(content), prompt)
	}

	// Verify filename format
	filename := filepath.Base(filePath)
	if !strings.HasPrefix(filename, agentID+"-") {
		t.Errorf("Filename should start with agent ID, got %s", filename)
	}
	if !strings.HasSuffix(filename, ".txt") {
		t.Errorf("Filename should end with .txt, got %s", filename)
	}
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
		for len(mock.GetCalls()) == 0 {
			time.Sleep(10 * time.Millisecond)
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

// TestInteractiveMode tests that interactive mode doesn't execute CLI
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

	// In interactive mode, should print prompt location and exit
	err := RunSupervisor(ctx, config)

	// Should exit cleanly (no error) and not call executor
	if err != nil && err != context.DeadlineExceeded {
		t.Errorf("RunSupervisor() error = %v, want nil or DeadlineExceeded", err)
	}

	if len(mock.GetCalls()) > 0 {
		t.Error("Interactive mode should not execute CLI")
	}
}

// TestBuildPrompt tests the buildPrompt function
func TestBuildPrompt(t *testing.T) {
	now := time.Now().UTC()
	state := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			SpecRef:     "spec.md",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Tasks: []models.Task{
			{
				ID:          "task-1",
				Description: "Test task",
				Status:      models.TaskStatusClaimed,
				Priority:    1,
				SpecRef:     "spec.md",
				DoneWhen:    "Task is complete",
				Created:     now,
			},
		},
		Agents: make(map[string]models.Agent),
		Config: models.Config{
			IntegrationBranch: "main",
		},
	}

	tests := []struct {
		name        string
		role        string
		taskID      string
		initialTask string
		wantErr     bool
		contains    []string
	}{
		{
			name:     "coder prompt",
			role:     "coder",
			taskID:   "task-1",
			wantErr:  false,
			contains: []string{"coder", "Test goal", "task-1"},
		},
		{
			name:     "code-reviewer prompt",
			role:     "code-reviewer",
			taskID:   "task-1",
			wantErr:  false,
			contains: []string{"reviewer", "Test goal", "task-1"},
		},
		{
			name:     "planner prompt",
			role:     "planner",
			taskID:   "",
			wantErr:  false,
			contains: []string{"planner", "Test goal"},
		},
		{
			name:     "coder with non-existent task",
			role:     "coder",
			taskID:   "task-999",
			wantErr:  true,
			contains: nil,
		},
		{
			name:        "coder with initial task",
			role:        "coder",
			taskID:      "task-1",
			initialTask: "task-1",
			wantErr:     false,
			contains:    []string{"coder", "RESUME CONTEXT", "task-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			config := SupervisorConfig{
				Role:        tt.role,
				AgentID:     tt.role + "-1",
				ProjectRoot: tmpDir,
				SpecsDir:    filepath.Join(tmpDir, "specs"),
				StatePath:   filepath.Join(tmpDir, "state.yaml"),
				InitialTask: tt.initialTask,
			}

			prompt, err := buildPrompt(state, config, tt.taskID)

			if (err != nil) != tt.wantErr {
				t.Errorf("buildPrompt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Check that prompt contains expected strings
			for _, expected := range tt.contains {
				if !strings.Contains(prompt, expected) {
					t.Errorf("buildPrompt() prompt should contain %q", expected)
				}
			}

			// Verify prompt is not empty
			if prompt == "" {
				t.Error("buildPrompt() returned empty prompt")
			}
		})
	}
}

// TestSavePromptMultipleCalls tests that savePrompt creates unique filenames
func TestSavePromptMultipleCalls(t *testing.T) {
	tmpDir := t.TempDir()
	promptDir := filepath.Join(tmpDir, "prompts")

	// Save multiple prompts
	path1, err := savePrompt(promptDir, "coder-1", "prompt 1")
	if err != nil {
		t.Fatalf("savePrompt() error = %v", err)
	}

	// Small delay to ensure different timestamp
	time.Sleep(1 * time.Second)

	path2, err := savePrompt(promptDir, "coder-1", "prompt 2")
	if err != nil {
		t.Fatalf("savePrompt() error = %v", err)
	}

	// Verify paths are different
	if path1 == path2 {
		t.Error("savePrompt() should create unique filenames")
	}

	// Verify both files exist
	if _, err := os.Stat(path1); os.IsNotExist(err) {
		t.Error("First prompt file should exist")
	}
	if _, err := os.Stat(path2); os.IsNotExist(err) {
		t.Error("Second prompt file should exist")
	}
}

// TestIsSystemStopped tests the isSystemStopped helper function
func TestIsSystemStopped(t *testing.T) {
	tests := []struct {
		name         string
		stateMode    models.SystemMode
		wantStopped  bool
		wantReasonRe string
	}{
		{
			name:         "state-based STOPPED mode",
			stateMode:    models.SystemModeStopped,
			wantStopped:  true,
			wantReasonRe: "STOPPED",
		},
		{
			name:        "not stopped",
			stateMode:   models.SystemModeRunning,
			wantStopped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			lizaDir := filepath.Join(tmpDir, ".liza")
			if err := os.MkdirAll(lizaDir, 0755); err != nil {
				t.Fatalf("Failed to create .liza dir: %v", err)
			}

			state := testhelpers.CreateValidState()
			state.Config.Mode = tt.stateMode

			stopped, reason := isSystemStopped(state, lizaDir)

			if stopped != tt.wantStopped {
				t.Errorf("isSystemStopped() stopped = %v, want %v", stopped, tt.wantStopped)
			}

			if tt.wantStopped && tt.wantReasonRe != "" && !strings.Contains(reason, tt.wantReasonRe) {
				t.Errorf("isSystemStopped() reason = %q, should contain %q", reason, tt.wantReasonRe)
			}

			if !tt.wantStopped && reason != "" {
				t.Errorf("isSystemStopped() reason should be empty when not stopped, got %q", reason)
			}
		})
	}
}

// TestWaitForWorkEventDrivenAbortStateMode tests ABORT detection via state mode in event-driven wait
func TestWaitForWorkEventDrivenAbortStateMode(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	lizaDir := filepath.Dir(statePath)

	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{} // No work available
	testhelpers.WriteInitialState(t, statePath, state)

	bb := db.New(statePath)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	resultCh := make(chan bool, 1)
	errCh := make(chan error, 1)

	// Start waiting in background
	go func() {
		hasWork, err := waitForCoderWork(ctx, bb, lizaDir, 10*time.Millisecond, 5*time.Second)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- hasWork
	}()

	// Wait for watcher to start
	time.Sleep(50 * time.Millisecond)

	// Set state to STOPPED
	if err := bb.Modify(func(s *models.State) error {
		s.Config.Mode = models.SystemModeStopped
		return nil
	}); err != nil {
		t.Fatalf("Failed to set STOPPED mode: %v", err)
	}

	// Should return quickly
	select {
	case err := <-errCh:
		t.Fatalf("waitForCoderWork() error = %v", err)
	case hasWork := <-resultCh:
		elapsed := time.Since(startTime)

		if hasWork {
			t.Error("waitForCoderWork() should return false when ABORT detected")
		}

		// Should respond within 200ms
		if elapsed > 200*time.Millisecond {
			t.Errorf("ABORT detection took %v, expected < 200ms", elapsed)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("Timeout waiting for ABORT response")
	}
}

// TestWaitForWorkEventDrivenAbortFile tests ABORT detection via file in event-driven wait
// TestWaitForWorkPollingAbortStateMode tests ABORT detection via state mode in polling wait
func TestWaitForWorkPollingAbortStateMode(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	lizaDir := filepath.Dir(statePath)

	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{} // No work available
	testhelpers.WriteInitialState(t, statePath, state)

	bb := db.New(statePath)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	resultCh := make(chan bool, 1)
	errCh := make(chan error, 1)

	// Use polling wait with 50ms poll interval
	checkWork := func(s *models.State) (bool, string) {
		return CountClaimableTasks(s) > 0, ""
	}

	// Start waiting in background
	go func() {
		hasWork, err := waitForWorkPolling(ctx, bb, lizaDir, 50*time.Millisecond, 5*time.Second, checkWork)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- hasWork
	}()

	// Wait a bit then set STOPPED mode
	time.Sleep(25 * time.Millisecond)

	if err := bb.Modify(func(s *models.State) error {
		s.Config.Mode = models.SystemModeStopped
		return nil
	}); err != nil {
		t.Fatalf("Failed to set STOPPED mode: %v", err)
	}

	// Should detect on next poll (within 100ms)
	select {
	case err := <-errCh:
		t.Fatalf("waitForWorkPolling() error = %v", err)
	case hasWork := <-resultCh:
		elapsed := time.Since(startTime)

		if hasWork {
			t.Error("waitForWorkPolling() should return false when ABORT detected")
		}

		// Should respond within 100ms
		if elapsed > 100*time.Millisecond {
			t.Errorf("ABORT detection took %v, expected < 100ms", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for ABORT response")
	}
}

// TestWaitForWorkPollingAbortFile tests ABORT detection via file in polling wait
// TestAbortPrecedenceOverWork tests that ABORT takes precedence over work
func TestAbortPrecedenceOverWork(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	lizaDir := filepath.Dir(statePath)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Config.Mode = models.SystemModeStopped // STOPPED mode
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusUnclaimed, now), // Work available
	}
	testhelpers.WriteInitialState(t, statePath, state)

	bb := db.New(statePath)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Should return false (ABORT), not true (work available)
	hasWork, err := waitForCoderWork(ctx, bb, lizaDir, 10*time.Millisecond, 100*time.Millisecond)

	if err != nil {
		t.Fatalf("waitForCoderWork() error = %v", err)
	}

	if hasWork {
		t.Error("waitForCoderWork() should return false when ABORT present, even with work available")
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

	// Send ABORT signal after 1 second
	go func() {
		time.Sleep(1 * time.Second)
		if err := db.New(statePath).Modify(func(s *models.State) error {
			s.Config.Mode = models.SystemModeStopped
			return nil
		}); err != nil {
			t.Logf("Failed to set STOPPED mode: %v", err)
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

// TestPlannerStatusTransitions tests planner agent status lifecycle
func TestPlannerStatusTransitions(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	// No tasks - triggers INITIAL_PLANNING work for planner
	state.Tasks = []models.Task{}
	testhelpers.WriteInitialState(t, statePath, state)

	bb := db.New(statePath)

	// Register planner agent
	agentID := "planner-1"
	err := registerAgent(bb, tmpDir, agentID, "planner", "terminal-1", 1800)
	if err != nil {
		t.Fatalf("registerAgent() error = %v", err)
	}

	// Verify initial status is IDLE
	state, err = bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	agent := state.Agents[agentID]
	if agent.Status != models.AgentStatusIdle {
		t.Errorf("Initial status = %s, want IDLE", agent.Status)
	}

	// Simulate planner starting work - set PLANNING status
	err = setAgentToPlanningStatus(bb, agentID)
	if err != nil {
		t.Fatalf("setAgentToPlanningStatus() error = %v", err)
	}

	// Verify status is now PLANNING
	state, err = bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	agent = state.Agents[agentID]
	if agent.Status != models.AgentStatusPlanning {
		t.Errorf("Status after setAgentToPlanningStatus = %s, want PLANNING", agent.Status)
	}

	// Verify heartbeat was updated
	if agent.Heartbeat.Before(time.Now().UTC().Add(-5 * time.Second)) {
		t.Error("Heartbeat should be updated when status changes to PLANNING")
	}

	// Simulate planner completing work - reset to IDLE
	err = resetAgentToIdle(bb, agentID)
	if err != nil {
		t.Fatalf("resetAgentToIdle() error = %v", err)
	}

	// Verify status is back to IDLE
	state, err = bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	agent = state.Agents[agentID]
	if agent.Status != models.AgentStatusIdle {
		t.Errorf("Status after resetAgentToIdle = %s, want IDLE", agent.Status)
	}
}

// TestSetAgentToPlanningStatusNonExistent tests error handling for non-existent agent
func TestSetAgentToPlanningStatusNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, statePath, state)

	bb := db.New(statePath)

	// Try to set status for non-existent agent
	err := setAgentToPlanningStatus(bb, "planner-999")
	if err == nil {
		t.Error("setAgentToPlanningStatus() should return error for non-existent agent")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error should mention 'not found', got: %v", err)
	}
}

// TestVerifyPlannerStateChanges_IntegrationFailedClaimedByCoder verifies that
// the planner validation accepts when a coder claims an INTEGRATION_FAILED task
func TestVerifyPlannerStateChanges_IntegrationFailedClaimedByCoder(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()

	// State before: task is INTEGRATION_FAILED
	stateBefore := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"planner-1": {Role: "planner", Status: models.AgentStatusPlanning, Heartbeat: now},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now),
		},
		Config: models.Config{IntegrationBranch: "main"},
	}

	// State after: task is CLAIMED (by coder)
	stateAfter := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"planner-1": {Role: "planner", Status: models.AgentStatusIdle, Heartbeat: now},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusClaimed, now),
		},
		Config: models.Config{IntegrationBranch: "main"},
	}

	testhelpers.WriteInitialState(t, statePath, stateAfter)

	bb := db.New(statePath)

	err := verifyPlannerStateChanges(bb, stateBefore)
	if err != nil {
		t.Errorf("Expected validation to pass when coder claims INTEGRATION_FAILED task, got error: %v", err)
	}
}

// TestVerifyPlannerStateChanges_IntegrationFailedSuperseded verifies that
// the planner validation accepts when planner supersedes an INTEGRATION_FAILED task
func TestVerifyPlannerStateChanges_IntegrationFailedSuperseded(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()

	// State before: task is INTEGRATION_FAILED
	stateBefore := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"planner-1": {Role: "planner", Status: models.AgentStatusPlanning, Heartbeat: now},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now),
		},
		Config: models.Config{IntegrationBranch: "main"},
	}

	// State after: task is SUPERSEDED
	stateAfter := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"planner-1": {Role: "planner", Status: models.AgentStatusIdle, Heartbeat: now},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusSuperseded, now),
			testhelpers.BuildTaskByStatus("task-2", models.TaskStatusUnclaimed, now),
		},
		Config: models.Config{IntegrationBranch: "main"},
	}

	testhelpers.WriteInitialState(t, statePath, stateAfter)

	bb := db.New(statePath)

	err := verifyPlannerStateChanges(bb, stateBefore)
	if err != nil {
		t.Errorf("Expected validation to pass when planner supersedes INTEGRATION_FAILED task, got error: %v", err)
	}
}

// TestVerifyPlannerStateChanges_IntegrationFailedNotHandled verifies that
// the planner validation fails when INTEGRATION_FAILED task remains unchanged
func TestVerifyPlannerStateChanges_IntegrationFailedNotHandled(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()

	// State before: task is INTEGRATION_FAILED
	stateBefore := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"planner-1": {Role: "planner", Status: models.AgentStatusPlanning, Heartbeat: now},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now),
		},
		Config: models.Config{IntegrationBranch: "main"},
	}

	// State after: task STILL INTEGRATION_FAILED (no change)
	stateAfter := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"planner-1": {Role: "planner", Status: models.AgentStatusIdle, Heartbeat: now},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now),
		},
		Config: models.Config{IntegrationBranch: "main"},
	}

	testhelpers.WriteInitialState(t, statePath, stateAfter)

	bb := db.New(statePath)

	err := verifyPlannerStateChanges(bb, stateBefore)
	if err == nil {
		t.Error("Expected validation to fail when INTEGRATION_FAILED task remains unchanged")
	}

	if !strings.Contains(err.Error(), "no tasks were handled") {
		t.Errorf("Expected error to mention 'no tasks were handled', got: %v", err)
	}
}

// TestVerifyPlannerStateChanges_IntegrationFailedMixedOutcomes verifies that
// the planner validation accepts when some tasks are handled (claimed/superseded) and others remain
func TestVerifyPlannerStateChanges_IntegrationFailedMixedOutcomes(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()

	// State before: 3 tasks are INTEGRATION_FAILED
	stateBefore := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"planner-1": {Role: "planner", Status: models.AgentStatusPlanning, Heartbeat: now},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now),
			testhelpers.BuildTaskByStatus("task-2", models.TaskStatusIntegrationFailed, now),
			testhelpers.BuildTaskByStatus("task-3", models.TaskStatusIntegrationFailed, now),
		},
		Config: models.Config{IntegrationBranch: "main"},
	}

	// State after: 1 CLAIMED, 1 SUPERSEDED, 1 still INTEGRATION_FAILED
	stateAfter := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"planner-1": {Role: "planner", Status: models.AgentStatusIdle, Heartbeat: now},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusClaimed, now),
			testhelpers.BuildTaskByStatus("task-2", models.TaskStatusSuperseded, now),
			testhelpers.BuildTaskByStatus("task-3", models.TaskStatusIntegrationFailed, now),
			testhelpers.BuildTaskByStatus("task-4", models.TaskStatusUnclaimed, now),
		},
		Config: models.Config{IntegrationBranch: "main"},
	}

	testhelpers.WriteInitialState(t, statePath, stateAfter)

	bb := db.New(statePath)

	err := verifyPlannerStateChanges(bb, stateBefore)
	if err != nil {
		t.Errorf("Expected validation to pass when some INTEGRATION_FAILED tasks are handled, got error: %v", err)
	}
}

// TestHasPendingMerges tests the hasPendingMerges function
func TestHasPendingMerges(t *testing.T) {
	tests := []struct {
		name     string
		tasks    []models.Task
		agentID  string
		expected bool
	}{
		{
			name:     "no tasks returns false",
			tasks:    []models.Task{},
			agentID:  "code-reviewer-1",
			expected: false,
		},
		{
			name: "approved task with merge_commit set returns false",
			tasks: []models.Task{
				{
					ID:          "task-1",
					Status:      models.TaskStatusApproved,
					ApprovedBy:  testhelpers.StringPtr("code-reviewer-1"),
					MergeCommit: testhelpers.StringPtr("abc123"),
				},
			},
			agentID:  "code-reviewer-1",
			expected: false,
		},
		{
			name: "approved task by different agent returns false",
			tasks: []models.Task{
				{
					ID:          "task-1",
					Status:      models.TaskStatusApproved,
					ApprovedBy:  testhelpers.StringPtr("code-reviewer-2"),
					MergeCommit: nil,
				},
			},
			agentID:  "code-reviewer-1",
			expected: false,
		},
		{
			name: "approved task by this agent without merge_commit returns true",
			tasks: []models.Task{
				{
					ID:          "task-1",
					Status:      models.TaskStatusApproved,
					ApprovedBy:  testhelpers.StringPtr("code-reviewer-1"),
					MergeCommit: nil,
				},
			},
			agentID:  "code-reviewer-1",
			expected: true,
		},
		{
			name: "multiple tasks, one pending returns true",
			tasks: []models.Task{
				{
					ID:          "task-1",
					Status:      models.TaskStatusApproved,
					ApprovedBy:  testhelpers.StringPtr("code-reviewer-1"),
					MergeCommit: testhelpers.StringPtr("abc123"),
				},
				{
					ID:          "task-2",
					Status:      models.TaskStatusApproved,
					ApprovedBy:  testhelpers.StringPtr("code-reviewer-1"),
					MergeCommit: nil,
				},
			},
			agentID:  "code-reviewer-1",
			expected: true,
		},
		{
			name: "integration_failed task returns false",
			tasks: []models.Task{
				{
					ID:          "task-1",
					Status:      models.TaskStatusIntegrationFailed,
					ApprovedBy:  testhelpers.StringPtr("code-reviewer-1"),
					MergeCommit: nil,
				},
			},
			agentID:  "code-reviewer-1",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test blackboard with tasks
			tmpDir := t.TempDir()
			statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

			state := testhelpers.CreateValidState()
			state.Tasks = tt.tasks

			testhelpers.WriteInitialState(t, statePath, state)

			bb := db.New(statePath)

			// Test hasPendingMerges
			result := hasPendingMerges(bb, tt.agentID)
			if result != tt.expected {
				t.Errorf("hasPendingMerges() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
