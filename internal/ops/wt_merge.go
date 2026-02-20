package ops

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// Integration failure reason constants.
const (
	IntegrationReasonHEADMismatch  = "worktree HEAD mismatch"
	IntegrationReasonMergeConflict = "merge conflict"
	IntegrationReasonTestsFailed   = "integration tests failed"
)

// IntegrationFailedError indicates the merge or integration tests failed.
// State has been updated to INTEGRATION_FAILED appropriately.
type IntegrationFailedError struct {
	Reason        string
	TestOutput    string // non-empty when failure is from integration tests
	RollbackError error  // non-nil if rollback (ResetHard) also failed — integration branch may contain failing code
}

func (e *IntegrationFailedError) Error() string {
	if e.RollbackError != nil {
		return fmt.Sprintf("integration failed: %s (rollback also failed: %v)", e.Reason, e.RollbackError)
	}
	return fmt.Sprintf("integration failed: %s", e.Reason)
}

// MergeResult contains the outcome of a successful worktree merge.
type MergeResult struct {
	TaskID      string
	MergeCommit string
	FastForward bool
	TestsRan    bool
	TestOutput  string   // captured stdout+stderr from integration tests (if any)
	Warnings    []string // non-fatal warnings from cleanup/metrics
}

// appendUniqueAgentID adds an agent ID to failed_by if not already present
func appendUniqueAgentID(failedBy []string, agentID string) []string {
	if slices.Contains(failedBy, agentID) {
		return failedBy
	}
	return append(failedBy, agentID)
}

// markIntegrationFailed transitions a task to INTEGRATION_FAILED under lock.
// Re-validates the task is still APPROVED to prevent concurrent transitions.
// If mergeCommit is non-empty, it's recorded on both the task and the history entry.
func markIntegrationFailed(bb *db.Blackboard, taskID, agentID, reason, mergeCommit string) error {
	return bb.Modify(func(s *models.State) error {
		t := s.FindTask(taskID)
		if t == nil {
			return fmt.Errorf("task not found: %s", taskID)
		}
		if t.Status != models.TaskStatusApproved {
			return fmt.Errorf("task %s status changed concurrently (now %s)", taskID, t.Status)
		}
		if err := t.Transition(models.TaskStatusIntegrationFailed); err != nil {
			return err
		}
		t.FailedBy = appendUniqueAgentID(t.FailedBy, agentID)

		entry := models.TaskHistoryEntry{
			Time:   time.Now(),
			Event:  "integration_failed",
			Agent:  &agentID,
			Reason: &reason,
		}
		if mergeCommit != "" {
			t.MergeCommit = &mergeCommit
			entry.Commit = &mergeCommit
		}
		t.History = append(t.History, entry)

		return nil
	})
}

// MergeWorktree merges an approved task into the integration branch.
// This is the final step in the task lifecycle, integrating completed work.
// Returns IntegrationFailedError if merge conflicts or integration tests fail.
//
// No terminal I/O — integration test output is captured and returned in the result or error.
func MergeWorktree(projectRoot, taskID, agentID string) (*MergeResult, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID is required")
	}
	if agentID == "" {
		return nil, fmt.Errorf("agent ID is required")
	}

	// Setup paths
	statePath := paths.New(projectRoot).StatePath()

	// Read state
	bb := db.New(statePath)
	state, task, err := readTaskState(bb, taskID)
	if err != nil {
		return nil, err
	}

	if task.Status != models.TaskStatusApproved {
		return nil, fmt.Errorf("task must be APPROVED to merge (current status: %s)", task.Status)
	}

	if task.Worktree == nil {
		return nil, fmt.Errorf("task has no worktree")
	}

	if task.ReviewCommit == nil {
		return nil, fmt.Errorf("task has no review_commit")
	}

	// Initialize git wrapper
	gitWrapper := git.New(projectRoot)

	// Get worktree HEAD and verify it matches review_commit
	wtHEAD, err := gitWrapper.GetWorktreeHEAD(taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree HEAD: %w", err)
	}

	// Normalize review_commit to full SHA and compare exact match.
	// This mirrors shell parity: resolve refs/short SHAs first, then compare canonical SHAs.
	reviewCommit := *task.ReviewCommit
	expectedCommit, err := gitWrapper.GetCommitSHA(reviewCommit)
	if err != nil {
		return nil, fmt.Errorf("review_commit (%s) not found in repository: %w", reviewCommit, err)
	}
	if wtHEAD != expectedCommit {
		// HEAD mismatch indicates state corruption — stops retry loops, preserves worktree
		shortWtHEAD := wtHEAD
		if len(wtHEAD) > 7 {
			shortWtHEAD = wtHEAD[:7]
		}
		shortReviewCommit := expectedCommit
		if len(expectedCommit) > 7 {
			shortReviewCommit = expectedCommit[:7]
		}
		reason := fmt.Sprintf("worktree HEAD (%s) does not match approved commit (%s)", shortWtHEAD, shortReviewCommit)
		if err := markIntegrationFailed(bb, taskID, agentID, reason, ""); err != nil {
			return nil, fmt.Errorf("failed to update state to INTEGRATION_FAILED: %w", err)
		}

		return nil, &IntegrationFailedError{Reason: IntegrationReasonHEADMismatch}
	}

	// Get integration branch
	integrationBranch := state.Config.IntegrationBranch
	if integrationBranch == "" {
		integrationBranch = "main"
	}

	// Checkout integration branch
	if err := gitWrapper.CheckoutBranch(integrationBranch); err != nil {
		return nil, fmt.Errorf("failed to checkout integration branch: %w", err)
	}

	// Capture pre-merge HEAD for rollback if integration tests fail
	preMergeHEAD, err := gitWrapper.GetCommitSHA("HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to get pre-merge HEAD: %w", err)
	}

	// Attempt merge
	taskBranch := "task/" + taskID
	fastForward, mergeCommit, err := gitWrapper.MergeBranch(taskBranch)

	if err != nil {
		_ = gitWrapper.AbortMerge()

		if updateErr := markIntegrationFailed(bb, taskID, agentID, "merge conflict", ""); updateErr != nil {
			return nil, fmt.Errorf("failed to update state to INTEGRATION_FAILED: %w", updateErr)
		}

		return nil, &IntegrationFailedError{Reason: IntegrationReasonMergeConflict}
	}

	// Run integration tests if they exist
	var testsRan bool
	var testOutput string
	integrationTestScript := filepath.Join(projectRoot, "scripts", "integration-test.sh")
	if _, statErr := os.Stat(integrationTestScript); statErr == nil {
		testsRan = true
		var combinedOutput bytes.Buffer
		cmd := exec.Command(integrationTestScript)
		cmd.Dir = projectRoot
		cmd.Stdout = &combinedOutput
		cmd.Stderr = &combinedOutput

		if runErr := cmd.Run(); runErr != nil {
			testOutput = combinedOutput.String()

			// Rollback: reset integration branch to pre-merge state
			rollbackErr := gitWrapper.ResetHard(preMergeHEAD)

			if updateErr := markIntegrationFailed(bb, taskID, agentID, "integration tests failed", mergeCommit); updateErr != nil {
				return nil, fmt.Errorf("failed to update state to INTEGRATION_FAILED: %w", updateErr)
			}

			return nil, &IntegrationFailedError{
				Reason:        IntegrationReasonTestsFailed,
				TestOutput:    testOutput,
				RollbackError: rollbackErr,
			}
		}

		testOutput = combinedOutput.String()
	}

	// Update state to MERGED (before worktree cleanup — if write fails,
	// worktree still exists for investigation; reverse order would lose the worktree
	// while state still says APPROVED)
	err = bb.Modify(func(s *models.State) error {
		t := s.FindTask(taskID)
		if t == nil {
			return fmt.Errorf("task not found: %s", taskID)
		}
		// Re-validate status under lock to prevent concurrent transition
		if t.Status != models.TaskStatusApproved {
			return fmt.Errorf("task %s status changed concurrently (now %s)", taskID, t.Status)
		}
		if err := t.Transition(models.TaskStatusMerged); err != nil {
			return err
		}
		t.Worktree = nil
		t.MergeCommit = &mergeCommit

		// Release the assigned agent
		if t.AssignedTo != nil {
			s.ReleaseAgent(*t.AssignedTo)
		}

		// Add history entry
		t.History = append(t.History, models.TaskHistoryEntry{
			Time:   time.Now(),
			Event:  "merged",
			Agent:  &agentID,
			Commit: &mergeCommit,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update state to MERGED: %w", err)
	}

	// Cleanup: Remove worktree (after state commit — safe to lose worktree now)
	// Errors are non-fatal — state is already committed, collect as warnings
	var warnings []string
	if err := gitWrapper.RemoveWorktree(taskID); err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to remove worktree: %v", err))
	}
	if err := gitWrapper.DeleteBranch(taskBranch); err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to delete branch %s: %v", taskBranch, err))
	}

	// Update sprint metrics — non-fatal
	if _, err := UpdateSprintMetrics(projectRoot); err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to update sprint metrics: %v", err))
	}

	return &MergeResult{
		TaskID:      taskID,
		MergeCommit: mergeCommit,
		FastForward: fastForward,
		TestsRan:    testsRan,
		TestOutput:  testOutput,
		Warnings:    warnings,
	}, nil
}
