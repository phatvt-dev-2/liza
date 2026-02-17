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

	// Verify .bak backup was created
	bakContent, err := os.ReadFile(filepath.Join(tmpDir, "CORE.md.bak"))
	if err != nil {
		t.Fatal("CORE.md.bak not created")
	}
	if string(bakContent) != "old content" {
		t.Errorf("CORE.md.bak has wrong content: %q", string(bakContent))
	}
}

func TestSetupCommand_CustomizableFileSkipped(t *testing.T) {
	tmpDir := t.TempDir()

	// Pre-create CORE.md and AGENT_TOOLS.md
	if err := os.WriteFile(filepath.Join(tmpDir, "CORE.md"), []byte("old core"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "AGENT_TOOLS.md"), []byte("my custom tools"), 0644); err != nil {
		t.Fatal(err)
	}

	// Provide "y\n" for bulk overwrite, then "n\n" to skip AGENT_TOOLS.md
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString("y\nn\n"); err != nil {
		t.Fatal(err)
	}
	w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	err = SetupCommand(tmpDir, true)
	if err != nil {
		t.Fatalf("SetupCommand failed: %v", err)
	}

	// CORE.md should be overwritten
	coreContent, err := os.ReadFile(filepath.Join(tmpDir, "CORE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(coreContent) == "old core" {
		t.Error("CORE.md was not overwritten")
	}

	// AGENT_TOOLS.md should be preserved (user declined)
	toolsContent, err := os.ReadFile(filepath.Join(tmpDir, "AGENT_TOOLS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(toolsContent) != "my custom tools" {
		t.Error("AGENT_TOOLS.md was overwritten despite user declining")
	}

	// AGENT_TOOLS.md.bak should NOT exist (file was skipped, not overwritten)
	if _, err := os.Stat(filepath.Join(tmpDir, "AGENT_TOOLS.md.bak")); err == nil {
		t.Error("AGENT_TOOLS.md.bak should not exist when file was skipped")
	}

	// CORE.md.bak should exist
	bakContent, err := os.ReadFile(filepath.Join(tmpDir, "CORE.md.bak"))
	if err != nil {
		t.Fatal("CORE.md.bak not created")
	}
	if string(bakContent) != "old core" {
		t.Errorf("CORE.md.bak has wrong content: %q", string(bakContent))
	}
}

func TestSetupCommand_CustomizableFileOverwritten(t *testing.T) {
	tmpDir := t.TempDir()

	// Pre-create CORE.md and AGENT_TOOLS.md
	if err := os.WriteFile(filepath.Join(tmpDir, "CORE.md"), []byte("old core"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "AGENT_TOOLS.md"), []byte("my custom tools"), 0644); err != nil {
		t.Fatal(err)
	}

	// Provide "y\n" for bulk overwrite, then "y\n" to also overwrite AGENT_TOOLS.md
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString("y\ny\n"); err != nil {
		t.Fatal(err)
	}
	w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	err = SetupCommand(tmpDir, true)
	if err != nil {
		t.Fatalf("SetupCommand failed: %v", err)
	}

	// AGENT_TOOLS.md should be overwritten (user accepted)
	toolsContent, err := os.ReadFile(filepath.Join(tmpDir, "AGENT_TOOLS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(toolsContent) == "my custom tools" {
		t.Error("AGENT_TOOLS.md was not overwritten despite user accepting")
	}

	// AGENT_TOOLS.md.bak should exist with original content
	bakContent, err := os.ReadFile(filepath.Join(tmpDir, "AGENT_TOOLS.md.bak"))
	if err != nil {
		t.Fatal("AGENT_TOOLS.md.bak not created")
	}
	if string(bakContent) != "my custom tools" {
		t.Errorf("AGENT_TOOLS.md.bak has wrong content: %q", string(bakContent))
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
