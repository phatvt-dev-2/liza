package git

import (
	"fmt"
	"strconv"
	"strings"
)

// CalculateDrift returns the number of commits between baseCommit and targetBranch
// This indicates how far the integration branch has moved since the task started
func (g *Git) CalculateDrift(baseCommit, targetBranch string) (int, error) {
	// git rev-list --count base..target
	output, err := g.exec("rev-list", "--count", baseCommit+".."+targetBranch)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate drift: %w", err)
	}

	count, err := strconv.Atoi(output)
	if err != nil {
		return 0, fmt.Errorf("failed to parse commit count: %w", err)
	}

	return count, nil
}

// IsAncestor checks if commitA is an ancestor of commitB
func (g *Git) IsAncestor(commitA, commitB string) (bool, error) {
	_, err := g.exec("merge-base", "--is-ancestor", commitA, commitB)
	if err != nil {
		// Check if error is "not an ancestor" (exit code 1)
		if strings.Contains(err.Error(), "exit status 1") {
			return false, nil
		}
		return false, fmt.Errorf("merge-base failed: %w", err)
	}
	return true, nil
}

// GetWorktreeHEAD returns the commit SHA of the worktree's HEAD
func (g *Git) GetWorktreeHEAD(taskID string) (string, error) {
	wtPath := g.GetWorktreePath(taskID)
	return g.execInDir(wtPath, "rev-parse", "HEAD")
}

// GetWorktreeBranch returns the current branch name in a worktree
func (g *Git) GetWorktreeBranch(wtPath string) (string, error) {
	branch, err := g.execInDir(wtPath, "branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("failed to get worktree branch: %w", err)
	}
	return branch, nil
}
