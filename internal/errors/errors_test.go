package errors

import (
	"fmt"
	"testing"
)

func TestNotFoundError(t *testing.T) {
	tests := []struct {
		name   string
		entity string
		id     string
		field  string
		want   string
	}{
		{
			name:   "entity only",
			entity: "task",
			want:   "task not found",
		},
		{
			name:   "entity with field",
			entity: "config",
			field:  "mode",
			want:   "config field 'mode' not found",
		},
		{
			name:   "entity with id",
			entity: "task",
			id:     "task-42",
			want:   "task not found: task-42",
		},
		{
			name:   "agent with id",
			entity: "agent",
			id:     "coder-1",
			want:   "agent not found: coder-1",
		},
		{
			name:   "entity with id and field",
			entity: "agent",
			id:     "coder-1",
			field:  "status",
			want:   "agent coder-1 field 'status' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &NotFoundError{
				Entity: tt.entity,
				ID:     tt.id,
				Field:  tt.field,
			}

			if got := err.Error(); got != tt.want {
				t.Errorf("NotFoundError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "NotFoundError",
			err:  &NotFoundError{Entity: "task"},
			want: true,
		},
		{
			name: "NotFoundError with ID",
			err:  &NotFoundError{Entity: "task", ID: "task-42"},
			want: true,
		},
		{
			name: "wrapped NotFoundError",
			err:  fmt.Errorf("modification function failed: %w", &NotFoundError{Entity: "task", ID: "task-42"}),
			want: true,
		},
		{
			name: "double-wrapped NotFoundError",
			err:  fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", &NotFoundError{Entity: "agent", ID: "coder-1"})),
			want: true,
		},
		{
			name: "other error",
			err:  &testError{msg: "some error"},
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

// testError is a helper type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
