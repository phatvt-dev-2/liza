package agent

import (
	"github.com/liza-mas/liza/internal/models"
)

// PlannerWakeTrigger represents what triggered the planner to wake
type PlannerWakeTrigger string

const (
	WakeTriggerInitialPlanning     PlannerWakeTrigger = "INITIAL_PLANNING"
	WakeTriggerBlocked             PlannerWakeTrigger = "BLOCKED_TASKS"
	WakeTriggerIntegrationFailed   PlannerWakeTrigger = "INTEGRATION_FAILED"
	WakeTriggerHypothesisExhausted PlannerWakeTrigger = "HYPOTHESIS_EXHAUSTED"
	WakeTriggerImmediateDiscovery  PlannerWakeTrigger = "IMMEDIATE_DISCOVERY"
	WakeTriggerSprintComplete      PlannerWakeTrigger = "SPRINT_COMPLETE"
	WakeTriggerNone                PlannerWakeTrigger = "NONE"
)

// PlannerWakeResult contains the wake trigger and count
type PlannerWakeResult struct {
	Trigger PlannerWakeTrigger
	Count   int
}

type plannerWakeTriggerSpec struct {
	Trigger     PlannerWakeTrigger
	Description string
	Count       func(state *models.State) int
}

var plannerWakeTriggerSpecs = []plannerWakeTriggerSpec{
	{
		Trigger:     WakeTriggerInitialPlanning,
		Description: "No tasks exist yet, so initial planning is required.",
		Count: func(state *models.State) int {
			if len(state.Tasks) == 0 {
				return 1
			}
			return 0
		},
	},
	{
		Trigger:     WakeTriggerBlocked,
		Description: "Blocked tasks need planner intervention.",
		Count: func(state *models.State) int {
			return countTasksByStatus(state, models.TaskStatusBlocked)
		},
	},
	{
		Trigger:     WakeTriggerIntegrationFailed,
		Description: "Integration failures need planner intervention.",
		Count: func(state *models.State) int {
			return countTasksByStatus(state, models.TaskStatusIntegrationFailed)
		},
	},
	{
		Trigger:     WakeTriggerHypothesisExhausted,
		Description: "Tasks with repeated coder failures need planner intervention.",
		Count:       countHypothesisExhaustedTasks,
	},
	{
		Trigger:     WakeTriggerImmediateDiscovery,
		Description: "Immediate discoveries need planner triage.",
		Count:       countImmediateDiscoveries,
	},
	{
		Trigger:     WakeTriggerSprintComplete,
		Description: "All planned tasks are terminal and the sprint can be closed out.",
		Count: func(state *models.State) int {
			if state.AllPlannedTasksTerminal() {
				return len(state.Sprint.Scope.Planned)
			}
			return 0
		},
	},
}

// DetectPlannerWakeTriggers detects conditions that should wake the planner
// Returns the highest-priority trigger and count of items for that trigger
// Priority order:
// 1. No tasks (initial planning)
// 2. Blocked tasks
// 3. Integration failed
// 4. Hypothesis exhausted (2+ failed_by)
// 5. Immediate discoveries (not yet converted to tasks)
// 6. Sprint complete (all planned tasks terminal)
func DetectPlannerWakeTriggers(state *models.State) PlannerWakeResult {
	for _, triggerSpec := range plannerWakeTriggerSpecs {
		if count := triggerSpec.Count(state); count > 0 {
			return PlannerWakeResult{
				Trigger: triggerSpec.Trigger,
				Count:   count,
			}
		}
	}

	return PlannerWakeResult{
		Trigger: WakeTriggerNone,
		Count:   0,
	}
}

func countTasksByStatus(state *models.State, status models.TaskStatus) int {
	count := 0
	for _, task := range state.Tasks {
		if task.Status == status {
			count++
		}
	}
	return count
}

func countHypothesisExhaustedTasks(state *models.State) int {
	count := 0
	for _, task := range state.Tasks {
		if len(task.FailedBy) >= 2 && !task.Status.IsTerminal() {
			count++
		}
	}
	return count
}

func countImmediateDiscoveries(state *models.State) int {
	count := 0
	for _, disc := range state.Discovered {
		if disc.Urgency == "immediate" && disc.ConvertedToTask == nil {
			count++
		}
	}
	return count
}
