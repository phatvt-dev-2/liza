package agent

import (
	"os"
	"sort"
	"strings"
)

// SecretMasker replaces known secret values with a redaction marker in text.
// It collects secret values from environment variables at construction time
// and applies longest-first replacement to handle overlapping values.
type SecretMasker struct {
	// secrets holds non-empty env values whose keys matched a sensitive pattern,
	// sorted longest-first so overlapping substrings are replaced correctly.
	secrets []string
}

// secretKeyMatchers defines the rules for identifying sensitive environment variable names.
// Each matcher returns true if the given uppercase key should be treated as secret.
//
// This is an allowlist for precision over recall — unrecognized secret patterns
// (e.g. _CREDENTIALS, _PRIVATE_KEY) are unmasked by default. Extend as needed.
var secretKeyMatchers = []func(string) bool{
	// Exact matches
	func(k string) bool { return k == "API_KEY" || k == "SECRET_KEY" },

	// Suffix patterns: _API_KEY, _APIKEY, _TOKEN, _SECRET, _PASSWORD
	func(k string) bool {
		return strings.HasSuffix(k, "_API_KEY") ||
			strings.HasSuffix(k, "_APIKEY") ||
			strings.HasSuffix(k, "_TOKEN") ||
			strings.HasSuffix(k, "_SECRET") ||
			strings.HasSuffix(k, "_PASSWORD")
	},

	// Prefix: SECRET_
	func(k string) bool { return strings.HasPrefix(k, "SECRET_") },

	// Provider-specific exact names. Some overlap with suffix matchers above
	// (e.g. ANTHROPIC_API_KEY matches _API_KEY). The switch covers keys that
	// don't follow suffix patterns (e.g. DOCKER_PASSWORD, GH_TOKEN).
	func(k string) bool {
		switch k {
		case "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GOOGLE_API_KEY",
			"GEMINI_API_KEY", "MISTRAL_API_KEY", "MOONSHOT_API_KEY",
			"AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
			"GITHUB_TOKEN", "GH_TOKEN", "GITLAB_TOKEN",
			"NPM_TOKEN", "DOCKER_PASSWORD":
			return true
		}
		return false
	},
}

const (
	redactionMarker = "***"
	// minSecretLen avoids false positives from short env values (e.g. "v1")
	// that might match legitimate log content.
	minSecretLen = 8
)

// isSecretKey reports whether the environment variable name looks sensitive.
func isSecretKey(key string) bool {
	upper := strings.ToUpper(key)
	for _, match := range secretKeyMatchers {
		if match(upper) {
			return true
		}
	}
	return false
}

// NewSecretMasker builds a masker from the current process environment.
// It snapshots os.Environ() at call time; later env changes are not reflected.
func NewSecretMasker() *SecretMasker {
	return newSecretMaskerFromEnv(os.Environ())
}

// newSecretMaskerFromEnv builds a masker from an explicit environ slice (for testing).
func newSecretMaskerFromEnv(environ []string) *SecretMasker {
	seen := make(map[string]struct{})
	var secrets []string

	for _, entry := range environ {
		k, v, ok := strings.Cut(entry, "=")
		if !ok || v == "" || len(v) < minSecretLen {
			continue
		}
		if !isSecretKey(k) {
			continue
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		secrets = append(secrets, v)
	}

	// Sort longest first so overlapping values are replaced correctly.
	sort.Slice(secrets, func(i, j int) bool {
		return len(secrets[i]) > len(secrets[j])
	})

	return &SecretMasker{secrets: secrets}
}

// MaskText replaces all occurrences of collected secret values in text.
func (m *SecretMasker) MaskText(text string) string {
	if len(m.secrets) == 0 {
		return text
	}
	result := text
	for _, secret := range m.secrets {
		result = strings.ReplaceAll(result, secret, redactionMarker)
	}
	return result
}
