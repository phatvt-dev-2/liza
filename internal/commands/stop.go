package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

// StopCommand stops the Liza system and prints the result to stdout.
// Delegates business logic to ops.Stop.
func StopCommand(projectRoot, reason, changedBy string) error {
	result, err := ops.Stop(projectRoot, reason, changedBy)
	if err != nil {
		return fmt.Errorf("stop: %w", err)
	}

	printModeChangeResult("System stopped", result,
		"Agents will exit cleanly at their next check.",
	)
	return nil
}
