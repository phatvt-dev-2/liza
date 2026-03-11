package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/liza-mas/liza/internal/commands"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("liza version %s\n", Version)
		fmt.Printf("  commit: %s\n", GitCommit)
		fmt.Printf("  built:  %s\n", BuildDate)
	},
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "One-time global setup of Liza contracts and skills",
	Long: `Write Liza contracts and skills to ~/.liza/ for global access.

This is a one-time setup step that populates the global config directory.
Contracts are written flat (e.g., ~/.liza/CORE.md) and skills are written
to ~/.liza/skills/.

After running setup, use 'liza init' in each project to create the
project-local blackboard and symlinks.

Use --force to overwrite an existing global config.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir, err := paths.GlobalLizaDir()
		if err != nil {
			return err
		}
		force, _ := cmd.Flags().GetBool("force")
		return commands.SetupCommand(targetDir, force, os.Stdin)
	},
}

var initCmd = &cobra.Command{
	Use:   "init [description]",
	Short: "Initialize a new Liza workspace",
	Long: `Initialize a new Liza workspace by creating .liza directory structure,
generating initial state.yaml, and setting up the integration branch.

The description argument is required and describes the goal.
The spec file (default: specs/vision.md) must exist before initialization.

Use --config to provide a pipeline YAML file (defaults to ~/.liza/pipeline.yaml).
The config is validated and frozen into .liza/pipeline.yaml. Use --entry-point to
specify which entry-point to use (must be defined in the config).

Use --post-worktree-cmd to specify a shell command that runs after every worktree
creation (e.g. 'make setup', 'npm install'). This ensures worktrees are
build/test-ready without hardcoding project-specific tooling into Liza.
Existing workspaces can add post_worktree_cmd to state.yaml's config section.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		description := args[0]
		specRef, _ := cmd.Flags().GetString("spec")
		configPath, _ := cmd.Flags().GetString("config")
		entryPoint, _ := cmd.Flags().GetString("entry-point")
		postCreateCmd, _ := cmd.Flags().GetString("post-worktree-cmd")
		return commands.InitCommandWithConfig(commands.InitParams{
			Description:     description,
			SpecRef:         specRef,
			ConfigPath:      configPath,
			EntryPoint:      entryPoint,
			PostWorktreeCmd: postCreateCmd,
			Stdin:           os.Stdin,
		})
	},
}

var validateCmd = &cobra.Command{
	Use:   "validate [state-file]",
	Short: "Validate state.yaml against schema rules",
	Long: `Validate the state.yaml file against all 43+ validation rules including:
- Required fields and task state invariants
- Dependency validation (existence, circularity, MERGED deps for executing tasks)
- Agent validation (WORKING must have current_task)
- Lease expiry checking with grace periods
- Spec file reference validation
Returns detailed error messages if validation fails.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		statePath := ""
		if len(args) > 0 {
			statePath = args[0]
		} else {
			statePath = filepath.Join(paths.LizaDirName, paths.StateFileName)
		}

		skipSpecCheck, _ := cmd.Flags().GetBool("skip-spec-check")
		err := commands.ValidateCommand(statePath, skipSpecCheck)
		if err != nil {
			return err
		}
		fmt.Println("VALID")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(validateCmd)

	// Setup command flags
	setupCmd.Flags().Bool("force", false, "overwrite existing global config")

	// Init command flags
	initCmd.Flags().String("spec", "specs/vision.md", "path to goal spec file")
	initCmd.Flags().String("config", defaultPipelineConfigPath(), "path to pipeline YAML config file")
	initCmd.Flags().String("entry-point", "", `entry-point name: "general-objective" or "detailed-spec" in default pipeline (default: auto-classified by orchestrator)`)
	initCmd.Flags().String("post-worktree-cmd", "", "shell command to run after worktree creation (e.g. 'make setup')")

	// Validate command flags
	validateCmd.Flags().Bool("skip-spec-check", false, "skip spec file existence check")
}
