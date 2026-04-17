package ops

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/embedded"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestCreateWorktree_Validation(t *testing.T) {
	_, err := CreateWorktree("/nonexistent", "", false)
	if err == nil {
		t.Fatal("Expected error for empty task ID")
	}
	if !strings.Contains(err.Error(), "task ID is required") {
		t.Errorf("Error = %q, want to contain 'task ID is required'", err.Error())
	}
}

func TestCreateWorktree_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := CreateWorktree(tmpDir, "nonexistent", false)
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestCreateWorktree_WrongStatus(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := CreateWorktree(tmpDir, "task-1", false)
	if err == nil {
		t.Fatal("Expected error for non-executing task")
	}
	if !strings.Contains(err.Error(), "not in an executing state") {
		t.Errorf("Error = %q, want to contain 'not in an executing state'", err.Error())
	}
}

func TestCreateWorktree_CodePlanningStatus(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusCodePlanning, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := CreateWorktree(tmpDir, "task-1", false)
	if err != nil {
		t.Fatalf("CreateWorktree() for CODE_PLANNING task: unexpected error: %v", err)
	}
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
}

func TestCreateWorktree_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	// Create the worktree directory manually
	testhelpers.CreateTestWorktree(t, tmpDir, "task-1")

	result, err := CreateWorktree(tmpDir, "task-1", false)
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}

	if !result.AlreadyExisted {
		t.Error("AlreadyExisted should be true")
	}
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
}

// TestCreateWorktree_InstallsPreCommitHook covers the post-submit commit guard:
// after wt-create, the worktree must have the liza pre-commit hook wired up
// via extensions.worktreeConfig + --worktree core.hooksPath. Without all three
// pieces (extension on main, hooks file, per-worktree config) git would silently
// fall back to the main repo's hooks and the guard would never fire.
func TestCreateWorktree_InstallsPreCommitHook(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := CreateWorktree(tmpDir, "task-1", false)
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}

	// 1. Hook file exists at the expected path and is executable.
	hookPath := filepath.Join(result.WorktreeDir, ".liza-hooks", "pre-commit")
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("pre-commit hook not installed at %s: %v", hookPath, err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("pre-commit hook is not executable: mode=%v", info.Mode())
	}

	// 2. Main repo has extensions.worktreeConfig=true.
	ext := runGitInDir(t, tmpDir, "config", "--get", "extensions.worktreeConfig")
	if ext != "true" {
		t.Errorf("extensions.worktreeConfig = %q, want %q (required for per-worktree core.hooksPath)", ext, "true")
	}

	// 3. Worktree has core.hooksPath pointing at the installed dir.
	hooksAbs, err := filepath.Abs(filepath.Join(result.WorktreeDir, ".liza-hooks"))
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	// EvalSymlinks because tmp dirs on macOS go through /var → /private/var.
	wantHooksAbs, err := filepath.EvalSymlinks(hooksAbs)
	if err != nil {
		wantHooksAbs = hooksAbs
	}
	got := runGitInDir(t, result.WorktreeDir, "config", "--worktree", "--get", "core.hooksPath")
	gotResolved, err := filepath.EvalSymlinks(got)
	if err != nil {
		gotResolved = got
	}
	if gotResolved != wantHooksAbs {
		t.Errorf("core.hooksPath = %q, want %q", got, hooksAbs)
	}
}

// TestCreateWorktree_InstallsHookOnExisting verifies the upgrade path: a
// pre-hook-era worktree picks up the hook on the next wt-create without
// requiring fresh=true.
func TestCreateWorktree_InstallsHookOnExisting(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)}
	testhelpers.WriteInitialState(t, stateFile, state)

	// First call creates the worktree and installs the hook.
	result, err := CreateWorktree(tmpDir, "task-1", false)
	if err != nil {
		t.Fatalf("first CreateWorktree() error: %v", err)
	}

	// Simulate a pre-hook-era worktree by deleting the hook file.
	hookPath := filepath.Join(result.WorktreeDir, ".liza-hooks", "pre-commit")
	if err := os.RemoveAll(filepath.Join(result.WorktreeDir, ".liza-hooks")); err != nil {
		t.Fatalf("setup: remove hooks dir: %v", err)
	}

	// Second call on the existing worktree must re-install.
	result2, err := CreateWorktree(tmpDir, "task-1", false)
	if err != nil {
		t.Fatalf("second CreateWorktree() error: %v", err)
	}
	if !result2.AlreadyExisted {
		t.Fatal("expected AlreadyExisted=true on the upgrade path")
	}

	if _, err := os.Stat(hookPath); err != nil {
		t.Fatalf("hook not re-installed on AlreadyExisted path: %v", err)
	}
}

// TestCreateWorktree_HookFiresAndRejects is the end-to-end guard:
// it verifies the hook is actually invoked by git (the whole point of the
// extensions.worktreeConfig + core.hooksPath dance) and rejects commits when
// the task is in a non-executing state. This would have caught the earlier
// P0 bug where the hook was installed under .git/worktrees/<id>/hooks/ but
// git never looked there.
//
// The hook is rendered with an inert "liza" binary path; we stub a shell
// script at that path returning exit 1 so the hook rejects unconditionally,
// then confirm git commit rejects. A second pass with the stub returning 0
// confirms the hook path is otherwise permissive.
func TestCreateWorktree_HookFiresAndRejects(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := CreateWorktree(tmpDir, "task-1", false)
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}

	// Overwrite the installed hook with a deterministic rejector so we're
	// testing the hook-invocation plumbing, not CheckCommitAllowed's logic
	// (which has its own unit tests).
	hookPath := filepath.Join(result.WorktreeDir, ".liza-hooks", "pre-commit")
	rejector := "#!/bin/sh\necho 'liza-test-reject' 1>&2\nexit 1\n"
	if err := os.WriteFile(hookPath, []byte(rejector), 0755); err != nil {
		t.Fatalf("write rejector: %v", err)
	}

	// Configure commit identity inside the worktree.
	runGitInDir(t, result.WorktreeDir, "config", "user.email", "test@example.com")
	runGitInDir(t, result.WorktreeDir, "config", "user.name", "Test User")

	// Attempt an empty commit — hook must fire and reject.
	cmd := exec.Command("git", "-C", result.WorktreeDir, "commit", "--allow-empty", "-m", "should-fail")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("git commit succeeded but hook should have rejected. Output:\n%s", out)
	}
	if !strings.Contains(string(out), "liza-test-reject") {
		t.Errorf("hook output missing — git didn't invoke our hook. Output:\n%s", out)
	}

	// --no-verify must bypass, proving the hook is the thing that blocked.
	cmd = exec.Command("git", "-C", result.WorktreeDir, "commit", "--allow-empty", "--no-verify", "-m", "bypass-ok")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("--no-verify should bypass the hook: %v\n%s", err, out)
	}

	// Swap the hook to an allower and confirm a subsequent commit succeeds,
	// ruling out "hook rejects regardless of content" false positives.
	allower := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(hookPath, []byte(allower), 0755); err != nil {
		t.Fatalf("write allower: %v", err)
	}
	cmd = exec.Command("git", "-C", result.WorktreeDir, "commit", "--allow-empty", "-m", "should-pass")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("allower hook should have allowed commit: %v\n%s", err, out)
	}
}

// TestHookShellFailSafeOnUnknownExitCode proves the shell hook enforces the
// fail-safe-allow contract at the shell boundary, not just inside the Go CLI.
// A stub "liza" that exits with a non-policy code (e.g. 127 "command not
// found", 139 "segfault", 2 "panic") must be interpreted as allow by the
// hook wrapper — otherwise a crashing or upgraded-out-of-sync binary would
// deadlock every commit in a worktree.
func TestHookShellFailSafeOnUnknownExitCode(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := CreateWorktree(tmpDir, "task-1", false)
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}

	// Overwrite the installed hook with a re-render of the SHIPPED template
	// (not a handwritten copy) pointing at a stub "liza" that exits 127.
	// Using the real embedded template means this test protects the
	// in-repo script: if someone deletes the case statement, this test
	// fails.
	hookPath := filepath.Join(result.WorktreeDir, ".liza-hooks", "pre-commit")
	stubBin := filepath.Join(tmpDir, "stub-liza")
	if err := os.WriteFile(stubBin, []byte("#!/bin/sh\nexit 127\n"), 0755); err != nil {
		t.Fatalf("write stub liza: %v", err)
	}
	renderedHook := embedded.RenderWorktreePreCommitHook(stubBin, "task-1")
	if err := os.WriteFile(hookPath, renderedHook, 0755); err != nil {
		t.Fatalf("write hook: %v", err)
	}

	runGitInDir(t, result.WorktreeDir, "config", "user.email", "test@example.com")
	runGitInDir(t, result.WorktreeDir, "config", "user.name", "Test User")

	// Stub exits 127 → hook translates to exit 0 → git allows the commit.
	cmd := exec.Command("git", "-C", result.WorktreeDir, "commit", "--allow-empty", "-m", "stub-127-should-allow")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("commit should succeed when stub liza exits 127 (fail-safe allow), got:\n%s\n%v", out, err)
	}
}

// runGitInDir is a test helper for asserting git config state. Returns the
// trimmed stdout of `git -C dir <args...>`.
func runGitInDir(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v in %s: %v", args, dir, err)
	}
	return strings.TrimSpace(string(out))
}
