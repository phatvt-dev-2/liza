package testhelpers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/liza-mas/liza/internal/embedded"
)

// SetupPipelineConfig writes the embedded pipeline.yaml into the .liza/ directory
// of the given tmpDir. Creates the .liza directory if it doesn't exist.
func SetupPipelineConfig(t *testing.T, tmpDir string) {
	t.Helper()

	lizaDir := filepath.Join(tmpDir, ".liza")
	if err := os.MkdirAll(lizaDir, 0755); err != nil {
		t.Fatalf("Failed to create .liza directory: %v", err)
	}
	if err := embedded.WritePipelineConfig(lizaDir); err != nil {
		t.Fatalf("Failed to write pipeline config: %v", err)
	}
}
