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
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusUnclaimed, now),
			},
			want: 1,
		},
		{
			name: "unclaimed task with satisfied dependencies",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusUnclaimed, now)
					task.DependsOn = []string{"task-1"}
					return task
				}(),
			},
			want: 1,
		},
		{
			name: "unclaimed task with unsatisfied dependencies",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusClaimed, now),
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusUnclaimed, now)
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
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusClaimed, now),
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
					task := testhelpers.BuildTaskByStatus("task-3", models.TaskStatusUnclaimed, now)
					task.DependsOn = []string{"task-1"}
					return task
				}(),
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-4", models.TaskStatusUnclaimed, now)
					task.DependsOn = []string{"task-1", "task-2"}
					return task
				}(),
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-5", models.TaskStatusUnclaimed, now)
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
			got := models.CountClaimableTasks(state)
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
					reviewer := "reviewer-1"
					task.ReviewingBy = &reviewer
					expiredTime := now.Add(-1 * time.Hour)
					task.ReviewLeaseExpires = &expiredTime
					return task
				}(),
			},
			want: 1,
		},
		{
			name: "ready_for_review with valid lease",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
					reviewer := "reviewer-1"
					task.ReviewingBy = &reviewer
					futureTime := now.Add(30 * time.Minute)
					task.ReviewLeaseExpires = &futureTime
					return task
				}(),
			},
			want: 0,
		},
		{
			name: "ready_for_review with reviewer but no lease (malformed)",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
					reviewer := "reviewer-1"
					task.ReviewingBy = &reviewer
					task.ReviewLeaseExpires = nil
					return task
				}(),
			},
			want: 0, // Not reviewable - malformed state
		},
		{
			name: "mixed reviewable and non-reviewable",
			tasks: []models.Task{
				// Reviewable
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now),
				// Reviewable (expired)
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReadyForReview, now)
					reviewer := "reviewer-1"
					task.ReviewingBy = &reviewer
					expiredTime := now.Add(-1 * time.Hour)
					task.ReviewLeaseExpires = &expiredTime
					return task
				}(),
				// Not reviewable (valid lease)
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-3", models.TaskStatusReadyForReview, now)
					reviewer := "reviewer-2"
					task.ReviewingBy = &reviewer
					futureTime := now.Add(30 * time.Minute)
					task.ReviewLeaseExpires = &futureTime
					return task
				}(),
				// Not reviewable (claimed)
				testhelpers.BuildTaskByStatus("task-4", models.TaskStatusClaimed, now),
			},
			want: 2,
		},
		{
			name: "no ready_for_review tasks",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusClaimed, now),
				testhelpers.BuildTaskByStatus("task-2", models.TaskStatusUnclaimed, now),
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
			got := models.CountReviewableTasks(state)
			if got != tt.want {
				t.Errorf("CountReviewableTasks() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDetectPlannerWakeTriggers(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name        string
		state       *models.State
		wantTrigger PlannerWakeTrigger
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
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusUnclaimed, now)
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
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusUnclaimed, now),
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
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusClaimed, now),
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
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusUnclaimed, now),
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
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusUnclaimed, now),
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectPlannerWakeTriggers(tt.state)
			if result.Trigger != tt.wantTrigger {
				t.Errorf("DetectPlannerWakeTriggers() trigger = %v, want %v", result.Trigger, tt.wantTrigger)
			}
			if result.Count != tt.wantCount {
				t.Errorf("DetectPlannerWakeTriggers() count = %d, want %d", result.Count, tt.wantCount)
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
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusUnclaimed, now),
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
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusClaimed, now),
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
			got := models.CountClaimableTasks(tt.state) > 0
			if got != tt.want {
				t.Errorf("HasCoderWork() = %v, want %v", got, tt.want)
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
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusClaimed, now),
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
			got := models.CountReviewableTasks(tt.state) > 0
			if got != tt.want {
				t.Errorf("HasReviewerWork() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasPlannerWork(t *testing.T) {
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
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusClaimed, now),
				}
				return state
			}(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectPlannerWakeTriggers(tt.state)
			got := result.Trigger != WakeTriggerNone
			if got != tt.want {
				t.Errorf("HasPlannerWork() = %v, want %v", got, tt.want)
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
				task1 := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusUnclaimed, now)
				task1.DependsOn = []string{"task-0"}
				task2 := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusUnclaimed, now)
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
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusClaimed, now),
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
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusUnclaimed, now),
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
				task1 := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusUnclaimed, now)
				task1.DependsOn = []string{"task-0"}
				task2 := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusClaimed, now)
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
				task1 := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusUnclaimed, now)
				task2 := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusUnclaimed, now)
				task2.DependsOn = []string{"task-0"}
				state.Tasks = []models.Task{task1, task2}
				return state
			}(),
			wantMsg: "Found 1 claimable task(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := models.GetCoderWorkDiagnostics(tt.state)
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
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusClaimed, now),
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
			wantMsg: "Found 2 reviewable task(s): 2 unassigned",
		},
		{
			name: "tasks with expired leases",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task1 := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
				reviewer := "reviewer-1"
				task1.ReviewingBy = &reviewer
				expiredTime := now.Add(-1 * time.Hour)
				task1.ReviewLeaseExpires = &expiredTime
				state.Tasks = []models.Task{task1}
				return state
			}(),
			wantMsg: "Found 1 reviewable task(s): 1 with expired leases",
		},
		{
			name: "mixed unassigned and expired leases",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task1 := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
				task2 := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReadyForReview, now)
				reviewer := "reviewer-1"
				task2.ReviewingBy = &reviewer
				expiredTime := now.Add(-1 * time.Hour)
				task2.ReviewLeaseExpires = &expiredTime
				state.Tasks = []models.Task{task1, task2}
				return state
			}(),
			wantMsg: "Found 2 reviewable task(s): 1 unassigned, 1 with expired leases",
		},
		{
			name: "actively being reviewed",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task1 := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
				reviewer := "reviewer-1"
				task1.ReviewingBy = &reviewer
				futureTime := now.Add(30 * time.Minute)
				task1.ReviewLeaseExpires = &futureTime
				state.Tasks = []models.Task{task1}
				return state
			}(),
			wantMsg: "No reviewable tasks; 1 actively being reviewed",
		},
		{
			name: "multiple actively being reviewed",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task1 := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
				reviewer1 := "reviewer-1"
				task1.ReviewingBy = &reviewer1
				futureTime1 := now.Add(30 * time.Minute)
				task1.ReviewLeaseExpires = &futureTime1

				task2 := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReadyForReview, now)
				reviewer2 := "reviewer-2"
				task2.ReviewingBy = &reviewer2
				futureTime2 := now.Add(45 * time.Minute)
				task2.ReviewLeaseExpires = &futureTime2

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
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusUnclaimed, now),
					testhelpers.BuildTaskByStatus("task-2", models.TaskStatusClaimed, now),
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
			got := models.GetReviewerWorkDiagnostics(tt.state)
			if got != tt.wantMsg {
				t.Errorf("GetReviewerWorkDiagnostics() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}
