package mcp

import (
	"fmt"
	"strings"

	"github.com/liza-mas/liza/internal/identity"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/roles"
)

// Version and BuildCommit are set from the embedded package's build-time
// variables when the MCP server binary starts. Defaults for dev/test use.
var (
	Version     = "dev"
	BuildCommit = "unknown"
)

// textResult builds a standard MCP text content response.
func textResult(msg string) (any, error) {
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": msg,
			},
		},
	}, nil
}

// resourceContent builds a standard MCP resource content response.
func resourceContent(uri, mimeType, text string) any {
	return map[string]any{
		"contents": []any{
			map[string]any{
				"uri":      uri,
				"mimeType": mimeType,
				"text":     text,
			},
		},
	}
}

// requireString extracts a required non-empty string parameter.
func requireString(params map[string]any, key string) (string, error) {
	v, ok := params[key].(string)
	if !ok || v == "" {
		return "", fmt.Errorf("%s parameter required", key)
	}
	return v, nil
}

// extractStringSlice extracts an optional []string from a JSON params map.
// JSON arrays arrive as []any; non-string elements are silently skipped.
func extractStringSlice(params map[string]any, key string) []string {
	raw, ok := params[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// extractScopeExtensions extracts an optional []ScopeExtensionEntry from a JSON params map.
// JSON arrays arrive as []any of map[string]any; malformed entries are silently skipped.
func extractScopeExtensions(params map[string]any, key string) []ops.ScopeExtensionEntry {
	raw, ok := params[key].([]any)
	if !ok {
		return nil
	}
	out := make([]ops.ScopeExtensionEntry, 0, len(raw))
	for _, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		file, _ := m["file"].(string)
		justification, _ := m["justification"].(string)
		if file != "" && justification != "" {
			out = append(out, ops.ScopeExtensionEntry{
				File:          file,
				Justification: justification,
			})
		}
	}
	return out
}

// appendWarnings appends warning lines to a message string.
func appendWarnings(msg string, warnings []string) string {
	if len(warnings) == 0 {
		return msg
	}
	var b strings.Builder
	b.WriteString(msg)
	for _, w := range warnings {
		b.WriteString("\nWarning: ")
		b.WriteString(w)
	}
	return b.String()
}

// requireTaskAndAgent extracts the common task_id + agent_id pair.
func requireTaskAndAgent(params map[string]any) (taskID, agentID string, err error) {
	taskID, err = requireString(params, "task_id")
	if err != nil {
		return "", "", err
	}
	agentID, err = requireString(params, "agent_id")
	if err != nil {
		return "", "", err
	}
	return taskID, agentID, nil
}

// RoleError indicates an agent does not have the required role for an operation.
// The message is intentionally client-facing so agents receive actionable feedback.
type RoleError struct {
	Expected []string
	Got      string
	AgentID  string
}

func (e *RoleError) Error() string {
	return fmt.Sprintf("requires one of %v roles (got %s from %s)", e.Expected, e.Got, e.AgentID)
}

// requireRole validates agent ID format and that it matches the expected runtime role.
func requireRole(agentID, expectedRole string) error {
	if err := identity.ValidateFormat(agentID); err != nil {
		return fmt.Errorf("invalid agent ID %q: %w", agentID, err)
	}
	role, _ := identity.ExtractRole(agentID) // cannot fail after ValidateFormat
	if role != expectedRole {
		return &RoleError{Expected: []string{expectedRole}, Got: role, AgentID: agentID}
	}
	return nil
}

// requireDoerRole validates agent ID format and that it has a doer role.
func requireDoerRole(agentID string) error {
	if err := identity.ValidateFormat(agentID); err != nil {
		return fmt.Errorf("invalid agent ID %q: %w", agentID, err)
	}
	role, _ := identity.ExtractRole(agentID)
	if !roles.IsDoerRole(role) {
		return &RoleError{Expected: roles.DoerRoles(), Got: role, AgentID: agentID}
	}
	return nil
}

// requireDoerOrOrchestratorRole validates agent ID format and that it has a doer or orchestrator role.
// Used for operations like wt-delete that doers perform on their own tasks
// but the orchestrator also performs for cleanup of superseded/blocked tasks.
func requireDoerOrOrchestratorRole(agentID string) error {
	if err := identity.ValidateFormat(agentID); err != nil {
		return fmt.Errorf("invalid agent ID %q: %w", agentID, err)
	}
	role, _ := identity.ExtractRole(agentID)
	if roles.IsDoerRole(role) || role == roles.RuntimeOrchestrator {
		return nil
	}
	doers := roles.DoerRoles()
	allowed := make([]string, len(doers)+1)
	copy(allowed, doers)
	allowed[len(doers)] = roles.RuntimeOrchestrator
	return &RoleError{Expected: allowed, Got: role, AgentID: agentID}
}

// requireReviewerRole validates agent ID format and that it has a reviewer role.
func requireReviewerRole(agentID string) error {
	if err := identity.ValidateFormat(agentID); err != nil {
		return fmt.Errorf("invalid agent ID %q: %w", agentID, err)
	}
	role, _ := identity.ExtractRole(agentID)
	if !roles.IsReviewerRole(role) {
		return &RoleError{Expected: roles.ReviewerRoles(), Got: role, AgentID: agentID}
	}
	return nil
}

// authorizeClaimRelease validates that the agent's runtime role is authorized
// to release the requested claim type. Orchestrator can release any claim;
// others can only release claims matching their own role category.
func authorizeClaimRelease(agentID, claimRole string) error {
	if err := identity.ValidateFormat(agentID); err != nil {
		return fmt.Errorf("invalid agent ID %q: %w", agentID, err)
	}
	agentRole, _ := identity.ExtractRole(agentID)
	switch agentRole {
	case roles.RuntimeOrchestrator:
		return nil
	case roles.RuntimeCoder, roles.RuntimeCodePlanner:
		if claimRole != roles.ClaimDoer {
			return fmt.Errorf("agent %s (role %s) can only release doer claims", agentID, agentRole)
		}
	case roles.RuntimeCodeReviewer, roles.RuntimeCodePlanReviewer:
		if claimRole != roles.ClaimReviewer {
			return fmt.Errorf("agent %s (role %s) can only release reviewer claims", agentID, agentRole)
		}
	default:
		return fmt.Errorf("agent %s has unrecognized role %q for claim release", agentID, agentRole)
	}
	return nil
}

// extractTaskInputs converts a raw JSON array into []ops.AddTaskInput.
// Returns indexed errors for malformed elements.
func extractTaskInputs(raw []any) ([]ops.AddTaskInput, error) {
	out := make([]ops.AddTaskInput, 0, len(raw))
	for i, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("tasks[%d]: must be an object, got %T", i, v)
		}

		id := stringFromMap(m, "id")
		if id == "" {
			return nil, fmt.Errorf("tasks[%d]: missing required field 'id'", i)
		}
		desc := stringFromMap(m, "desc")
		if desc == "" {
			return nil, fmt.Errorf("tasks[%d]: missing required field 'desc'", i)
		}
		spec := stringFromMap(m, "spec")
		if spec == "" {
			return nil, fmt.Errorf("tasks[%d]: missing required field 'spec'", i)
		}
		done := stringFromMap(m, "done")
		if done == "" {
			return nil, fmt.Errorf("tasks[%d]: missing required field 'done'", i)
		}
		scope := stringFromMap(m, "scope")
		if scope == "" {
			return nil, fmt.Errorf("tasks[%d]: missing required field 'scope'", i)
		}

		priority := 1
		if p, ok := m["priority"].(float64); ok {
			priority = int(p)
		} else if p, ok := m["priority"].(int); ok {
			priority = p
		}

		depends := extractStringSlice(m, "depends")
		taskType := stringFromMap(m, "type")
		rolePair := stringFromMap(m, "role_pair")

		out = append(out, ops.AddTaskInput{
			ID:          id,
			Type:        taskType,
			RolePair:    rolePair,
			Description: desc,
			SpecRef:     spec,
			DoneWhen:    done,
			Scope:       scope,
			Priority:    priority,
			DependsOn:   depends,
		})
	}
	return out, nil
}

// formatAddTasksResult builds a human-readable summary of batch results.
func formatAddTasksResult(result *ops.AddTasksResult) string {
	succeeded := 0
	for _, r := range result.Results {
		if r.Success {
			succeeded++
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Added %d/%d tasks", succeeded, len(result.Results))
	for _, r := range result.Results {
		if r.Success {
			fmt.Fprintf(&b, "\n  %s: added", r.TaskID)
			for _, w := range r.Warnings {
				fmt.Fprintf(&b, " (warning: %s)", w)
			}
		} else {
			fmt.Fprintf(&b, "\n  %s: error: %s", r.TaskID, r.Error)
		}
	}
	return b.String()
}

// extractOutputEntries converts a raw JSON array into []models.OutputEntry.
// Returns an error if any element is not an object (strict — no silent drops).
func extractOutputEntries(raw []any) ([]models.OutputEntry, error) {
	out := make([]models.OutputEntry, 0, len(raw))
	for i, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("output[%d] must be an object, got %T", i, v)
		}
		entry := models.OutputEntry{
			Desc:     stringFromMap(m, "desc"),
			DoneWhen: stringFromMap(m, "done_when"),
			Scope:    stringFromMap(m, "scope"),
			SpecRef:  stringFromMap(m, "spec_ref"),
		}
		out = append(out, entry)
	}
	return out, nil
}

// stringFromMap extracts a string value from a map, returning "" if absent or wrong type.
func stringFromMap(m map[string]any, key string) string {
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return v
}
