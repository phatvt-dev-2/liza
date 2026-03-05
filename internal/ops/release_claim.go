package ops

import (
	"fmt"
	"log"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/pipeline"
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
	activeStatus:   models.TaskStatusImplementing,
	releasedStatus: models.TaskStatusReady,
	eventName:      "coder_claim_released",
	clearFn: func(t *models.Task) {
		t.AssignedTo = nil
		t.LeaseExpires = nil
		t.Worktree = nil
		t.BaseCommit = nil
		t.Iteration = 0
	},
	missingLeaseMsg: "lease expires missing for task %s, use --force to clear",
	activeLeaseMsg:  "coder lease still valid until %s, use --force to clear",
}

// ResolveReleaseStatuses returns the active/released status pairs for doer and
// reviewer claims, resolving from the pipeline when the task has a RolePair.
// Falls back to legacy defaults (IMPLEMENTING→READY, REVIEWING→READY_FOR_REVIEW).
func ResolveReleaseStatuses(task *models.Task, resolver *pipeline.Resolver) (doerActive, doerReleased, reviewerActive, reviewerReleased models.TaskStatus) {
	doerActive = models.TaskStatusImplementing
	doerReleased = models.TaskStatusReady
	reviewerActive = models.TaskStatusReviewing
	reviewerReleased = models.TaskStatusReadyForReview
	if resolver == nil || task.RolePair == "" {
		return
	}
	initial, initialErr := resolver.InitialStatus(task.RolePair)
	executing, executingErr := resolver.ExecutingStatus(task.RolePair)
	if initialErr == nil && executingErr == nil {
		doerActive = executing
		doerReleased = initial
	}
	submitted, submittedErr := resolver.SubmittedStatus(task.RolePair)
	reviewing, reviewingErr := resolver.ReviewingStatus(task.RolePair)
	if submittedErr == nil && reviewingErr == nil {
		reviewerActive = reviewing
		reviewerReleased = submitted
	}
	return
}

// resolveClaimReleaseStatuses returns coder and reviewer claimRelease configs with
// pipeline-resolved active/released statuses when the task has a RolePair and a
// resolver is available. Falls back to legacy defaults otherwise.
func resolveClaimReleaseStatuses(task *models.Task, resolver *pipeline.Resolver) (coder claimRelease, reviewer claimRelease) {
	coder = coderRelease
	reviewer = reviewerRelease
	coder.activeStatus, coder.releasedStatus, reviewer.activeStatus, reviewer.releasedStatus = ResolveReleaseStatuses(task, resolver)
	return coder, reviewer
}

// releaseOneClaim executes the 9-step release sequence for a single role's claim.
// pipelineTransitions, if non-nil, overrides the default transition map.
// Returns true if a claim was released.
func releaseOneClaim(state *models.State, task *models.Task, cfg claimRelease, pipelineTransitions map[models.TaskStatus][]models.TaskStatus, force bool, agentID, reason string, now time.Time) (bool, error) {
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
		if pipelineTransitions != nil {
			if err := task.TransitionWith(cfg.releasedStatus, pipelineTransitions); err != nil {
				return false, err
			}
		} else {
			if err := task.Transition(cfg.releasedStatus); err != nil {
				return false, err
			}
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

	if role != "code-reviewer" && role != "coder" && role != "both" {
		return nil, fmt.Errorf("role must be code-reviewer, coder, or both, got: %s", role)
	}

	if agentID == "" {
		agentID = "human"
	}

	if reason == "" {
		reason = "manual release"
	}

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())

	releasedReviewer := false
	releasedCoder := false

	now := time.Now().UTC()

	// Load pipeline resolver for pipeline-aware status resolution
	var pipelineTransitions map[models.TaskStatus][]models.TaskStatus
	resolver, cfg, resolverErr := loadResolver(projectRoot)
	if resolverErr != nil {
		return nil, fmt.Errorf("failed to load pipeline config: %w", resolverErr)
	}
	if resolver != nil && cfg != nil {
		pipelineTransitions = BuildPipelineTransitions(resolver, cfg)
	}

	err := bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}

		// Resolve pipeline-aware statuses for claim release
		effectiveCoderRelease, effectiveReviewerRelease := resolveClaimReleaseStatuses(task, resolver)

		if role == "code-reviewer" || role == "both" {
			released, err := releaseOneClaim(state, task, effectiveReviewerRelease, pipelineTransitions, force, agentID, reason, now)
			if err != nil {
				return err
			}
			releasedReviewer = released
		}

		if role == "coder" || role == "both" {
			released, err := releaseOneClaim(state, task, effectiveCoderRelease, pipelineTransitions, force, agentID, reason, now)
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

	// Clean up worktree and branch after successful coder release.
	// Errors are warnings — state is already correct.
	if releasedCoder {
		gitWrapper := git.New(lp.ProjectRoot())
		branchName := paths.TaskBranchPrefix + taskID
		if cleanupErr := gitWrapper.RemoveWorktree(taskID); cleanupErr != nil {
			log.Printf("WARNING: release-claim %s: failed to remove worktree: %v", taskID, cleanupErr)
		}
		if cleanupErr := gitWrapper.DeleteBranch(branchName); cleanupErr != nil {
			log.Printf("WARNING: release-claim %s: failed to delete branch %s: %v", taskID, branchName, cleanupErr)
		}
	}

	return &ReleaseClaimResult{
		TaskID:           taskID,
		Role:             role,
		ReleasedReviewer: releasedReviewer,
		ReleasedCoder:    releasedCoder,
	}, nil
}
