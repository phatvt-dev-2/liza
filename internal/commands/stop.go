package commands

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// StopCommand stops the Liza system by setting config.mode to STOPPED
// Agents will detect this mode and exit cleanly
func StopCommand(projectRoot, reason, changedBy string) error {
	// Setup paths
	statePath := paths.New(projectRoot).StatePath()

	// Create blackboard
	blackboard := db.New(statePath)

	// Update mode atomically
	timestamp := time.Now()
	var previousMode models.SystemMode
	err := blackboard.Modify(func(s *models.State) error {
		previousMode = s.Config.Mode

		// Treat empty mode as RUNNING
		if previousMode == "" {
			previousMode = models.SystemModeRunning
		}

		// Check if already stopped
		if previousMode == models.SystemModeStopped {
			return fmt.Errorf("system is already STOPPED")
		}

		// Set mode to STOPPED
		s.Config.Mode = models.SystemModeStopped
		s.Config.ModeChangedAt = &timestamp
		s.Config.ModeChangedBy = &changedBy

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to stop system: %w", err)
	}

	// Output success message
	fmt.Println("System stopped")
	fmt.Printf("  Mode: %s → %s\n", previousMode, models.SystemModeStopped)
	fmt.Printf("  Changed by: %s\n", changedBy)
	if reason != "" {
		fmt.Printf("  Reason: %s\n", reason)
	}
	fmt.Println()
	fmt.Println("Agents will exit cleanly at their next check.")

	return nil
}
