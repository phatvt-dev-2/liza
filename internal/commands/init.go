// Package commands implements Liza CLI commands.
package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/embedded"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
)

// InitCommand initializes a new Liza workspace.
// It creates the .liza directory structure, generates initial state.yaml,
// validates the spec file exists, and creates the integration branch.
func InitCommand(description string, specRef string) error {
	// Get project paths
	lizaPaths, err := paths.LizaPathsFromGit()
	if err != nil {
		return fmt.Errorf("failed to setup paths: %w", err)
	}

	// Validate .liza doesn't already exist
	if _, err := os.Stat(lizaPaths.LizaDir()); !os.IsNotExist(err) {
		return fmt.Errorf(".liza already exists at %s. Remove or use existing.", lizaPaths.LizaDir())
	}

	// Validate spec file exists
	specPath := filepath.Join(lizaPaths.ProjectRoot(), specRef)
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		return fmt.Errorf("spec file does not exist: %s\nCreate spec document first. See templates/vision-template.md", specRef)
	}

	// Create directory structure
	if err := os.MkdirAll(lizaPaths.LizaDir(), 0755); err != nil {
		return fmt.Errorf("failed to create .liza directory: %w", err)
	}

	archiveDir := lizaPaths.ArchiveDir()
	if err := os.Mkdir(archiveDir, 0755); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Write embedded files (contracts, skills, specs)
	if err := embedded.WriteAllFiles(lizaPaths.ProjectRoot()); err != nil {
		// Clean up on failure
		os.RemoveAll(lizaPaths.LizaDir())
		return fmt.Errorf("failed to write embedded files: %w", err)
	}

	// Write/merge Claude Code settings to .claude/
	// This is non-fatal - if it fails, just warn
	// Note: This may prompt user for input if settings file exists
	if err := embedded.WriteClaudeSettings(lizaPaths.ProjectRoot()); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write claude-settings.json: %v\n", err)
	}

	// Write/merge MCP server configuration to .mcp.json
	// This is non-fatal - if it fails, just warn
	// Note: This may prompt user for input if settings file exists
	if err := embedded.WriteMCPSettings(lizaPaths.ProjectRoot()); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write .mcp.json: %v\n", err)
	}

	// Generate IDs and timestamps
	timestamp := time.Now().UTC()
	goalID := fmt.Sprintf("goal-%d", timestamp.Unix())

	// Create initial state
	state := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          goalID,
			Description: description,
			SpecRef:     specRef,
			Created:     timestamp,
			Status:      models.GoalStatusInProgress,
			AlignmentHistory: []models.AlignmentHistory{
				{
					Timestamp: timestamp,
					Event:     "initialization",
					Summary:   "Initial goal. No tasks defined yet.",
				},
			},
		},
		Tasks:       []models.Task{},
		Agents:      make(map[string]models.Agent),
		Discovered:  []models.Discovery{},
		Handoff:     make(map[string]models.HandoffNote),
		HumanNotes:  []models.HumanNote{},
		SpecChanges: []models.SpecChange{},
		Anomalies:   []models.Anomaly{},
		Sprint: models.Sprint{
			ID:      "sprint-1",
			GoalRef: goalID,
			Scope: models.SprintScope{
				Planned: []string{},
				Stretch: []string{},
			},
			Timeline: models.SprintTimeline{
				Started:      timestamp,
				Deadline:     time.Time{}, // zero value for null
				CheckpointAt: nil,
				Ended:        nil,
			},
			Status: models.SprintStatusInProgress,
			Metrics: models.SprintMetrics{
				TasksDone:         0,
				TasksInProgress:   0,
				TasksBlocked:      0,
				IterationsTotal:   0,
				ReviewCyclesTotal: 0,
			},
			Retrospective: nil,
		},
		CircuitBreaker: models.CircuitBreaker{
			LastCheck:      time.Time{}, // zero value for null
			Status:         "OK",
			CurrentTrigger: nil,
			History:        []models.CircuitBreakerHistory{},
		},
		Config: models.Config{
			MaxCoderIterations: 10,
			MaxReviewCycles:    5,
			HeartbeatInterval:  60,
			LeaseDuration:      1800,
			CoderPollInterval:  30,
			CoderMaxWait:       1800,
			IntegrationBranch:  "integration",
			EscalationWebhook:  nil,
			Mode:               models.SystemModeRunning,
		},
	}

	// Write state file
	bb := db.New(lizaPaths.StatePath())
	if err := bb.Write(state); err != nil {
		// Clean up on failure
		os.RemoveAll(lizaPaths.LizaDir())
		return fmt.Errorf("failed to write state file: %w", err)
	}

	// Create log file
	// Note: Using simple file write for log since it's not managed by blackboard
	logPath := lizaPaths.LogPath()
	logContent := fmt.Sprintf(`- timestamp: %s
  agent: system
  action: initialized
  detail: %s
`, timestamp.Format(time.RFC3339), description)

	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		os.RemoveAll(lizaPaths.LizaDir())
		return fmt.Errorf("failed to write log file: %w", err)
	}

	// Create supporting files
	alertsPath := lizaPaths.AlertsLogPath()
	if err := os.WriteFile(alertsPath, []byte{}, 0644); err != nil {
		os.RemoveAll(lizaPaths.LizaDir())
		return fmt.Errorf("failed to create alerts.log: %w", err)
	}

	// Create lock file
	if err := os.WriteFile(lizaPaths.LockPath(), []byte{}, 0644); err != nil {
		os.RemoveAll(lizaPaths.LizaDir())
		return fmt.Errorf("failed to create lock file: %w", err)
	}

	// Create integration branch if it doesn't exist
	if err := createIntegrationBranch(); err != nil {
		// Don't fail the entire init if branch creation fails
		// Just log the error - this is what bash version does
		fmt.Fprintf(os.Stderr, "Warning: failed to create integration branch: %v\n", err)
	}

	fmt.Printf("Liza initialized at %s\n", lizaPaths.LizaDir())
	fmt.Println("Integration branch: integration")

	return nil
}

// createIntegrationBranch creates the integration branch if it doesn't exist
func createIntegrationBranch() error {
	// Check if integration branch exists
	cmd := exec.Command("git", "rev-parse", "--verify", "integration")
	if err := cmd.Run(); err == nil {
		// Branch already exists
		return nil
	}

	// Create integration branch from HEAD
	cmd = exec.Command("git", "branch", "integration", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git branch failed: %w: %s", err, string(output))
	}

	return nil
}
