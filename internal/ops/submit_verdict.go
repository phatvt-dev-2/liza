package ops

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/identity"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// VerdictResult contains the outcome of a successful verdict submission.
type VerdictResult struct {
	TaskID             string
	Verdict            string // "APPROVED" or "REJECTED"
	AgentID            string
	Reason             string // non-empty for rejections
	EscalatedToBlocked bool
	BlockedReason      string
}

// SubmitVerdict atomically applies a review verdict: APPROVED transitions to
// APPROVED status, REJECTED increments review cycles and requires a reason.
// No terminal I/O.
func SubmitVerdict(projectRoot, taskID, verdict, reason, agentID string) (*VerdictResult, error) {
	if taskID == "" {
		return nil, &PreconditionError{Reason: "task ID is required"}
	}
	if verdict == "" {
		return nil, &PreconditionError{Reason: "verdict is required"}
	}
	if agentID == "" {
		return nil, &PreconditionError{Reason: "LIZA_AGENT_ID is required"}
	}

	verdict = strings.ToUpper(verdict)
	if verdict != "APPROVED" && verdict != "REJECTED" {
		return nil, &PreconditionError{Reason: fmt.Sprintf("verdict must be APPROVED or REJECTED, got: %s", verdict)}
	}

	if verdict == "REJECTED" && reason == "" {
		return nil, &PreconditionError{Reason: "rejection reason is required for REJECTED verdict"}
	}

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())

	if _, err := identity.ExtractRole(agentID); err != nil {
		return nil, fmt.Errorf("invalid agent ID %s: %w", agentID, err)
	}

	// Phase 1: Read state and validate preconditions
	_, task, err := readTaskState(bb, taskID)
	if err != nil {
		return nil, err
	}

	// Resolve expected statuses from pipeline config
	resolver, _, resolverErr := loadResolver(projectRoot)
	if resolverErr != nil {
		return nil, fmt.Errorf("failed to load pipeline config: %w", resolverErr)
	}
	if task.RolePair == "" {
		return nil, &PreconditionError{Reason: fmt.Sprintf("task %s has no role_pair set", taskID)}
	}
	expectedReviewingStatus, err := resolver.ReviewingStatus(task.RolePair)
	if err != nil {
		return nil, fmt.Errorf("invalid role-pair %q: %w", task.RolePair, err)
	}
	approvedStatus, err := resolver.ApprovedStatus(task.RolePair)
	if err != nil {
		return nil, fmt.Errorf("invalid role-pair %q: %w", task.RolePair, err)
	}
	rejectedStatus, err := resolver.RejectedStatus(task.RolePair)
	if err != nil {
		return nil, fmt.Errorf("invalid role-pair %q: %w", task.RolePair, err)
	}
	pipelineTransitions := BuildPipelineTransitions(resolver)

	// Fast-fail before git operations; re-checked authoritatively inside Modify.
	if task.Status != expectedReviewingStatus {
		return nil, &PreconditionError{Reason: fmt.Sprintf("task %s is not %s (current status: %s)", taskID, expectedReviewingStatus, task.Status)}
	}

	// Phase 2: Validate ReviewCommit exists and matches worktree HEAD
	if task.ReviewCommit == nil {
		return nil, &PreconditionError{Reason: fmt.Sprintf("task %s has no review_commit — cannot submit verdict", taskID)}
	}

	g := git.New(projectRoot)
	wtPath := g.GetWorktreePath(taskID)
	if _, statErr := os.Stat(wtPath); os.IsNotExist(statErr) {
		// Worktree absent on disk (e.g. tests without real worktrees) — skip check.
	} else if statErr != nil {
		return nil, fmt.Errorf("failed to stat worktree %s: %w", wtPath, statErr)
	} else {
		wtHEAD, headErr := g.GetWorktreeHEAD(taskID)
		if headErr != nil {
			return nil, fmt.Errorf("failed to get worktree HEAD: %w", headErr)
		}
		if *task.ReviewCommit != wtHEAD {
			return nil, &PreconditionError{Reason: fmt.Sprintf("review_commit %s does not match worktree HEAD %s — worktree was modified after submission", *task.ReviewCommit, wtHEAD)}
		}
	}

	// Phase 3: Atomic state update
	now := time.Now().UTC()
	escalatedToBlocked := false
	blockedReasonOut := ""

	err = bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		if task.Status != expectedReviewingStatus {
			return &PreconditionError{Reason: fmt.Sprintf("task %s is not %s (current status: %s)", taskID, expectedReviewingStatus, task.Status)}
		}

		transitionTask := func(to models.TaskStatus) error {
			return task.TransitionWith(to, pipelineTransitions)
		}

		if verdict == "APPROVED" {
			if err := transitionTask(approvedStatus); err != nil {
				return err
			}

			// Build approval from agent registry and append to approvals list
			provider := ""
			if agent, ok := state.Agents[agentID]; ok {
				provider = agent.Provider
			}
			task.Approvals = append(task.Approvals, models.Approval{
				Agent:     agentID,
				Provider:  provider,
				Timestamp: now,
			})

			// Derived field for backward compatibility
			task.ApprovedBy = &agentID
			task.RejectionReason = nil

			task.History = append(task.History, models.TaskHistoryEntry{
				Time:  now,
				Event: models.TaskEventApproved,
				Agent: &agentID,
			})
		} else {
			if err := transitionTask(rejectedStatus); err != nil {
				return err
			}

			// Rejection at any stage clears all approvals (spec: both reviewers re-review)
			task.ClearApprovals()

			task.RejectionReason = &reason
			task.ReviewCyclesCurrent++
			task.ReviewCyclesTotal++

			task.History = append(task.History, models.TaskHistoryEntry{
				Time:   now,
				Event:  models.TaskEventRejected,
				Agent:  &agentID,
				Reason: &reason,
			})

			reviewLimit := effectiveReviewCycleLimit(state.Config)
			iterationLimit := effectiveCoderIterationLimit(task, state.Config)

			escalation, shouldEscalate := classifyLimitEscalation(
				task.ReviewCyclesCurrent,
				reviewLimit,
				task.Iteration,
				iterationLimit,
			)
			if shouldEscalate {
				if err := transitionTask(models.TaskStatusBlocked); err != nil {
					return err
				}

				blockedReason := escalation.reason
				task.BlockedReason = &blockedReason
				task.BlockedQuestions = escalation.questions
				task.LeaseExpires = nil
				escalatedToBlocked = true
				blockedReasonOut = blockedReason

				if task.AssignedTo != nil {
					assignedCoder := *task.AssignedTo
					if assignedCoder != agentID {
						state.ReleaseAgent(assignedCoder)
					}
				}
				task.AssignedTo = nil

				task.History = append(task.History, models.TaskHistoryEntry{
					Time:   now,
					Event:  models.TaskEventBlocked,
					Agent:  &agentID,
					Reason: &blockedReason,
				})
			}
		}

		task.ReviewingBy = nil
		task.ReviewLeaseExpires = nil
		state.ReleaseAgent(agentID)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to submit verdict: %w", err)
	}

	return &VerdictResult{
		TaskID:             taskID,
		Verdict:            verdict,
		AgentID:            agentID,
		Reason:             reason,
		EscalatedToBlocked: escalatedToBlocked,
		BlockedReason:      blockedReasonOut,
	}, nil
}
