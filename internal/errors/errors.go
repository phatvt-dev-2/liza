package errors

import (
	"fmt"
	"os"
)

// Exit codes used by liza commands
const (
	ExitSuccess  = 0  // Success
	ExitError    = 1  // General error
	ExitLock     = 2  // Lock timeout
	ExitNotFound = 3  // Entity or field not found
	ExitRestart  = 42 // Agent restart request
)

// ExitWithCode prints an error message and exits with the specified code
func ExitWithCode(code int, format string, args ...any) {
	if code != ExitSuccess {
		fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	}
	os.Exit(code)
}

// NotFoundError represents an error where an entity or field was not found
type NotFoundError struct {
	Entity string
	Field  string
}

func (e *NotFoundError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s field '%s' not found", e.Entity, e.Field)
	}
	return fmt.Sprintf("%s not found", e.Entity)
}

// IsNotFound checks if an error is a NotFoundError
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*NotFoundError)
	return ok
}
