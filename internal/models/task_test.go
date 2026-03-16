package models

import (
	"testing"
	"time"
)

func TestApprovalHelpers(t *testing.T) {
	t.Run("ApprovalCount", func(t *testing.T) {
		t.Run("empty list", func(t *testing.T) {
			task := &Task{}
			if got := task.ApprovalCount(); got != 0 {
				t.Errorf("ApprovalCount() = %d, want 0", got)
			}
		})

		t.Run("nil list", func(t *testing.T) {
			task := &Task{Approvals: nil}
			if got := task.ApprovalCount(); got != 0 {
				t.Errorf("ApprovalCount() = %d, want 0", got)
			}
		})

		t.Run("single approval", func(t *testing.T) {
			task := &Task{
				Approvals: []Approval{
					{Agent: "reviewer-1", Provider: "claude", Timestamp: time.Now()},
				},
			}
			if got := task.ApprovalCount(); got != 1 {
				t.Errorf("ApprovalCount() = %d, want 1", got)
			}
		})

		t.Run("multiple approvals", func(t *testing.T) {
			task := &Task{
				Approvals: []Approval{
					{Agent: "reviewer-1", Provider: "claude", Timestamp: time.Now()},
					{Agent: "reviewer-2", Provider: "codex", Timestamp: time.Now()},
				},
			}
			if got := task.ApprovalCount(); got != 2 {
				t.Errorf("ApprovalCount() = %d, want 2", got)
			}
		})
	})

	t.Run("HasProviderDiversity", func(t *testing.T) {
		t.Run("empty list", func(t *testing.T) {
			task := &Task{}
			if task.HasProviderDiversity() {
				t.Error("HasProviderDiversity() = true, want false for empty list")
			}
		})

		t.Run("single approval", func(t *testing.T) {
			task := &Task{
				Approvals: []Approval{
					{Agent: "reviewer-1", Provider: "claude", Timestamp: time.Now()},
				},
			}
			if task.HasProviderDiversity() {
				t.Error("HasProviderDiversity() = true, want false for single approval")
			}
		})

		t.Run("same provider", func(t *testing.T) {
			task := &Task{
				Approvals: []Approval{
					{Agent: "reviewer-1", Provider: "claude", Timestamp: time.Now()},
					{Agent: "reviewer-2", Provider: "claude", Timestamp: time.Now()},
				},
			}
			if task.HasProviderDiversity() {
				t.Error("HasProviderDiversity() = true, want false for same provider")
			}
		})

		t.Run("diverse providers", func(t *testing.T) {
			task := &Task{
				Approvals: []Approval{
					{Agent: "reviewer-1", Provider: "claude", Timestamp: time.Now()},
					{Agent: "reviewer-2", Provider: "codex", Timestamp: time.Now()},
				},
			}
			if !task.HasProviderDiversity() {
				t.Error("HasProviderDiversity() = false, want true for diverse providers")
			}
		})

		t.Run("three approvals mixed providers", func(t *testing.T) {
			task := &Task{
				Approvals: []Approval{
					{Agent: "reviewer-1", Provider: "claude", Timestamp: time.Now()},
					{Agent: "reviewer-2", Provider: "claude", Timestamp: time.Now()},
					{Agent: "reviewer-3", Provider: "codex", Timestamp: time.Now()},
				},
			}
			if !task.HasProviderDiversity() {
				t.Error("HasProviderDiversity() = false, want true when at least 2 distinct providers exist")
			}
		})
	})

	t.Run("ClearApprovals", func(t *testing.T) {
		t.Run("clears non-empty list", func(t *testing.T) {
			task := &Task{
				Approvals: []Approval{
					{Agent: "reviewer-1", Provider: "claude", Timestamp: time.Now()},
					{Agent: "reviewer-2", Provider: "codex", Timestamp: time.Now()},
				},
			}
			task.ClearApprovals()
			if len(task.Approvals) != 0 {
				t.Errorf("ClearApprovals() left %d approvals, want 0", len(task.Approvals))
			}
		})

		t.Run("clears empty list", func(t *testing.T) {
			task := &Task{}
			task.ClearApprovals()
			if task.Approvals != nil {
				t.Error("ClearApprovals() on empty task should leave nil")
			}
		})

		t.Run("clears nil list", func(t *testing.T) {
			task := &Task{Approvals: nil}
			task.ClearApprovals()
			if task.Approvals != nil {
				t.Error("ClearApprovals() on nil should leave nil")
			}
		})
	})

	t.Run("LastApprover", func(t *testing.T) {
		t.Run("empty list", func(t *testing.T) {
			task := &Task{}
			if got := task.LastApprover(); got != "" {
				t.Errorf("LastApprover() = %q, want empty string", got)
			}
		})

		t.Run("single approval", func(t *testing.T) {
			task := &Task{
				Approvals: []Approval{
					{Agent: "reviewer-1", Provider: "claude", Timestamp: time.Now()},
				},
			}
			if got := task.LastApprover(); got != "reviewer-1" {
				t.Errorf("LastApprover() = %q, want %q", got, "reviewer-1")
			}
		})

		t.Run("multiple approvals returns last", func(t *testing.T) {
			task := &Task{
				Approvals: []Approval{
					{Agent: "reviewer-1", Provider: "claude", Timestamp: time.Now()},
					{Agent: "reviewer-2", Provider: "codex", Timestamp: time.Now()},
				},
			}
			if got := task.LastApprover(); got != "reviewer-2" {
				t.Errorf("LastApprover() = %q, want %q", got, "reviewer-2")
			}
		})
	})
}
