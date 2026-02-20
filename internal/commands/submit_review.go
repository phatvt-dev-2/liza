package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

// SubmitForReviewCommand submits a task for review and prints the result to stdout.
// Delegates business logic to ops.SubmitForReview.
func SubmitForReviewCommand(projectRoot, taskID, commitSHA, agentID string) error {
	result, err := ops.SubmitForReview(projectRoot, taskID, commitSHA, agentID)
	if err != nil {
		return fmt.Errorf("submit for review: %w", err)
	}

	fmt.Printf("SUBMITTED FOR REVIEW: %s\n", result.TaskID)
	fmt.Printf("  review_commit: %s\n", result.ReviewCommit)
	fmt.Printf("  submitted_by: %s\n", result.AgentID)
	return nil
}
