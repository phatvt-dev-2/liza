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
	WakeTriggerNone                PlannerWakeTrigger = "NONE"
)

// PlannerWakeResult contains the wake trigger and count
type PlannerWakeResult struct {
	Trigger PlannerWakeTrigger
	Count   int
}

// DetectPlannerWakeTriggers detects conditions that should wake the planner
// Returns the highest-priority trigger and count of items for that trigger
// Priority order (per bash script):
// 1. No tasks (initial planning)
// 2. Blocked tasks
// 3. Integration failed
// 4. Hypothesis exhausted (2+ failed_by)
// 5. Immediate discoveries (not yet converted to tasks)
func DetectPlannerWakeTriggers(state *models.State) PlannerWakeResult {
	// Check for initial planning
	if len(state.Tasks) == 0 {
		return PlannerWakeResult{
			Trigger: WakeTriggerInitialPlanning,
			Count:   1,
		}
	}

	// Count blocked tasks
	blocked := 0
	for _, task := range state.Tasks {
		if task.Status == models.TaskStatusBlocked {
			blocked++
		}
	}
	if blocked > 0 {
		return PlannerWakeResult{
			Trigger: WakeTriggerBlocked,
			Count:   blocked,
		}
	}

	// Count integration failures
	integrationFailed := 0
	for _, task := range state.Tasks {
		if task.Status == models.TaskStatusIntegrationFailed {
			integrationFailed++
		}
	}
	if integrationFailed > 0 {
		return PlannerWakeResult{
			Trigger: WakeTriggerIntegrationFailed,
			Count:   integrationFailed,
		}
	}

	// Count hypothesis exhaustion (2+ failed coders on non-terminal tasks)
	hypothesisExhausted := 0
	for _, task := range state.Tasks {
		if len(task.FailedBy) >= 2 && !task.Status.IsTerminal() {
			hypothesisExhausted++
		}
	}
	if hypothesisExhausted > 0 {
		return PlannerWakeResult{
			Trigger: WakeTriggerHypothesisExhausted,
			Count:   hypothesisExhausted,
		}
	}

	// Count immediate discoveries not yet converted to tasks
	immediateDiscoveries := 0
	for _, disc := range state.Discovered {
		if disc.Urgency == "immediate" && disc.ConvertedToTask == nil {
			immediateDiscoveries++
		}
	}
	if immediateDiscoveries > 0 {
		return PlannerWakeResult{
			Trigger: WakeTriggerImmediateDiscovery,
			Count:   immediateDiscoveries,
		}
	}

	// No triggers
	return PlannerWakeResult{
		Trigger: WakeTriggerNone,
		Count:   0,
	}
}
