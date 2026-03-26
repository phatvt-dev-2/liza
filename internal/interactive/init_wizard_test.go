package interactive

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectContractConflict_NoConflict(t *testing.T) {
	dir := t.TempDir()
	contractTarget := filepath.Join(dir, "CORE.md")

	// No files exist — no conflict
	got := DetectContractConflict(dir, []string{"claude", "codex"}, contractTarget)
	if got != "" {
		t.Errorf("expected no conflict, got %q", got)
	}
}

func TestDetectContractConflict_ExistingNonLizaFile(t *testing.T) {
	dir := t.TempDir()
	contractTarget := filepath.Join(dir, "CORE.md")

	// Create a regular file at CLAUDE.md (not a Liza symlink)
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	got := DetectContractConflict(dir, []string{"claude"}, contractTarget)
	if got != "CLAUDE.md" {
		t.Errorf("expected conflict on CLAUDE.md, got %q", got)
	}
}

func TestDetectContractConflict_ExistingLizaSymlink(t *testing.T) {
	dir := t.TempDir()
	contractTarget := filepath.Join(dir, "CORE.md")

	// Create CORE.md so the symlink target exists
	if err := os.WriteFile(contractTarget, []byte("core"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a Liza symlink — no conflict
	if err := os.Symlink(contractTarget, filepath.Join(dir, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}

	got := DetectContractConflict(dir, []string{"claude"}, contractTarget)
	if got != "" {
		t.Errorf("expected no conflict for Liza symlink, got %q", got)
	}
}

func TestDetectContractConflict_NonLizaSymlink(t *testing.T) {
	dir := t.TempDir()
	contractTarget := filepath.Join(dir, "CORE.md")
	otherTarget := filepath.Join(dir, "OTHER.md")

	// Create a symlink pointing somewhere else — conflict
	if err := os.WriteFile(otherTarget, []byte("other"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(otherTarget, filepath.Join(dir, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}

	got := DetectContractConflict(dir, []string{"claude"}, contractTarget)
	if got != "CLAUDE.md" {
		t.Errorf("expected conflict for non-Liza symlink, got %q", got)
	}
}

func TestDetectContractConflict_MistralSkipped(t *testing.T) {
	dir := t.TempDir()
	contractTarget := filepath.Join(dir, "CORE.md")

	// Mistral doesn't use repo-root symlinks — no conflict even with files present
	got := DetectContractConflict(dir, []string{"mistral"}, contractTarget)
	if got != "" {
		t.Errorf("expected no conflict for mistral, got %q", got)
	}
}

func TestDetectContractConflict_FirstConflictReturned(t *testing.T) {
	dir := t.TempDir()
	contractTarget := filepath.Join(dir, "CORE.md")

	// Create conflicts for multiple agents
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	// Should return the first conflict found (iteration order of agents slice)
	got := DetectContractConflict(dir, []string{"claude", "codex"}, contractTarget)
	if got != "CLAUDE.md" {
		t.Errorf("expected first conflict CLAUDE.md, got %q", got)
	}
}

func TestDetectContractConflict_EmptyProjectRoot(t *testing.T) {
	// Empty project root — no files to check
	got := DetectContractConflict("", []string{"claude"}, "/tmp/CORE.md")
	if got != "" {
		t.Errorf("expected no conflict for empty project root, got %q", got)
	}
}
