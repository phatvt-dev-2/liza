package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestMutationCommandWiring(t *testing.T) {
	t.Run("claim-task wires positional args to handler", func(t *testing.T) {
		projectRoot, statePath := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-claim-alpha", models.TaskStatusReady, now),
			}
		})

		err := executeRootCommand(t, projectRoot, "claim-task", "task-claim-alpha", "coder-42")
		if err != nil {
			t.Fatalf("claim-task execute failed: %v", err)
		}

		state := readState(t, statePath)
		task := mustFindTask(t, state, "task-claim-alpha")
		if task.Status != models.TaskStatusImplementing {
			t.Fatalf("task status = %s, want %s", task.Status, models.TaskStatusImplementing)
		}
		if task.AssignedTo == nil || *task.AssignedTo != "coder-42" {
			t.Fatalf("task assigned_to = %v, want coder-42", task.AssignedTo)
		}

		agent, ok := state.Agents["coder-42"]
		if !ok {
			t.Fatalf("agent coder-42 not created")
		}
		if agent.CurrentTask == nil || *agent.CurrentTask != "task-claim-alpha" {
			t.Fatalf("agent current_task = %v, want task-claim-alpha", agent.CurrentTask)
		}
	})

	t.Run("submit-verdict uses --agent-id flag", func(t *testing.T) {
		projectRoot, statePath := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-review-flag", models.TaskStatusReviewing, now),
			}
		})

		err := executeRootCommand(t, projectRoot, "submit-verdict", "task-review-flag", "APPROVED", "--agent-id", "code-reviewer-9")
		if err != nil {
			t.Fatalf("submit-verdict execute failed: %v", err)
		}

		state := readState(t, statePath)
		task := mustFindTask(t, state, "task-review-flag")
		if task.Status != models.TaskStatusApproved {
			t.Fatalf("task status = %s, want %s", task.Status, models.TaskStatusApproved)
		}
		if task.ApprovedBy == nil || *task.ApprovedBy != "code-reviewer-9" {
			t.Fatalf("approved_by = %v, want code-reviewer-9", task.ApprovedBy)
		}
	})

	t.Run("submit-verdict falls back to LIZA_AGENT_ID and forwards reason", func(t *testing.T) {
		projectRoot, statePath := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-review-env", models.TaskStatusReviewing, now),
			}
		})

		t.Setenv("LIZA_AGENT_ID", "code-reviewer-8")
		err := executeRootCommand(t, projectRoot, "submit-verdict", "task-review-env", "REJECTED", "needs-work")
		if err != nil {
			t.Fatalf("submit-verdict execute failed: %v", err)
		}

		state := readState(t, statePath)
		task := mustFindTask(t, state, "task-review-env")
		if task.Status != models.TaskStatusRejected {
			t.Fatalf("task status = %s, want %s", task.Status, models.TaskStatusRejected)
		}
		if task.RejectionReason == nil || *task.RejectionReason != "needs-work" {
			t.Fatalf("rejection_reason = %v, want needs-work", task.RejectionReason)
		}
		if len(task.History) == 0 {
			t.Fatalf("expected history entry for verdict")
		}
		last := task.History[len(task.History)-1]
		if last.Event != "rejected" {
			t.Fatalf("history event = %s, want rejected", last.Event)
		}
		if last.Agent == nil || *last.Agent != "code-reviewer-8" {
			t.Fatalf("history agent = %v, want code-reviewer-8", last.Agent)
		}
		if last.Reason == nil || *last.Reason != "needs-work" {
			t.Fatalf("history reason = %v, want needs-work", last.Reason)
		}
	})

	t.Run("wt-merge routes parsed args to merge handler", func(t *testing.T) {
		projectRoot, _ := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-wt-merge", models.TaskStatusReady, now),
			}
		})

		err := executeRootCommand(t, projectRoot, "wt-merge", "task-wt-merge", "--agent-id", "code-reviewer-3")
		if err == nil {
			t.Fatalf("expected wt-merge error, got nil")
		}
		if !strings.Contains(err.Error(), "task must be APPROVED or CODING_PLAN_APPROVED to merge (current status: READY)") {
			t.Fatalf("unexpected wt-merge error: %v", err)
		}
	})

	t.Run("release-claim uses --changed-by over env fallback", func(t *testing.T) {
		projectRoot, statePath := setupMutationTestProject(t, func(state *models.State) {
			now := time.Now().UTC()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-release-claim", models.TaskStatusImplementing, now),
			}
		})

		t.Setenv("LIZA_AGENT_ID", "coder-99")
		err := executeRootCommand(t, projectRoot, "release-claim", "task-release-claim", "--role", "coder", "--force", "--changed-by", "auditor-7")
		if err != nil {
			t.Fatalf("release-claim execute failed: %v", err)
		}

		state := readState(t, statePath)
		task := mustFindTask(t, state, "task-release-claim")
		if task.Status != models.TaskStatusReady {
			t.Fatalf("task status = %s, want %s", task.Status, models.TaskStatusReady)
		}
		if len(task.History) == 0 {
			t.Fatalf("expected history entry for released claim")
		}
		last := task.History[len(task.History)-1]
		if last.Event != "coder_claim_released" {
			t.Fatalf("history event = %s, want coder_claim_released", last.Event)
		}
		if last.Agent == nil || *last.Agent != "auditor-7" {
			t.Fatalf("history agent = %v, want auditor-7", last.Agent)
		}
	})
}

func setupMutationTestProject(t *testing.T, mutateState func(*models.State)) (string, string) {
	t.Helper()

	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	statePath, _ := testhelpers.SetupLizaDir(t, projectRoot)

	state := testhelpers.CreateValidState()
	if mutateState != nil {
		mutateState(state)
	}
	testhelpers.WriteInitialState(t, statePath, state)

	return projectRoot, statePath
}

// executeRootCommand runs a CLI command against the given project root.
// NOTE: os.Chdir is process-global state, which prevents t.Parallel() in this
// package. rootCmd resolves the project root from CWD; until it accepts an
// explicit --project-root flag, Chdir is the only option.
func executeRootCommand(t *testing.T, projectRoot string, args ...string) error {
	t.Helper()

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	defer func() {
		if chdirErr := os.Chdir(oldDir); chdirErr != nil {
			t.Fatalf("failed to restore working directory: %v", chdirErr)
		}
	}()

	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("failed to chdir to project root: %v", err)
	}

	// rootCmd and db singletons are process globals; reset before each command
	// execution so tests don't leak state across runs.
	resetRootCmdForTest(t)
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

func readState(t *testing.T, statePath string) *models.State {
	t.Helper()
	state, err := db.For(statePath).Read()
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	return state
}

func mustFindTask(t *testing.T, state *models.State, taskID string) *models.Task {
	t.Helper()
	task := state.FindTask(taskID)
	if task == nil {
		t.Fatalf("task %s not found", taskID)
	}
	return task
}
