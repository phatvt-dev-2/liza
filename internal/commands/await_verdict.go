package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/ops"
)

// AwaitVerdictCommand blocks until a review verdict arrives for a submitted task.
// Delegates business logic to ops.AwaitVerdict.
func AwaitVerdictCommand(projectRoot, taskID, agentID string, timeout time.Duration) error {
	result, err := ops.AwaitVerdict(context.Background(), projectRoot, taskID, agentID, timeout)
	if err != nil {
		return fmt.Errorf("await verdict: %w", err)
	}

	fmt.Printf("Verdict: %s\nStatus: %s\n", result.Verdict, result.TaskStatus)
	if result.Reason != "" {
		fmt.Printf("Reason: %s\n", result.Reason)
	}
	if result.ReviewerAgent != "" {
		fmt.Printf("Reviewer: %s\n", result.ReviewerAgent)
	}
	if result.Guidance != "" {
		fmt.Printf("\n%s\n", result.Guidance)
	}
	return nil
}
