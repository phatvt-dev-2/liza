package embedded

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnforceInitHook_AllowsCodexBashDocReads(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	hookPath := writeEnforceInitHook(t)
	projectRoot := t.TempDir()
	sessionID := "test-codex-bash-doc-reads-" + strings.ReplaceAll(t.Name(), "/", "-") + "-" + time.Now().Format("150405.000000000")
	stateDir := filepath.Join(os.TempDir(), "liza-init-gate-"+sessionID)
	defer os.RemoveAll(stateDir)

	runHook(t, hookPath, `{"session_id":"`+sessionID+`","cwd":"`+projectRoot+`","tool_name":"Bash","tool_input":{"command":"sed -n '1,120p' ~/.liza/AGENT_TOOLS.md"}}`, 0)
	runHook(t, hookPath, `{"session_id":"`+sessionID+`","cwd":"`+projectRoot+`","tool_name":"Bash","tool_input":{"command":"cat ~/.liza/PAIRING_MODE.md"}}`, 0)

	if _, err := os.Stat(filepath.Join(stateDir, "CLEARED")); err != nil {
		t.Fatalf("expected init gate to clear after Codex Bash doc reads: %v", err)
	}

	runHook(t, hookPath, `{"session_id":"`+sessionID+`","cwd":"`+projectRoot+`","tool_name":"Bash","tool_input":{"command":"git status --short"}}`, 0)
}

func TestEnforceInitHook_BlocksComplexCodexBashDocReads(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	hookPath := writeEnforceInitHook(t)
	projectRoot := t.TempDir()
	sessionID := "test-codex-bash-complex-read-" + strings.ReplaceAll(t.Name(), "/", "-") + "-" + time.Now().Format("150405.000000000")
	stateDir := filepath.Join(os.TempDir(), "liza-init-gate-"+sessionID)
	defer os.RemoveAll(stateDir)

	output := runHook(t, hookPath, `{"session_id":"`+sessionID+`","cwd":"`+projectRoot+`","tool_name":"Bash","tool_input":{"command":"sed -n '1p' ~/.liza/AGENT_TOOLS.md; rm -rf /tmp/not-real"}}`, 2)
	if !strings.Contains(output, "simple read-only doc commands") {
		t.Fatalf("expected complex Bash doc read to explain the restriction, got:\n%s", output)
	}
}

func TestEnforceInitHook_BlocksUnsafeCodexBashDocReads(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	hookPath := writeEnforceInitHook(t)
	projectRoot := t.TempDir()

	cases := []struct {
		name    string
		command string
	}{
		{
			name:    "cat extra operand",
			command: "cat ~/.liza/AGENT_TOOLS.md /tmp/not-a-doc",
		},
		{
			name:    "sed in-place",
			command: "sed -i s/x/y/ ~/.liza/AGENT_TOOLS.md",
		},
		{
			name:    "cat redirected",
			command: "cat ~/.liza/AGENT_TOOLS.md > /tmp/copy",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sessionID := "test-codex-bash-unsafe-read-" + strings.ReplaceAll(t.Name(), "/", "-") + "-" + time.Now().Format("150405.000000000")
			stateDir := filepath.Join(os.TempDir(), "liza-init-gate-"+sessionID)
			defer os.RemoveAll(stateDir)

			payload := `{"session_id":"` + sessionID + `","cwd":"` + projectRoot + `","tool_name":"Bash","tool_input":{"command":"` + tc.command + `"}}`
			output := runHook(t, hookPath, payload, 2)
			if !strings.Contains(output, "simple read-only doc commands") {
				t.Fatalf("expected unsafe Bash doc read to explain the restriction, got:\n%s", output)
			}
			if _, err := os.Stat(filepath.Join(stateDir, "CLEARED")); !os.IsNotExist(err) {
				t.Fatalf("unsafe command should not clear gate, stat err: %v", err)
			}
		})
	}
}

func writeEnforceInitHook(t *testing.T) string {
	t.Helper()
	hookPath := filepath.Join(t.TempDir(), "enforce-init.sh")
	if err := os.WriteFile(hookPath, enforceInitHookContent, 0755); err != nil {
		t.Fatalf("write hook: %v", err)
	}
	return hookPath
}

func runHook(t *testing.T, hookPath, payload string, wantCode int) string {
	t.Helper()
	cmd := exec.Command("bash", hookPath)
	cmd.Stdin = strings.NewReader(payload)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	if wantCode == 0 {
		if err != nil {
			t.Fatalf("hook exited non-zero: %v\n%s", err, output.String())
		}
		return output.String()
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("hook exit = %v, want code %d\n%s", err, wantCode, output.String())
	}
	if exitErr.ExitCode() != wantCode {
		t.Fatalf("hook exit code = %d, want %d\n%s", exitErr.ExitCode(), wantCode, output.String())
	}
	return output.String()
}
