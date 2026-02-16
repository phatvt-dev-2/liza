package commands

import (
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

// IntegrationFailedError indicates the merge or integration tests failed.
// State has been updated to INTEGRATION_FAILED appropriately.
type IntegrationFailedError struct {
	Reason string
}

func (e *IntegrationFailedError) Error() string {
	return fmt.Sprintf("integration failed: %s", e.Reason)
}

// appendUniqueAgentID adds an agent ID to failed_by if not already present
func appendUniqueAgentID(failedBy []string, agentID string) []string {
	if slices.Contains(failedBy, agentID) {
		return failedBy
	}
	return append(failedBy, agentID)
}

// WtMergeCommand merges an approved task into the integration branch
// This is the final step in the task lifecycle, integrating completed work
// Returns IntegrationFailedError if merge conflicts or integration tests fail
func WtMergeCommand(projectRoot, taskID, agentID string) error {
	// Validate input
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}
	if agentID == "" {
		return fmt.Errorf("agent ID is required")
	}

	// Setup paths
	statePath := paths.New(projectRoot).StatePath()

	// Read state
	bb := db.New(statePath)
	state, err := bb.Read()
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	// Find task
	var task *models.Task
	for i := range state.Tasks {
		if state.Tasks[i].ID == taskID {
			task = &state.Tasks[i]
			break
		}
	}

	if task == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// Validate task status
	if task.Status != models.TaskStatusApproved {
		return fmt.Errorf("task must be APPROVED to merge (current status: %s)", task.Status)
	}

	// Validate worktree exists
	if task.Worktree == nil {
		return fmt.Errorf("task has no worktree")
	}

	// Validate review_commit is set
	if task.ReviewCommit == nil {
		return fmt.Errorf("task has no review_commit")
	}

	// Initialize git wrapper
	gitWrapper := git.New(projectRoot)

	// Get worktree HEAD and verify it matches review_commit
	wtHEAD, err := gitWrapper.GetWorktreeHEAD(taskID)
	if err != nil {
		return fmt.Errorf("failed to get worktree HEAD: %w", err)
	}

	// Normalize review_commit to full SHA and compare exact match.
	// This mirrors shell parity: resolve refs/short SHAs first, then compare canonical SHAs.
	reviewCommit := *task.ReviewCommit
	expectedCommit, err := gitWrapper.GetCommitSHA(reviewCommit)
	if err != nil {
		return fmt.Errorf("review_commit (%s) not found in repository: %w", reviewCommit, err)
	}
	if wtHEAD != expectedCommit {
		// HEAD mismatch indicates state corruption - treat as integration failure
		// This stops retry loops and preserves worktree for investigation
		fmt.Fprintf(os.Stderr, "⚠️  Worktree HEAD does not match approved commit\n")
		fmt.Fprintf(os.Stderr, "Task %s marked as INTEGRATION_FAILED\n", taskID)
		fmt.Fprintf(os.Stderr, "Worktree preserved for investigation\n")
		fmt.Fprintf(os.Stderr, "  Expected: %s\n", expectedCommit)
		fmt.Fprintf(os.Stderr, "  Actual:   %s\n", wtHEAD)

		// Update state to INTEGRATION_FAILED
		updateErr := bb.Modify(func(s *models.State) error {
			for i := range s.Tasks {
				if s.Tasks[i].ID == taskID {
					// Re-validate status under lock to prevent concurrent transition
					if s.Tasks[i].Status != models.TaskStatusApproved {
						return fmt.Errorf("task %s status changed concurrently (now %s)", taskID, s.Tasks[i].Status)
					}
					s.Tasks[i].Status = models.TaskStatusIntegrationFailed
					s.Tasks[i].FailedBy = appendUniqueAgentID(s.Tasks[i].FailedBy, agentID)

					// Add history entry with specific reason
					shortWtHEAD := wtHEAD
					if len(wtHEAD) > 7 {
						shortWtHEAD = wtHEAD[:7]
					}
					shortReviewCommit := expectedCommit
					if len(expectedCommit) > 7 {
						shortReviewCommit = expectedCommit[:7]
					}
					reason := fmt.Sprintf("worktree HEAD (%s) does not match approved commit (%s)", shortWtHEAD, shortReviewCommit)
					s.Tasks[i].History = append(s.Tasks[i].History, models.TaskHistoryEntry{
						Time:   time.Now(),
						Event:  "integration_failed",
						Agent:  &agentID,
						Reason: &reason,
					})

					return nil
				}
			}
			return fmt.Errorf("task not found: %s", taskID)
		})

		if updateErr != nil {
			return fmt.Errorf("failed to update state to INTEGRATION_FAILED: %w", updateErr)
		}

		return &IntegrationFailedError{Reason: "worktree HEAD mismatch"}
	}

	// Get integration branch
	integrationBranch := state.Config.IntegrationBranch
	if integrationBranch == "" {
		integrationBranch = "main"
	}

	// Checkout integration branch
	if err := gitWrapper.CheckoutBranch(integrationBranch); err != nil {
		return fmt.Errorf("failed to checkout integration branch: %w", err)
	}

	// Capture pre-merge HEAD for rollback if integration tests fail
	preMergeHEAD, err := gitWrapper.GetCommitSHA("HEAD")
	if err != nil {
		return fmt.Errorf("failed to get pre-merge HEAD: %w", err)
	}

	// Attempt merge
	taskBranch := "task/" + taskID
	fastForward, mergeCommit, err := gitWrapper.MergeBranch(taskBranch)

	if err != nil {
		// Merge conflict - mark as INTEGRATION_FAILED
		fmt.Fprintf(os.Stderr, "⚠️  Merge conflict detected\n")
		fmt.Fprintf(os.Stderr, "Task %s marked as INTEGRATION_FAILED\n", taskID)
		fmt.Fprintf(os.Stderr, "Worktree preserved for conflict resolution\n")

		// Abort the merge
		_ = gitWrapper.AbortMerge()

		// Update state to INTEGRATION_FAILED
		updateErr := bb.Modify(func(s *models.State) error {
			for i := range s.Tasks {
				if s.Tasks[i].ID == taskID {
					// Re-validate status under lock to prevent concurrent transition
					if s.Tasks[i].Status != models.TaskStatusApproved {
						return fmt.Errorf("task %s status changed concurrently (now %s)", taskID, s.Tasks[i].Status)
					}
					s.Tasks[i].Status = models.TaskStatusIntegrationFailed
					s.Tasks[i].FailedBy = appendUniqueAgentID(s.Tasks[i].FailedBy, agentID)

					// Add history entry
					reason := "merge conflict"
					s.Tasks[i].History = append(s.Tasks[i].History, models.TaskHistoryEntry{
						Time:   time.Now(),
						Event:  "integration_failed",
						Agent:  &agentID,
						Reason: &reason,
					})

					return nil
				}
			}
			return fmt.Errorf("task not found: %s", taskID)
		})

		if updateErr != nil {
			return fmt.Errorf("failed to update state to INTEGRATION_FAILED: %w", updateErr)
		}

		return &IntegrationFailedError{Reason: "merge conflict"}
	}

	// Merge successful
	if fastForward {
		fmt.Printf("✓ Fast-forward merge successful\n")
	} else {
		fmt.Printf("✓ Merge commit created\n")
	}

	// Run integration tests if they exist
	integrationTestScript := filepath.Join(projectRoot, "scripts", "integration-test.sh")
	if _, err := os.Stat(integrationTestScript); err == nil {
		fmt.Println("Running integration tests...")
		cmd := exec.Command(integrationTestScript)
		cmd.Dir = projectRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Integration tests failed\n")

			// Rollback: reset integration branch to pre-merge state
			if resetErr := gitWrapper.ResetHard(preMergeHEAD); resetErr != nil {
				fmt.Fprintf(os.Stderr, "⚠️  Failed to rollback merge: %v\n", resetErr)
				shortMerge := mergeCommit
				if len(shortMerge) > 7 {
					shortMerge = shortMerge[:7]
				}
				fmt.Fprintf(os.Stderr, "  Integration branch may contain failing code at commit %s\n", shortMerge)
			} else {
				shortPre := preMergeHEAD
				if len(shortPre) > 7 {
					shortPre = shortPre[:7]
				}
				fmt.Fprintf(os.Stderr, "✓ Merge rolled back to %s\n", shortPre)
			}

			fmt.Fprintf(os.Stderr, "Task %s marked as INTEGRATION_FAILED\n", taskID)

			// Update state to INTEGRATION_FAILED
			updateErr := bb.Modify(func(s *models.State) error {
				for i := range s.Tasks {
					if s.Tasks[i].ID == taskID {
						// Re-validate status under lock to prevent concurrent transition
						if s.Tasks[i].Status != models.TaskStatusApproved {
							return fmt.Errorf("task %s status changed concurrently (now %s)", taskID, s.Tasks[i].Status)
						}
						s.Tasks[i].Status = models.TaskStatusIntegrationFailed
						s.Tasks[i].FailedBy = appendUniqueAgentID(s.Tasks[i].FailedBy, agentID)
						s.Tasks[i].MergeCommit = &mergeCommit

						// Add history entry
						failReason := "integration tests failed"
						s.Tasks[i].History = append(s.Tasks[i].History, models.TaskHistoryEntry{
							Time:   time.Now(),
							Event:  "integration_failed",
							Agent:  &agentID,
							Reason: &failReason,
							Commit: &mergeCommit,
						})

						return nil
					}
				}
				return fmt.Errorf("task not found: %s", taskID)
			})

			if updateErr != nil {
				return fmt.Errorf("failed to update state to INTEGRATION_FAILED: %w", updateErr)
			}

			return &IntegrationFailedError{Reason: "integration tests failed"}
		}

		fmt.Println("✓ Integration tests passed")
	}

	// Update state to MERGED (before worktree cleanup — if write fails,
	// worktree still exists for investigation; reverse order would lose the worktree
	// while state still says APPROVED)
	err = bb.Modify(func(s *models.State) error {
		for i := range s.Tasks {
			if s.Tasks[i].ID == taskID {
				// Re-validate status under lock to prevent concurrent transition
				if s.Tasks[i].Status != models.TaskStatusApproved {
					return fmt.Errorf("task %s status changed concurrently (now %s)", taskID, s.Tasks[i].Status)
				}
				s.Tasks[i].Status = models.TaskStatusMerged
				s.Tasks[i].Worktree = nil
				s.Tasks[i].MergeCommit = &mergeCommit

				// Add history entry
				s.Tasks[i].History = append(s.Tasks[i].History, models.TaskHistoryEntry{
					Time:   time.Now(),
					Event:  "merged",
					Agent:  &agentID,
					Commit: &mergeCommit,
				})

				return nil
			}
		}
		return fmt.Errorf("task not found: %s", taskID)
	})

	if err != nil {
		return fmt.Errorf("failed to update state to MERGED: %w", err)
	}

	// Cleanup: Remove worktree (after state commit — safe to lose worktree now)
	if err := gitWrapper.RemoveWorktree(taskID); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove worktree: %v\n", err)
	}

	// Delete branch
	_ = gitWrapper.DeleteBranch(taskBranch)

	// Update sprint metrics
	if err := UpdateSprintMetricsCommand(projectRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update sprint metrics: %v\n", err)
	}

	// Guard against short merge commit strings
	shortCommit := mergeCommit
	if len(shortCommit) > 7 {
		shortCommit = shortCommit[:7]
	}
	fmt.Printf("✓ Task %s merged successfully\n", taskID)
	fmt.Printf("  Merge commit: %s\n", shortCommit)

	return nil
}
