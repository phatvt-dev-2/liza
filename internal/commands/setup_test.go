package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupCommand_NewInstall(t *testing.T) {
	tmpDir := t.TempDir()

	err := SetupCommand(SetupParams{TargetDir: tmpDir})
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

	err := SetupCommand(SetupParams{TargetDir: tmpDir})
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
	stdin := strings.NewReader("y\n")

	err := SetupCommand(SetupParams{TargetDir: tmpDir, Force: true, Stdin: stdin})
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
	stdin := strings.NewReader("y\nn\n")

	err := SetupCommand(SetupParams{TargetDir: tmpDir, Force: true, Stdin: stdin})
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
	stdin := strings.NewReader("y\ny\n")

	err := SetupCommand(SetupParams{TargetDir: tmpDir, Force: true, Stdin: stdin})
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
	stdin := strings.NewReader("n\n")

	err := SetupCommand(SetupParams{TargetDir: tmpDir, Force: true, Stdin: stdin})
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

func TestSetupCommand_CustomAgentTools(t *testing.T) {
	tmpDir := t.TempDir()

	// Create custom agent-tools file
	customContent := "# My Custom Agent Tools\n\nCustom tool configuration."
	customFile := filepath.Join(t.TempDir(), "my-agent-tools.md")
	if err := os.WriteFile(customFile, []byte(customContent), 0644); err != nil {
		t.Fatal(err)
	}

	err := SetupCommand(SetupParams{
		TargetDir:      tmpDir,
		AgentToolsPath: customFile,
	})
	if err != nil {
		t.Fatalf("SetupCommand failed: %v", err)
	}

	// AGENT_TOOLS.md should have custom content (with frontmatter prepended)
	toolsContent, err := os.ReadFile(filepath.Join(tmpDir, "AGENT_TOOLS.md"))
	if err != nil {
		t.Fatal(err)
	}
	toolsStr := string(toolsContent)

	if !strings.Contains(toolsStr, "My Custom Agent Tools") {
		t.Error("AGENT_TOOLS.md does not contain custom content")
	}
	if !strings.Contains(toolsStr, "Custom tool configuration.") {
		t.Error("AGENT_TOOLS.md does not contain custom body")
	}

	// Should have frontmatter
	if !strings.HasPrefix(toolsStr, "---\n") {
		t.Error("Custom AGENT_TOOLS.md missing frontmatter")
	}
	if !strings.Contains(toolsStr, "liza_version:") {
		t.Error("Custom AGENT_TOOLS.md missing version metadata")
	}

	// Embedded AGENT_TOOLS.md content should NOT be present
	// (the embedded version starts with "# Agent Tools" typically)
	// Just verify our custom content is the body, not the embedded default
	if !strings.Contains(toolsStr, customContent) {
		t.Error("Custom content not fully preserved")
	}
}

func TestSetupCommand_CustomAgentToolsOverwrite(t *testing.T) {
	tmpDir := t.TempDir()

	// Pre-create CORE.md and AGENT_TOOLS.md with existing content
	if err := os.WriteFile(filepath.Join(tmpDir, "CORE.md"), []byte("old core"), 0644); err != nil {
		t.Fatal(err)
	}
	oldToolsContent := "my previous custom tools"
	if err := os.WriteFile(filepath.Join(tmpDir, "AGENT_TOOLS.md"), []byte(oldToolsContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create custom agent-tools file
	customContent := "# New Custom Tools\n\nNew configuration."
	customFile := filepath.Join(t.TempDir(), "new-tools.md")
	if err := os.WriteFile(customFile, []byte(customContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Only "y\n" for bulk overwrite — no per-file prompt should fire.
	stdin := strings.NewReader("y\n")

	// Capture stderr to verify the per-file AGENT_TOOLS.md prompt is suppressed.
	origStderr := os.Stderr
	stderrR, stderrW, _ := os.Pipe()
	os.Stderr = stderrW

	err := SetupCommand(SetupParams{
		TargetDir:      tmpDir,
		Force:          true,
		AgentToolsPath: customFile,
		Stdin:          stdin,
	})

	stderrW.Close()
	os.Stderr = origStderr
	var stderrBuf bytes.Buffer
	io.Copy(&stderrBuf, stderrR)

	if err != nil {
		t.Fatalf("SetupCommand failed: %v", err)
	}

	// The per-file "Overwrite AGENT_TOOLS.md?" prompt must NOT appear.
	if strings.Contains(stderrBuf.String(), "AGENT_TOOLS.md") {
		t.Errorf("Per-file prompt for AGENT_TOOLS.md should be suppressed when --agent-tools is used, stderr: %s", stderrBuf.String())
	}

	// Custom content should be written
	toolsContent, err := os.ReadFile(filepath.Join(tmpDir, "AGENT_TOOLS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(toolsContent), "New Custom Tools") {
		t.Error("AGENT_TOOLS.md does not contain new custom content")
	}

	// Backup should exist with old content
	bakContent, err := os.ReadFile(filepath.Join(tmpDir, "AGENT_TOOLS.md.bak"))
	if err != nil {
		t.Fatal("AGENT_TOOLS.md.bak not created")
	}
	if string(bakContent) != oldToolsContent {
		t.Errorf("AGENT_TOOLS.md.bak has wrong content: %q", string(bakContent))
	}

	// CORE.md should also be overwritten (bulk "y" covers it)
	coreContent, err := os.ReadFile(filepath.Join(tmpDir, "CORE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(coreContent) == "old core" {
		t.Error("CORE.md was not overwritten")
	}
}

func TestSetupCommand_CustomAgentToolsFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	err := SetupCommand(SetupParams{
		TargetDir:      tmpDir,
		AgentToolsPath: "/nonexistent/path/agent-tools.md",
	})
	if err == nil {
		t.Fatal("Expected error for missing custom agent-tools file")
	}
	if !strings.Contains(err.Error(), "failed to read custom agent-tools file") {
		t.Errorf("Expected 'failed to read custom agent-tools file' in error, got: %v", err)
	}

	// No files should have been written
	if _, err := os.Stat(filepath.Join(tmpDir, "CORE.md")); err == nil {
		t.Error("CORE.md should not exist — early validation should prevent any writes")
	}
}
