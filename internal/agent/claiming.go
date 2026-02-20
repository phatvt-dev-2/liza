package agent

import (
	"errors"
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
)

// selectHighestPriorityTask returns the highest-priority task from candidates,
// using creation time as FIFO tie-breaker. Returns nil if candidates is empty.
func selectHighestPriorityTask(candidates []*models.Task) *models.Task {
	var best *models.Task
	for _, t := range candidates {
		if best == nil || t.Priority < best.Priority {
			best = t
		} else if t.Priority == best.Priority && best.Created.After(t.Created) {
			best = t
		}
	}
	return best
}

// claimCoderTask finds and claims a claimable task.
// If the same coder previously initiated a handoff, it resumes that task first.
func claimCoderTask(projectRoot, agentID string, bb *db.Blackboard) (taskID, worktree string, err error) {
	logger := GetLogger()

	state, err := bb.Read()
	if err != nil {
		return "", "", fmt.Errorf("failed to read state: %w", err)
	}

	id, wt, found, err := resumeHandoffTask(bb, state, agentID)
	if err != nil {
		return "", "", err
	}
	if found {
		logger.Info("Resuming claimed task from handoff", "task_id", id, "agent_id", agentID)
		return id, wt, nil
	}

	var candidates []*models.Task
	for i := range state.Tasks {
		if state.Tasks[i].IsClaimable(models.RoleCoder, state.Tasks) {
			candidates = append(candidates, &state.Tasks[i])
		}
	}
	task := selectHighestPriorityTask(candidates)

	if task == nil {
		return "", "", fmt.Errorf("no claimable tasks found")
	}

	if err := commands.ClaimTaskCommand(projectRoot, task.ID, agentID); err != nil {
		logger.Error("Claim error", "error", err)
		return "", "", err
	}

	state, err = bb.Read()
	if err != nil {
		return "", "", fmt.Errorf("failed to read state after claim: %w", err)
	}

	if claimed := state.FindTask(task.ID); claimed != nil && claimed.Worktree != nil {
		return task.ID, *claimed.Worktree, nil
	}

	return "", "", fmt.Errorf("task worktree not set after claim")
}

// resumeHandoffTask looks for a handoff task assigned to agentID and resumes it.
// Returns found=false when no resumable handoff exists.
func resumeHandoffTask(bb *db.Blackboard, state *models.State, agentID string) (taskID, worktree string, found bool, err error) {
	for i := range state.Tasks {
		task := &state.Tasks[i]
		if !isResumableHandoff(task, agentID) {
			continue
		}
		if task.Worktree == nil {
			return "", "", false, fmt.Errorf("handoff task %s missing worktree", task.ID)
		}

		now := time.Now().UTC()
		id := task.ID
		wt := *task.Worktree

		err := bb.Modify(func(s *models.State) error {
			t := s.FindTask(id)
			if t == nil {
				return fmt.Errorf("task %s not found while resuming handoff", id)
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
					leaseDuration = 1800
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
			GetLogger().Warn("Handoff resume conflict, trying next candidate", "task_id", id, "error", err)
			continue
		}

		return id, wt, true, nil
	}
	return "", "", false, nil
}

// claimReviewerTask finds and claims a reviewable task
func claimReviewerTask(agentID string, leaseDuration int, bb *db.Blackboard) (taskID, worktree, reviewCommit string, err error) {
	logger := GetLogger()
	now := time.Now().UTC()
	leaseExpires := now.Add(time.Duration(leaseDuration) * time.Second)

	err = bb.Modify(func(state *models.State) error {
		// Find reviewable task with highest priority
		// READY_FOR_REVIEW tasks are available for claiming (stale REVIEWING leases
		// are reverted to READY_FOR_REVIEW by ClearStaleReviewClaimsCommand)
		var candidates []*models.Task
		for i := range state.Tasks {
			if state.Tasks[i].IsClaimable(models.RoleCodeReviewer, state.Tasks) {
				candidates = append(candidates, &state.Tasks[i])
			}
		}
		task := selectHighestPriorityTask(candidates)

		if task == nil {
			return fmt.Errorf("no reviewable tasks found")
		}

		// Atomically claim the task and transition to REVIEWING
		if err := task.Transition(models.TaskStatusReviewing); err != nil {
			return err
		}
		task.ReviewingBy = &agentID
		task.ReviewLeaseExpires = &leaseExpires

		// Update agent status
		agent := state.Agents[agentID]
		agent.Status = models.AgentStatusReviewing
		currentTask := task.ID
		agent.CurrentTask = &currentTask
		agent.Heartbeat = now
		agent.LeaseExpires = &leaseExpires
		state.Agents[agentID] = agent

		// Capture values to return
		taskID = task.ID
		if task.Worktree != nil {
			worktree = *task.Worktree
		}
		if task.ReviewCommit != nil {
			reviewCommit = *task.ReviewCommit
		}

		return nil
	})

	if err != nil {
		logger.Error("Review claim error", "error", err)
		return "", "", "", err
	}

	return taskID, worktree, reviewCommit, nil
}

// handleApprovedMerges handles merging approved tasks
func handleApprovedMerges(projectRoot, agentID string, bb *db.Blackboard) error {
	logger := GetLogger()
	state, err := bb.Read()
	if err != nil {
		return err
	}

	// Find APPROVED tasks where approved_by = agentID and merge_commit = null
	for i := range state.Tasks {
		task := &state.Tasks[i]
		if task.Status == models.TaskStatusApproved &&
			task.ApprovedBy != nil && *task.ApprovedBy == agentID &&
			task.MergeCommit == nil {

			GetLogger().Info("Merging approved task", "task_id", task.ID)

			// Execute merge - WtMergeCommand handles all validation and state updates
			err := commands.WtMergeCommand(projectRoot, task.ID, agentID)
			if err != nil {
				// Check if this is an integration failure (merge conflict or test failure)
				var integrationErr *commands.IntegrationFailedError
				if errors.As(err, &integrationErr) {
					// Integration failed - state already updated, no success message
					continue
				}
				// Other error - log and continue
				logger.Warn("Failed to merge task, will retry",
					"task_id", task.ID,
					"error", err)
				continue
			}

			// Merge succeeded
			GetLogger().Info("Successfully merged task", "task_id", task.ID)
		}
	}

	return nil
}

// hasPendingMerges checks if there are APPROVED tasks awaiting merge by this agent
func hasPendingMerges(bb *db.Blackboard, agentID string) bool {
	state, err := bb.ReadCached()
	if err != nil {
		return false // Safe default: proceed to normal wait
	}

	for i := range state.Tasks {
		task := &state.Tasks[i]
		if task.Status == models.TaskStatusApproved &&
			task.ApprovedBy != nil && *task.ApprovedBy == agentID &&
			task.MergeCommit == nil {
			return true
		}
	}
	return false
}

// logTaskSubmissionIfCompleted checks if a claimed task was submitted for review
// and logs this transition for visibility in agent logs
func logTaskSubmissionIfCompleted(bb *db.Blackboard, taskID, agentID string) error {
	state, err := bb.Read()
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	// Find the task
	if task := state.FindTask(taskID); task != nil {
		// Check if it's now READY_FOR_REVIEW
		if task.Status == models.TaskStatusReadyForReview {
			// Log the successful submission
			reviewCommit := "unknown"
			if task.ReviewCommit != nil {
				reviewCommit = *task.ReviewCommit
			}

			GetLogger().Info("Task submitted for review",
				"task_id", task.ID,
				"review_commit", reviewCommit,
				"agent_id", agentID,
				"integration_fix", task.IntegrationFix)

			return nil
		}

		// If task is still IMPLEMENTING, agent may have exited without completing
		if task.Status == models.TaskStatusImplementing {
			GetLogger().Warn("Agent exited with task still claimed",
				"task_id", task.ID,
				"agent_id", agentID,
				"hint", "Agent may have been interrupted or encountered an issue")
			return nil
		}

		// If task is BLOCKED, agent discovered a dependency issue
		if task.Status == models.TaskStatusBlocked {
			GetLogger().Info("Agent blocked task due to dependency issue",
				"task_id", task.ID,
				"agent_id", agentID)
			return nil
		}

		// Task exists but wasn't submitted (still in other status)
		// This is normal if agent exited for other reasons (context switch, failure, etc.)
		return nil
	}

	// Task not found - unusual but not an error
	return nil
}
