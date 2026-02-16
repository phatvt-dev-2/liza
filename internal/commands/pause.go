package commands

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// PauseCommand pauses the Liza system by setting config.mode to PAUSED
// Agents will detect this mode and block until the system is resumed
func PauseCommand(projectRoot, reason, changedBy string) error {
	// Setup paths
	statePath := paths.New(projectRoot).StatePath()

	// Create blackboard
	blackboard := db.New(statePath)

	// Update mode atomically
	timestamp := time.Now()
	err := blackboard.Modify(func(s *models.State) error {
		currentMode := s.Config.Mode

		// Treat empty mode as RUNNING
		if currentMode == "" {
			currentMode = models.SystemModeRunning
		}

		// Check if already paused
		if currentMode == models.SystemModePaused {
			return fmt.Errorf("system is already PAUSED")
		}

		// Cannot pause if stopped
		if currentMode == models.SystemModeStopped {
			return fmt.Errorf("cannot pause: system is STOPPED (use resume only from PAUSED state)")
		}

		// Set mode to PAUSED
		s.Config.Mode = models.SystemModePaused
		s.Config.ModeChangedAt = &timestamp
		s.Config.ModeChangedBy = &changedBy

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to pause system: %w", err)
	}

	// Output success message
	fmt.Println("System paused")
	fmt.Printf("  Mode: %s → %s\n", models.SystemModeRunning, models.SystemModePaused)
	fmt.Printf("  Changed by: %s\n", changedBy)
	if reason != "" {
		fmt.Printf("  Reason: %s\n", reason)
	}
	fmt.Println()
	fmt.Println("Agents will pause at their next check.")
	fmt.Println("Use 'liza resume' to continue.")

	return nil
}
