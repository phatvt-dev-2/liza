package testhelpers

import (
	"errors"
	"strings"
	"testing"
)

// mockT is a mock testing.T that captures error messages instead of failing
type mockT struct {
	errorCalled  bool
	errorfCalled bool
	lastMessage  string
	helperCalled bool
}

func (m *mockT) Helper() {
	m.helperCalled = true
}

func (m *mockT) Error(args ...any) {
	m.errorCalled = true
	if len(args) > 0 {
		m.lastMessage = args[0].(string)
	}
}

func (m *mockT) Errorf(format string, args ...any) {
	m.errorfCalled = true
	m.lastMessage = format
	// Simple formatting for testing
	if strings.Contains(format, "%q") || strings.Contains(format, "%v") {
		m.lastMessage = format // Store format for verification
	}
}

func (m *mockT) Fatal(args ...any) {
	m.errorCalled = true
	if len(args) > 0 {
		m.lastMessage = args[0].(string)
	}
}

func (m *mockT) Fatalf(format string, args ...any) {
	m.errorfCalled = true
	m.lastMessage = format
}

func TestAssertErrorContains_Success(t *testing.T) {
	// Test with error containing the substring
	err := errors.New("invalid task status: INVALID")

	// This should not fail
	AssertErrorContains(t, err, "invalid task")
	AssertErrorContains(t, err, "status")
	AssertErrorContains(t, err, "INVALID")
}

func TestAssertErrorContains_EmptySubstring(t *testing.T) {
	// Empty substring should be a no-op (allows conditional checking)
	err := errors.New("some error")

	// This should not fail even with error
	AssertErrorContains(t, err, "")

	// This should not fail even without error
	AssertErrorContains(t, nil, "")
}

func TestAssertErrorContains_FailsOnNilError(t *testing.T) {
	mock := &mockT{}

	// Should fail when expecting substring but got no error
	AssertErrorContains(mock, nil, "expected error")

	if !mock.errorfCalled {
		t.Error("Expected Errorf to be called")
	}
	if !mock.helperCalled {
		t.Error("Expected Helper to be called")
	}
	if !strings.Contains(mock.lastMessage, "Expected error containing") {
		t.Errorf("Expected error message about nil error, got: %s", mock.lastMessage)
	}
}

func TestAssertErrorContains_FailsOnMissingSubstring(t *testing.T) {
	mock := &mockT{}
	err := errors.New("something went wrong")

	// Should fail when error doesn't contain substring
	AssertErrorContains(mock, err, "expected substring")

	if !mock.errorfCalled {
		t.Error("Expected Errorf to be called")
	}
	if !mock.helperCalled {
		t.Error("Expected Helper to be called")
	}
	if !strings.Contains(mock.lastMessage, "Expected error containing") {
		t.Errorf("Expected error message about missing substring, got: %s", mock.lastMessage)
	}
}

func TestAssertErrorContains_CaseSensitive(t *testing.T) {
	mock := &mockT{}
	err := errors.New("Invalid Task Status")

	// Should be case-sensitive
	AssertErrorContains(mock, err, "invalid")

	if !mock.errorfCalled {
		t.Error("Expected case-sensitive comparison to fail")
	}
}

func TestAssertError_WantErrorGotError(t *testing.T) {
	// Should succeed when expecting error and getting error
	err := errors.New("some error")
	AssertError(t, err, true)
}

func TestAssertError_NoErrorExpectedNoError(t *testing.T) {
	// Should succeed when not expecting error and getting no error
	AssertError(t, nil, false)
}

func TestAssertError_WantErrorGotNil(t *testing.T) {
	mock := &mockT{}

	// Should fail when expecting error but got nil
	AssertError(mock, nil, true)

	if !mock.errorCalled {
		t.Error("Expected Error to be called")
	}
	if !mock.helperCalled {
		t.Error("Expected Helper to be called")
	}
	if !strings.Contains(mock.lastMessage, "Expected an error") {
		t.Errorf("Expected error message about missing error, got: %s", mock.lastMessage)
	}
}

func TestAssertError_NoErrorExpectedButGotError(t *testing.T) {
	mock := &mockT{}
	err := errors.New("unexpected error")

	// Should fail when not expecting error but got one
	AssertError(mock, err, false)

	if !mock.errorfCalled {
		t.Error("Expected Errorf to be called")
	}
	if !mock.helperCalled {
		t.Error("Expected Helper to be called")
	}
	if !strings.Contains(mock.lastMessage, "Expected no error") {
		t.Errorf("Expected error message about unexpected error, got: %s", mock.lastMessage)
	}
}

func TestAssertNoError_Success(t *testing.T) {
	// Should succeed with nil error
	AssertNoError(t, nil)
}

func TestAssertNoError_Failure(t *testing.T) {
	mock := &mockT{}
	err := errors.New("unexpected error")

	// Should fail with non-nil error
	AssertNoError(mock, err)

	if !mock.errorfCalled {
		t.Error("Expected Errorf to be called")
	}
	if !mock.helperCalled {
		t.Error("Expected Helper to be called")
	}
	if !strings.Contains(mock.lastMessage, "Expected no error") {
		t.Errorf("Expected error message about unexpected error, got: %s", mock.lastMessage)
	}
}

func TestAssertions_HelperCalled(t *testing.T) {
	// Verify all assertion functions call t.Helper()
	mock := &mockT{}

	// Force failures to check Helper was called
	AssertErrorContains(mock, nil, "error")
	if !mock.helperCalled {
		t.Error("AssertErrorContains didn't call Helper")
	}

	mock = &mockT{}
	AssertError(mock, nil, true)
	if !mock.helperCalled {
		t.Error("AssertError didn't call Helper")
	}

	mock = &mockT{}
	AssertNoError(mock, errors.New("err"))
	if !mock.helperCalled {
		t.Error("AssertNoError didn't call Helper")
	}
}

func TestAssertErrorContains_RealWorldPatterns(t *testing.T) {
	// Test patterns that appear in actual test files

	// Pattern 1: From claim_task_test.go, wt_create_test.go, wt_delete_test.go
	tests := []struct {
		name        string
		err         error
		errContains string
		shouldFail  bool
	}{
		{
			name:        "task not found",
			err:         errors.New("task not found: task-123"),
			errContains: "not found",
			shouldFail:  false,
		},
		{
			name:        "already claimed",
			err:         errors.New("task already claimed by agent-1"),
			errContains: "already claimed",
			shouldFail:  false,
		},
		{
			name:        "mismatch",
			err:         errors.New("invalid status"),
			errContains: "not found",
			shouldFail:  true,
		},
		{
			name:        "nil error with empty substring",
			err:         nil,
			errContains: "",
			shouldFail:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockT{}
			AssertErrorContains(mock, tt.err, tt.errContains)

			if tt.shouldFail && !mock.errorfCalled {
				t.Error("Expected assertion to fail")
			}
			if !tt.shouldFail && mock.errorfCalled {
				t.Errorf("Expected assertion to pass, but got error: %s", mock.lastMessage)
			}
		})
	}
}

func TestAssertErrorContains_ManualSubstringPattern(t *testing.T) {
	// Test pattern from release_claim_test.go and submit_verdict_test.go
	// These tests use manual substring search loop - our helper should handle the same cases

	err := errors.New("task task-1 has lease expiring at 2024-01-01 12:00:00")

	// Should find substring anywhere in the error
	AssertErrorContains(t, err, "task task-1")
	AssertErrorContains(t, err, "has lease")
	AssertErrorContains(t, err, "expiring at")
	AssertErrorContains(t, err, "2024-01-01")

	// Should handle edge cases
	AssertErrorContains(t, err, "task")     // beginning
	AssertErrorContains(t, err, "12:00:00") // end

	// Verify exact match works
	fullMsg := "task task-1 has lease expiring at 2024-01-01 12:00:00"
	AssertErrorContains(t, err, fullMsg)
}

func TestAssertError_TableDrivenPattern(t *testing.T) {
	// Test table-driven pattern commonly used in tests
	tests := []struct {
		name    string
		err     error
		wantErr bool
	}{
		{"success case", nil, false},
		{"error case", errors.New("failed"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AssertError(t, tt.err, tt.wantErr)
		})
	}
}

func TestAssertNoError_CommonUsage(t *testing.T) {
	// Test common usage pattern from test files

	// Simulating a function that should always succeed in tests
	operation := func() error {
		return nil
	}

	err := operation()
	AssertNoError(t, err)

	// This pattern appears frequently:
	// if err := someOperation(); err != nil {
	//     t.Fatalf("Failed: %v", err)
	// }
	// Our helper simplifies this to:
	// AssertNoError(t, someOperation())
}
