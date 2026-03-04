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
	blackboard := db.For(statePath)

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
	ResumedFrom    string
	ChangedBy      string
	SprintAdvanced *AdvanceSprintResult // non-nil when sprint was advanced to next
}

// Resume transitions from PAUSED or CIRCUIT_BREAKER_TRIPPED to RUNNING,
// and/or resumes sprint from CHECKPOINT or COMPLETED. No terminal I/O.
//
// Sprint transitions:
//   - CHECKPOINT + not all terminal → IN_PROGRESS (mid-sprint resume)
//   - CHECKPOINT + all terminal → COMPLETED (sprint done, ready for proceed)
//   - COMPLETED → archive sprint, create new IN_PROGRESS sprint (advance)
//
// Mode changes and sprint operations happen in a single Modify to avoid
// partial mutations on failure.
func Resume(projectRoot, changedBy string) (*ResumeResult, error) {
	lizaPaths := paths.New(projectRoot)
	statePath := lizaPaths.StatePath()
	blackboard := db.For(statePath)

	timestamp := time.Now()
	var resumedFrom string
	var advanceResult *AdvanceSprintResult

	err := blackboard.Modify(func(s *models.State) error {
		currentMode := s.Config.Mode

		if currentMode == "" {
			currentMode = models.SystemModeRunning
		}

		// Fail fast on STOPPED — no sprint mutations allowed while system is stopped.
		if currentMode == models.SystemModeStopped {
			return fmt.Errorf("cannot resume from STOPPED state (system must be restarted)")
		}

		isPaused := currentMode == models.SystemModePaused
		isCircuitBreakerTripped := currentMode == models.SystemModeCircuitBreakerTripped
		isCheckpoint := s.Sprint.Status == models.SprintStatusCheckpoint
		isCompleted := s.Sprint.Status == models.SprintStatusCompleted

		if !isPaused && !isCircuitBreakerTripped && !isCheckpoint && !isCompleted {
			return fmt.Errorf("system is not PAUSED, circuit breaker not tripped, and sprint is not at CHECKPOINT or COMPLETED (current mode: %s, sprint status: %s)", currentMode, s.Sprint.Status)
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

		if isCompleted {
			// COMPLETED sprint — archive and create new sprint.
			// This is the second step: human ran liza proceed, now starts next sprint.
			plan, err := planSprintAdvanceFromCompleted(s, timestamp.UTC())
			if err != nil {
				return fmt.Errorf("sprint advance failed: %w", err)
			}
			archivePath := lizaPaths.SprintArchivePath(plan.archivedSprint.Number)

			if err := writeSprintArchive(archivePath, &plan.archivedSprint); err != nil {
				return fmt.Errorf("archive write failed (state unchanged): %w", err)
			}

			applySprintAdvance(s, plan)
			advanceResult = &AdvanceSprintResult{
				ArchivedSprintID: plan.archivedSprint.ID,
				NewSprintID:      plan.newSprintID,
				NewSprintNumber:  plan.newNumber,
				CarriedTasks:     plan.carriedTasks,
				ArchivePath:      archivePath,
			}
			if resumedFrom != "" {
				resumedFrom += " and COMPLETED sprint"
			} else {
				resumedFrom = "COMPLETED sprint"
			}
		} else if isCheckpoint {
			if s.AllPlannedTasksTerminal() {
				// Sprint is truly done — mark COMPLETED for human review.
				// Human runs liza proceed, then liza resume again to advance.
				s.Sprint.Status = models.SprintStatusCompleted
			} else {
				// Mid-sprint checkpoint — just resume the same sprint.
				s.Sprint.Status = models.SprintStatusInProgress
			}
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
		ResumedFrom:    resumedFrom,
		ChangedBy:      changedBy,
		SprintAdvanced: advanceResult,
	}, nil
}
