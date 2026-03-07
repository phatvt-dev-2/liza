package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// CreateWorktreeResult contains the outcome of creating a worktree.
type CreateWorktreeResult struct {
	TaskID         string
	WorktreeDir    string
	BaseCommit     string
	AlreadyExisted bool // true if worktree existed and fresh was false
	Warnings       []string
}

// CreateWorktree provisions a git worktree from the integration branch for an
// IMPLEMENTING task and records its base_commit. When fresh is true, deletes
// any existing worktree first (for reassignment). No terminal I/O.
func CreateWorktree(projectRoot, taskID string, fresh bool) (*CreateWorktreeResult, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID is required")
	}

	lp := paths.New(projectRoot)
	worktreeRel := filepath.Join(paths.WorktreesDirName, taskID)
	worktreeDir := filepath.Join(lp.ProjectRoot(), worktreeRel)

	bb := db.For(lp.StatePath())
	state, task, err := readTaskState(bb, taskID)
	if err != nil {
		return nil, err
	}

	if task.Status != models.TaskStatusImplementing {
		return nil, fmt.Errorf("task %s is not IMPLEMENTING (status: %s)", taskID, task.Status)
	}

	integrationBranch := state.Config.IntegrationBranch
	postCmd := state.Config.PostWorktreeCmd

	gitWrapper := git.New(lp.ProjectRoot())

	// Check if worktree already exists
	if _, err := os.Stat(worktreeDir); err == nil {
		if !fresh {
			result := &CreateWorktreeResult{
				TaskID:         taskID,
				WorktreeDir:    worktreeDir,
				AlreadyExisted: true,
			}
			// Run post-worktree command even on existing worktrees — idempotent, catches prior failures.
			if postCmd != nil {
				if err := RunPostWorktreeCmd(*postCmd, worktreeDir); err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("post-worktree-cmd: %v", err))
				}
			}
			return result, nil
		}
	}

	var baseCommit string
	if fresh {
		baseCommit, err = gitWrapper.CreateWorktreeFresh(taskID, integrationBranch)
	} else {
		baseCommit, err = gitWrapper.CreateWorktree(taskID, integrationBranch)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create worktree: %w", err)
	}

	err = bb.Modify(func(state *models.State) error {
		task := state.FindTask(taskID)
		if task == nil {
			return &errors.NotFoundError{Entity: "task", ID: taskID}
		}
		task.BaseCommit = &baseCommit
		return nil
	})

	if err != nil {
		_ = gitWrapper.RemoveWorktree(taskID)
		return nil, fmt.Errorf("failed to update state: %w", err)
	}

	result := &CreateWorktreeResult{
		TaskID:      taskID,
		WorktreeDir: worktreeDir,
		BaseCommit:  baseCommit,
	}

	// Run post-worktree command so the worktree is build/test-ready.
	// Non-fatal: agents can run the command manually.
	if postCmd != nil {
		if err := RunPostWorktreeCmd(*postCmd, worktreeDir); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("post-worktree-cmd: %v", err))
		}
	}

	return result, nil
}

// RunPostWorktreeCmd runs the configured post-worktree shell command in the given directory.
// It is idempotent and safe to call on both new and existing worktrees.
//
// Trust model: the command comes from state.yaml which lives inside .liza/ in
// the project root. Write access to state.yaml implies write access to the
// repo (same trust boundary as Makefile, .github/workflows/, package.json
// scripts). No additional confirmation gate is needed.
func RunPostWorktreeCmd(cmdStr, dir string) error {
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}
