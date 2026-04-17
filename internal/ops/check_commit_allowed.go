package ops

import (
	"fmt"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// CheckCommitAllowedResult reports whether a commit is permitted for a task
// in its current state. Reason is populated when Allowed is false to give the
// user a clear explanation in the pre-commit hook's stderr.
type CheckCommitAllowedResult struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
	Status  string `json:"status,omitempty"`
}

// CheckCommitAllowed evaluates whether a commit should be allowed in the task
// worktree for taskID. The policy is fail-safe: on any evaluation error (state
// unreadable, task not found, resolver failure, role not resolvable) the commit
// is allowed. The hook is a guard, not a lock — the authoritative check is
// submit-verdict's review_commit == HEAD comparison.
//
// A commit is allowed when the task is in one of:
//   - The executing status for its role pair (coder is actively working)
//   - The rejected status for its role pair (coder addressing feedback)
//   - BLOCKED (agents may need to commit diagnostic files before marking blocked)
//
// Everything else (READY_FOR_REVIEW, REVIEWING, APPROVED, terminal states) rejects.
func CheckCommitAllowed(projectRoot, taskID string) *CheckCommitAllowedResult {
	if taskID == "" {
		return &CheckCommitAllowedResult{Allowed: true, Reason: "empty task ID; fail-safe allow"}
	}

	lp := paths.New(projectRoot)
	bb := db.For(lp.StatePath())
	state, err := bb.Read()
	if err != nil {
		return &CheckCommitAllowedResult{Allowed: true, Reason: "state unreadable; fail-safe allow"}
	}

	task := state.FindTask(taskID)
	if task == nil {
		return &CheckCommitAllowedResult{Allowed: true, Reason: "task not found; fail-safe allow"}
	}

	if task.Status == models.TaskStatusBlocked {
		return &CheckCommitAllowedResult{Allowed: true, Status: string(task.Status)}
	}

	pr, err := LoadResolverForModels(projectRoot)
	if err != nil || pr == nil {
		return &CheckCommitAllowedResult{Allowed: true, Reason: "pipeline resolver unavailable; fail-safe allow"}
	}

	// Task lacks a role_pair → can't resolve executing/rejected statuses.
	// Fail-safe allow: this is the guard-not-lock contract.
	if task.RolePair == "" {
		return &CheckCommitAllowedResult{Allowed: true, Reason: "task has no role_pair; fail-safe allow"}
	}

	if models.IsExecutingStatus(task, pr) {
		return &CheckCommitAllowedResult{Allowed: true, Status: string(task.Status)}
	}

	rejected, err := pr.RejectedStatus(task.RolePair)
	if err != nil {
		// Pipeline config doesn't know this role_pair (drift between state.yaml
		// and pipeline.yaml). Fail-safe allow rather than turning config drift
		// into a commit-blocking failure.
		return &CheckCommitAllowedResult{Allowed: true, Reason: "rejected-status lookup failed; fail-safe allow"}
	}
	if task.Status == rejected {
		return &CheckCommitAllowedResult{Allowed: true, Status: string(task.Status)}
	}

	return &CheckCommitAllowedResult{
		Allowed: false,
		Status:  string(task.Status),
		Reason: fmt.Sprintf(
			"task %s is in status %s — commits are only permitted while the task is executing, rejected, or blocked",
			taskID, task.Status,
		),
	}
}
