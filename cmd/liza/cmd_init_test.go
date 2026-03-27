package main

import (
	"os"
	"strings"
	"testing"
)

func TestInitDispatch_WorkspaceFlagsRequireDescription(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "branch without description errors",
			args:    []string{"init", "--branch", "custom"},
			wantErr: "requires a description argument",
		},
		{
			name:    "config without description errors",
			args:    []string{"init", "--config", "custom.yaml"},
			wantErr: "requires a description argument",
		},
		{
			name:    "spec without description errors",
			args:    []string{"init", "--spec", "custom-spec.md"},
			wantErr: "requires a description argument",
		},
		{
			name:    "post-worktree-cmd without description errors",
			args:    []string{"init", "--post-worktree-cmd", "make setup"},
			wantErr: "requires a description argument",
		},
		{
			name:    "entry-point without description errors",
			args:    []string{"init", "--entry-point", "detailed-spec"},
			wantErr: "requires a description argument",
		},
		{
			name:    "auto-resume without description gets specific error",
			args:    []string{"init", "--claude", "--auto-resume"},
			wantErr: "--auto-resume requires full workspace init",
		},
		{
			name:    "agent flag with workspace flag and no description errors",
			args:    []string{"init", "--claude", "--branch", "foo"},
			wantErr: "workspace flags",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetRootCmdForTest(t)
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestInitDispatch_AgentFlagAlonePassesDispatch(t *testing.T) {
	// Run in a temp dir with fake HOME to prevent side effects on the
	// developer's workspace. The command will fail downstream (no git repo,
	// no ~/.liza), but it must NOT fail at the dispatch level.
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(oldDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	resetRootCmdForTest(t)
	rootCmd.SetArgs([]string{"init", "--claude"})
	err = rootCmd.Execute()

	// It will error (no git repo / no global config), but not at dispatch.
	dispatchErrors := []string{"requires a description", "workspace flags", "--auto-resume requires"}
	if err != nil {
		for _, de := range dispatchErrors {
			if strings.Contains(err.Error(), de) {
				t.Fatalf("hit dispatch-level error: %v", err)
			}
		}
	}
}
