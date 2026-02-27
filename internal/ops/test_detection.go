package ops

import (
	"path/filepath"
	"strings"

	"github.com/liza-mas/liza/internal/git"
)

// isTestFile returns true if the filename matches known test file patterns
// across Go, Python, JS/TS, Shell, Ruby, Java, Kotlin, and Rust.
func isTestFile(name string) bool {
	base := filepath.Base(name)

	// Go: *_test.go
	if strings.HasSuffix(base, "_test.go") {
		return true
	}

	// Python: *_test.py, test_*.py
	if strings.HasSuffix(base, "_test.py") || (strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py")) {
		return true
	}

	// JS/TS: *.test.{js,ts,jsx,tsx}, *.spec.{js,ts,jsx,tsx}, or any file under __tests__/
	for _, ext := range []string{".js", ".ts", ".jsx", ".tsx"} {
		if strings.HasSuffix(base, ".test"+ext) || strings.HasSuffix(base, ".spec"+ext) {
			return true
		}
		if strings.HasSuffix(base, ext) {
			slashed := filepath.ToSlash(name)
			if strings.Contains(slashed, "/__tests__/") || strings.HasPrefix(slashed, "__tests__/") {
				return true
			}
		}
	}

	// Shell: test_*.sh, *_test.sh
	if strings.HasSuffix(base, ".sh") {
		noExt := strings.TrimSuffix(base, ".sh")
		if strings.HasPrefix(noExt, "test_") || strings.HasSuffix(noExt, "_test") {
			return true
		}
	}

	// Ruby: *_test.rb, *_spec.rb
	if strings.HasSuffix(base, "_test.rb") || strings.HasSuffix(base, "_spec.rb") {
		return true
	}

	// Java: *Test.java, Test*.java, *Tests.java
	if strings.HasSuffix(base, "Test.java") || strings.HasSuffix(base, "Tests.java") ||
		(strings.HasPrefix(base, "Test") && strings.HasSuffix(base, ".java")) {
		return true
	}

	// Kotlin: *Test.kt, Test*.kt, *Tests.kt
	if strings.HasSuffix(base, "Test.kt") || strings.HasSuffix(base, "Tests.kt") ||
		(strings.HasPrefix(base, "Test") && strings.HasSuffix(base, ".kt")) {
		return true
	}

	// Rust: *_test.rs, or any .rs file under a tests/ directory
	if strings.HasSuffix(base, "_test.rs") {
		return true
	}
	if strings.HasSuffix(base, ".rs") {
		slashed := filepath.ToSlash(name)
		if strings.Contains(slashed, "/tests/") || strings.HasPrefix(slashed, "tests/") {
			return true
		}
	}

	return false
}

// HasTestFiles checks whether the commits between baseCommit and HEAD in the
// task worktree include any test files (added or modified).
func HasTestFiles(g *git.Git, taskID, baseCommit string) (bool, error) {
	wtPath := g.GetWorktreePath(taskID)
	files, err := g.DiffFiles(wtPath, baseCommit, "HEAD")
	if err != nil {
		return false, err
	}
	for _, f := range files {
		if isTestFile(f) {
			return true, nil
		}
	}
	return false, nil
}
