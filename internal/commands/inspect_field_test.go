package commands

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
)

func TestGetField(t *testing.T) {
	now := time.Now()
	state := &models.State{
		Version: 1,
		Config: models.Config{
			Mode:               models.SystemModeRunning,
			MaxCoderIterations: 10,
			MaxReviewCycles:    5,
			IntegrationBranch:  "main",
		},
		Sprint: models.Sprint{
			ID:     "sprint-1",
			Status: models.SprintStatusInProgress,
			Timeline: models.SprintTimeline{
				Started:  now.Add(-2 * time.Hour),
				Deadline: now.Add(6 * time.Hour),
			},
			Metrics: models.SprintMetrics{
				TasksDone:       5,
				TasksInProgress: 3,
				TasksBlocked:    1,
			},
		},
		Tasks: []models.Task{
			{
				ID:          "task-1",
				Description: "Implement feature X",
				Status:      models.TaskStatusClaimed,
				Priority:    1,
				Created:     now.Add(-1 * time.Hour),
			},
		},
		Agents: map[string]models.Agent{
			"coder-1": {
				Role:      "coder",
				Status:    models.AgentStatusWorking,
				Heartbeat: now.Add(-5 * time.Second),
			},
		},
	}

	tests := []struct {
		name      string
		fieldPath string
		want      any
		wantErr   bool
	}{
		{
			name:      "config.mode",
			fieldPath: "config.mode",
			want:      "RUNNING",
			wantErr:   false,
		},
		{
			name:      "config.max_coder_iterations",
			fieldPath: "config.max_coder_iterations",
			want:      10,
			wantErr:   false,
		},
		{
			name:      "sprint.status",
			fieldPath: "sprint.status",
			want:      "IN_PROGRESS",
			wantErr:   false,
		},
		{
			name:      "sprint.id",
			fieldPath: "sprint.id",
			want:      "sprint-1",
			wantErr:   false,
		},
		{
			name:      "sprint.metrics.tasks_done",
			fieldPath: "sprint.metrics.tasks_done",
			want:      5,
			wantErr:   false,
		},
		{
			name:      "invalid field",
			fieldPath: "config.nonexistent",
			want:      nil,
			wantErr:   true,
		},
		{
			name:      "invalid entity",
			fieldPath: "nonexistent.field",
			want:      nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getField(state, tt.fieldPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("getField() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("getField() = %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestGetComputedField(t *testing.T) {
	now := time.Now()
	state := &models.State{
		Sprint: models.Sprint{
			ID:     "sprint-1",
			Status: models.SprintStatusInProgress,
			Timeline: models.SprintTimeline{
				Started:  now.Add(-2 * time.Hour),
				Deadline: now.Add(6 * time.Hour),
			},
			Metrics: models.SprintMetrics{
				TasksDone:       5,
				TasksInProgress: 3,
				TasksBlocked:    1,
			},
		},
		Tasks: []models.Task{
			{ID: "task-1", Status: models.TaskStatusMerged},
			{ID: "task-2", Status: models.TaskStatusMerged},
			{ID: "task-3", Status: models.TaskStatusMerged},
			{ID: "task-4", Status: models.TaskStatusMerged},
			{ID: "task-5", Status: models.TaskStatusMerged},
			{ID: "task-6", Status: models.TaskStatusClaimed},
			{ID: "task-7", Status: models.TaskStatusClaimed},
			{ID: "task-8", Status: models.TaskStatusClaimed},
			{ID: "task-9", Status: models.TaskStatusUnclaimed},
		},
		Agents: map[string]models.Agent{
			"coder-1": {
				Role:      "coder",
				Status:    models.AgentStatusWorking,
				Heartbeat: now.Add(-5 * time.Second),
			},
			"coder-2": {
				Role:      "coder",
				Status:    models.AgentStatusIdle,
				Heartbeat: now.Add(-3 * time.Second),
			},
			"reviewer-1": {
				Role:      "code-reviewer",
				Status:    models.AgentStatusWorking,
				Heartbeat: now.Add(-2 * time.Second),
			},
		},
	}

	tests := []struct {
		name      string
		fieldPath string
		wantType  string // Type check instead of exact value (for time-based fields)
		wantErr   bool
	}{
		{
			name:      "agents.active_count",
			fieldPath: "agents.active_count",
			wantType:  "int",
			wantErr:   false,
		},
		{
			name:      "agents.utilization",
			fieldPath: "agents.utilization",
			wantType:  "float64",
			wantErr:   false,
		},
		{
			name:      "tasks.completion_rate",
			fieldPath: "tasks.completion_rate",
			wantType:  "float64",
			wantErr:   false,
		},
		{
			name:      "sprint.elapsed",
			fieldPath: "sprint.elapsed",
			wantType:  "string",
			wantErr:   false,
		},
		{
			name:      "sprint.remaining",
			fieldPath: "sprint.remaining",
			wantType:  "string",
			wantErr:   false,
		},
		{
			name:      "sprint.progress_percent",
			fieldPath: "sprint.progress_percent",
			wantType:  "float64",
			wantErr:   false,
		},
		{
			name:      "invalid computed field",
			fieldPath: "sprint.nonexistent",
			wantType:  "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getComputedField(state, tt.fieldPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("getComputedField() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Type check
			switch tt.wantType {
			case "int":
				if _, ok := got.(int); !ok {
					t.Errorf("getComputedField() type = %T, want int", got)
				}
			case "float64":
				if _, ok := got.(float64); !ok {
					t.Errorf("getComputedField() type = %T, want float64", got)
				}
			case "string":
				if _, ok := got.(string); !ok {
					t.Errorf("getComputedField() type = %T, want string", got)
				}
			}
		})
	}
}

func TestGetComputedFieldAgentSpecific(t *testing.T) {
	now := time.Now()
	state := &models.State{
		Agents: map[string]models.Agent{
			"coder-1": {
				Role:      "coder",
				Status:    models.AgentStatusWorking,
				Heartbeat: now.Add(-30 * time.Second),
			},
		},
		Tasks: []models.Task{
			{
				ID:     "task-1",
				Status: models.TaskStatusClaimed,
				History: []models.TaskHistoryEntry{
					{
						Time:  now.Add(-15 * time.Minute),
						Event: "claimed",
						Agent: stringPtr("coder-1"),
					},
				},
			},
		},
	}

	tests := []struct {
		name      string
		fieldPath string
		wantType  string
	}{
		{
			name:      "agent.coder-1.time_since_heartbeat",
			fieldPath: "agent.coder-1.time_since_heartbeat",
			wantType:  "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getComputedField(state, tt.fieldPath)
			if err != nil {
				t.Errorf("getComputedField() error = %v", err)
				return
			}

			// Check type and that it's a duration string
			if str, ok := got.(string); ok {
				if !strings.Contains(str, "s") && !strings.Contains(str, "m") && !strings.Contains(str, "h") {
					t.Errorf("getComputedField() = %q, doesn't look like a duration", str)
				}
			} else {
				t.Errorf("getComputedField() type = %T, want string", got)
			}
		})
	}
}

func TestGetComputedFieldTaskSpecific(t *testing.T) {
	now := time.Now()
	state := &models.State{
		Tasks: []models.Task{
			{
				ID:      "task-1",
				Status:  models.TaskStatusClaimed,
				Created: now.Add(-2 * time.Hour),
				History: []models.TaskHistoryEntry{
					{
						Time:  now.Add(-30 * time.Minute),
						Event: "claimed",
					},
				},
			},
		},
	}

	tests := []struct {
		name      string
		fieldPath string
		wantType  string
	}{
		{
			name:      "task.task-1.age",
			fieldPath: "task.task-1.age",
			wantType:  "string",
		},
		{
			name:      "task.task-1.time_in_status",
			fieldPath: "task.task-1.time_in_status",
			wantType:  "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getComputedField(state, tt.fieldPath)
			if err != nil {
				t.Errorf("getComputedField() error = %v", err)
				return
			}

			// Check type and that it's a duration string
			if str, ok := got.(string); ok {
				if !strings.Contains(str, "s") && !strings.Contains(str, "m") && !strings.Contains(str, "h") {
					t.Errorf("getComputedField() = %q, doesn't look like a duration", str)
				}
			} else {
				t.Errorf("getComputedField() type = %T, want string", got)
			}
		})
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
