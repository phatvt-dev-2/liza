package agent

import (
	"github.com/liza-mas/liza/internal/models"
)

// OrchestratorWakeTrigger represents what triggered the orchestrator to wake
type OrchestratorWakeTrigger string

const (
	WakeTriggerInitialPlanning     OrchestratorWakeTrigger = "INITIAL_PLANNING"
	WakeTriggerBlocked             OrchestratorWakeTrigger = "BLOCKED_TASKS"
	WakeTriggerIntegrationFailed   OrchestratorWakeTrigger = "INTEGRATION_FAILED"
	WakeTriggerHypothesisExhausted OrchestratorWakeTrigger = "HYPOTHESIS_EXHAUSTED"
	WakeTriggerImmediateDiscovery  OrchestratorWakeTrigger = "IMMEDIATE_DISCOVERY"
	WakeTriggerPlanningComplete    OrchestratorWakeTrigger = "PLANNING_COMPLETE"
	WakeTriggerSprintComplete      OrchestratorWakeTrigger = "SPRINT_COMPLETE"
	WakeTriggerNone                OrchestratorWakeTrigger = "NONE"
)

// OrchestratorWakeResult contains the wake trigger and count
type OrchestratorWakeResult struct {
	Trigger OrchestratorWakeTrigger
	Count   int
}

type orchestratorWakeTriggerSpec struct {
	Trigger     OrchestratorWakeTrigger
	Description string
	Count       func(state *models.State) int
}

var orchestratorWakeTriggerSpecs = []orchestratorWakeTriggerSpec{
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
		Description: "Blocked tasks need orchestrator intervention.",
		Count: func(state *models.State) int {
			return countTasksByStatus(state, models.TaskStatusBlocked)
		},
	},
	{
		Trigger:     WakeTriggerIntegrationFailed,
		Description: "Integration failures need orchestrator intervention.",
		Count: func(state *models.State) int {
			return countTasksByStatus(state, models.TaskStatusIntegrationFailed)
		},
	},
	{
		Trigger:     WakeTriggerHypothesisExhausted,
		Description: "Tasks with repeated coder failures need orchestrator intervention.",
		Count:       countHypothesisExhaustedTasks,
	},
	{
		Trigger:     WakeTriggerImmediateDiscovery,
		Description: "Immediate discoveries need orchestrator triage.",
		Count:       countImmediateDiscoveries,
	},
	// WakeTriggerSprintComplete is handled separately in DetectOrchestratorWakeTriggers
	// because it requires pipeline-aware terminal state checking.
}

// DetectOrchestratorWakeTriggers detects conditions that should wake the orchestrator.
// pipelineTerminals provides pipeline-defined sprint-terminal states (from ops.SprintTerminalStates).
// Pass nil for legacy projects.
//
// Returns the highest-priority trigger and count of items for that trigger.
// Priority order:
// 1. No tasks (initial planning)
// 2. Blocked tasks
// 3. Integration failed
// 4. Hypothesis exhausted (2+ failed_by)
// 5. Immediate discoveries (not yet converted to tasks)
// 6. Planning complete (all planned tasks terminal, merged tasks have output[])
// 7. Sprint complete (all planned tasks terminal)
func DetectOrchestratorWakeTriggers(state *models.State, pipelineTerminals []models.TaskStatus) OrchestratorWakeResult {
	for _, triggerSpec := range orchestratorWakeTriggerSpecs {
		if count := triggerSpec.Count(state); count > 0 {
			return OrchestratorWakeResult{
				Trigger: triggerSpec.Trigger,
				Count:   count,
			}
		}
	}

	// Sprint-complete check: pipeline-aware when terminals are provided,
	// falls back to universal terminals when nil.
	// Guard: suppress when sprint is already CHECKPOINT or COMPLETED to prevent
	// the re-wake loop (supervisor sets COMPLETED → state change fires detection
	// → orchestrator wakes → calls sprint_checkpoint → rejected).
	if state.AllPlannedTasksTerminalWith(pipelineTerminals) {
		if state.Sprint.Status == models.SprintStatusCheckpoint ||
			state.Sprint.Status == models.SprintStatusCompleted {
			return OrchestratorWakeResult{Trigger: WakeTriggerNone}
		}
		// Distinguish planning completion (merged tasks with output[]) from sprint completion.
		if n := countMergedPlanningTasksWithOutput(state); n > 0 {
			return OrchestratorWakeResult{
				Trigger: WakeTriggerPlanningComplete,
				Count:   n,
			}
		}
		return OrchestratorWakeResult{
			Trigger: WakeTriggerSprintComplete,
			Count:   len(state.Sprint.Scope.Planned),
		}
	}

	return OrchestratorWakeResult{
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

// countMergedPlanningTasksWithOutput counts planned tasks that are MERGED,
// have the code-planning-pair role pair, and have non-empty Output[] entries,
// indicating a planning task whose output is ready to be expanded into coding tasks.
// Only planning tasks qualify — coding tasks with output[] are ignored to prevent
// misclassification as PLANNING_COMPLETE during normal coding sprints.
func countMergedPlanningTasksWithOutput(state *models.State) int {
	count := 0
	for _, taskID := range state.Sprint.Scope.Planned {
		task := state.FindTask(taskID)
		if task != nil && task.Status == models.TaskStatusMerged && len(task.Output) > 0 && task.RolePair == "code-planning-pair" {
			count++
		}
	}
	return count
}
