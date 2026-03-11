package git

import (
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/testhelpers"
)

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
