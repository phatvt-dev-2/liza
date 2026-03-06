package testhelpers

import (
	"os"
	"path/filepath"
	"testing"
)

// utils.go contains common utility functions for tests.
//
// This file provides simple helper functions that are used across multiple tests,
// such as pointer creation helpers for test data construction.
//
// Usage Example:
//
//	func TestTaskWithOptionalFields(t *testing.T) {
//	    task := models.Task{
//	        ID:          "task-1",
//	        Description: "Test task",
//	        AssignedTo:  testhelpers.StringPtr("agent-1"),
//	        LeaseExpires: testhelpers.TimePtr(time.Now().Add(30 * time.Minute)),
//	    }
//	    // ... continue with test
//	}
//
// These utilities eliminate duplicated helper functions that were independently
// defined in multiple test files (e.g., stringPtr appearing in claim_task_test.go
// and models_test.go).

// FindRepoRoot walks up from the current working directory to find the repository
// root (directory containing go.mod). Useful for locating testdata files.
func FindRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (go.mod)")
		}
		dir = parent
	}
}
