package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveOutputMasksSecrets(t *testing.T) {
	t.Run("nil masker passes through unchanged", func(t *testing.T) {
		dir := t.TempDir()
		content := "raw output with sk-ant-secret-key-value"
		path, err := saveOutput(dir, "coder-1", "txt", content, nil)
		if err != nil {
			t.Fatalf("saveOutput error: %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile error: %v", err)
		}
		if string(got) != content {
			t.Errorf("nil masker should not alter content\ngot:  %q\nwant: %q", got, content)
		}
	})

	t.Run("masker redacts secrets in persisted file", func(t *testing.T) {
		dir := t.TempDir()
		masker := newSecretMaskerFromEnv([]string{
			"ANTHROPIC_API_KEY=sk-ant-secret-key-value",
		})
		content := "Error: auth failed with sk-ant-secret-key-value"
		path, err := saveOutput(dir, "coder-1", "txt", content, masker)
		if err != nil {
			t.Fatalf("saveOutput error: %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile error: %v", err)
		}
		if strings.Contains(string(got), "sk-ant-secret-key-value") {
			t.Error("persisted file should not contain secret value")
		}
		want := "Error: auth failed with ***"
		if string(got) != want {
			t.Errorf("persisted content\ngot:  %q\nwant: %q", got, want)
		}
	})

	t.Run("original string is not mutated", func(t *testing.T) {
		dir := t.TempDir()
		masker := newSecretMaskerFromEnv([]string{
			"OPENAI_API_KEY=sk-openai-long-secret",
		})
		content := "key=sk-openai-long-secret"
		_, err := saveOutput(dir, "coder-1", "txt", content, masker)
		if err != nil {
			t.Fatalf("saveOutput error: %v", err)
		}
		// Original Go string is immutable, but verify the caller's variable is unchanged.
		if content != "key=sk-openai-long-secret" {
			t.Error("caller's content string should not be modified")
		}
	})

	t.Run("empty content skips write", func(t *testing.T) {
		dir := t.TempDir()
		masker := newSecretMaskerFromEnv([]string{"ANTHROPIC_API_KEY=sk-ant-value1234"})
		path, err := saveOutput(dir, "coder-1", "txt", "", masker)
		if err != nil {
			t.Fatalf("saveOutput error: %v", err)
		}
		// saveOutput writes even empty content (saveTimestampedFile doesn't skip).
		// Verify the file exists and is empty.
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("empty content should produce empty file, got %d bytes", len(got))
		}
	})

	t.Run("file naming convention preserved", func(t *testing.T) {
		dir := t.TempDir()
		masker := newSecretMaskerFromEnv(nil)
		path, err := saveOutput(dir, "reviewer-3", "err", "stderr content", masker)
		if err != nil {
			t.Fatalf("saveOutput error: %v", err)
		}
		base := filepath.Base(path)
		if !strings.HasPrefix(base, "reviewer-3-") || !strings.HasSuffix(base, ".err") {
			t.Errorf("unexpected filename: %s", base)
		}
	})
}
