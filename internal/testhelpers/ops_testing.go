package testhelpers

// ops_testing.go contains test helpers for tests in the internal/ops package.
//
// This file provides standardized assertion patterns that appear repeatedly
// across ops test files (wt_delete_test.go, wt_merge_test.go,
// supersede_task_test.go, submit_verdict_test.go, etc.).

import (
	"strings"
)

// RequireErrorContains fails the test immediately if err is nil or doesn't contain substring.
// This combines the nil check and substring check that appears 50+ times across ops tests.
//
// This replaces the pattern:
//
//	if err == nil {
//	    t.Fatal("Expected error, got nil")
//	}
//	if !strings.Contains(err.Error(), "expected message") {
//	    t.Errorf("Error = %q, want to contain %q", err.Error(), "expected message")
//	}
//
// With:
//
//	testhelpers.RequireErrorContains(t, err, "expected message")
//
// Usage:
//
//	err := SomeOperation()
//	testhelpers.RequireErrorContains(t, err, "task not found")
func RequireErrorContains(t testingT, err error, substring string) {
	t.Helper()

	if err == nil {
		t.Fatalf("Expected error containing %q, but got nil", substring)
		return
	}

	if !strings.Contains(err.Error(), substring) {
		t.Fatalf("Error = %q, want to contain %q", err.Error(), substring)
	}
}
