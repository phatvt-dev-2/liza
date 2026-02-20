package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

// PauseCommand pauses the Liza system and prints the result to stdout.
// Delegates business logic to ops.Pause.
func PauseCommand(projectRoot, reason, changedBy string) error {
	result, err := ops.Pause(projectRoot, reason, changedBy)
	if err != nil {
		return fmt.Errorf("pause: %w", err)
	}

	printModeChangeResult("System paused", result,
		"Agents will pause at their next check.",
		"Use 'liza resume' to continue.",
	)
	return nil
}
