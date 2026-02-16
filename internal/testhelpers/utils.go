package testhelpers

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
