package embedded

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestArtifactConsistency verifies that repo master files are byte-identical
// to their embedded copies under internal/embedded/. This catches drift when
// a master is modified without running `make sync-embedded`.
func TestArtifactConsistency(t *testing.T) {
	repoRoot := findRepoRoot(t)
	embeddedDir := filepath.Join(repoRoot, "internal", "embedded")

	t.Run("contracts", func(t *testing.T) {
		masterDir := filepath.Join(repoRoot, "contracts")
		embDir := filepath.Join(embeddedDir, "contracts")

		// sync-embedded copies contracts/*.md (top-level .md files only)
		entries, err := os.ReadDir(masterDir)
		if err != nil {
			t.Fatalf("reading contracts dir: %v", err)
		}

		var checked int
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			checked++
			compareMasterToEmbedded(t, filepath.Join(masterDir, e.Name()), filepath.Join(embDir, e.Name()))
		}
		if checked == 0 {
			t.Fatal("no .md files found in contracts/")
		}
	})

	t.Run("skills", func(t *testing.T) {
		masterDir := filepath.Join(repoRoot, "skills")
		embDir := filepath.Join(embeddedDir, "skills")

		var checked int
		err := filepath.WalkDir(masterDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "__pycache__" {
					return filepath.SkipDir
				}
				return nil
			}
			rel, err := filepath.Rel(masterDir, path)
			if err != nil {
				return err
			}
			checked++
			compareMasterToEmbedded(t, path, filepath.Join(embDir, rel))
			return nil
		})
		if err != nil {
			t.Fatalf("walking skills dir: %v", err)
		}
		if checked == 0 {
			t.Fatal("no files found in skills/")
		}
	})

	t.Run("support-docs", func(t *testing.T) {
		masterDir := filepath.Join(repoRoot, "support-docs")
		embDir := filepath.Join(embeddedDir, "support-docs")

		entries, err := os.ReadDir(masterDir)
		if err != nil {
			t.Fatalf("reading support-docs dir: %v", err)
		}

		var checked int
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			checked++
			compareMasterToEmbedded(t, filepath.Join(masterDir, e.Name()), filepath.Join(embDir, e.Name()))
		}
		if checked == 0 {
			t.Fatal("no .md files found in support-docs/")
		}
	})

	t.Run("docs support stubs resolve", func(t *testing.T) {
		stubs := map[string]string{
			"docs/CONFIGURATION.md":           "support-docs/CONFIGURATION.md",
			"docs/CUSTOMIZING_AGENT_TOOLS.md": "support-docs/CUSTOMIZING_AGENT_TOOLS.md",
			"docs/TROUBLESHOOTING.md":         "support-docs/TROUBLESHOOTING.md",
			"docs/USAGE_MULTI_AGENTS.md":      "support-docs/USAGE_MULTI_AGENTS.md",
			"docs/USAGE_PAIRING.md":           "support-docs/USAGE_PAIRING.md",
			"docs/how-to-produce-a-goal.md":   "support-docs/how-to-produce-a-goal.md",
		}

		for stub, target := range stubs {
			stubPath := filepath.Join(repoRoot, stub)
			targetPath := filepath.Join(repoRoot, target)
			content, err := os.ReadFile(stubPath)
			if err != nil {
				t.Fatalf("reading stub %s: %v", stub, err)
			}
			if _, err := os.Stat(targetPath); err != nil {
				t.Fatalf("stub target missing for %s -> %s: %v", stub, target, err)
			}
			if !strings.Contains(string(content), target) {
				t.Fatalf("stub %s does not point to %s", stub, target)
			}
		}
	})
}

// compareMasterToEmbedded reads both files and reports a test error if they differ.
func compareMasterToEmbedded(t *testing.T, masterPath, embeddedPath string) {
	t.Helper()

	master, err := os.ReadFile(masterPath)
	if err != nil {
		t.Errorf("reading master %s: %v", masterPath, err)
		return
	}

	embedded, err := os.ReadFile(embeddedPath)
	if err != nil {
		t.Errorf("reading embedded copy %s: %v", embeddedPath, err)
		return
	}

	if string(master) != string(embedded) {
		t.Errorf("DRIFT: master %s differs from embedded copy %s — run `make sync-embedded`",
			masterPath, embeddedPath)
	}
}

// findRepoRoot walks up from the working directory to find the directory
// containing go.mod (the repository root).
func findRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod found in any parent directory)")
		}
		dir = parent
	}
}
