package protocol

// Standard JSON-RPC 2.0 error codes
const (
	// ParseError invalid JSON was received by the server
	ParseError = -32700

	// InvalidRequest the JSON sent is not a valid Request object
	InvalidRequest = -32600

	// MethodNotFound the method does not exist / is not available
	MethodNotFound = -32601

	// InvalidParams invalid method parameter(s)
	InvalidParams = -32602

	// InternalError internal JSON-RPC error
	InternalError = -32603
)

// MCP-specific error codes (application-defined)
const (
	// LockTimeout lock acquisition timeout
	LockTimeout = -32001

	// ValidationError state validation failed
	ValidationError = -32002

	// RaceCondition three-phase commit race condition
	RaceCondition = -32003

	// NotFound entity not found
	NotFound = -32004
)

// NewError creates a new JSONRPCError
func NewError(code int, message string, data any) *JSONRPCError {
	return &JSONRPCError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// NewParseError creates a parse error
func NewParseError(message string) *JSONRPCError {
	return NewError(ParseError, message, nil)
}

// NewInvalidRequestError creates an invalid request error
func NewInvalidRequestError(message string) *JSONRPCError {
	return NewError(InvalidRequest, message, nil)
}

// NewMethodNotFoundError creates a method not found error
func NewMethodNotFoundError(method string) *JSONRPCError {
	return NewError(MethodNotFound, "Method not found: "+method, nil)
}

// NewInvalidParamsError creates an invalid params error
func NewInvalidParamsError(message string) *JSONRPCError {
	return NewError(InvalidParams, message, nil)
}

// NewInternalError creates an internal error
func NewInternalError(message string) *JSONRPCError {
	return NewError(InternalError, message, nil)
}
