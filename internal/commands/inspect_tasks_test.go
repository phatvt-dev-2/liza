package commands

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
)

func TestInspectTasks(t *testing.T) {
	// Create test state with various tasks
	now := time.Now()
	assignedTo := "coder-1"
	blockedReason := "waiting for input"

	state := &models.State{
		Tasks: []models.Task{
			{
				ID:          "task-1",
				Description: "Implement feature A",
				Status:      models.TaskStatusClaimed,
				Priority:    1,
				AssignedTo:  &assignedTo,
				Created:     now.Add(-2 * time.Hour),
				History: []models.TaskHistoryEntry{
					{Time: now.Add(-1 * time.Hour), Event: "claimed"},
				},
			},
			{
				ID:            "task-2",
				Description:   "Fix bug B",
				Status:        models.TaskStatusBlocked,
				Priority:      2,
				BlockedReason: &blockedReason,
				Created:       now.Add(-24 * time.Hour),
				History: []models.TaskHistoryEntry{
					{Time: now.Add(-23 * time.Hour), Event: "blocked"},
				},
			},
			{
				ID:          "task-3",
				Description: "Add tests C",
				Status:      models.TaskStatusMerged,
				Priority:    3,
				Created:     now.Add(-48 * time.Hour),
			},
			{
				ID:          "task-4",
				Description: "Unclaimed task",
				Status:      models.TaskStatusUnclaimed,
				Priority:    4,
				Created:     now.Add(-1 * time.Hour),
			},
		},
	}

	tests := []struct {
		name       string
		opts       inspectTasksOptions
		wantCount  int
		wantIDs    []string
		wantFormat string // "json", "yaml", "table", or ""
		wantErr    bool
	}{
		{
			name:       "list all tasks",
			opts:       inspectTasksOptions{},
			wantCount:  4,
			wantIDs:    []string{"task-1", "task-2", "task-3", "task-4"},
			wantFormat: "table",
		},
		{
			name: "filter by status CLAIMED",
			opts: inspectTasksOptions{
				StatusFilter: string(models.TaskStatusClaimed),
			},
			wantCount:  1,
			wantIDs:    []string{"task-1"},
			wantFormat: "table",
		},
		{
			name: "filter by status BLOCKED",
			opts: inspectTasksOptions{
				StatusFilter: string(models.TaskStatusBlocked),
			},
			wantCount:  1,
			wantIDs:    []string{"task-2"},
			wantFormat: "table",
		},
		{
			name: "filter by assigned_to",
			opts: inspectTasksOptions{
				AssignedToFilter: "coder-1",
			},
			wantCount:  1,
			wantIDs:    []string{"task-1"},
			wantFormat: "table",
		},
		{
			name: "filter by blocked=true",
			opts: inspectTasksOptions{
				BlockedFilter: true,
			},
			wantCount:  1,
			wantIDs:    []string{"task-2"},
			wantFormat: "table",
		},
		{
			name: "JSON format",
			opts: inspectTasksOptions{
				Format: "json",
			},
			wantCount:  4,
			wantFormat: "json",
		},
		{
			name: "YAML format",
			opts: inspectTasksOptions{
				Format: "yaml",
			},
			wantCount:  4,
			wantFormat: "yaml",
		},
		{
			name: "internal flag returns structured data",
			opts: inspectTasksOptions{
				Internal: true,
			},
			wantCount:  4,
			wantFormat: "internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := inspectTasks(state, tt.opts)
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
				// Should return []taskInfo
				tasks, ok := result.([]taskInfo)
				if !ok {
					t.Errorf("expected []taskInfo, got %T", result)
					return
				}
				if len(tasks) != tt.wantCount {
					t.Errorf("expected %d tasks, got %d", tt.wantCount, len(tasks))
				}
				// Check IDs if specified
				if tt.wantIDs != nil {
					for i, id := range tt.wantIDs {
						if tasks[i].ID != id {
							t.Errorf("expected task %d to be %s, got %s", i, id, tasks[i].ID)
						}
					}
				}
			case "json":
				output, ok := result.(string)
				if !ok {
					t.Errorf("expected string output, got %T", result)
					return
				}
				// Validate JSON
				var tasks []taskInfo
				if err := json.Unmarshal([]byte(output), &tasks); err != nil {
					t.Errorf("invalid JSON output: %v", err)
				}
				if len(tasks) != tt.wantCount {
					t.Errorf("expected %d tasks in JSON, got %d", tt.wantCount, len(tasks))
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

func TestInspectTask(t *testing.T) {
	now := time.Now()
	assignedTo := "coder-1"
	blockedReason := "waiting for approval"

	state := &models.State{
		Tasks: []models.Task{
			{
				ID:          "task-1",
				Description: "Implement feature A",
				Status:      models.TaskStatusClaimed,
				Priority:    1,
				AssignedTo:  &assignedTo,
				Created:     now.Add(-2 * time.Hour),
				History: []models.TaskHistoryEntry{
					{Time: now.Add(-30 * time.Minute), Event: "created"},
					{Time: now.Add(-1 * time.Hour), Event: "claimed"},
				},
			},
			{
				ID:            "task-2",
				Description:   "Blocked task",
				Status:        models.TaskStatusBlocked,
				Priority:      2,
				BlockedReason: &blockedReason,
				Created:       now.Add(-5 * time.Hour),
				History: []models.TaskHistoryEntry{
					{Time: now.Add(-5 * time.Hour), Event: "created"},
					{Time: now.Add(-4 * time.Hour), Event: "claimed"},
					{Time: now.Add(-2 * time.Hour), Event: "blocked"},
				},
			},
		},
	}

	tests := []struct {
		name         string
		taskID       string
		opts         inspectTasksOptions
		wantTaskID   string
		wantErr      bool
		wantNotFound bool
	}{
		{
			name:       "get task by ID",
			taskID:     "task-1",
			opts:       inspectTasksOptions{},
			wantTaskID: "task-1",
		},
		{
			name:       "get task with JSON format",
			taskID:     "task-1",
			opts:       inspectTasksOptions{Format: "json"},
			wantTaskID: "task-1",
		},
		{
			name:       "get task with YAML format",
			taskID:     "task-2",
			opts:       inspectTasksOptions{Format: "yaml"},
			wantTaskID: "task-2",
		},
		{
			name:       "get task with value format",
			taskID:     "task-1",
			opts:       inspectTasksOptions{Format: "value"},
			wantTaskID: "task-1",
		},
		{
			name:         "task not found",
			taskID:       "nonexistent",
			opts:         inspectTasksOptions{},
			wantErr:      true,
			wantNotFound: true,
		},
		{
			name:       "internal flag returns taskInfo",
			taskID:     "task-1",
			opts:       inspectTasksOptions{Internal: true},
			wantTaskID: "task-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := inspectTask(state, tt.taskID, tt.opts)
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
				taskInfo, ok := result.(taskInfo)
				if !ok {
					t.Errorf("expected taskInfo, got %T", result)
					return
				}
				if taskInfo.ID != tt.wantTaskID {
					t.Errorf("expected task ID %s, got %s", tt.wantTaskID, taskInfo.ID)
				}
				// Verify computed fields are present
				if taskInfo.Age == "" {
					t.Errorf("expected Age to be computed")
				}
				if taskInfo.TimeInStatus == "" {
					t.Errorf("expected TimeInStatus to be computed")
				}
			} else {
				output, ok := result.(string)
				if !ok {
					t.Errorf("expected string output, got %T", result)
					return
				}
				// Check output contains task ID
				if !strings.Contains(output, tt.wantTaskID) {
					t.Errorf("expected output to contain task ID %s", tt.wantTaskID)
				}
			}
		})
	}
}

func TestTaskInfo_ComputedFields(t *testing.T) {
	now := time.Now()
	assignedTo := "coder-1"

	task := models.Task{
		ID:          "task-1",
		Description: "Test task",
		Status:      models.TaskStatusClaimed,
		Priority:    1,
		AssignedTo:  &assignedTo,
		Created:     now.Add(-2 * time.Hour),
		History: []models.TaskHistoryEntry{
			{Time: now.Add(-2 * time.Hour), Event: "created"},
			{Time: now.Add(-1 * time.Hour), Event: "claimed"},
		},
	}

	info := buildtaskInfo(&task)

	// Check that computed fields are set
	if info.Age == "" {
		t.Errorf("expected Age to be set")
	}
	if info.TimeInStatus == "" {
		t.Errorf("expected TimeInStatus to be set")
	}

	// Age should be approximately 2 hours
	if !strings.Contains(info.Age, "2h") {
		t.Errorf("expected Age to contain '2h', got %s", info.Age)
	}

	// TimeInStatus should be approximately 1 hour (time since "claimed" event)
	if !strings.Contains(info.TimeInStatus, "1h") {
		t.Errorf("expected TimeInStatus to contain '1h', got %s", info.TimeInStatus)
	}
}

func TestTaskInfo_MultipleFilters(t *testing.T) {
	now := time.Now()
	assignedTo1 := "coder-1"
	assignedTo2 := "coder-2"
	blockedReason := "waiting"

	state := &models.State{
		Tasks: []models.Task{
			{
				ID:         "task-1",
				Status:     models.TaskStatusClaimed,
				AssignedTo: &assignedTo1,
				Created:    now,
			},
			{
				ID:         "task-2",
				Status:     models.TaskStatusClaimed,
				AssignedTo: &assignedTo2,
				Created:    now,
			},
			{
				ID:            "task-3",
				Status:        models.TaskStatusBlocked,
				AssignedTo:    &assignedTo1,
				BlockedReason: &blockedReason,
				Created:       now,
			},
		},
	}

	// Filter by status AND assigned_to
	opts := inspectTasksOptions{
		StatusFilter:     string(models.TaskStatusClaimed),
		AssignedToFilter: "coder-1",
		Internal:         true,
	}

	result, err := inspectTasks(state, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tasks := result.([]taskInfo)
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
	if len(tasks) > 0 && tasks[0].ID != "task-1" {
		t.Errorf("expected task-1, got %s", tasks[0].ID)
	}
}

func TestCalculateTimeInStatus(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name             string
		task             *models.Task
		expectedContains string // what the output should contain
	}{
		{
			name: "time since claimed",
			task: &models.Task{
				Status:  models.TaskStatusClaimed,
				Created: now.Add(-5 * time.Hour),
				History: []models.TaskHistoryEntry{
					{Time: now.Add(-5 * time.Hour), Event: "created"},
					{Time: now.Add(-2 * time.Hour), Event: "claimed"},
				},
			},
			expectedContains: "2h",
		},
		{
			name: "time since blocked",
			task: &models.Task{
				Status:  models.TaskStatusBlocked,
				Created: now.Add(-10 * time.Hour),
				History: []models.TaskHistoryEntry{
					{Time: now.Add(-10 * time.Hour), Event: "created"},
					{Time: now.Add(-8 * time.Hour), Event: "claimed"},
					{Time: now.Add(-3 * time.Hour), Event: "blocked"},
				},
			},
			expectedContains: "3h",
		},
		{
			name: "no history - use created time",
			task: &models.Task{
				Status:  models.TaskStatusUnclaimed,
				Created: now.Add(-1 * time.Hour),
				History: []models.TaskHistoryEntry{},
			},
			expectedContains: "1h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration := calculateTimeInStatus(tt.task)
			formatted := formatDuration(duration)
			if !strings.Contains(formatted, tt.expectedContains) {
				t.Errorf("expected duration to contain '%s', got '%s'", tt.expectedContains, formatted)
			}
		})
	}
}

func TestTaskInfo_DependenciesField(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		task          *models.Task
		wantDepsCount int
		wantDepsIDs   []string
	}{
		{
			name: "task with no dependencies",
			task: &models.Task{
				ID:        "task-1",
				Status:    models.TaskStatusUnclaimed,
				Created:   now,
				DependsOn: nil,
			},
			wantDepsCount: 0,
			wantDepsIDs:   nil,
		},
		{
			name: "task with empty dependencies",
			task: &models.Task{
				ID:        "task-2",
				Status:    models.TaskStatusUnclaimed,
				Created:   now,
				DependsOn: []string{},
			},
			wantDepsCount: 0,
			wantDepsIDs:   []string{},
		},
		{
			name: "task with single dependency",
			task: &models.Task{
				ID:        "task-3",
				Status:    models.TaskStatusUnclaimed,
				Created:   now,
				DependsOn: []string{"task-1"},
			},
			wantDepsCount: 1,
			wantDepsIDs:   []string{"task-1"},
		},
		{
			name: "task with multiple dependencies",
			task: &models.Task{
				ID:        "task-4",
				Status:    models.TaskStatusUnclaimed,
				Created:   now,
				DependsOn: []string{"task-1", "task-2", "task-3"},
			},
			wantDepsCount: 3,
			wantDepsIDs:   []string{"task-1", "task-2", "task-3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := buildtaskInfo(tt.task)

			if len(info.DependsOn) != tt.wantDepsCount {
				t.Errorf("expected %d dependencies, got %d", tt.wantDepsCount, len(info.DependsOn))
			}

			if tt.wantDepsIDs != nil {
				for i, id := range tt.wantDepsIDs {
					if info.DependsOn[i] != id {
						t.Errorf("expected dependency %d to be %s, got %s", i, id, info.DependsOn[i])
					}
				}
			}
		})
	}
}

func TestFormatTasksTable_WithDependencies(t *testing.T) {
	tasks := []taskInfo{
		{
			ID:           "task-1",
			Description:  "No dependencies",
			Status:       "UNCLAIMED",
			Priority:     1,
			DependsOn:    nil,
			Age:          "1h ago",
			TimeInStatus: "1h ago",
		},
		{
			ID:           "task-2",
			Description:  "Single dependency",
			Status:       "UNCLAIMED",
			Priority:     2,
			DependsOn:    []string{"task-1"},
			Age:          "30m ago",
			TimeInStatus: "30m ago",
		},
		{
			ID:           "task-3",
			Description:  "Multiple dependencies",
			Status:       "UNCLAIMED",
			Priority:     3,
			DependsOn:    []string{"task-1", "task-2"},
			Age:          "15m ago",
			TimeInStatus: "15m ago",
		},
	}

	output := formatTasksTable(tasks)

	// Check header includes DEPS column
	if !strings.Contains(output, "DEPS") {
		t.Errorf("expected table header to contain 'DEPS'")
	}

	// Check dependency counts appear in output
	if !strings.Contains(output, "-") { // task with no deps should show "-"
		t.Errorf("expected output to contain '-' for no dependencies")
	}
	if !strings.Contains(output, "1") { // task with 1 dep should show "1"
		t.Errorf("expected output to contain '1' for single dependency")
	}
	if !strings.Contains(output, "2") { // task with 2 deps should show "2"
		t.Errorf("expected output to contain '2' for multiple dependencies")
	}
}

func TestFormatTaskValue_WithDependencies(t *testing.T) {
	tests := []struct {
		name             string
		task             taskInfo
		expectContains   []string
		notExpectContain []string
	}{
		{
			name: "task with no dependencies",
			task: taskInfo{
				ID:           "task-1",
				Description:  "Test task",
				Status:       "UNCLAIMED",
				Priority:     1,
				DependsOn:    nil,
				Age:          "1h ago",
				TimeInStatus: "1h ago",
			},
			expectContains: []string{
				"ID: task-1",
				"Dependencies: none",
			},
		},
		{
			name: "task with empty dependencies slice",
			task: taskInfo{
				ID:           "task-2",
				Description:  "Test task",
				Status:       "UNCLAIMED",
				Priority:     1,
				DependsOn:    []string{},
				Age:          "1h ago",
				TimeInStatus: "1h ago",
			},
			expectContains: []string{
				"ID: task-2",
				"Dependencies: none",
			},
		},
		{
			name: "task with single dependency",
			task: taskInfo{
				ID:           "task-3",
				Description:  "Test task",
				Status:       "UNCLAIMED",
				Priority:     1,
				DependsOn:    []string{"task-1"},
				Age:          "1h ago",
				TimeInStatus: "1h ago",
			},
			expectContains: []string{
				"ID: task-3",
				"Dependencies: task-1",
			},
			notExpectContain: []string{
				"Dependencies: none",
			},
		},
		{
			name: "task with multiple dependencies",
			task: taskInfo{
				ID:           "task-4",
				Description:  "Test task",
				Status:       "UNCLAIMED",
				Priority:     1,
				DependsOn:    []string{"task-1", "task-2", "task-3"},
				Age:          "1h ago",
				TimeInStatus: "1h ago",
			},
			expectContains: []string{
				"ID: task-4",
				"Dependencies: task-1, task-2, task-3",
			},
			notExpectContain: []string{
				"Dependencies: none",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := formatTaskValue(tt.task)

			for _, expected := range tt.expectContains {
				if !strings.Contains(output, expected) {
					t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", expected, output)
				}
			}

			for _, notExpected := range tt.notExpectContain {
				if strings.Contains(output, notExpected) {
					t.Errorf("expected output NOT to contain %q, but it did.\nOutput:\n%s", notExpected, output)
				}
			}
		})
	}
}
