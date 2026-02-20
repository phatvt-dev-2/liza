package commands

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/ops"
)

// CheckpointCommand creates a sprint checkpoint and prints the result to stdout.
// Delegates business logic to ops.Checkpoint.
func CheckpointCommand(projectRoot string) error {
	result, err := ops.Checkpoint(projectRoot)
	if err != nil {
		return fmt.Errorf("checkpoint: %w", err)
	}

	fmt.Println("Sprint checkpoint created")
	fmt.Printf("  Status: IN_PROGRESS → CHECKPOINT\n")
	fmt.Printf("  Checkpoint at: %s\n", result.CheckpointAt.Format(time.RFC3339))
	fmt.Println()
	fmt.Printf("Sprint summary written to: %s\n", result.ReportPath)
	fmt.Println()
	fmt.Println("Agents will pause at their next check.")
	fmt.Println("Review the sprint summary, then use 'liza resume' to continue.")
	return nil
}
