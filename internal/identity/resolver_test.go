package identity

import (
	"os"
	"testing"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		envValue    string
		setEnv      bool
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name: "flag takes priority over env var",
			config: Config{
				FlagValue:    "coder-1",
				DefaultValue: "default-1",
				Required:     true,
			},
			envValue: "coder-2",
			setEnv:   true,
			want:     "coder-1",
			wantErr:  false,
		},
		{
			name: "env var used when flag empty",
			config: Config{
				FlagValue:    "",
				DefaultValue: "default-1",
				Required:     true,
			},
			envValue: "coder-2",
			setEnv:   true,
			want:     "coder-2",
			wantErr:  false,
		},
		{
			name: "default used when flag and env empty",
			config: Config{
				FlagValue:    "",
				DefaultValue: "planner-1",
				Required:     false,
			},
			envValue: "",
			setEnv:   false,
			want:     "planner-1",
			wantErr:  false,
		},
		{
			name: "error when required but nothing provided",
			config: Config{
				FlagValue:    "",
				DefaultValue: "",
				Required:     true,
			},
			envValue:    "",
			setEnv:      false,
			want:        "",
			wantErr:     true,
			errContains: "agent ID required",
		},
		{
			name: "empty string when not required and nothing provided",
			config: Config{
				FlagValue:    "",
				DefaultValue: "",
				Required:     false,
			},
			envValue: "",
			setEnv:   false,
			want:     "",
			wantErr:  false,
		},
		{
			name: "flag with spaces is trimmed",
			config: Config{
				FlagValue:    "  coder-1  ",
				DefaultValue: "",
				Required:     true,
			},
			envValue: "",
			setEnv:   false,
			want:     "coder-1",
			wantErr:  false,
		},
		{
			name: "env var with spaces is trimmed",
			config: Config{
				FlagValue:    "",
				DefaultValue: "",
				Required:     true,
			},
			envValue: "  code-reviewer-1  ",
			setEnv:   true,
			want:     "code-reviewer-1",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			oldEnv := os.Getenv("LIZA_AGENT_ID")
			defer func() {
				if oldEnv != "" {
					os.Setenv("LIZA_AGENT_ID", oldEnv)
				} else {
					os.Unsetenv("LIZA_AGENT_ID")
				}
			}()

			if tt.setEnv {
				os.Setenv("LIZA_AGENT_ID", tt.envValue)
			} else {
				os.Unsetenv("LIZA_AGENT_ID")
			}

			// Test
			got, err := Resolve(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !contains(err.Error(), tt.errContains) {
					t.Errorf("Resolve() error = %v, want error containing %q", err, tt.errContains)
				}
			}
			if got != tt.want {
				t.Errorf("Resolve() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateFormat(t *testing.T) {
	tests := []struct {
		name        string
		agentID     string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid format coder-1",
			agentID: "coder-1",
			wantErr: false,
		},
		{
			name:    "valid format code-reviewer-2",
			agentID: "code-reviewer-2",
			wantErr: false,
		},
		{
			name:    "valid format planner-100",
			agentID: "planner-100",
			wantErr: false,
		},
		{
			name:        "empty string",
			agentID:     "",
			wantErr:     true,
			errContains: "agent ID required",
		},
		{
			name:        "no hyphen",
			agentID:     "coder1",
			wantErr:     true,
			errContains: "expected {role}-{number}",
		},
		{
			name:        "no number suffix",
			agentID:     "coder-",
			wantErr:     true,
			errContains: "suffix must be numeric",
		},
		{
			name:        "non-numeric suffix",
			agentID:     "coder-abc",
			wantErr:     true,
			errContains: "suffix must be numeric",
		},
		{
			name:        "negative number",
			agentID:     "coder--1",
			wantErr:     true,
			errContains: "suffix must be numeric",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFormat(tt.agentID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateFormat() error = %v, want error containing %q", err, tt.errContains)
				}
			}
		})
	}
}

func TestValidateRole(t *testing.T) {
	tests := []struct {
		name         string
		agentID      string
		expectedRole string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "matching role coder",
			agentID:      "coder-1",
			expectedRole: "coder",
			wantErr:      false,
		},
		{
			name:         "matching role code-reviewer",
			agentID:      "code-reviewer-2",
			expectedRole: "code-reviewer",
			wantErr:      false,
		},
		{
			name:         "matching role planner",
			agentID:      "planner-1",
			expectedRole: "planner",
			wantErr:      false,
		},
		{
			name:         "role mismatch",
			agentID:      "coder-1",
			expectedRole: "reviewer",
			wantErr:      true,
			errContains:  "role mismatch",
		},
		{
			name:         "invalid format no hyphen",
			agentID:      "coder1",
			expectedRole: "coder",
			wantErr:      true,
			errContains:  "expected {role}-{number}",
		},
		{
			name:         "empty agent ID",
			agentID:      "",
			expectedRole: "coder",
			wantErr:      true,
			errContains:  "agent ID required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRole(tt.agentID, tt.expectedRole)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRole() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateRole() error = %v, want error containing %q", err, tt.errContains)
				}
			}
		})
	}
}

func TestExtractRole(t *testing.T) {
	tests := []struct {
		name        string
		agentID     string
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "coder-1",
			agentID: "coder-1",
			want:    "coder",
			wantErr: false,
		},
		{
			name:    "code-reviewer-2",
			agentID: "code-reviewer-2",
			want:    "code-reviewer",
			wantErr: false,
		},
		{
			name:    "planner-100",
			agentID: "planner-100",
			want:    "planner",
			wantErr: false,
		},
		{
			name:        "no hyphen",
			agentID:     "coder1",
			want:        "",
			wantErr:     true,
			errContains: "expected {role}-{number}",
		},
		{
			name:        "empty string",
			agentID:     "",
			want:        "",
			wantErr:     true,
			errContains: "agent ID required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractRole(tt.agentID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractRole() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !contains(err.Error(), tt.errContains) {
					t.Errorf("ExtractRole() error = %v, want error containing %q", err, tt.errContains)
				}
			}
			if got != tt.want {
				t.Errorf("ExtractRole() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractNumber(t *testing.T) {
	tests := []struct {
		name        string
		agentID     string
		want        int
		wantErr     bool
		errContains string
	}{
		{
			name:    "coder-1",
			agentID: "coder-1",
			want:    1,
			wantErr: false,
		},
		{
			name:    "code-reviewer-2",
			agentID: "code-reviewer-2",
			want:    2,
			wantErr: false,
		},
		{
			name:    "planner-100",
			agentID: "planner-100",
			want:    100,
			wantErr: false,
		},
		{
			name:        "no hyphen",
			agentID:     "coder1",
			want:        0,
			wantErr:     true,
			errContains: "expected {role}-{number}",
		},
		{
			name:        "non-numeric suffix",
			agentID:     "coder-abc",
			want:        0,
			wantErr:     true,
			errContains: "suffix must be numeric",
		},
		{
			name:        "empty string",
			agentID:     "",
			want:        0,
			wantErr:     true,
			errContains: "agent ID required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractNumber(tt.agentID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !contains(err.Error(), tt.errContains) {
					t.Errorf("ExtractNumber() error = %v, want error containing %q", err, tt.errContains)
				}
			}
			if got != tt.want {
				t.Errorf("ExtractNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
