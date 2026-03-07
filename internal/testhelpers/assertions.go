package testhelpers

// assertions.go contains test helpers for common assertion patterns.
//
// This file provides standardized assertion functions that replace inconsistent
// error checking patterns found across test files. Using these helpers improves
// test readability and provides consistent error messages.
//
// Usage Example:
//
//	func TestValidation(t *testing.T) {
//	    result, err := SomeOperation()
//
//	    // Check for expected error with specific message
//	    testhelpers.AssertErrorContains(t, err, "invalid task status")
//
//	    // Or verify no error occurred
//	    testhelpers.AssertNoError(t, err)
//
//	    // Or check error presence matches expectation
//	    testhelpers.AssertError(t, err, true) // expect error
//	}
//
// All assertion functions use t.Helper() to ensure test failures are reported
// at the correct line in the calling test code, not inside the helper function.

import (
	"strings"
)

// testingT is a minimal interface for testing helpers.
// This allows us to test the assertion functions themselves.
type testingT interface {
	Helper()
	Error(args ...any)
	Errorf(format string, args ...any)
	Fatal(args ...any)
	Fatalf(format string, args ...any)
}

// AssertErrorContains checks that an error contains a specific substring.
// If substring is empty, the function does nothing (allows conditional checking).
// If err is nil but substring is non-empty, the test fails.
// If err is non-nil but doesn't contain substring, the test fails.
//
// This helper standardizes error message checking across test files, replacing
// two inconsistent patterns:
//   - Pattern 1: Simple strings.Contains check (used in some tests)
//   - Pattern 2: Manual substring search loop (used in release_claim_test.go, etc.)
//
// This eliminates ~10-15 lines of duplicated error checking code that appears
// 15-20 times across test files.
//
// Usage:
//
//	err := SomeOperation()
//	testhelpers.AssertErrorContains(t, err, "invalid task")
func AssertErrorContains(t testingT, err error, substring string) {
	t.Helper()

	if substring == "" {
		return
	}

	if err == nil {
		t.Errorf("Expected error containing %q, but got nil error", substring)
		return
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, substring) {
		t.Errorf("Expected error containing %q, got %q", substring, errMsg)
	}
}

// AssertError validates whether an error was returned based on expectations.
// If wantErr is true, fails if err is nil.
// If wantErr is false, fails if err is non-nil.
//
// This is a common pattern in table-driven tests where each test case specifies
// whether an error is expected.
//
// Usage:
//
//	err := SomeOperation()
//	testhelpers.AssertError(t, err, tt.wantErr)
func AssertError(t testingT, err error, wantErr bool) {
	t.Helper()

	if wantErr && err == nil {
		t.Error("Expected an error, but got nil")
	} else if !wantErr && err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
}

// AssertNoError fails the test if err is non-nil.
// This is a very common pattern for operations that should always succeed in tests.
//
// Usage:
//
//	err := SomeOperation()
//	testhelpers.AssertNoError(t, err)
func AssertNoError(t testingT, err error) {
	t.Helper()

	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
}
