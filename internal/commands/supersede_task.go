package commands

import (
	"fmt"
	"strings"

	"github.com/liza-mas/liza/internal/ops"
)

// SupersedeTaskCommand marks a task as SUPERSEDED and prints the result to stdout.
// Delegates business logic to ops.SupersedeTask.
func SupersedeTaskCommand(projectRoot, taskID string, replacementIDs []string, reason, agentID string) error {
	result, err := ops.SupersedeTask(projectRoot, taskID, replacementIDs, reason, agentID)
	if err != nil {
		return fmt.Errorf("supersede task: %w", err)
	}

	fmt.Printf("Superseded task %s (was %s) with replacements: %s\n",
		result.TaskID, result.OriginalStatus, strings.Join(result.ReplacementIDs, ", "))
	return nil
}
