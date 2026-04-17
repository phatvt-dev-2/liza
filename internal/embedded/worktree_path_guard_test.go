package embedded

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestWorktreePathGuardHook shells out to the SHIPPED script (written from
// the embedded content) with crafted JSON payloads and asserts the hook
// denies only on the .worktrees/<id>/<id>/ duplication pattern. Running
// against the rendered template (not a handwritten copy) means regressions
// in the script — dropped case, broken regex, missing field extraction —
// fail these tests.
func TestWorktreePathGuardHook(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	// Write the shipped hook to a temp file so we test what actually ships.
	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, "worktree-path-guard.sh")
	if err := os.WriteFile(hookPath, worktreePathGuardHookContent, 0755); err != nil {
		t.Fatalf("write hook: %v", err)
	}

	cases := []struct {
		name       string
		payload    string
		wantDenied bool
	}{
		{
			name:       "duplicated task id denies",
			payload:    `{"tool_name":"Read","tool_input":{"file_path":"/p/.worktrees/task-1/task-1/foo.go"}}`,
			wantDenied: true,
		},
		{
			name:       "duplicated id deep in path denies",
			payload:    `{"tool_name":"Edit","tool_input":{"file_path":"/p/.worktrees/my-complex-task-id-42/my-complex-task-id-42/nested/file.py"}}`,
			wantDenied: true,
		},
		{
			name:       "normal worktree path passes",
			payload:    `{"tool_name":"Read","tool_input":{"file_path":"/p/.worktrees/task-1/foo.go"}}`,
			wantDenied: false,
		},
		{
			name:       "task id appears twice but not consecutive passes",
			payload:    `{"tool_name":"Read","tool_input":{"file_path":"/p/.worktrees/task-1/sub/task-1-notes.md"}}`,
			wantDenied: false,
		},
		{
			name:       "task id as substring of next segment passes",
			payload:    `{"tool_name":"Read","tool_input":{"file_path":"/p/.worktrees/task-1/task-12/foo.go"}}`,
			wantDenied: false,
		},
		{
			name:       "missing file_path passes silently",
			payload:    `{"tool_name":"Read","tool_input":{}}`,
			wantDenied: false,
		},
		{
			name:       "path is exactly the duplicated prefix (no trailing slash) denies",
			payload:    `{"tool_name":"Write","tool_input":{"file_path":"/p/.worktrees/task-1/task-1"}}`,
			wantDenied: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("bash", hookPath)
			cmd.Stdin = strings.NewReader(tc.payload)
			var stdout bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stdout
			if err := cmd.Run(); err != nil {
				t.Fatalf("hook exited non-zero (hook must exit 0 regardless of decision): %v\n%s", err, stdout.String())
			}

			got := stdout.String()
			gotDenied := strings.Contains(got, `"permissionDecision":"deny"`)

			if gotDenied != tc.wantDenied {
				t.Errorf("denied=%v, want %v. Output:\n%s", gotDenied, tc.wantDenied, got)
			}

			if tc.wantDenied && !strings.Contains(got, ".worktrees/") {
				t.Errorf("deny message should include the duplicated segment for diagnostics. Output:\n%s", got)
			}
		})
	}
}
