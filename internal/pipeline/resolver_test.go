package pipeline

import (
	"slices"
	"testing"

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
