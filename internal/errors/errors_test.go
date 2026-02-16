package errors

import (
	"testing"
)

func TestNotFoundError(t *testing.T) {
	tests := []struct {
		name   string
		entity string
		field  string
		want   string
	}{
		{
			name:   "entity only",
			entity: "task",
			field:  "",
			want:   "task not found",
		},
		{
			name:   "entity with field",
			entity: "config",
			field:  "mode",
			want:   "config field 'mode' not found",
		},
		{
			name:   "agent with id",
			entity: "agent coder-1",
			field:  "",
			want:   "agent coder-1 not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &NotFoundError{
				Entity: tt.entity,
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
