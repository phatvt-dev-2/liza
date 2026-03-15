package ops

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/identity"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// ClaimReviewerTaskInput contains the parameters for claiming a reviewer task.
type ClaimReviewerTaskInput struct {
	ProjectRoot   string
	AgentID       string
	WorkflowRole  string
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
		return nil, &PreconditionError{Reason: "agent ID is required"}
	}
	if input.LeaseDuration <= 0 {
		input.LeaseDuration = models.DefaultLeaseDurationSeconds
	}

	workflowRole := input.WorkflowRole
	if workflowRole == "" {
		// Backward-compatible default: infer from agent_id, then default to code reviewer.
		role, err := identity.ExtractRole(input.AgentID)
		if err == nil && role == "code-plan-reviewer" {
			workflowRole = models.RoleCodePlanReviewer
		} else {
			workflowRole = models.RoleCodeReviewer
		}
	}

	lp := paths.New(input.ProjectRoot)
	bb := db.For(lp.StatePath())

	now := time.Now().UTC()
	leaseExpires := now.Add(time.Duration(input.LeaseDuration) * time.Second)

	var result ClaimReviewerTaskResult

	// Load pipeline config once for both IsClaimable and transition.
	pb, err := loadPipelineBundle(input.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load pipeline config: %w", err)
	}
	pr := pb.pr

	err = bb.Modify(func(state *models.State) error {
		// Find reviewable task with highest priority
		// READY_FOR_REVIEW tasks are available for claiming (stale REVIEWING leases
		// are reverted to READY_FOR_REVIEW by ops.ClearStaleReviewClaims)
		var candidates []*models.Task
		for i := range state.Tasks {
			if state.Tasks[i].IsClaimable(workflowRole, state.Tasks, pr) {
				candidates = append(candidates, &state.Tasks[i])
			}
		}
		task := pickRandomFromTopTier(candidates)

		if task == nil {
			return &PreconditionError{Reason: "no reviewable tasks found"}
		}

		// Invariant: task must have review_commit before it can be claimed for review
		if task.ReviewCommit == nil {
			return &PreconditionError{Reason: fmt.Sprintf("task %s has no review_commit — cannot claim for review", task.ID)}
		}

		if task.RolePair == "" {
			return &PreconditionError{Reason: fmt.Sprintf("task %s has no role_pair set", task.ID)}
		}
		reviewing, err := pr.ReviewingStatus(task.RolePair)
		if err != nil {
			return fmt.Errorf("failed to resolve reviewing status for role-pair %q: %w", task.RolePair, err)
		}
		if err := task.TransitionWith(reviewing, pb.transitions); err != nil {
			return err
		}
		task.ReviewingBy = &input.AgentID
		task.ReviewLeaseExpires = &leaseExpires

		agent := state.Agents[input.AgentID]
		agent.Status = models.AgentStatusReviewing
		currentTask := task.ID
		agent.CurrentTask = &currentTask
		agent.Heartbeat = now
		agent.LeaseExpires = &leaseExpires
		state.Agents[input.AgentID] = agent

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

// pickRandomFromTopTier selects a random task from the highest-priority tier.
// This prevents multiple reviewers from deterministically converging on the
// same task. Returns nil if candidates is empty.
func pickRandomFromTopTier(candidates []*models.Task) *models.Task {
	tier := models.TopPriorityTier(candidates)
	if len(tier) == 0 {
		return nil
	}
	return tier[rand.IntN(len(tier))]
}
