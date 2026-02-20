package ops

import (
	"strings"
	"testing"
	"time"

	"os"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestDeleteAgent_Validation(t *testing.T) {
	_, err := DeleteAgent("/nonexistent", "", false, false, "reason")
	if err == nil {
		t.Fatal("Expected error for empty agent ID")
	}
	if !strings.Contains(err.Error(), "agent ID required") {
		t.Errorf("Error = %q, want to contain 'agent ID required'", err.Error())
	}
}

func TestDeleteAgent_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := DeleteAgent(tmpDir, "nonexistent", false, false, "reason")
	if err == nil {
		t.Fatal("Expected error for nonexistent agent")
	}
	if !strings.Contains(err.Error(), "agent not found") {
		t.Errorf("Error = %q, want to contain 'agent not found'", err.Error())
	}
}

func TestDeleteAgent_IdleAgent(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:   "coder",
		Status: models.AgentStatusIdle,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := DeleteAgent(tmpDir, "coder-1", false, false, "no longer needed")
	if err != nil {
		t.Fatalf("DeleteAgent() error: %v", err)
	}
	if result.AgentID != "coder-1" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "coder-1")
	}

	// Verify agent removed
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if _, exists := readState.Agents["coder-1"]; exists {
		t.Error("Agent should be removed from state")
	}

	// Verify human note added
	if len(readState.HumanNotes) == 0 {
		t.Fatal("Expected human note to be added")
	}
	lastNote := readState.HumanNotes[len(readState.HumanNotes)-1]
	if !strings.Contains(lastNote.Message, "coder-1") {
		t.Errorf("Note message = %q, want to contain agent ID", lastNote.Message)
	}
}

func TestDeleteAgent_ActiveLease_NoForce(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	leaseExpires := now.Add(30 * time.Minute)
	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:         "coder",
		Status:       models.AgentStatusWorking,
		LeaseExpires: &leaseExpires,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := DeleteAgent(tmpDir, "coder-1", false, false, "reason")
	if err == nil {
		t.Fatal("Expected error for active lease without force")
	}
	if !strings.Contains(err.Error(), "active lease") {
		t.Errorf("Error = %q, want to contain 'active lease'", err.Error())
	}
}

func TestDeleteAgent_ActiveLease_Force(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	leaseExpires := now.Add(30 * time.Minute)
	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:         "coder",
		Status:       models.AgentStatusWorking,
		LeaseExpires: &leaseExpires,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := DeleteAgent(tmpDir, "coder-1", true, false, "force remove")
	if err != nil {
		t.Fatalf("DeleteAgent() with force error: %v", err)
	}
	if result.AgentID != "coder-1" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "coder-1")
	}
}

func TestDeleteAgent_BusyWithTask_NoForce(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	taskRef := "task-1"
	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:        "coder",
		Status:      models.AgentStatusWorking,
		CurrentTask: &taskRef,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := DeleteAgent(tmpDir, "coder-1", false, false, "reason")
	if err == nil {
		t.Fatal("Expected error for busy agent without force")
	}
	if !strings.Contains(err.Error(), "working on task") {
		t.Errorf("Error = %q, want to contain 'working on task'", err.Error())
	}
}

func TestDeleteAgent_AllowRunningPID_BypassesPIDOnly(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	leaseExpires := now.Add(30 * time.Minute)
	taskRef := "task-1"
	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:         "coder",
		Status:       models.AgentStatusWorking,
		PID:          os.Getpid(), // alive PID
		LeaseExpires: &leaseExpires,
		CurrentTask:  &taskRef,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	// allowRunningPID=true should still refuse due to active lease
	_, err := DeleteAgent(tmpDir, "coder-1", false, true, "pid confirmed")
	if err == nil {
		t.Fatal("Expected error: allowRunningPID should not bypass lease check")
	}
	if !strings.Contains(err.Error(), "active lease") {
		t.Errorf("Error = %q, want to contain 'active lease'", err.Error())
	}
}

func TestDeleteAgent_AllowRunningPID_BypassesPIDWithNoLease(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:   "coder",
		Status: models.AgentStatusIdle,
		PID:    os.Getpid(), // alive PID
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	// allowRunningPID=true with no lease/task should succeed
	result, err := DeleteAgent(tmpDir, "coder-1", false, true, "pid confirmed")
	if err != nil {
		t.Fatalf("DeleteAgent() error: %v", err)
	}
	if result.AgentID != "coder-1" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "coder-1")
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if _, exists := readState.Agents["coder-1"]; exists {
		t.Error("Agent should be removed from state")
	}
}

func TestDeleteAgent_BusyWithTask_Force(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	taskRef := "task-1"
	state := testhelpers.CreateValidState()
	state.Agents["coder-1"] = models.Agent{
		Role:        "coder",
		Status:      models.AgentStatusWorking,
		CurrentTask: &taskRef,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := DeleteAgent(tmpDir, "coder-1", true, false, "force remove")
	if err != nil {
		t.Fatalf("DeleteAgent() with force error: %v", err)
	}
	if result.AgentID != "coder-1" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "coder-1")
	}
}
