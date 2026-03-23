package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// saveTimestampedFile writes content to dir/<agentID>-<timestamp>.<ext> and returns the path.
func saveTimestampedFile(dir, agentID, ext, content string) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create directory %s: %w", dir, err)
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	filePath := filepath.Join(dir, fmt.Sprintf("%s-%s.%s", agentID, timestamp, ext))

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write file %s: %w", filePath, err)
	}

	return filePath, nil
}

func saveOutput(outputsDir, agentID, ext, output string, masker *SecretMasker) (string, error) {
	masked := output
	if masker != nil {
		masked = masker.MaskText(output)
	}
	return saveTimestampedFile(outputsDir, agentID, ext, masked)
}
