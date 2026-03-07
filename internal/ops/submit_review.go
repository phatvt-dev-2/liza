package ops

import (
	stderrors "errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	gitpkg "github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/identity"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/roles"
)

// SubmitForReviewResult contains the outcome of submitting a task for review.
type SubmitForReviewResult struct {
	TaskID       string
	ReviewCommit string
	AgentID      string
}

// SubmitForReview validates that commitSHA matches the worktree HEAD before rebase,
// rebases the task branch onto the integration branch to catch conflicts early,
// then atomically transitions the task to READY_FOR_REVIEW.
// No terminal I/O.
func SubmitForReview(projectRoot, taskID, commitSHA, agentID string) (*SubmitForReviewResult, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID is required")
	}
	if commitSHA == "" {
		return nil, fmt.Errorf("commit SHA is required")
	}
	if agentID == "" {
		return nil, fmt.Errorf("LIZA_AGENT_ID is required")
	}

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())

	runtimeRole, err := identity.ExtractRole(agentID)
	if err != nil {
		return nil, fmt.Errorf("invalid agent ID %s: %w", agentID, err)
	}

	// Phase 1: Read state to get config and validate preconditions
	state, task, err := readTaskState(bb, taskID)
	if err != nil {
		return nil, err
	}

	// Resolve expected statuses (pipeline or legacy)
	expectedCurrentStatus := models.TaskStatusImplementing
	targetSubmittedStatus := models.TaskStatusReadyForReview
	var pipelineTransitions map[models.TaskStatus][]models.TaskStatus

	resolver, _, resolverErr := loadResolver(projectRoot)
	if resolverErr != nil {
		return nil, fmt.Errorf("failed to load pipeline config: %w", resolverErr)
	}
	if resolver != nil && task.RolePair != "" {
		if expectedCurrentStatus, err = resolver.ExecutingStatus(task.RolePair); err != nil {
			return nil, fmt.Errorf("invalid role-pair %q: %w", task.RolePair, err)
		}
		if targetSubmittedStatus, err = resolver.SubmittedStatus(task.RolePair); err != nil {
			return nil, fmt.Errorf("invalid role-pair %q: %w", task.RolePair, err)
		}
		pipelineTransitions = BuildPipelineTransitions(resolver)
	} else if runtimeRole == roles.RuntimeCodePlanner {
		expectedCurrentStatus = models.TaskStatusCodePlanning
		targetSubmittedStatus = models.TaskStatusCodingPlanToReview
	}

	if task.Status != expectedCurrentStatus {
		return nil, fmt.Errorf("task %s is not %s (current status: %s)", taskID, expectedCurrentStatus, task.Status)
	}

	if task.AssignedTo == nil || *task.AssignedTo != agentID {
		currentAgent := "none"
		if task.AssignedTo != nil {
			currentAgent = *task.AssignedTo
		}
		return nil, fmt.Errorf("task %s is not assigned to agent %s (currently assigned to: %s)", taskID, agentID, currentAgent)
	}

	if task.Worktree == nil {
		return nil, fmt.Errorf("task %s has no worktree", taskID)
	}

	// Pre-execution checkpoint required before submission
	if !HasCheckpoint(task.History, agentID) {
		return nil, &PreconditionError{Reason: fmt.Sprintf("task %s: pre-execution checkpoint required before submission (use liza_write_checkpoint)", taskID)}
	}

	// Phase 2: Execute git operations outside the lock
	g := gitpkg.New(projectRoot)
	wtPath := g.GetWorktreePath(taskID)

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("worktree directory does not exist: %s", wtPath)
	}

	wtBranch, err := g.GetWorktreeBranch(wtPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree branch: %w", err)
	}

	expectedBranch := paths.TaskBranchPrefix + taskID
	if wtBranch != expectedBranch {
		if wtBranch == "" {
			return nil, fmt.Errorf("worktree is in detached HEAD state (expected branch: %s)", expectedBranch)
		}
		return nil, fmt.Errorf("worktree is on branch %s (expected: %s)", wtBranch, expectedBranch)
	}

	preRebaseCommit, err := g.GetWorktreeHEAD(taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pre-rebase commit SHA: %w", err)
	}
	if commitSHA != preRebaseCommit {
		return nil, fmt.Errorf("provided commit SHA %s does not match worktree HEAD %s", commitSHA, preRebaseCommit)
	}

	// TDD enforcement: code tasks must include test files (coder role only).
	if runtimeRole == roles.RuntimeCoder && task.EffectiveType() == models.TaskTypeCoding && task.BaseCommit != nil {
		hasTests, err := HasTestFiles(g, taskID, *task.BaseCommit)
		if err != nil {
			return nil, fmt.Errorf("failed to check test files: %w", err)
		}
		if !hasTests && GetTDDWaiver(task.History, agentID) == "" {
			return nil, &PreconditionError{Reason: fmt.Sprintf("task %s: code tasks must include test files (e.g. *_test.go, *.test.ts, test_*.py) — TDD is mandatory", taskID)}
		}
	}

	integrationBranch := state.Config.IntegrationBranch
	if err := g.FetchFromLocal(wtPath, integrationBranch); err != nil {
		return nil, fmt.Errorf("failed to fetch integration branch: %w", err)
	}

	// Capture integration HEAD immediately after fetch — this is the exact ref
	// the rebase targets. Must be before rebase to avoid TOCTOU if integration
	// advances between fetch and rebase completion.
	rebaseBase, err := g.GetCommitSHA(integrationBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to get integration branch HEAD for rebase base: %w", err)
	}

	if err := g.RebaseOnto(wtPath, "FETCH_HEAD"); err != nil {
		// Abort rebase to restore clean worktree state — don't leave agents
		// in a mid-rebase state where they struggle with --continue/--abort.
		if abortErr := g.AbortRebase(wtPath); abortErr != nil {
			log.Printf("WARNING: failed to abort rebase in %s: %v", wtPath, abortErr)
		}

		// Only transition to INTEGRATION_FAILED for true merge conflicts.
		// Generic rebase failures (tool/env issues) are returned as-is so the
		// agent can retry without a state transition.
		var rebaseConflict *gitpkg.RebaseConflictError
		if !stderrors.As(err, &rebaseConflict) {
			return nil, fmt.Errorf("failed to rebase onto integration: %w", err)
		}

		// Transition to INTEGRATION_FAILED so the orchestrator re-queues the task.
		// This catches conflicts early (before review), avoiding a wasted review cycle.
		// See also: markIntegrationFailed in wt_merge.go (sibling for post-review merge path).
		markErr := markSubmitRebaseConflict(bb, taskID, agentID, pipelineTransitions)
		if markErr != nil {
			return nil, fmt.Errorf("rebase conflict on %s (also failed to transition to INTEGRATION_FAILED: %w)", taskID, markErr)
		}
		return nil, &IntegrationFailedError{Reason: IntegrationReasonMergeConflict}
	}

	postRebaseCommit, err := g.GetWorktreeHEAD(taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get post-rebase commit SHA: %w", err)
	}

	// Phase 3: Atomic update with new commit SHA
	now := time.Now().UTC()

	err = bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		if task.Status != expectedCurrentStatus {
			return fmt.Errorf("task %s is not %s (current status: %s)", taskID, expectedCurrentStatus, task.Status)
		}

		if task.AssignedTo == nil || *task.AssignedTo != agentID {
			currentAgent := "none"
			if task.AssignedTo != nil {
				currentAgent = *task.AssignedTo
			}
			return fmt.Errorf("task %s is not assigned to agent %s (currently assigned to: %s)", taskID, agentID, currentAgent)
		}

		if pipelineTransitions != nil {
			if err := task.TransitionWith(targetSubmittedStatus, pipelineTransitions); err != nil {
				return err
			}
		} else {
			if err := task.Transition(targetSubmittedStatus); err != nil {
				return err
			}
		}
		task.ReviewCommit = &postRebaseCommit
		// Update BaseCommit from "branched from" to "rebased onto" — this is the
		// integration HEAD the rebase targeted, ensuring the reviewer diffs only
		// the coder's changes (not integration commits that landed since claim).
		task.BaseCommit = &rebaseBase

		task.History = append(task.History, models.TaskHistoryEntry{
			Time:  now,
			Event: "submitted_for_review",
			Agent: &agentID,
		})

		if agent, ok := state.Agents[agentID]; ok {
			agent.Status = models.AgentStatusWaiting
			agent.CurrentTask = nil
			agent.LeaseExpires = nil
			state.Agents[agentID] = agent
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to submit task for review: %w", err)
	}

	return &SubmitForReviewResult{
		TaskID:       taskID,
		ReviewCommit: postRebaseCommit,
		AgentID:      agentID,
	}, nil
}

// markSubmitRebaseConflict transitions a task from IMPLEMENTING (or pipeline executing
// state) to INTEGRATION_FAILED when a rebase conflict is detected during submission.
// Releases the agent so the orchestrator can re-assign a coder for conflict resolution.
//
// Sibling: markIntegrationFailed in wt_merge.go handles the post-review merge path.
// Both share the pattern: transition → append FailedBy → write history entry.
// They differ in pre-conditions (approved vs implementing) and post-actions (agent release).
func markSubmitRebaseConflict(bb *db.Blackboard, taskID, agentID string, pipelineTransitions map[models.TaskStatus][]models.TaskStatus) error {
	reason := IntegrationReasonMergeConflict
	return bb.Modify(func(s *models.State) error {
		t := s.FindTask(taskID)
		if t == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}
		if pipelineTransitions != nil {
			if err := t.TransitionWith(models.TaskStatusIntegrationFailed, pipelineTransitions); err != nil {
				return err
			}
		} else {
			if err := t.Transition(models.TaskStatusIntegrationFailed); err != nil {
				return err
			}
		}
		t.FailedBy = appendUniqueAgentID(t.FailedBy, agentID)
		t.IntegrationFix = false
		t.AssignedTo = nil
		t.LeaseExpires = nil

		entry := models.TaskHistoryEntry{
			Time:   time.Now().UTC(),
			Event:  "integration_failed",
			Agent:  &agentID,
			Reason: &reason,
		}
		t.History = append(t.History, entry)

		// Release the agent so it can pick up other work
		if agent, ok := s.Agents[agentID]; ok {
			agent.Status = models.AgentStatusWaiting
			agent.CurrentTask = nil
			agent.LeaseExpires = nil
			s.Agents[agentID] = agent
		}

		return nil
	})
}
