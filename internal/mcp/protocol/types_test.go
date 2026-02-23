package protocol

import (
	"testing"
)

// TestToolDefinitionJSON verifies tool schema serialization
func TestToolDefinitionJSON(t *testing.T) {
	tool := Tool{
		Name:        "liza_get",
		Description: "Query Liza state",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"query": {
					Type:        "string",
					Description: "Query path (e.g., 'tasks', 'agents')",
				},
			},
			Required: []string{"query"},
		},
	}

	parsed := mustMarshalToMap(t, tool)

	if parsed["name"] != "liza_get" {
		t.Errorf("Expected name liza_get, got %v", parsed["name"])
	}

	schema := mustGetMap(t, parsed, "inputSchema")
	if schema["type"] != "object" {
		t.Errorf("Expected type object, got %v", schema["type"])
	}

	props := mustGetMap(t, schema, "properties")
	queryProp := mustGetMap(t, props, "query")

	if queryProp["type"] != "string" {
		t.Errorf("Expected query type string, got %v", queryProp["type"])
	}
}

// TestResourceDefinitionJSON verifies resource schema serialization
func TestResourceDefinitionJSON(t *testing.T) {
	resource := Resource{
		URI:         "liza://state",
		Name:        "Current State",
		Description: "Current workspace state (state.yaml)",
		MimeType:    "application/x-yaml",
	}

	parsed := mustMarshalToMap(t, resource)

	if parsed["uri"] != "liza://state" {
		t.Errorf("Expected uri liza://state, got %v", parsed["uri"])
	}

	if parsed["mimeType"] != "application/x-yaml" {
		t.Errorf("Expected mimeType application/x-yaml, got %v", parsed["mimeType"])
	}
}

// TestMCPErrorJSON verifies error structure serialization
func TestMCPErrorJSON(t *testing.T) {
	mcpErr := JSONRPCError{
		Code:    -32001,
		Message: "Lock acquisition timeout",
		Data: map[string]any{
			"type":    "lock_timeout",
			"timeout": 30,
		},
	}

	parsed := mustMarshalToMap(t, mcpErr)

	if parsed["code"].(float64) != -32001 {
		t.Errorf("Expected code -32001, got %v", parsed["code"])
	}

	if parsed["message"] != "Lock acquisition timeout" {
		t.Errorf("Expected message 'Lock acquisition timeout', got %v", parsed["message"])
	}

	dataField := mustGetMap(t, parsed, "data")
	if dataField["type"] != "lock_timeout" {
		t.Errorf("Expected type lock_timeout, got %v", dataField["type"])
	}
}
