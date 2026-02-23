package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
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

// TestReadRequest_NormalRequest verifies normal requests are read correctly
func TestReadRequest_NormalRequest(t *testing.T) {
	rawJSON := `{"jsonrpc":"2.0","id":1,"method":"test"}` + "\n"
	transport := newStdioTransport(strings.NewReader(rawJSON), io.Discard)

	req, err := transport.ReadRequest()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if req.Method != "test" {
		t.Errorf("Expected method 'test', got %s", req.Method)
	}
	if req.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got %s", req.JSONRPC)
	}
}

// TestReadRequest_OversizedRequest verifies oversized requests return ErrRequestTooLarge
func TestReadRequest_OversizedRequest(t *testing.T) {
	// Create a request larger than MaxRequestSize
	largePayload := make([]byte, MaxRequestSize+100)
	for i := range largePayload {
		largePayload[i] = 'x'
	}
	// Add newline at the end so it looks like a valid line
	largePayload[len(largePayload)-1] = '\n'

	transport := newStdioTransport(bytes.NewReader(largePayload), io.Discard)

	_, err := transport.ReadRequest()
	if !errors.Is(err, ErrRequestTooLarge) {
		t.Fatalf("Expected ErrRequestTooLarge, got %v", err)
	}
}

// TestReadRequest_AtSizeLimit verifies requests at exactly the limit work
func TestReadRequest_AtSizeLimit(t *testing.T) {
	// Build a valid JSON request that is exactly MaxRequestSize bytes (including newline)
	baseRequest := `{"jsonrpc":"2.0","id":999,"method":"test","params":{"p":"` + strings.Repeat("x", MaxRequestSize) + `"}}` + "\n"
	// Trim padding to hit exact size
	overhead := len(baseRequest) - MaxRequestSize
	paddingSize := MaxRequestSize - overhead
	if paddingSize < 0 {
		t.Skip("MaxRequestSize too small for this test")
	}
	request := `{"jsonrpc":"2.0","id":999,"method":"test","params":{"p":"` + strings.Repeat("x", paddingSize) + `"}}` + "\n"

	if len(request) != MaxRequestSize+1 { // +1 for trailing newline which scanner strips
		// Adjust
		diff := MaxRequestSize + 1 - len(request)
		request = `{"jsonrpc":"2.0","id":999,"method":"test","params":{"p":"` + strings.Repeat("x", paddingSize+diff) + `"}}` + "\n"
	}

	transport := newStdioTransport(strings.NewReader(request), io.Discard)

	req, err := transport.ReadRequest()
	if err != nil {
		t.Fatalf("Expected no error for request at size limit, got %v", err)
	}
	if req.Method != "test" {
		t.Errorf("Expected method 'test', got %s", req.Method)
	}
}

// TestReadRequest_AtSizeLimitEOFTerminated verifies max-size EOF-terminated requests work
func TestReadRequest_AtSizeLimitEOFTerminated(t *testing.T) {
	// Build a valid JSON request at MaxRequestSize bytes, terminated by EOF (no newline)
	baseJSON := `{"jsonrpc":"2.0","id":1,"method":"test","params":{"p":"` + strings.Repeat("x", MaxRequestSize) + `"}}`
	overhead := len(baseJSON) - MaxRequestSize
	paddingSize := MaxRequestSize - overhead
	if paddingSize < 0 {
		t.Skip("MaxRequestSize too small for this test")
	}
	request := `{"jsonrpc":"2.0","id":1,"method":"test","params":{"p":"` + strings.Repeat("x", paddingSize) + `"}}`

	if len(request) != MaxRequestSize {
		diff := MaxRequestSize - len(request)
		request = `{"jsonrpc":"2.0","id":1,"method":"test","params":{"p":"` + strings.Repeat("x", paddingSize+diff) + `"}}`
	}

	// No trailing newline — EOF terminates
	transport := newStdioTransport(strings.NewReader(request), io.Discard)

	req, err := transport.ReadRequest()
	if err != nil {
		t.Fatalf("Expected no error for EOF-terminated max-size request, got %v", err)
	}
	if req.Method != "test" {
		t.Errorf("Expected method 'test', got %s", req.Method)
	}
}

// TestReadRequest_ErrorResponseAfterOversized verifies we can write error after oversized request
func TestReadRequest_ErrorResponseAfterOversized(t *testing.T) {
	// Create a request larger than MaxRequestSize
	largePayload := make([]byte, MaxRequestSize+100)
	for i := range largePayload {
		largePayload[i] = 'x'
	}
	largePayload[len(largePayload)-1] = '\n'

	var output bytes.Buffer
	transport := newStdioTransport(bytes.NewReader(largePayload), &output)

	// Read should fail with ErrRequestTooLarge
	_, err := transport.ReadRequest()
	if !errors.Is(err, ErrRequestTooLarge) {
		t.Fatalf("Expected ErrRequestTooLarge, got %v", err)
	}

	// We should be able to write an error response
	err = transport.WriteError(nil, RequestTooLarge, "Request exceeds maximum allowed size", map[string]any{
		"maxSize": MaxRequestSize,
		"error":   "request_too_large",
	})
	if err != nil {
		t.Fatalf("Failed to write error response: %v", err)
	}

	// Verify the error response was written
	outputStr := output.String()
	if !strings.Contains(outputStr, `"code":-32005`) {
		t.Errorf("Expected error code -32005 in output, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, `"message":"Request exceeds maximum allowed size"`) {
		t.Errorf("Expected error message in output, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, `"error":"request_too_large"`) {
		t.Errorf("Expected error data in output, got: %s", outputStr)
	}
}

// TestReadRequest_MultipleRequests verifies multiple sequential requests work
func TestReadRequest_MultipleRequests(t *testing.T) {
	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"method1"}`,
		`{"jsonrpc":"2.0","id":2,"method":"method2"}`,
		`{"jsonrpc":"2.0","id":3,"method":"method3"}`,
	}
	input := strings.Join(requests, "\n") + "\n"

	transport := newStdioTransport(strings.NewReader(input), io.Discard)

	for i, expectedMethod := range []string{"method1", "method2", "method3"} {
		req, err := transport.ReadRequest()
		if err != nil {
			t.Fatalf("Request %d: Expected no error, got %v", i+1, err)
		}
		if req.Method != expectedMethod {
			t.Errorf("Request %d: Expected method '%s', got '%s'", i+1, expectedMethod, req.Method)
		}
		if req.JSONRPC != "2.0" {
			t.Errorf("Request %d: Expected JSONRPC '2.0', got '%s'", i+1, req.JSONRPC)
		}
	}
}

// TestReadRequest_EOFHandling verifies EOF is handled correctly
func TestReadRequest_EOFHandling(t *testing.T) {
	// Empty input should return EOF
	transport := newStdioTransport(strings.NewReader(""), io.Discard)

	_, err := transport.ReadRequest()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("Expected io.EOF for empty input, got %v", err)
	}
}

// TestReadRequest_InvalidJSONAfterSizeCheck verifies invalid JSON is still detected
func TestReadRequest_InvalidJSONAfterSizeCheck(t *testing.T) {
	input := "not valid json\n"
	transport := newStdioTransport(strings.NewReader(input), io.Discard)

	_, err := transport.ReadRequest()
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
	if errors.Is(err, ErrRequestTooLarge) {
		t.Error("Should not get ErrRequestTooLarge for small invalid JSON")
	}
}

// TestReadRequest_SizeExactlyAtLimitPlusOne verifies limit enforcement
func TestReadRequest_SizeExactlyAtLimitPlusOne(t *testing.T) {
	// Create a request that is exactly MaxRequestSize + 1 bytes (excluding newline)
	baseRequest := `{"jsonrpc":"2.0","id":999,"method":"test"}`
	paddingSize := MaxRequestSize - len(baseRequest) // Will be +1 after adding extra char
	if paddingSize < 0 {
		t.Skip("MaxRequestSize too small for this test")
	}

	params := strings.Repeat("x", paddingSize-10)
	request := `{"jsonrpc":"2.0","id":999,"method":"test","params":{"p":"` + params + `"}}` + "\n"

	// Verify this is exactly MaxRequestSize + 1 + 1 (content + newline)
	if len(request) != MaxRequestSize+2 {
		diff := MaxRequestSize + 2 - len(request)
		params = strings.Repeat("x", paddingSize-10+diff)
		request = `{"jsonrpc":"2.0","id":999,"method":"test","params":{"p":"` + params + `"}}` + "\n"
	}

	transport := newStdioTransport(strings.NewReader(request), io.Discard)

	_, err := transport.ReadRequest()
	if !errors.Is(err, ErrRequestTooLarge) {
		t.Fatalf("Expected ErrRequestTooLarge for request at MaxRequestSize+1, got %v", err)
	}
}
