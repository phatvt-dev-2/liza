package agent

import (
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestCountClaimableTasks(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name  string
		tasks []models.Task
		want  int
	}{
		{
			name: "single unclaimed task with no dependencies",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
			},
			want: 1,
		},
		{
			name: "unclaimed task with satisfied dependencies",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReady, now)
					task.DependsOn = []string{"task-1"}
					return task
				}(),
			},
			want: 1,
		},
		{
			name: "unclaimed task with unsatisfied dependencies",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReady, now)
					task.DependsOn = []string{"task-1"}
					return task
				}(),
			},
			want: 0,
		},
		{
			name: "rejected task is claimable",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now),
			},
			want: 1,
		},
		{
			name: "integration_failed task is claimable",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now),
			},
			want: 1,
		},
		{
			name: "claimed task is not claimable",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
			},
			want: 0,
		},
		{
			name: "ready_for_review task is not claimable",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now),
			},
			want: 0,
		},
		{
			name: "mixed tasks with complex dependencies",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
				testhelpers.BuildTaskByStatus("task-2", models.TaskStatusMerged, now),
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-3", models.TaskStatusReady, now)
					task.DependsOn = []string{"task-1"}
					return task
				}(),
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-4", models.TaskStatusReady, now)
					task.DependsOn = []string{"task-1", "task-2"}
					return task
				}(),
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-5", models.TaskStatusReady, now)
					task.DependsOn = []string{"task-3"} // task-3 not merged yet
					return task
				}(),
			},
			want: 2, // task-3 and task-4 are claimable, task-5 is not
		},
		{
			name:  "empty task list",
			tasks: []models.Task{},
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := testhelpers.CreateValidState()
			state.Tasks = tt.tasks
			got := models.CountClaimableTasks(state, models.RoleCoder, nil)
			if got != tt.want {
				t.Errorf("CountClaimableTasks() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountReviewableTasks(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name  string
		tasks []models.Task
		want  int
	}{
		{
			name: "ready_for_review with no reviewer",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now),
			},
			want: 1,
		},
		{
			name: "ready_for_review with expired lease",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
					reviewer := "code-reviewer-1"
					task.ReviewingBy = &reviewer
					expiredTime := now.Add(-1 * time.Hour)
					task.ReviewLeaseExpires = &expiredTime
					return task
				}(),
			},
			want: 1,
		},
		{
			name: "reviewing with valid lease",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
					futureTime := now.Add(30 * time.Minute)
					task.ReviewLeaseExpires = &futureTime
					return task
				}(),
			},
			want: 0,
		},
		{
			name: "reviewing with reviewer but no lease (malformed)",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
					task.ReviewLeaseExpires = nil
					return task
				}(),
			},
			want: 0, // Not reviewable - malformed state (no lease to check expiry)
		},
		{
			name: "reviewing with expired lease not counted (needs stale claim clearing first)",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
					expiredTime := now.Add(-1 * time.Hour)
					task.ReviewLeaseExpires = &expiredTime
					return task
				}(),
			},
			want: 0,
		},
		{
			name: "mixed reviewable and non-reviewable",
			tasks: []models.Task{
				// Reviewable (READY_FOR_REVIEW, unassigned)
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now),
				// Not reviewable (REVIEWING with expired lease — needs stale claim clearing)
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReviewing, now)
					expiredTime := now.Add(-1 * time.Hour)
					task.ReviewLeaseExpires = &expiredTime
					return task
				}(),
				// Not reviewable (REVIEWING with valid lease)
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-3", models.TaskStatusReviewing, now)
					futureTime := now.Add(30 * time.Minute)
					task.ReviewLeaseExpires = &futureTime
					return task
				}(),
				// Not reviewable (IMPLEMENTING)
				testhelpers.BuildTaskByStatus("task-4", models.TaskStatusImplementing, now),
			},
			want: 1,
		},
		{
			name: "no ready_for_review tasks",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
				testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReady, now),
			},
			want: 0,
		},
		{
			name:  "empty task list",
			tasks: []models.Task{},
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := testhelpers.CreateValidState()
			state.Tasks = tt.tasks
			got := models.CountReviewableTasks(state, models.RoleCodeReviewer, nil)
			if got != tt.want {
				t.Errorf("CountReviewableTasks() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestOrchestratorWakeTriggerSpecs(t *testing.T) {
	// SprintComplete is handled separately in DetectOrchestratorWakeTriggers
	// (requires pipeline-aware terminal state checking), so it's not in the table.
	wantOrder := []OrchestratorWakeTrigger{
		WakeTriggerInitialPlanning,
		WakeTriggerBlocked,
		WakeTriggerIntegrationFailed,
		WakeTriggerHypothesisExhausted,
		WakeTriggerImmediateDiscovery,
	}

	if len(orchestratorWakeTriggerSpecs) != len(wantOrder) {
		t.Fatalf("orchestratorWakeTriggerSpecs has %d entries, want %d", len(orchestratorWakeTriggerSpecs), len(wantOrder))
	}

	for i, wantTrigger := range wantOrder {
		spec := orchestratorWakeTriggerSpecs[i]
		if spec.Trigger != wantTrigger {
			t.Errorf("orchestratorWakeTriggerSpecs[%d].Trigger = %q, want %q", i, spec.Trigger, wantTrigger)
		}
		if spec.Description == "" {
			t.Errorf("orchestratorWakeTriggerSpecs[%d].Description must not be empty", i)
		}
		if spec.Count == nil {
			t.Errorf("orchestratorWakeTriggerSpecs[%d].Count must not be nil", i)
		}
	}
}

func TestDetectOrchestratorWakeTriggers(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name        string
		state       *models.State
		wantTrigger OrchestratorWakeTrigger
		wantCount   int
	}{
		{
			name: "initial planning (no tasks)",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{}
				return state
			}(),
			wantTrigger: WakeTriggerInitialPlanning,
			wantCount:   1,
		},
		{
			name: "blocked tasks trigger",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
					testhelpers.BuildTaskByStatus("task-2", models.TaskStatusBlocked, now),
				}
				return state
			}(),
			wantTrigger: WakeTriggerBlocked,
			wantCount:   2,
		},
		{
			name: "integration failed trigger",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now),
				}
				return state
			}(),
			wantTrigger: WakeTriggerIntegrationFailed,
			wantCount:   1,
		},
		{
			name: "hypothesis exhaustion trigger",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
				task.FailedBy = []string{"coder-1", "coder-2"}
				state.Tasks = []models.Task{task}
				return state
			}(),
			wantTrigger: WakeTriggerHypothesisExhausted,
			wantCount:   1,
		},
		{
			name: "immediate discovery trigger",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
				}
				state.Discovered = []models.Discovery{
					{
						ID:             "disc-1",
						By:             "coder-1",
						During:         "task-1",
						Description:    "Critical bug",
						Severity:       "critical",
						Urgency:        "immediate",
						Recommendation: "Fix now",
						Created:        now,
					},
				}
				return state
			}(),
			wantTrigger: WakeTriggerImmediateDiscovery,
			wantCount:   1,
		},
		{
			name: "no triggers (tasks in progress)",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
					testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReadyForReview, now),
				}
				return state
			}(),
			wantTrigger: WakeTriggerNone,
			wantCount:   0,
		},
		{
			name: "multiple triggers (blocked takes priority)",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task1 := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
				task2 := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusIntegrationFailed, now)
				state.Tasks = []models.Task{task1, task2}
				return state
			}(),
			wantTrigger: WakeTriggerBlocked,
			wantCount:   1,
		},
		{
			name: "deferred discovery does not trigger",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
				}
				state.Discovered = []models.Discovery{
					{
						ID:             "disc-1",
						By:             "coder-1",
						During:         "task-1",
						Description:    "Minor issue",
						Severity:       "low",
						Urgency:        "deferred",
						Recommendation: "Fix later",
						Created:        now,
					},
				}
				return state
			}(),
			wantTrigger: WakeTriggerNone,
			wantCount:   0,
		},
		{
			name: "converted discovery does not trigger",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
				}
				taskID := "task-2"
				state.Discovered = []models.Discovery{
					{
						ID:              "disc-1",
						By:              "coder-1",
						During:          "task-1",
						Description:     "Critical bug",
						Severity:        "critical",
						Urgency:         "immediate",
						Recommendation:  "Fix now",
						Created:         now,
						ConvertedToTask: &taskID,
					},
				}
				return state
			}(),
			wantTrigger: WakeTriggerNone,
			wantCount:   0,
		},
		{
			name: "sprint complete (all planned tasks merged)",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Sprint.Scope.Planned = []string{"task-1", "task-2"}
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
					testhelpers.BuildTaskByStatus("task-2", models.TaskStatusMerged, now),
				}
				return state
			}(),
			wantTrigger: WakeTriggerSprintComplete,
			wantCount:   2,
		},
		{
			name: "sprint complete (mixed terminal states)",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Sprint.Scope.Planned = []string{"task-1", "task-2", "task-3"}
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
					testhelpers.BuildTaskByStatus("task-2", models.TaskStatusAbandoned, now),
					testhelpers.BuildTaskByStatus("task-3", models.TaskStatusSuperseded, now),
				}
				return state
			}(),
			wantTrigger: WakeTriggerSprintComplete,
			wantCount:   3,
		},
		{
			name: "sprint not complete (some tasks in progress)",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Sprint.Scope.Planned = []string{"task-1", "task-2"}
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
					testhelpers.BuildTaskByStatus("task-2", models.TaskStatusImplementing, now),
				}
				return state
			}(),
			wantTrigger: WakeTriggerNone,
			wantCount:   0,
		},
		{
			name: "sprint not complete (planned task not in task list)",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Sprint.Scope.Planned = []string{"task-1", "task-2"}
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
				}
				return state
			}(),
			wantTrigger: WakeTriggerNone,
			wantCount:   0,
		},
		{
			name: "empty planned list does not trigger sprint complete",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Sprint.Scope.Planned = []string{}
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
				}
				return state
			}(),
			wantTrigger: WakeTriggerNone,
			wantCount:   0,
		},
		{
			name: "planning complete (merged planning task with output[])",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
				task.RolePair = "code-planning-pair"
				task.Output = []models.OutputEntry{
					{Desc: "implement X", DoneWhen: "tests pass", Scope: "pkg/x"},
					{Desc: "implement Y", DoneWhen: "linter green", Scope: "pkg/y"},
				}
				state.Sprint.Scope.Planned = []string{"task-1"}
				state.Tasks = []models.Task{task}
				return state
			}(),
			wantTrigger: WakeTriggerPlanningComplete,
			wantCount:   1,
		},
		{
			name: "merged coding task with output[] triggers sprint complete not planning complete",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
				// No RolePair or non-planning role pair — coding task
				task.Output = []models.OutputEntry{
					{Desc: "something", DoneWhen: "done", Scope: "pkg/x"},
				}
				state.Sprint.Scope.Planned = []string{"task-1"}
				state.Tasks = []models.Task{task}
				return state
			}(),
			wantTrigger: WakeTriggerSprintComplete,
			wantCount:   1,
		},
		{
			name: "merged task without output[] triggers sprint complete not planning complete",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Sprint.Scope.Planned = []string{"task-1"}
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
				}
				return state
			}(),
			wantTrigger: WakeTriggerSprintComplete,
			wantCount:   1,
		},
		{
			name: "sprint CHECKPOINT suppresses re-wake",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Sprint.Status = models.SprintStatusCheckpoint
				state.Sprint.Scope.Planned = []string{"task-1", "task-2"}
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
					testhelpers.BuildTaskByStatus("task-2", models.TaskStatusMerged, now),
				}
				return state
			}(),
			wantTrigger: WakeTriggerNone,
			wantCount:   0,
		},
		{
			name: "sprint CHECKPOINT suppresses planning complete re-wake",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Sprint.Status = models.SprintStatusCheckpoint
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
				task.RolePair = "code-planning-pair"
				task.Output = []models.OutputEntry{
					{Desc: "implement X", DoneWhen: "tests pass", Scope: "pkg/x"},
				}
				state.Sprint.Scope.Planned = []string{"task-1"}
				state.Tasks = []models.Task{task}
				return state
			}(),
			wantTrigger: WakeTriggerNone,
			wantCount:   0,
		},
		{
			name: "sprint COMPLETED suppresses re-wake",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Sprint.Status = models.SprintStatusCompleted
				state.Sprint.Scope.Planned = []string{"task-1"}
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
				}
				return state
			}(),
			wantTrigger: WakeTriggerNone,
			wantCount:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectOrchestratorWakeTriggers(tt.state, nil, nil)
			if result.Trigger != tt.wantTrigger {
				t.Errorf("DetectOrchestratorWakeTriggers() trigger = %v, want %v", result.Trigger, tt.wantTrigger)
			}
			if result.Count != tt.wantCount {
				t.Errorf("DetectOrchestratorWakeTriggers() count = %d, want %d", result.Count, tt.wantCount)
			}
		})
	}
}

func TestDetectOrchestratorWakeTriggers_PipelineTerminals(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name              string
		state             *models.State
		pipelineTerminals []models.TaskStatus
		planningPairs     map[string]bool
		wantTrigger       OrchestratorWakeTrigger
		wantCount         int
	}{
		{
			name: "CODING_PLAN_APPROVED is sprint-terminal with pipeline terminals",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusCodingPlanApproved, now)
				task.RolePair = "code-planning-pair"
				state.Tasks = []models.Task{task}
				state.Sprint.Scope.Planned = []string{"task-1"}
				return state
			}(),
			pipelineTerminals: []models.TaskStatus{models.TaskStatusCodingPlanApproved, models.TaskStatusMerged},
			wantTrigger:       WakeTriggerSprintComplete,
			wantCount:         1,
		},
		{
			name: "CODING_PLAN_APPROVED is NOT sprint-terminal without pipeline terminals",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusCodingPlanApproved, now)
				task.RolePair = "code-planning-pair"
				state.Tasks = []models.Task{task}
				state.Sprint.Scope.Planned = []string{"task-1"}
				return state
			}(),
			pipelineTerminals: nil,
			wantTrigger:       WakeTriggerNone,
			wantCount:         0,
		},
		{
			name: "mixed legacy and pipeline tasks — all terminal",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				legacyTask := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
				pipelineTask := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusCodingPlanApproved, now)
				pipelineTask.RolePair = "code-planning-pair"
				state.Tasks = []models.Task{legacyTask, pipelineTask}
				state.Sprint.Scope.Planned = []string{"task-1", "task-2"}
				return state
			}(),
			pipelineTerminals: []models.TaskStatus{models.TaskStatusCodingPlanApproved, models.TaskStatusMerged},
			wantTrigger:       WakeTriggerSprintComplete,
			wantCount:         2,
		},
		{
			name: "mixed — pipeline task not yet terminal",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				legacyTask := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
				pipelineTask := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusCodePlanning, now)
				pipelineTask.RolePair = "code-planning-pair"
				state.Tasks = []models.Task{legacyTask, pipelineTask}
				state.Sprint.Scope.Planned = []string{"task-1", "task-2"}
				return state
			}(),
			pipelineTerminals: []models.TaskStatus{models.TaskStatusCodingPlanApproved, models.TaskStatusMerged},
			wantTrigger:       WakeTriggerNone,
			wantCount:         0,
		},
		{
			name: "epic-planning-pair with planningPairs triggers PLANNING_COMPLETE",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
				task.RolePair = "epic-planning-pair"
				task.Output = []models.OutputEntry{
					{Desc: "user story 1", DoneWhen: "accepted", Scope: "feature/x"},
				}
				state.Sprint.Scope.Planned = []string{"task-1"}
				state.Tasks = []models.Task{task}
				return state
			}(),
			planningPairs: map[string]bool{"epic-planning-pair": true, "code-planning-pair": true},
			wantTrigger:   WakeTriggerPlanningComplete,
			wantCount:     1,
		},
		{
			name: "epic-planning-pair without planningPairs falls back to SPRINT_COMPLETE",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
				task.RolePair = "epic-planning-pair"
				task.Output = []models.OutputEntry{
					{Desc: "user story 1", DoneWhen: "accepted", Scope: "feature/x"},
				}
				state.Sprint.Scope.Planned = []string{"task-1"}
				state.Tasks = []models.Task{task}
				return state
			}(),
			planningPairs: nil,
			wantTrigger:   WakeTriggerSprintComplete,
			wantCount:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectOrchestratorWakeTriggers(tt.state, tt.pipelineTerminals, tt.planningPairs)
			if result.Trigger != tt.wantTrigger {
				t.Errorf("DetectOrchestratorWakeTriggers() trigger = %v, want %v", result.Trigger, tt.wantTrigger)
			}
			if result.Count != tt.wantCount {
				t.Errorf("DetectOrchestratorWakeTriggers() count = %d, want %d", result.Count, tt.wantCount)
			}
		})
	}
}

func TestHasCoderWork(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name  string
		state *models.State
		want  bool
	}{
		{
			name: "claimable task available",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
				}
				return state
			}(),
			want: true,
		},
		{
			name: "no claimable tasks",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
				}
				return state
			}(),
			want: false,
		},
		{
			name: "empty tasks",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{}
				return state
			}(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := models.CountClaimableTasks(tt.state, models.RoleCoder, nil) > 0
			if got != tt.want {
				t.Errorf("CountClaimableTasks() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasReviewerWork(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name  string
		state *models.State
		want  bool
	}{
		{
			name: "reviewable task available",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now),
				}
				return state
			}(),
			want: true,
		},
		{
			name: "no reviewable tasks",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
				}
				return state
			}(),
			want: false,
		},
		{
			name: "empty tasks",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{}
				return state
			}(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := models.CountReviewableTasks(tt.state, models.RoleCodeReviewer, nil) > 0
			if got != tt.want {
				t.Errorf("CountReviewableTasks() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasOrchestratorWork(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name  string
		state *models.State
		want  bool
	}{
		{
			name: "initial planning needed",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{}
				return state
			}(),
			want: true,
		},
		{
			name: "blocked tasks trigger",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
				}
				return state
			}(),
			want: true,
		},
		{
			name: "no triggers",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
				}
				return state
			}(),
			want: false,
		},
		{
			name: "sprint complete triggers orchestrator work",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Sprint.Scope.Planned = []string{"task-1", "task-2"}
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
					testhelpers.BuildTaskByStatus("task-2", models.TaskStatusMerged, now),
				}
				return state
			}(),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectOrchestratorWakeTriggers(tt.state, nil, nil)
			got := result.Trigger != WakeTriggerNone
			if got != tt.want {
				t.Errorf("HasOrchestratorWork() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetCoderWorkDiagnostics(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name    string
		state   *models.State
		wantMsg string
	}{
		{
			name: "no tasks at all",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{}
				return state
			}(),
			wantMsg: "No claimable tasks",
		},
		{
			name: "tasks blocked by dependencies",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task1 := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
				task1.DependsOn = []string{"task-0"}
				task2 := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReady, now)
				task2.DependsOn = []string{"task-0"}
				state.Tasks = []models.Task{task1, task2}
				return state
			}(),
			wantMsg: "No claimable tasks; 2 blocked by dependencies",
		},
		{
			name: "tasks in progress",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
					testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReadyForReview, now),
				}
				return state
			}(),
			wantMsg: "No claimable tasks; 2 in progress",
		},
		{
			name: "claimable tasks available",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
					testhelpers.BuildTaskByStatus("task-2", models.TaskStatusRejected, now),
				}
				return state
			}(),
			wantMsg: "Found 2 claimable task(s)",
		},
		{
			name: "mixed: blocked and in progress",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task1 := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
				task1.DependsOn = []string{"task-0"}
				task2 := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusImplementing, now)
				state.Tasks = []models.Task{task1, task2}
				return state
			}(),
			wantMsg: "No claimable tasks; 1 blocked by dependencies; 1 in progress",
		},
		{
			name: "claimable with integration failed",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now),
				}
				return state
			}(),
			wantMsg: "Found 1 claimable task(s)",
		},
		{
			name: "mixed: some claimable, some blocked",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task1 := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
				task2 := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReady, now)
				task2.DependsOn = []string{"task-0"}
				state.Tasks = []models.Task{task1, task2}
				return state
			}(),
			wantMsg: "Found 1 claimable task(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := models.GetCoderWorkDiagnostics(tt.state, nil)
			if got != tt.wantMsg {
				t.Errorf("GetCoderWorkDiagnostics() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestGetReviewerWorkDiagnostics(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name    string
		state   *models.State
		wantMsg string
	}{
		{
			name: "no reviewable tasks",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
				}
				return state
			}(),
			wantMsg: "No reviewable tasks",
		},
		{
			name: "unassigned reviewable tasks",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now),
					testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReadyForReview, now),
				}
				return state
			}(),
			wantMsg: "Found 2 reviewable task(s)",
		},
		{
			name: "tasks with expired leases only",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task1 := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
				expiredTime := now.Add(-1 * time.Hour)
				task1.ReviewLeaseExpires = &expiredTime
				state.Tasks = []models.Task{task1}
				return state
			}(),
			wantMsg: "No reviewable tasks; 1 with stale leases (pending reclamation)",
		},
		{
			name: "mixed unassigned and expired leases",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task1 := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
				task2 := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReviewing, now)
				expiredTime := now.Add(-1 * time.Hour)
				task2.ReviewLeaseExpires = &expiredTime
				state.Tasks = []models.Task{task1, task2}
				return state
			}(),
			wantMsg: "Found 1 reviewable task(s); 1 with stale leases (pending reclamation)",
		},
		{
			name: "actively being reviewed",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task1 := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
				state.Tasks = []models.Task{task1}
				return state
			}(),
			wantMsg: "No reviewable tasks; 1 actively being reviewed",
		},
		{
			name: "multiple actively being reviewed",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task1 := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
				task2 := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReviewing, now)
				state.Tasks = []models.Task{task1, task2}
				return state
			}(),
			wantMsg: "No reviewable tasks; 2 actively being reviewed",
		},
		{
			name: "no ready_for_review tasks at all",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
					testhelpers.BuildTaskByStatus("task-2", models.TaskStatusImplementing, now),
					testhelpers.BuildTaskByStatus("task-3", models.TaskStatusMerged, now),
				}
				return state
			}(),
			wantMsg: "No reviewable tasks",
		},
		{
			name: "empty task list",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{}
				return state
			}(),
			wantMsg: "No reviewable tasks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := models.GetReviewerWorkDiagnostics(tt.state, nil)
			if got != tt.wantMsg {
				t.Errorf("GetReviewerWorkDiagnostics() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}
