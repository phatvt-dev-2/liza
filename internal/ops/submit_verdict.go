package ops

import (
	"fmt"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
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
		return nil, fmt.Errorf("task ID is required")
	}
	if verdict == "" {
		return nil, fmt.Errorf("verdict is required")
	}
	if agentID == "" {
		return nil, fmt.Errorf("LIZA_AGENT_ID is required")
	}

	verdict = strings.ToUpper(verdict)
	if verdict != "APPROVED" && verdict != "REJECTED" {
		return nil, fmt.Errorf("verdict must be APPROVED or REJECTED, got: %s", verdict)
	}

	if verdict == "REJECTED" && reason == "" {
		return nil, fmt.Errorf("rejection reason is required for REJECTED verdict")
	}

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	now := time.Now().UTC()
	escalatedToBlocked := false
	blockedReasonOut := ""

	err := bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		if task.Status != models.TaskStatusReviewing {
			return fmt.Errorf("task %s is not REVIEWING (current status: %s)", taskID, task.Status)
		}

		if verdict == "APPROVED" {
			if err := task.Transition(models.TaskStatusApproved); err != nil {
				return err
			}
			task.ApprovedBy = &agentID
			task.RejectionReason = nil

			agentPtr := &agentID
			task.History = append(task.History, models.TaskHistoryEntry{
				Time:  now,
				Event: "approved",
				Agent: agentPtr,
			})
		} else {
			if err := task.Transition(models.TaskStatusRejected); err != nil {
				return err
			}
			task.RejectionReason = &reason
			task.ReviewCyclesCurrent++
			task.ReviewCyclesTotal++

			agentPtr := &agentID
			reasonPtr := &reason
			task.History = append(task.History, models.TaskHistoryEntry{
				Time:   now,
				Event:  "rejected",
				Agent:  agentPtr,
				Reason: reasonPtr,
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
				if err := task.Transition(models.TaskStatusBlocked); err != nil {
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

				blockedReasonPtr := &blockedReason
				task.History = append(task.History, models.TaskHistoryEntry{
					Time:   now,
					Event:  "blocked",
					Agent:  agentPtr,
					Reason: blockedReasonPtr,
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
