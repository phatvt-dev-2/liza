package ops

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/identity"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/roles"
)

// HandoffResult contains the outcome of a successful handoff initiation.
type HandoffResult struct {
	TaskID  string
	AgentID string
}

// Handoff atomically marks a task for context-exhaustion handoff: sets
// handoff_pending, records a handoff note with summary/next_action, and
// transitions the initiating agent to HANDOFF status. No terminal I/O.
func Handoff(projectRoot, taskID, summary, nextAction, agentID string) (*HandoffResult, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID is required")
	}
	if summary == "" {
		return nil, fmt.Errorf("summary is required")
	}
	if nextAction == "" {
		return nil, fmt.Errorf("next action is required")
	}
	if agentID == "" {
		return nil, fmt.Errorf("LIZA_AGENT_ID is required")
	}

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	now := time.Now().UTC()

	runtimeRole, err := identity.ExtractRole(agentID)
	if err != nil {
		return nil, fmt.Errorf("invalid agent ID %s: %w", agentID, err)
	}

	err = bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		expectedStatus := models.TaskStatusImplementing
		if runtimeRole == roles.RuntimeCodePlanner {
			expectedStatus = models.TaskStatusCodePlanning
		}
		if task.Status != expectedStatus {
			return fmt.Errorf("task %s is not %s (current status: %s)", taskID, expectedStatus, task.Status)
		}

		if task.AssignedTo == nil || *task.AssignedTo != agentID {
			return fmt.Errorf("task %s is not assigned to agent %s", taskID, agentID)
		}

		task.HandoffPending = true
		note := fmt.Sprintf("summary: %s | next_action: %s", summary, nextAction)
		task.History = append(task.History, models.TaskHistoryEntry{
			Time:  now,
			Event: "handoff_initiated",
			Agent: &agentID,
			Note:  &note,
		})

		if state.Handoff == nil {
			state.Handoff = make(map[string]models.HandoffNote)
		}
		state.Handoff[taskID] = models.HandoffNote{
			Agent:      agentID,
			Timestamp:  now,
			Summary:    summary,
			NextAction: nextAction,
		}

		agent, exists := state.Agents[agentID]
		if !exists {
			agent = models.Agent{Role: runtimeRole}
		}
		agent.Status = models.AgentStatusHandoff
		agent.CurrentTask = &taskID
		agent.Heartbeat = now
		state.Agents[agentID] = agent

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initiate handoff: %w", err)
	}

	return &HandoffResult{
		TaskID:  taskID,
		AgentID: agentID,
	}, nil
}
