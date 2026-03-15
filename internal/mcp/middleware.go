package mcp

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/identity"
	"github.com/liza-mas/liza/internal/pipeline"
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

// OperationError indicates an agent's role does not permit the requested operation.
type OperationError struct {
	Operation string
	Role      string
	AgentID   string
}

func (e *OperationError) Error() string {
	return fmt.Sprintf("operation %s not allowed for role %s (agent %s)", e.Operation, e.Role, e.AgentID)
}

// mcpToolToOperation converts an MCP tool name to a YAML operation name.
// Example: "liza_submit_for_review" → "submit-for-review"
func mcpToolToOperation(toolName string) string {
	op := strings.TrimPrefix(toolName, "liza_")
	return strings.ReplaceAll(op, "_", "-")
}

// isOperationAllowed checks whether the agent identified by agentID is permitted
// to invoke the given MCP tool, based on the pipeline YAML allowed-operations.
// Returns nil if allowed, OperationError if not.
func isOperationAllowed(resolver *pipeline.Resolver, agentID, mcpToolName string) error {
	if err := identity.ValidateFormat(agentID); err != nil {
		return fmt.Errorf("invalid agent ID %q: %w", agentID, err)
	}
	if resolver == nil {
		return fmt.Errorf("pipeline resolver not loaded — cannot authorize operations")
	}

	role, _ := identity.ExtractRole(agentID)
	operation := mcpToolToOperation(mcpToolName)

	ops, err := resolver.AllowedOperations(role)
	if err != nil {
		return &OperationError{Operation: operation, Role: role, AgentID: agentID}
	}

	if !slices.Contains(ops, operation) {
		return &OperationError{Operation: operation, Role: role, AgentID: agentID}
	}
	return nil
}

// operationChecker returns a RoleChecker that validates the agent's role has the
// given MCP tool in its YAML allowed-operations list. This is the declarative
// replacement for the hardcoded requireDoerRole/requireReviewerRole helpers.
func operationChecker(resolver *pipeline.Resolver, mcpToolName string) RoleChecker {
	return func(agentID string) error {
		return isOperationAllowed(resolver, agentID, mcpToolName)
	}
}

// typeChecker returns a RoleChecker that validates the agent's role type
// (doer, reviewer, orchestrator) matches one of the allowed types.
// This is the declarative replacement for tools that need role-type
// authorization but don't have explicit allowed-operations entries.
func typeChecker(resolver *pipeline.Resolver, allowedTypes ...string) RoleChecker {
	return func(agentID string) error {
		if err := identity.ValidateFormat(agentID); err != nil {
			return fmt.Errorf("invalid agent ID %q: %w", agentID, err)
		}
		if resolver == nil {
			return fmt.Errorf("pipeline resolver not loaded — cannot authorize operations")
		}
		role, _ := identity.ExtractRole(agentID)
		roleType, err := resolver.RoleType(role)
		if err != nil {
			return &RoleError{Expected: allowedTypes, Got: role, AgentID: agentID}
		}
		if !slices.Contains(allowedTypes, roleType) {
			return &RoleError{Expected: allowedTypes, Got: roleType, AgentID: agentID}
		}
		return nil
	}
}
