package paths

import "testing"

func TestGoalSlug(t *testing.T) {
	tests := []struct {
		specRef string
		want    string
	}{
		{"specs/goals/20260417-cross-architect-blocked-rewake.md", "20260417-cross-architect-blocked-rewake"},
		{"specs/goals/20260405-integration-pipeline.md", "20260405-integration-pipeline"},
		{"specs/goals/feature.md", "feature"},
		{"feature.md", "feature"},
		{"", ""},
		// Sanitization: spaces, uppercase, special chars
		{"specs/build/0 - Vision.md", "0-vision"},
		{"specs/goals/My Feature (v2).md", "my-feature-v2"},
		{"specs/goals/foo.bar.md", "foo-bar"},
		{"specs/goals/UPPER_CASE.md", "upper-case"},
		{"specs/goals/---leading-trailing---.md", "leading-trailing"},
		{"specs/goals/!!!.md", "unnamed"},
		{"specs/goals/日本語.md", "unnamed"},
	}
	for _, tt := range tests {
		got := GoalSlug(tt.specRef)
		if got != tt.want {
			t.Errorf("GoalSlug(%q) = %q, want %q", tt.specRef, got, tt.want)
		}
	}
}
