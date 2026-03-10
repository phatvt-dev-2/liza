package testhelpers

import (
	"path/filepath"
	"testing"

	"github.com/liza-mas/liza/internal/embedded"
)

// SetupPipelineConfig writes the embedded pipeline.yaml into the .liza/ directory
// of the given tmpDir. Call after SetupLizaDir to make pipeline operations work
// in tests.
func SetupPipelineConfig(t *testing.T, tmpDir string) {
	t.Helper()

	lizaDir := filepath.Join(tmpDir, ".liza")
	if err := embedded.WritePipelineConfig(lizaDir); err != nil {
		t.Fatalf("Failed to write pipeline config: %v", err)
	}
}
