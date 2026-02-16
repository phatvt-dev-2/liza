package commands

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

const (
	// DefaultCheckInterval is how often to run checks
	DefaultCheckInterval = 10 * time.Second
	// LeaseGracePeriod is the grace period after lease expiry before alerting
	LeaseGracePeriod = 120 * time.Second
	// StallThreshold is when to alert about no log activity
	StallThreshold = 30 * time.Minute
	// StaleDraftThreshold is when to alert about stale drafts
	StaleDraftThreshold = 30 * time.Minute
	// CheckpointStaleThreshold is first warning for stale checkpoint
	CheckpointStaleThreshold = 30 * time.Minute
	// CheckpointStuckThreshold is second warning for stuck checkpoint
	CheckpointStuckThreshold = 2 * time.Hour
	// CheckpointAbandonedThreshold is critical warning for abandoned checkpoint
	CheckpointAbandonedThreshold = 8 * time.Hour
	// PauseStaleThreshold is first warning for stale pause
	PauseStaleThreshold = 30 * time.Minute
	// PauseForgottenThreshold is critical warning for forgotten pause
	PauseForgottenThreshold = 2 * time.Hour
	// OrphanedGracePeriod is grace period before alerting about orphaned rejected tasks
	OrphanedGracePeriod = 30 * time.Second
)

// alertLevel represents the severity of an alert
type alertLevel string

const (
	alertLevelWarning  alertLevel = "⚠️"
	alertLevelCritical alertLevel = "🚨"
)

// Alert represents a monitoring alert
type alert struct {
	Timestamp time.Time
	Level     alertLevel
	Category  string
	Message   string
}

// String formats the alert for display
func (a alert) String() string {
	return fmt.Sprintf("[%s] %s %s: %s",
		a.Timestamp.UTC().Format(time.RFC3339),
		a.Level,
		a.Category,
		a.Message)
}

// WatchConfig configures the watch command
type WatchConfig struct {
	ProjectRoot   string
	CheckInterval time.Duration
	AlertsLog     string
	// StateCache is used to track seen alerts across checks
	StateCache map[string]time.Time
}

// WatchCommand runs the monitoring daemon
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

	statePath := lizaPaths.StatePath()
	logPath := lizaPaths.LogPath()

	fmt.Printf("[%s] Watching %s\n",
		time.Now().UTC().Format("15:04:05"),
		lizaPaths.LizaDir())

	ticker := time.NewTicker(config.CheckInterval)
	defer ticker.Stop()

	// Run checks immediately on start
	if err := runChecks(ctx, config, statePath, logPath); err != nil {
		fmt.Fprintf(os.Stderr, "Check error: %v\n", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := runChecks(ctx, config, statePath, logPath); err != nil {
				fmt.Fprintf(os.Stderr, "Check error: %v\n", err)
			}
		}
	}
}

// runChecks executes all monitoring checks
func runChecks(_ context.Context, config WatchConfig, statePath, logPath string) error {
	// Check if state file exists
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		return nil // Skip checks if no state yet
	}

	// Read state
	bb := db.New(statePath)
	state, err := bb.Read()
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	// Run all checks
	var alerts []alert

	alerts = append(alerts, checkExpiredLeases(state)...)
	alerts = append(alerts, checkBlockedTasks(state, config.StateCache)...)
	alerts = append(alerts, checkOrphanedRejected(state, config.StateCache)...)
	alerts = append(alerts, checkReviewLoops(state)...)
	alerts = append(alerts, checkIntegrationFailures(state)...)
	alerts = append(alerts, checkHypothesisExhaustion(state)...)
	alerts = append(alerts, checkReassigned(state, config.StateCache)...)
	alerts = append(alerts, checkApproachingLimits(state)...)
	alerts = append(alerts, checkStalled(logPath, config.StateCache)...)
	alerts = append(alerts, checkStaleDrafts(state)...)
	alerts = append(alerts, checkImmediateDiscoveries(state)...)

	// Validate state by calling ValidateCommand
	// Use the state path directly for validation
	if err := ValidateCommand(statePath, true); err != nil {
		alerts = append(alerts, alert{
			Timestamp: time.Now().UTC(),
			Level:     alertLevelCritical,
			Category:  "INVALID STATE",
			Message:   err.Error(),
		})
	}

	// Write alerts
	for _, alert := range alerts {
		if err := writeAlert(config.AlertsLog, alert); err != nil {
			return fmt.Errorf("failed to write alert: %w", err)
		}
		fmt.Fprintln(os.Stderr, alert.String())
	}

	return nil
}

// checkExpiredLeases checks for expired coder and reviewer leases
func checkExpiredLeases(state *models.State) []alert {
	var alerts []alert
	now := time.Now().UTC()
	graceDeadline := now.Add(-LeaseGracePeriod)

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

	// Check reviewer leases (READY_FOR_REVIEW tasks)
	for _, task := range state.Tasks {
		if task.Status != models.TaskStatusReadyForReview {
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

// checkBlockedTasks checks for tasks in BLOCKED status
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

// checkOrphanedRejected checks for REJECTED tasks assigned to inactive agents
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

// checkReviewLoops checks for tasks at or above the review cycle limit
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

// checkIntegrationFailures checks for tasks in INTEGRATION_FAILED status
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

// checkHypothesisExhaustion checks for tasks with multiple failed coders
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

// checkReassigned checks for tasks reassigned to different coders
func checkReassigned(state *models.State, cache map[string]time.Time) []alert {
	var alerts []alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		if task.Status != models.TaskStatusClaimed {
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

// checkApproachingLimits checks for tasks approaching iteration or review cycle limits
func checkApproachingLimits(state *models.State) []alert {
	var alerts []alert
	now := time.Now().UTC()

	for _, task := range state.Tasks {
		// Coder iterations: warn at 8, cliff at 10
		if task.Status == models.TaskStatusClaimed && task.Iteration >= 8 && task.Iteration < 10 {
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

// checkStalled checks for no log activity in the last 30 minutes
// Throttles alerts to once every 5 minutes to reduce log noise
func checkStalled(logPath string, cache map[string]time.Time) []alert {
	var alerts []alert
	now := time.Now().UTC()

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return alerts
	}

	// Read log entries
	data, err := os.ReadFile(logPath)
	if err != nil || len(data) == 0 {
		return alerts
	}

	// Parse to find last timestamp
	lines := strings.Split(string(data), "\n")
	var lastTimestamp time.Time

	// Find last non-empty line with timestamp
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || line == "-" {
			continue
		}
		// Look for "timestamp:" field
		if strings.Contains(line, "timestamp:") {
			parts := strings.SplitN(line, "timestamp:", 2)
			if len(parts) == 2 {
				timestampStr := strings.TrimSpace(parts[1])
				if t, err := time.Parse(time.RFC3339, timestampStr); err == nil {
					lastTimestamp = t
					break
				}
			}
		}
	}

	if !lastTimestamp.IsZero() {
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
	}

	return alerts
}

// checkStaleDrafts checks for DRAFT tasks older than 30 minutes
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
				Message: fmt.Sprintf("%s — created %dmin ago, never finalized (Planner crash?)",
					task.ID, int(age.Minutes())),
			})
		}
	}

	return alerts
}

// checkImmediateDiscoveries checks for immediate urgency discoveries not converted to tasks
func checkImmediateDiscoveries(state *models.State) []alert {
	var alerts []alert
	now := time.Now().UTC()

	for _, disc := range state.Discovered {
		if disc.Urgency == "immediate" && disc.ConvertedToTask == nil {
			alerts = append(alerts, alert{
				Timestamp: now,
				Level:     alertLevelCritical,
				Category:  "IMMEDIATE DISCOVERY",
				Message:   fmt.Sprintf("%s — %s (Planner should wake)", disc.ID, disc.Description),
			})
		}
	}

	return alerts
}

// writeAlert appends an alert to the alerts log file
func writeAlert(alertsLog string, alert alert) error {
	f, err := os.OpenFile(alertsLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open alerts log: %w", err)
	}
	defer f.Close()

	_, err = fmt.Fprintln(f, alert.String())
	if err != nil {
		return fmt.Errorf("failed to write alert: %w", err)
	}

	return nil
}
