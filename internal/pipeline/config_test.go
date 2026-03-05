package pipeline

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/models"
)

func TestLoad_ValidConfig(t *testing.T) {
	cfg, err := Load("testdata/valid-coding-subpipeline.yaml")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	// Verify role-pairs parsed.
	if len(cfg.Pipeline.RolePairs) != 2 {
		t.Fatalf("expected 2 role-pairs, got %d", len(cfg.Pipeline.RolePairs))
	}
	if _, ok := cfg.Pipeline.RolePairs["coding-pair"]; !ok {
		t.Error("missing role-pair coding-pair")
	}
	if _, ok := cfg.Pipeline.RolePairs["code-planning-pair"]; !ok {
		t.Error("missing role-pair code-planning-pair")
	}

	// Verify agent-roles parsed.
	if len(cfg.Pipeline.AgentRoles) != 4 {
		t.Fatalf("expected 4 agent-roles, got %d", len(cfg.Pipeline.AgentRoles))
	}

	// Verify sub-pipelines parsed.
	sp, ok := cfg.Pipeline.SubPipelines["coding-subpipeline"]
	if !ok {
		t.Fatal("missing sub-pipeline coding-subpipeline")
	}
	if len(sp.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(sp.Steps))
	}
	if len(sp.Transitions) != 1 {
		t.Fatalf("expected 1 transition, got %d", len(sp.Transitions))
	}

	// Verify entry-points.
	if ep, ok := cfg.Pipeline.EntryPoints["detailed-spec"]; !ok {
		t.Error("missing entry-point detailed-spec")
	} else if ep != "coding-subpipeline.code-planning-pair" {
		t.Errorf("entry-point value = %q, want %q", ep, "coding-subpipeline.code-planning-pair")
	}
}

func TestLoad_MissingStateField(t *testing.T) {
	yaml := `
pipeline:
  agent-roles:
    coder: "Coder"
    reviewer: "Reviewer"
  role-pairs:
    pair-a:
      doer: coder
      reviewer: reviewer
      states:
        initial: S1
        executing: ""
        submitted: S3
        reviewing: S4
        approved: S5
        rejected: S6
  sub-pipelines: {}
  entry-points: {}
`
	cfg := writeTemp(t, yaml)
	_, err := Load(cfg)
	if err == nil {
		t.Fatal("expected error for empty state field")
	}
	assertContains(t, err.Error(), "executing state is empty")
}

func TestLoad_DuplicateStateNames(t *testing.T) {
	yaml := `
pipeline:
  agent-roles:
    a: "A"
    b: "B"
  role-pairs:
    pair-a:
      doer: a
      reviewer: b
      states:
        initial: DRAFT
        executing: WORKING
        submitted: SUBMITTED
        reviewing: REVIEWING
        approved: APPROVED
        rejected: REJECTED
    pair-b:
      doer: a
      reviewer: b
      states:
        initial: OTHER
        executing: WORKING
        submitted: SUB2
        reviewing: REV2
        approved: APP2
        rejected: REJ2
  sub-pipelines:
    sp1:
      steps: [pair-a]
      transitions: []
    sp2:
      steps: [pair-b]
      transitions: []
  entry-points: {}
`
	cfg := writeTemp(t, yaml)
	_, err := Load(cfg)
	if err == nil {
		t.Fatal("expected error for duplicate state name")
	}
	assertContains(t, err.Error(), "duplicate state name")
	assertContains(t, err.Error(), "WORKING")
}

func TestLoad_InvalidTransitionReference(t *testing.T) {
	yaml := `
pipeline:
  agent-roles:
    a: "A"
    b: "B"
  role-pairs:
    pair-a:
      doer: a
      reviewer: b
      states:
        initial: S1
        executing: S2
        submitted: S3
        reviewing: S4
        approved: S5
        rejected: S6
  sub-pipelines:
    sp:
      steps: [pair-a]
      transitions:
        - name: bad-transition
          from: nonexistent-pair.approved
          to: pair-a.initial
          trigger: manual
          cardinality: per-subtask
  entry-points: {}
`
	cfg := writeTemp(t, yaml)
	_, err := Load(cfg)
	if err == nil {
		t.Fatal("expected error for invalid transition reference")
	}
	assertContains(t, err.Error(), "nonexistent-pair")
}

func TestLoad_UnknownCardinality(t *testing.T) {
	yaml := `
pipeline:
  agent-roles:
    a: "A"
    b: "B"
  role-pairs:
    pair-a:
      doer: a
      reviewer: b
      states:
        initial: S1
        executing: S2
        submitted: S3
        reviewing: S4
        approved: S5
        rejected: S6
    pair-b:
      doer: a
      reviewer: b
      states:
        initial: T1
        executing: T2
        submitted: T3
        reviewing: T4
        approved: T5
        rejected: T6
  sub-pipelines:
    sp:
      steps: [pair-a, pair-b]
      transitions:
        - name: t1
          from: pair-a.approved
          to: pair-b.initial
          trigger: manual
          cardinality: many-to-many
  entry-points: {}
`
	cfg := writeTemp(t, yaml)
	_, err := Load(cfg)
	if err == nil {
		t.Fatal("expected error for unknown cardinality")
	}
	assertContains(t, err.Error(), "cardinality")
	assertContains(t, err.Error(), "many-to-many")
}

func TestLoad_UnknownTrigger(t *testing.T) {
	yaml := `
pipeline:
  agent-roles:
    a: "A"
    b: "B"
  role-pairs:
    pair-a:
      doer: a
      reviewer: b
      states:
        initial: S1
        executing: S2
        submitted: S3
        reviewing: S4
        approved: S5
        rejected: S6
    pair-b:
      doer: a
      reviewer: b
      states:
        initial: T1
        executing: T2
        submitted: T3
        reviewing: T4
        approved: T5
        rejected: T6
  sub-pipelines:
    sp:
      steps: [pair-a, pair-b]
      transitions:
        - name: t1
          from: pair-a.approved
          to: pair-b.initial
          trigger: cron
          cardinality: per-subtask
  entry-points: {}
`
	cfg := writeTemp(t, yaml)
	_, err := Load(cfg)
	if err == nil {
		t.Fatal("expected error for unknown trigger")
	}
	assertContains(t, err.Error(), "trigger")
	assertContains(t, err.Error(), "cron")
}

// Blocker fix 1: role-pair cannot appear in multiple sub-pipelines.
func TestLoad_RolePairInMultipleSubPipelines(t *testing.T) {
	yaml := `
pipeline:
  agent-roles:
    a: "A"
    b: "B"
  role-pairs:
    shared-pair:
      doer: a
      reviewer: b
      states:
        initial: S1
        executing: S2
        submitted: S3
        reviewing: S4
        approved: S5
        rejected: S6
    other-pair:
      doer: a
      reviewer: b
      states:
        initial: T1
        executing: T2
        submitted: T3
        reviewing: T4
        approved: T5
        rejected: T6
  sub-pipelines:
    sp1:
      steps: [shared-pair]
      transitions: []
    sp2:
      steps: [shared-pair, other-pair]
      transitions: []
  entry-points: {}
`
	cfg := writeTemp(t, yaml)
	_, err := Load(cfg)
	if err == nil {
		t.Fatal("expected error for role-pair in multiple sub-pipelines")
	}
	assertContains(t, err.Error(), "shared-pair")
	assertContains(t, err.Error(), "multiple sub-pipelines")
}

// Blocker fix 2: entry-point role-pair must be a step of the referenced sub-pipeline.
func TestLoad_EntryPointRolePairNotInSubPipeline(t *testing.T) {
	yaml := `
pipeline:
  agent-roles:
    a: "A"
    b: "B"
  role-pairs:
    pair-a:
      doer: a
      reviewer: b
      states:
        initial: S1
        executing: S2
        submitted: S3
        reviewing: S4
        approved: S5
        rejected: S6
    pair-b:
      doer: a
      reviewer: b
      states:
        initial: T1
        executing: T2
        submitted: T3
        reviewing: T4
        approved: T5
        rejected: T6
  sub-pipelines:
    sp1:
      steps: [pair-a]
      transitions: []
    sp2:
      steps: [pair-b]
      transitions: []
  entry-points:
    bad-entry: sp1.pair-b
`
	cfg := writeTemp(t, yaml)
	_, err := Load(cfg)
	if err == nil {
		t.Fatal("expected error for entry-point referencing role-pair not in sub-pipeline")
	}
	assertContains(t, err.Error(), "pair-b")
	assertContains(t, err.Error(), "not a step of sub-pipeline")
}

func TestLoad_EntryPointNonexistentSubPipeline(t *testing.T) {
	yaml := `
pipeline:
  agent-roles:
    a: "A"
    b: "B"
  role-pairs:
    pair-a:
      doer: a
      reviewer: b
      states:
        initial: S1
        executing: S2
        submitted: S3
        reviewing: S4
        approved: S5
        rejected: S6
  sub-pipelines:
    sp1:
      steps: [pair-a]
      transitions: []
  entry-points:
    bad-entry: nonexistent.pair-a
`
	cfg := writeTemp(t, yaml)
	_, err := Load(cfg)
	if err == nil {
		t.Fatal("expected error for entry-point referencing nonexistent sub-pipeline")
	}
	assertContains(t, err.Error(), "nonexistent")
	assertContains(t, err.Error(), "not found")
}

func TestLoad_DoerNotInAgentRoles(t *testing.T) {
	yaml := `
pipeline:
  agent-roles:
    a: "A"
  role-pairs:
    pair-a:
      doer: unknown
      reviewer: a
      states:
        initial: S1
        executing: S2
        submitted: S3
        reviewing: S4
        approved: S5
        rejected: S6
  sub-pipelines: {}
  entry-points: {}
`
	cfg := writeTemp(t, yaml)
	_, err := Load(cfg)
	if err == nil {
		t.Fatal("expected error for doer not in agent-roles")
	}
	assertContains(t, err.Error(), "doer")
	assertContains(t, err.Error(), "unknown")
}

func TestLoad_TransitionFromPairNotInSubPipelineSteps(t *testing.T) {
	yaml := `
pipeline:
  agent-roles:
    a: "A"
    b: "B"
  role-pairs:
    pair-a:
      doer: a
      reviewer: b
      states:
        initial: S1
        executing: S2
        submitted: S3
        reviewing: S4
        approved: S5
        rejected: S6
    pair-b:
      doer: a
      reviewer: b
      states:
        initial: T1
        executing: T2
        submitted: T3
        reviewing: T4
        approved: T5
        rejected: T6
    pair-c:
      doer: a
      reviewer: b
      states:
        initial: U1
        executing: U2
        submitted: U3
        reviewing: U4
        approved: U5
        rejected: U6
  sub-pipelines:
    sp1:
      steps: [pair-a, pair-b]
      transitions:
        - name: bad
          from: pair-c.approved
          to: pair-b.initial
          trigger: manual
          cardinality: per-subtask
    sp2:
      steps: [pair-c]
      transitions: []
  entry-points: {}
`
	cfg := writeTemp(t, yaml)
	_, err := Load(cfg)
	if err == nil {
		t.Fatal("expected error for transition from role-pair not in sub-pipeline steps")
	}
	assertContains(t, err.Error(), "pair-c")
	assertContains(t, err.Error(), "not a step")
}

func TestLoadFrozen_NoFile(t *testing.T) {
	cfg, err := LoadFrozen(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil config for missing file")
	}
}

func TestLoadFrozen_ValidFile(t *testing.T) {
	dir := t.TempDir()
	lizaDir := filepath.Join(dir, ".liza")
	if err := os.MkdirAll(lizaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile("testdata/valid-coding-subpipeline.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lizaDir, "pipeline.yaml"), src, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrozen(dir)
	if err != nil {
		t.Fatalf("LoadFrozen failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Pipeline.RolePairs) != 2 {
		t.Errorf("expected 2 role-pairs, got %d", len(cfg.Pipeline.RolePairs))
	}
}

// --- Resolver tests ---

func loadTestConfig(t *testing.T) *PipelineConfig {
	t.Helper()
	cfg, err := Load("testdata/valid-coding-subpipeline.yaml")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	return cfg
}

func TestResolver_InitialStatus(t *testing.T) {
	r := NewResolver(loadTestConfig(t))
	got, err := r.InitialStatus("coding-pair")
	if err != nil {
		t.Fatal(err)
	}
	if got != "DRAFT_CODE" {
		t.Errorf("InitialStatus(coding-pair) = %q, want %q", got, "DRAFT_CODE")
	}
}

func TestResolver_ExecutingStatus(t *testing.T) {
	r := NewResolver(loadTestConfig(t))
	got, err := r.ExecutingStatus("code-planning-pair")
	if err != nil {
		t.Fatal(err)
	}
	if got != "CODE_PLANNING" {
		t.Errorf("ExecutingStatus(code-planning-pair) = %q, want %q", got, "CODE_PLANNING")
	}
}

func TestResolver_AllStatusMethods(t *testing.T) {
	r := NewResolver(loadTestConfig(t))
	tests := []struct {
		method   string
		rolePair string
		want     models.TaskStatus
	}{
		{"Initial", "coding-pair", "DRAFT_CODE"},
		{"Executing", "coding-pair", "IMPLEMENTING_CODE"},
		{"Submitted", "coding-pair", "CODE_READY_FOR_REVIEW"},
		{"Reviewing", "coding-pair", "REVIEWING_CODE"},
		{"Approved", "coding-pair", "CODE_APPROVED"},
		{"Rejected", "coding-pair", "CODE_REJECTED"},
		{"Initial", "code-planning-pair", "DRAFT_CODING_PLAN"},
		{"Executing", "code-planning-pair", "CODE_PLANNING"},
		{"Submitted", "code-planning-pair", "CODING_PLAN_TO_REVIEW"},
		{"Reviewing", "code-planning-pair", "REVIEWING_CODING_PLAN"},
		{"Approved", "code-planning-pair", "CODING_PLAN_APPROVED"},
		{"Rejected", "code-planning-pair", "CODING_PLAN_REJECTED"},
	}
	for _, tt := range tests {
		var got models.TaskStatus
		var err error
		switch tt.method {
		case "Initial":
			got, err = r.InitialStatus(tt.rolePair)
		case "Executing":
			got, err = r.ExecutingStatus(tt.rolePair)
		case "Submitted":
			got, err = r.SubmittedStatus(tt.rolePair)
		case "Reviewing":
			got, err = r.ReviewingStatus(tt.rolePair)
		case "Approved":
			got, err = r.ApprovedStatus(tt.rolePair)
		case "Rejected":
			got, err = r.RejectedStatus(tt.rolePair)
		}
		if err != nil {
			t.Errorf("%s(%q): unexpected error: %v", tt.method, tt.rolePair, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%s(%q) = %q, want %q", tt.method, tt.rolePair, got, tt.want)
		}
	}
}

func TestResolver_StatusUnknownRolePair(t *testing.T) {
	r := NewResolver(loadTestConfig(t))
	_, err := r.InitialStatus("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown role-pair")
	}
}

func TestResolver_TransitionMap(t *testing.T) {
	r := NewResolver(loadTestConfig(t))
	tm := r.TransitionMap()

	// Check code-planning-pair transitions.
	assertTransitions(t, tm, "DRAFT_CODING_PLAN", []string{"CODE_PLANNING"})
	assertTransitions(t, tm, "CODE_PLANNING", []string{"CODING_PLAN_TO_REVIEW"})
	assertTransitions(t, tm, "CODING_PLAN_TO_REVIEW", []string{"REVIEWING_CODING_PLAN"})
	assertTransitions(t, tm, "REVIEWING_CODING_PLAN", []string{"CODING_PLAN_APPROVED", "CODING_PLAN_REJECTED"})
	assertTransitions(t, tm, "CODING_PLAN_REJECTED", []string{"DRAFT_CODING_PLAN"})
	assertTransitions(t, tm, "CODING_PLAN_APPROVED", []string{})

	// Check coding-pair transitions.
	assertTransitions(t, tm, "DRAFT_CODE", []string{"IMPLEMENTING_CODE"})
	assertTransitions(t, tm, "IMPLEMENTING_CODE", []string{"CODE_READY_FOR_REVIEW"})
	assertTransitions(t, tm, "CODE_READY_FOR_REVIEW", []string{"REVIEWING_CODE"})
	assertTransitions(t, tm, "REVIEWING_CODE", []string{"CODE_APPROVED", "CODE_REJECTED"})
	assertTransitions(t, tm, "CODE_REJECTED", []string{"DRAFT_CODE"})
	assertTransitions(t, tm, "CODE_APPROVED", []string{})
}

func TestResolver_AllDeclaredStates(t *testing.T) {
	r := NewResolver(loadTestConfig(t))
	states := r.AllDeclaredStates()
	// 2 role-pairs * 6 states = 12.
	if len(states) != 12 {
		t.Errorf("expected 12 declared states, got %d", len(states))
	}
	expected := []models.TaskStatus{
		"DRAFT_CODING_PLAN", "CODE_PLANNING", "CODING_PLAN_TO_REVIEW",
		"REVIEWING_CODING_PLAN", "CODING_PLAN_APPROVED", "CODING_PLAN_REJECTED",
		"DRAFT_CODE", "IMPLEMENTING_CODE", "CODE_READY_FOR_REVIEW",
		"REVIEWING_CODE", "CODE_APPROVED", "CODE_REJECTED",
	}
	for _, e := range expected {
		if !slices.Contains(states, e) {
			t.Errorf("missing declared state %q", e)
		}
	}
}

func TestResolver_SprintTerminalStates(t *testing.T) {
	r := NewResolver(loadTestConfig(t))
	got := r.SprintTerminalStates()
	want := []models.TaskStatus{"CODING_PLAN_APPROVED", "MERGED"}
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

func TestResolver_RolePair(t *testing.T) {
	r := NewResolver(loadTestConfig(t))
	rp, err := r.RolePair("coding-pair")
	if err != nil {
		t.Fatal(err)
	}
	if rp.Doer != "coder" {
		t.Errorf("doer = %q, want %q", rp.Doer, "coder")
	}
	if rp.Reviewer != "code-reviewer" {
		t.Errorf("reviewer = %q, want %q", rp.Reviewer, "code-reviewer")
	}
}

func TestResolver_DoerReviewerRole(t *testing.T) {
	r := NewResolver(loadTestConfig(t))
	doer, err := r.DoerRole("code-planning-pair")
	if err != nil {
		t.Fatal(err)
	}
	if doer != "code-planner" {
		t.Errorf("DoerRole = %q, want %q", doer, "code-planner")
	}
	reviewer, err := r.ReviewerRole("code-planning-pair")
	if err != nil {
		t.Fatal(err)
	}
	if reviewer != "code-plan-reviewer" {
		t.Errorf("ReviewerRole = %q, want %q", reviewer, "code-plan-reviewer")
	}
}

func TestResolver_Transition(t *testing.T) {
	r := NewResolver(loadTestConfig(t))
	tr, err := r.Transition("code-plan-to-coding")
	if err != nil {
		t.Fatal(err)
	}
	if tr.From != "code-planning-pair.approved" {
		t.Errorf("from = %q, want %q", tr.From, "code-planning-pair.approved")
	}
	if tr.To != "coding-pair.initial" {
		t.Errorf("to = %q, want %q", tr.To, "coding-pair.initial")
	}
	if tr.Trigger != "manual" {
		t.Errorf("trigger = %q, want %q", tr.Trigger, "manual")
	}
	if tr.Cardinality != "per-subtask" {
		t.Errorf("cardinality = %q, want %q", tr.Cardinality, "per-subtask")
	}
}

func TestResolver_TransitionUnknown(t *testing.T) {
	r := NewResolver(loadTestConfig(t))
	_, err := r.Transition("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown transition")
	}
}

func TestResolver_AvailableTransitions(t *testing.T) {
	r := NewResolver(loadTestConfig(t))

	// CODING_PLAN_APPROVED with no executed transitions.
	got := r.AvailableTransitions("CODING_PLAN_APPROVED", nil)
	if len(got) != 1 || got[0] != "code-plan-to-coding" {
		t.Errorf("AvailableTransitions(CODING_PLAN_APPROVED, nil) = %v, want [code-plan-to-coding]", got)
	}

	// Already executed.
	got = r.AvailableTransitions("CODING_PLAN_APPROVED", map[string]bool{"code-plan-to-coding": true})
	if len(got) != 0 {
		t.Errorf("AvailableTransitions with executed = %v, want []", got)
	}

	// State with no transitions.
	got = r.AvailableTransitions("DRAFT_CODE", nil)
	if len(got) != 0 {
		t.Errorf("AvailableTransitions(DRAFT_CODE, nil) = %v, want []", got)
	}
}

func TestResolver_TransitionTargetRolePair(t *testing.T) {
	r := NewResolver(loadTestConfig(t))
	rp, err := r.TransitionTargetRolePair("code-plan-to-coding")
	if err != nil {
		t.Fatal(err)
	}
	if rp != "coding-pair" {
		t.Errorf("TransitionTargetRolePair = %q, want %q", rp, "coding-pair")
	}
}

func TestResolver_IsDeclaredState(t *testing.T) {
	r := NewResolver(loadTestConfig(t))
	if !r.IsDeclaredState("DRAFT_CODE") {
		t.Error("DRAFT_CODE should be a declared state")
	}
	if r.IsDeclaredState("NONEXISTENT") {
		t.Error("NONEXISTENT should not be a declared state")
	}
}

// --- helpers ---

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "pipeline.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("error %q does not contain %q", got, want)
	}
}

func assertTransitions(t *testing.T, tm map[models.TaskStatus][]models.TaskStatus, from string, wantStrs []string) {
	t.Helper()
	got := tm[models.TaskStatus(from)]
	want := make([]models.TaskStatus, len(wantStrs))
	for i, s := range wantStrs {
		want[i] = models.TaskStatus(s)
	}
	slices.Sort(got)
	slices.Sort(want)
	if len(got) != len(want) {
		t.Errorf("transitions from %s: got %v, want %v", from, got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("transitions from %s: got %v, want %v", from, got, want)
			return
		}
	}
}
