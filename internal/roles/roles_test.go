package roles

import (
	"slices"
	"testing"
)

func TestConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"RuntimeCoder", RuntimeCoder, "coder"},
		{"RuntimeCodeReviewer", RuntimeCodeReviewer, "code-reviewer"},
		{"RuntimeOrchestrator", RuntimeOrchestrator, "orchestrator"},
		{"RuntimeCodePlanner", RuntimeCodePlanner, "code-planner"},
		{"RuntimeCodePlanReviewer", RuntimeCodePlanReviewer, "code-plan-reviewer"},
		{"RuntimeEpicPlanner", RuntimeEpicPlanner, "epic-planner"},
		{"RuntimeEpicPlanReviewer", RuntimeEpicPlanReviewer, "epic-plan-reviewer"},
		{"RuntimeUSWriter", RuntimeUSWriter, "us-writer"},
		{"RuntimeUSReviewer", RuntimeUSReviewer, "us-reviewer"},
		{"WorkflowCoder", WorkflowCoder, "coder"},
		{"WorkflowCodeReviewer", WorkflowCodeReviewer, "code_reviewer"},
		{"WorkflowOrchestrator", WorkflowOrchestrator, "orchestrator"},
		{"WorkflowCodePlanner", WorkflowCodePlanner, "code_planner"},
		{"WorkflowCodePlanReviewer", WorkflowCodePlanReviewer, "code_plan_reviewer"},
		{"WorkflowEpicPlanner", WorkflowEpicPlanner, "epic_planner"},
		{"WorkflowEpicPlanReviewer", WorkflowEpicPlanReviewer, "epic_plan_reviewer"},
		{"WorkflowUSWriter", WorkflowUSWriter, "us_writer"},
		{"WorkflowUSReviewer", WorkflowUSReviewer, "us_reviewer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

func TestToWorkflow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		runtimeRole string
		want        string
		wantErr     bool
	}{
		{
			name:        "coder maps to coder",
			runtimeRole: RuntimeCoder,
			want:        WorkflowCoder,
			wantErr:     false,
		},
		{
			name:        "code-reviewer maps to code_reviewer",
			runtimeRole: RuntimeCodeReviewer,
			want:        WorkflowCodeReviewer,
			wantErr:     false,
		},
		{
			name:        "orchestrator maps to orchestrator",
			runtimeRole: RuntimeOrchestrator,
			want:        WorkflowOrchestrator,
			wantErr:     false,
		},
		{
			name:        "code-planner maps to code_planner",
			runtimeRole: RuntimeCodePlanner,
			want:        WorkflowCodePlanner,
			wantErr:     false,
		},
		{
			name:        "code-plan-reviewer maps to code_plan_reviewer",
			runtimeRole: RuntimeCodePlanReviewer,
			want:        WorkflowCodePlanReviewer,
			wantErr:     false,
		},
		{
			name:        "epic-planner maps to epic_planner",
			runtimeRole: RuntimeEpicPlanner,
			want:        WorkflowEpicPlanner,
			wantErr:     false,
		},
		{
			name:        "epic-plan-reviewer maps to epic_plan_reviewer",
			runtimeRole: RuntimeEpicPlanReviewer,
			want:        WorkflowEpicPlanReviewer,
			wantErr:     false,
		},
		{
			name:        "us-writer maps to us_writer",
			runtimeRole: RuntimeUSWriter,
			want:        WorkflowUSWriter,
			wantErr:     false,
		},
		{
			name:        "us-reviewer maps to us_reviewer",
			runtimeRole: RuntimeUSReviewer,
			want:        WorkflowUSReviewer,
			wantErr:     false,
		},
		{
			name:        "unknown role returns error",
			runtimeRole: "unknown-role",
			want:        "",
			wantErr:     true,
		},
		{
			name:        "underscore form not valid runtime",
			runtimeRole: "code_reviewer",
			want:        "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToWorkflow(tt.runtimeRole)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToWorkflow(%q) error = %v, wantErr %v", tt.runtimeRole, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ToWorkflow(%q) = %q, want %q", tt.runtimeRole, got, tt.want)
			}
		})
	}
}

func TestToRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		workflowRole string
		want         string
		wantErr      bool
	}{
		{
			name:         "coder maps to coder",
			workflowRole: WorkflowCoder,
			want:         RuntimeCoder,
			wantErr:      false,
		},
		{
			name:         "code_reviewer maps to code-reviewer",
			workflowRole: WorkflowCodeReviewer,
			want:         RuntimeCodeReviewer,
			wantErr:      false,
		},
		{
			name:         "orchestrator maps to orchestrator",
			workflowRole: WorkflowOrchestrator,
			want:         RuntimeOrchestrator,
			wantErr:      false,
		},
		{
			name:         "code_planner maps to code-planner",
			workflowRole: WorkflowCodePlanner,
			want:         RuntimeCodePlanner,
			wantErr:      false,
		},
		{
			name:         "code_plan_reviewer maps to code-plan-reviewer",
			workflowRole: WorkflowCodePlanReviewer,
			want:         RuntimeCodePlanReviewer,
			wantErr:      false,
		},
		{
			name:         "epic_planner maps to epic-planner",
			workflowRole: WorkflowEpicPlanner,
			want:         RuntimeEpicPlanner,
			wantErr:      false,
		},
		{
			name:         "epic_plan_reviewer maps to epic-plan-reviewer",
			workflowRole: WorkflowEpicPlanReviewer,
			want:         RuntimeEpicPlanReviewer,
			wantErr:      false,
		},
		{
			name:         "us_writer maps to us-writer",
			workflowRole: WorkflowUSWriter,
			want:         RuntimeUSWriter,
			wantErr:      false,
		},
		{
			name:         "us_reviewer maps to us-reviewer",
			workflowRole: WorkflowUSReviewer,
			want:         RuntimeUSReviewer,
			wantErr:      false,
		},
		{
			name:         "unknown role returns error",
			workflowRole: "unknown_role",
			want:         "",
			wantErr:      true,
		},
		{
			name:         "hyphen form not valid workflow",
			workflowRole: "code-reviewer",
			want:         "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToRuntime(tt.workflowRole)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToRuntime(%q) error = %v, wantErr %v", tt.workflowRole, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ToRuntime(%q) = %q, want %q", tt.workflowRole, got, tt.want)
			}
		})
	}
}

func TestIsValidRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		role string
		want bool
	}{
		{RuntimeCoder, RuntimeCoder, true},
		{RuntimeCodeReviewer, RuntimeCodeReviewer, true},
		{RuntimeOrchestrator, RuntimeOrchestrator, true},
		{RuntimeCodePlanner, RuntimeCodePlanner, true},
		{RuntimeCodePlanReviewer, RuntimeCodePlanReviewer, true},
		{RuntimeEpicPlanner, RuntimeEpicPlanner, true},
		{RuntimeEpicPlanReviewer, RuntimeEpicPlanReviewer, true},
		{RuntimeUSWriter, RuntimeUSWriter, true},
		{RuntimeUSReviewer, RuntimeUSReviewer, true},
		{"code_reviewer", "code_reviewer", false},
		{"unknown", "unknown", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidRuntime(tt.role); got != tt.want {
				t.Errorf("IsValidRuntime(%q) = %v, want %v", tt.role, got, tt.want)
			}
		})
	}
}

func TestIsValidWorkflow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		role string
		want bool
	}{
		{WorkflowCoder, WorkflowCoder, true},
		{WorkflowCodeReviewer, WorkflowCodeReviewer, true},
		{WorkflowOrchestrator, WorkflowOrchestrator, true},
		{WorkflowCodePlanner, WorkflowCodePlanner, true},
		{WorkflowCodePlanReviewer, WorkflowCodePlanReviewer, true},
		{WorkflowEpicPlanner, WorkflowEpicPlanner, true},
		{WorkflowEpicPlanReviewer, WorkflowEpicPlanReviewer, true},
		{WorkflowUSWriter, WorkflowUSWriter, true},
		{WorkflowUSReviewer, WorkflowUSReviewer, true},
		{"code-reviewer", "code-reviewer", false},
		{"unknown", "unknown", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidWorkflow(tt.role); got != tt.want {
				t.Errorf("IsValidWorkflow(%q) = %v, want %v", tt.role, got, tt.want)
			}
		})
	}
}

func TestAllRuntime(t *testing.T) {
	t.Parallel()

	got := AllRuntime()
	want := []string{
		RuntimeCoder, RuntimeCodeReviewer, RuntimeOrchestrator,
		RuntimeCodePlanner, RuntimeCodePlanReviewer,
		RuntimeEpicPlanner, RuntimeEpicPlanReviewer,
		RuntimeUSWriter, RuntimeUSReviewer,
	}

	if len(got) != len(want) {
		t.Errorf("AllRuntime() returned %d roles, want %d", len(got), len(want))
	}

	for _, role := range want {
		if !slices.Contains(got, role) {
			t.Errorf("AllRuntime() missing role %q", role)
		}
	}
}

func TestAllWorkflow(t *testing.T) {
	t.Parallel()

	got := AllWorkflow()
	want := []string{
		WorkflowCoder, WorkflowCodeReviewer, WorkflowOrchestrator,
		WorkflowCodePlanner, WorkflowCodePlanReviewer,
		WorkflowEpicPlanner, WorkflowEpicPlanReviewer,
		WorkflowUSWriter, WorkflowUSReviewer,
	}

	if len(got) != len(want) {
		t.Errorf("AllWorkflow() returned %d roles, want %d", len(got), len(want))
	}

	for _, role := range want {
		if !slices.Contains(got, role) {
			t.Errorf("AllWorkflow() missing role %q", role)
		}
	}
}

// TestBidirectionalMapping ensures all valid runtime roles map to workflow and back.
func TestBidirectionalMapping(t *testing.T) {
	t.Parallel()

	for _, runtime := range AllRuntime() {
		workflow, err := ToWorkflow(runtime)
		if err != nil {
			t.Errorf("ToWorkflow(%q) failed: %v", runtime, err)
			continue
		}

		backToRuntime, err := ToRuntime(workflow)
		if err != nil {
			t.Errorf("ToRuntime(%q) failed: %v", workflow, err)
			continue
		}

		if backToRuntime != runtime {
			t.Errorf("Round-trip failed: %q -> %q -> %q", runtime, workflow, backToRuntime)
		}
	}
}

// TestCrossBoundaryResolution verifies the core requirement:
// agent runtime roles can be resolved to workflow roles for task operations.
func TestCrossBoundaryResolution(t *testing.T) {
	t.Parallel()

	// Simulate: agent with ID "code-reviewer-1" claims a task
	// The agent uses runtime role "code-reviewer"
	// The task workflow uses "code_reviewer"

	agentRuntimeRole := RuntimeCodeReviewer
	workflowRole, err := ToWorkflow(agentRuntimeRole)
	if err != nil {
		t.Fatalf("Failed to resolve runtime role to workflow: %v", err)
	}

	// Verify it matches the models constant
	if workflowRole != WorkflowCodeReviewer {
		t.Errorf("Workflow role mismatch: got %q, want %q", workflowRole, WorkflowCodeReviewer)
	}
}
