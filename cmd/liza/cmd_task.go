package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/spf13/cobra"
)

var claimTaskCmd = &cobra.Command{
	Use:   "claim-task <task-id> <agent-id>",
	Short: "Claim a task for a doer agent",
	Long: `Claim a task for a doer agent using the three-phase claim pattern.

Supports claiming from multiple source states:
  - Initial state: normal new claim (e.g. DRAFT_CODE, DRAFT_CODING_PLAN, DRAFT_EPIC_PLAN, DRAFT_US)
  - Rejected state: re-claim (same doer preserves worktree, different doer gets fresh)
  - INTEGRATION_FAILED: any doer can claim (worktree preserved for conflict resolution)

Phase 1: Validate under lock (check status, deps, agent availability)
Phase 2: Handle worktree outside lock (create/preserve/delete as needed)
Phase 3: Re-validate and commit under lock (atomic state update)

This pattern prevents TOCTOU races in multi-agent scenarios.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		agentID := args[1]

		projectRoot, err := requireProjectRoot()
		if err != nil {
			return err
		}

		return commands.ClaimTaskCommand(projectRoot, taskID, agentID)
	},
}

var addTaskCmd = &cobra.Command{
	Use:   "add-task",
	Short: "Add a new task to the state",
	Long: `Add a new task to state.yaml with the specified properties.

Task details can be provided via CLI flags or loaded from a YAML file using --file.
When using --file, CLI flags can override specific fields from the file.

Updates sprint.scope.planned, goal.alignment_history, and logs the action.
Runs validation after adding the task.

Example YAML file format:
  id: task-1
  description: Implement feature X
  spec_ref: specs/vision.md
  done_when: Feature X is implemented and tested
  scope: Add feature X to the codebase
  priority: 1
  depends_on:
    - task-0`,
	RunE: func(cmd *cobra.Command, args []string) error {
		statePath, _ := cmd.Flags().GetString("state")
		logPath, _ := cmd.Flags().GetString("log")

		if statePath == "" && logPath == "" {
			statePath = filepath.Join(paths.LizaDirName, paths.StateFileName)
			logPath = filepath.Join(paths.LizaDirName, paths.LogFileName)
		} else if statePath != "" && logPath == "" {
			return fmt.Errorf("if --state is provided, --log must also be provided")
		} else if statePath == "" && logPath != "" {
			return fmt.Errorf("if --log is provided, --state must also be provided")
		}

		filePath, _ := cmd.Flags().GetString("file")
		var input *commands.TaskInput

		if filePath != "" {
			var err error
			input, err = commands.LoadTaskInputFromFile(filePath)
			if err != nil {
				return err
			}
		} else {
			input = &commands.TaskInput{}
		}

		if cmd.Flags().Changed("id") {
			input.ID, _ = cmd.Flags().GetString("id")
		}
		if cmd.Flags().Changed("desc") {
			input.Description, _ = cmd.Flags().GetString("desc")
		}
		if cmd.Flags().Changed("spec") {
			specVal, _ := cmd.Flags().GetString("spec")
			absSpec, err := filepath.Abs(specVal)
			if err != nil {
				return fmt.Errorf("failed to resolve spec path: %w", err)
			}
			input.SpecRef = absSpec
		}
		if cmd.Flags().Changed("done") {
			input.DoneWhen, _ = cmd.Flags().GetString("done")
		}
		if cmd.Flags().Changed("scope") {
			input.Scope, _ = cmd.Flags().GetString("scope")
		}
		if cmd.Flags().Changed("priority") {
			input.Priority, _ = cmd.Flags().GetInt("priority")
		}
		if cmd.Flags().Changed("depends") {
			dependsStr, _ := cmd.Flags().GetString("depends")
			if dependsStr != "" {
				input.DependsOn = strings.Split(dependsStr, ",")
			} else {
				input.DependsOn = []string{}
			}
		}
		if cmd.Flags().Changed("type") {
			input.Type, _ = cmd.Flags().GetString("type")
		}

		if input.Priority == 0 {
			input.Priority = 1
		}

		orchestratorID, err := resolveOrchestratorID(cmd)
		if err != nil {
			return err
		}

		return commands.AddTaskCommand(statePath, logPath, input, orchestratorID)
	},
}

var supersedeTaskCmd = &cobra.Command{
	Use:   "supersede-task <task-id> <replacement-task-ids> <rescope-reason>",
	Short: "Mark a task as SUPERSEDED by replacement tasks",
	Long: `Mark a task as SUPERSEDED when it needs to be replaced by new task(s).

Used by orchestrator when rescoping blocked, rejected, or problematic tasks.

Requirements:
  - Task must be in BLOCKED, rejected, or initial status
  - At least one replacement task ID must be provided
  - Rescope reason must explain why the task is being superseded

The replacement task IDs should be comma-separated.

Example:
  liza supersede-task task-3 task-4,task-5 "Split into smaller tasks due to complexity"`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		replacementIDsStr := args[1]
		reason := args[2]

		replacementIDs := strings.Split(replacementIDsStr, ",")
		for i := range replacementIDs {
			replacementIDs[i] = strings.TrimSpace(replacementIDs[i])
		}

		agentID, err := resolveOrchestratorID(cmd)
		if err != nil {
			return err
		}

		projectRoot, err := requireProjectRoot()
		if err != nil {
			return err
		}

		return commands.SupersedeTaskCommand(projectRoot, taskID, replacementIDs, reason, agentID)
	},
}

var markBlockedCmd = &cobra.Command{
	Use:   "mark-blocked <task-id>",
	Short: "Mark a task as BLOCKED due to unresolvable blocker",
	Long: `Mark a task as BLOCKED when work cannot proceed.

Per the blocking protocol (specs/architecture/roles.md), use this when:
  - Spec ambiguity prevents implementation
  - Missing external dependency blocks progress
  - Design conflict discovered that requires rescoping

Requirements:
  - Agent ID must be provided (via --agent-id flag or LIZA_AGENT_ID env var)
  - Task must be in an executing status (e.g. IMPLEMENTING_CODE, CODE_PLANNING)
  - Only the assigned agent can mark a task as blocked
  - Requires a reason and 1-3 clarifying questions

Effects:
  - status = BLOCKED
  - blocked_reason = <reason>
  - blocked_questions = [<questions>]
  - Clear assigned_to
  - Clear lease_expires
  - Add history entry with event "blocked"
  - Triggers orchestrator wake`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]

		reason, _ := cmd.Flags().GetString("reason")
		questions, _ := cmd.Flags().GetStringSlice("questions")

		agentID, err := requireAgentID(cmd)
		if err != nil {
			return err
		}

		projectRoot, err := requireProjectRoot()
		if err != nil {
			return err
		}

		return commands.MarkBlockedCommand(projectRoot, taskID, reason, questions, agentID)
	},
}

var deleteTaskCmd = &cobra.Command{
	Use:   "task <task-id>",
	Short: "Delete a task from the state database",
	Long: `Remove a task from the state database.

Useful for removing tasks that were created but are no longer needed. Tasks
in MERGED state cannot be deleted by default (as they represent integrated work).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		force, _ := cmd.Flags().GetBool("force")
		deleteWorktree, _ := cmd.Flags().GetBool("delete-worktree")
		reason, _ := cmd.Flags().GetString("reason")
		projectRoot, err := requireProjectRoot()
		if err != nil {
			return err
		}
		return commands.DeleteTaskCommand(projectRoot, taskID, force, deleteWorktree, reason, os.Stdin)
	},
}

func init() {
	rootCmd.AddCommand(claimTaskCmd)
	rootCmd.AddCommand(addTaskCmd)
	rootCmd.AddCommand(supersedeTaskCmd)
	rootCmd.AddCommand(markBlockedCmd)
	deleteCmd.AddCommand(deleteTaskCmd)

	// Mark-blocked command flags
	markBlockedCmd.Flags().String("reason", "", "reason why the task is blocked (required)")
	markBlockedCmd.Flags().StringSlice("questions", nil, "clarifying questions (1-3 required)")
	markBlockedCmd.Flags().String("agent-id", "", "agent ID marking the task as blocked")
	markBlockedCmd.MarkFlagRequired("reason")
	markBlockedCmd.MarkFlagRequired("questions")

	// Add-task command flags
	addTaskCmd.Flags().String("file", "", "path to YAML file containing task details")
	addTaskCmd.Flags().String("id", "", "task ID (required unless using --file)")
	addTaskCmd.Flags().String("desc", "", "task description (required unless using --file)")
	addTaskCmd.Flags().String("spec", "", "spec reference (required unless using --file)")
	addTaskCmd.Flags().String("done", "", "done-when criteria (required unless using --file)")
	addTaskCmd.Flags().String("scope", "", "task scope (required unless using --file)")
	addTaskCmd.Flags().Int("priority", 0, "task priority (default: 1, overrides file value)")
	addTaskCmd.Flags().String("depends", "", "comma-separated list of task IDs this task depends on (overrides file value)")
	addTaskCmd.Flags().String("type", "", "task type determining role workflow (default: coding)")
	addTaskCmd.Flags().String("state", "", "path to state.yaml (default: .liza/state.yaml)")
	addTaskCmd.Flags().String("log", "", "path to log.yaml (default: .liza/log.yaml)")

	// Delete task command flags
	deleteTaskCmd.Flags().Bool("force", false, "force deletion even if task has dependencies or is in restricted state")
	deleteTaskCmd.Flags().Bool("delete-worktree", false, "also delete the associated git worktree and branch")
	deleteTaskCmd.Flags().String("reason", "manual deletion", "reason for deleting the task")
}
