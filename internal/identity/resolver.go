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

// ValidateFormat validates agent ID format: {role}-{number}
// The role can contain hyphens (e.g., "code-reviewer"), but the number must be numeric.
func ValidateFormat(agentID string) error {
	if agentID == "" {
		return fmt.Errorf("agent ID required")
	}

	// Find the last hyphen to split role and number
	lastHyphen := strings.LastIndex(agentID, "-")
	if lastHyphen == -1 {
		return fmt.Errorf("invalid agent ID format (expected {role}-{number}): %s", agentID)
	}

	// Extract role and number
	role := agentID[:lastHyphen]
	numStr := agentID[lastHyphen+1:]

	// Validate role doesn't end with hyphen (no consecutive hyphens allowed)
	if role == "" || strings.HasSuffix(role, "-") {
		return fmt.Errorf("agent ID suffix must be numeric: %s", agentID)
	}

	// Validate number suffix exists and is numeric
	if numStr == "" {
		return fmt.Errorf("agent ID suffix must be numeric: %s", agentID)
	}

	// Validate number is numeric and positive
	num, err := strconv.Atoi(numStr)
	if err != nil || num < 0 {
		return fmt.Errorf("agent ID suffix must be numeric: %s", agentID)
	}

	return nil
}

// ValidateRole validates agent ID matches expected role
func ValidateRole(agentID, expectedRole string) error {
	// First validate format
	if err := ValidateFormat(agentID); err != nil {
		return err
	}

	// Extract role from agent ID
	role, err := ExtractRole(agentID)
	if err != nil {
		return err
	}

	// Check if role matches
	if role != expectedRole {
		return fmt.Errorf("agent ID role mismatch (ID=%s, expected=%s)", role, expectedRole)
	}

	return nil
}

// ExtractRole extracts role from agent ID (e.g., "coder-1" -> "coder")
func ExtractRole(agentID string) (string, error) {
	if agentID == "" {
		return "", fmt.Errorf("agent ID required")
	}

	// Find the last hyphen
	lastHyphen := strings.LastIndex(agentID, "-")
	if lastHyphen == -1 {
		return "", fmt.Errorf("invalid agent ID format (expected {role}-{number}): %s", agentID)
	}

	return agentID[:lastHyphen], nil
}

// ExtractNumber extracts number from agent ID (e.g., "coder-1" -> 1)
func ExtractNumber(agentID string) (int, error) {
	if agentID == "" {
		return 0, fmt.Errorf("agent ID required")
	}

	// Find the last hyphen
	lastHyphen := strings.LastIndex(agentID, "-")
	if lastHyphen == -1 {
		return 0, fmt.Errorf("invalid agent ID format (expected {role}-{number}): %s", agentID)
	}

	// Extract and parse number
	numStr := agentID[lastHyphen+1:]
	num, err := strconv.Atoi(numStr)
	if err != nil || num < 0 {
		return 0, fmt.Errorf("agent ID suffix must be numeric: %s", agentID)
	}

	return num, nil
}
