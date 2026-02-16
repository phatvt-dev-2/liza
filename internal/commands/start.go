package commands

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// StartCommand starts the Liza system by setting config.mode to RUNNING
// Can only be used when the system is STOPPED
// Agents must be manually restarted after starting the system
func StartCommand(projectRoot, reason, changedBy string) error {
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

		// Check if already running
		if previousMode == models.SystemModeRunning {
			return fmt.Errorf("system is already RUNNING")
		}

		// Check if paused - should use resume instead
		if previousMode == models.SystemModePaused {
			return fmt.Errorf("system is PAUSED - use 'liza resume' instead")
		}

		// Check if in STOPPED mode
		if previousMode != models.SystemModeStopped {
			return fmt.Errorf("can only start from STOPPED mode (current: %s)", previousMode)
		}

		// Set mode to RUNNING
		s.Config.Mode = models.SystemModeRunning
		s.Config.ModeChangedAt = &timestamp
		s.Config.ModeChangedBy = &changedBy

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to start system: %w", err)
	}

	// Output success message with guidance
	fmt.Println("System started")
	fmt.Printf("  Mode: %s → %s\n", previousMode, models.SystemModeRunning)
	fmt.Printf("  Changed by: %s\n", changedBy)
	if reason != "" {
		fmt.Printf("  Reason: %s\n", reason)
	}
	fmt.Println()
	fmt.Println("The system mode is now RUNNING. Restart agents to resume work:")
	fmt.Println("  LIZA_AGENT_ID=coder-1 liza agent coder &")
	fmt.Println("  LIZA_AGENT_ID=reviewer-1 liza agent code-reviewer &")

	return nil
}
