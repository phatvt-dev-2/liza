package protocol

import (
	"encoding/json"
	"testing"
)

// mustUnmarshalRequest parses raw JSON into a JSONRPCRequest, failing the test on error.
func mustUnmarshalRequest(t *testing.T, raw string) JSONRPCRequest {
	t.Helper()
	var req JSONRPCRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	return req
}

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

	req := mustUnmarshalRequest(t, rawJSON)

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

	parsed := mustMarshalToMap(t, resp)

	if parsed["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc 2.0, got %v", parsed["jsonrpc"])
	}

	result := mustGetMap(t, parsed, "result")
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

	req := mustUnmarshalRequest(t, rawJSON)

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

	args := mustGetMap(t, params, "arguments")
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

	parsed := mustMarshalToMap(t, resp)
	result := mustGetMap(t, parsed, "result")
	content := mustGetSlice(t, result, "content")

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

	parsed := mustMarshalToMap(t, resp)
	errObj := mustGetMap(t, parsed, "error")

	if errObj["code"].(float64) != -32602 {
		t.Errorf("Expected error code -32602, got %v", errObj["code"])
	}

	if errObj["message"] != "Invalid params" {
		t.Errorf("Expected message 'Invalid params', got %v", errObj["message"])
	}
}
