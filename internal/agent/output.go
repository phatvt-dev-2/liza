package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// saveOutput saves the agent output to a file and returns the path.
// ext is the file extension (e.g. "txt" for stdout, "err" for stderr).
func saveOutput(outputsDir, agentID, ext, output string) (string, error) {
	// Create outputs directory if missing
	if err := os.MkdirAll(outputsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create outputs directory: %w", err)
	}

	// Generate filename with timestamp (same format as savePrompt)
	timestamp := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.%s", agentID, timestamp, ext)
	filePath := filepath.Join(outputsDir, filename)

	// Write output
	if err := os.WriteFile(filePath, []byte(output), 0644); err != nil {
		return "", fmt.Errorf("failed to write output file: %w", err)
	}

	return filePath, nil
}
