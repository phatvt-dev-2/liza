package mcp

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"
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
