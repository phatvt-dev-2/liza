package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
)

const defaultMaxMergeRetries = 3

// reviewerStrategy handles review roles: code-reviewer, code-plan-reviewer,
// epic-plan-reviewer, us-reviewer.
type reviewerStrategy struct {
	role         string             // runtime role
	workflowRole string             // workflow role
	buildContext contextBuilderFunc // per-role prompt context builder
	mergeRetries int                // current retry counter (mutable per-loop state)
	maxRetries   int                // max merge retries before proceeding (0 = use default)
}

func (s *reviewerStrategy) effectiveMaxRetries() int {
	if s.maxRetries > 0 {
		return s.maxRetries
	}
	return defaultMaxMergeRetries
}

func (s *reviewerStrategy) DefaultTimeout() time.Duration {
	return 30 * time.Minute
}

func (s *reviewerStrategy) WaitConfig(state *models.State) (pollInterval, maxWait time.Duration) {
	poll := nonZeroOr(state.Config.ReviewerPollInterval, models.DefaultReviewerPollInterval)
	max := nonZeroOr(state.Config.ReviewerMaxWait, models.DefaultReviewerMaxWait)
	return time.Duration(poll) * time.Second, time.Duration(max) * time.Second
}

func (s *reviewerStrategy) PreWork(_ context.Context, bb *db.Blackboard, config SupervisorConfig) (bool, error) {
	logger := GetLogger()

	pr, prErr := ops.LoadResolverForModels(config.ProjectRoot)
	if prErr != nil {
		logger.Warn("Failed to load pipeline resolver — skipping merge handling", "error", prErr)
	} else {
		if err := handleApprovedMerges(config.ProjectRoot, config.AgentID, bb, pr); err != nil {
			logger.Warn("Merge handler error", "error", err)
		}
	}

	// Execute pipeline transitions on newly-merged tasks
	if err := handleAvailableTransitions(config.ProjectRoot); err != nil {
		logger.Warn("Transition handler error", "error", err)
	}

	// If there are still pending merges (transient errors), retry with
	// backoff up to a max count, then proceed to waitForWork
	if prErr == nil && hasPendingMerges(bb, config.AgentID, pr) {
		s.mergeRetries++
		if s.mergeRetries <= s.effectiveMaxRetries() {
			delay := time.Duration(s.mergeRetries) * time.Second
			logger.Info("Pending merges remain, retrying after delay",
				"agent_id", config.AgentID,
				"retry", s.mergeRetries,
				"delay", delay)
			time.Sleep(delay)
			return true, nil // shouldContinue: restart loop iteration
		}
		logger.Warn("Max merge retries reached, proceeding to wait for work",
			"agent_id", config.AgentID,
			"retries", s.mergeRetries)
		s.mergeRetries = 0
	} else {
		s.mergeRetries = 0
	}

	return false, nil
}

func (s *reviewerStrategy) WaitForWork(ctx context.Context, bb *db.Blackboard, config SupervisorConfig, pollInterval, maxWait time.Duration) (bool, error) {
	if cleared, err := ops.ClearStaleReviewClaims(config.ProjectRoot); err != nil {
		GetLogger().Warn("Failed to clear stale review claims before reviewer wait", "error", err)
	} else if cleared > 0 {
		GetLogger().Info("Cleared stale review claims before reviewer wait", "count", cleared)
	}

	pr := loadResolver(config.ProjectRoot)
	return waitForWorkEventDriven(ctx, bb, config.ProjectRoot, pollInterval, maxWait,
		func(state *models.State) (bool, string) {
			count := models.CountReviewableTasks(state, s.workflowRole, pr)
			if count > 0 {
				return true, fmt.Sprintf("Found %d %s-reviewable task(s)", count, s.role)
			}

			// Use richer diagnostics for code-reviewer role
			if s.role == "code-reviewer" {
				return false, models.GetReviewerWorkDiagnostics(state, pr)
			}
			return false, fmt.Sprintf("No %s-reviewable tasks", s.role)
		})
}

func (s *reviewerStrategy) ClaimTask(config SupervisorConfig, bb *db.Blackboard) (string, string, error) {
	logger := GetLogger()

	taskID, _, reviewCommit, err := claimReviewerTaskForRole(config.ProjectRoot, config.AgentID, s.workflowRole, 1800, bb)
	if err != nil {
		return "", "", err
	}

	logger.Info("Reviewer claimed task for review",
		"agent_id", config.AgentID,
		"task_id", taskID,
		"review_commit", reviewCommit)

	// Verify worktree exists before launching agent.
	_, wtErr := ensureReviewerWorktree(config.ProjectRoot, bb, taskID, config.AgentID)
	if wtErr != nil {
		logger.Warn("Reviewer worktree check failed",
			"task_id", taskID, "error", wtErr)
		if !errors.Is(wtErr, errTaskBlocked) {
			// Transient error (bb.Read, BranchExists, AttachWorktree).
			// blockReviewerTask was NOT called, so the reviewer claim
			// and agent state are still dangling — release them.
			releaseReviewerClaimQuietly(config.ProjectRoot, taskID, config.AgentID)
		}
		// For errTaskBlocked, blockReviewerTask already cleared
		// claim fields and released agent state.
		return "", "", wtErr
	}

	return taskID, "", nil
}

func (s *reviewerStrategy) PreExecution(_ *db.Blackboard, _ SupervisorConfig) error {
	return nil
}

func (s *reviewerStrategy) BuildPrompt(state *models.State, config SupervisorConfig, taskID string) (string, error) {
	return buildPromptWithContext(state, config, taskID, s.buildContext)
}

func (s *reviewerStrategy) PostExecution(_ *db.Blackboard, _ SupervisorConfig, _, _ string, _ *models.State) error {
	return nil
}
