package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

// AssessHypothesisExhaustedCommand records an orchestrator assessment of a hypothesis-exhausted task.
// Delegates business logic to ops.AssessHypothesisExhausted.
func AssessHypothesisExhaustedCommand(projectRoot, taskID, note, agentID string) error {
	result, err := ops.AssessHypothesisExhausted(projectRoot, taskID, note, agentID)
	if err != nil {
		return fmt.Errorf("assess hypothesis-exhausted: %w", err)
	}

	fmt.Printf("Task %s assessed by orchestrator (hypothesis-exhausted)\n", result.TaskID)
	return nil
}
