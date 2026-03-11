package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/testhelpers"
)

// setupTestRepo creates a test git repository with an integration branch
func setupTestRepo(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "test-repo")

	// Create directory
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Initialize git repo with basic setup (init, config, initial commit, integration branch)
	testhelpers.SetupTestGitRepo(t, repoDir)

	// Checkout integration branch and add another commit (specific to worktree tests)
	testFile := filepath.Join(repoDir, "README.md")
	testhelpers.MustGit(t, repoDir, "checkout", "integration")
	if err := os.WriteFile(testFile, []byte("# Test\nIntegration branch\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, repoDir, "add", "README.md")
	testhelpers.MustGit(t, repoDir, "commit", "-m", "Integration setup")

	return repoDir
}

func TestCreateWorktree(t *testing.T) {
	repoDir := setupTestRepo(t)
	git := New(repoDir)

	taskID := "task-123"
	worktreePath := filepath.Join(repoDir, ".worktrees", taskID)

	// Create worktree from integration branch
	baseCommit, err := git.CreateWorktree(taskID, "integration")
	if err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	// Verify worktree directory exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Error("Worktree directory was not created")
	}

	// Verify task branch was created
	branches := testhelpers.MustGit(t, repoDir, "branch", "--list", "task/"+taskID)
	if !strings.Contains(branches, "task/"+taskID) {
		t.Errorf("task/%s branch was not created", taskID)
	}

	// Verify base_commit is a full SHA
	if len(baseCommit) != 40 {
		t.Errorf("baseCommit length = %d, want 40", len(baseCommit))
	}

	// Verify worktree is on correct branch
	wtBranch := testhelpers.MustGit(t, worktreePath, "branch", "--show-current")
	if wtBranch != "task/"+taskID {
		t.Errorf("Worktree branch = %q, want %q", wtBranch, "task/"+taskID)
	}
}

func TestCreateWorktreeAlreadyExists(t *testing.T) {
	repoDir := setupTestRepo(t)
	git := New(repoDir)

	taskID := "task-123"

	// Create worktree first time
	if _, err := git.CreateWorktree(taskID, "integration"); err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	// Try to create again without fresh flag - should return error or succeed
	_, err := git.CreateWorktree(taskID, "integration")
	if err == nil {
		t.Log("CreateWorktree() succeeded when worktree already exists (acceptable)")
	} else {
		// Error is also acceptable behavior
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("CreateWorktree() error should mention 'already exists', got: %v", err)
		}
	}
}

func TestCreateWorktreeFresh(t *testing.T) {
	repoDir := setupTestRepo(t)
	git := New(repoDir)

	taskID := "task-123"

	// Create worktree first time
	if _, err := git.CreateWorktree(taskID, "integration"); err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	// Create with fresh flag - should delete and recreate
	_, err := git.CreateWorktreeFresh(taskID, "integration")
	if err != nil {
		t.Fatalf("CreateWorktreeFresh() error = %v", err)
	}

	// Verify worktree still exists
	worktreePath := filepath.Join(repoDir, ".worktrees", taskID)
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Error("Worktree directory does not exist after fresh create")
	}
}

func TestRemoveWorktree(t *testing.T) {
	repoDir := setupTestRepo(t)
	git := New(repoDir)

	taskID := "task-123"
	worktreePath := filepath.Join(repoDir, ".worktrees", taskID)

	// Create worktree first
	if _, err := git.CreateWorktree(taskID, "integration"); err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	// Remove worktree
	if err := git.RemoveWorktree(taskID); err != nil {
		t.Fatalf("RemoveWorktree() error = %v", err)
	}

	// Verify worktree directory is gone
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("Worktree directory still exists after removal")
	}

	// Verify branch is deleted
	branches := testhelpers.MustGit(t, repoDir, "branch", "--list", "task/"+taskID)
	if strings.Contains(branches, "task/"+taskID) {
		t.Errorf("task/%s branch still exists after removal", taskID)
	}
}

func TestRemoveWorktreeNotExists(t *testing.T) {
	repoDir := setupTestRepo(t)
	git := New(repoDir)

	// Try to remove non-existent worktree - should not error
	err := git.RemoveWorktree("task-nonexistent")
	if err != nil {
		t.Errorf("RemoveWorktree() on non-existent worktree error = %v, want nil", err)
	}
}

func TestListWorktrees(t *testing.T) {
	repoDir := setupTestRepo(t)
	git := New(repoDir)

	// Initially should have just the main worktree
	worktrees, err := git.ListWorktrees()
	if err != nil {
		t.Fatalf("ListWorktrees() error = %v", err)
	}
	if len(worktrees) == 0 {
		t.Fatal("ListWorktrees() returned empty list")
	}

	// Create a task worktree
	taskID := "task-123"
	if _, err := git.CreateWorktree(taskID, "integration"); err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	// List again
	worktrees, err = git.ListWorktrees()
	if err != nil {
		t.Fatalf("ListWorktrees() error = %v", err)
	}

	// Should have at least 2 worktrees now
	if len(worktrees) < 2 {
		t.Errorf("ListWorktrees() returned %d worktrees, want at least 2", len(worktrees))
	}

	// Check that our task worktree is in the list
	found := false
	expectedPath := filepath.Join(repoDir, ".worktrees", taskID)

	// Resolve symlinks for comparison (macOS /var -> /private/var)
	expectedResolved, err := filepath.EvalSymlinks(expectedPath)
	if err != nil {
		t.Fatalf("EvalSymlinks() error = %v", err)
	}

	for _, wt := range worktrees {
		wtResolved, err := filepath.EvalSymlinks(wt.Path)
		if err != nil {
			// Skip worktrees we can't resolve
			continue
		}

		if wtResolved == expectedResolved {
			found = true
			if wt.Branch != "task/"+taskID {
				t.Errorf("Worktree branch = %q, want %q", wt.Branch, "task/"+taskID)
			}
			break
		}
	}

	if !found {
		t.Errorf("Task worktree %s (resolved: %s) not found in list", expectedPath, expectedResolved)
	}
}

func TestGetWorktreePath(t *testing.T) {
	repoDir := "/test/repo"
	git := New(repoDir)

	taskID := "task-123"
	expected := filepath.Join(repoDir, ".worktrees", taskID)

	got := git.GetWorktreePath(taskID)
	if got != expected {
		t.Errorf("GetWorktreePath() = %q, want %q", got, expected)
	}
}

func TestGetWorktreeRelPath(t *testing.T) {
	git := New("/test/repo")

	taskID := "task-123"
	expected := filepath.Join(".worktrees", taskID)

	got := git.GetWorktreeRelPath(taskID)
	if got != expected {
		t.Errorf("GetWorktreeRelPath() = %q, want %q", got, expected)
	}
}
