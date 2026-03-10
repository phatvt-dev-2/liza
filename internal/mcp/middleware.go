package mcp

import (
	"log/slog"
	"time"
)

// RoleChecker validates that an agent_id has the required role.
type RoleChecker func(agentID string) error

// withLogging wraps a ToolHandler with timing and success/error logging.
func withLogging(logger *slog.Logger, name string, handler ToolHandler) ToolHandler {
	return func(params map[string]any) (any, error) {
		start := time.Now()
		result, err := handler(params)
		duration := time.Since(start).Milliseconds()
		if err != nil {
			logger.Error("mcp", "tool", name, "duration_ms", duration, "error", err.Error())
		} else {
			logger.Info("mcp", "tool", name, "duration_ms", duration)
		}
		return result, err
	}
}

// withRole wraps a ToolHandler with role validation.
// It extracts agent_id from params and validates it using the provided checker.
func withRole(handler ToolHandler, checker RoleChecker) ToolHandler {
	return func(params map[string]any) (any, error) {
		agentID, err := requireString(params, "agent_id")
		if err != nil {
			return nil, err
		}
		if err := checker(agentID); err != nil {
			return nil, err
		}
		return handler(params)
	}
}
