package git

import (
	"errors"
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

func TestNew(t *testing.T) {
	repoDir := setupTestRepo(t)

	git := New(repoDir)
	if git.projectRoot != repoDir {
		t.Errorf("projectRoot = %q, want %q", git.projectRoot, repoDir)
	}
}

func TestGetCurrentBranch(t *testing.T) {
	repoDir := setupTestRepo(t)
	git := New(repoDir)

	// Should be on integration branch from setup
	branch, err := git.GetCurrentBranch()
	if err != nil {
		t.Fatalf("GetCurrentBranch() error = %v", err)
	}
	if branch != "integration" {
		t.Errorf("GetCurrentBranch() = %q, want %q", branch, "integration")
	}
}

func TestGetCommitSHA(t *testing.T) {
	repoDir := setupTestRepo(t)
	git := New(repoDir)

	// Get HEAD SHA
	sha, err := git.GetCommitSHA("HEAD")
	if err != nil {
		t.Fatalf("GetCommitSHA(HEAD) error = %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("GetCommitSHA() returned %d chars, want 40", len(sha))
	}

	// Get short SHA
	shortSHA, err := git.GetCommitSHA("HEAD", true)
	if err != nil {
		t.Fatalf("GetCommitSHA(HEAD, short) error = %v", err)
	}
	if len(shortSHA) != 7 {
		t.Errorf("GetCommitSHA(short) returned %d chars, want 7", len(shortSHA))
	}
	if !strings.HasPrefix(sha, shortSHA) {
		t.Errorf("short SHA %q is not prefix of full SHA %q", shortSHA, sha)
	}
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

func TestCalculateDrift(t *testing.T) {
	repoDir := setupTestRepo(t)
	git := New(repoDir)

	// Get current integration HEAD as base
	baseCommit, err := git.GetCommitSHA("integration")
	if err != nil {
		t.Fatalf("GetCommitSHA() error = %v", err)
	}

	// No drift initially
	drift, err := git.CalculateDrift(baseCommit, "integration")
	if err != nil {
		t.Fatalf("CalculateDrift() error = %v", err)
	}
	if drift != 0 {
		t.Errorf("CalculateDrift() = %d, want 0", drift)
	}

	// Make a new commit on integration
	testFile := filepath.Join(repoDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, repoDir, "add", "test.txt")
	testhelpers.MustGit(t, repoDir, "commit", "-m", "New commit")

	// Now there should be 1 commit of drift
	drift, err = git.CalculateDrift(baseCommit, "integration")
	if err != nil {
		t.Fatalf("CalculateDrift() error = %v", err)
	}
	if drift != 1 {
		t.Errorf("CalculateDrift() = %d, want 1", drift)
	}
}

func TestBranchExists(t *testing.T) {
	repoDir := setupTestRepo(t)
	git := New(repoDir)

	// integration branch should exist
	exists, err := git.BranchExists("integration")
	if err != nil {
		t.Fatalf("BranchExists(integration) error = %v", err)
	}
	if !exists {
		t.Error("BranchExists(integration) = false, want true")
	}

	// nonexistent branch should not exist
	exists, err = git.BranchExists("nonexistent")
	if err != nil {
		t.Fatalf("BranchExists(nonexistent) error = %v", err)
	}
	if exists {
		t.Error("BranchExists(nonexistent) = true, want false")
	}
}

func TestDeleteBranch(t *testing.T) {
	repoDir := setupTestRepo(t)
	git := New(repoDir)

	// Create a test branch
	testhelpers.MustGit(t, repoDir, "checkout", "-b", "test-branch")
	testhelpers.MustGit(t, repoDir, "checkout", "integration") // Switch back

	// Delete the branch
	if err := git.DeleteBranch("test-branch"); err != nil {
		t.Fatalf("DeleteBranch() error = %v", err)
	}

	// Verify it's gone
	exists, err := git.BranchExists("test-branch")
	if err != nil {
		t.Fatalf("BranchExists() error = %v", err)
	}
	if exists {
		t.Error("Branch still exists after deletion")
	}
}

func TestCheckoutBranch(t *testing.T) {
	repoDir := setupTestRepo(t)
	git := New(repoDir)

	// Checkout main
	testhelpers.MustGit(t, repoDir, "checkout", "main")

	// Use our function to checkout integration
	if err := git.CheckoutBranch("integration"); err != nil {
		t.Fatalf("CheckoutBranch() error = %v", err)
	}

	// Verify we're on integration
	branch, err := git.GetCurrentBranch()
	if err != nil {
		t.Fatalf("GetCurrentBranch() error = %v", err)
	}
	if branch != "integration" {
		t.Errorf("Current branch = %q, want %q", branch, "integration")
	}
}

func TestCreateBranch(t *testing.T) {
	repoDir := setupTestRepo(t)
	git := New(repoDir)

	branchName := "test-new-branch"

	// Create a new branch
	if err := git.CreateBranch(branchName); err != nil {
		t.Fatalf("CreateBranch() error = %v", err)
	}

	// Verify branch exists
	exists, err := git.BranchExists(branchName)
	if err != nil {
		t.Fatalf("BranchExists() error = %v", err)
	}
	if !exists {
		t.Error("Branch was not created")
	}

	// Verify we're still on original branch (CreateBranch doesn't checkout)
	current, err := git.GetCurrentBranch()
	if err != nil {
		t.Fatalf("GetCurrentBranch() error = %v", err)
	}
	if current == branchName {
		t.Error("CreateBranch should not checkout the new branch")
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

func TestGetWorktreeHEAD(t *testing.T) {
	repoDir := setupTestRepo(t)
	git := New(repoDir)

	taskID := "task-123"

	// Create a worktree
	if _, err := git.CreateWorktree(taskID, "integration"); err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	// Get the worktree's HEAD
	headSHA, err := git.GetWorktreeHEAD(taskID)
	if err != nil {
		t.Fatalf("GetWorktreeHEAD() error = %v", err)
	}

	// Verify it's a valid SHA (40 chars)
	if len(headSHA) != 40 {
		t.Errorf("GetWorktreeHEAD() returned %d chars, want 40", len(headSHA))
	}

	// Get integration branch HEAD for comparison
	integrationSHA, err := git.GetCommitSHA("integration")
	if err != nil {
		t.Fatalf("GetCommitSHA() error = %v", err)
	}

	// Worktree should be at same commit as integration (since we just created it)
	if headSHA != integrationSHA {
		t.Errorf("GetWorktreeHEAD() = %s, want %s", headSHA, integrationSHA)
	}
}

func TestUpdateRef(t *testing.T) {
	repoDir := setupTestRepo(t)
	g := New(repoDir)

	testhelpers.MustGit(t, repoDir, "checkout", "integration")

	sha1, err := g.GetCommitSHA("HEAD")
	if err != nil {
		t.Fatal(err)
	}

	// Add another commit so we have two distinct SHAs
	testFile := filepath.Join(repoDir, "update-ref-test.txt")
	if writeErr := os.WriteFile(testFile, []byte("v1\n"), 0644); writeErr != nil {
		t.Fatal(writeErr)
	}
	testhelpers.MustGit(t, repoDir, "add", ".")
	testhelpers.MustGit(t, repoDir, "commit", "-m", "for update-ref test")
	sha2, err := g.GetCommitSHA("HEAD")
	if err != nil {
		t.Fatal(err)
	}

	ref := "refs/heads/integration"

	t.Run("unconditional update (empty expectedOldSHA)", func(t *testing.T) {
		// Move ref to sha1 unconditionally
		if err := g.UpdateRef(ref, sha1, ""); err != nil {
			t.Fatalf("UpdateRef unconditional failed: %v", err)
		}
		cur, _ := g.GetCommitSHA(ref)
		if cur != sha1 {
			t.Errorf("ref = %s, want %s", cur, sha1)
		}
	})

	t.Run("CAS success", func(t *testing.T) {
		// Set to sha1 first
		if err := g.UpdateRef(ref, sha1, ""); err != nil {
			t.Fatal(err)
		}
		// CAS: sha1 → sha2 with correct expected
		if err := g.UpdateRef(ref, sha2, sha1); err != nil {
			t.Fatalf("CAS should succeed: %v", err)
		}
		cur, _ := g.GetCommitSHA(ref)
		if cur != sha2 {
			t.Errorf("ref = %s, want %s", cur, sha2)
		}
	})

	t.Run("CAS failure returns RefConflictError", func(t *testing.T) {
		// ref is at sha2, but we claim it's at sha1
		err := g.UpdateRef(ref, sha1, sha1)
		if err == nil {
			t.Fatal("CAS should fail when expected doesn't match actual")
		}
		var casErr *RefConflictError
		if !errors.As(err, &casErr) {
			t.Fatalf("expected *RefConflictError, got %T: %v", err, err)
		}
		if casErr.Ref != ref {
			t.Errorf("RefConflictError.Ref = %q, want %q", casErr.Ref, ref)
		}
		if casErr.Expected != sha1 {
			t.Errorf("RefConflictError.Expected = %q, want %q", casErr.Expected, sha1)
		}
		if casErr.Actual != sha2 {
			t.Errorf("RefConflictError.Actual = %q, want %q (parsed from git error)", casErr.Actual, sha2)
		}
		// Verify ref was NOT moved
		cur, _ := g.GetCommitSHA(ref)
		if cur != sha2 {
			t.Errorf("ref should still be %s after CAS failure, got %s", sha2, cur)
		}
	})
}

func TestMergeBranchFastForward(t *testing.T) {
	repoDir := setupTestRepo(t)
	git := New(repoDir)

	// Create a feature branch from main
	testhelpers.MustGit(t, repoDir, "checkout", "main")
	testhelpers.MustGit(t, repoDir, "checkout", "-b", "feature")

	// Add a commit on feature
	testFile := filepath.Join(repoDir, "feature.txt")
	if err := os.WriteFile(testFile, []byte("feature\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, repoDir, "add", "feature.txt")
	testhelpers.MustGit(t, repoDir, "commit", "-m", "Feature commit")

	// Go back to main and merge (should be fast-forward)
	testhelpers.MustGit(t, repoDir, "checkout", "main")

	fastForward, mergeCommit, err := git.MergeBranch("feature")
	if err != nil {
		t.Fatalf("MergeBranch() error = %v", err)
	}
	if !fastForward {
		t.Error("Expected fast-forward merge")
	}
	if mergeCommit == "" {
		t.Error("Expected merge commit SHA")
	}

	// Verify the feature file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Feature file should exist after merge")
	}
}

func TestMergeBranchNoFastForward(t *testing.T) {
	repoDir := setupTestRepo(t)
	git := New(repoDir)

	// Start from integration branch
	testhelpers.MustGit(t, repoDir, "checkout", "integration")

	// Create feature branch
	testhelpers.MustGit(t, repoDir, "checkout", "-b", "feature")
	featureFile := filepath.Join(repoDir, "feature.txt")
	if err := os.WriteFile(featureFile, []byte("feature\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, repoDir, "add", "feature.txt")
	testhelpers.MustGit(t, repoDir, "commit", "-m", "Feature commit")

	// Go back to integration and make another commit (diverge)
	testhelpers.MustGit(t, repoDir, "checkout", "integration")
	otherFile := filepath.Join(repoDir, "other.txt")
	if err := os.WriteFile(otherFile, []byte("other\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, repoDir, "add", "other.txt")
	testhelpers.MustGit(t, repoDir, "commit", "-m", "Other commit")

	// Now merge feature (cannot be fast-forward)
	fastForward, mergeCommit, err := git.MergeBranch("feature")
	if err != nil {
		t.Fatalf("MergeBranch() error = %v", err)
	}
	if fastForward {
		t.Error("Expected non-fast-forward merge")
	}
	if mergeCommit == "" {
		t.Error("Expected merge commit SHA")
	}

	// Verify both files exist
	if _, err := os.Stat(featureFile); os.IsNotExist(err) {
		t.Error("Feature file should exist after merge")
	}
	if _, err := os.Stat(otherFile); os.IsNotExist(err) {
		t.Error("Other file should exist after merge")
	}
}

func TestMergeBranchConflict(t *testing.T) {
	repoDir := setupTestRepo(t)
	git := New(repoDir)

	// Start from integration branch
	testhelpers.MustGit(t, repoDir, "checkout", "integration")

	// Create feature branch and modify README
	testhelpers.MustGit(t, repoDir, "checkout", "-b", "feature")
	readmeFile := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(readmeFile, []byte("# Feature version\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, repoDir, "add", "README.md")
	testhelpers.MustGit(t, repoDir, "commit", "-m", "Feature README")

	// Go back to integration and modify README differently
	testhelpers.MustGit(t, repoDir, "checkout", "integration")
	if err := os.WriteFile(readmeFile, []byte("# Integration version\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, repoDir, "add", "README.md")
	testhelpers.MustGit(t, repoDir, "commit", "-m", "Integration README")

	// Try to merge - should conflict
	_, _, err := git.MergeBranch("feature")
	if err == nil {
		t.Error("Expected merge conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "conflict") && !strings.Contains(err.Error(), "CONFLICT") {
		t.Errorf("Expected conflict error, got: %v", err)
	}

	// Abort the merge
	if err := git.AbortMerge(); err != nil {
		t.Fatalf("AbortMerge() error = %v", err)
	}

	// Verify we're back to clean state
	output := testhelpers.MustGit(t, repoDir, "status", "--porcelain")
	if output != "" {
		t.Error("Expected clean state after merge abort")
	}
}
