package agent

import (
	"errors"
	"fmt"
	"math/rand/v2"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/roles"
)

func claimDoerTask(projectRoot, agentID, workflowRole string, bb *db.Blackboard) (taskID, worktree string, err error) {
	logger := GetLogger()

	if workflowRole == models.RoleCoder || workflowRole == models.RoleCodePlanner {
		handoffResult, err := ops.ResumeHandoff(ops.ResumeHandoffInput{
			ProjectRoot: projectRoot,
			AgentID:     agentID,
		})
		if err != nil {
			return "", "", err
		}
		if handoffResult.Found {
			logger.Info("Resuming claimed task from handoff", "task_id", handoffResult.TaskID, "agent_id", agentID)
			return handoffResult.TaskID, handoffResult.Worktree, nil
		}
	}

	state, err := bb.Read()
	if err != nil {
		return "", "", fmt.Errorf("failed to read state: %w", err)
	}

	pr := loadResolver(projectRoot)

	var candidates []*models.Task
	for i := range state.Tasks {
		if state.Tasks[i].IsClaimable(workflowRole, state.Tasks, pr) {
			candidates = append(candidates, &state.Tasks[i])
		}
	}
	tier := shuffledByPriorityTier(candidates)

	if len(tier) == 0 {
		return "", "", fmt.Errorf("no claimable tasks found")
	}

	// Try each candidate in the shuffled tier until one succeeds.
	var lastErr error
	for _, task := range tier {
		result, claimErr := ops.ClaimTask(projectRoot, task.ID, agentID)
		if claimErr != nil {
			logger.Warn("Claim attempt failed, trying next candidate",
				"task_id", task.ID, "error", claimErr)
			lastErr = claimErr
			continue
		}
		return result.TaskID, result.WorktreeRel, nil
	}

	return "", "", fmt.Errorf("all %d candidates in top priority tier failed to claim: %w", len(tier), lastErr)
}

// claimCoderTask wraps claimDoerTask for backward compatibility.
func claimCoderTask(projectRoot, agentID string, bb *db.Blackboard) (taskID, worktree string, err error) {
	return claimDoerTask(projectRoot, agentID, models.RoleCoder, bb)
}

// shuffledByPriorityTier returns candidates in the highest-priority tier,
// shuffled randomly. This prevents multiple agents from deterministically
// converging on the same task.
func shuffledByPriorityTier(candidates []*models.Task) []*models.Task {
	tier := models.TopPriorityTier(candidates)
	rand.Shuffle(len(tier), func(i, j int) {
		tier[i], tier[j] = tier[j], tier[i]
	})
	return tier
}

func claimReviewerTaskForRole(projectRoot, agentID, workflowRole string, leaseDuration int, bb *db.Blackboard) (taskID, worktree, reviewCommit string, err error) {
	logger := GetLogger()

	result, err := ops.ClaimReviewerTask(ops.ClaimReviewerTaskInput{
		ProjectRoot:   projectRoot,
		AgentID:       agentID,
		WorkflowRole:  workflowRole,
		LeaseDuration: leaseDuration,
	})
	if err != nil {
		logger.Error("Review claim error", "error", err)
		return "", "", "", err
	}

	return result.TaskID, result.Worktree, result.ReviewCommit, nil
}

// claimReviewerTask wraps claimReviewerTaskForRole for backward compatibility.
func claimReviewerTask(projectRoot, agentID string, leaseDuration int, bb *db.Blackboard) (taskID, worktree, reviewCommit string, err error) {
	return claimReviewerTaskForRole(projectRoot, agentID, models.RoleCodeReviewer, leaseDuration, bb)
}

// releaseReviewerClaimQuietly releases a reviewer claim, logging but not
// propagating errors. Used in supervisor recovery paths for transient failures
// where blockReviewerTask was NOT called.
func releaseReviewerClaimQuietly(projectRoot, taskID, agentID string) {
	_, err := ops.ReleaseClaim(projectRoot, taskID, roles.ClaimReviewer, true, "supervisor: worktree check transient failure", agentID)
	if err != nil {
		GetLogger().Warn("Failed to release reviewer claim during recovery",
			"task_id", taskID, "error", err)
	}
}

func handleApprovedMerges(projectRoot, agentID string, bb *db.Blackboard, pr models.PipelineResolver) error {
	logger := GetLogger()
	state, err := bb.Read()
	if err != nil {
		return err
	}

	for i := range state.Tasks {
		task := &state.Tasks[i]
		if models.IsApprovedForMerge(task, pr) &&
			task.LastApprover() == agentID &&
			task.MergeCommit == nil {

			logger.Info("Merging approved task", "task_id", task.ID)

			result, err := ops.MergeWorktree(projectRoot, task.ID, agentID)
			if err != nil {
				var integrationErr *ops.IntegrationFailedError
				if errors.As(err, &integrationErr) {
					logArgs := []any{
						"task_id", task.ID,
						"reason", integrationErr.Reason,
					}
					if integrationErr.TestOutput != "" {
						logArgs = append(logArgs, "test_output", integrationErr.TestOutput)
					}
					if integrationErr.RollbackError != nil {
						logArgs = append(logArgs, "rollback_error", integrationErr.RollbackError)
					}
					logger.Warn("Integration failed", logArgs...)
					continue
				}
				logger.Warn("Failed to merge task, will retry",
					"task_id", task.ID,
					"error", err)
				continue
			}

			for _, w := range result.Warnings {
				logger.Warn("Merge cleanup warning", "task_id", task.ID, "warning", w)
			}

			logger.Info("Successfully merged task", "task_id", task.ID)
		}
	}

	return nil
}

func hasPendingMerges(bb *db.Blackboard, agentID string, pr models.PipelineResolver) bool {
	state, err := bb.ReadCached()
	if err != nil {
		return false // Safe default: proceed to normal wait
	}

	for i := range state.Tasks {
		task := &state.Tasks[i]
		if models.IsApprovedForMerge(task, pr) &&
			task.LastApprover() == agentID &&
			task.MergeCommit == nil {
			return true
		}
	}
	return false
}

// handleAvailableTransitions creates child tasks from pipeline transitions.
// Children are NOT added to the current sprint's scope — they carry to the next sprint.
func handleAvailableTransitions(projectRoot string) error {
	results, err := ops.ExecuteAvailableTransitions(projectRoot)
	if err != nil {
		return err
	}

	logger := GetLogger()
	for _, r := range results {
		logger.Info("Pipeline transition executed",
			"source_task", r.SourceTaskID,
			"transition", r.TransitionName,
			"children_created", len(r.ChildTaskIDs))
	}

	return nil
}

func logTaskSubmissionIfCompleted(bb *db.Blackboard, taskID, agentID string, pr models.PipelineResolver) error {
	state, err := bb.Read()
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	if task := state.FindTask(taskID); task != nil {
		if models.IsSubmittedStatus(task, pr) {
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

		if models.IsExecutingStatus(task, pr) {
			GetLogger().Warn("Agent exited with task still claimed",
				"task_id", task.ID,
				"agent_id", agentID,
				"hint", "Agent may have been interrupted or encountered an issue")
			return nil
		}

		if task.Status == models.TaskStatusBlocked {
			GetLogger().Info("Agent blocked task due to dependency issue",
				"task_id", task.ID,
				"agent_id", agentID)
			return nil
		}

		return nil
	}

	return nil
}
