package agent

import (
	"context"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/roles"
)

// TestNewRoleStrategy verifies the factory creates the correct strategy type
// with correct role and workflowRole for all 9 runtime roles.
func TestNewRoleStrategy(t *testing.T) {
	tests := []struct {
		role         string
		wantType     string // "doer", "reviewer", "orchestrator"
		wantWorkflow string
	}{
		{roles.RuntimeCoder, "doer", roles.WorkflowCoder},
		{roles.RuntimeCodePlanner, "doer", roles.WorkflowCodePlanner},
		{roles.RuntimeEpicPlanner, "doer", roles.WorkflowEpicPlanner},
		{roles.RuntimeUSWriter, "doer", roles.WorkflowUSWriter},
		{roles.RuntimeCodeReviewer, "reviewer", roles.WorkflowCodeReviewer},
		{roles.RuntimeCodePlanReviewer, "reviewer", roles.WorkflowCodePlanReviewer},
		{roles.RuntimeEpicPlanReviewer, "reviewer", roles.WorkflowEpicPlanReviewer},
		{roles.RuntimeUSReviewer, "reviewer", roles.WorkflowUSReviewer},
		{roles.RuntimeOrchestrator, "orchestrator", ""},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			s, err := NewRoleStrategy(tt.role)
			if err != nil {
				t.Fatalf("NewRoleStrategy(%q) error = %v", tt.role, err)
			}

			switch tt.wantType {
			case "doer":
				ds, ok := s.(*doerStrategy)
				if !ok {
					t.Fatalf("expected *doerStrategy, got %T", s)
				}
				if ds.role != tt.role {
					t.Errorf("role = %q, want %q", ds.role, tt.role)
				}
				if ds.workflowRole != tt.wantWorkflow {
					t.Errorf("workflowRole = %q, want %q", ds.workflowRole, tt.wantWorkflow)
				}
				if ds.buildContext == nil {
					t.Error("buildContext should not be nil")
				}
			case "reviewer":
				rs, ok := s.(*reviewerStrategy)
				if !ok {
					t.Fatalf("expected *reviewerStrategy, got %T", s)
				}
				if rs.role != tt.role {
					t.Errorf("role = %q, want %q", rs.role, tt.role)
				}
				if rs.workflowRole != tt.wantWorkflow {
					t.Errorf("workflowRole = %q, want %q", rs.workflowRole, tt.wantWorkflow)
				}
				if rs.buildContext == nil {
					t.Error("buildContext should not be nil")
				}
			case "orchestrator":
				if _, ok := s.(*orchestratorStrategy); !ok {
					t.Fatalf("expected *orchestratorStrategy, got %T", s)
				}
			}
		})
	}
}

// TestNewRoleStrategy_UnknownRole verifies the factory returns an error for unknown roles.
func TestNewRoleStrategy_UnknownRole(t *testing.T) {
	_, err := NewRoleStrategy("nonexistent-role")
	if err == nil {
		t.Fatal("expected error for unknown role, got nil")
	}
}

// TestDefaultTimeout verifies correct timeout per category.
func TestDefaultTimeout(t *testing.T) {
	tests := []struct {
		role string
		want time.Duration
	}{
		{roles.RuntimeCoder, 2 * time.Hour},
		{roles.RuntimeCodePlanner, 2 * time.Hour},
		{roles.RuntimeEpicPlanner, 2 * time.Hour},
		{roles.RuntimeUSWriter, 2 * time.Hour},
		{roles.RuntimeCodeReviewer, 30 * time.Minute},
		{roles.RuntimeCodePlanReviewer, 30 * time.Minute},
		{roles.RuntimeEpicPlanReviewer, 30 * time.Minute},
		{roles.RuntimeUSReviewer, 30 * time.Minute},
		{roles.RuntimeOrchestrator, 4 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			s, err := NewRoleStrategy(tt.role)
			if err != nil {
				t.Fatalf("NewRoleStrategy(%q) error = %v", tt.role, err)
			}
			if got := s.DefaultTimeout(); got != tt.want {
				t.Errorf("DefaultTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDoerPreWork_IsNoOp verifies doer PreWork returns (false, nil).
func TestDoerPreWork_IsNoOp(t *testing.T) {
	for _, role := range roles.DoerRoles() {
		t.Run(role, func(t *testing.T) {
			s, err := NewRoleStrategy(role)
			if err != nil {
				t.Fatalf("NewRoleStrategy(%q) error = %v", role, err)
			}
			shouldContinue, err := s.PreWork(context.Background(), nil, SupervisorConfig{})
			if err != nil {
				t.Errorf("PreWork() error = %v", err)
			}
			if shouldContinue {
				t.Error("PreWork() shouldContinue = true, want false")
			}
		})
	}
}

// TestOrchestratorPreWork_IsNoOp verifies orchestrator PreWork returns (false, nil).
func TestOrchestratorPreWork_IsNoOp(t *testing.T) {
	s, err := NewRoleStrategy(roles.RuntimeOrchestrator)
	if err != nil {
		t.Fatalf("NewRoleStrategy() error = %v", err)
	}
	shouldContinue, err := s.PreWork(context.Background(), nil, SupervisorConfig{})
	if err != nil {
		t.Errorf("PreWork() error = %v", err)
	}
	if shouldContinue {
		t.Error("PreWork() shouldContinue = true, want false")
	}
}

// TestOrchestratorClaimTask_ReturnsEmpty verifies orchestrator ClaimTask returns ("", "", nil).
func TestOrchestratorClaimTask_ReturnsEmpty(t *testing.T) {
	s, err := NewRoleStrategy(roles.RuntimeOrchestrator)
	if err != nil {
		t.Fatalf("NewRoleStrategy() error = %v", err)
	}
	taskID, claimedID, err := s.ClaimTask(SupervisorConfig{}, nil)
	if err != nil {
		t.Errorf("ClaimTask() error = %v", err)
	}
	if taskID != "" || claimedID != "" {
		t.Errorf("ClaimTask() = (%q, %q), want (\"\", \"\")", taskID, claimedID)
	}
}

// TestReviewerPostExecution_IsNoOp verifies reviewer PostExecution returns nil.
func TestReviewerPostExecution_IsNoOp(t *testing.T) {
	for _, role := range roles.ReviewerRoles() {
		t.Run(role, func(t *testing.T) {
			s, err := NewRoleStrategy(role)
			if err != nil {
				t.Fatalf("NewRoleStrategy(%q) error = %v", role, err)
			}
			if err := s.PostExecution(nil, SupervisorConfig{}, "", "", nil); err != nil {
				t.Errorf("PostExecution() error = %v", err)
			}
		})
	}
}

// TestDoerPreExecution_IsNoOp verifies doer PreExecution returns nil.
func TestDoerPreExecution_IsNoOp(t *testing.T) {
	for _, role := range roles.DoerRoles() {
		t.Run(role, func(t *testing.T) {
			s, err := NewRoleStrategy(role)
			if err != nil {
				t.Fatalf("NewRoleStrategy(%q) error = %v", role, err)
			}
			if err := s.PreExecution(nil, SupervisorConfig{}); err != nil {
				t.Errorf("PreExecution() error = %v", err)
			}
		})
	}
}

// TestReviewerPreExecution_IsNoOp verifies reviewer PreExecution returns nil.
func TestReviewerPreExecution_IsNoOp(t *testing.T) {
	for _, role := range roles.ReviewerRoles() {
		t.Run(role, func(t *testing.T) {
			s, err := NewRoleStrategy(role)
			if err != nil {
				t.Fatalf("NewRoleStrategy(%q) error = %v", role, err)
			}
			if err := s.PreExecution(nil, SupervisorConfig{}); err != nil {
				t.Errorf("PreExecution() error = %v", err)
			}
		})
	}
}

// TestReviewerEffectiveMaxRetries verifies the default and override behavior.
func TestReviewerEffectiveMaxRetries(t *testing.T) {
	// Default (maxRetries = 0) → defaultMaxMergeRetries
	rs := &reviewerStrategy{maxRetries: 0}
	if got := rs.effectiveMaxRetries(); got != defaultMaxMergeRetries {
		t.Errorf("effectiveMaxRetries() = %d, want %d", got, defaultMaxMergeRetries)
	}

	// Override
	rs.maxRetries = 5
	if got := rs.effectiveMaxRetries(); got != 5 {
		t.Errorf("effectiveMaxRetries() = %d, want 5", got)
	}
}

// TestDoerPostExecution_NilClaimedTaskID verifies PostExecution is a no-op when
// claimedTaskID is empty (no task was claimed).
func TestDoerPostExecution_NilClaimedTaskID(t *testing.T) {
	s, err := NewRoleStrategy(roles.RuntimeCoder)
	if err != nil {
		t.Fatalf("NewRoleStrategy() error = %v", err)
	}
	// Empty claimedTaskID should return nil without touching bb
	if err := s.PostExecution(nil, SupervisorConfig{}, "", "", nil); err != nil {
		t.Errorf("PostExecution() error = %v", err)
	}
}

// TestAllRolesHaveStrategy ensures every role from roles.AllRuntime() has a strategy.
func TestAllRolesHaveStrategy(t *testing.T) {
	for _, role := range roles.AllRuntime() {
		t.Run(role, func(t *testing.T) {
			s, err := NewRoleStrategy(role)
			if err != nil {
				t.Fatalf("NewRoleStrategy(%q) error = %v", role, err)
			}
			if s == nil {
				t.Fatal("strategy should not be nil")
			}
		})
	}
}
