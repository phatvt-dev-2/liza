package commands

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"gopkg.in/yaml.v3"
)

func TestInspectAgents(t *testing.T) {
	// Create test state with various agents
	now := time.Now()
	task1 := "task-1"
	task2 := "task-2"
	leaseExpires := now.Add(5 * time.Minute)

	state := &models.State{
		Agents: map[string]models.Agent{
			"coder-1": {
				Role:            "coder",
				Status:          models.AgentStatusWorking,
				PID:             os.Getpid(),
				CurrentTask:     &task1,
				LeaseExpires:    &leaseExpires,
				Heartbeat:       now.Add(-30 * time.Second),
				Terminal:        "terminal1",
				IterationsTotal: 5,
				ContextPercent:  45,
			},
			"coder-2": {
				Role:            "coder",
				Status:          models.AgentStatusIdle,
				PID:             0,
				CurrentTask:     nil,
				LeaseExpires:    nil,
				Heartbeat:       now.Add(-2 * time.Minute),
				Terminal:        "terminal2",
				IterationsTotal: 3,
				ContextPercent:  20,
			},
			"reviewer-1": {
				Role:            "reviewer",
				Status:          models.AgentStatusReviewing,
				PID:             999999,
				CurrentTask:     &task2,
				LeaseExpires:    &leaseExpires,
				Heartbeat:       now.Add(-10 * time.Second),
				Terminal:        "terminal3",
				IterationsTotal: 8,
				ContextPercent:  60,
			},
		},
		Tasks: []models.Task{
			{
				ID:         "task-1",
				Status:     models.TaskStatusImplementing,
				AssignedTo: strPtr("coder-1"),
				Created:    now.Add(-1 * time.Hour),
				History: []models.TaskHistoryEntry{
					{Time: now.Add(-30 * time.Minute), Event: "claimed"},
				},
			},
		},
	}

	tests := []struct {
		name       string
		opts       inspectAgentsOptions
		wantCount  int
		wantIDs    []string
		wantFormat string // "json", "yaml", "table", or "internal"
		wantErr    bool
	}{
		{
			name:       "list all agents",
			opts:       inspectAgentsOptions{},
			wantCount:  3,
			wantIDs:    []string{"coder-1", "coder-2", "reviewer-1"},
			wantFormat: "table",
		},
		{
			name: "filter by role coder",
			opts: inspectAgentsOptions{
				RoleFilter: "coder",
			},
			wantCount:  2,
			wantIDs:    []string{"coder-1", "coder-2"},
			wantFormat: "table",
		},
		{
			name: "filter by role reviewer",
			opts: inspectAgentsOptions{
				RoleFilter: "reviewer",
			},
			wantCount:  1,
			wantIDs:    []string{"reviewer-1"},
			wantFormat: "table",
		},
		{
			name: "filter by status WORKING",
			opts: inspectAgentsOptions{
				StatusFilter: string(models.AgentStatusWorking),
			},
			wantCount:  1,
			wantIDs:    []string{"coder-1"},
			wantFormat: "table",
		},
		{
			name: "filter by status IDLE",
			opts: inspectAgentsOptions{
				StatusFilter: string(models.AgentStatusIdle),
			},
			wantCount:  1,
			wantIDs:    []string{"coder-2"},
			wantFormat: "table",
		},
		{
			name: "JSON format",
			opts: inspectAgentsOptions{
				Format: "json",
			},
			wantCount:  3,
			wantFormat: "json",
		},
		{
			name: "YAML format",
			opts: inspectAgentsOptions{
				Format: "yaml",
			},
			wantCount:  3,
			wantFormat: "yaml",
		},
		{
			name: "internal flag returns structured data",
			opts: inspectAgentsOptions{
				Internal: true,
			},
			wantCount:  3,
			wantFormat: "internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := inspectAgents(state, tt.opts)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Validate result based on format
			switch tt.wantFormat {
			case "internal":
				// Should return []agentInfo
				agents, ok := result.([]agentInfo)
				if !ok {
					t.Errorf("expected []agentInfo, got %T", result)
					return
				}
				if len(agents) != tt.wantCount {
					t.Errorf("expected %d agents, got %d", tt.wantCount, len(agents))
				}
				// Check computed fields are present
				for _, agent := range agents {
					if agent.TimeSinceHeartbeat == "" {
						t.Errorf("expected TimeSinceHeartbeat to be computed for agent %s", agent.ID)
					}
					// TimeOnTask may be empty if agent is idle
				}
			case "json":
				output, ok := result.(string)
				if !ok {
					t.Errorf("expected string output, got %T", result)
					return
				}
				// Validate JSON
				var agents []agentInfo
				if err := json.Unmarshal([]byte(output), &agents); err != nil {
					t.Errorf("invalid JSON output: %v", err)
				}
				if len(agents) != tt.wantCount {
					t.Errorf("expected %d agents in JSON, got %d", tt.wantCount, len(agents))
				}
			case "yaml":
				output, ok := result.(string)
				if !ok {
					t.Errorf("expected string output, got %T", result)
					return
				}
				// Just check it's not empty
				if output == "" {
					t.Errorf("expected non-empty YAML output")
				}
			case "table":
				output, ok := result.(string)
				if !ok {
					t.Errorf("expected string output, got %T", result)
					return
				}
				// Check that all expected IDs appear in output
				for _, id := range tt.wantIDs {
					if !strings.Contains(output, id) {
						t.Errorf("expected output to contain %s", id)
					}
				}
			}
		})
	}
}

func TestInspectAgent(t *testing.T) {
	now := time.Now()
	task1 := "task-1"
	leaseExpires := now.Add(5 * time.Minute)

	state := &models.State{
		Agents: map[string]models.Agent{
			"coder-1": {
				Role:            "coder",
				Status:          models.AgentStatusWorking,
				PID:             os.Getpid(),
				CurrentTask:     &task1,
				LeaseExpires:    &leaseExpires,
				Heartbeat:       now.Add(-30 * time.Second),
				Terminal:        "terminal1",
				IterationsTotal: 5,
				ContextPercent:  45,
			},
			"reviewer-1": {
				Role:            "reviewer",
				Status:          models.AgentStatusIdle,
				PID:             0,
				CurrentTask:     nil,
				LeaseExpires:    nil,
				Heartbeat:       now.Add(-2 * time.Minute),
				Terminal:        "terminal2",
				IterationsTotal: 3,
				ContextPercent:  20,
			},
		},
		Tasks: []models.Task{
			{
				ID:         "task-1",
				Status:     models.TaskStatusImplementing,
				AssignedTo: strPtr("coder-1"),
				Created:    now.Add(-1 * time.Hour),
				History: []models.TaskHistoryEntry{
					{Time: now.Add(-30 * time.Minute), Event: "claimed"},
				},
			},
		},
	}

	tests := []struct {
		name         string
		agentID      string
		opts         inspectAgentsOptions
		wantAgentID  string
		wantErr      bool
		wantNotFound bool
	}{
		{
			name:        "get agent by ID",
			agentID:     "coder-1",
			opts:        inspectAgentsOptions{},
			wantAgentID: "coder-1",
		},
		{
			name:        "get agent with JSON format",
			agentID:     "coder-1",
			opts:        inspectAgentsOptions{Format: "json"},
			wantAgentID: "coder-1",
		},
		{
			name:        "get agent with YAML format",
			agentID:     "reviewer-1",
			opts:        inspectAgentsOptions{Format: "yaml"},
			wantAgentID: "reviewer-1",
		},
		{
			name:        "get agent with value format",
			agentID:     "coder-1",
			opts:        inspectAgentsOptions{Format: "value"},
			wantAgentID: "coder-1",
		},
		{
			name:         "agent not found",
			agentID:      "nonexistent",
			opts:         inspectAgentsOptions{},
			wantErr:      true,
			wantNotFound: true,
		},
		{
			name:        "internal flag returns agentInfo",
			agentID:     "coder-1",
			opts:        inspectAgentsOptions{Internal: true},
			wantAgentID: "coder-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := inspectAgent(state, tt.agentID, tt.opts)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if tt.wantNotFound && !errors.IsNotFound(err) {
					t.Errorf("expected NotFoundError, got %T: %v", err, err)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Validate result based on format
			if tt.opts.Internal {
				agentInfo, ok := result.(agentInfo)
				if !ok {
					t.Errorf("expected agentInfo, got %T", result)
					return
				}
				if agentInfo.ID != tt.wantAgentID {
					t.Errorf("expected agent ID %s, got %s", tt.wantAgentID, agentInfo.ID)
				}
				// Verify computed fields are present
				if agentInfo.TimeSinceHeartbeat == "" {
					t.Errorf("expected TimeSinceHeartbeat to be computed")
				}
			} else {
				output, ok := result.(string)
				if !ok {
					t.Errorf("expected string output, got %T", result)
					return
				}
				// Check output contains agent ID
				if !strings.Contains(output, tt.wantAgentID) {
					t.Errorf("expected output to contain agent ID %s", tt.wantAgentID)
				}
			}
		})
	}
}

func TestAgentInfo_ComputedFields(t *testing.T) {
	now := time.Now()
	task1 := "task-1"
	// Use 5 minutes + 1 second to account for time passage during test execution
	// This ensures formatDuration() rounds down to 5m, not 4m
	leaseExpires := now.Add(5*time.Minute + 1*time.Second)

	agent := models.Agent{
		Role:            "coder",
		Status:          models.AgentStatusWorking,
		PID:             os.Getpid(),
		CurrentTask:     &task1,
		LeaseExpires:    &leaseExpires,
		Heartbeat:       now.Add(-2 * time.Minute),
		Terminal:        "terminal1",
		IterationsTotal: 5,
		ContextPercent:  45,
	}

	task := &models.Task{
		ID:         "task-1",
		Status:     models.TaskStatusImplementing,
		AssignedTo: strPtr("coder-1"),
		Created:    now.Add(-1 * time.Hour),
		History: []models.TaskHistoryEntry{
			{Time: now.Add(-30 * time.Minute), Event: "claimed"},
		},
	}

	info := buildagentInfo("coder-1", &agent, task)

	// Check that computed fields are set
	if info.TimeSinceHeartbeat == "" {
		t.Errorf("expected TimeSinceHeartbeat to be set")
	}
	if info.TimeOnTask == "" {
		t.Errorf("expected TimeOnTask to be set when agent has current task")
	}

	// TimeSinceHeartbeat should be approximately 2 minutes
	if !strings.Contains(info.TimeSinceHeartbeat, "2m") {
		t.Errorf("expected TimeSinceHeartbeat to contain '2m', got %s", info.TimeSinceHeartbeat)
	}

	// TimeOnTask should be approximately 30 minutes
	if !strings.Contains(info.TimeOnTask, "30m") {
		t.Errorf("expected TimeOnTask to contain '30m', got %s", info.TimeOnTask)
	}

	// LeaseExpires should be approximately 5 minutes
	if info.LeaseExpires == nil {
		t.Errorf("expected LeaseExpires to be set, got nil")
	} else if !strings.Contains(*info.LeaseExpires, "5m") {
		t.Errorf("expected LeaseExpires to contain '5m', got %q", *info.LeaseExpires)
	}
}

func TestAgentInfo_IdleAgent(t *testing.T) {
	now := time.Now()

	agent := models.Agent{
		Role:            "coder",
		Status:          models.AgentStatusIdle,
		PID:             0,
		CurrentTask:     nil,
		LeaseExpires:    nil,
		Heartbeat:       now.Add(-30 * time.Second),
		Terminal:        "terminal1",
		IterationsTotal: 3,
		ContextPercent:  20,
	}

	info := buildagentInfo("coder-2", &agent, nil)

	// Check that computed fields are set appropriately
	if info.TimeSinceHeartbeat == "" {
		t.Errorf("expected TimeSinceHeartbeat to be set")
	}

	// TimeOnTask should be empty for idle agent
	if info.TimeOnTask != "" {
		t.Errorf("expected TimeOnTask to be empty for idle agent, got %s", info.TimeOnTask)
	}

	// LeaseExpires should be nil
	if info.LeaseExpires != nil {
		t.Errorf("expected LeaseExpires to be nil for idle agent, got %v", info.LeaseExpires)
	}
}

// Helper function
func strPtr(s string) *string {
	return &s
}

// TestIsAgentProcessAlive_InspectAgents tests the process checking function
func TestIsAgentProcessAlive_InspectAgents(t *testing.T) {
	tests := []struct {
		name      string
		pid       int
		wantAlive bool
	}{
		{
			name:      "current process is alive",
			pid:       os.Getpid(),
			wantAlive: true,
		},
		{
			name:      "PID 0 is not alive",
			pid:       0,
			wantAlive: false,
		},
		{
			name:      "negative PID is not alive",
			pid:       -1,
			wantAlive: false,
		},
		{
			name:      "high PID unlikely to exist",
			pid:       999999,
			wantAlive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ops.IsProcessAlive(tt.pid)
			if got != tt.wantAlive {
				t.Errorf("ops.IsProcessAlive(%d) = %v, want %v", tt.pid, got, tt.wantAlive)
			}
		})
	}
}

// TestAgentInfo_PIDAndProcessStatus tests that buildagentInfo populates PID and ProcessStatus
func TestAgentInfo_PIDAndProcessStatus(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name              string
		agentPID          int
		wantProcessStatus string
	}{
		{
			name:              "PID 0 shows n/a",
			agentPID:          0,
			wantProcessStatus: "n/a",
		},
		{
			name:              "current process shows running",
			agentPID:          os.Getpid(),
			wantProcessStatus: "running",
		},
		{
			name:              "dead process shows not found",
			agentPID:          999999,
			wantProcessStatus: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := models.Agent{
				Role:            "coder",
				Status:          models.AgentStatusIdle,
				PID:             tt.agentPID,
				Heartbeat:       now,
				Terminal:        "terminal1",
				IterationsTotal: 1,
				ContextPercent:  10,
			}

			info := buildagentInfo("test-agent", &agent, nil)

			if info.PID != tt.agentPID {
				t.Errorf("buildagentInfo() PID = %d, want %d", info.PID, tt.agentPID)
			}
			if info.ProcessStatus != tt.wantProcessStatus {
				t.Errorf("buildagentInfo() ProcessStatus = %s, want %s", info.ProcessStatus, tt.wantProcessStatus)
			}
		})
	}
}

// TestFormatAgentsTable_IncludesPID tests that table format includes PID column
func TestFormatAgentsTable_IncludesPID(t *testing.T) {
	agents := []agentInfo{
		{
			ID:                 "agent-1",
			Role:               "coder",
			Status:             "WORKING",
			PID:                os.Getpid(),
			ProcessStatus:      "running",
			TimeSinceHeartbeat: "30s",
			ContextPercent:     45,
		},
		{
			ID:                 "agent-2",
			Role:               "coder",
			Status:             "IDLE",
			PID:                999999,
			ProcessStatus:      "not found",
			TimeSinceHeartbeat: "1m",
			ContextPercent:     20,
		},
		{
			ID:                 "agent-3",
			Role:               "reviewer",
			Status:             "IDLE",
			PID:                0,
			ProcessStatus:      "n/a",
			TimeSinceHeartbeat: "2m",
			ContextPercent:     10,
		},
	}

	output := formatAgentsTable(agents)

	// Check header includes PID
	if !strings.Contains(output, "PID") {
		t.Errorf("table output should contain PID header, got: %s", output)
	}

	// Check running process shows checkmark
	if !strings.Contains(output, "✓") {
		t.Errorf("table output should contain ✓ for running process")
	}

	// Check dead process shows X
	if !strings.Contains(output, "✗") {
		t.Errorf("table output should contain ✗ for dead process")
	}

	// Check PID 0 shows "- n/a"
	if !strings.Contains(output, "- n/a") {
		t.Errorf("table output should contain '- n/a' for PID 0")
	}
}

// TestFormatAgentValue_IncludesPID tests that value format includes PID and ProcessStatus
func TestFormatAgentValue_IncludesPID(t *testing.T) {
	tests := []struct {
		name           string
		agent          agentInfo
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "running process shows PID and status",
			agent: agentInfo{
				ID:                 "agent-1",
				Role:               "coder",
				Status:             "WORKING",
				PID:                12345,
				ProcessStatus:      "running",
				TimeSinceHeartbeat: "30s",
				Terminal:           "terminal1",
				IterationsTotal:    5,
				ContextPercent:     45,
			},
			wantContains: []string{
				"PID: 12345",
				"Process Status: running",
			},
		},
		{
			name: "PID 0 shows n/a",
			agent: agentInfo{
				ID:                 "agent-2",
				Role:               "coder",
				Status:             "IDLE",
				PID:                0,
				ProcessStatus:      "n/a",
				TimeSinceHeartbeat: "1m",
				Terminal:           "terminal2",
				IterationsTotal:    3,
				ContextPercent:     20,
			},
			wantContains: []string{
				"PID: n/a",
				"Process Status: n/a",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := formatAgentValue(tt.agent)

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("formatAgentValue() output should contain %q, got: %s", want, output)
				}
			}
		})
	}
}

// TestFormatAgentJSON_IncludesPID tests that JSON format includes PID and ProcessStatus
func TestFormatAgentJSON_IncludesPID(t *testing.T) {
	agent := agentInfo{
		ID:                 "agent-1",
		Role:               "coder",
		Status:             "WORKING",
		PID:                12345,
		ProcessStatus:      "running",
		TimeSinceHeartbeat: "30s",
		Terminal:           "terminal1",
		IterationsTotal:    5,
		ContextPercent:     45,
	}

	output, err := formatJSON(agent)
	if err != nil {
		t.Fatalf("formatJSON() error = %v", err)
	}

	// Parse JSON to verify fields
	var parsed agentInfo
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if parsed.PID != 12345 {
		t.Errorf("JSON output PID = %d, want 12345", parsed.PID)
	}
	if parsed.ProcessStatus != "running" {
		t.Errorf("JSON output ProcessStatus = %s, want running", parsed.ProcessStatus)
	}
}

// TestFormatAgentYAML_IncludesPID tests that YAML format includes PID and ProcessStatus
func TestFormatAgentYAML_IncludesPID(t *testing.T) {
	agent := agentInfo{
		ID:                 "agent-1",
		Role:               "coder",
		Status:             "WORKING",
		PID:                12345,
		ProcessStatus:      "running",
		TimeSinceHeartbeat: "30s",
		Terminal:           "terminal1",
		IterationsTotal:    5,
		ContextPercent:     45,
	}

	output, err := formatYAML(agent)
	if err != nil {
		t.Fatalf("formatYAML() error = %v", err)
	}

	// Parse YAML to verify fields
	var parsed agentInfo
	if err := yaml.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("failed to parse YAML output: %v", err)
	}

	if parsed.PID != 12345 {
		t.Errorf("YAML output PID = %d, want 12345", parsed.PID)
	}
	if parsed.ProcessStatus != "running" {
		t.Errorf("YAML output ProcessStatus = %s, want running", parsed.ProcessStatus)
	}
}
