package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/pipeline"
	"github.com/liza-mas/liza/internal/roles"
)

// SupervisorConfig contains all configuration for the agent supervisor
type SupervisorConfig struct {
	AgentID          string
	Role             string // roles.RuntimeCoder, roles.RuntimeCodeReviewer, roles.RuntimeOrchestrator
	ProjectRoot      string
	StatePath        string
	LogPath          string
	SpecsDir         string // For prompt building
	CLIName          string // "claude", "codex", "gemini", "mistral", "kimi"
	Interactive      bool   // Print prompt location, don't execute
	InitialTask      string // Optional task ID to resume
	Executor         CLIExecutor
	ExecutionTimeout time.Duration // Max time for agent execution before timeout
}

type exit42RestartState struct {
	RestartCount int
	Signature    string
}

type exit42RestartOutcome struct {
	Delay        time.Duration
	RestartCount int
	BlockedTask  bool
}

type exit42RestartTracker struct {
	byKey map[string]exit42RestartState
}

func newExit42RestartTracker() *exit42RestartTracker {
	return &exit42RestartTracker{
		byKey: make(map[string]exit42RestartState),
	}
}

func (t *exit42RestartTracker) reset(taskID string) {
	if taskID == "" {
		return
	}
	delete(t.byKey, "task:"+taskID)
}

func (t *exit42RestartTracker) Handle(bb *db.Blackboard, projectRoot, role, taskID, agentID string) (exit42RestartOutcome, error) {
	state, err := bb.Read()
	if err != nil {
		return exit42RestartOutcome{}, fmt.Errorf("read state for exit-42 tracking: %w", err)
	}

	maxBackoff := effectiveExit42MaxBackoff(state.Config)
	restartLimit := effectiveExit42RestartLimit(state.Config)
	key := exit42TrackerKey(taskID, role, agentID)
	prev := t.byKey[key]

	var signature string
	if taskID != "" {
		task := state.FindTask(taskID)
		if task != nil {
			signature = exit42TaskProgressSignature(task)
		}
	}

	if prev.Signature != "" && signature != "" && prev.Signature != signature {
		prev.RestartCount = 0
	}

	prev.RestartCount++
	prev.Signature = signature

	outcome := exit42RestartOutcome{
		Delay:        computeExit42BackoffDelay(prev.RestartCount, maxBackoff),
		RestartCount: prev.RestartCount,
	}

	blockedTask := false
	if err := bb.Modify(func(s *models.State) error {
		if taskID == "" {
			return nil
		}

		task := s.FindTask(taskID)
		if task == nil {
			return nil
		}

		task.Exit42RestartCount = outcome.RestartCount

		if role != roles.RuntimeCoder {
			return nil
		}
		if outcome.RestartCount <= restartLimit {
			return nil
		}
		// Check if task is in an executing state using pipeline resolver
		pr, prErr := ops.LoadResolverForModels(projectRoot)
		if prErr != nil {
			return prErr
		}
		if !models.IsExecutingStatus(task, pr) {
			return nil
		}
		if task.AssignedTo == nil || *task.AssignedTo != agentID {
			return nil
		}

		reason := fmt.Sprintf(
			"exit code 42 restart loop detected: %d consecutive restarts without progress (threshold=%d)",
			outcome.RestartCount,
			restartLimit,
		)
		questions := []string{
			"What task/environment issue is causing repeated exit code 42 without progress?",
			"Should this task be decomposed or the spec clarified before retrying?",
		}

		pipelineTransitions, ptErr := ops.LoadPipelineTransitions(projectRoot)
		if ptErr != nil {
			return ptErr
		}
		if err := task.TransitionWith(models.TaskStatusBlocked, pipelineTransitions); err != nil {
			return err
		}

		now := time.Now().UTC()
		task.BlockedReason = &reason
		task.BlockedQuestions = questions
		task.AssignedTo = nil
		task.LeaseExpires = nil
		task.History = append(task.History, models.TaskHistoryEntry{
			Time:   now,
			Event:  models.TaskEventBlocked,
			Agent:  &agentID,
			Reason: &reason,
		})
		blockedTask = true
		return nil
	}); err != nil {
		return exit42RestartOutcome{}, fmt.Errorf("persist exit-42 tracking state: %w", err)
	}

	outcome.BlockedTask = blockedTask
	if blockedTask {
		outcome.Delay = 0
		delete(t.byKey, key)
		return outcome, nil
	}

	t.byKey[key] = prev
	return outcome, nil
}

func exit42TrackerKey(taskID, role, agentID string) string {
	if taskID != "" {
		return "task:" + taskID
	}
	return "agent:" + role + ":" + agentID
}

func effectiveExit42MaxBackoff(cfg models.Config) time.Duration {
	seconds := cfg.Exit42MaxBackoffSeconds
	if seconds <= 0 {
		seconds = models.DefaultExit42MaxBackoffSec
	}
	return time.Duration(seconds) * time.Second
}

func effectiveExit42RestartLimit(cfg models.Config) int {
	limit := cfg.Exit42RestartThreshold
	if limit <= 0 {
		limit = models.DefaultExit42RestartLimit
	}
	return limit
}

func computeExit42BackoffDelay(restartCount int, maxBackoff time.Duration) time.Duration {
	if restartCount <= 0 {
		restartCount = 1
	}
	if maxBackoff <= 0 {
		maxBackoff = time.Duration(models.DefaultExit42MaxBackoffSec) * time.Second
	}

	delay := 2 * time.Second
	if delay > maxBackoff {
		return maxBackoff
	}

	for i := 1; i < restartCount; i++ {
		if delay >= maxBackoff {
			return maxBackoff
		}
		if delay > maxBackoff/2 {
			return maxBackoff
		}
		delay *= 2
	}

	if delay > maxBackoff {
		return maxBackoff
	}
	return delay
}

func exit42TaskProgressSignature(task *models.Task) string {
	snapshot := *task
	snapshot.Exit42RestartCount = 0

	payload, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Sprintf("%s|%d|%t", task.Status, task.Iteration, task.HandoffPending)
	}
	return string(payload)
}

// CLIExecutor interface for testing (mock vs real CLI)
type CLIExecutor interface {
	Execute(ctx context.Context, cliName string, agentID string, prompt string, projectRoot string) (exitCode int, err error)
	// ExecuteInteractive launches the CLI without a prompt arg, with stdin connected,
	// so the user can paste the prompt manually. Used by -i (interactive) mode.
	ExecuteInteractive(ctx context.Context, cliName string, projectRoot string) (exitCode int, err error)
}

type DefaultCLIExecutor struct {
	outputsDir string // Directory to save agent outputs (if empty, output goes to stdout)
}

func NewDefaultCLIExecutor(outputsDir string) *DefaultCLIExecutor {
	return &DefaultCLIExecutor{outputsDir: outputsDir}
}

func (d *DefaultCLIExecutor) Execute(ctx context.Context, cliName string, agentID string, prompt string, projectRoot string) (int, error) {
	// Map CLI names (mistral -> vibe)
	actualCLI := cliName
	if cliName == "mistral" {
		actualCLI = "vibe"
	}

	// Build command based on CLI.
	// Structured output flags (stream-json, --json, etc.) are only added when --log is
	// active (outputsDir != ""), so normal runs keep human-readable terminal output.
	var cmd *exec.Cmd
	switch actualCLI {
	case "claude":
		args := []string{"-p", prompt}
		if d.outputsDir != "" {
			args = append(args, "--verbose", "--output-format", "stream-json")
		}
		cmd = exec.CommandContext(ctx, "claude", args...)
	case "codex":
		args := []string{
			"-c", fmt.Sprintf("mcp_servers.liza.command=%q", "liza-mcp"),
			"-c", fmt.Sprintf("mcp_servers.liza.args=[%q,%q]", "--project-root", projectRoot),
			"exec", prompt,
		}
		if d.outputsDir != "" {
			args = append(args, "--json")
		}
		cmd = exec.CommandContext(ctx, "codex", args...)
	case "gemini":
		args := []string{"-p", prompt}
		if d.outputsDir != "" {
			args = append(args, "--output-format", "stream-json")
		}
		cmd = exec.CommandContext(ctx, "gemini", args...)
	case "vibe":
		args := []string{"-p", prompt}
		if d.outputsDir != "" {
			args = append(args, "--output", "streaming")
		}
		cmd = exec.CommandContext(ctx, "vibe", args...)
	case "kimi":
		args := []string{"-p", prompt}
		if d.outputsDir != "" {
			args = append(args, "--verbose", "--output-format", "stream-json")
		}
		cmd = exec.CommandContext(ctx, "kimi", args...)
	default:
		return 0, fmt.Errorf("unknown CLI: %s", cliName)
	}

	// Set working directory to project root so claude can find .mcp.json and .claude/settings.json
	cmd.Dir = projectRoot

	// Don't inherit stdin - agents are autonomous and don't require input.
	// Inheriting stdin causes the subprocess to block indefinitely waiting for EOF,
	// preventing clean exit after work completion.
	cmd.Stdin = nil

	// Handle output: either save to file or stream to stdout/stderr.
	// Separate buffers avoid the concurrency issue: exec.Cmd drains each pipe
	// in its own goroutine, so each buffer is written by exactly one goroutine.
	var stdoutBuf, stderrBuf strings.Builder
	if d.outputsDir != "" {
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	err := cmd.Run()

	// Save stdout and stderr to separate files if logging is enabled.
	if d.outputsDir != "" {
		save := func(ext, content string) {
			if content == "" {
				return
			}
			if _, saveErr := saveOutput(d.outputsDir, agentID, ext, content); saveErr != nil {
				GetLogger().Warn("Failed to save agent output", "error", saveErr, "agent_id", agentID, "ext", ext)
			}
		}
		save("txt", stdoutBuf.String())
		save("err", stderrBuf.String())
	}

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
	bb := db.For(config.StatePath)
	lizaPaths := paths.New(config.ProjectRoot)

	// Validate identity
	if err := validateIdentity(config.AgentID, config.Role); err != nil {
		return err
	}

	// Load pipeline resolver for role type classification.
	// The resolver reads role definitions from the pipeline YAML,
	// enabling custom YAML-defined roles without Go code changes.
	pipelineCfg, pipelineErr := pipeline.LoadFrozen(config.ProjectRoot)
	if pipelineErr != nil {
		return fmt.Errorf("loading pipeline config for strategy selection: %w", pipelineErr)
	}
	resolver := pipeline.NewResolver(pipelineCfg)

	strategy, err := NewRoleStrategy(config.Role, resolver)
	if err != nil {
		return err
	}

	if err := registerAgent(bb, config.ProjectRoot, config.AgentID, config.Role, "terminal-1", 1800, config.CLIName); err != nil {
		return err
	}
	defer unregisterAgent(bb, config.AgentID, config.ProjectRoot)

	state, err := bb.Read()
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}

	pollInterval, maxWait := strategy.WaitConfig(state)

	// Set execution timeout if not configured
	if config.ExecutionTimeout == 0 {
		config.ExecutionTimeout = strategy.DefaultTimeout()
	}

	exit42Tracker := newExit42RestartTracker()

	for {
		// Check context cancellation (signal received)
		if ctx.Err() != nil {
			GetLogger().Info("Signal received, shutting down")
			return nil
		}

		// Check ABORT
		if checkAbort(config.ProjectRoot) {
			GetLogger().Info("ABORT signal received, system shutting down")
			return nil
		}

		// Wait while PAUSE/CHECKPOINT
		if err := waitWhilePaused(ctx, config.ProjectRoot); err != nil {
			return err
		}

		// Pre-work (reviewer: merge handling; others: no-op)
		shouldContinue, err := strategy.PreWork(ctx, bb, config)
		if err != nil {
			return err
		}
		if shouldContinue {
			continue
		}

		// Wait for work
		hasWork, err := strategy.WaitForWork(ctx, bb, config, pollInterval, maxWait)
		if err != nil {
			return err
		}
		if !hasWork {
			GetLogger().Info("No work available, supervisor exiting")
			return nil
		}

		// Claim task
		taskID, claimedTaskID, err := strategy.ClaimTask(config, bb)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		// Pre-execution (orchestrator: PLANNING status; others: no-op)
		if err := strategy.PreExecution(bb, config); err != nil {
			GetLogger().Warn("Pre-execution failed", "error", err, "agent_id", config.AgentID)
		}

		// Build and save prompt
		stateBefore, err := bb.Read()
		if err != nil {
			return fmt.Errorf("failed to read state for prompt: %w", err)
		}

		prompt, err := strategy.BuildPrompt(stateBefore, config, taskID)
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
		if err := resetAgentAfterExit(bb, config.AgentID, config.ProjectRoot); err != nil {
			GetLogger().Warn("Failed to reset agent status after exit", "error", err, "agent_id", config.AgentID)
		}

		// Handle exit code
		switch exitCode {
		case 0:
			GetLogger().Info("Agent completed, checking for more work")
			if err := strategy.PostExecution(bb, config, taskID, claimedTaskID, stateBefore); err != nil {
				GetLogger().Warn("Post-execution error", "error", err)
			}
			exit42Tracker.reset(taskID)
		case 42:
			restartTaskID := claimedTaskID
			if restartTaskID == "" {
				restartTaskID = taskID
			}

			outcome, trackErr := exit42Tracker.Handle(bb, config.ProjectRoot, config.Role, restartTaskID, config.AgentID)
			if trackErr != nil {
				GetLogger().Warn("Exit-42 tracker failed, using default retry delay",
					"error", trackErr,
					"task_id", restartTaskID)
				time.Sleep(2 * time.Second)
				break
			}

			if outcome.BlockedTask {
				GetLogger().Warn("Task transitioned to BLOCKED after repeated exit 42 restarts",
					"task_id", restartTaskID,
					"restart_count", outcome.RestartCount)
				break
			}

			GetLogger().Info("Agent aborted gracefully, restarting",
				"exit_code", 42,
				"task_id", restartTaskID,
				"restart_count", outcome.RestartCount,
				"delay_seconds", int(outcome.Delay/time.Second))
			time.Sleep(outcome.Delay)
		default:
			exit42Tracker.reset(taskID)
			GetLogger().Error("Agent crashed, restarting", "exit_code", exitCode, "delay_seconds", 5)
			time.Sleep(5 * time.Second)
		}

		// Clear initial task after first run
		config.InitialTask = ""
	}
}
