package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

// UpdateReviewCommitCommand updates review_commit and prints the result to stdout.
// Delegates business logic to ops.UpdateReviewCommit.
func UpdateReviewCommitCommand(projectRoot, taskID, changedBy string) error {
	result, err := ops.UpdateReviewCommit(projectRoot, taskID, changedBy)
	if err != nil {
		return fmt.Errorf("update review commit: %w", err)
	}

	fmt.Printf("Updated review_commit for %s\n", result.TaskID)
	fmt.Printf("  old: %s\n", result.OldReviewCommit)
	fmt.Printf("  new: %s\n", result.NewReviewCommit)
	if result.ReviewerReleased {
		fmt.Println("  reviewer claim released (task returned to submitted state)")
	}
	return nil
}
