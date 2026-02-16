package protocol

import (
	"encoding/json"
	"testing"
)

// TestParseInitializeRequest verifies we can parse MCP initialize requests
func TestParseInitializeRequest(t *testing.T) {
	rawJSON := `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "initialize",
		"params": {
			"protocolVersion": "2024-11-05",
			"capabilities": {},
			"clientInfo": {
				"name": "test-client",
				"version": "1.0.0"
			}
		}
	}`

	var req JSONRPCRequest
	if err := json.Unmarshal([]byte(rawJSON), &req); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if req.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC 2.0, got %s", req.JSONRPC)
	}

	if req.Method != "initialize" {
		t.Errorf("Expected method initialize, got %s", req.Method)
	}

	params, ok := req.Params.(map[string]any)
	if !ok {
		t.Fatal("Expected params to be map")
	}

	if params["protocolVersion"] != "2024-11-05" {
		t.Errorf("Unexpected protocol version: %v", params["protocolVersion"])
	}
}

// TestSerializeInitializeResponse verifies we can serialize MCP initialize responses
func TestSerializeInitializeResponse(t *testing.T) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools":     map[string]any{},
				"resources": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "liza-mcp",
				"version": "0.1.0",
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if parsed["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc 2.0, got %v", parsed["jsonrpc"])
	}

	result, ok := parsed["result"].(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("Unexpected protocol version in result")
	}
}

// TestParseToolCallRequest verifies we can parse tools/call requests
func TestParseToolCallRequest(t *testing.T) {
	rawJSON := `{
		"jsonrpc": "2.0",
		"id": 2,
		"method": "tools/call",
		"params": {
			"name": "liza_get",
			"arguments": {
				"query": "tasks"
			}
		}
	}`

	var req JSONRPCRequest
	if err := json.Unmarshal([]byte(rawJSON), &req); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if req.Method != "tools/call" {
		t.Errorf("Expected method tools/call, got %s", req.Method)
	}

	params, ok := req.Params.(map[string]any)
	if !ok {
		t.Fatal("Expected params to be map")
	}

	if params["name"] != "liza_get" {
		t.Errorf("Expected tool name liza_get, got %v", params["name"])
	}

	args, ok := params["arguments"].(map[string]any)
	if !ok {
		t.Fatal("Expected arguments to be map")
	}

	if args["query"] != "tasks" {
		t.Errorf("Expected query tasks, got %v", args["query"])
	}
}

// TestSerializeToolCallResponse verifies we can serialize tools/call responses
func TestSerializeToolCallResponse(t *testing.T) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Result: map[string]any{
			"content": []any{
				map[string]any{
					"type": "text",
					"text": "Task list result",
				},
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	result, ok := parsed["result"].(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	content, ok := result["content"].([]any)
	if !ok {
		t.Fatal("Expected content to be array")
	}

	if len(content) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(content))
	}
}

// TestErrorSerialization verifies we can serialize JSON-RPC errors
func TestErrorSerialization(t *testing.T) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Error: &JSONRPCError{
			Code:    -32602,
			Message: "Invalid params",
			Data: map[string]any{
				"field": "query",
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	errObj, ok := parsed["error"].(map[string]any)
	if !ok {
		t.Fatal("Expected error to be map")
	}

	if errObj["code"].(float64) != -32602 {
		t.Errorf("Expected error code -32602, got %v", errObj["code"])
	}

	if errObj["message"] != "Invalid params" {
		t.Errorf("Expected message 'Invalid params', got %v", errObj["message"])
	}
}
