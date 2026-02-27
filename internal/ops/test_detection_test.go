package ops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestHasTestFiles_WithTestFile(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)

	g := git.New(tmpDir)
	taskID := "task-1"
	baseCommit, err := g.CreateWorktree(taskID, "main")
	if err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}

	wtPath := g.GetWorktreePath(taskID)

	// Add a test file
	testFile := filepath.Join(wtPath, "hello_test.go")
	if err := os.WriteFile(testFile, []byte("package hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, wtPath, "add", "hello_test.go")
	testhelpers.MustGit(t, wtPath, "commit", "-m", "Add test file")

	hasTests, err := HasTestFiles(g, taskID, baseCommit)
	if err != nil {
		t.Fatalf("HasTestFiles failed: %v", err)
	}
	if !hasTests {
		t.Error("Expected HasTestFiles to return true when test file is present")
	}
}

func TestHasTestFiles_WithoutTestFile(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)

	g := git.New(tmpDir)
	taskID := "task-1"
	baseCommit, err := g.CreateWorktree(taskID, "main")
	if err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}

	wtPath := g.GetWorktreePath(taskID)

	// Add a non-test file only
	implFile := filepath.Join(wtPath, "hello.go")
	if err := os.WriteFile(implFile, []byte("package hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testhelpers.MustGit(t, wtPath, "add", "hello.go")
	testhelpers.MustGit(t, wtPath, "commit", "-m", "Add implementation without tests")

	hasTests, err := HasTestFiles(g, taskID, baseCommit)
	if err != nil {
		t.Fatalf("HasTestFiles failed: %v", err)
	}
	if hasTests {
		t.Error("Expected HasTestFiles to return false when no test file is present")
	}
}

func TestHasTestFiles_NoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)

	g := git.New(tmpDir)
	taskID := "task-1"
	baseCommit, err := g.CreateWorktree(taskID, "main")
	if err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}

	// No commits since worktree creation
	hasTests, err := HasTestFiles(g, taskID, baseCommit)
	if err != nil {
		t.Fatalf("HasTestFiles failed: %v", err)
	}
	if hasTests {
		t.Error("Expected HasTestFiles to return false when no changes exist")
	}
}
