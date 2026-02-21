package ops

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
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

	err := bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return fmt.Errorf("task not found: %s", taskID)
		}

		if task.Status != models.TaskStatusImplementing {
			return fmt.Errorf("task %s is not IMPLEMENTING (current status: %s)", taskID, task.Status)
		}

		if task.AssignedTo == nil || *task.AssignedTo != agentID {
			return fmt.Errorf("task %s is not assigned to agent %s", taskID, agentID)
		}

		task.HandoffPending = true
		note := fmt.Sprintf("summary: %s | next_action: %s", summary, nextAction)
		notePtr := &note
		agentPtr := &agentID
		task.History = append(task.History, models.TaskHistoryEntry{
			Time:  now,
			Event: "handoff_initiated",
			Agent: agentPtr,
			Note:  notePtr,
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
			agent = models.Agent{Role: "coder"}
		}
		currentTask := taskID
		agent.Status = models.AgentStatusHandoff
		agent.CurrentTask = &currentTask
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
