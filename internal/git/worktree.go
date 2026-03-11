package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/liza-mas/liza/internal/paths"
)

// CreateWorktree creates a worktree for the given task from the specified branch.
// Returns the base commit (full SHA) for drift tracking.
func (g *Git) CreateWorktree(taskID, fromBranch string) (string, error) {
	if err := paths.ValidateTaskID(taskID); err != nil {
		return "", fmt.Errorf("invalid task ID: %w", err)
	}
	worktreeRel := filepath.Join(paths.WorktreesDirName, taskID)
	worktreePath := filepath.Join(g.projectRoot, worktreeRel)
	branchName := paths.TaskBranchPrefix + taskID

	// Check if worktree already exists
	if _, err := os.Stat(worktreePath); err == nil {
		return "", fmt.Errorf("worktree already exists: %s", worktreePath)
	}

	// Ensure .worktrees directory exists
	worktreesDir := filepath.Join(g.projectRoot, paths.WorktreesDirName)
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .worktrees directory: %w", err)
	}

	// Get base commit before creating worktree
	baseCommit, err := g.GetCommitSHA(fromBranch)
	if err != nil {
		return "", fmt.Errorf("failed to get base commit: %w", err)
	}

	// Create worktree with new branch
	_, err = g.exec("worktree", "add", worktreePath, fromBranch, "-b", branchName)
	if err != nil {
		return "", fmt.Errorf("failed to create worktree: %w", err)
	}

	return baseCommit, nil
}

// AttachWorktree creates a worktree for an already-existing branch.
// Unlike CreateWorktree, this does not create a new branch with -b.
func (g *Git) AttachWorktree(taskID, existingBranch string) error {
	if err := paths.ValidateTaskID(taskID); err != nil {
		return fmt.Errorf("invalid task ID: %w", err)
	}
	worktreePath := filepath.Join(g.projectRoot, paths.WorktreesDirName, taskID)

	if _, err := os.Stat(worktreePath); err == nil {
		return fmt.Errorf("worktree already exists: %s", worktreePath)
	}

	worktreesDir := filepath.Join(g.projectRoot, paths.WorktreesDirName)
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return fmt.Errorf("failed to create .worktrees directory: %w", err)
	}

	_, err := g.exec("worktree", "add", worktreePath, existingBranch)
	if err != nil {
		return fmt.Errorf("failed to attach worktree: %w", err)
	}

	return nil
}

// CreateWorktreeFresh creates a worktree, deleting any existing one first
// This is used for task reassignment scenarios
func (g *Git) CreateWorktreeFresh(taskID, fromBranch string) (string, error) {
	// Remove existing worktree if it exists (ignore errors)
	_ = g.RemoveWorktree(taskID)

	// Create new worktree
	return g.CreateWorktree(taskID, fromBranch)
}

// RemoveWorktree removes a worktree and its associated branch
func (g *Git) RemoveWorktree(taskID string) error {
	if err := paths.ValidateTaskID(taskID); err != nil {
		return fmt.Errorf("invalid task ID: %w", err)
	}
	worktreeRel := filepath.Join(paths.WorktreesDirName, taskID)
	worktreePath := filepath.Join(g.projectRoot, worktreeRel)
	branchName := paths.TaskBranchPrefix + taskID

	// Check if worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		// Worktree doesn't exist, clean up branch if it exists
		exists, err := g.BranchExists(branchName)
		if err != nil {
			return err
		}
		if exists {
			return g.DeleteBranch(branchName)
		}
		return nil
	}

	// Remove worktree (force to handle any uncommitted changes)
	_, err := g.exec("worktree", "remove", "--force", worktreePath)
	if err != nil {
		// If git worktree remove fails, try manual cleanup
		if err := os.RemoveAll(worktreePath); err != nil {
			return fmt.Errorf("failed to remove worktree directory: %w", err)
		}
		// Clean up git's internal worktree tracking (.git/worktrees/<name>)
		// to prevent future "worktree add" from failing on dangling entries
		_, _ = g.exec("worktree", "prune")
	}

	// Delete the branch (may not exist, so ignore errors)
	_ = g.DeleteBranch(branchName)

	return nil
}

// ListWorktrees returns a list of all worktrees
func (g *Git) ListWorktrees() ([]Worktree, error) {
	// git worktree list --porcelain gives us machine-readable output
	output, err := g.exec("worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var worktrees []Worktree
	var current Worktree

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			// Empty line separates worktree entries
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = Worktree{}
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current.Path = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "HEAD ") {
			current.Commit = strings.TrimPrefix(line, "HEAD ")
		} else if strings.HasPrefix(line, "branch ") {
			branchRef := strings.TrimPrefix(line, "branch ")
			// Extract branch name from refs/heads/...
			current.Branch = strings.TrimPrefix(branchRef, "refs/heads/")
		}
	}

	// Add the last entry if present
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

// GetWorktreePath returns the absolute path for a task's worktree
// Callers must validate taskID before calling this — see paths.ValidateTaskID.
func (g *Git) GetWorktreePath(taskID string) string {
	return filepath.Join(g.projectRoot, paths.WorktreesDirName, taskID)
}

// GetWorktreeRelPath returns the relative path for a task's worktree
func (g *Git) GetWorktreeRelPath(taskID string) string {
	return filepath.Join(paths.WorktreesDirName, taskID)
}
