package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/liza-mas/liza/internal/analysis"
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/log"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/paths"
)

const (
	DefaultCheckInterval         = 10 * time.Second
	StallThreshold               = 30 * time.Minute
	StaleDraftThreshold          = 30 * time.Minute
	CheckpointStaleThreshold     = 30 * time.Minute
	CheckpointStuckThreshold     = 2 * time.Hour
	CheckpointAbandonedThreshold = 8 * time.Hour
	PauseStaleThreshold          = 30 * time.Minute
	PauseForgottenThreshold      = 2 * time.Hour
	OrphanedGracePeriod          = 30 * time.Second
)

type alertLevel string

const (
	alertLevelWarning  alertLevel = "⚠️"
	alertLevelCritical alertLevel = "🚨"
)

type alert struct {
	Timestamp time.Time
	Level     alertLevel
	Category  string
	Message   string
}

func (a alert) String() string {
	return fmt.Sprintf("[%s] %s %s: %s",
		a.Timestamp.UTC().Format(time.RFC3339),
		a.Level,
		a.Category,
		a.Message)
}

type WatchConfig struct {
	ProjectRoot   string
	CheckInterval time.Duration
	AlertsLog     string
	// StateCache is used to track seen alerts across checks
	StateCache map[string]time.Time
}

func WatchCommand(ctx context.Context, config WatchConfig) error {
	if config.CheckInterval == 0 {
		config.CheckInterval = DefaultCheckInterval
	}
	lizaPaths := paths.New(config.ProjectRoot)
	if config.AlertsLog == "" {
		config.AlertsLog = lizaPaths.AlertsLogPath()
	}
	if config.StateCache == nil {
		config.StateCache = make(map[string]time.Time)
	}

	fmt.Printf("[%s] Watching %s\n",
		time.Now().UTC().Format("15:04:05"),
		lizaPaths.LizaDir())

	ticker := time.NewTicker(config.CheckInterval)
	defer ticker.Stop()

	// Run checks immediately on start
	if err := runChecks(ctx, config); err != nil {
		fmt.Fprintf(os.Stderr, "Check error: %v\n", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := runChecks(ctx, config); err != nil {
				fmt.Fprintf(os.Stderr, "Check error: %v\n", err)
			}
		}
	}
}

func runChecks(_ context.Context, config WatchConfig) error {
	lizaPaths := paths.New(config.ProjectRoot)
	statePath := lizaPaths.StatePath()

	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		return nil
	}

	bb := db.For(statePath)
	state, err := bb.Read()
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	logPath := lizaPaths.LogPath()
	checks := []func() []alert{
		func() []alert { return checkExpiredLeases(state) },
		func() []alert { return checkBlockedTasks(state, config.StateCache) },
		func() []alert { return checkOrphanedRejected(state, config.StateCache) },
		func() []alert { return checkReviewLoops(state) },
		func() []alert { return checkIntegrationFailures(state) },
		func() []alert { return checkHypothesisExhaustion(state) },
		func() []alert { return checkReassigned(state, config.StateCache) },
		func() []alert { return checkApproachingLimits(state) },
		func() []alert { return checkStalled(logPath, config.StateCache) },
		func() []alert { return checkStaleDrafts(state) },
		func() []alert { return checkImmediateDiscoveries(state) },
	}

	var alerts []alert
	for _, check := range checks {
		alerts = append(alerts, check()...)
	}

	circuitBreakerAlerts, err := checkCircuitBreakerEscalation(config.ProjectRoot, state)
	if err != nil {
		alerts = append(alerts, alert{
			Timestamp: time.Now().UTC(),
			Level:     alertLevelCritical,
			Category:  "CIRCUIT BREAKER ERROR",
			Message:   err.Error(),
		})
	} else {
		alerts = append(alerts, circuitBreakerAlerts...)
	}

	sprintStalledAlerts, err := checkSprintStalled(config.ProjectRoot, state, config.StateCache)
	if err != nil {
		alerts = append(alerts, alert{
			Timestamp: time.Now().UTC(),
			Level:     alertLevelCritical,
			Category:  "SPRINT STALL ERROR",
			Message:   err.Error(),
		})
	} else {
		alerts = append(alerts, sprintStalledAlerts...)
	}

	if err := ValidateCommand(statePath, true); err != nil {
		alerts = append(alerts, alert{
			Timestamp: time.Now().UTC(),
			Level:     alertLevelCritical,
			Category:  "INVALID STATE",
			Message:   err.Error(),
		})
	}

	for _, a := range alerts {
		if err := writeAlert(config.AlertsLog, a); err != nil {
			return fmt.Errorf("failed to write alert: %w", err)
		}
		fmt.Fprintln(os.Stderr, a.String())
	}

	return nil
}

func checkCircuitBreakerEscalation(projectRoot string, state *models.State) ([]alert, error) {
	mode := state.Config.Mode
	if mode == "" {
		mode = models.SystemModeRunning
	}

	// Auto-escalation should only run during active execution.
	if mode != models.SystemModeRunning || state.Sprint.Status != models.SprintStatusInProgress {
		return nil, nil
	}

	// Keep both checks: manual edits or interrupted writes can leave one field stale.
	// Either value indicates a previously triggered circuit-breaker state.
	if state.CircuitBreaker.Status == "TRIGGERED" || state.CircuitBreaker.CurrentTrigger != nil {
		return nil, nil
	}

	patternResult := analysis.DetectPatterns(state.Anomalies)
	if !patternResult.Triggered {
		return nil, nil
	}

	// Analyze/Checkpoint each re-read state under lock. This snapshot pre-check is
	// best-effort only and may race with manual mode/sprint changes.
	analyzeResult, err := ops.Analyze(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("auto circuit-breaker analysis failed: %w", err)
	}
	if !analyzeResult.Triggered {
		return nil, nil
	}

	sprintCheckpointResult, err := ops.SprintCheckpoint(projectRoot)
	if err != nil {
		// Another process may checkpoint between read and mutation.
		if errors.Is(err, ops.ErrSprintAlreadyCheckpoint) {
			return []alert{{
				Timestamp: time.Now().UTC(),
				Level:     alertLevelCritical,
				Category:  "CIRCUIT BREAKER",
				Message: fmt.Sprintf("pattern=%s severity=%s report=%s (sprint already at CHECKPOINT)",
					analyzeResult.Pattern, analyzeResult.Severity, analyzeResult.ReportPath),
			}}, nil
		}
		return nil, fmt.Errorf("auto checkpoint after circuit-breaker trigger failed: %w", err)
	}

	timestamp := time.Now().UTC()
	return []alert{
		{
			Timestamp: timestamp,
			Level:     alertLevelCritical,
			Category:  "CIRCUIT BREAKER",
			Message: fmt.Sprintf("pattern=%s severity=%s report=%s",
				analyzeResult.Pattern, analyzeResult.Severity, analyzeResult.ReportPath),
		},
		{
			Timestamp: timestamp,
			Level:     alertLevelCritical,
			Category:  "AUTO CHECKPOINT",
			Message: fmt.Sprintf("created at %s report=%s",
				sprintCheckpointResult.CheckpointAt.UTC().Format(time.RFC3339), sprintCheckpointResult.ReportPath),
		},
	}, nil
}

func checkSprintStalled(projectRoot string, state *models.State, cache map[string]time.Time) ([]alert, error) {
	mode := state.Config.Mode
	if mode == "" {
		mode = models.SystemModeRunning
	}

	if mode != models.SystemModeRunning || state.Sprint.Status != models.SprintStatusInProgress {
		// Clear throttle when sprint leaves IN_PROGRESS (e.g. after checkpoint).
		// This ensures that if the human resumes without unblocking tasks,
		// the next stall detection re-triggers a fresh checkpoint.
		delete(cache, "sprint_stalled:alert")
		return nil, nil
	}

	if !state.SprintStalled() {
		delete(cache, "sprint_stalled:alert")
		return nil, nil
	}

	// Throttle: only alert once per stall event within a single IN_PROGRESS period.
	// The sprint status guard above resets the throttle across checkpoint/resume cycles.
	if _, seen := cache["sprint_stalled:alert"]; seen {
		return nil, nil
	}

	// Count blocked planned tasks for the message
	blockedCount := 0
	for _, taskID := range state.Sprint.Scope.Planned {
		task := state.FindTask(taskID)
		if task != nil && task.Status == models.TaskStatusBlocked {
			blockedCount++
		}
	}

	// ops.SprintCheckpoint re-reads state under lock. Another process may checkpoint
	// between our snapshot read and this call (same race pattern as circuit breaker).
	sprintCheckpointResult, err := ops.SprintCheckpoint(projectRoot)
	if err != nil {
		if errors.Is(err, ops.ErrSprintAlreadyCheckpoint) {
			cache["sprint_stalled:alert"] = time.Now().UTC()
			return []alert{{
				Timestamp: time.Now().UTC(),
				Level:     alertLevelCritical,
				Category:  "SPRINT STALLED",
				Message: fmt.Sprintf("all %d non-terminal planned tasks are BLOCKED (sprint already at CHECKPOINT)",
					blockedCount),
			}}, nil
		}
		return nil, fmt.Errorf("auto checkpoint after sprint stall failed: %w", err)
	}

	cache["sprint_stalled:alert"] = time.Now().UTC()
	timestamp := time.Now().UTC()
	return []alert{
		{
			Timestamp: timestamp,
			Level:     alertLevelCritical,
			Category:  "SPRINT STALLED",
			Message: fmt.Sprintf("all %d non-terminal planned tasks are BLOCKED",
				blockedCount),
		},
		{
			Timestamp: timestamp,
			Level:     alertLevelCritical,
			Category:  "AUTO CHECKPOINT",
			Message: fmt.Sprintf("created at %s report=%s",
				sprintCheckpointResult.CheckpointAt.UTC().Format(time.RFC3339), sprintCheckpointResult.ReportPath),
		},
	}, nil
}

func checkExpiredLeases(state *models.State) []alert {
	var alerts []alert
	now := time.Now().UTC()
	graceDeadline := now.Add(-models.LeaseExpiryGracePeriod)

	// Check agent leases (coders with active tasks)
	for agentID, agent := range state.Agents {
		if agent.CurrentTask == nil {
			continue
		}
		if agent.LeaseExpires == nil {
			continue
		}
		if agent.LeaseExpires.Before(graceDeadline) {
			alerts = append(alerts, alert{
				Timestamp: now,
				Level:     alertLevelWarning,
				Category:  "LEASE EXPIRED",
				Message:   fmt.Sprintf("%s on %s", agentID, *agent.CurrentTask),
			})
		}
	}

	// Check reviewer leases (REVIEWING tasks with expired leases)
	for _, task := range state.Tasks {
		if task.Status != models.TaskStatusReviewing {
			continue
		}
		if task.ReviewingBy == nil {
			continue
		}
		if task.ReviewLeaseExpires == nil {
			continue
		}
		if task.ReviewLeaseExpires.Before(graceDeadline) {
			alerts = append(alerts, alert{
				Timestamp: now,
				Level:     alertLevelWarning,
				Category:  "REVIEW LEASE EXPIRED",
				Message:   fmt.Sprintf("%s on %s — review can be reclaimed", *task.ReviewingBy, task.ID),
			})
		}
	}

	return alerts
}

func checkBlockedTasks(state *models.State, cache map[string]time.Time) []alert {
	var alerts []alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		if task.Status != models.TaskStatusBlocked {
			continue
		}

		cacheKey := "blocked:" + task.ID
		if _, seen := cache[cacheKey]; !seen {
			reason := "no reason"
			if task.BlockedReason != nil {
				reason = *task.BlockedReason
			}
			alerts = append(alerts, alert{
				Timestamp: now,
				Level:     alertLevelWarning,
				Category:  "BLOCKED",
				Message:   fmt.Sprintf("%s — %s", task.ID, reason),
			})
			cache[cacheKey] = now
		}
	}

	return alerts
}

func checkOrphanedRejected(state *models.State, cache map[string]time.Time) []alert {
	var alerts []alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		if task.Status != models.TaskStatusRejected {
			continue
		}
		if task.AssignedTo == nil {
			continue
		}

		assignee := *task.AssignedTo
		agent, exists := state.Agents[assignee]
		agentStatus := "MISSING"
		if exists {
			agentStatus = string(agent.Status)
		}

		if agentStatus != "WORKING" {
			cacheKey := "orphaned:" + task.ID
			firstSeen, seen := cache[cacheKey]
			if !seen {
				cache[cacheKey] = now
			} else if now.Sub(firstSeen) > OrphanedGracePeriod {
				alerts = append(alerts, alert{
					Timestamp: now,
					Level:     alertLevelCritical,
					Category:  "ORPHANED REJECTED",
					Message: fmt.Sprintf("%s — assigned to %s but agent is %s (orphaned %ds+)",
						task.ID, assignee, agentStatus, int(OrphanedGracePeriod.Seconds())),
				})
				delete(cache, cacheKey) // Alert once per grace period
			}
		} else {
			// Agent is working, clear cache
			delete(cache, "orphaned:"+task.ID)
		}
	}

	return alerts
}

func checkReviewLoops(state *models.State) []alert {
	var alerts []alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		if task.ReviewCyclesCurrent >= 5 {
			alerts = append(alerts, alert{
				Timestamp: now,
				Level:     alertLevelCritical,
				Category:  "REVIEW LOOP",
				Message:   fmt.Sprintf("%s — %d cycles (at cliff)", task.ID, task.ReviewCyclesCurrent),
			})
		}
	}

	return alerts
}

func checkIntegrationFailures(state *models.State) []alert {
	var alerts []alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		if task.Status == models.TaskStatusIntegrationFailed {
			alerts = append(alerts, alert{
				Timestamp: now,
				Level:     alertLevelCritical,
				Category:  "INTEGRATION FAILED",
				Message:   task.ID,
			})
		}
	}

	return alerts
}

func checkHypothesisExhaustion(state *models.State) []alert {
	var alerts []alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		if len(task.FailedBy) >= 2 {
			alerts = append(alerts, alert{
				Timestamp: now,
				Level:     alertLevelCritical,
				Category:  "HYPOTHESIS EXHAUSTION",
				Message:   fmt.Sprintf("%s — requires rescope", task.ID),
			})
		}
	}

	return alerts
}

func checkReassigned(state *models.State, cache map[string]time.Time) []alert {
	var alerts []alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		if task.Status != models.TaskStatusImplementing {
			continue
		}
		if task.AssignedTo == nil {
			continue
		}

		// Find first claimer from history
		var firstClaimer string
		for _, entry := range task.History {
			if entry.Event == "claimed" && entry.Agent != nil {
				firstClaimer = *entry.Agent
				break
			}
		}

		if firstClaimer != "" && *task.AssignedTo != firstClaimer {
			cacheKey := "reassigned:" + task.ID
			if _, seen := cache[cacheKey]; !seen {
				alerts = append(alerts, alert{
					Timestamp: now,
					Level:     alertLevelWarning,
					Category:  "REASSIGNED",
					Message: fmt.Sprintf("%s — now %s (was %s), hypothesis exhaustion risk",
						task.ID, *task.AssignedTo, firstClaimer),
				})
				cache[cacheKey] = now
			}
		}
	}

	return alerts
}

func checkApproachingLimits(state *models.State) []alert {
	var alerts []alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		// Coder iterations: warn at 8, cliff at 10
		if task.Status == models.TaskStatusImplementing && task.Iteration >= 8 && task.Iteration < 10 {
			alerts = append(alerts, alert{
				Timestamp: now,
				Level:     alertLevelWarning,
				Category:  "APPROACHING LIMIT",
				Message:   fmt.Sprintf("%s — iteration %d/10", task.ID, task.Iteration),
			})
		}

		// Review cycles: warn at 3, cliff at 5
		if task.ReviewCyclesCurrent >= 3 && task.ReviewCyclesCurrent < 5 {
			alerts = append(alerts, alert{
				Timestamp: now,
				Level:     alertLevelWarning,
				Category:  "APPROACHING LIMIT",
				Message:   fmt.Sprintf("%s — review cycle %d/5", task.ID, task.ReviewCyclesCurrent),
			})
		}

		// Coder failures: warn at 1 IF review_cycles_current >= 3
		if len(task.FailedBy) == 1 && task.ReviewCyclesCurrent >= 3 {
			alerts = append(alerts, alert{
				Timestamp: now,
				Level:     alertLevelWarning,
				Category:  "APPROACHING LIMIT",
				Message: fmt.Sprintf("%s — 1 coder failed + %d review cycles (hypothesis exhaustion risk)",
					task.ID, task.ReviewCyclesCurrent),
			})
		}
	}

	return alerts
}

// Throttles stall alerts to once every 5 minutes.
func checkStalled(logPath string, cache map[string]time.Time) []alert {
	var alerts []alert
	now := time.Now().UTC()

	// Use typed log parsing to get the last timestamp
	logger := log.New(logPath)
	lastTimestamp, err := logger.GetLastTimestamp()
	if err != nil || lastTimestamp.IsZero() {
		return alerts
	}

	age := time.Since(lastTimestamp)
	if age > StallThreshold {
		// Throttle alerts to once every 5 minutes
		cacheKey := "stalled:alert"
		lastAlert, seen := cache[cacheKey]
		if !seen || now.Sub(lastAlert) >= 5*time.Minute {
			alerts = append(alerts, alert{
				Timestamp: now,
				Level:     alertLevelWarning,
				Category:  "STALLED",
				Message:   fmt.Sprintf("no progress for %d minutes", int(age.Minutes())),
			})
			cache[cacheKey] = now
		}
	} else {
		// Clear cache if system is no longer stalled
		delete(cache, "stalled:alert")
	}

	return alerts
}

func checkStaleDrafts(state *models.State) []alert {
	var alerts []alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		if task.Status != models.TaskStatusDraft {
			continue
		}

		age := now.Sub(task.Created)
		if age > StaleDraftThreshold {
			alerts = append(alerts, alert{
				Timestamp: now,
				Level:     alertLevelWarning,
				Category:  "STALE DRAFT",
				Message: fmt.Sprintf("%s — created %dmin ago, never finalized (Orchestrator crash?)",
					task.ID, int(age.Minutes())),
			})
		}
	}

	return alerts
}

func checkImmediateDiscoveries(state *models.State) []alert {
	var alerts []alert
	now := time.Now().UTC()

	for _, disc := range state.Discovered {
		if disc.Urgency == "immediate" && disc.ConvertedToTask == nil {
			alerts = append(alerts, alert{
				Timestamp: now,
				Level:     alertLevelCritical,
				Category:  "IMMEDIATE DISCOVERY",
				Message:   fmt.Sprintf("%s — %s (Orchestrator should wake)", disc.ID, disc.Description),
			})
		}
	}

	return alerts
}

func writeAlert(alertsLog string, a alert) error {
	f, err := os.OpenFile(alertsLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open alerts log: %w", err)
	}
	defer f.Close()

	_, err = fmt.Fprintln(f, a.String())
	if err != nil {
		return fmt.Errorf("failed to write alert: %w", err)
	}

	return nil
}
