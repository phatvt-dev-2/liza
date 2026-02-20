package ops

import (
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// ModeChangeResult contains the outcome of a system mode change.
type ModeChangeResult struct {
	Previous  models.SystemMode
	New       models.SystemMode
	ChangedBy string
	Reason    string
}

// Start transitions system mode from STOPPED to RUNNING. No terminal I/O.
func Start(projectRoot, reason, changedBy string) (*ModeChangeResult, error) {
	return changeMode(projectRoot, reason, changedBy, models.SystemModeRunning)
}

// Stop transitions system mode to STOPPED. Agents detect this and exit
// cleanly. No terminal I/O.
func Stop(projectRoot, reason, changedBy string) (*ModeChangeResult, error) {
	return changeMode(projectRoot, reason, changedBy, models.SystemModeStopped)
}

// Pause transitions system mode to PAUSED. Agents block until resumed.
// No terminal I/O.
func Pause(projectRoot, reason, changedBy string) (*ModeChangeResult, error) {
	return changeMode(projectRoot, reason, changedBy, models.SystemModePaused)
}

// changeMode is the shared implementation for Start, Stop, and Pause.
// It validates the transition via the systemModeTransitions table and applies it.
func changeMode(projectRoot, reason, changedBy string, target models.SystemMode) (*ModeChangeResult, error) {
	statePath := paths.New(projectRoot).StatePath()
	blackboard := db.New(statePath)

	timestamp := time.Now()
	var previousMode models.SystemMode

	err := blackboard.Modify(func(s *models.State) error {
		previousMode = s.Config.Mode
		if previousMode == "" {
			previousMode = models.SystemModeRunning
		}

		if err := previousMode.ValidateTransition(target); err != nil {
			return err
		}

		s.Config.Mode = target
		s.Config.ModeChangedAt = &timestamp
		s.Config.ModeChangedBy = &changedBy

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to change mode to %s: %w", target, err)
	}

	return &ModeChangeResult{
		Previous:  previousMode,
		New:       target,
		ChangedBy: changedBy,
		Reason:    reason,
	}, nil
}

// ResumeResult contains the outcome of a system resume.
type ResumeResult struct {
	ResumedFrom string
	ChangedBy   string
}

// Resume transitions from PAUSED or CIRCUIT_BREAKER_TRIPPED to RUNNING,
// and/or resumes sprint from CHECKPOINT to IN_PROGRESS. No terminal I/O.
func Resume(projectRoot, changedBy string) (*ResumeResult, error) {
	statePath := paths.New(projectRoot).StatePath()
	blackboard := db.New(statePath)

	timestamp := time.Now()
	var resumedFrom string

	err := blackboard.Modify(func(s *models.State) error {
		currentMode := s.Config.Mode

		if currentMode == "" {
			currentMode = models.SystemModeRunning
		}

		isPaused := currentMode == models.SystemModePaused
		isCircuitBreakerTripped := currentMode == models.SystemModeCircuitBreakerTripped
		isCheckpoint := s.Sprint.Status == models.SprintStatusCheckpoint

		if !isPaused && !isCircuitBreakerTripped && !isCheckpoint {
			if currentMode == models.SystemModeStopped {
				return fmt.Errorf("cannot resume from STOPPED state (system must be restarted)")
			}
			return fmt.Errorf("system is not PAUSED, circuit breaker not tripped, and sprint is not at CHECKPOINT (current mode: %s, sprint status: %s)", currentMode, s.Sprint.Status)
		}

		if isPaused {
			s.Config.Mode = models.SystemModeRunning
			s.Config.ModeChangedAt = &timestamp
			s.Config.ModeChangedBy = &changedBy
			resumedFrom = "PAUSED mode"
		} else if isCircuitBreakerTripped {
			s.Config.Mode = models.SystemModeRunning
			s.Config.ModeChangedAt = &timestamp
			s.Config.ModeChangedBy = &changedBy
			s.CircuitBreaker.Status = "OK"
			s.CircuitBreaker.CurrentTrigger = nil
			resumedFrom = "CIRCUIT_BREAKER_TRIPPED mode"
		}

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
		return nil, fmt.Errorf("failed to resume system: %w", err)
	}

	return &ResumeResult{
		ResumedFrom: resumedFrom,
		ChangedBy:   changedBy,
	}, nil
}
