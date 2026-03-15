package mcp

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/pipeline"
)

func TestWithLogging(t *testing.T) {
	t.Run("logs success with duration", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))

		inner := func(params map[string]any) (any, error) {
			return "ok", nil
		}
		wrapped := withLogging(logger, "test_tool", inner)

		result, err := wrapped(map[string]any{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "ok" {
			t.Fatalf("expected 'ok', got %v", result)
		}

		logOutput := buf.String()
		if !strings.Contains(logOutput, "level=INFO") {
			t.Errorf("expected INFO log, got: %s", logOutput)
		}
		if !strings.Contains(logOutput, "tool=test_tool") {
			t.Errorf("expected tool=test_tool in log, got: %s", logOutput)
		}
		if !strings.Contains(logOutput, "duration_ms=") {
			t.Errorf("expected duration_ms in log, got: %s", logOutput)
		}
	})

	t.Run("logs error with duration", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))

		inner := func(params map[string]any) (any, error) {
			return nil, fmt.Errorf("something failed")
		}
		wrapped := withLogging(logger, "fail_tool", inner)

		_, err := wrapped(map[string]any{})
		if err == nil {
			t.Fatal("expected error")
		}

		logOutput := buf.String()
		if !strings.Contains(logOutput, "level=ERROR") {
			t.Errorf("expected ERROR log, got: %s", logOutput)
		}
		if !strings.Contains(logOutput, "tool=fail_tool") {
			t.Errorf("expected tool=fail_tool in log, got: %s", logOutput)
		}
		if !strings.Contains(logOutput, "something failed") {
			t.Errorf("expected error message in log, got: %s", logOutput)
		}
	})
}

func TestWithRole(t *testing.T) {
	called := false
	inner := func(params map[string]any) (any, error) {
		called = true
		return "ok", nil
	}

	t.Run("permits authorized agent", func(t *testing.T) {
		called = false
		checker := RoleChecker(func(agentID string) error { return nil })
		wrapped := withRole(inner, checker)

		result, err := wrapped(map[string]any{"agent_id": "coder-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !called {
			t.Error("expected inner handler to be called")
		}
		if result != "ok" {
			t.Errorf("expected 'ok', got %v", result)
		}
	})

	t.Run("blocks unauthorized agent", func(t *testing.T) {
		called = false
		checker := RoleChecker(func(agentID string) error {
			return fmt.Errorf("requires orchestrator role")
		})
		wrapped := withRole(inner, checker)

		_, err := wrapped(map[string]any{"agent_id": "coder-1"})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "requires orchestrator role") {
			t.Errorf("expected role error, got: %v", err)
		}
		if called {
			t.Error("inner handler should not be called when role check fails")
		}
	})

	t.Run("rejects missing agent_id", func(t *testing.T) {
		called = false
		checker := RoleChecker(func(agentID string) error { return nil })
		wrapped := withRole(inner, checker)

		_, err := wrapped(map[string]any{})
		if err == nil {
			t.Fatal("expected error for missing agent_id")
		}
		if !strings.Contains(err.Error(), "agent_id parameter required") {
			t.Errorf("expected 'agent_id parameter required' error, got: %v", err)
		}
		if called {
			t.Error("inner handler should not be called when agent_id missing")
		}
	})
}

func TestMcpToolToOperation(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"liza_submit_for_review", "submit-for-review"},
		{"liza_add_tasks", "add-tasks"},
		{"liza_analyze", "analyze"},
		{"liza_clear_stale_review_claims", "clear-stale-review-claims"},
		{"liza_wt_create", "wt-create"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mcpToolToOperation(tt.input)
			if got != tt.want {
				t.Errorf("mcpToolToOperation(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// testMiddlewarePipelineYAML is a minimal pipeline config for middleware tests.
// It defines a coder doer role with submit-for-review in allowed-operations,
// a code-reviewer reviewer role with submit-verdict, and an orchestrator with add-tasks.
var testMiddlewarePipelineYAML = []byte(`
pipeline:
  roles:
    coder:
      type: doer
      display-name: "Coder"
      allowed-operations:
        - submit-for-review
        - write-checkpoint
        - handoff
    code-reviewer:
      type: reviewer
      display-name: "Code Reviewer"
      allowed-operations:
        - submit-verdict
    orchestrator:
      type: orchestrator
      display-name: "Orchestrator"
      allowed-operations:
        - add-tasks
        - supersede-task
        - analyze
  role-pairs:
    coding-pair:
      doer: coder
      reviewer: code-reviewer
      states:
        initial: READY
        executing: CODING
        submitted: CODE_SUBMITTED
        reviewing: CODE_REVIEWING
        approved: CODE_APPROVED
        rejected: CODE_REJECTED
  sub-pipelines:
    coding:
      steps:
        - coding-pair
      transitions: []
  entry-points:
    default: coding.coding-pair
`)

func testMiddlewareResolver(t *testing.T) *pipeline.Resolver {
	t.Helper()
	cfg, err := pipeline.LoadFromBytes(testMiddlewarePipelineYAML)
	if err != nil {
		t.Fatalf("testMiddlewareResolver: %v", err)
	}
	return pipeline.NewResolver(cfg)
}

func TestIsOperationAllowed(t *testing.T) {
	resolver := testMiddlewareResolver(t)

	t.Run("allows coder submit-for-review", func(t *testing.T) {
		err := isOperationAllowed(resolver, nil, "coder-1", "liza_submit_for_review")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("rejects coder add-tasks", func(t *testing.T) {
		err := isOperationAllowed(resolver, nil, "coder-1", "liza_add_tasks")
		if err == nil {
			t.Fatal("expected error")
		}
		var opErr *OperationError
		if !errors.As(err, &opErr) {
			t.Fatalf("expected OperationError, got %T: %v", err, err)
		}
		if opErr.Operation != "add-tasks" {
			t.Errorf("operation = %q, want %q", opErr.Operation, "add-tasks")
		}
		if opErr.Role != "coder" {
			t.Errorf("role = %q, want %q", opErr.Role, "coder")
		}
	})

	t.Run("allows orchestrator add-tasks", func(t *testing.T) {
		err := isOperationAllowed(resolver, nil, "orchestrator-1", "liza_add_tasks")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("rejects reviewer submit-for-review", func(t *testing.T) {
		err := isOperationAllowed(resolver, nil, "code-reviewer-1", "liza_submit_for_review")
		if err == nil {
			t.Fatal("expected error")
		}
		var opErr *OperationError
		if !errors.As(err, &opErr) {
			t.Fatalf("expected OperationError, got %T: %v", err, err)
		}
	})

	t.Run("rejects invalid agent ID", func(t *testing.T) {
		err := isOperationAllowed(resolver, nil, "invalid", "liza_submit_for_review")
		if err == nil {
			t.Fatal("expected error for invalid agent ID")
		}
		if !strings.Contains(err.Error(), "invalid agent ID") {
			t.Errorf("expected 'invalid agent ID' error, got: %v", err)
		}
	})

	t.Run("rejects nil resolver", func(t *testing.T) {
		err := isOperationAllowed(nil, nil, "coder-1", "liza_submit_for_review")
		if err == nil {
			t.Fatal("expected error for nil resolver")
		}
		if !strings.Contains(err.Error(), "pipeline resolver not loaded") {
			t.Errorf("expected 'pipeline resolver not loaded' error, got: %v", err)
		}
	})
}

func TestOperationChecker(t *testing.T) {
	resolver := testMiddlewareResolver(t)

	t.Run("returns nil for allowed operation", func(t *testing.T) {
		checker := operationChecker(resolver, nil, "liza_submit_for_review")
		err := checker("coder-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("returns OperationError for disallowed", func(t *testing.T) {
		checker := operationChecker(resolver, nil, "liza_add_tasks")
		err := checker("coder-1")
		if err == nil {
			t.Fatal("expected error")
		}
		var opErr *OperationError
		if !errors.As(err, &opErr) {
			t.Fatalf("expected OperationError, got %T: %v", err, err)
		}
	})
}

func TestTypeChecker(t *testing.T) {
	resolver := testMiddlewareResolver(t)

	t.Run("allows matching role type", func(t *testing.T) {
		checker := typeChecker(resolver, nil, "doer")
		err := checker("coder-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("rejects non-matching role type", func(t *testing.T) {
		checker := typeChecker(resolver, nil, "doer")
		err := checker("code-reviewer-1")
		if err == nil {
			t.Fatal("expected error")
		}
		var roleErr *RoleError
		if !errors.As(err, &roleErr) {
			t.Fatalf("expected RoleError, got %T: %v", err, err)
		}
		if roleErr.Got != "reviewer" {
			t.Errorf("got = %q, want %q", roleErr.Got, "reviewer")
		}
	})

	t.Run("allows multi-type match", func(t *testing.T) {
		checker := typeChecker(resolver, nil, "doer", "orchestrator")
		err := checker("orchestrator-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("rejects invalid agent ID", func(t *testing.T) {
		checker := typeChecker(resolver, nil, "doer")
		err := checker("badid")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "invalid agent ID") {
			t.Errorf("expected 'invalid agent ID' error, got: %v", err)
		}
	})

	t.Run("rejects nil resolver", func(t *testing.T) {
		checker := typeChecker(nil, nil, "doer")
		err := checker("coder-1")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "pipeline resolver not loaded") {
			t.Errorf("expected 'pipeline resolver not loaded' error, got: %v", err)
		}
	})
}
