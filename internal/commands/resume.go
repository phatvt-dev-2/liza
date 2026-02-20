package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

// ResumeCommand resumes the Liza system and prints the result to stdout.
// Delegates business logic to ops.Resume.
func ResumeCommand(projectRoot, changedBy string) error {
	result, err := ops.Resume(projectRoot, changedBy)
	if err != nil {
		return fmt.Errorf("resume: %w", err)
	}

	fmt.Println("System resumed")
	fmt.Printf("  Resumed from: %s\n", result.ResumedFrom)
	fmt.Printf("  Changed by: %s\n", result.ChangedBy)
	fmt.Println()
	fmt.Println("Agents will resume at their next check.")
	return nil
}
