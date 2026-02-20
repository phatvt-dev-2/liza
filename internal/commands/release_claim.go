package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

// ReleaseClaimCommand releases claims on a task and prints the result to stdout.
// Delegates business logic to ops.ReleaseClaim.
func ReleaseClaimCommand(projectRoot, taskID, role string, force bool, reason, agentID string) error {
	result, err := ops.ReleaseClaim(projectRoot, taskID, role, force, reason, agentID)
	if err != nil {
		return fmt.Errorf("release claim: %w", err)
	}

	if result.ReleasedReviewer {
		fmt.Printf("Released review claim for %s\n", result.TaskID)
	}
	if result.ReleasedCoder {
		fmt.Printf("Released coder claim for %s\n", result.TaskID)
	}
	return nil
}
