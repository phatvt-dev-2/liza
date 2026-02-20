package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

// HandoffCommand initiates a context-exhaustion handoff and prints the result to stdout.
// Delegates business logic to ops.Handoff.
func HandoffCommand(projectRoot, taskID, summary, nextAction, agentID string) error {
	result, err := ops.Handoff(projectRoot, taskID, summary, nextAction, agentID)
	if err != nil {
		return fmt.Errorf("handoff: %w", err)
	}

	fmt.Printf("HANDOFF: %s\n", result.TaskID)
	fmt.Printf("  by: %s\n", result.AgentID)
	return nil
}
