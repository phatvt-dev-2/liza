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

	// State after: task is IMPLEMENTING (by coder)
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
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
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
			testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReady, now),
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

	// State after: 1 IMPLEMENTING, 1 SUPERSEDED, 1 still INTEGRATION_FAILED
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
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
			testhelpers.BuildTaskByStatus("task-2", models.TaskStatusSuperseded, now),
			testhelpers.BuildTaskByStatus("task-3", models.TaskStatusIntegrationFailed, now),
			testhelpers.BuildTaskByStatus("task-4", models.TaskStatusReady, now),
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
