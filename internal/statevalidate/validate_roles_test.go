package statevalidate

import (
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/models"
)

func TestValidateRoleNames_UnderscoreDetected(t *testing.T) {
	tests := []struct {
		name    string
		role    string
		wantErr bool
		wantMsg string
	}{
		{
			name:    "underscore code_reviewer triggers error",
			role:    "code_reviewer",
			wantErr: true,
			wantMsg: "liza migrate",
		},
		{
			name:    "underscore code_planner triggers error",
			role:    "code_planner",
			wantErr: true,
			wantMsg: "liza migrate",
		},
		{
			name:    "underscore epic_plan_reviewer triggers error",
			role:    "epic_plan_reviewer",
			wantErr: true,
			wantMsg: "liza migrate",
		},
		{
			name:    "hyphenated code-reviewer passes",
			role:    "code-reviewer",
			wantErr: false,
		},
		{
			name:    "hyphenated code-planner passes",
			role:    "code-planner",
			wantErr: false,
		},
		{
			name:    "single-word coder passes",
			role:    "coder",
			wantErr: false,
		},
		{
			name:    "single-word orchestrator passes",
			role:    "orchestrator",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.State{
				Agents: map[string]models.Agent{
					"test-agent": {Role: tt.role},
				},
			}
			err := validateRoleNames(state, "/tmp", false)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for role %q, got nil", tt.role)
				}
				if !strings.Contains(err.Error(), tt.wantMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.wantMsg)
				}
				if !strings.Contains(err.Error(), tt.role) {
					t.Errorf("error %q should mention the role name %q", err.Error(), tt.role)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for role %q: %v", tt.role, err)
				}
			}
		})
	}
}

func TestValidateRoleNames_MultipleAgents(t *testing.T) {
	state := &models.State{
		Agents: map[string]models.Agent{
			"coder-1":    {Role: "coder"},
			"reviewer-1": {Role: "code_reviewer"},
		},
	}
	err := validateRoleNames(state, "/tmp", false)
	if err == nil {
		t.Fatal("expected error when any agent has underscore role")
	}
	if !strings.Contains(err.Error(), "liza migrate") {
		t.Errorf("error should mention liza migrate, got: %v", err)
	}
}

func TestValidateRoleNames_NoAgents(t *testing.T) {
	state := &models.State{
		Agents: map[string]models.Agent{},
	}
	if err := validateRoleNames(state, "/tmp", false); err != nil {
		t.Fatalf("empty agents should pass: %v", err)
	}
}
