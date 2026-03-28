package agent

// validCLIs is the canonical list of supported CLI backends.
var validCLIs = []string{"claude", "codex", "gemini", "mistral", "kimi"}

// ValidCLIs returns the supported CLI backends. Returns a fresh copy to prevent mutation.
func ValidCLIs() []string {
	out := make([]string, len(validCLIs))
	copy(out, validCLIs)
	return out
}

// DefaultCLI is the CLI used when none is specified.
const DefaultCLI = "claude"
