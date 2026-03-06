package ops

import (
	"fmt"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// SetTaskOutputInput contains the parameters for setting output entries on a task.
type SetTaskOutputInput struct {
	TaskID  string
	AgentID string
	Output  []models.OutputEntry
}

// SetTaskOutput sets the output[] entries on a task. The task must exist, be assigned
// to the given agent, and be in an executing state (IMPLEMENTING, CODE_PLANNING, or
// a pipeline-defined executing status). Overwrites any existing output (idempotent).
func SetTaskOutput(projectRoot string, input *SetTaskOutputInput) error {
	if input.TaskID == "" {
		return fmt.Errorf("task_id is required")
	}
	if input.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if len(input.Output) == 0 {
		return fmt.Errorf("output is required (at least one entry)")
	}

	for i, entry := range input.Output {
		if entry.Desc == "" {
			return fmt.Errorf("output[%d].desc is required", i)
		}
		if entry.DoneWhen == "" {
			return fmt.Errorf("output[%d].done_when is required", i)
		}
		if entry.Scope == "" {
			return fmt.Errorf("output[%d].scope is required", i)
		}
	}

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())

	// Collect pipeline executing statuses (if pipeline config exists)
	var pipelineExecuting []models.TaskStatus
	resolver, _, err := loadResolver(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to load pipeline config: %w", err)
	}
	if resolver != nil {
		for _, rpName := range resolver.RolePairNames() {
			if es, err := resolver.ExecutingStatus(rpName); err == nil {
				pipelineExecuting = append(pipelineExecuting, es)
			}
		}
	}

	return bb.Modify(func(state *models.State) error {
		task := state.FindTask(input.TaskID)
		if task == nil {
			return fmt.Errorf("task %s not found", input.TaskID)
		}

		if !isExecutingStatus(task.Status, pipelineExecuting) {
			return fmt.Errorf("task %s is not in an executing state (current status: %s)", input.TaskID, task.Status)
		}

		if task.AssignedTo == nil || *task.AssignedTo != input.AgentID {
			currentAgent := "none"
			if task.AssignedTo != nil {
				currentAgent = *task.AssignedTo
			}
			return fmt.Errorf("task %s is not assigned to agent %s (currently assigned to: %s)", input.TaskID, input.AgentID, currentAgent)
		}

		task.Output = input.Output
		return nil
	})
}
