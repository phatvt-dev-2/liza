package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectQuotaExhaustion_CodexMatch(t *testing.T) {
	output := `{"type":"turn.started"}
{"type":"error","message":"You've hit your usage limit. Upgrade to Pro."}
{"type":"turn.failed"}`

	result := DetectQuotaExhaustion(output, "codex")
	if result == nil {
		t.Fatal("expected quota exhaustion detected, got nil")
	}
	if result.Provider != "codex" {
		t.Errorf("Provider = %q, want %q", result.Provider, "codex")
	}
	if result.Message == "" {
		t.Error("Message should not be empty")
	}
}

func TestDetectQuotaExhaustion_WrongProvider(t *testing.T) {
	output := `{"type":"error","message":"You've hit your usage limit."}`

	result := DetectQuotaExhaustion(output, "claude")
	if result != nil {
		t.Errorf("expected nil for non-matching provider, got %+v", result)
	}
}

func TestDetectQuotaExhaustion_NoMatch(t *testing.T) {
	output := `{"type":"turn.completed","usage":{"input_tokens":100}}`

	result := DetectQuotaExhaustion(output, "codex")
	if result != nil {
		t.Errorf("expected nil for non-matching output, got %+v", result)
	}
}

func TestDetectQuotaExhaustion_EmptyOutput(t *testing.T) {
	result := DetectQuotaExhaustion("", "codex")
	if result != nil {
		t.Errorf("expected nil for empty output, got %+v", result)
	}
}

func TestQuotaSignal_WriteCheckClear(t *testing.T) {
	projectRoot := t.TempDir()
	lizaDir := filepath.Join(projectRoot, ".liza")
	if err := os.MkdirAll(lizaDir, 0755); err != nil {
		t.Fatal(err)
	}

	if CheckQuotaSignal(projectRoot, "codex") {
		t.Fatal("signal should not exist before write")
	}

	if err := WriteQuotaSignal(projectRoot, "codex", "You've hit your usage limit"); err != nil {
		t.Fatalf("WriteQuotaSignal failed: %v", err)
	}

	if !CheckQuotaSignal(projectRoot, "codex") {
		t.Fatal("signal should exist after write")
	}

	// Other providers unaffected
	if CheckQuotaSignal(projectRoot, "claude") {
		t.Fatal("claude signal should not exist")
	}

	if err := ClearQuotaSignal(projectRoot, "codex"); err != nil {
		t.Fatalf("ClearQuotaSignal failed: %v", err)
	}

	if CheckQuotaSignal(projectRoot, "codex") {
		t.Fatal("signal should not exist after clear")
	}
}

func TestClearQuotaSignal_Idempotent(t *testing.T) {
	projectRoot := t.TempDir()

	// Clear on non-existent file should not error.
	if err := ClearQuotaSignal(projectRoot, "codex"); err != nil {
		t.Fatalf("ClearQuotaSignal on missing file: %v", err)
	}
}

func TestLatestOutputContent(t *testing.T) {
	dir := t.TempDir()

	// Write two files — should return the latest (lexicographically last).
	if err := os.WriteFile(filepath.Join(dir, "agent-1-20260328-100000.txt"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "agent-1-20260328-110000.txt"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	content := latestOutputContent(dir, "agent-1")
	if content != "new" {
		t.Errorf("latestOutputContent = %q, want %q", content, "new")
	}
}

func TestLatestOutputContent_NoFiles(t *testing.T) {
	dir := t.TempDir()

	content := latestOutputContent(dir, "agent-1")
	if content != "" {
		t.Errorf("latestOutputContent = %q, want empty", content)
	}
}
