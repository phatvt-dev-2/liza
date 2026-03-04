package main

import (
	"strings"
	"testing"
)

func TestAgentHelpListsAllRuntimeRoles(t *testing.T) {
	helpText := agentCmd.Long

	required := []string{
		"coder",
		"code-reviewer",
		"orchestrator",
		"code-planner",
		"code-plan-reviewer",
	}

	for _, role := range required {
		if !strings.Contains(helpText, role) {
			t.Fatalf("agent help missing role %q", role)
		}
	}
}
