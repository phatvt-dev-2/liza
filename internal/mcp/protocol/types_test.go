package protocol

import (
	"encoding/json"
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

	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if parsed["name"] != "liza_get" {
		t.Errorf("Expected name liza_get, got %v", parsed["name"])
	}

	schema, ok := parsed["inputSchema"].(map[string]any)
	if !ok {
		t.Fatal("Expected inputSchema to be map")
	}

	if schema["type"] != "object" {
		t.Errorf("Expected type object, got %v", schema["type"])
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties to be map")
	}

	queryProp, ok := props["query"].(map[string]any)
	if !ok {
		t.Fatal("Expected query property to be map")
	}

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

	data, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

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

	data, err := json.Marshal(mcpErr)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if parsed["code"].(float64) != -32001 {
		t.Errorf("Expected code -32001, got %v", parsed["code"])
	}

	if parsed["message"] != "Lock acquisition timeout" {
		t.Errorf("Expected message 'Lock acquisition timeout', got %v", parsed["message"])
	}

	dataField, ok := parsed["data"].(map[string]any)
	if !ok {
		t.Fatal("Expected data to be map")
	}

	if dataField["type"] != "lock_timeout" {
		t.Errorf("Expected type lock_timeout, got %v", dataField["type"])
	}
}
