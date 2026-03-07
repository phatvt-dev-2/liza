package errors

import (
	"errors"
	"fmt"
)

// Exit codes used by liza commands
const (
	ExitSuccess  = 0  // Success
	ExitError    = 1  // General error
	ExitLock     = 2  // Lock timeout
	ExitNotFound = 3  // Entity or field not found
	ExitRestart  = 42 // Agent restart request
)

// NotFoundError represents an error where an entity or field was not found
type NotFoundError struct {
	Entity string // "task", "agent", "config"
	ID     string // optional: "task-42", "orchestrator-1"
	Field  string // optional: field name
}

func (e *NotFoundError) Error() string {
	base := e.Entity
	if e.ID != "" {
		base += " " + e.ID
	}
	if e.Field != "" {
		return fmt.Sprintf("%s field '%s' not found", base, e.Field)
	}
	if e.ID != "" {
		return fmt.Sprintf("%s not found: %s", e.Entity, e.ID)
	}
	return fmt.Sprintf("%s not found", e.Entity)
}

// IsNotFound checks if an error is a NotFoundError.
// Supports wrapped errors (e.g. from bb.Modify).
func IsNotFound(err error) bool {
	var nfe *NotFoundError
	return errors.As(err, &nfe)
}
