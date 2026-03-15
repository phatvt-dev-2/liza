package mcp

import (
	"fmt"
	"strings"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/paths"
)

// handleGet implements the liza_get tool
// Maps to: liza get <query>
func (s *Server) handleGet(params map[string]any) (any, error) {
	query, err := requireString(params, "query")
	if err != nil {
		return nil, err
	}

	format := "json"
	if f, ok := params["format"].(string); ok && f != "" {
		format = f
	}
	// Normalize "text" → "value": MCP schema uses "text" (natural for agents),
	// but the inspect backend uses "value" as the canonical format name.
	if format == "text" {
		format = "value"
	}

	opts := commands.InspectOptions{
		Format:      format,
		ProjectRoot: s.projectRoot,
		Internal:    false, // Get formatted output
	}

	// Split on "/" so "tasks/<id>" becomes ["tasks", "<id>"]
	args := strings.SplitN(query, "/", 2)

	result, err := commands.InspectCommand(args, opts)
	if err != nil {
		return nil, fmt.Errorf("inspect command failed: %w", err)
	}

	return textResult(result)
}

// handleStatus implements the liza_status tool
// Maps to: liza status
func (s *Server) handleStatus(params map[string]any) (any, error) {
	opts := commands.StatusOptions{
		ProjectRoot: s.projectRoot,
	}

	result, err := commands.StatusCommand(opts)
	if err != nil {
		return nil, fmt.Errorf("status command failed: %w", err)
	}

	return textResult(result)
}

// handleValidate implements the liza_validate tool
// Maps to: liza validate
func (s *Server) handleValidate(params map[string]any) (any, error) {
	statePath := paths.New(s.projectRoot).StatePath()

	skipSpecFileCheck := false
	if skip, ok := params["skip_spec_check"].(bool); ok {
		skipSpecFileCheck = skip
	}

	err := commands.ValidateCommand(statePath, skipSpecFileCheck)

	var resultText string
	if err != nil {
		resultText = fmt.Sprintf("Validation failed: %v", err)
	} else {
		resultText = "Validation passed: workspace state is consistent"
	}

	return map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": resultText,
			},
		},
		"isError": err != nil,
	}, nil
}

// handleVersion implements the liza_version tool
// Maps to: liza version
func (s *Server) handleVersion(params map[string]any) (any, error) {
	return textResult(fmt.Sprintf("liza-mcp version %s (commit: %s)", Version, BuildCommit))
}

// handleResourceReadInternal reads a resource by URI
func (s *Server) handleResourceReadInternal(uri string) (any, error) {
	switch uri {
	case "liza://state":
		return s.readStateResource()
	case "liza://tasks":
		return s.inspectResource(uri, "tasks")
	case "liza://agents":
		return s.inspectResource(uri, "agents")
	default:
		if taskID, ok := strings.CutPrefix(uri, "liza://tasks/"); ok {
			return s.inspectResource(uri, "tasks", taskID)
		}
		return nil, fmt.Errorf("unknown resource URI: %s", uri)
	}
}

// readStateResource returns the raw state.yaml content under flock protection.
func (s *Server) readStateResource() (any, error) {
	data, err := s.bb.ReadRaw()
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	return resourceContent("liza://state", "application/x-yaml", string(data)), nil
}

// inspectResource reads a Liza resource via the inspect command.
func (s *Server) inspectResource(uri string, args ...string) (any, error) {
	opts := commands.InspectOptions{
		Format:      "json",
		ProjectRoot: s.projectRoot,
		Internal:    false,
	}
	result, err := commands.InspectCommand(args, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", uri, err)
	}
	return resourceContent(uri, "application/json", result), nil
}
