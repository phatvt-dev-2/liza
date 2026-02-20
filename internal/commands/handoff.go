package commands

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// HandoffCommand atomically initiates context-exhaustion handoff for a claimed task.
// It marks task.handoff_pending, records a handoff note, appends task history,
// and sets the initiating agent status to HANDOFF.
func HandoffCommand(projectRoot, taskID, summary, nextAction, agentID string) error {
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}
	if summary == "" {
		return fmt.Errorf("summary is required")
	}
	if nextAction == "" {
		return fmt.Errorf("next action is required")
	}
	if agentID == "" {
		return fmt.Errorf("LIZA_AGENT_ID is required")
	}

	lp := paths.New(projectRoot)
	bb := db.New(lp.StatePath())
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
		return fmt.Errorf("failed to initiate handoff: %w", err)
	}

	fmt.Printf("HANDOFF: %s\n", taskID)
	fmt.Printf("  by: %s\n", agentID)
	return nil
}
