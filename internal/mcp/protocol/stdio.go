package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// StdioTransport implements JSON-RPC 2.0 over stdin/stdout
type StdioTransport struct {
	reader *bufio.Reader
	writer *bufio.Writer
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport() *StdioTransport {
	return &StdioTransport{
		reader: bufio.NewReader(os.Stdin),
		writer: bufio.NewWriter(os.Stdout),
	}
}

// ReadRequest reads a JSON-RPC request from stdin
func (t *StdioTransport) ReadRequest() (*JSONRPCRequest, error) {
	line, err := t.reader.ReadBytes('\n')
	if err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("failed to read request: %w", err)
	}

	var req JSONRPCRequest
	if err := json.Unmarshal(line, &req); err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC request: %w", err)
	}

	// Validate JSON-RPC version
	if req.JSONRPC != "2.0" {
		return nil, fmt.Errorf("unsupported JSON-RPC version: %s", req.JSONRPC)
	}

	return &req, nil
}

// WriteResponse writes a JSON-RPC response to stdout
func (t *StdioTransport) WriteResponse(resp *JSONRPCResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	if _, err := t.writer.Write(data); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}

	if err := t.writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	if err := t.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush: %w", err)
	}

	return nil
}

// WriteError writes a JSON-RPC error response to stdout
func (t *StdioTransport) WriteError(id json.RawMessage, code int, message string, data any) error {
	resp := &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	return t.WriteResponse(resp)
}
