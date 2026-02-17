package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupCommand_NewInstall(t *testing.T) {
	tmpDir := t.TempDir()

	err := SetupCommand(tmpDir, false)
	if err != nil {
		t.Fatalf("SetupCommand failed: %v", err)
	}

	// Verify contracts are flat in targetDir
	coreFile := filepath.Join(tmpDir, "CORE.md")
	if _, err := os.Stat(coreFile); os.IsNotExist(err) {
		t.Error("CORE.md not created in targetDir")
	}

	pairingFile := filepath.Join(tmpDir, "PAIRING_MODE.md")
	if _, err := os.Stat(pairingFile); os.IsNotExist(err) {
		t.Error("PAIRING_MODE.md not created in targetDir")
	}

	// Verify skills are in targetDir/skills/
	skillFile := filepath.Join(tmpDir, "skills", "code-review", "SKILL.md")
	if _, err := os.Stat(skillFile); os.IsNotExist(err) {
		t.Error("skills/code-review/SKILL.md not created")
	}

	// Verify nested skill files (clean-code/languages/)
	langFile := filepath.Join(tmpDir, "skills", "clean-code", "languages", "go.md")
	if _, err := os.Stat(langFile); os.IsNotExist(err) {
		t.Error("skills/clean-code/languages/go.md not created")
	}
}

func TestSetupCommand_ExistingWithoutForce(t *testing.T) {
	tmpDir := t.TempDir()

	// Pre-create CORE.md
	if err := os.WriteFile(filepath.Join(tmpDir, "CORE.md"), []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	err := SetupCommand(tmpDir, false)
	if err == nil {
		t.Fatal("Expected error when existing config found without --force")
	}

	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Expected 'already exists' in error, got: %v", err)
	}
}

func TestSetupCommand_ExistingWithForce(t *testing.T) {
	tmpDir := t.TempDir()

	// Pre-create CORE.md with old content
	if err := os.WriteFile(filepath.Join(tmpDir, "CORE.md"), []byte("old content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Provide "y\n" on stdin so the overwrite prompt is accepted
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString("y\n"); err != nil {
		t.Fatal(err)
	}
	w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	err = SetupCommand(tmpDir, true)
	if err != nil {
		t.Fatalf("SetupCommand with --force failed: %v", err)
	}

	// Verify CORE.md was overwritten
	content, err := os.ReadFile(filepath.Join(tmpDir, "CORE.md"))
	if err != nil {
		t.Fatal(err)
	}

	if string(content) == "old content" {
		t.Error("CORE.md was not overwritten with --force")
	}
}

func TestSetupCommand_ExistingWithForceDeclined(t *testing.T) {
	tmpDir := t.TempDir()

	// Pre-create CORE.md
	if err := os.WriteFile(filepath.Join(tmpDir, "CORE.md"), []byte("keep me"), 0644); err != nil {
		t.Fatal(err)
	}

	// Provide "n\n" on stdin — user declines overwrite
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString("n\n"); err != nil {
		t.Fatal(err)
	}
	w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	err = SetupCommand(tmpDir, true)
	if err == nil {
		t.Fatal("Expected error when user declines overwrite")
	}
	if !strings.Contains(err.Error(), "aborted") {
		t.Errorf("Expected 'aborted' in error, got: %v", err)
	}

	// Verify CORE.md was NOT overwritten
	content, err := os.ReadFile(filepath.Join(tmpDir, "CORE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "keep me" {
		t.Error("CORE.md was overwritten despite user declining")
	}
}
