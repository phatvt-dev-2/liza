package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/prompts"
)

// SupervisorConfig contains all configuration for the agent supervisor
type SupervisorConfig struct {
	AgentID          string
	Role             string // "coder", "code-reviewer", "planner"
	ProjectRoot      string
	StatePath        string
	LogPath          string
	SpecsDir         string // For prompt building
	CLIName          string // "claude", "codex", "gemini", "mistral"
	Interactive      bool   // Print prompt location, don't execute
	InitialTask      string // Optional task ID to resume
	Executor         CLIExecutor
	ExecutionTimeout time.Duration // Max time for agent execution before timeout
}

// CLIExecutor interface for testing (mock vs real CLI)
type CLIExecutor interface {
	Execute(ctx context.Context, cliName string, prompt string, projectRoot string) (exitCode int, err error)
	// ExecuteInteractive launches the CLI without a prompt arg, with stdin connected,
	// so the user can paste the prompt manually. Used by -i (interactive) mode.
	ExecuteInteractive(ctx context.Context, cliName string, projectRoot string) (exitCode int, err error)
}

// DefaultCLIExecutor implements real CLI execution
type DefaultCLIExecutor struct{}

func (d *DefaultCLIExecutor) Execute(ctx context.Context, cliName string, prompt string, projectRoot string) (int, error) {
	// Map CLI names (mistral -> vibe)
	actualCLI := cliName
	if cliName == "mistral" {
		actualCLI = "vibe"
	}

	// Build command based on CLI
	var cmd *exec.Cmd
	switch actualCLI {
	case "claude":
		cmd = exec.CommandContext(ctx, "claude", "-p", prompt)
	case "codex":
		cmd = exec.CommandContext(ctx, "codex", "exec", prompt)
	case "gemini":
		cmd = exec.CommandContext(ctx, "gemini", "-p", prompt)
	case "vibe":
		cmd = exec.CommandContext(ctx, "vibe", "-p", prompt)
	default:
		return 0, fmt.Errorf("unknown CLI: %s", cliName)
	}

	// Set working directory to project root so claude can find .mcp.json and .claude/settings.json
	cmd.Dir = projectRoot

	// Run command and capture exit code
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Don't inherit stdin - agents are autonomous and don't require input.
	// Inheriting stdin causes the subprocess to block indefinitely waiting for EOF,
	// preventing clean exit after work completion.
	cmd.Stdin = nil

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 0, err
	}

	return 0, nil
}

func (d *DefaultCLIExecutor) ExecuteInteractive(ctx context.Context, cliName string, projectRoot string) (int, error) {
	// Map CLI names (mistral -> vibe)
	actualCLI := cliName
	if cliName == "mistral" {
		actualCLI = "vibe"
	}

	// Launch CLI without prompt arg — user pastes prompt manually
	var cmd *exec.Cmd
	switch actualCLI {
	case "codex":
		cmd = exec.CommandContext(ctx, "codex")
	default:
		cmd = exec.CommandContext(ctx, actualCLI)
	}

	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 0, err
	}

	return 0, nil
}

// RunSupervisor is the main entry point for the agent supervisor
func RunSupervisor(ctx context.Context, config SupervisorConfig) error {
	bb := db.New(config.StatePath)
	lizaPaths := paths.New(config.ProjectRoot)

	// Validate identity
	if err := validateIdentity(config.AgentID, config.Role); err != nil {
		return err
	}

	// Register agent (sets STARTING → IDLE)
	if err := registerAgent(bb, config.ProjectRoot, config.AgentID, config.Role, "terminal-1", 1800); err != nil {
		return err
	}
	defer unregisterAgent(bb, config.AgentID)

	// Load config from state
	state, err := bb.Read()
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	pollInterval, maxWait := getRoleWaitConfig(state, config.Role)

	// Set execution timeout if not configured
	if config.ExecutionTimeout == 0 {
		// Default timeouts based on role
		switch config.Role {
		case "code-reviewer":
			config.ExecutionTimeout = 30 * time.Minute
		case "coder":
			config.ExecutionTimeout = 2 * time.Hour
		case "planner":
			config.ExecutionTimeout = 4 * time.Hour
		default:
			config.ExecutionTimeout = 2 * time.Hour
		}
	}

	const maxMergeRetries = 3
	mergeRetries := 0

	for {
		// Check ABORT
		if checkAbort(config.ProjectRoot) {
			GetLogger().Info("ABORT signal received, system shutting down")
			return nil
		}

		// Wait while PAUSE/CHECKPOINT
		if err := waitWhilePaused(ctx, config.ProjectRoot); err != nil {
			return err
		}

		// Handle approved merges (reviewer only)
		if config.Role == "code-reviewer" {
			if err := handleApprovedMerges(config.ProjectRoot, config.AgentID, bb); err != nil {
				GetLogger().Warn("Merge handler error", "error", err)
			}

			// If there are still pending merges (transient errors), retry with
			// backoff up to a max count, then proceed to waitForWork
			if hasPendingMerges(bb, config.AgentID) {
				mergeRetries++
				if mergeRetries <= maxMergeRetries {
					delay := time.Duration(mergeRetries) * time.Second
					GetLogger().Info("Pending merges remain, retrying after delay",
						"agent_id", config.AgentID,
						"retry", mergeRetries,
						"delay", delay)
					time.Sleep(delay)
					continue
				}
				GetLogger().Warn("Max merge retries reached, proceeding to wait for work",
					"agent_id", config.AgentID,
					"retries", mergeRetries)
				mergeRetries = 0
			} else {
				mergeRetries = 0
			}
		}

		// Wait for work
		hasWork, err := waitForWork(ctx, bb, config.ProjectRoot, config.Role, config, pollInterval, maxWait)
		if err != nil {
			return err
		}
		if !hasWork {
			GetLogger().Info("No work available, supervisor exiting")
			return nil
		}

		// Claim task (coder/reviewer only)
		var taskID string
		var claimedTaskID string // Track claimed task for completion logging
		if config.Role == "coder" {
			taskID, _, err = claimCoderTask(config.ProjectRoot, config.AgentID, bb)
			if err != nil {
				// Error already logged in claimCoderTask
				time.Sleep(5 * time.Second)
				continue
			}
			claimedTaskID = taskID
		} else if config.Role == "code-reviewer" {
			var reviewCommit string
			taskID, _, reviewCommit, err = claimReviewerTask(config.AgentID, 1800, bb)
			if err != nil {
				// Error already logged in claimReviewerTask
				time.Sleep(5 * time.Second)
				continue // Race condition, retry
			}

			// Log successful reviewer claim
			GetLogger().Info("Reviewer claimed task for review",
				"agent_id", config.AgentID,
				"task_id", taskID,
				"review_commit", reviewCommit)
		}

		// Set planner status to PLANNING
		if config.Role == "planner" {
			if err := setAgentToPlanningStatus(bb, config.AgentID); err != nil {
				GetLogger().Warn("Failed to set planner status", "error", err, "agent_id", config.AgentID)
			}
		}

		// Build and save prompt
		state, err := bb.Read()
		if err != nil {
			return fmt.Errorf("failed to read state for prompt: %w", err)
		}

		prompt, err := buildPrompt(state, config, taskID)
		if err != nil {
			return fmt.Errorf("failed to build prompt: %w", err)
		}

		promptFile, err := savePrompt(lizaPaths.AgentPromptsDir(), config.AgentID, prompt)
		if err != nil {
			return fmt.Errorf("failed to save prompt: %w", err)
		}
		GetLogger().Info("Prompt saved", "file", promptFile)

		// Execute agent
		exitCode, err := executeAgent(ctx, config, prompt)
		if err != nil {
			return fmt.Errorf("agent execution error: %w", err)
		}

		// Reset runtime status after CLI exits, but preserve explicit command-driven
		// states such as WAITING and HANDOFF.
		if err := resetAgentAfterExit(bb, config.AgentID); err != nil {
			GetLogger().Warn("Failed to reset agent status after exit", "error", err, "agent_id", config.AgentID)
		}

		// Handle exit code
		switch exitCode {
		case 0:
			GetLogger().Info("Agent completed, checking for more work")

			// Log task submission if it happened (coder role only)
			if config.Role == "coder" && claimedTaskID != "" {
				if err := logTaskSubmissionIfCompleted(bb, claimedTaskID, config.AgentID); err != nil {
					GetLogger().Warn("Failed to log task submission", "error", err, "task_id", claimedTaskID)
				}
			}

			// Verify expected state changes for planner
			if config.Role == "planner" {
				if err := verifyPlannerStateChanges(bb, state); err != nil {
					GetLogger().Warn("Planner state verification failed",
						"error", err,
						"hint", "Agent may not have executed required commands - check prompt file")
				}
			}
		case 42:
			GetLogger().Info("Agent aborted gracefully, restarting", "exit_code", 42, "delay_seconds", 2)
			time.Sleep(2 * time.Second)
		default:
			GetLogger().Error("Agent crashed, restarting", "exit_code", exitCode, "delay_seconds", 5)
			time.Sleep(5 * time.Second)
		}

		// Clear initial task after first run
		config.InitialTask = ""
	}
}

// validateIdentity validates agent ID format: {role}-{number}
func validateIdentity(agentID, role string) error {
	if agentID == "" {
		return fmt.Errorf("agent ID required")
	}

	// Split on last hyphen
	lastHyphen := -1
	for i := len(agentID) - 1; i >= 0; i-- {
		if agentID[i] == '-' {
			lastHyphen = i
			break
		}
	}

	if lastHyphen == -1 {
		return fmt.Errorf("invalid agent ID format (expected {role}-{number}): %s", agentID)
	}

	idRole := agentID[:lastHyphen]
	numStr := agentID[lastHyphen+1:]

	// Validate number is numeric
	if _, err := strconv.Atoi(numStr); err != nil {
		return fmt.Errorf("agent ID suffix must be numeric: %s", agentID)
	}

	// Validate role matches
	if idRole != role {
		return fmt.Errorf("agent ID role mismatch (ID=%s, config=%s)", idRole, role)
	}

	return nil
}

// registerAgent registers an agent with collision detection
func registerAgent(bb *db.Blackboard, projectRoot, agentID, role, terminal string, leaseDuration int) error {
	logger := GetLogger()
	now := time.Now().UTC()
	leaseExpires := now.Add(time.Duration(leaseDuration) * time.Second)

	// Single atomic registration - skip STARTING state, go directly to IDLE
	err := bb.Modify(func(state *models.State) error {
		// Check for collision
		if existing, exists := state.Agents[agentID]; exists {
			// Check if lease is still valid
			if existing.LeaseExpires != nil && existing.LeaseExpires.After(now) {
				return fmt.Errorf("agent ID collision: %s already registered with valid lease (expires %s)",
					agentID, existing.LeaseExpires.Format(time.RFC3339))
			}
			logger.Info("Taking over expired agent lease", "agent_id", agentID)
		}

		// Register agent directly as IDLE (atomic operation)
		pid := os.Getpid()
		state.Agents[agentID] = models.Agent{
			Role:         role,
			Status:       models.AgentStatusIdle,
			Heartbeat:    now,
			Terminal:     terminal,
			LeaseExpires: &leaseExpires,
			PID:          pid,
		}

		return nil
	})

	if err != nil {
		return err
	}

	// If code-reviewer: clear stale review claims
	if role == "code-reviewer" {
		if _, err := commands.ClearStaleReviewClaimsCommand(projectRoot); err != nil {
			logger.Warn("Failed to clear stale review claims", "error", err, "role", role)
		}
	}

	return nil
}

// unregisterAgent removes an agent from the state
func unregisterAgent(bb *db.Blackboard, agentID string) {
	logger := GetLogger()
	err := bb.Modify(func(state *models.State) error {
		delete(state.Agents, agentID)
		return nil
	})

	if err != nil {
		logger.Warn("Failed to unregister agent", "error", err, "agent_id", agentID)
	}
}

// resetAgentToIdle resets an agent's status to IDLE and clears CurrentTask
func resetAgentToIdle(bb *db.Blackboard, agentID string) error {
	now := time.Now().UTC()

	return bb.Modify(func(state *models.State) error {
		agent, exists := state.Agents[agentID]
		if !exists {
			return fmt.Errorf("agent %s not found", agentID)
		}

		// Reset to IDLE state
		agent.Status = models.AgentStatusIdle
		agent.CurrentTask = nil
		agent.Heartbeat = now

		state.Agents[agentID] = agent
		return nil
	})
}

// resetAgentAfterExit clears transient runtime states after CLI exit while preserving
// explicit command-driven states that are meaningful between loops.
func resetAgentAfterExit(bb *db.Blackboard, agentID string) error {
	now := time.Now().UTC()

	return bb.Modify(func(state *models.State) error {
		agent, exists := state.Agents[agentID]
		if !exists {
			return fmt.Errorf("agent %s not found", agentID)
		}

		switch agent.Status {
		case models.AgentStatusWaiting, models.AgentStatusHandoff:
			agent.Heartbeat = now
			state.Agents[agentID] = agent
			return nil
		default:
			agent.Status = models.AgentStatusIdle
			agent.CurrentTask = nil
			agent.Heartbeat = now
			state.Agents[agentID] = agent
			return nil
		}
	})
}

// setAgentToPlanningStatus sets a planner agent's status to PLANNING
func setAgentToPlanningStatus(bb *db.Blackboard, agentID string) error {
	now := time.Now().UTC()

	return bb.Modify(func(state *models.State) error {
		agent, exists := state.Agents[agentID]
		if !exists {
			return fmt.Errorf("agent %s not found", agentID)
		}

		// Set to PLANNING state
		agent.Status = models.AgentStatusPlanning
		planning := "planning"
		agent.CurrentTask = &planning
		agent.Heartbeat = now

		// Renew lease
		leaseDuration := state.Config.LeaseDuration
		if leaseDuration <= 0 {
			leaseDuration = 1800
		}
		leaseExpires := now.Add(time.Duration(leaseDuration) * time.Second)
		agent.LeaseExpires = &leaseExpires

		state.Agents[agentID] = agent
		return nil
	})
}

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
	case "planner":
		pollSeconds = nonZeroOr(state.Config.PlannerPollInterval, 60)
		maxWaitSeconds = nonZeroOr(state.Config.PlannerMaxWait, 1800)
	case "code-reviewer":
		pollSeconds = nonZeroOr(state.Config.ReviewerPollInterval, 30)
		maxWaitSeconds = nonZeroOr(state.Config.ReviewerMaxWait, 1800)
	default:
		pollSeconds = nonZeroOr(state.Config.CoderPollInterval, 30)
		maxWaitSeconds = nonZeroOr(state.Config.CoderMaxWait, 1800)
	}

	return time.Duration(pollSeconds) * time.Second, time.Duration(maxWaitSeconds) * time.Second
}

// waitForWork is a dispatcher to role-specific wait functions
func waitForWork(ctx context.Context, bb *db.Blackboard, projectRoot string, role string, config SupervisorConfig, pollInterval, maxWait time.Duration) (bool, error) {
	logger := GetLogger()

	// For planners, maxWait should be effectively infinite since they're persistent coordinators
	// that only exit on STOPPED mode or context cancellation
	effectiveMaxWait := maxWait
	if role == "planner" {
		// Use a very large duration to effectively wait indefinitely
		// The loop will still exit on ABORT or context cancellation
		effectiveMaxWait = 365 * 24 * time.Hour // 1 year
		logger.Info("planner agent waiting indefinitely for wake triggers (will only exit on STOPPED or cancellation)")
	} else {
		logger.Debug("agent waiting for work", "maxWait", maxWait, "role", role)
	}

	switch role {
	case "coder":
		return waitForCoderWork(ctx, bb, projectRoot, config.AgentID, pollInterval, effectiveMaxWait)
	case "code-reviewer":
		return waitForReviewerWork(ctx, bb, projectRoot, pollInterval, effectiveMaxWait)
	case "planner":
		return waitForPlannerWork(ctx, bb, projectRoot, pollInterval, effectiveMaxWait)
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
	return waitForWorkEventDriven(ctx, bb, projectRoot, pollInterval, maxWait,
		func(s *models.State) (bool, string) {
			claimable := models.CountClaimableTasks(s, models.RoleCoder)
			resumableHandoffs := countResumableHandoffTasks(s, agentID)
			logMsg := models.GetCoderWorkDiagnostics(s)

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

func isResumableHandoff(task *models.Task, agentID string) bool {
	return task.Status == models.TaskStatusImplementing &&
		task.HandoffPending &&
		task.AssignedTo != nil &&
		*task.AssignedTo == agentID
}

func countResumableHandoffTasks(state *models.State, agentID string) int {
	count := 0
	for i := range state.Tasks {
		if isResumableHandoff(&state.Tasks[i], agentID) {
			count++
		}
	}
	return count
}

// waitForReviewerWork waits for reviewable tasks using event-driven detection
func waitForReviewerWork(ctx context.Context, bb *db.Blackboard, projectRoot string, pollInterval, maxWait time.Duration) (bool, error) {
	return waitForWorkEventDriven(ctx, bb, projectRoot, pollInterval, maxWait,
		func(s *models.State) (bool, string) {
			count := models.CountReviewableTasks(s, models.RoleCodeReviewer)
			logMsg := models.GetReviewerWorkDiagnostics(s)
			return count > 0, logMsg
		})
}

// waitForPlannerWork waits for planner wake triggers using event-driven detection
func waitForPlannerWork(ctx context.Context, bb *db.Blackboard, projectRoot string, pollInterval, maxWait time.Duration) (bool, error) {
	return waitForWorkEventDriven(ctx, bb, projectRoot, pollInterval, maxWait,
		func(s *models.State) (bool, string) {
			result := DetectPlannerWakeTriggers(s)
			if result.Trigger != WakeTriggerNone {
				return true, fmt.Sprintf("Planner wake trigger: %s (count: %d)", result.Trigger, result.Count)
			}
			return false, ""
		})
}

// logTaskSubmissionIfCompleted checks if a claimed task was submitted for review
// and logs this transition for visibility in agent logs
func logTaskSubmissionIfCompleted(bb *db.Blackboard, taskID, agentID string) error {
	state, err := bb.Read()
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	// Find the task
	for i := range state.Tasks {
		task := &state.Tasks[i]
		if task.ID != taskID {
			continue
		}

		// Check if it's now READY_FOR_REVIEW
		if task.Status == models.TaskStatusReadyForReview {
			// Log the successful submission
			reviewCommit := "unknown"
			if task.ReviewCommit != nil {
				reviewCommit = *task.ReviewCommit
			}

			GetLogger().Info("Task submitted for review",
				"task_id", task.ID,
				"review_commit", reviewCommit,
				"agent_id", agentID,
				"integration_fix", task.IntegrationFix)

			return nil
		}

		// If task is still IMPLEMENTING, agent may have exited without completing
		if task.Status == models.TaskStatusImplementing {
			GetLogger().Warn("Agent exited with task still claimed",
				"task_id", task.ID,
				"agent_id", agentID,
				"hint", "Agent may have been interrupted or encountered an issue")
			return nil
		}

		// If task is BLOCKED, agent discovered a dependency issue
		if task.Status == models.TaskStatusBlocked {
			GetLogger().Info("Agent blocked task due to dependency issue",
				"task_id", task.ID,
				"agent_id", agentID)
			return nil
		}

		// Task exists but wasn't submitted (still in other status)
		// This is normal if agent exited for other reasons (context switch, failure, etc.)
		return nil
	}

	// Task not found - unusual but not an error
	return nil
}

// resumeHandoffTask looks for a handoff task assigned to agentID and resumes it.
// Returns found=false when no resumable handoff exists.
func resumeHandoffTask(bb *db.Blackboard, state *models.State, agentID string) (taskID, worktree string, found bool, err error) {
	for i := range state.Tasks {
		task := &state.Tasks[i]
		if !isResumableHandoff(task, agentID) {
			continue
		}
		if task.Worktree == nil {
			return "", "", false, fmt.Errorf("handoff task %s missing worktree", task.ID)
		}

		now := time.Now().UTC()
		id := task.ID
		wt := *task.Worktree

		err := bb.Modify(func(s *models.State) error {
			var t *models.Task
			for j := range s.Tasks {
				if s.Tasks[j].ID == id {
					t = &s.Tasks[j]
					break
				}
			}
			if t == nil {
				return fmt.Errorf("task %s not found while resuming handoff", id)
			}
			if t.Status != models.TaskStatusImplementing {
				return fmt.Errorf("task %s is no longer IMPLEMENTING", id)
			}
			if t.AssignedTo == nil || *t.AssignedTo != agentID {
				return fmt.Errorf("task %s is no longer assigned to %s", id, agentID)
			}

			if t.LeaseExpires == nil || t.LeaseExpires.Before(now) {
				leaseDuration := s.Config.LeaseDuration
				if leaseDuration <= 0 {
					leaseDuration = 1800
				}
				renewed := now.Add(time.Duration(leaseDuration) * time.Second)
				t.LeaseExpires = &renewed
			}

			t.HandoffPending = false
			agentPtr := &agentID
			t.History = append(t.History, models.TaskHistoryEntry{
				Time:  now,
				Event: "handoff_resumed",
				Agent: agentPtr,
			})

			agent, ok := s.Agents[agentID]
			if !ok {
				agent = models.Agent{Role: "coder"}
			}
			agent.Status = models.AgentStatusWorking
			agent.CurrentTask = &id
			agent.LeaseExpires = t.LeaseExpires
			agent.Heartbeat = now
			s.Agents[agentID] = agent
			return nil
		})
		if err != nil {
			GetLogger().Warn("Handoff resume conflict, trying next candidate", "task_id", id, "error", err)
			continue
		}

		return id, wt, true, nil
	}
	return "", "", false, nil
}

// claimCoderTask finds and claims a claimable task.
// If the same coder previously initiated a handoff, it resumes that task first.
func claimCoderTask(projectRoot, agentID string, bb *db.Blackboard) (taskID, worktree string, err error) {
	logger := GetLogger()

	state, err := bb.Read()
	if err != nil {
		return "", "", fmt.Errorf("failed to read state: %w", err)
	}

	id, wt, found, err := resumeHandoffTask(bb, state, agentID)
	if err != nil {
		return "", "", err
	}
	if found {
		logger.Info("Resuming claimed task from handoff", "task_id", id, "agent_id", agentID)
		return id, wt, nil
	}

	var candidates []*models.Task
	for i := range state.Tasks {
		if state.Tasks[i].IsClaimable(models.RoleCoder, state.Tasks) {
			candidates = append(candidates, &state.Tasks[i])
		}
	}
	task := selectHighestPriorityTask(candidates)

	if task == nil {
		return "", "", fmt.Errorf("no claimable tasks found")
	}

	if err := commands.ClaimTaskCommand(projectRoot, task.ID, agentID); err != nil {
		logger.Error("Claim error", "error", err)
		return "", "", err
	}

	state, err = bb.Read()
	if err != nil {
		return "", "", fmt.Errorf("failed to read state after claim: %w", err)
	}

	for i := range state.Tasks {
		if state.Tasks[i].ID == task.ID && state.Tasks[i].Worktree != nil {
			return task.ID, *state.Tasks[i].Worktree, nil
		}
	}

	return "", "", fmt.Errorf("task worktree not set after claim")
}

// claimReviewerTask finds and claims a reviewable task
func claimReviewerTask(agentID string, leaseDuration int, bb *db.Blackboard) (taskID, worktree, reviewCommit string, err error) {
	logger := GetLogger()
	now := time.Now().UTC()
	leaseExpires := now.Add(time.Duration(leaseDuration) * time.Second)

	err = bb.Modify(func(state *models.State) error {
		// Find reviewable task with highest priority
		// READY_FOR_REVIEW tasks are available for claiming (stale REVIEWING leases
		// are reverted to READY_FOR_REVIEW by ClearStaleReviewClaimsCommand)
		var candidates []*models.Task
		for i := range state.Tasks {
			if state.Tasks[i].IsClaimable(models.RoleCodeReviewer, state.Tasks) {
				candidates = append(candidates, &state.Tasks[i])
			}
		}
		task := selectHighestPriorityTask(candidates)

		if task == nil {
			return fmt.Errorf("no reviewable tasks found")
		}

		// Atomically claim the task and transition to REVIEWING
		if err := task.Transition(models.TaskStatusReviewing); err != nil {
			return err
		}
		task.ReviewingBy = &agentID
		task.ReviewLeaseExpires = &leaseExpires

		// Update agent status
		agent := state.Agents[agentID]
		agent.Status = models.AgentStatusReviewing
		currentTask := task.ID
		agent.CurrentTask = &currentTask
		agent.Heartbeat = now
		agent.LeaseExpires = &leaseExpires
		state.Agents[agentID] = agent

		// Capture values to return
		taskID = task.ID
		if task.Worktree != nil {
			worktree = *task.Worktree
		}
		if task.ReviewCommit != nil {
			reviewCommit = *task.ReviewCommit
		}

		return nil
	})

	if err != nil {
		logger.Error("Review claim error", "error", err)
		return "", "", "", err
	}

	return taskID, worktree, reviewCommit, nil
}

// buildPrompt creates the complete prompt for the agent
func buildPrompt(state *models.State, config SupervisorConfig, taskID string) (string, error) {
	// Build base prompt
	baseConfig := prompts.BasePromptConfig{
		Role:        config.Role,
		AgentID:     config.AgentID,
		SpecsDir:    config.SpecsDir,
		ProjectRoot: config.ProjectRoot,
		StatePath:   config.StatePath,
		GoalDesc:    state.Goal.Description,
		GoalSpecRef: state.Goal.SpecRef,
	}

	prompt := prompts.BuildBasePrompt(baseConfig)

	// Add role-specific context
	switch config.Role {
	case "coder":
		task := findTaskByID(state.Tasks, taskID)
		if task == nil {
			return "", fmt.Errorf("task not found: %s", taskID)
		}

		var handoffNote *models.HandoffNote
		if note, ok := state.Handoff[task.ID]; ok {
			noteCopy := note
			handoffNote = &noteCopy
		}

		coderConfig := prompts.CoderContextConfig{
			ProjectRoot:       config.ProjectRoot,
			AgentID:           config.AgentID,
			IntegrationBranch: state.Config.IntegrationBranch,
			HandoffNote:       handoffNote,
		}
		prompt += prompts.BuildCoderContext(task, coderConfig)

	case "code-reviewer":
		task := findTaskByID(state.Tasks, taskID)
		if task == nil {
			return "", fmt.Errorf("task not found: %s", taskID)
		}

		reviewerConfig := prompts.ReviewerContextConfig{
			ProjectRoot: config.ProjectRoot,
			AgentID:     config.AgentID,
		}
		prompt += prompts.BuildReviewerContext(task, reviewerConfig)

	case "planner":
		plannerConfig := prompts.PlannerContextConfig{}
		prompt += prompts.BuildPlannerContext(state, plannerConfig)
	}

	// Add resume context if initial task
	if config.InitialTask != "" {
		prompt += fmt.Sprintf("\n\n=== RESUME CONTEXT ===\nResuming task: %s\n", config.InitialTask)
	}

	return prompt, nil
}

// selectHighestPriorityTask returns the highest-priority task from candidates,
// using creation time as FIFO tie-breaker. Returns nil if candidates is empty.
func selectHighestPriorityTask(candidates []*models.Task) *models.Task {
	var best *models.Task
	for _, t := range candidates {
		if best == nil || t.Priority < best.Priority {
			best = t
		} else if t.Priority == best.Priority && best.Created.After(t.Created) {
			best = t
		}
	}
	return best
}

// findTaskByID finds a task by ID in a task slice
func findTaskByID(tasks []models.Task, taskID string) *models.Task {
	for i := range tasks {
		if tasks[i].ID == taskID {
			return &tasks[i]
		}
	}
	return nil
}

// savePrompt saves the prompt to a file and returns the path
func savePrompt(promptDir, agentID, prompt string) (string, error) {
	// Create prompt directory if missing
	if err := os.MkdirAll(promptDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create prompt directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.txt", agentID, timestamp)
	filePath := filepath.Join(promptDir, filename)

	// Write prompt
	if err := os.WriteFile(filePath, []byte(prompt), 0644); err != nil {
		return "", fmt.Errorf("failed to write prompt file: %w", err)
	}

	return filePath, nil
}

// executeAgent executes the CLI with heartbeat and timeout
func executeAgent(ctx context.Context, config SupervisorConfig, prompt string) (int, error) {
	logger := GetLogger()
	// Interactive mode: launch CLI without -p so user can paste the prompt
	if config.Interactive {
		fmt.Println("=== INTERACTIVE MODE ===")
		fmt.Println("Paste the prompt from the file above into the CLI session.")
		fmt.Printf("Launching: %s\n", config.CLIName)
		return config.Executor.ExecuteInteractive(ctx, config.CLIName, config.ProjectRoot)
	}

	// Create timeout context for CLI execution
	execCtx, cancelExec := context.WithTimeout(ctx, config.ExecutionTimeout)
	defer cancelExec()

	// Start heartbeat
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat()

	hb := NewHeartbeat(HeartbeatConfig{
		AgentID:   config.AgentID,
		StatePath: config.StatePath,
		Interval:  60 * time.Second,
	})

	go func() {
		if err := hb.Start(heartbeatCtx); err != nil && err != context.Canceled {
			logger.Error("Heartbeat error during agent execution", "error", err, "agent_id", config.AgentID)
		}
	}()

	// Execute CLI with timeout
	exitCode, err := config.Executor.Execute(execCtx, config.CLIName, prompt, config.ProjectRoot)

	// Stop heartbeat
	cancelHeartbeat()

	// Check if execution timed out
	if err != nil && errors.Is(err, context.DeadlineExceeded) {
		logger.Error("Agent execution timeout",
			"agent_id", config.AgentID,
			"timeout", config.ExecutionTimeout,
			"hint", "CLI may be hung, will retry")
		return 1, nil // Return failure code to trigger retry
	}

	// Check if timeout context was cancelled (even if Execute returned successfully)
	if execCtx.Err() == context.DeadlineExceeded {
		logger.Error("Agent execution timeout (context deadline exceeded)",
			"agent_id", config.AgentID,
			"timeout", config.ExecutionTimeout,
			"hint", "CLI may be hung, will retry")
		return 1, nil // Return failure code to trigger retry
	}

	return exitCode, err
}

// checkAbort returns true if system mode is STOPPED
func checkAbort(projectRoot string) bool {
	statePath := paths.New(projectRoot).StatePath()
	if bb := db.New(statePath); bb != nil {
		state, err := bb.Read()
		if err == nil {
			stopped, _ := isSystemStopped(state)
			return stopped
		}
	}
	return false
}

// isSystemStopped checks if system is in STOPPED mode from already-read state
// Returns (stopped bool, reason string)
func isSystemStopped(state *models.State) (bool, string) {
	if state.Config.Mode == models.SystemModeStopped {
		return true, "System mode is STOPPED"
	}
	return false, ""
}

// waitWhilePaused blocks while system is PAUSED or sprint is in CHECKPOINT status
func waitWhilePaused(ctx context.Context, projectRoot string) error {
	logger := GetLogger()
	statePath := paths.New(projectRoot).StatePath()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		isPaused := false
		pauseReason := ""

		if bb := db.New(statePath); bb != nil {
			state, err := bb.Read()
			if err == nil {
				if state.Config.Mode == models.SystemModePaused {
					isPaused = true
					pauseReason = "[PAUSED] System mode is PAUSED"
				} else if state.Config.Mode == models.SystemModeCircuitBreakerTripped {
					isPaused = true
					pauseReason = "[CIRCUIT BREAKER] Circuit breaker triggered - system halted"
				} else if state.Sprint.Status == models.SprintStatusCheckpoint {
					isPaused = true
					pauseReason = "[CHECKPOINT] Sprint is at checkpoint"
				}
			}
		}

		if !isPaused {
			// Not paused, continue
			return nil
		}

		// Wait and check again
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			logger.Info("System paused, waiting for resume", "pause_reason", pauseReason)
		}
	}
}

// handleApprovedMerges handles merging approved tasks
func handleApprovedMerges(projectRoot, agentID string, bb *db.Blackboard) error {
	logger := GetLogger()
	state, err := bb.Read()
	if err != nil {
		return err
	}

	// Find APPROVED tasks where approved_by = agentID and merge_commit = null
	for i := range state.Tasks {
		task := &state.Tasks[i]
		if task.Status == models.TaskStatusApproved &&
			task.ApprovedBy != nil && *task.ApprovedBy == agentID &&
			task.MergeCommit == nil {

			GetLogger().Info("Merging approved task", "task_id", task.ID)

			// Execute merge - WtMergeCommand handles all validation and state updates
			err := commands.WtMergeCommand(projectRoot, task.ID, agentID)
			if err != nil {
				// Check if this is an integration failure (merge conflict or test failure)
				var integrationErr *commands.IntegrationFailedError
				if errors.As(err, &integrationErr) {
					// Integration failed - state already updated, no success message
					continue
				}
				// Other error - log and continue
				logger.Warn("Failed to merge task, will retry",
					"task_id", task.ID,
					"error", err)
				continue
			}

			// Merge succeeded
			GetLogger().Info("Successfully merged task", "task_id", task.ID)
		}
	}

	return nil
}

// hasPendingMerges checks if there are APPROVED tasks awaiting merge by this agent
func hasPendingMerges(bb *db.Blackboard, agentID string) bool {
	state, err := bb.ReadCached()
	if err != nil {
		return false // Safe default: proceed to normal wait
	}

	for i := range state.Tasks {
		task := &state.Tasks[i]
		if task.Status == models.TaskStatusApproved &&
			task.ApprovedBy != nil && *task.ApprovedBy == agentID &&
			task.MergeCommit == nil {
			return true
		}
	}
	return false
}

// verifyPlannerStateChanges checks if planner made expected state changes after completion
func verifyPlannerStateChanges(bb *db.Blackboard, stateBefore *models.State) error {
	logger := GetLogger()
	// Read state after agent execution
	stateAfter, err := bb.ReadCached()
	if err != nil {
		return fmt.Errorf("failed to read state after agent execution: %w", err)
	}

	// Detect the wake trigger that caused this planner run
	result := DetectPlannerWakeTriggers(stateBefore)

	// Verify expected changes based on trigger
	switch result.Trigger {
	case WakeTriggerInitialPlanning:
		// INITIAL_PLANNING: expect tasks to be created
		if len(stateAfter.Tasks) == 0 {
			return fmt.Errorf("planner completed with INITIAL_PLANNING trigger but no tasks were created")
		}
		logger.Info("Planner created tasks", "task_count", len(stateAfter.Tasks))

	case WakeTriggerBlocked:
		// BLOCKED_TASKS: expect blocked tasks to be unblocked or superseded
		blockedBefore := 0
		blockedAfter := 0
		for _, task := range stateBefore.Tasks {
			if task.Status == models.TaskStatusBlocked {
				blockedBefore++
			}
		}
		for _, task := range stateAfter.Tasks {
			if task.Status == models.TaskStatusBlocked {
				blockedAfter++
			}
		}
		if blockedAfter >= blockedBefore {
			return fmt.Errorf("planner completed with BLOCKED_TASKS trigger but blocked count didn't decrease (before: %d, after: %d)", blockedBefore, blockedAfter)
		}
		logger.Info("Planner resolved blocked tasks", "before", blockedBefore, "after", blockedAfter)

	case WakeTriggerIntegrationFailed:
		// INTEGRATION_FAILED: expect failed tasks to be claimed by coders or handled by planner
		// Count tasks that were INTEGRATION_FAILED before planner ran
		failedBefore := 0
		failedTaskIDs := make([]string, 0)
		for _, task := range stateBefore.Tasks {
			if task.Status == models.TaskStatusIntegrationFailed {
				failedBefore++
				failedTaskIDs = append(failedTaskIDs, task.ID)
			}
		}

		// Check what happened to those tasks after planner ran
		stillFailed := 0
		claimed := 0
		superseded := 0
		for _, taskID := range failedTaskIDs {
			var afterTask *models.Task
			for i := range stateAfter.Tasks {
				if stateAfter.Tasks[i].ID == taskID {
					afterTask = &stateAfter.Tasks[i]
					break
				}
			}
			if afterTask != nil {
				switch afterTask.Status {
				case models.TaskStatusIntegrationFailed:
					stillFailed++
				case models.TaskStatusImplementing:
					claimed++
				case models.TaskStatusSuperseded:
					superseded++
				}
			}
		}

		// Success conditions:
		// 1. Tasks were claimed by coders (expected case)
		// 2. Tasks were superseded by planner (structural issue)
		// 3. Combination of both
		handled := claimed + superseded
		if handled == 0 && stillFailed == failedBefore {
			return fmt.Errorf("planner completed with INTEGRATION_FAILED trigger but no tasks were handled (still %d INTEGRATION_FAILED)", stillFailed)
		}

		logger.Info("Planner checked integration failures", "claimed", claimed, "superseded", superseded, "still_failed", stillFailed)

	case WakeTriggerHypothesisExhausted:
		// HYPOTHESIS_EXHAUSTED: expect exhausted tasks to be updated or superseded
		exhaustedBefore := 0
		exhaustedAfter := 0
		for _, task := range stateBefore.Tasks {
			if len(task.FailedBy) >= 2 {
				exhaustedBefore++
			}
		}
		for _, task := range stateAfter.Tasks {
			if len(task.FailedBy) >= 2 {
				exhaustedAfter++
			}
		}
		if exhaustedAfter >= exhaustedBefore {
			return fmt.Errorf("planner completed with HYPOTHESIS_EXHAUSTED trigger but exhausted count didn't decrease (before: %d, after: %d)", exhaustedBefore, exhaustedAfter)
		}
		logger.Info("Planner handled exhausted hypotheses", "before", exhaustedBefore, "after", exhaustedAfter)

	case WakeTriggerImmediateDiscovery:
		// IMMEDIATE_DISCOVERY: expect discoveries to be converted to tasks
		immediateBefore := 0
		immediateAfter := 0
		for _, disc := range stateBefore.Discovered {
			if disc.Urgency == "immediate" && disc.ConvertedToTask == nil {
				immediateBefore++
			}
		}
		for _, disc := range stateAfter.Discovered {
			if disc.Urgency == "immediate" && disc.ConvertedToTask == nil {
				immediateAfter++
			}
		}
		if immediateAfter >= immediateBefore {
			return fmt.Errorf("planner completed with IMMEDIATE_DISCOVERY trigger but unconverted count didn't decrease (before: %d, after: %d)", immediateBefore, immediateAfter)
		}
		logger.Info("Planner handled immediate discoveries", "before", immediateBefore, "after", immediateAfter)

	case WakeTriggerSprintComplete:
		// SPRINT_COMPLETE: expect sprint status to be CHECKPOINT (or COMPLETED)
		if stateAfter.Sprint.Status != models.SprintStatusCheckpoint && stateAfter.Sprint.Status != models.SprintStatusCompleted {
			return fmt.Errorf("planner completed with SPRINT_COMPLETE trigger but sprint status is %s (expected CHECKPOINT or COMPLETED)", stateAfter.Sprint.Status)
		}
		if stateAfter.Sprint.Timeline.CheckpointAt == nil {
			return fmt.Errorf("planner completed with SPRINT_COMPLETE trigger but checkpoint_at is not set")
		}
		logger.Info("Planner completed sprint", "status", stateAfter.Sprint.Status)
	}

	return nil
}
