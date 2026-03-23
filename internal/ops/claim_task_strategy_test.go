package ops

import (
	"testing"

	"github.com/liza-mas/liza/internal/models"
)

func TestFreshClaimStrategy_MutateTask_SetsAttemptOnFirstClaim(t *testing.T) {
	task := &models.Task{Attempt: 0}
	ctx := &claimContext{
		worktreeRel: ".worktrees/test-task",
		baseCommit:  "abc123",
	}
	strategy := freshClaimStrategy{}
	strategy.mutateTask(task, ctx)

	if task.Attempt != 1 {
		t.Errorf("mutateTask() set Attempt = %d, want 1 for first claim", task.Attempt)
	}
}

func TestFreshClaimStrategy_MutateTask_PreservesNonZeroAttempt(t *testing.T) {
	task := &models.Task{Attempt: 2}
	ctx := &claimContext{
		worktreeRel: ".worktrees/test-task",
		baseCommit:  "abc123",
	}
	strategy := freshClaimStrategy{}
	strategy.mutateTask(task, ctx)

	if task.Attempt != 2 {
		t.Errorf("mutateTask() changed Attempt to %d, want 2 (preserved)", task.Attempt)
	}
}
