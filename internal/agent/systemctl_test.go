package agent

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

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
			state := testhelpers.CreateValidState()
			state.Config.Mode = tt.stateMode

			stopped, reason := isSystemStopped(state)

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

// TestVerifyOrchestratorStateChanges_BlockedNotResolved verifies that
// the orchestrator validation accepts when blocked tasks remain unchanged (no-op exit)
func TestVerifyOrchestratorStateChanges_BlockedNotResolved(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	now := time.Now().UTC()

	// State before: task is BLOCKED
	stateBefore := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"orchestrator-1": {Role: "orchestrator", Status: models.AgentStatusPlanning, Heartbeat: now},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
		},
		Config: models.Config{IntegrationBranch: "main"},
	}

	// State after: task STILL BLOCKED (orchestrator couldn't resolve)
	stateAfter := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"orchestrator-1": {Role: "orchestrator", Status: models.AgentStatusIdle, Heartbeat: now},
		},
		Tasks: []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
		},
		Config: models.Config{IntegrationBranch: "main"},
	}

	testhelpers.WriteInitialState(t, statePath, stateAfter)

	bb := db.New(statePath)

	err := verifyOrchestratorStateChanges(bb, stateBefore, nil, nil)
	if err != nil {
		t.Errorf("Expected no error for no-op BLOCKED exit (may require human intervention), got: %v", err)
	}
}

// TestVerifyOrchestratorStateChanges_HypothesisExhaustedNotResolved verifies that
// the orchestrator validation accepts when exhausted tasks remain unchanged (no-op exit)
func TestVerifyOrchestratorStateChanges_HypothesisExhaustedNotResolved(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	now := time.Now().UTC()

	// State before: task has 2+ failed_by (hypothesis exhausted)
	exhaustedTask := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	exhaustedTask.FailedBy = []string{"coder-1", "coder-2"}
	stateBefore := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"orchestrator-1": {Role: "orchestrator", Status: models.AgentStatusPlanning, Heartbeat: now},
		},
		Tasks:  []models.Task{exhaustedTask},
		Config: models.Config{IntegrationBranch: "main"},
	}

	// State after: task STILL exhausted (orchestrator couldn't resolve)
	exhaustedTaskAfter := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	exhaustedTaskAfter.FailedBy = []string{"coder-1", "coder-2"}
	stateAfter := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Agents: map[string]models.Agent{
			"orchestrator-1": {Role: "orchestrator", Status: models.AgentStatusIdle, Heartbeat: now},
		},
		Tasks:  []models.Task{exhaustedTaskAfter},
		Config: models.Config{IntegrationBranch: "main"},
	}

	testhelpers.WriteInitialState(t, statePath, stateAfter)

	bb := db.New(statePath)

	err := verifyOrchestratorStateChanges(bb, stateBefore, nil, nil)
	if err != nil {
		t.Errorf("Expected no error for no-op HYPOTHESIS_EXHAUSTED exit (may require human intervention), got: %v", err)
	}
}
