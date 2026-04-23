package paths

import (
	"path/filepath"
	"regexp"
	"strings"
)

var nonSlugChars = regexp.MustCompile(`[^a-z0-9-]+`)
var multiDash = regexp.MustCompile(`-{2,}`)

// GoalSlug extracts a filesystem-safe slug from a goal SpecRef path.
// The result is lowercase kebab-case: spaces, uppercase, and special characters
// are normalized so the slug is safe for unquoted shell paths.
//
// Example: "specs/goals/20260417-cross-architect-blocked-rewake.md" → "20260417-cross-architect-blocked-rewake"
// Example: "specs/build/0 - Vision.md" → "0-vision"
func GoalSlug(specRef string) string {
	if specRef == "" {
		return ""
	}
	base := filepath.Base(specRef)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")
	slug = nonSlugChars.ReplaceAllString(slug, "-")
	slug = multiDash.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "unnamed"
	}
	return slug
}
