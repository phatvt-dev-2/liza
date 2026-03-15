package mcp

import (
	"strings"
	"testing"

	"github.com/liza-mas/liza/internal/pipeline"
)

// testHelpersPipelineYAML defines a minimal pipeline with both standard and
// custom roles to verify resolver-based authorization in authorizeClaimRelease.
var testHelpersPipelineYAML = []byte(`
pipeline:
  roles:
    coder:
      type: doer
      display-name: "Coder"
      allowed-operations:
        - submit-for-review
    code-reviewer:
      type: reviewer
      display-name: "Code Reviewer"
      allowed-operations:
        - submit-verdict
    orchestrator:
      type: orchestrator
      display-name: "Orchestrator"
      allowed-operations:
        - add-tasks
    data-engineer:
      type: doer
      display-name: "Data Engineer"
      allowed-operations:
        - submit-for-review
    security-reviewer:
      type: reviewer
      display-name: "Security Reviewer"
      allowed-operations:
        - submit-verdict
  role-pairs:
    coding-pair:
      doer: coder
      reviewer: code-reviewer
      states:
        initial: READY
        executing: CODING
        submitted: CODE_SUBMITTED
        reviewing: CODE_REVIEWING
        approved: CODE_APPROVED
        rejected: CODE_REJECTED
  sub-pipelines:
    coding:
      steps:
        - coding-pair
      transitions: []
  entry-points:
    default: coding.coding-pair
`)

func testHelpersResolver(t *testing.T) *pipeline.Resolver {
	t.Helper()
	cfg, err := pipeline.LoadFromBytes(testHelpersPipelineYAML)
	if err != nil {
		t.Fatalf("testHelpersResolver: %v", err)
	}
	return pipeline.NewResolver(cfg)
}

func TestAuthorizeClaimRelease(t *testing.T) {
	resolver := testHelpersResolver(t)

	t.Run("standard doer can release doer claim", func(t *testing.T) {
		err := authorizeClaimRelease("coder-1", "doer", resolver)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("standard doer cannot release reviewer claim", func(t *testing.T) {
		err := authorizeClaimRelease("coder-1", "reviewer", resolver)
		if err == nil {
			t.Fatal("expected error for doer releasing reviewer claim")
		}
		if !strings.Contains(err.Error(), "can only release doer claims") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("standard reviewer can release reviewer claim", func(t *testing.T) {
		err := authorizeClaimRelease("code-reviewer-1", "reviewer", resolver)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("standard reviewer cannot release doer claim", func(t *testing.T) {
		err := authorizeClaimRelease("code-reviewer-1", "doer", resolver)
		if err == nil {
			t.Fatal("expected error for reviewer releasing doer claim")
		}
		if !strings.Contains(err.Error(), "can only release reviewer claims") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("orchestrator can release any claim type", func(t *testing.T) {
		for _, claimRole := range []string{"doer", "reviewer", "both"} {
			err := authorizeClaimRelease("orchestrator-1", claimRole, resolver)
			if err != nil {
				t.Errorf("orchestrator should be allowed to release %q claim, got: %v", claimRole, err)
			}
		}
	})

	t.Run("custom YAML doer role passes doer claim release", func(t *testing.T) {
		err := authorizeClaimRelease("data-engineer-1", "doer", resolver)
		if err != nil {
			t.Errorf("custom doer role should authorize doer claim release: %v", err)
		}
	})

	t.Run("custom YAML doer role cannot release reviewer claim", func(t *testing.T) {
		err := authorizeClaimRelease("data-engineer-1", "reviewer", resolver)
		if err == nil {
			t.Fatal("expected error for custom doer releasing reviewer claim")
		}
	})

	t.Run("custom YAML reviewer role passes reviewer claim release", func(t *testing.T) {
		err := authorizeClaimRelease("security-reviewer-1", "reviewer", resolver)
		if err != nil {
			t.Errorf("custom reviewer role should authorize reviewer claim release: %v", err)
		}
	})

	t.Run("custom YAML reviewer role cannot release doer claim", func(t *testing.T) {
		err := authorizeClaimRelease("security-reviewer-1", "doer", resolver)
		if err == nil {
			t.Fatal("expected error for custom reviewer releasing doer claim")
		}
	})

	t.Run("nil resolver rejects all", func(t *testing.T) {
		err := authorizeClaimRelease("coder-1", "doer", nil)
		if err == nil {
			t.Fatal("expected error when resolver is nil")
		}
		if !strings.Contains(err.Error(), "resolver") {
			t.Errorf("expected resolver-related error, got: %v", err)
		}
	})

	t.Run("invalid agent ID format rejected", func(t *testing.T) {
		err := authorizeClaimRelease("invalid", "doer", resolver)
		if err == nil {
			t.Fatal("expected error for invalid agent ID")
		}
	})

	t.Run("unknown role rejected", func(t *testing.T) {
		// Role extracted from agent ID but not in pipeline YAML
		err := authorizeClaimRelease("phantom-1", "doer", resolver)
		if err == nil {
			t.Fatal("expected error for unknown role")
		}
	})
}
