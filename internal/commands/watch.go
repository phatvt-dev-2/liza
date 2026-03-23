package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/analysis"
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/log"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/pipeline"
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

	var alerts []alert

	// Load pipeline resolver once for checks that need it.
	pr, prErr := ops.LoadResolverForModels(config.ProjectRoot)
	pipelineCacheKey := "pipeline-config-error"
	if prErr != nil && !errors.Is(prErr, pipeline.ErrConfigNotFound) {
		// Malformed config: emit one-time alert, don't spam every 10s tick.
		if _, seen := config.StateCache[pipelineCacheKey]; !seen {
			alerts = append(alerts, alert{
				Timestamp: time.Now().UTC(),
				Level:     alertLevelWarning,
				Category:  "PIPELINE CONFIG",
				Message:   prErr.Error(),
			})
			config.StateCache[pipelineCacheKey] = time.Now().UTC()
		}
	} else {
		// Clear on success (or ErrConfigNotFound) so a later regression re-alerts.
		delete(config.StateCache, pipelineCacheKey)
	}
	// pr is nil on any error — pipeline-aware checks skip gracefully.

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
		func() []alert { return checkMissingRoles(state, pr, config.StateCache) },
	}
	for _, check := range checks {
		alerts = append(alerts, check()...)
	}

	alerts = append(alerts, checkCircuitBreakerEscalation(state, config.StateCache)...)
	alerts = append(alerts, checkSprintStalled(state, config.StateCache)...)

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

func checkCircuitBreakerEscalation(state *models.State, cache map[string]time.Time) []alert {
	mode := state.Config.Mode
	if mode == "" {
		mode = models.SystemModeRunning
	}

	// Only check during active execution.
	if mode != models.SystemModeRunning || state.Sprint.Status != models.SprintStatusInProgress {
		delete(cache, "circuit_breaker:alert")
		return nil
	}

	// Keep both checks: manual edits or interrupted writes can leave one field stale.
	// Either value indicates a previously triggered circuit-breaker state.
	if state.CircuitBreaker.Status == "TRIGGERED" || state.CircuitBreaker.CurrentTrigger != nil {
		delete(cache, "circuit_breaker:alert")
		return nil
	}

	patternResult := analysis.DetectPatterns(state.Anomalies)
	if !patternResult.Triggered {
		delete(cache, "circuit_breaker:alert")
		return nil
	}

	// Throttle: only alert once per triggered period.
	if _, seen := cache["circuit_breaker:alert"]; seen {
		return nil
	}

	cache["circuit_breaker:alert"] = time.Now().UTC()
	return []alert{{
		Timestamp: time.Now().UTC(),
		Level:     alertLevelCritical,
		Category:  "CIRCUIT BREAKER",
		Message: fmt.Sprintf("pattern=%s severity=%s — run 'liza analyze' then 'liza sprint-checkpoint'",
			patternResult.Pattern, patternResult.Severity),
	}}
}

func checkSprintStalled(state *models.State, cache map[string]time.Time) []alert {
	mode := state.Config.Mode
	if mode == "" {
		mode = models.SystemModeRunning
	}

	if mode != models.SystemModeRunning || state.Sprint.Status != models.SprintStatusInProgress {
		// Clear throttle when sprint leaves IN_PROGRESS (e.g. after checkpoint).
		// This ensures that if the human resumes without unblocking tasks,
		// the next stall detection re-triggers a fresh alert.
		delete(cache, "sprint_stalled:alert")
		return nil
	}

	if !state.SprintStalled() {
		delete(cache, "sprint_stalled:alert")
		return nil
	}

	// Throttle: only alert once per stall event within a single IN_PROGRESS period.
	// The sprint status guard above resets the throttle across checkpoint/resume cycles.
	if _, seen := cache["sprint_stalled:alert"]; seen {
		return nil
	}

	blockedCount := 0
	for _, taskID := range state.Sprint.Scope.Planned {
		task := state.FindTask(taskID)
		if task != nil && task.Status == models.TaskStatusBlocked {
			blockedCount++
		}
	}

	cache["sprint_stalled:alert"] = time.Now().UTC()
	return []alert{{
		Timestamp: time.Now().UTC(),
		Level:     alertLevelCritical,
		Category:  "SPRINT STALLED",
		Message: fmt.Sprintf("all %d non-terminal planned tasks are BLOCKED",
			blockedCount),
	}}
}

func checkExpiredLeases(state *models.State) []alert {
	var alerts []alert
	now := time.Now().UTC()
	graceDeadline := now.Add(-models.LeaseExpiryGracePeriod)

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

		// Sentinel AssignedTo (e.g. "$transitioning") is a transition in
		// progress, not an orphaned assignment. Clear any stale cache entry
		// from before the transition to prevent false-positive alerts when
		// the task becomes genuinely orphaned later.
		if strings.HasPrefix(*task.AssignedTo, "$") {
			delete(cache, "orphaned:"+task.ID)
			continue
		}

		assignee := *task.AssignedTo
		agent, exists := state.Agents[assignee]
		agentStatus := "MISSING"
		if exists {
			agentStatus = string(agent.Status)
		}

		if agentStatus == "WORKING" {
			delete(cache, "orphaned:"+task.ID)
			continue
		}

		cacheKey := "orphaned:" + task.ID
		firstSeen, seen := cache[cacheKey]
		if !seen {
			cache[cacheKey] = now
			continue
		}
		if now.Sub(firstSeen) > OrphanedGracePeriod {
			alerts = append(alerts, alert{
				Timestamp: now,
				Level:     alertLevelCritical,
				Category:  "ORPHANED REJECTED",
				Message: fmt.Sprintf("%s — assigned to %s but agent is %s (orphaned %ds+)",
					task.ID, assignee, agentStatus, int(OrphanedGracePeriod.Seconds())),
			})
			delete(cache, cacheKey)
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
		if task.EffectiveAttempt() != 2 {
			continue
		}

		cacheKey := "attempt2:" + task.ID
		if _, seen := cache[cacheKey]; !seen {
			alerts = append(alerts, alert{
				Timestamp: now,
				Level:     alertLevelWarning,
				Category:  "ATTEMPT",
				Message:   fmt.Sprintf("%s — attempt 2 (final attempt)", task.ID),
			})
			cache[cacheKey] = now
		}
	}

	return alerts
}

func checkApproachingLimits(state *models.State) []alert {
	var alerts []alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		attemptNum := task.EffectiveAttempt()

		// Coder iterations: warn at 8, cliff at 10
		if task.Status == models.TaskStatusImplementing && task.Iteration >= 8 && task.Iteration < 10 {
			var msg string
			if attemptNum == 2 {
				msg = fmt.Sprintf("%s — attempt 2 (final), iteration %d/10", task.ID, task.Iteration)
			} else {
				msg = fmt.Sprintf("%s — attempt %d, iteration %d/10", task.ID, attemptNum, task.Iteration)
			}
			alerts = append(alerts, alert{
				Timestamp: now,
				Level:     alertLevelWarning,
				Category:  "APPROACHING LIMIT",
				Message:   msg,
			})
		}

		// Review cycles: warn at 3, cliff at 5
		if task.ReviewCyclesCurrent >= 3 && task.ReviewCyclesCurrent < 5 {
			var msg string
			if attemptNum == 2 {
				msg = fmt.Sprintf("%s — attempt 2 (final), review cycle %d/5", task.ID, task.ReviewCyclesCurrent)
			} else {
				msg = fmt.Sprintf("%s — attempt %d, review cycle %d/5", task.ID, attemptNum, task.ReviewCyclesCurrent)
			}
			alerts = append(alerts, alert{
				Timestamp: now,
				Level:     alertLevelWarning,
				Category:  "APPROACHING LIMIT",
				Message:   msg,
			})
		}
	}

	return alerts
}

// Throttles stall alerts to once every 5 minutes.
func checkStalled(logPath string, cache map[string]time.Time) []alert {
	var alerts []alert
	now := time.Now().UTC()

	logger := log.New(logPath)
	lastTimestamp, err := logger.GetLastTimestamp()
	if err != nil || lastTimestamp.IsZero() {
		return alerts
	}

	age := time.Since(lastTimestamp)
	if age <= StallThreshold {
		delete(cache, "stalled:alert")
		return alerts
	}

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

// checkMissingRoles alerts when claimable tasks exist but no agent of the
// required role is registered. This catches a common first-user mistake (e.g.,
// starting only a coder but not a code-planner).
//
// Design trade-off: Uses IsClaimable which checks both status AND dependency
// satisfaction, so this only alerts when tasks are *immediately* stuck. Tasks
// blocked by unmet deps won't trigger an alert even if the needed role is
// missing — the alert fires later when deps resolve. This is conservative
// (fewer false positives) at the cost of delayed detection.
func checkMissingRoles(state *models.State, pr models.PipelineResolver, cache map[string]time.Time) []alert {
	if pr == nil {
		return nil
	}

	// Build set of registered runtime roles from state.Agents.
	// Any agent with a matching Role field counts, regardless of status —
	// the point is: is there anyone who could eventually claim this work?
	registeredRoles := make(map[string]bool)
	for _, agent := range state.Agents {
		if agent.Role != "" {
			registeredRoles[agent.Role] = true
		}
	}

	// Map missing runtime role → list of claimable task IDs waiting for that role.
	missingRoleTasks := make(map[string][]string)

	for i := range state.Tasks {
		task := &state.Tasks[i]
		if task.Status.IsTerminal() || task.RolePair == "" {
			continue
		}

		// Resolve doer and reviewer runtime roles for this task's role-pair.
		doerRuntime, err := pr.DoerRole(task.RolePair)
		if err != nil {
			continue // unknown role pair — skip gracefully
		}
		reviewerRuntime, err := pr.ReviewerRole(task.RolePair)
		if err != nil {
			continue
		}

		// Check doer: skip if role is registered, use role directly for IsClaimable.
		if !registeredRoles[doerRuntime] && task.IsClaimable(doerRuntime, state.Tasks, pr) {
			missingRoleTasks[doerRuntime] = append(missingRoleTasks[doerRuntime], task.ID)
		}

		// Check reviewer: same pattern.
		if !registeredRoles[reviewerRuntime] && task.IsClaimable(reviewerRuntime, state.Tasks, pr) {
			missingRoleTasks[reviewerRuntime] = append(missingRoleTasks[reviewerRuntime], task.ID)
		}
	}

	// Emit alerts for each missing role, throttled by cache.
	var alerts []alert
	now := time.Now().UTC()

	// Sort keys for deterministic alert order.
	sortedRoles := make([]string, 0, len(missingRoleTasks))
	for role := range missingRoleTasks {
		sortedRoles = append(sortedRoles, role)
	}
	sort.Strings(sortedRoles)

	for _, role := range sortedRoles {
		taskIDs := missingRoleTasks[role]
		cacheKey := "missing-role:" + role
		if _, seen := cache[cacheKey]; seen {
			continue
		}

		// Format task list, capping at 5 IDs.
		const maxListed = 5
		listed := taskIDs
		suffix := ""
		if len(taskIDs) > maxListed {
			listed = taskIDs[:maxListed]
			suffix = fmt.Sprintf("... and %d more", len(taskIDs)-maxListed)
		}
		msg := fmt.Sprintf("no registered agent for role %s — %d task(s) waiting (%s",
			role, len(taskIDs), strings.Join(listed, ", "))
		if suffix != "" {
			msg += ", " + suffix
		}
		msg += ")"

		alerts = append(alerts, alert{
			Timestamp: now,
			Level:     alertLevelWarning,
			Category:  "MISSING ROLE",
			Message:   msg,
		})
		cache[cacheKey] = now
	}

	// Clear cache entries for roles no longer in the missing set — either because
	// an agent appeared or because the waiting tasks stopped being claimable
	// (merged, abandoned, deps unmet, etc.). Without this, a stale cache entry
	// would suppress the alert if a *new* task later becomes claimable for the
	// same absent role.
	for key := range cache {
		if !strings.HasPrefix(key, "missing-role:") {
			continue
		}
		role := strings.TrimPrefix(key, "missing-role:")
		if _, stillMissing := missingRoleTasks[role]; !stillMissing {
			delete(cache, key)
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
