package statevalidate

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestValidate_RejectsWorktreePrefixInTaskSpecRef(t *testing.T) {
	state := testhelpers.CreateValidState()
	now := time.Now().UTC()
	state.Tasks = []models.Task{
		{
			ID:          "task-1",
			Description: "Test task",
			Status:      models.TaskStatusReady,
			Priority:    1,
			Created:     now,
			SpecRef:     ".worktrees/code-planning-1/specs/plans/auth.md",
			DoneWhen:    "Done",
			Scope:       "test",
			Iteration:   1,
		},
	}

	err := validateTaskInvariants(state, t.TempDir(), true, nil)
	if err == nil {
		t.Fatal("Expected error for worktree-prefixed spec_ref")
	}
	if !strings.Contains(err.Error(), "worktree prefix") {
		t.Errorf("Error = %q, want to contain 'worktree prefix'", err.Error())
	}
}

func TestValidate_RejectsWorktreePrefixInOutputSpecRef(t *testing.T) {
	state := testhelpers.CreateValidState()
	now := time.Now().UTC()
	state.Tasks = []models.Task{
		{
			ID:           "task-1",
			Description:  "Plan task",
			Status:       models.TaskStatusCodingPlanApproved,
			Priority:     1,
			Created:      now,
			SpecRef:      "specs/plans/auth.md",
			DoneWhen:     "Plan approved",
			Scope:        "auth",
			Iteration:    1,
			ReviewCommit: testhelpers.StringPtr("abc123"),
			Output: []models.OutputEntry{
				{
					Desc:     "Implement login",
					DoneWhen: "POST /login works",
					Scope:    "auth",
					SpecRef:  ".worktrees/code-planning-1/specs/plans/auth.md#login",
				},
			},
		},
	}

	err := validateTaskInvariants(state, t.TempDir(), true, nil)
	if err == nil {
		t.Fatal("Expected error for worktree-prefixed output spec_ref")
	}
	if !strings.Contains(err.Error(), "worktree prefix") {
		t.Errorf("Error = %q, want to contain 'worktree prefix'", err.Error())
	}
}

func TestValidate_AcceptsRepoRelativeSpecRef(t *testing.T) {
	state := testhelpers.CreateValidState()
	now := time.Now().UTC()
	state.Tasks = []models.Task{
		{
			ID:           "task-1",
			Description:  "Plan task",
			Status:       models.TaskStatusCodingPlanApproved,
			Priority:     1,
			Created:      now,
			SpecRef:      "specs/plans/auth.md",
			DoneWhen:     "Plan approved",
			Scope:        "auth",
			Iteration:    1,
			ReviewCommit: testhelpers.StringPtr("abc123"),
			Output: []models.OutputEntry{
				{
					Desc:     "Implement login",
					DoneWhen: "POST /login works",
					Scope:    "auth",
					SpecRef:  "specs/plans/auth.md#login",
				},
			},
		},
	}

	err := validateTaskInvariants(state, t.TempDir(), true, nil)
	if err != nil {
		t.Fatalf("Unexpected error for repo-relative spec_ref: %v", err)
	}
}
