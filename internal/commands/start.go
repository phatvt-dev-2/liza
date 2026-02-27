package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

// StartCommand starts the Liza system and prints the result to stdout.
// Delegates business logic to ops.Start.
func StartCommand(projectRoot, reason, changedBy string) error {
	result, err := ops.Start(projectRoot, reason, changedBy)
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}

	printModeChangeResult("System started", result,
		"The system mode is now RUNNING. Restart agents to resume work:",
		"  LIZA_AGENT_ID=coder-1 liza agent coder &",
		"  LIZA_AGENT_ID=code-reviewer-1 liza agent code-reviewer &",
	)
	return nil
}

func printModeChangeResult(header string, r *ops.ModeChangeResult, footer ...string) {
	fmt.Println(header)
	fmt.Printf("  Mode: %s → %s\n", r.Previous, r.New)
	fmt.Printf("  Changed by: %s\n", r.ChangedBy)
	if r.Reason != "" {
		fmt.Printf("  Reason: %s\n", r.Reason)
	}
	fmt.Println()
	for _, line := range footer {
		fmt.Println(line)
	}
}
