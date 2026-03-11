package ops

import (
	"testing"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestFindOrchestratorID(t *testing.T) {
	tests := []struct {
		name      string
		agents    map[string]models.Agent
		wantID    string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "single orchestrator found",
			agents: map[string]models.Agent{
				"orchestrator-1": {Role: "orchestrator"},
				"coder-1":        {Role: "coder"},
			},
			wantID: "orchestrator-1",
		},
		{
			name: "custom orchestrator ID",
			agents: map[string]models.Agent{
				"my-orch-1": {Role: "orchestrator"},
				"coder-1":   {Role: "coder"},
			},
			wantID: "my-orch-1",
		},
		{
			name: "no orchestrator registered",
			agents: map[string]models.Agent{
				"coder-1": {Role: "coder"},
			},
			wantErr:   true,
			errSubstr: "no orchestrator agent registered",
		},
		{
			name:      "empty agents map",
			agents:    map[string]models.Agent{},
			wantErr:   true,
			errSubstr: "no orchestrator agent registered",
		},
		{
			name: "multiple orchestrators",
			agents: map[string]models.Agent{
				"orchestrator-1": {Role: "orchestrator"},
				"orchestrator-2": {Role: "orchestrator"},
			},
			wantErr:   true,
			errSubstr: "multiple orchestrators registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{Agents: tt.agents}
			id, err := state.FindOrchestratorID()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				testhelpers.AssertErrorContains(t, err, tt.errSubstr)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tt.wantID {
				t.Errorf("got %q, want %q", id, tt.wantID)
			}
		})
	}
}

func TestResolveOrchestratorFromState(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Agents["orchestrator-1"] = models.Agent{Role: "orchestrator"}
	testhelpers.WriteInitialState(t, stateFile, state)

	id, err := ResolveOrchestratorFromState(stateFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "orchestrator-1" {
		t.Errorf("got %q, want %q", id, "orchestrator-1")
	}
}

func TestResolveOrchestratorFromState_NoOrchestrator(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ResolveOrchestratorFromState(stateFile)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	testhelpers.AssertErrorContains(t, err, "no orchestrator agent registered")
}
