package identity

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds agent ID resolution configuration
type Config struct {
	FlagValue    string // Value from CLI flag
	DefaultValue string // Fallback value
	Required     bool   // If true, error if no value resolved
}

// Resolve resolves agent ID from multiple sources with priority:
// 1. CLI flag (if provided)
// 2. LIZA_AGENT_ID environment variable (if set)
// 3. Default value (if provided)
// 4. Empty string or error (based on Required)
func Resolve(config Config) (string, error) {
	// Priority 1: Flag value (trim whitespace)
	flagValue := strings.TrimSpace(config.FlagValue)
	if flagValue != "" {
		return flagValue, nil
	}

	// Priority 2: Environment variable
	envValue := strings.TrimSpace(os.Getenv("LIZA_AGENT_ID"))
	if envValue != "" {
		return envValue, nil
	}

	// Priority 3: Default value
	if config.DefaultValue != "" {
		return config.DefaultValue, nil
	}

	// Priority 4: Error if required, empty string otherwise
	if config.Required {
		return "", fmt.Errorf("agent ID required (use --agent-id flag or set LIZA_AGENT_ID environment variable)")
	}

	return "", nil
}

// parseAgentID splits an agent ID into its role and numeric suffix.
// Returns an error if the ID is empty, missing a hyphen, has an empty/trailing-hyphen
// role, or has a non-numeric suffix.
func parseAgentID(agentID string) (role string, number int, err error) {
	if agentID == "" {
		return "", 0, fmt.Errorf("agent ID required")
	}

	lastHyphen := strings.LastIndex(agentID, "-")
	if lastHyphen == -1 {
		return "", 0, fmt.Errorf("invalid agent ID format (expected {role}-{number}): %s", agentID)
	}

	role = agentID[:lastHyphen]
	if role == "" || strings.HasSuffix(role, "-") {
		return "", 0, fmt.Errorf("agent ID suffix must be numeric: %s", agentID)
	}

	numStr := agentID[lastHyphen+1:]
	if numStr == "" {
		return "", 0, fmt.Errorf("agent ID suffix must be numeric: %s", agentID)
	}

	number, err = strconv.Atoi(numStr)
	if err != nil || number < 0 {
		return "", 0, fmt.Errorf("agent ID suffix must be numeric: %s", agentID)
	}

	return role, number, nil
}

// ValidateFormat validates agent ID format: {role}-{number}
// The role can contain hyphens (e.g., "code-reviewer"), but the number must be numeric.
func ValidateFormat(agentID string) error {
	_, _, err := parseAgentID(agentID)
	return err
}

// ValidateRole validates agent ID matches expected role
func ValidateRole(agentID, expectedRole string) error {
	role, _, err := parseAgentID(agentID)
	if err != nil {
		return err
	}
	if role != expectedRole {
		return fmt.Errorf("agent ID role mismatch (ID=%s, expected=%s)", role, expectedRole)
	}
	return nil
}

// ExtractRole extracts role from agent ID (e.g., "coder-1" -> "coder")
func ExtractRole(agentID string) (string, error) {
	role, _, err := parseAgentID(agentID)
	return role, err
}

// ExtractNumber extracts number from agent ID (e.g., "coder-1" -> 1)
func ExtractNumber(agentID string) (int, error) {
	_, number, err := parseAgentID(agentID)
	return number, err
}
