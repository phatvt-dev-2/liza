package commands

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// ResumeCommand resumes the Liza system by setting config.mode to RUNNING
// and sprint.status to IN_PROGRESS if at CHECKPOINT
// Can only be used when the system is PAUSED or sprint is at CHECKPOINT
func ResumeCommand(projectRoot, changedBy string) error {
	// Setup paths
	statePath := paths.New(projectRoot).StatePath()

	// Create blackboard
	blackboard := db.New(statePath)

	// Update mode atomically
	timestamp := time.Now()
	var resumedFrom string
	err := blackboard.Modify(func(s *models.State) error {
		currentMode := s.Config.Mode

		// Treat empty mode as RUNNING
		if currentMode == "" {
			currentMode = models.SystemModeRunning
		}

		// Check if resuming from PAUSED mode, CIRCUIT_BREAKER_TRIPPED mode, or CHECKPOINT status
		isPaused := currentMode == models.SystemModePaused
		isCircuitBreakerTripped := currentMode == models.SystemModeCircuitBreakerTripped
		isCheckpoint := s.Sprint.Status == models.SprintStatusCheckpoint

		if !isPaused && !isCircuitBreakerTripped && !isCheckpoint {
			if currentMode == models.SystemModeStopped {
				return fmt.Errorf("cannot resume from STOPPED state (system must be restarted)")
			}
			return fmt.Errorf("system is not PAUSED, circuit breaker not tripped, and sprint is not at CHECKPOINT (current mode: %s, sprint status: %s)", currentMode, s.Sprint.Status)
		}

		// Set mode to RUNNING if paused or circuit breaker tripped
		if isPaused {
			s.Config.Mode = models.SystemModeRunning
			s.Config.ModeChangedAt = &timestamp
			s.Config.ModeChangedBy = &changedBy
			resumedFrom = "PAUSED mode"
		} else if isCircuitBreakerTripped {
			// Clear circuit breaker state and set mode to RUNNING
			s.Config.Mode = models.SystemModeRunning
			s.Config.ModeChangedAt = &timestamp
			s.Config.ModeChangedBy = &changedBy
			s.CircuitBreaker.Status = "OK"
			s.CircuitBreaker.CurrentTrigger = nil
			resumedFrom = "CIRCUIT_BREAKER_TRIPPED mode"
		}

		// Set sprint status to IN_PROGRESS if at checkpoint
		if isCheckpoint {
			s.Sprint.Status = models.SprintStatusInProgress
			if resumedFrom != "" {
				resumedFrom += " and CHECKPOINT"
			} else {
				resumedFrom = "CHECKPOINT"
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to resume system: %w", err)
	}

	// Output success message
	fmt.Println("System resumed")
	fmt.Printf("  Resumed from: %s\n", resumedFrom)
	fmt.Printf("  Changed by: %s\n", changedBy)
	fmt.Println()
	fmt.Println("Agents will resume at their next check.")

	return nil
}
