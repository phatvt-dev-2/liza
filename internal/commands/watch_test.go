package commands

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/log"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestCheckExpiredLeases(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		state      *models.State
		wantAlerts int
		validate   func(*testing.T, []alert)
	}{
		{
			name: "expired coder lease",
			state: &models.State{
				Tasks: []models.Task{
					func() models.Task {
						task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
						return task
					}(),
				},
				Agents: map[string]models.Agent{
					"coder-1": {
						Status:      models.AgentStatusWorking,
						CurrentTask: testhelpers.StringPtr("task-1"),
						// Expired lease (past grace period)
						LeaseExpires: testhelpers.TimePtr(now.Add(-3 * time.Minute)),
					},
				},
			},
			wantAlerts: 1,
			validate: func(t *testing.T, alerts []alert) {
				if alerts[0].Category != "LEASE EXPIRED" {
					t.Errorf("Category = %q, want %q", alerts[0].Category, "LEASE EXPIRED")
				}
				if alerts[0].Level != alertLevelWarning {
					t.Errorf("Level = %q, want %q", alerts[0].Level, alertLevelWarning)
				}
			},
		},
		{
			name: "expired reviewer lease",
			state: &models.State{
				Tasks: []models.Task{
					func() models.Task {
						task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
						reviewer := "code-reviewer-1"
						task.ReviewingBy = &reviewer
						// Expired lease (past grace period)
						expiredTime := now.Add(-3 * time.Minute)
						task.ReviewLeaseExpires = &expiredTime
						return task
					}(),
				},
				Agents: map[string]models.Agent{},
			},
			wantAlerts: 1,
			validate: func(t *testing.T, alerts []alert) {
				if alerts[0].Category != "REVIEW LEASE EXPIRED" {
					t.Errorf("Category = %q, want %q", alerts[0].Category, "REVIEW LEASE EXPIRED")
				}
			},
		},
		{
			name: "lease within grace period",
			state: &models.State{
				Tasks: []models.Task{
					func() models.Task {
						task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
						return task
					}(),
				},
				Agents: map[string]models.Agent{
					"coder-1": {
						Status:      models.AgentStatusWorking,
						CurrentTask: testhelpers.StringPtr("task-1"),
						// Expired but within grace period
						LeaseExpires: testhelpers.TimePtr(now.Add(-1 * time.Minute)),
					},
				},
			},
			wantAlerts: 0,
		},
		{
			name: "valid lease",
			state: &models.State{
				Tasks: []models.Task{
					func() models.Task {
						task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
						return task
					}(),
				},
				Agents: map[string]models.Agent{
					"coder-1": {
						Status:       models.AgentStatusWorking,
						CurrentTask:  testhelpers.StringPtr("task-1"),
						LeaseExpires: testhelpers.TimePtr(now.Add(30 * time.Minute)),
					},
				},
			},
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alerts := checkExpiredLeases(tt.state)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}

			if tt.validate != nil && len(alerts) > 0 {
				tt.validate(t, alerts)
			}
		})
	}
}

func TestCheckBlockedTasks(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		tasks      []models.Task
		cache      map[string]time.Time
		wantAlerts int
		wantCached bool
	}{
		{
			name: "blocked task - first time",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
					return task
				}(),
			},
			cache:      make(map[string]time.Time),
			wantAlerts: 1,
			wantCached: true,
		},
		{
			name: "blocked task - already seen",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
					return task
				}(),
			},
			cache: map[string]time.Time{
				"blocked:task-1": now.Add(-5 * time.Minute),
			},
			wantAlerts: 0,
			wantCached: true,
		},
		{
			name: "no blocked tasks",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
			},
			cache:      make(map[string]time.Time),
			wantAlerts: 0,
			wantCached: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{Tasks: tt.tasks}
			alerts := checkBlockedTasks(state, tt.cache)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}

			if tt.wantCached {
				if _, cached := tt.cache["blocked:task-1"]; !cached {
					t.Error("Expected task to be cached but it wasn't")
				}
			}
		})
	}
}

func TestCheckOrphanedRejected(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		tasks      []models.Task
		agents     map[string]models.Agent
		cache      map[string]time.Time
		wantAlerts int
	}{
		{
			name: "orphaned rejected - agent missing",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
					return task
				}(),
			},
			agents: map[string]models.Agent{},
			cache: map[string]time.Time{
				// Already past grace period
				"orphaned:task-1": now.Add(-1 * time.Minute),
			},
			wantAlerts: 1,
		},
		{
			name: "orphaned rejected - agent idle",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
					return task
				}(),
			},
			agents: map[string]models.Agent{
				"coder-1": {
					Status: models.AgentStatusIdle,
				},
			},
			cache: map[string]time.Time{
				"orphaned:task-1": now.Add(-1 * time.Minute),
			},
			wantAlerts: 1,
		},
		{
			name: "not orphaned - agent working",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
					return task
				}(),
			},
			agents: map[string]models.Agent{
				"coder-1": {
					Status: models.AgentStatusWorking,
				},
			},
			cache:      make(map[string]time.Time),
			wantAlerts: 0,
		},
		{
			name: "within grace period",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
					return task
				}(),
			},
			agents: map[string]models.Agent{},
			cache: map[string]time.Time{
				// Just added to cache (within grace period)
				"orphaned:task-1": now.Add(-5 * time.Second),
			},
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{
				Tasks:  tt.tasks,
				Agents: tt.agents,
			}
			alerts := checkOrphanedRejected(state, tt.cache)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
		})
	}
}

func TestCheckReviewLoops(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		tasks      []models.Task
		wantAlerts int
	}{
		{
			name: "at review cycle limit",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
					task.ReviewCyclesCurrent = 5
					return task
				}(),
			},
			wantAlerts: 1,
		},
		{
			name: "above review cycle limit",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
					task.ReviewCyclesCurrent = 6
					return task
				}(),
			},
			wantAlerts: 1,
		},
		{
			name: "below limit",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
					task.ReviewCyclesCurrent = 3
					return task
				}(),
			},
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{Tasks: tt.tasks}
			alerts := checkReviewLoops(state)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
		})
	}
}

func TestCheckIntegrationFailures(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		tasks      []models.Task
		wantAlerts int
	}{
		{
			name: "integration failed task",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now),
			},
			wantAlerts: 1,
		},
		{
			name: "no integration failures",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
			},
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{Tasks: tt.tasks}
			alerts := checkIntegrationFailures(state)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
		})
	}
}

func TestCheckHypothesisExhaustion(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		tasks      []models.Task
		wantAlerts int
	}{
		{
			name: "two failed coders",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
					task.FailedBy = []string{"coder-1", "coder-2"}
					return task
				}(),
			},
			wantAlerts: 1,
		},
		{
			name: "one failed coder",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
					task.FailedBy = []string{"coder-1"}
					return task
				}(),
			},
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{Tasks: tt.tasks}
			alerts := checkHypothesisExhaustion(state)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
		})
	}
}

func TestCheckReassigned(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		tasks      []models.Task
		cache      map[string]time.Time
		wantAlerts int
	}{
		{
			name: "task reassigned to different coder",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
					assignee := "coder-2"
					task.AssignedTo = &assignee
					// Add history showing first claimer was different
					firstClaimer := "coder-1"
					task.History = []models.TaskHistoryEntry{
						{
							Time:  now.Add(-1 * time.Hour),
							Event: models.TaskEventClaimed,
							Agent: &firstClaimer,
						},
					}
					return task
				}(),
			},
			cache:      make(map[string]time.Time),
			wantAlerts: 1,
		},
		{
			name: "same coder - no reassignment",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
					assignee := "coder-1"
					task.AssignedTo = &assignee
					task.History = []models.TaskHistoryEntry{
						{
							Time:  now.Add(-1 * time.Hour),
							Event: models.TaskEventClaimed,
							Agent: &assignee,
						},
					}
					return task
				}(),
			},
			cache:      make(map[string]time.Time),
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{Tasks: tt.tasks}
			alerts := checkReassigned(state, tt.cache)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
		})
	}
}

func TestCheckApproachingLimits(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		tasks      []models.Task
		wantAlerts int
	}{
		{
			name: "approaching iteration limit",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
					task.Iteration = 8
					return task
				}(),
			},
			wantAlerts: 1,
		},
		{
			name: "at iteration cliff",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
					task.Iteration = 10
					return task
				}(),
			},
			wantAlerts: 0, // Don't warn at cliff, only approaching
		},
		{
			name: "approaching review cycle limit",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)
					task.ReviewCyclesCurrent = 3
					return task
				}(),
			},
			wantAlerts: 1,
		},
		{
			name: "one failure + high review cycles",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
					task.FailedBy = []string{"coder-1"}
					task.ReviewCyclesCurrent = 3
					return task
				}(),
			},
			wantAlerts: 2, // Both review cycle and failure+cycle warnings fire
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{Tasks: tt.tasks}
			alerts := checkApproachingLimits(state)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
		})
	}
}

func TestCheckStalled(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		setupLog   func(string) error
		wantAlerts int
	}{
		{
			name: "stalled - no activity for 31 minutes",
			setupLog: func(logPath string) error {
				logger := log.New(logPath)
				return logger.Append(log.Entry{
					Timestamp: now.Add(-31 * time.Minute),
					Agent:     "test-agent",
					Action:    "test_action",
					Detail:    "test",
				})
			},
			wantAlerts: 1,
		},
		{
			name: "not stalled - recent activity",
			setupLog: func(logPath string) error {
				logger := log.New(logPath)
				return logger.Append(log.Entry{
					Timestamp: now.Add(-5 * time.Minute),
					Agent:     "test-agent",
					Action:    "test_action",
					Detail:    "test",
				})
			},
			wantAlerts: 0,
		},
		{
			name: "no log file",
			setupLog: func(logPath string) error {
				return nil // Don't create log
			},
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "log.yaml")

			if err := tt.setupLog(logPath); err != nil {
				t.Fatalf("Failed to setup log: %v", err)
			}

			cache := make(map[string]time.Time)
			alerts := checkStalled(logPath, cache)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
		})
	}
}

func TestCheckStalledThrottling(t *testing.T) {
	now := time.Now().UTC()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log.yaml")

	// Setup log with stale timestamp (31 minutes old)
	logger := log.New(logPath)
	if err := logger.Append(log.Entry{
		Timestamp: now.Add(-31 * time.Minute),
		Agent:     "test-agent",
		Action:    "test_action",
		Detail:    "test",
	}); err != nil {
		t.Fatalf("Failed to setup log: %v", err)
	}

	cache := make(map[string]time.Time)

	// First call - should generate alert
	alerts := checkStalled(logPath, cache)
	if len(alerts) != 1 {
		t.Errorf("First call: len(alerts) = %d, want 1", len(alerts))
	}

	// Second call immediately after - should be throttled
	alerts = checkStalled(logPath, cache)
	if len(alerts) != 0 {
		t.Errorf("Second call (throttled): len(alerts) = %d, want 0", len(alerts))
	}

	// Simulate 5 minutes passing by updating cache to 5+ minutes ago
	cache["stalled:alert"] = now.Add(-6 * time.Minute)

	// Third call after 5 minutes - should generate alert again
	alerts = checkStalled(logPath, cache)
	if len(alerts) != 1 {
		t.Errorf("Third call (after 5 min): len(alerts) = %d, want 1", len(alerts))
	}
}

func TestCheckStaleDrafts(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		tasks      []models.Task
		wantAlerts int
	}{
		{
			name: "stale draft - 31 minutes old",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusDraft, now)
					task.Created = now.Add(-31 * time.Minute)
					return task
				}(),
			},
			wantAlerts: 1,
		},
		{
			name: "fresh draft",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusDraft, now)
					task.Created = now.Add(-5 * time.Minute)
					return task
				}(),
			},
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{Tasks: tt.tasks}
			alerts := checkStaleDrafts(state)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
		})
	}
}

func TestCheckImmediateDiscoveries(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		discovered []models.Discovery
		wantAlerts int
	}{
		{
			name: "immediate discovery not converted",
			discovered: []models.Discovery{
				{
					ID:          "disc-1",
					Description: "Critical bug found",
					Urgency:     "immediate",
					Created:     now,
				},
			},
			wantAlerts: 1,
		},
		{
			name: "immediate discovery already converted",
			discovered: []models.Discovery{
				{
					ID:              "disc-1",
					Description:     "Critical bug found",
					Urgency:         "immediate",
					Created:         now,
					ConvertedToTask: testhelpers.StringPtr("task-1"),
				},
			},
			wantAlerts: 0,
		},
		{
			name: "deferred urgency discovery",
			discovered: []models.Discovery{
				{
					ID:          "disc-1",
					Description: "Minor improvement",
					Urgency:     "deferred",
					Created:     now,
				},
			},
			wantAlerts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{Discovered: tt.discovered}
			alerts := checkImmediateDiscoveries(state)

			if len(alerts) != tt.wantAlerts {
				t.Errorf("len(alerts) = %d, want %d", len(alerts), tt.wantAlerts)
			}
		})
	}
}

func TestWatchCommand(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name      string
		setupFunc func(*testing.T, string) *models.State
		runTime   time.Duration
		wantErr   bool
	}{
		{
			name: "basic watch - single check cycle",
			setupFunc: func(t *testing.T, tmpDir string) *models.State {
				state := testhelpers.CreateValidState()
				// Add a task that will trigger an alert
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now),
				}
				return state
			},
			runTime: 100 * time.Millisecond,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
			testhelpers.SetupPipelineConfig(t, tmpDir)
			alertsLog := paths.New(tmpDir).AlertsLogPath()

			state := tt.setupFunc(t, tmpDir)
			testhelpers.WriteInitialState(t, stateFile, state)

			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), tt.runTime)
			defer cancel()

			config := WatchConfig{
				ProjectRoot:   tmpDir,
				CheckInterval: 50 * time.Millisecond,
				AlertsLog:     alertsLog,
				StateCache:    make(map[string]time.Time),
			}

			err := WatchCommand(ctx, config)

			// Should get context.DeadlineExceeded or context.Canceled
			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantErr && err != nil && err != context.DeadlineExceeded && err != context.Canceled {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check that alerts were written
			if _, err := os.Stat(alertsLog); err == nil {
				data, _ := os.ReadFile(alertsLog)
				if len(data) == 0 {
					t.Error("Expected alerts to be written but file is empty")
				}
			}
		})
	}
}

func TestCheckSprintStalled(t *testing.T) {
	now := time.Now().UTC()

	t.Run("sprint stalled - all blocked - triggers checkpoint", func(t *testing.T) {
		tmpDir := t.TempDir()
		stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
		testhelpers.SetupPipelineConfig(t, tmpDir)
		lizaPaths := paths.New(tmpDir)

		state := testhelpers.CreateValidState()
		state.Sprint.Scope.Planned = []string{"task-1", "task-2"}
		state.Tasks = []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
			testhelpers.BuildTaskByStatus("task-2", models.TaskStatusBlocked, now),
		}
		testhelpers.WriteInitialState(t, stateFile, state)

		cache := make(map[string]time.Time)
		alerts, err := checkSprintStalled(tmpDir, state, cache)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(alerts) != 2 {
			t.Fatalf("len(alerts) = %d, want 2", len(alerts))
		}
		if alerts[0].Category != "SPRINT STALLED" {
			t.Errorf("alert[0].Category = %q, want %q", alerts[0].Category, "SPRINT STALLED")
		}
		if alerts[0].Level != alertLevelCritical {
			t.Errorf("alert[0].Level = %q, want %q", alerts[0].Level, alertLevelCritical)
		}
		if !strings.Contains(alerts[0].Message, "2 non-terminal planned tasks are BLOCKED") {
			t.Errorf("alert[0].Message = %q, expected blocked count", alerts[0].Message)
		}
		if alerts[1].Category != "AUTO CHECKPOINT" {
			t.Errorf("alert[1].Category = %q, want %q", alerts[1].Category, "AUTO CHECKPOINT")
		}

		// Verify sprint was checkpointed
		bb := db.New(stateFile)
		updatedState, err := bb.Read()
		if err != nil {
			t.Fatalf("failed to read state: %v", err)
		}
		if updatedState.Sprint.Status != models.SprintStatusCheckpoint {
			t.Errorf("sprint.status = %s, want %s", updatedState.Sprint.Status, models.SprintStatusCheckpoint)
		}

		// Verify report was written
		if _, err := os.Stat(lizaPaths.SprintSummaryPath()); err != nil {
			t.Fatalf("expected sprint summary report: %v", err)
		}
	})

	t.Run("mix of terminal and blocked - triggers checkpoint", func(t *testing.T) {
		tmpDir := t.TempDir()
		stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
		testhelpers.SetupPipelineConfig(t, tmpDir)

		state := testhelpers.CreateValidState()
		state.Sprint.Scope.Planned = []string{"task-1", "task-2"}
		state.Tasks = []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
			testhelpers.BuildTaskByStatus("task-2", models.TaskStatusBlocked, now),
		}
		testhelpers.WriteInitialState(t, stateFile, state)

		cache := make(map[string]time.Time)
		alerts, err := checkSprintStalled(tmpDir, state, cache)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(alerts) != 2 {
			t.Fatalf("len(alerts) = %d, want 2", len(alerts))
		}
		if !strings.Contains(alerts[0].Message, "1 non-terminal planned tasks are BLOCKED") {
			t.Errorf("alert[0].Message = %q, expected 1 blocked", alerts[0].Message)
		}
	})

	t.Run("tasks in progress - no alert", func(t *testing.T) {
		state := testhelpers.CreateValidState()
		state.Sprint.Scope.Planned = []string{"task-1", "task-2"}
		state.Tasks = []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
			testhelpers.BuildTaskByStatus("task-2", models.TaskStatusImplementing, now),
		}

		cache := make(map[string]time.Time)
		alerts, err := checkSprintStalled("/nonexistent", state, cache)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(alerts) != 0 {
			t.Errorf("len(alerts) = %d, want 0", len(alerts))
		}
	})

	t.Run("sprint already at CHECKPOINT - no action", func(t *testing.T) {
		state := testhelpers.CreateValidState()
		state.Sprint.Status = models.SprintStatusCheckpoint
		state.Sprint.Scope.Planned = []string{"task-1"}
		state.Tasks = []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
		}

		cache := make(map[string]time.Time)
		alerts, err := checkSprintStalled("/nonexistent", state, cache)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(alerts) != 0 {
			t.Errorf("len(alerts) = %d, want 0 (guarded by sprint status)", len(alerts))
		}
	})

	t.Run("stall then resume still stalled re-triggers checkpoint", func(t *testing.T) {
		// Regression: cache must be cleared when sprint leaves IN_PROGRESS,
		// so a resumed-but-still-stalled sprint gets a fresh checkpoint.
		tmpDir := t.TempDir()
		stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
		testhelpers.SetupPipelineConfig(t, tmpDir)

		state := testhelpers.CreateValidState()
		state.Sprint.Scope.Planned = []string{"task-1"}
		state.Tasks = []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
		}
		testhelpers.WriteInitialState(t, stateFile, state)

		cache := make(map[string]time.Time)

		// Step 1: Stall detected → checkpoint
		alerts, err := checkSprintStalled(tmpDir, state, cache)
		if err != nil {
			t.Fatalf("step 1: unexpected error: %v", err)
		}
		if len(alerts) != 2 {
			t.Fatalf("step 1: len(alerts) = %d, want 2", len(alerts))
		}

		// Step 2: Sprint is now at CHECKPOINT — simulate watch tick
		bb := db.New(stateFile)
		checkpointState, err := bb.Read()
		if err != nil {
			t.Fatalf("failed to read checkpointed state: %v", err)
		}
		if checkpointState.Sprint.Status != models.SprintStatusCheckpoint {
			t.Fatalf("sprint.status = %s, want CHECKPOINT", checkpointState.Sprint.Status)
		}
		alerts, err = checkSprintStalled(tmpDir, checkpointState, cache)
		if err != nil {
			t.Fatalf("step 2: unexpected error: %v", err)
		}
		if len(alerts) != 0 {
			t.Errorf("step 2: len(alerts) = %d, want 0", len(alerts))
		}
		// Cache should have been cleared by the non-IN_PROGRESS guard
		if _, cached := cache["sprint_stalled:alert"]; cached {
			t.Error("step 2: cache should be cleared when sprint not IN_PROGRESS")
		}

		// Step 3: Human resumes without fixing → sprint back to IN_PROGRESS, still stalled
		err = bb.Modify(func(s *models.State) error {
			s.Sprint.Status = models.SprintStatusInProgress
			s.Sprint.Timeline.CheckpointAt = nil
			return nil
		})
		if err != nil {
			t.Fatalf("failed to resume sprint: %v", err)
		}
		resumedState, err := bb.Read()
		if err != nil {
			t.Fatalf("failed to read resumed state: %v", err)
		}

		// Step 4: Stall re-detected → fresh checkpoint
		alerts, err = checkSprintStalled(tmpDir, resumedState, cache)
		if err != nil {
			t.Fatalf("step 4: unexpected error: %v", err)
		}
		if len(alerts) != 2 {
			t.Fatalf("step 4: len(alerts) = %d, want 2 (should re-trigger after resume)", len(alerts))
		}
		if alerts[0].Category != "SPRINT STALLED" {
			t.Errorf("step 4: alert[0].Category = %q, want SPRINT STALLED", alerts[0].Category)
		}
	})

	t.Run("throttling - second call does not re-trigger", func(t *testing.T) {
		tmpDir := t.TempDir()
		stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
		testhelpers.SetupPipelineConfig(t, tmpDir)

		state := testhelpers.CreateValidState()
		state.Sprint.Scope.Planned = []string{"task-1"}
		state.Tasks = []models.Task{
			testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
		}
		testhelpers.WriteInitialState(t, stateFile, state)

		cache := make(map[string]time.Time)

		// First call triggers
		alerts, err := checkSprintStalled(tmpDir, state, cache)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(alerts) != 2 {
			t.Fatalf("first call: len(alerts) = %d, want 2", len(alerts))
		}

		// Re-read state since checkpoint modified it
		bb := db.New(stateFile)
		updatedState, err := bb.Read()
		if err != nil {
			t.Fatalf("failed to read state: %v", err)
		}

		// Second call should be guarded (sprint is now at CHECKPOINT)
		alerts, err = checkSprintStalled(tmpDir, updatedState, cache)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(alerts) != 0 {
			t.Errorf("second call: len(alerts) = %d, want 0 (sprint now at CHECKPOINT)", len(alerts))
		}
	})
}

func TestRunChecks_AutoSprintCheckpointOnCircuitBreakerPattern(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	lizaPaths := paths.New(tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Anomalies = []models.Anomaly{
		{
			Timestamp: now,
			Task:      "task-1",
			Reporter:  "coder-1",
			Type:      "retry_loop",
			Details: map[string]any{
				"count":         3,
				"error_pattern": "connection refused",
			},
		},
		{
			Timestamp: now,
			Task:      "task-2",
			Reporter:  "coder-2",
			Type:      "retry_loop",
			Details: map[string]any{
				"count":         3,
				"error_pattern": "connection refused",
			},
		},
		{
			Timestamp: now,
			Task:      "task-3",
			Reporter:  "code-reviewer-1",
			Type:      "retry_loop",
			Details: map[string]any{
				"count":         3,
				"error_pattern": "connection refused",
			},
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   lizaPaths.AlertsLogPath(),
		StateCache:  make(map[string]time.Time),
	}

	if err := runChecks(context.Background(), config); err != nil {
		t.Fatalf("runChecks() error: %v", err)
	}

	bb := db.New(stateFile)
	updatedState, err := bb.Read()
	if err != nil {
		t.Fatalf("failed to read updated state: %v", err)
	}

	if updatedState.Config.Mode != models.SystemModeCircuitBreakerTripped {
		t.Errorf("mode = %s, want %s", updatedState.Config.Mode, models.SystemModeCircuitBreakerTripped)
	}
	if updatedState.Sprint.Status != models.SprintStatusCheckpoint {
		t.Errorf("sprint.status = %s, want %s", updatedState.Sprint.Status, models.SprintStatusCheckpoint)
	}
	if updatedState.Sprint.Timeline.CheckpointAt == nil {
		t.Fatal("expected sprint checkpoint_at to be set")
	}
	if updatedState.CircuitBreaker.Status != "TRIGGERED" {
		t.Errorf("circuit_breaker.status = %s, want TRIGGERED", updatedState.CircuitBreaker.Status)
	}
	if updatedState.CircuitBreaker.CurrentTrigger == nil {
		t.Fatal("expected current_trigger to be populated")
	}
	if updatedState.CircuitBreaker.CurrentTrigger.Pattern != "retry_cluster" {
		t.Errorf("trigger pattern = %s, want retry_cluster", updatedState.CircuitBreaker.CurrentTrigger.Pattern)
	}

	if _, err := os.Stat(lizaPaths.CircuitBreakerReportPath()); err != nil {
		t.Fatalf("expected circuit breaker report: %v", err)
	}
	reportData, err := os.ReadFile(lizaPaths.CircuitBreakerReportPath())
	if err != nil {
		t.Fatalf("failed to read circuit breaker report: %v", err)
	}
	reportText := string(reportData)
	if !strings.Contains(reportText, "retry_cluster") {
		t.Errorf("expected retry_cluster in report, got:\n%s", reportText)
	}
	if !strings.Contains(reportText, "3 retry_loop anomalies with similar error patterns") {
		t.Errorf("expected retry evidence in report, got:\n%s", reportText)
	}
	if _, err := os.Stat(lizaPaths.SprintSummaryPath()); err != nil {
		t.Fatalf("expected sprint summary report: %v", err)
	}

	alertLogData, err := os.ReadFile(lizaPaths.AlertsLogPath())
	if err != nil {
		t.Fatalf("failed to read alerts log: %v", err)
	}
	alertLogText := string(alertLogData)
	if !strings.Contains(alertLogText, "CIRCUIT BREAKER") {
		t.Errorf("expected CIRCUIT BREAKER alert in log, got:\n%s", alertLogText)
	}
	if !strings.Contains(alertLogText, "AUTO CHECKPOINT") {
		t.Errorf("expected AUTO CHECKPOINT alert in log, got:\n%s", alertLogText)
	}
}

func TestRunChecks_NoCircuitBreakerEscalationBelowThreshold(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	lizaPaths := paths.New(tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Anomalies = []models.Anomaly{
		{
			Timestamp: now,
			Task:      "task-1",
			Reporter:  "coder-1",
			Type:      "retry_loop",
			Details: map[string]any{
				"count":         2,
				"error_pattern": "timeout",
			},
		},
		{
			Timestamp: now,
			Task:      "task-2",
			Reporter:  "coder-2",
			Type:      "retry_loop",
			Details: map[string]any{
				"count":         2,
				"error_pattern": "timeout",
			},
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   lizaPaths.AlertsLogPath(),
		StateCache:  make(map[string]time.Time),
	}

	if err := runChecks(context.Background(), config); err != nil {
		t.Fatalf("runChecks() error: %v", err)
	}

	bb := db.New(stateFile)
	updatedState, err := bb.Read()
	if err != nil {
		t.Fatalf("failed to read updated state: %v", err)
	}

	if updatedState.Config.Mode != models.SystemModeRunning {
		t.Errorf("mode = %s, want %s", updatedState.Config.Mode, models.SystemModeRunning)
	}
	if updatedState.Sprint.Status != models.SprintStatusInProgress {
		t.Errorf("sprint.status = %s, want %s", updatedState.Sprint.Status, models.SprintStatusInProgress)
	}
	if updatedState.Sprint.Timeline.CheckpointAt != nil {
		t.Fatalf("did not expect checkpoint_at, got %s", updatedState.Sprint.Timeline.CheckpointAt.UTC().Format(time.RFC3339))
	}
	if updatedState.CircuitBreaker.Status != "OK" {
		t.Errorf("circuit_breaker.status = %s, want OK", updatedState.CircuitBreaker.Status)
	}

	if _, err := os.Stat(lizaPaths.CircuitBreakerReportPath()); !os.IsNotExist(err) {
		t.Errorf("expected no circuit breaker report, err=%v", err)
	}
	if _, err := os.Stat(lizaPaths.SprintSummaryPath()); !os.IsNotExist(err) {
		t.Errorf("expected no sprint summary report, err=%v", err)
	}
}

func TestRunChecks_CircuitBreakerErrorDoesNotDropOtherAlerts(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	lizaPaths := paths.New(tmpDir)

	// Force ops.Analyze report write failure: report path exists as a directory.
	if err := os.Mkdir(lizaPaths.CircuitBreakerReportPath(), 0755); err != nil {
		t.Fatalf("failed to create report-path directory: %v", err)
	}

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-99", models.TaskStatusIntegrationFailed, now),
	}
	state.Anomalies = []models.Anomaly{
		{
			Timestamp: now,
			Task:      "task-1",
			Reporter:  "coder-1",
			Type:      "retry_loop",
			Details: map[string]any{
				"count":         3,
				"error_pattern": "connection refused",
			},
		},
		{
			Timestamp: now,
			Task:      "task-2",
			Reporter:  "coder-2",
			Type:      "retry_loop",
			Details: map[string]any{
				"count":         3,
				"error_pattern": "connection refused",
			},
		},
		{
			Timestamp: now,
			Task:      "task-3",
			Reporter:  "code-reviewer-1",
			Type:      "retry_loop",
			Details: map[string]any{
				"count":         3,
				"error_pattern": "connection refused",
			},
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	config := WatchConfig{
		ProjectRoot: tmpDir,
		AlertsLog:   lizaPaths.AlertsLogPath(),
		StateCache:  make(map[string]time.Time),
	}

	if err := runChecks(context.Background(), config); err != nil {
		t.Fatalf("runChecks() should not fail on circuit-breaker escalation errors: %v", err)
	}

	// Existing operational alerts should still be emitted.
	alertLogData, err := os.ReadFile(lizaPaths.AlertsLogPath())
	if err != nil {
		t.Fatalf("failed to read alerts log: %v", err)
	}
	alertLogText := string(alertLogData)
	if !strings.Contains(alertLogText, "INTEGRATION FAILED") {
		t.Errorf("expected INTEGRATION FAILED alert in log, got:\n%s", alertLogText)
	}
	if !strings.Contains(alertLogText, "CIRCUIT BREAKER ERROR") {
		t.Errorf("expected CIRCUIT BREAKER ERROR alert in log, got:\n%s", alertLogText)
	}

	// On escalation failure, state should remain unmutated by analyze/checkpoint.
	bb := db.New(stateFile)
	updatedState, err := bb.Read()
	if err != nil {
		t.Fatalf("failed to read updated state: %v", err)
	}
	if updatedState.Config.Mode != models.SystemModeRunning {
		t.Errorf("mode = %s, want %s", updatedState.Config.Mode, models.SystemModeRunning)
	}
	if updatedState.Sprint.Status != models.SprintStatusInProgress {
		t.Errorf("sprint.status = %s, want %s", updatedState.Sprint.Status, models.SprintStatusInProgress)
	}
}
