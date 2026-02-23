package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/paths"
)

// SupervisorConfig contains all configuration for the agent supervisor
type SupervisorConfig struct {
	AgentID          string
	Role             string // "coder", "code-reviewer", "planner"
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
	case "kimi":
		cmd = exec.CommandContext(ctx, "kimi", "-p", prompt) // kimi is an alias to claude with Kimi specific env vars
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
	bb := db.For(config.StatePath)
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
