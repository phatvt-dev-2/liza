package agent

import (
	"context"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
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

// TestWaitConfig verifies each role resolves the correct config keys and defaults.
func TestWaitConfig(t *testing.T) {
	tests := []struct {
		role        string
		wantPoll    time.Duration
		wantMaxWait time.Duration
	}{
		// Doer roles use Coder defaults
		{roles.RuntimeCoder, 30 * time.Second, 1800 * time.Second},
		{roles.RuntimeCodePlanner, 30 * time.Second, 1800 * time.Second},
		{roles.RuntimeEpicPlanner, 30 * time.Second, 1800 * time.Second},
		{roles.RuntimeUSWriter, 30 * time.Second, 1800 * time.Second},
		// Reviewer roles use Reviewer defaults
		{roles.RuntimeCodeReviewer, 30 * time.Second, 1800 * time.Second},
		{roles.RuntimeCodePlanReviewer, 30 * time.Second, 1800 * time.Second},
		{roles.RuntimeEpicPlanReviewer, 30 * time.Second, 1800 * time.Second},
		{roles.RuntimeUSReviewer, 30 * time.Second, 1800 * time.Second},
		// Orchestrator uses Orchestrator defaults
		{roles.RuntimeOrchestrator, 60 * time.Second, 1800 * time.Second},
	}

	zeroState := &models.State{}

	for _, tt := range tests {
		t.Run(tt.role+"/defaults", func(t *testing.T) {
			s, err := NewRoleStrategy(tt.role)
			if err != nil {
				t.Fatalf("NewRoleStrategy(%q) error = %v", tt.role, err)
			}
			poll, maxWait := s.WaitConfig(zeroState)
			if poll != tt.wantPoll {
				t.Errorf("WaitConfig() poll = %v, want %v", poll, tt.wantPoll)
			}
			if maxWait != tt.wantMaxWait {
				t.Errorf("WaitConfig() maxWait = %v, want %v", maxWait, tt.wantMaxWait)
			}
		})
	}

	// Verify each category reads the correct config keys (not each other's).
	t.Run("custom_config/doer", func(t *testing.T) {
		state := &models.State{Config: models.Config{CoderPollInterval: 5, CoderMaxWait: 60}}
		s, _ := NewRoleStrategy(roles.RuntimeCoder)
		poll, maxWait := s.WaitConfig(state)
		if poll != 5*time.Second || maxWait != 60*time.Second {
			t.Errorf("doer WaitConfig() = (%v, %v), want (5s, 1m0s)", poll, maxWait)
		}
	})

	t.Run("custom_config/reviewer", func(t *testing.T) {
		state := &models.State{Config: models.Config{ReviewerPollInterval: 10, ReviewerMaxWait: 120}}
		s, _ := NewRoleStrategy(roles.RuntimeCodeReviewer)
		poll, maxWait := s.WaitConfig(state)
		if poll != 10*time.Second || maxWait != 120*time.Second {
			t.Errorf("reviewer WaitConfig() = (%v, %v), want (10s, 2m0s)", poll, maxWait)
		}
	})

	t.Run("custom_config/orchestrator", func(t *testing.T) {
		state := &models.State{Config: models.Config{OrchestratorPollInterval: 15, OrchestratorMaxWait: 300}}
		s, _ := NewRoleStrategy(roles.RuntimeOrchestrator)
		poll, maxWait := s.WaitConfig(state)
		if poll != 15*time.Second || maxWait != 300*time.Second {
			t.Errorf("orchestrator WaitConfig() = (%v, %v), want (15s, 5m0s)", poll, maxWait)
		}
	})

	// Cross-contamination: doer config should NOT affect reviewer or orchestrator
	t.Run("custom_config/isolation", func(t *testing.T) {
		state := &models.State{Config: models.Config{CoderPollInterval: 99, CoderMaxWait: 99}}
		reviewer, _ := NewRoleStrategy(roles.RuntimeCodeReviewer)
		poll, _ := reviewer.WaitConfig(state)
		if poll == 99*time.Second {
			t.Error("reviewer should not read CoderPollInterval")
		}
		orch, _ := NewRoleStrategy(roles.RuntimeOrchestrator)
		poll, _ = orch.WaitConfig(state)
		if poll == 99*time.Second {
			t.Error("orchestrator should not read CoderPollInterval")
		}
	})
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
