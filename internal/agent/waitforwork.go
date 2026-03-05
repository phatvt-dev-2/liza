package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/roles"
)

// nonZeroOr returns val if positive, otherwise fallback.
func nonZeroOr(val, fallback int) int {
	if val > 0 {
		return val
	}
	return fallback
}

// getRoleWaitConfig returns poll interval and max wait based on role-specific config.
// Falls back to shell-script parity defaults when config values are unset.
func getRoleWaitConfig(state *models.State, role string) (pollInterval, maxWait time.Duration) {
	var pollSeconds, maxWaitSeconds int

	switch role {
	case roles.RuntimeOrchestrator:
		pollSeconds = nonZeroOr(state.Config.OrchestratorPollInterval, models.DefaultOrchestratorPollInterval)
		maxWaitSeconds = nonZeroOr(state.Config.OrchestratorMaxWait, models.DefaultOrchestratorMaxWait)
	case roles.RuntimeCodeReviewer, roles.RuntimeCodePlanReviewer:
		pollSeconds = nonZeroOr(state.Config.ReviewerPollInterval, models.DefaultReviewerPollInterval)
		maxWaitSeconds = nonZeroOr(state.Config.ReviewerMaxWait, models.DefaultReviewerMaxWait)
	default:
		pollSeconds = nonZeroOr(state.Config.CoderPollInterval, models.DefaultCoderPollInterval)
		maxWaitSeconds = nonZeroOr(state.Config.CoderMaxWait, models.DefaultCoderMaxWait)
	}

	return time.Duration(pollSeconds) * time.Second, time.Duration(maxWaitSeconds) * time.Second
}

// waitForWork is a dispatcher to role-specific wait functions
func waitForWork(ctx context.Context, bb *db.Blackboard, projectRoot string, role string, config SupervisorConfig, pollInterval, maxWait time.Duration) (bool, error) {
	logger := GetLogger()

	// Orchestrators use the configured maxWait from getRoleWaitConfig, which provides
	// a default if OrchestratorMaxWait is not set. The orchestrator wait loop will exit
	// on ABORT/state change or when maxWait is reached.
	logger.Debug("agent waiting for work", "maxWait", maxWait, "role", role)

	switch role {
	case roles.RuntimeCoder:
		return waitForCoderWork(ctx, bb, projectRoot, config.AgentID, pollInterval, maxWait)
	case roles.RuntimeCodePlanner:
		return waitForCodePlannerWork(ctx, bb, projectRoot, config.AgentID, pollInterval, maxWait)
	case roles.RuntimeCodeReviewer:
		return waitForReviewerWork(ctx, bb, projectRoot, pollInterval, maxWait)
	case roles.RuntimeCodePlanReviewer:
		return waitForCodePlanReviewerWork(ctx, bb, projectRoot, pollInterval, maxWait)
	case roles.RuntimeOrchestrator:
		return waitForOrchestratorWork(ctx, bb, projectRoot, pollInterval, maxWait)
	default:
		return false, fmt.Errorf("unknown role: %s", role)
	}
}

// workCheckFunc checks if work is available for an agent role.
// Returns (hasWork, logMessage). If logMessage is non-empty, it will be printed when work is found.
type workCheckFunc func(*models.State) (hasWork bool, logMessage string)

// waitForWorkEventDriven is a generic event-driven wait implementation for all agent roles.
// It uses fsnotify to detect state changes and wake immediately when work becomes available.
func waitForWorkEventDriven(
	ctx context.Context,
	bb *db.Blackboard,
	projectRoot string,
	pollInterval, maxWait time.Duration,
	checkWork workCheckFunc,
) (bool, error) {
	logger := GetLogger()

	// Check context cancellation before doing any work.
	// If we skip this check, and work is available, we'd return true immediately
	// even if the context was already cancelled, causing the supervisor to
	// continue running when it should stop.
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	deadline := time.Now().Add(maxWait)

	// Check immediately first
	state, err := bb.ReadCached()
	if err != nil {
		return false, fmt.Errorf("failed to read state: %w", err)
	}

	// Check for ABORT before checking for work
	if stopped, reason := isSystemStopped(state); stopped {
		logger.Info("ABORT detected", "reason", reason)
		return false, nil
	}

	if hasWork, logMsg := checkWork(state); hasWork {
		if logMsg != "" {
			logger.Info(logMsg)
		}
		return true, nil
	} else if state.Config.DiagnosticLogging && logMsg != "" {
		// Only show "no work" diagnostics if enabled
		logger.Info(logMsg)
	}

	// Try to set up event-driven watching
	watcher, err := bb.WatchForChanges()
	if err != nil {
		// Fallback to polling if watcher fails
		return waitForWorkPolling(ctx, bb, projectRoot, pollInterval, maxWait, checkWork)
	}
	defer watcher.Close()

	// Add ticker for periodic ABORT checks (file-based fallback)
	abortTicker := time.NewTicker(5 * time.Second)
	defer abortTicker.Stop()

	// Deadline timer — created once to avoid timer leak in the select loop
	deadlineTimer := time.NewTimer(time.Until(deadline))
	defer deadlineTimer.Stop()

	// Event-driven wait loop
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()

		case <-abortTicker.C:
			state, err := bb.ReadCached()
			if err != nil {
				return false, fmt.Errorf("failed to read state: %w", err)
			}
			if stopped, reason := isSystemStopped(state); stopped {
				logger.Info("ABORT detected", "reason", reason)
				return false, nil
			}
			if time.Now().After(deadline) {
				return false, nil
			}

		case <-watcher.Events():
			// State changed, check for work
			state, err := bb.ReadCached()
			if err != nil {
				return false, fmt.Errorf("failed to read state: %w", err)
			}

			// Check for ABORT before checking for work
			if stopped, reason := isSystemStopped(state); stopped {
				logger.Info("ABORT detected", "reason", reason)
				return false, nil
			}

			if hasWork, logMsg := checkWork(state); hasWork {
				if logMsg != "" {
					logger.Info(logMsg)
				}
				return true, nil
			} else if state.Config.DiagnosticLogging && logMsg != "" {
				// Only show "no work" diagnostics if enabled
				logger.Info(logMsg)
			}

			if time.Now().After(deadline) {
				return false, nil
			}

		case err := <-watcher.Errors():
			// Watcher error, fallback to polling
			logger.Warn("Watcher error, falling back to polling", "error", err)
			watcher.Close()
			return waitForWorkPolling(ctx, bb, projectRoot, pollInterval, maxWait, checkWork)

		case <-deadlineTimer.C:
			return false, nil
		}
	}
}

// waitForWorkPolling is a generic polling wait implementation for all agent roles.
// This is used as a fallback when fsnotify is unavailable or encounters errors.
func waitForWorkPolling(
	ctx context.Context,
	bb *db.Blackboard,
	projectRoot string,
	pollInterval, maxWait time.Duration,
	checkWork workCheckFunc,
) (bool, error) {
	logger := GetLogger()
	deadline := time.Now().Add(maxWait)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-ticker.C:
			state, err := bb.Read()
			if err != nil {
				return false, fmt.Errorf("failed to read state: %w", err)
			}

			// Check for ABORT before checking for work
			if stopped, reason := isSystemStopped(state); stopped {
				logger.Info("ABORT detected", "reason", reason)
				return false, nil
			}

			if hasWork, logMsg := checkWork(state); hasWork {
				if logMsg != "" {
					logger.Info(logMsg)
				}
				return true, nil
			} else if state.Config.DiagnosticLogging && logMsg != "" {
				// Only show "no work" diagnostics if enabled
				logger.Info(logMsg)
			}

			if time.Now().After(deadline) {
				return false, nil
			}
		}
	}
}

// waitForCoderWork waits for claimable tasks or resumable handoff tasks.
func waitForCoderWork(ctx context.Context, bb *db.Blackboard, projectRoot, agentID string, pollInterval, maxWait time.Duration) (bool, error) {
	pr := ops.LoadResolverForModels(projectRoot)
	return waitForWorkEventDriven(ctx, bb, projectRoot, pollInterval, maxWait,
		func(s *models.State) (bool, string) {
			claimable := models.CountClaimableTasks(s, models.RoleCoder, pr)
			resumableHandoffs := countResumableHandoffTasks(s, agentID, pr)
			logMsg := models.GetCoderWorkDiagnostics(s, pr)

			if resumableHandoffs > 0 {
				handoffMsg := fmt.Sprintf("Found %d resumable handoff task(s) for %s", resumableHandoffs, agentID)
				if logMsg != "" {
					logMsg = handoffMsg + "; " + logMsg
				} else {
					logMsg = handoffMsg
				}
			}

			return claimable > 0 || resumableHandoffs > 0, logMsg
		})
}

func waitForCodePlannerWork(ctx context.Context, bb *db.Blackboard, projectRoot, agentID string, pollInterval, maxWait time.Duration) (bool, error) {
	pr := ops.LoadResolverForModels(projectRoot)
	return waitForWorkEventDriven(ctx, bb, projectRoot, pollInterval, maxWait,
		func(s *models.State) (bool, string) {
			claimable := models.CountClaimableTasks(s, models.RoleCodePlanner, pr)
			resumableHandoffs := countResumableHandoffTasks(s, agentID, pr)
			logMsg := fmt.Sprintf("code-planner: %d claimable, %d resumable handoffs", claimable, resumableHandoffs)

			return claimable > 0 || resumableHandoffs > 0, logMsg
		})
}

func isResumableHandoff(task *models.Task, agentID string, pr models.PipelineResolver) bool {
	isExecuting := task.Status == models.TaskStatusImplementing || task.Status == models.TaskStatusCodePlanning
	// For pipeline tasks, also check pipeline-defined executing status.
	if !isExecuting && task.RolePair != "" && pr != nil {
		executing, err := pr.ExecutingStatus(task.RolePair)
		if err == nil && task.Status == executing {
			isExecuting = true
		}
	}
	return isExecuting &&
		task.HandoffPending &&
		task.AssignedTo != nil &&
		*task.AssignedTo == agentID
}

func countResumableHandoffTasks(state *models.State, agentID string, pr models.PipelineResolver) int {
	count := 0
	for i := range state.Tasks {
		if isResumableHandoff(&state.Tasks[i], agentID, pr) {
			count++
		}
	}
	return count
}

// waitForReviewerWork waits for reviewable tasks using event-driven detection
func waitForReviewerWork(ctx context.Context, bb *db.Blackboard, projectRoot string, pollInterval, maxWait time.Duration) (bool, error) {
	if cleared, err := ops.ClearStaleReviewClaims(projectRoot); err != nil {
		GetLogger().Warn("Failed to clear stale review claims before reviewer wait", "error", err)
	} else if cleared > 0 {
		GetLogger().Info("Cleared stale review claims before reviewer wait", "count", cleared)
	}

	pr := ops.LoadResolverForModels(projectRoot)
	return waitForWorkEventDriven(ctx, bb, projectRoot, pollInterval, maxWait,
		func(s *models.State) (bool, string) {
			count := models.CountReviewableTasks(s, models.RoleCodeReviewer, pr)
			logMsg := models.GetReviewerWorkDiagnostics(s, pr)
			return count > 0, logMsg
		})
}

func waitForCodePlanReviewerWork(ctx context.Context, bb *db.Blackboard, projectRoot string, pollInterval, maxWait time.Duration) (bool, error) {
	if cleared, err := ops.ClearStaleReviewClaims(projectRoot); err != nil {
		GetLogger().Warn("Failed to clear stale review claims before code-plan-reviewer wait", "error", err)
	} else if cleared > 0 {
		GetLogger().Info("Cleared stale review claims before code-plan-reviewer wait", "count", cleared)
	}

	pr := ops.LoadResolverForModels(projectRoot)
	return waitForWorkEventDriven(ctx, bb, projectRoot, pollInterval, maxWait,
		func(s *models.State) (bool, string) {
			count := models.CountReviewableTasks(s, models.RoleCodePlanReviewer, pr)
			if count > 0 {
				return true, fmt.Sprintf("Found %d code-plan-reviewable task(s)", count)
			}
			return false, "No code-plan-reviewable tasks"
		})
}

// waitForOrchestratorWork waits for orchestrator wake triggers using event-driven detection
func waitForOrchestratorWork(ctx context.Context, bb *db.Blackboard, projectRoot string, pollInterval, maxWait time.Duration) (bool, error) {
	pipelineTerminals := ops.SprintTerminalStates(projectRoot)
	return waitForWorkEventDriven(ctx, bb, projectRoot, pollInterval, maxWait,
		func(s *models.State) (bool, string) {
			result := DetectOrchestratorWakeTriggers(s, pipelineTerminals)
			if result.Trigger != WakeTriggerNone {
				return true, fmt.Sprintf("Orchestrator wake trigger: %s (count: %d)", result.Trigger, result.Count)
			}
			return false, ""
		})
}
