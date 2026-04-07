package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

// SetTaskOutputCommand sets output entries on a task.
// Delegates business logic to ops.SetTaskOutput.
func SetTaskOutputCommand(projectRoot string, input *ops.SetTaskOutputInput) error {
	if err := ops.SetTaskOutput(projectRoot, input); err != nil {
		return fmt.Errorf("set task output: %w", err)
	}

	fmt.Printf("Output set on task %s (%d entries)\n", input.TaskID, len(input.Output))
	return nil
}
