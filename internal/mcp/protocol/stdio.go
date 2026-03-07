package protocol

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// MaxRequestSize is the maximum allowed size for a JSON-RPC request (10MB)
const MaxRequestSize = 10 * 1024 * 1024

// ErrRequestTooLarge is returned when a request exceeds MaxRequestSize
var ErrRequestTooLarge = errors.New("request exceeds maximum allowed size")

// StdioTransport implements JSON-RPC 2.0 over stdin/stdout
type StdioTransport struct {
	scanner *bufio.Scanner
	writer  *bufio.Writer
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport() *StdioTransport {
	return newStdioTransport(os.Stdin, os.Stdout)
}

// newStdioTransport creates a transport from arbitrary reader/writer (for testing)
func newStdioTransport(r io.Reader, w io.Writer) *StdioTransport {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), MaxRequestSize+1)
	return &StdioTransport{
		scanner: scanner,
		writer:  bufio.NewWriter(w),
	}
}

// ReadRequest reads a JSON-RPC request from stdin with a maximum size limit.
// Returns ErrRequestTooLarge if the request exceeds MaxRequestSize.
func (t *StdioTransport) ReadRequest() (*JSONRPCRequest, error) {
	if !t.scanner.Scan() {
		err := t.scanner.Err()
		if err == nil {
			return nil, io.EOF
		}
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, ErrRequestTooLarge
		}
		return nil, fmt.Errorf("failed to read request: %w", err)
	}

	line := t.scanner.Bytes()
	if len(line) == 0 {
		return nil, io.EOF
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
