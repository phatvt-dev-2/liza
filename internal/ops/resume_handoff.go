package ops

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	lizaerrors "github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// ResumeHandoffInput contains the parameters for resuming a handoff task.
type ResumeHandoffInput struct {
	ProjectRoot string
	AgentID     string
}

// ResumeHandoffResult contains the outcome of a successful handoff resumption.
type ResumeHandoffResult struct {
	TaskID   string
	Worktree string
	Found    bool
}

// ResumeHandoff looks for a handoff task assigned to agentID and resumes it.
// Returns Found=false when no resumable handoff exists.
func ResumeHandoff(input ResumeHandoffInput) (*ResumeHandoffResult, error) {
	if input.AgentID == "" {
		return nil, fmt.Errorf("agent ID is required")
	}

	lp := paths.New(input.ProjectRoot)
	bb := db.For(lp.StatePath())

	state, err := bb.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read state: %w", err)
	}

	return resumeHandoffWithState(bb, state, input.AgentID)
}

// resumeHandoffWithState performs the handoff resumption with an already-read state.
// This allows for efficient checking without re-reading state.
func resumeHandoffWithState(bb *db.Blackboard, state *models.State, agentID string) (*ResumeHandoffResult, error) {
	now := time.Now().UTC()

	for i := range state.Tasks {
		task := &state.Tasks[i]
		if !isResumableHandoff(task, agentID) {
			continue
		}
		if task.Worktree == nil {
			return nil, fmt.Errorf("handoff task %s missing worktree", task.ID)
		}

		id := task.ID
		wt := *task.Worktree

		err := bb.Modify(func(s *models.State) error {
			t := s.FindTask(id)
			if t == nil {
				return &lizaerrors.NotFoundError{Entity: "task", ID: id}
			}
			if t.Status != models.TaskStatusImplementing {
				return fmt.Errorf("task %s is no longer IMPLEMENTING", id)
			}
			if t.AssignedTo == nil || *t.AssignedTo != agentID {
				return fmt.Errorf("task %s is no longer assigned to %s", id, agentID)
			}

			if t.LeaseExpires == nil || t.LeaseExpires.Before(now) {
				leaseDuration := s.Config.LeaseDuration
				if leaseDuration <= 0 {
					leaseDuration = models.DefaultLeaseDurationSeconds
				}
				renewed := now.Add(time.Duration(leaseDuration) * time.Second)
				t.LeaseExpires = &renewed
			}

			t.HandoffPending = false
			agentPtr := &agentID
			t.History = append(t.History, models.TaskHistoryEntry{
				Time:  now,
				Event: "handoff_resumed",
				Agent: agentPtr,
			})

			agent, ok := s.Agents[agentID]
			if !ok {
				agent = models.Agent{Role: "coder"}
			}
			agent.Status = models.AgentStatusWorking
			agent.CurrentTask = &id
			agent.LeaseExpires = t.LeaseExpires
			agent.Heartbeat = now
			s.Agents[agentID] = agent
			return nil
		})
		if err != nil {
			// Conflict on this candidate, try next
			continue
		}

		return &ResumeHandoffResult{
			TaskID:   id,
			Worktree: wt,
			Found:    true,
		}, nil
	}

	return &ResumeHandoffResult{Found: false}, nil
}

// isResumableHandoff checks if the task is a handoff that can be resumed by the given agent.
func isResumableHandoff(task *models.Task, agentID string) bool {
	if task.Status != models.TaskStatusImplementing {
		return false
	}
	if task.AssignedTo == nil || *task.AssignedTo != agentID {
		return false
	}
	if !task.HandoffPending {
		return false
	}
	return true
}
