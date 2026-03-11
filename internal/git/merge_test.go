package git

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/testhelpers"
)

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
