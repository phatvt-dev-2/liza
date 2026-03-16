package commands

import (
	"os"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
	"gopkg.in/yaml.v3"
)

func TestMigrateCommand_NormalizesUnderscoreRoles(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Agents = map[string]models.Agent{
		"coder-1": {
			Role:      "coder",
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-1",
		},
		"code-reviewer-1": {
			Role:      "code_reviewer", // underscore form — should be normalized
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-2",
		},
		"orchestrator-1": {
			Role:      "orchestrator",
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-3",
		},
	}
	testhelpers.WriteInitialState(t, statePath, state)

	changed, err := MigrateCommand(statePath)
	if err != nil {
		t.Fatalf("MigrateCommand() error = %v", err)
	}
	if !changed {
		t.Error("MigrateCommand() changed = false, want true")
	}

	// Verify the file was updated
	bb := db.New(statePath)
	updated, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read updated state: %v", err)
	}

	agent := updated.Agents["code-reviewer-1"]
	if agent.Role != "code-reviewer" {
		t.Errorf("Agent role = %q, want %q", agent.Role, "code-reviewer")
	}
}

func TestMigrateCommand_AlreadyMigratedNoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Agents = map[string]models.Agent{
		"coder-1": {
			Role:      "coder",
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-1",
		},
		"code-reviewer-1": {
			Role:      "code-reviewer", // already hyphenated
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-2",
		},
	}
	testhelpers.WriteInitialState(t, statePath, state)

	// Capture file mod time before migration
	infoBefore, _ := os.Stat(statePath)

	changed, err := MigrateCommand(statePath)
	if err != nil {
		t.Fatalf("MigrateCommand() error = %v", err)
	}
	if changed {
		t.Error("MigrateCommand() changed = true, want false (already migrated)")
	}

	// File should not have been modified
	infoAfter, _ := os.Stat(statePath)
	if infoBefore.ModTime() != infoAfter.ModTime() {
		t.Error("State file was modified even though no changes were needed")
	}
}

func TestMigrateCommand_MultipleUnderscoreRoles(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Agents = map[string]models.Agent{
		"code-planner-1": {
			Role:      "code_planner",
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-1",
		},
		"epic-plan-reviewer-1": {
			Role:      "epic_plan_reviewer",
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-2",
		},
		"us-writer-1": {
			Role:      "us_writer",
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-3",
		},
	}
	testhelpers.WriteInitialState(t, statePath, state)

	changed, err := MigrateCommand(statePath)
	if err != nil {
		t.Fatalf("MigrateCommand() error = %v", err)
	}
	if !changed {
		t.Error("MigrateCommand() changed = false, want true")
	}

	bb := db.New(statePath)
	updated, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read updated state: %v", err)
	}

	expectations := map[string]string{
		"code-planner-1":       "code-planner",
		"epic-plan-reviewer-1": "epic-plan-reviewer",
		"us-writer-1":          "us-writer",
	}
	for agentID, wantRole := range expectations {
		got := updated.Agents[agentID].Role
		if got != wantRole {
			t.Errorf("Agent %q role = %q, want %q", agentID, got, wantRole)
		}
	}
}

func TestMigrateCommand_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Agents = map[string]models.Agent{
		"code-reviewer-1": {
			Role:      "code_reviewer",
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-1",
		},
	}
	testhelpers.WriteInitialState(t, statePath, state)

	// First migration — should change
	changed1, err := MigrateCommand(statePath)
	if err != nil {
		t.Fatalf("First MigrateCommand() error = %v", err)
	}
	if !changed1 {
		t.Error("First MigrateCommand() changed = false, want true")
	}

	// Second migration — should report no changes
	changed2, err := MigrateCommand(statePath)
	if err != nil {
		t.Fatalf("Second MigrateCommand() error = %v", err)
	}
	if changed2 {
		t.Error("Second MigrateCommand() changed = true, want false")
	}
}

func TestMigrateCommand_UnknownRolePassesThrough(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Agents = map[string]models.Agent{
		"custom-1": {
			Role:      "custom_agent",
			Status:    models.AgentStatusIdle,
			Heartbeat: now,
			Terminal:  "term-1",
		},
	}
	testhelpers.WriteInitialState(t, statePath, state)

	changed, err := MigrateCommand(statePath)
	if err != nil {
		t.Fatalf("MigrateCommand() error = %v", err)
	}
	if changed {
		t.Error("MigrateCommand() changed = true, want false (unknown role should not be modified)")
	}

	// Verify the unknown role was not mutated
	data, _ := os.ReadFile(statePath)
	var updated models.State
	_ = yaml.Unmarshal(data, &updated)
	if updated.Agents["custom-1"].Role != "custom_agent" {
		t.Errorf("Unknown role was mutated: got %q, want %q", updated.Agents["custom-1"].Role, "custom_agent")
	}
}

func TestMigrateCommand_NoAgents(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Agents = map[string]models.Agent{}
	testhelpers.WriteInitialState(t, statePath, state)

	changed, err := MigrateCommand(statePath)
	if err != nil {
		t.Fatalf("MigrateCommand() error = %v", err)
	}
	if changed {
		t.Error("MigrateCommand() changed = true, want false (no agents to migrate)")
	}
}

func TestMigrateCommand_InvalidStatePath(t *testing.T) {
	_, err := MigrateCommand("/nonexistent/path/state.yaml")
	if err == nil {
		t.Error("MigrateCommand() error = nil, want error for nonexistent file")
	}
}
