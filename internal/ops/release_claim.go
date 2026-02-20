package ops

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// ReleaseClaimResult contains the outcome of releasing a claim.
type ReleaseClaimResult struct {
	TaskID           string
	Role             string
	ReleasedReviewer bool
	ReleasedCoder    bool
}

// claimRelease describes the field access pattern for one role's claim on a task.
type claimRelease struct {
	hasClaimFn      func(*models.Task) bool
	agentFieldFn    func(*models.Task) *string
	leaseFieldFn    func(*models.Task) *time.Time
	activeStatus    models.TaskStatus
	releasedStatus  models.TaskStatus
	eventName       string
	clearFn         func(*models.Task)
	missingLeaseMsg string
	activeLeaseMsg  string
}

var reviewerRelease = claimRelease{
	hasClaimFn:   func(t *models.Task) bool { return t.ReviewingBy != nil || t.ReviewLeaseExpires != nil },
	agentFieldFn: func(t *models.Task) *string { return t.ReviewingBy },
	leaseFieldFn: func(t *models.Task) *time.Time {
		if t.ReviewLeaseExpires == nil {
			return nil
		}
		return t.ReviewLeaseExpires
	},
	activeStatus:    models.TaskStatusReviewing,
	releasedStatus:  models.TaskStatusReadyForReview,
	eventName:       "review_claim_released",
	clearFn:         func(t *models.Task) { t.ReviewingBy = nil; t.ReviewLeaseExpires = nil },
	missingLeaseMsg: "review lease expires missing for task %s, use --force to clear",
	activeLeaseMsg:  "review lease still valid until %s, use --force to clear",
}

var coderRelease = claimRelease{
	hasClaimFn:   func(t *models.Task) bool { return t.AssignedTo != nil || t.LeaseExpires != nil },
	agentFieldFn: func(t *models.Task) *string { return t.AssignedTo },
	leaseFieldFn: func(t *models.Task) *time.Time {
		if t.LeaseExpires == nil {
			return nil
		}
		return t.LeaseExpires
	},
	activeStatus:    models.TaskStatusImplementing,
	releasedStatus:  models.TaskStatusReady,
	eventName:       "coder_claim_released",
	clearFn:         func(t *models.Task) { t.AssignedTo = nil; t.LeaseExpires = nil },
	missingLeaseMsg: "lease expires missing for task %s, use --force to clear",
	activeLeaseMsg:  "coder lease still valid until %s, use --force to clear",
}

// releaseOneClaim executes the 9-step release sequence for a single role's claim.
// Returns true if a claim was released.
func releaseOneClaim(state *models.State, task *models.Task, cfg claimRelease, force bool, agentID, reason string, now time.Time) (bool, error) {
	if !cfg.hasClaimFn(task) {
		return false, nil
	}

	agent := cfg.agentFieldFn(task)
	lease := cfg.leaseFieldFn(task)

	if agent != nil && lease == nil && !force {
		return false, fmt.Errorf(cfg.missingLeaseMsg, task.ID)
	}

	if lease != nil && !force {
		if lease.After(now) {
			return false, fmt.Errorf(cfg.activeLeaseMsg, lease.Format(time.RFC3339))
		}
	}

	if task.Status == cfg.activeStatus {
		if err := task.Transition(cfg.releasedStatus); err != nil {
			return false, err
		}
	}

	if agent != nil {
		state.ReleaseAgent(*agent)
	}

	cfg.clearFn(task)

	task.History = append(task.History, models.TaskHistoryEntry{
		Time:   now,
		Event:  cfg.eventName,
		Agent:  &agentID,
		Reason: &reason,
	})

	return true, nil
}

// ReleaseClaim releases reviewer, coder, or both claims on a task. Without
// force, refuses if lease is still valid. No terminal I/O.
func ReleaseClaim(projectRoot, taskID, role string, force bool, reason, agentID string) (*ReleaseClaimResult, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID is required")
	}

	if role != "reviewer" && role != "coder" && role != "both" {
		return nil, fmt.Errorf("role must be reviewer, coder, or both, got: %s", role)
	}

	if agentID == "" {
		agentID = "human"
	}

	if reason == "" {
		reason = "manual release"
	}

	lp := paths.New(projectRoot)
	bb := db.New(lp.StatePath())

	releasedReviewer := false
	releasedCoder := false

	now := time.Now().UTC()

	err := bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return fmt.Errorf("task not found: %s", taskID)
		}

		if role == "reviewer" || role == "both" {
			released, err := releaseOneClaim(state, task, reviewerRelease, force, agentID, reason, now)
			if err != nil {
				return err
			}
			releasedReviewer = released
		}

		if role == "coder" || role == "both" {
			released, err := releaseOneClaim(state, task, coderRelease, force, agentID, reason, now)
			if err != nil {
				return err
			}
			releasedCoder = released
		}

		if !releasedReviewer && !releasedCoder {
			return fmt.Errorf("no claims to release for task %s", taskID)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to release claim: %w", err)
	}

	return &ReleaseClaimResult{
		TaskID:           taskID,
		Role:             role,
		ReleasedReviewer: releasedReviewer,
		ReleasedCoder:    releasedCoder,
	}, nil
}
