package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	lizaerrors "github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/mcp/protocol"
)

func reqID(id int) json.RawMessage {
	b, _ := json.Marshal(id)
	return b
}

func TestHandleRequest_Routing(t *testing.T) {
	server := NewServer("/tmp/test", "/tmp/test/.liza/log.yaml")

	tests := []struct {
		name      string
		method    string
		wantError bool
		errorCode int
	}{
		{
			name:   "initialize returns result",
			method: "initialize",
		},
		{
			name:   "tools/list returns result",
			method: "tools/list",
		},
		{
			name:   "resources/list returns result",
			method: "resources/list",
		},
		{
			name:      "unknown method returns error",
			method:    "unknown/method",
			wantError: true,
			errorCode: protocol.MethodNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &protocol.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      reqID(1),
				Method:  tt.method,
			}

			resp := server.HandleRequest(req)

			if resp.JSONRPC != "2.0" {
				t.Errorf("JSONRPC = %q, want %q", resp.JSONRPC, "2.0")
			}

			if tt.wantError {
				if resp.Error == nil {
					t.Fatal("expected error response")
				}
				if resp.Error.Code != tt.errorCode {
					t.Errorf("error code = %d, want %d", resp.Error.Code, tt.errorCode)
				}
			} else {
				if resp.Error != nil {
					t.Fatalf("unexpected error: %v", resp.Error)
				}
				if resp.Result == nil {
					t.Fatal("expected result")
				}
			}
		})
	}
}

func TestHandleRequest_Initialize(t *testing.T) {
	server := NewServer("/tmp/test", "/tmp/test/.liza/log.yaml")

	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      reqID(1),
		Method:  "initialize",
	}

	resp := server.HandleRequest(req)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("protocolVersion = %v, want 2024-11-05", result["protocolVersion"])
	}

	serverInfo, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatal("serverInfo is not a map")
	}
	if serverInfo["name"] != "liza-mcp" {
		t.Errorf("serverInfo.name = %v, want liza-mcp", serverInfo["name"])
	}
}

func TestHandleRequest_ToolCall_InvalidParams(t *testing.T) {
	server := NewServer("/tmp/test", "/tmp/test/.liza/log.yaml")

	tests := []struct {
		name   string
		params any
	}{
		{
			name:   "nil params",
			params: nil,
		},
		{
			name:   "non-object params",
			params: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &protocol.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      reqID(1),
				Method:  "tools/call",
				Params:  tt.params,
			}

			resp := server.HandleRequest(req)
			if resp.Error == nil {
				t.Fatal("expected error for invalid params")
			}
			if resp.Error.Code != protocol.InvalidParams {
				t.Errorf("error code = %d, want %d", resp.Error.Code, protocol.InvalidParams)
			}
		})
	}
}

func TestHandleRequest_ToolCall_MissingName(t *testing.T) {
	server := NewServer("/tmp/test", "/tmp/test/.liza/log.yaml")

	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      reqID(1),
		Method:  "tools/call",
		Params:  map[string]any{"arguments": map[string]any{}},
	}

	resp := server.HandleRequest(req)
	if resp.Error == nil {
		t.Fatal("expected error for missing tool name")
	}
	if resp.Error.Code != protocol.InvalidParams {
		t.Errorf("error code = %d, want %d", resp.Error.Code, protocol.InvalidParams)
	}
}

func TestHandleRequest_ToolCall_UnknownTool(t *testing.T) {
	server := NewServer("/tmp/test", "/tmp/test/.liza/log.yaml")

	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      reqID(1),
		Method:  "tools/call",
		Params: map[string]any{
			"name":      "nonexistent_tool",
			"arguments": map[string]any{},
		},
	}

	resp := server.HandleRequest(req)
	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
	if resp.Error.Code != protocol.NotFound {
		t.Errorf("error code = %d, want %d", resp.Error.Code, protocol.NotFound)
	}
}

func TestHandleRequest_ToolCall_Success(t *testing.T) {
	server := NewServer("/tmp/test", "/tmp/test/.liza/log.yaml")

	// Register a test handler
	server.registerTool(protocol.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: protocol.InputSchema{Type: "object"},
	}, func(params map[string]any) (any, error) {
		return map[string]string{"status": "ok"}, nil
	})

	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      reqID(1),
		Method:  "tools/call",
		Params: map[string]any{
			"name":      "test_tool",
			"arguments": map[string]any{"key": "value"},
		},
	}

	resp := server.HandleRequest(req)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]string)
	if !ok {
		t.Fatal("result is not a map[string]string")
	}
	if result["status"] != "ok" {
		t.Errorf("result[status] = %q, want %q", result["status"], "ok")
	}
}

func TestHandleRequest_ToolCall_NilArguments(t *testing.T) {
	server := NewServer("/tmp/test", "/tmp/test/.liza/log.yaml")

	var receivedArgs map[string]any
	server.registerTool(protocol.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: protocol.InputSchema{Type: "object"},
	}, func(params map[string]any) (any, error) {
		receivedArgs = params
		return "ok", nil
	})

	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      reqID(1),
		Method:  "tools/call",
		Params: map[string]any{
			"name": "test_tool",
			// no "arguments" key
		},
	}

	resp := server.HandleRequest(req)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if receivedArgs == nil {
		t.Fatal("handler should receive non-nil args map")
	}
	if len(receivedArgs) != 0 {
		t.Errorf("args should be empty, got %v", receivedArgs)
	}
}

func TestHandleRequest_ToolCall_HandlerError(t *testing.T) {
	server := NewServer("/tmp/test", "/tmp/test/.liza/log.yaml")

	server.registerTool(protocol.Tool{
		Name:        "failing_tool",
		Description: "A tool that fails",
		InputSchema: protocol.InputSchema{Type: "object"},
	}, func(params map[string]any) (any, error) {
		return nil, &lizaerrors.NotFoundError{Entity: "task", ID: "task-42"}
	})

	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      reqID(1),
		Method:  "tools/call",
		Params: map[string]any{
			"name": "failing_tool",
		},
	}

	resp := server.HandleRequest(req)
	if resp.Error == nil {
		t.Fatal("expected error from failing handler")
	}
	// "not found" should be classified
	if resp.Error.Code != protocol.NotFound {
		t.Errorf("error code = %d, want %d (NotFound)", resp.Error.Code, protocol.NotFound)
	}
}

func TestHandleRequest_ResourceRead_InvalidParams(t *testing.T) {
	server := NewServer("/tmp/test", "/tmp/test/.liza/log.yaml")

	tests := []struct {
		name   string
		params any
	}{
		{
			name:   "nil params",
			params: nil,
		},
		{
			name:   "non-object params",
			params: "invalid",
		},
		{
			name:   "missing uri",
			params: map[string]any{"other": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &protocol.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      reqID(1),
				Method:  "resources/read",
				Params:  tt.params,
			}

			resp := server.HandleRequest(req)
			if resp.Error == nil {
				t.Fatal("expected error for invalid params")
			}
			if resp.Error.Code != protocol.InvalidParams {
				t.Errorf("error code = %d, want %d", resp.Error.Code, protocol.InvalidParams)
			}
		})
	}
}

func TestHandleRequest_PreservesRequestID(t *testing.T) {
	server := NewServer("/tmp/test", "/tmp/test/.liza/log.yaml")

	id := reqID(42)
	req := &protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "initialize",
	}

	resp := server.HandleRequest(req)
	if string(resp.ID) != string(id) {
		t.Errorf("response ID = %s, want %s", string(resp.ID), string(id))
	}
}

func TestClassifyError(t *testing.T) {
	server := NewServer("/tmp/test", "/tmp/test/.liza/log.yaml")

	tests := []struct {
		name     string
		err      error
		wantCode int
		wantMsg  string
	}{
		// Not found (typed)
		{
			name:     "typed NotFoundError",
			err:      &lizaerrors.NotFoundError{Entity: "task", ID: "task-42"},
			wantCode: protocol.NotFound,
			wantMsg:  "resource not found",
		},
		{
			name:     "wrapped NotFoundError",
			err:      fmt.Errorf("modification function failed: %w", &lizaerrors.NotFoundError{Entity: "agent", ID: "coder-1"}),
			wantCode: protocol.NotFound,
			wantMsg:  "resource not found",
		},
		// Not found (string fallback for external errors)
		{
			name:     "string not found fallback",
			err:      errors.New("branch not found"),
			wantCode: protocol.NotFound,
			wantMsg:  "resource not found",
		},
		{
			name:     "does not exist",
			err:      errors.New("agent does not exist"),
			wantCode: protocol.NotFound,
			wantMsg:  "resource not found",
		},
		// Lock timeout
		{
			name:     "lock timeout",
			err:      errors.New("failed to acquire lock: timeout"),
			wantCode: protocol.LockTimeout,
			wantMsg:  "lock acquisition timed out",
		},
		{
			name:     "lock timed out",
			err:      errors.New("lock timed out after 10s"),
			wantCode: protocol.LockTimeout,
			wantMsg:  "lock acquisition timed out",
		},
		// Race condition
		{
			name:     "race condition",
			err:      errors.New("race condition detected"),
			wantCode: protocol.RaceCondition,
			wantMsg:  "state changed concurrently, retry",
		},
		{
			name:     "changed concurrently",
			err:      errors.New("state changed concurrently"),
			wantCode: protocol.RaceCondition,
			wantMsg:  "state changed concurrently, retry",
		},
		// Validation errors
		{
			name:     "not IMPLEMENTING",
			err:      errors.New("task is not IMPLEMENTING"),
			wantCode: protocol.ValidationError,
			wantMsg:  "validation failed: precondition not met",
		},
		{
			name:     "not REVIEWING",
			err:      errors.New("task is not REVIEWING"),
			wantCode: protocol.ValidationError,
			wantMsg:  "validation failed: precondition not met",
		},
		{
			name:     "not READY_FOR_REVIEW",
			err:      errors.New("task is not READY_FOR_REVIEW"),
			wantCode: protocol.ValidationError,
			wantMsg:  "validation failed: precondition not met",
		},
		{
			name:     "not APPROVED",
			err:      errors.New("task is not APPROVED"),
			wantCode: protocol.ValidationError,
			wantMsg:  "validation failed: precondition not met",
		},
		{
			name:     "must be",
			err:      errors.New("status must be READY"),
			wantCode: protocol.ValidationError,
			wantMsg:  "validation failed: precondition not met",
		},
		{
			name:     "is required",
			err:      errors.New("agent_id is required"),
			wantCode: protocol.ValidationError,
			wantMsg:  "validation failed: precondition not met",
		},
		{
			name:     "invalid task ID",
			err:      errors.New("invalid task ID format"),
			wantCode: protocol.ValidationError,
			wantMsg:  "validation failed: precondition not met",
		},
		// Default: internal error
		{
			name:     "generic error",
			err:      errors.New("something unexpected happened"),
			wantCode: protocol.InternalError,
			wantMsg:  "internal error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jerr := server.classifyError(tt.err)
			if jerr.Code != tt.wantCode {
				t.Errorf("code = %d, want %d", jerr.Code, tt.wantCode)
			}
			if jerr.Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", jerr.Message, tt.wantMsg)
			}
		})
	}
}

func TestClassifyError_DoesNotLeakInternalDetails(t *testing.T) {
	server := NewServer("/tmp/test", "/tmp/test/.liza/log.yaml")

	sensitiveErrors := []error{
		errors.New("task not found: secret-task-id-12345"),
		errors.New("lock timed out on /home/user/.liza/state.yaml"),
		errors.New("something unexpected at internal/commands/foo.go:42"),
	}

	for _, err := range sensitiveErrors {
		jerr := server.classifyError(err)
		if jerr.Message == err.Error() {
			t.Errorf("classifyError leaked raw error: %q", err.Error())
		}
	}
}

func TestHandleNotification(t *testing.T) {
	server := NewServer("/tmp/test", "/tmp/test/.liza/log.yaml")

	// Should not panic for known notifications
	knownNotifications := []string{
		"notifications/initialized",
		"notifications/cancelled",
	}
	for _, method := range knownNotifications {
		t.Run(method, func(t *testing.T) {
			req := &protocol.JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  method,
			}
			// Should not panic
			server.handleNotification(req)
		})
	}

	// Unknown notification should also not panic
	t.Run("unknown notification", func(t *testing.T) {
		req := &protocol.JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "notifications/unknown",
		}
		server.handleNotification(req)
	})
}
