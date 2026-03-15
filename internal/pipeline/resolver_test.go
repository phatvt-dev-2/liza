package pipeline

import (
	"slices"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
)

func loadPhase2Config(t *testing.T) *PipelineConfig {
	t.Helper()
	cfg, err := Load("testdata/valid-phase2-full.yaml")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	return cfg
}

func TestResolver_Transition_PipelineTransition(t *testing.T) {
	r := NewResolver(loadPhase2Config(t))

	// Should find pipeline-transition "us-to-coding".
	tr, err := r.Transition("us-to-coding")
	if err != nil {
		t.Fatalf("Transition(us-to-coding): unexpected error: %v", err)
	}
	if tr.Name != "us-to-coding" {
		t.Errorf("name = %q, want %q", tr.Name, "us-to-coding")
	}
	if tr.From != "epic-spec-subpipeline.us-writing-pair.approved" {
		t.Errorf("from = %q, want 3-part ref", tr.From)
	}
	if tr.To != "coding-subpipeline.code-planning-pair.initial" {
		t.Errorf("to = %q, want 3-part ref", tr.To)
	}
	if tr.Trigger != "manual" {
		t.Errorf("trigger = %q, want %q", tr.Trigger, "manual")
	}
	if tr.Cardinality != "one-to-one" {
		t.Errorf("cardinality = %q, want %q", tr.Cardinality, "one-to-one")
	}
}

func TestResolver_Transition_SubPipelineStillWorks(t *testing.T) {
	r := NewResolver(loadPhase2Config(t))

	// Sub-pipeline transitions should still be found.
	tr, err := r.Transition("epic-to-us")
	if err != nil {
		t.Fatalf("Transition(epic-to-us): unexpected error: %v", err)
	}
	if tr.From != "epic-planning-pair.approved" {
		t.Errorf("from = %q, want %q", tr.From, "epic-planning-pair.approved")
	}

	tr, err = r.Transition("code-plan-to-coding")
	if err != nil {
		t.Fatalf("Transition(code-plan-to-coding): unexpected error: %v", err)
	}
	if tr.From != "code-planning-pair.approved" {
		t.Errorf("from = %q, want %q", tr.From, "code-planning-pair.approved")
	}
}

func TestResolver_AvailableTransitions_PipelineTransition(t *testing.T) {
	r := NewResolver(loadPhase2Config(t))

	// US_APPROVED should have "us-to-coding" available.
	got := r.AvailableTransitions("US_APPROVED", nil)
	want := []string{"us-to-coding"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("AvailableTransitions(US_APPROVED, nil) = %v, want %v", got, want)
	}

	// Already executed — should return empty.
	got = r.AvailableTransitions("US_APPROVED", map[string]bool{"us-to-coding": true})
	if len(got) != 0 {
		t.Errorf("AvailableTransitions with executed = %v, want []", got)
	}

	// EPIC_PLAN_APPROVED should have "epic-to-us" available.
	got = r.AvailableTransitions("EPIC_PLAN_APPROVED", nil)
	want = []string{"epic-to-us"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("AvailableTransitions(EPIC_PLAN_APPROVED, nil) = %v, want %v", got, want)
	}

	// CODING_PLAN_APPROVED should have "code-plan-to-coding" available.
	got = r.AvailableTransitions("CODING_PLAN_APPROVED", nil)
	want = []string{"code-plan-to-coding"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("AvailableTransitions(CODING_PLAN_APPROVED, nil) = %v, want %v", got, want)
	}

	// DRAFT_CODE should have no transitions.
	got = r.AvailableTransitions("DRAFT_CODE", nil)
	if len(got) != 0 {
		t.Errorf("AvailableTransitions(DRAFT_CODE, nil) = %v, want []", got)
	}
}

func TestResolver_SprintTerminalStates_Phase2(t *testing.T) {
	r := NewResolver(loadPhase2Config(t))
	got := r.SprintTerminalStates()

	// Pipeline-transition sources: us-writing-pair.approved (US_APPROVED)
	// Sub-pipeline transition sources: epic-planning-pair.approved (EPIC_PLAN_APPROVED),
	//   code-planning-pair.approved (CODING_PLAN_APPROVED)
	// Plus MERGED always included.
	want := []models.TaskStatus{
		"CODING_PLAN_APPROVED",
		"EPIC_PLAN_APPROVED",
		"MERGED",
		"US_APPROVED",
	}
	slices.Sort(want)

	if len(got) != len(want) {
		t.Fatalf("SprintTerminalStates = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("SprintTerminalStates[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResolver_RolePairNames(t *testing.T) {
	r := NewResolver(loadPhase2Config(t))
	got := r.RolePairNames()

	want := []string{"code-planning-pair", "coding-pair", "epic-planning-pair", "us-writing-pair"}
	if len(got) != len(want) {
		t.Fatalf("RolePairNames() = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("RolePairNames()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResolver_TransitionSourcePairs_Phase2(t *testing.T) {
	r := NewResolver(loadPhase2Config(t))
	got := r.TransitionSourcePairs()

	// Sub-pipeline transition sources: epic-planning-pair, code-planning-pair
	// Pipeline-transition sources: us-writing-pair
	want := map[string]bool{
		"epic-planning-pair": true,
		"code-planning-pair": true,
		"us-writing-pair":    true,
	}

	if len(got) != len(want) {
		t.Fatalf("TransitionSourcePairs() = %v, want %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("TransitionSourcePairs() missing %q", k)
		}
	}
}

func TestResolver_IsTransitionSourcePair_Phase2(t *testing.T) {
	r := NewResolver(loadPhase2Config(t))

	sources := []string{"epic-planning-pair", "code-planning-pair", "us-writing-pair"}
	for _, rp := range sources {
		if !r.IsTransitionSourcePair(rp) {
			t.Errorf("IsTransitionSourcePair(%q) = false, want true", rp)
		}
	}

	// coding-pair is a terminal pair, not a transition source
	if r.IsTransitionSourcePair("coding-pair") {
		t.Error("IsTransitionSourcePair(coding-pair) = true, want false")
	}
}

func TestRoleType(t *testing.T) {
	r := NewResolver(loadPhase2Config(t))

	tests := []struct {
		role     string
		wantType string
	}{
		{"coder", "doer"},
		{"code-reviewer", "reviewer"},
		{"orchestrator", "orchestrator"},
	}
	for _, tt := range tests {
		got, err := r.RoleType(tt.role)
		if err != nil {
			t.Errorf("RoleType(%q): unexpected error: %v", tt.role, err)
			continue
		}
		if got != tt.wantType {
			t.Errorf("RoleType(%q) = %q, want %q", tt.role, got, tt.wantType)
		}
	}

	// Unknown role returns error.
	_, err := r.RoleType("unknown-role")
	if err == nil {
		t.Error("RoleType(unknown-role): expected error, got nil")
	}
}

func TestDoerRoleNames(t *testing.T) {
	r := NewResolver(loadPhase2Config(t))
	got := r.DoerRoleNames()
	want := []string{"code-planner", "coder", "epic-planner", "us-writer"}
	if len(got) != len(want) {
		t.Fatalf("DoerRoleNames() = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("DoerRoleNames()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestReviewerRoleNames(t *testing.T) {
	r := NewResolver(loadPhase2Config(t))
	got := r.ReviewerRoleNames()
	want := []string{"code-plan-reviewer", "code-reviewer", "epic-plan-reviewer", "us-reviewer"}
	if len(got) != len(want) {
		t.Fatalf("ReviewerRoleNames() = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("ReviewerRoleNames()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAllRoleNames(t *testing.T) {
	r := NewResolver(loadPhase2Config(t))
	got := r.AllRoleNames()
	want := []string{
		"code-plan-reviewer", "code-planner", "code-reviewer", "coder",
		"epic-plan-reviewer", "epic-planner", "orchestrator", "us-reviewer", "us-writer",
	}
	if len(got) != len(want) {
		t.Fatalf("AllRoleNames() = %v (len %d), want %v (len %d)", got, len(got), want, len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("AllRoleNames()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAllowedOperations(t *testing.T) {
	r := NewResolver(loadPhase2Config(t))
	got, err := r.AllowedOperations("coder")
	if err != nil {
		t.Fatalf("AllowedOperations(coder): %v", err)
	}
	want := []string{"write-checkpoint", "submit-for-review", "mark-blocked", "handoff", "set-task-output"}
	if len(got) != len(want) {
		t.Fatalf("AllowedOperations(coder) = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("AllowedOperations(coder)[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	// Unknown role returns error.
	_, err = r.AllowedOperations("unknown-role")
	if err == nil {
		t.Error("AllowedOperations(unknown-role): expected error, got nil")
	}
}

func TestRoleTimeouts(t *testing.T) {
	r := NewResolver(loadPhase2Config(t))
	got, err := r.RoleTimeouts("coder")
	if err != nil {
		t.Fatalf("RoleTimeouts(coder): %v", err)
	}
	if got.Execution != 2*time.Hour {
		t.Errorf("Execution = %v, want 2h", got.Execution)
	}
	if got.PollInterval != 30*time.Second {
		t.Errorf("PollInterval = %v, want 30s", got.PollInterval)
	}
	if got.MaxWait != 30*time.Minute {
		t.Errorf("MaxWait = %v, want 30m", got.MaxWait)
	}

	// Unknown role returns error.
	_, err = r.RoleTimeouts("unknown-role")
	if err == nil {
		t.Error("RoleTimeouts(unknown-role): expected error, got nil")
	}
}

func TestResolver_TransitionTargetRolePair_PipelineTransition(t *testing.T) {
	r := NewResolver(loadPhase2Config(t))

	// Pipeline-transition target role-pair.
	rp, err := r.TransitionTargetRolePair("us-to-coding")
	if err != nil {
		t.Fatalf("TransitionTargetRolePair(us-to-coding): %v", err)
	}
	if rp != "code-planning-pair" {
		t.Errorf("TransitionTargetRolePair = %q, want %q", rp, "code-planning-pair")
	}

	// Sub-pipeline transition target role-pair should still work.
	rp, err = r.TransitionTargetRolePair("epic-to-us")
	if err != nil {
		t.Fatalf("TransitionTargetRolePair(epic-to-us): %v", err)
	}
	if rp != "us-writing-pair" {
		t.Errorf("TransitionTargetRolePair = %q, want %q", rp, "us-writing-pair")
	}
}

func TestResolver_RoleDisplayName(t *testing.T) {
	r := NewResolver(loadPhase2Config(t))

	// Known role returns display-name.
	if got := r.RoleDisplayName("coder"); got != "Coder" {
		t.Errorf("RoleDisplayName(coder) = %q, want %q", got, "Coder")
	}
	if got := r.RoleDisplayName("code-reviewer"); got != "Code Reviewer" {
		t.Errorf("RoleDisplayName(code-reviewer) = %q, want %q", got, "Code Reviewer")
	}
	if got := r.RoleDisplayName("orchestrator"); got != "Orchestrator" {
		t.Errorf("RoleDisplayName(orchestrator) = %q, want %q", got, "Orchestrator")
	}

	// Unknown role returns key itself.
	if got := r.RoleDisplayName("nonexistent"); got != "nonexistent" {
		t.Errorf("RoleDisplayName(nonexistent) = %q, want %q", got, "nonexistent")
	}
}

// TestMaxInstances_OrchestratorCoercedToOne verifies the spec invariant that
// orchestrator roles always return max-instances=1 regardless of YAML value.
// Regression test for the misconfiguration case where YAML sets max-instances: 2.
func TestMaxInstances_OrchestratorCoercedToOne(t *testing.T) {
	// Build a config where orchestrator explicitly sets max-instances: 2.
	yamlData := []byte(`
pipeline:
  roles:
    orchestrator:
      type: orchestrator
      max-instances: 2
      display-name: "Orchestrator"
    coder:
      type: doer
      display-name: "Coder"
    code-reviewer:
      type: reviewer
      display-name: "Code Reviewer"
  role-pairs:
    coding-pair:
      doer: coder
      reviewer: code-reviewer
      states:
        initial: DRAFT_CODE
        executing: IMPLEMENTING_CODE
        submitted: CODE_READY_FOR_REVIEW
        reviewing: REVIEWING_CODE
        approved: CODE_APPROVED
        rejected: CODE_REJECTED
  sub-pipelines:
    coding-subpipeline:
      steps:
        - coding-pair
  entry-points:
    default: coding-subpipeline.coding-pair
`)
	cfg, err := LoadFromBytes(yamlData)
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}
	r := NewResolver(cfg)

	// Despite YAML saying max-instances: 2, resolver must coerce to 1.
	got, err := r.MaxInstances("orchestrator")
	if err != nil {
		t.Fatalf("MaxInstances(orchestrator): %v", err)
	}
	if got != 1 {
		t.Errorf("MaxInstances(orchestrator) = %d, want 1 (spec invariant: orchestrator singularity)", got)
	}

	// Non-orchestrator roles should honor their YAML value.
	got, err = r.MaxInstances("coder")
	if err != nil {
		t.Fatalf("MaxInstances(coder): %v", err)
	}
	if got != 0 {
		t.Errorf("MaxInstances(coder) = %d, want 0 (unset = unlimited)", got)
	}
}
