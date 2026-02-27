package ops

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// ClaimReviewerTaskInput contains the parameters for claiming a reviewer task.
type ClaimReviewerTaskInput struct {
	ProjectRoot   string
	AgentID       string
	LeaseDuration int
}

// ClaimReviewerTaskResult contains the outcome of a successful reviewer task claim.
type ClaimReviewerTaskResult struct {
	TaskID       string
	Worktree     string
	ReviewCommit string
	LeaseExpires time.Time
}

// ClaimReviewerTask finds and claims a reviewable task for a code-reviewer agent.
// It atomically transitions the task to REVIEWING, assigns the reviewer, and updates
// the agent status. This operation is reachable from both MCP and CLI consumers.
func ClaimReviewerTask(input ClaimReviewerTaskInput) (*ClaimReviewerTaskResult, error) {
	if input.AgentID == "" {
		return nil, fmt.Errorf("agent ID is required")
	}
	if input.LeaseDuration <= 0 {
		input.LeaseDuration = models.DefaultLeaseDurationSeconds
	}

	lp := paths.New(input.ProjectRoot)
	bb := db.For(lp.StatePath())

	now := time.Now().UTC()
	leaseExpires := now.Add(time.Duration(input.LeaseDuration) * time.Second)

	var result ClaimReviewerTaskResult

	err := bb.Modify(func(state *models.State) error {
		// Find reviewable task with highest priority
		// READY_FOR_REVIEW tasks are available for claiming (stale REVIEWING leases
		// are reverted to READY_FOR_REVIEW by ops.ClearStaleReviewClaims)
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

		// Invariant: task must have review_commit before it can be claimed for review
		if task.ReviewCommit == nil {
			return fmt.Errorf("task %s has no review_commit — cannot claim for review", task.ID)
		}

		// Atomically claim the task and transition to REVIEWING
		if err := task.Transition(models.TaskStatusReviewing); err != nil {
			return err
		}
		task.ReviewingBy = &input.AgentID
		task.ReviewLeaseExpires = &leaseExpires

		// Update agent status
		agent := state.Agents[input.AgentID]
		agent.Status = models.AgentStatusReviewing
		currentTask := task.ID
		agent.CurrentTask = &currentTask
		agent.Heartbeat = now
		agent.LeaseExpires = &leaseExpires
		state.Agents[input.AgentID] = agent

		// Capture values to return
		result.TaskID = task.ID
		if task.Worktree != nil {
			result.Worktree = *task.Worktree
		}
		if task.ReviewCommit != nil {
			result.ReviewCommit = *task.ReviewCommit
		}
		result.LeaseExpires = leaseExpires

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &result, nil
}

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
