package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// SubmitForReviewCommand atomically marks a task as READY_FOR_REVIEW.
// Used by coder agents to submit completed work for review.
// This command includes automatic rebase onto the latest integration branch
// to catch conflicts early before the review phase.
func SubmitForReviewCommand(projectRoot, taskID, commitSHA, agentID string) error {
	// Validate input
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}
	if commitSHA == "" {
		return fmt.Errorf("commit SHA is required")
	}
	if agentID == "" {
		return fmt.Errorf("LIZA_AGENT_ID is required")
	}

	// Setup paths
	lp := paths.New(projectRoot)

	// Get database instance
	bb := db.New(lp.StatePath())

	// Phase 1: Read state to get config and validate preconditions
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

	// Validate task is in CLAIMED status
	if task.Status != models.TaskStatusClaimed {
		return fmt.Errorf("task %s is not CLAIMED (current status: %s)", taskID, task.Status)
	}

	// Validate task is assigned to this agent
	if task.AssignedTo == nil || *task.AssignedTo != agentID {
		currentAgent := "none"
		if task.AssignedTo != nil {
			currentAgent = *task.AssignedTo
		}
		return fmt.Errorf("task %s is not assigned to agent %s (currently assigned to: %s)", taskID, agentID, currentAgent)
	}

	// Validate worktree exists
	if task.Worktree == nil {
		return fmt.Errorf("task %s has no worktree", taskID)
	}

	// Phase 2: Execute git operations outside the lock
	g := git.New(projectRoot)
	wtPath := g.GetWorktreePath(taskID)

	// Check worktree directory exists
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		return fmt.Errorf("worktree directory does not exist: %s", wtPath)
	}

	// Get the current branch in the worktree
	wtBranch, err := g.GetWorktreeBranch(wtPath)
	if err != nil {
		return fmt.Errorf("failed to get worktree branch: %w", err)
	}

	// Verify worktree is on expected branch
	expectedBranch := "task/" + taskID
	if wtBranch != expectedBranch {
		if wtBranch == "" {
			return fmt.Errorf("worktree is in detached HEAD state (expected branch: %s)", expectedBranch)
		}
		return fmt.Errorf("worktree is on branch %s (expected: %s)", wtBranch, expectedBranch)
	}

	// Fetch latest integration branch from project root into worktree
	integrationBranch := state.Config.IntegrationBranch
	if err := g.FetchFromLocal(wtPath, integrationBranch); err != nil {
		return fmt.Errorf("failed to fetch integration branch: %w", err)
	}

	// Attempt rebase onto FETCH_HEAD
	if err := g.RebaseOnto(wtPath, "FETCH_HEAD"); err != nil {
		// Abort the rebase to leave worktree in clean state
		_ = g.AbortRebase(wtPath)

		// Return descriptive error with instructions
		return fmt.Errorf(`failed to submit task for review: rebase conflict detected

Your task branch has conflicts with the latest integration branch.

Worktree location: %s

To resolve:
  1. cd %s
  2. git status (see conflicting files)
  3. Edit files to resolve conflict markers
  4. git add <resolved-files>
  5. git rebase --continue
  6. Return to project root and retry: liza submit-for-review %s

Alternatively, abort the rebase and ask for help:
  git rebase --abort`, wtPath, wtPath, taskID)
	}

	// Get new HEAD SHA after successful rebase
	postRebaseCommit, err := g.GetWorktreeHEAD(taskID)
	if err != nil {
		return fmt.Errorf("failed to get post-rebase commit SHA: %w", err)
	}

	// Phase 3: Atomic update with new commit SHA
	now := time.Now().UTC()

	err = bb.Modify(func(state *models.State) error {
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

		// Validate task is in CLAIMED status
		if task.Status != models.TaskStatusClaimed {
			return fmt.Errorf("task %s is not CLAIMED (current status: %s)", taskID, task.Status)
		}

		// Validate task is assigned to this agent
		if task.AssignedTo == nil || *task.AssignedTo != agentID {
			currentAgent := "none"
			if task.AssignedTo != nil {
				currentAgent = *task.AssignedTo
			}
			return fmt.Errorf("task %s is not assigned to agent %s (currently assigned to: %s)", taskID, agentID, currentAgent)
		}

		// Update task status and review_commit (use post-rebase commit)
		task.Status = models.TaskStatusReadyForReview
		task.ReviewCommit = &postRebaseCommit

		// Add history entry
		agentPtr := &agentID
		historyEntry := models.TaskHistoryEntry{
			Time:  now,
			Event: "submitted_for_review",
			Agent: agentPtr,
		}
		task.History = append(task.History, historyEntry)

		// Update agent status: coder transitions to WAITING after submission.
		// CurrentTask is preserved so dashboards can correlate waiting coders with their tasks.
		if agent, ok := state.Agents[agentID]; ok {
			agent.Status = models.AgentStatusWaiting
			agent.LeaseExpires = nil
			state.Agents[agentID] = agent
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to submit task for review: %w", err)
	}

	fmt.Printf("SUBMITTED FOR REVIEW: %s\n", taskID)
	fmt.Printf("  review_commit: %s\n", postRebaseCommit)
	fmt.Printf("  submitted_by: %s\n", agentID)

	return nil
}
