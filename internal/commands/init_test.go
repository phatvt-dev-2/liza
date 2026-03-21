package commands

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// setupGlobalLiza delegates to testhelpers.SetupGlobalLiza.
func setupGlobalLiza(t *testing.T) string {
	return testhelpers.SetupGlobalLiza(t)
}

func TestInitCommand(t *testing.T) {
	tests := []struct {
		name        string
		description string
		specRef     string
		setup       func(t *testing.T, tmpDir string)
		skipGlobal  bool // if true, don't set up global liza
		wantErr     bool
		errContains string
	}{
		{
			name:        "successful initialization",
			description: "Test goal",
			specRef:     "specs/vision.md",
			setup: func(t *testing.T, tmpDir string) {
				testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n")
			},
			wantErr: false,
		},
		{
			name:        ".liza already exists",
			description: "Test goal",
			specRef:     "specs/vision.md",
			setup: func(t *testing.T, tmpDir string) {
				testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n")
				// Create .liza directory
				lizaDir := paths.New(tmpDir).LizaDir()
				if err := os.Mkdir(lizaDir, 0755); err != nil {
					t.Fatal(err)
				}
			},
			wantErr:     true,
			errContains: "already exists",
		},
		{
			name:        "spec file does not exist",
			description: "Test goal",
			specRef:     "specs/vision.md",
			setup:       func(t *testing.T, tmpDir string) {}, // No spec file
			wantErr:     true,
			errContains: "spec file does not exist",
		},
		{
			name:        "global config not found",
			description: "Test goal",
			specRef:     "specs/vision.md",
			skipGlobal:  true,
			setup: func(t *testing.T, tmpDir string) {
				testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n")
			},
			wantErr:     true,
			errContains: "Run 'liza setup' first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary git repo
			tmpDir := setupGitRepo(t)
			defer os.RemoveAll(tmpDir)

			// Set up global liza unless test skips it
			if !tt.skipGlobal {
				setupGlobalLiza(t)
			} else {
				// Point HOME to an empty dir so global check fails
				emptyHome := t.TempDir()
				t.Setenv("HOME", emptyHome)
			}

			// Change to temp directory
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			defer os.Chdir(originalDir)
			if err := os.Chdir(tmpDir); err != nil {
				t.Fatal(err)
			}

			// Run setup
			tt.setup(t, tmpDir)

			// Run init command
			err = InitCommand(tt.description, tt.specRef, nil)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("InitCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				testhelpers.AssertErrorContains(t, err, tt.errContains)
				return
			}

			// If no error expected, verify the initialization
			if !tt.wantErr {
				verifyInitialization(t, tmpDir, tt.description, tt.specRef)
			}
		})
	}
}

func TestInitCommandDirectoryStructure(t *testing.T) {
	tmpDir := setupGitRepo(t)
	defer os.RemoveAll(tmpDir)

	setupGlobalLiza(t)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n")

	// Run init
	if err := InitCommand("Test goal", "specs/vision.md", nil); err != nil {
		t.Fatalf("InitCommand() error = %v", err)
	}

	// Verify directory structure
	lizaDir := paths.New(tmpDir).LizaDir()
	expectedDirs := []string{
		lizaDir,
		filepath.Join(lizaDir, "archive"),
	}

	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("directory %s was not created", dir)
		}
	}

	// Verify files
	expectedFiles := []string{
		filepath.Join(lizaDir, "state.yaml"),
		filepath.Join(lizaDir, "log.yaml"),
		filepath.Join(lizaDir, "alerts.log"),
		filepath.Join(lizaDir, "state.yaml.lock"),
	}

	for _, file := range expectedFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			t.Errorf("file %s was not created", file)
		}
	}

	// Verify GUARDRAILS.md template was created at project root
	guardrailsPath := filepath.Join(tmpDir, "GUARDRAILS.md")
	if _, err := os.Stat(guardrailsPath); os.IsNotExist(err) {
		t.Error("GUARDRAILS.md was not created at project root")
	}
}

func TestInitCommandIntegrationBranch(t *testing.T) {
	tmpDir := setupGitRepo(t)
	defer os.RemoveAll(tmpDir)

	setupGlobalLiza(t)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n")

	// Verify integration branch doesn't exist
	cmd := exec.Command("git", "rev-parse", "--verify", "integration")
	if err := cmd.Run(); err == nil {
		t.Fatal("integration branch already exists before init")
	}

	// Run init
	if err := InitCommand("Test goal", "specs/vision.md", nil); err != nil {
		t.Fatalf("InitCommand() error = %v", err)
	}

	// Verify integration branch was created
	cmd = exec.Command("git", "rev-parse", "--verify", "integration")
	if err := cmd.Run(); err != nil {
		t.Error("integration branch was not created")
	}
}

// Helper functions

func setupGitRepo(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Resolve symlinks so paths match os.Getwd() on macOS
	// (macOS: /var -> /private/var, but t.TempDir() returns /var/...)
	tmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Initialize git repo with "main" as default branch
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Create initial commit
	readmeFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmeFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("git", "add", "README.md")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	return tmpDir
}

func verifyInitialization(t *testing.T, tmpDir, description, specRef string) {
	t.Helper()

	lizaDir := paths.New(tmpDir).LizaDir()
	statePath := filepath.Join(lizaDir, "state.yaml")

	// Read state
	bb := db.New(statePath)
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}

	// Verify version
	if state.Version != 1 {
		t.Errorf("state.Version = %d, want 1", state.Version)
	}

	// Verify goal
	if state.Goal.Description != description {
		t.Errorf("state.Goal.Description = %q, want %q", state.Goal.Description, description)
	}
	wantSpecRef := filepath.Join(tmpDir, specRef)
	if state.Goal.SpecRef != wantSpecRef {
		t.Errorf("state.Goal.SpecRef = %q, want %q", state.Goal.SpecRef, wantSpecRef)
	}
	if state.Goal.Status != models.GoalStatusInProgress {
		t.Errorf("state.Goal.Status = %v, want %v", state.Goal.Status, models.GoalStatusInProgress)
	}
	if state.Goal.ID == "" {
		t.Error("state.Goal.ID is empty")
	}
	if state.Goal.Created.IsZero() {
		t.Error("state.Goal.Created is zero")
	}
	if len(state.Goal.AlignmentHistory) == 0 {
		t.Error("state.Goal.AlignmentHistory is empty")
	}

	// Verify tasks is empty
	if len(state.Tasks) != 0 {
		t.Errorf("state.Tasks length = %d, want 0", len(state.Tasks))
	}

	// Verify agents is empty
	if len(state.Agents) != 0 {
		t.Errorf("state.Agents length = %d, want 0", len(state.Agents))
	}

	// Verify sprint
	if state.Sprint.ID != "sprint-1" {
		t.Errorf("state.Sprint.ID = %q, want %q", state.Sprint.ID, "sprint-1")
	}
	if state.Sprint.GoalRef != state.Goal.ID {
		t.Errorf("state.Sprint.GoalRef = %q, want %q", state.Sprint.GoalRef, state.Goal.ID)
	}
	if state.Sprint.Status != models.SprintStatusInProgress {
		t.Errorf("state.Sprint.Status = %v, want %v", state.Sprint.Status, models.SprintStatusInProgress)
	}

	// Verify config
	if state.Config.MaxCoderIterations != 10 {
		t.Errorf("state.Config.MaxCoderIterations = %d, want 10", state.Config.MaxCoderIterations)
	}
	if state.Config.MaxReviewCycles != 5 {
		t.Errorf("state.Config.MaxReviewCycles = %d, want 5", state.Config.MaxReviewCycles)
	}
	if state.Config.IntegrationBranch != "integration" {
		t.Errorf("state.Config.IntegrationBranch = %q, want %q", state.Config.IntegrationBranch, "integration")
	}
	if state.Config.Mode != models.SystemModeRunning {
		t.Errorf("state.Config.Mode = %q, want %q", state.Config.Mode, models.SystemModeRunning)
	}
	if state.Config.CoderMaxWait != models.DefaultCoderMaxWait {
		t.Errorf("state.Config.CoderMaxWait = %d, want %d", state.Config.CoderMaxWait, models.DefaultCoderMaxWait)
	}
	if state.Config.OrchestratorMaxWait != models.DefaultOrchestratorMaxWait {
		t.Errorf("state.Config.OrchestratorMaxWait = %d, want %d", state.Config.OrchestratorMaxWait, models.DefaultOrchestratorMaxWait)
	}
	if state.Config.ReviewerMaxWait != models.DefaultReviewerMaxWait {
		t.Errorf("state.Config.ReviewerMaxWait = %d, want %d", state.Config.ReviewerMaxWait, models.DefaultReviewerMaxWait)
	}

	// Verify circuit breaker
	if state.CircuitBreaker.Status != "OK" {
		t.Errorf("state.CircuitBreaker.Status = %q, want %q", state.CircuitBreaker.Status, "OK")
	}

	// Verify timestamp is recent (within 5 seconds)
	now := time.Now().UTC()
	diff := now.Sub(state.Goal.Created)
	if diff < 0 || diff > 5*time.Second {
		t.Errorf("state.Goal.Created timestamp difference = %v, want < 5s", diff)
	}
}

func TestInitCommand_CreatesContractSymlinks(t *testing.T) {
	// Create temporary git repo
	gitDir := setupGitRepo(t)
	defer os.RemoveAll(gitDir)

	fakeHome := setupGlobalLiza(t)

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)
	if err := os.Chdir(gitDir); err != nil {
		t.Fatal(err)
	}

	// Setup
	testhelpers.CreateSpecFile(t, gitDir, "vision.md", "# Vision\n")

	// Run init
	err = InitCommand("Test goal", "specs/vision.md", nil)
	if err != nil {
		t.Fatalf("InitCommand failed: %v", err)
	}

	// Verify contract symlinks point to absolute global path
	globalDir := filepath.Join(fakeHome, ".liza")
	expectedTarget := filepath.Join(globalDir, "CORE.md")
	for _, name := range []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md"} {
		linkPath := filepath.Join(gitDir, name)
		target, err := os.Readlink(linkPath)
		if err != nil {
			t.Errorf("Symlink %s not created: %v", name, err)
			continue
		}
		if target != expectedTarget {
			t.Errorf("Symlink %s target = %q, want %q", name, target, expectedTarget)
		}
	}
}

func TestInitCommand_SkipsCorrectSymlinks(t *testing.T) {
	gitDir := setupGitRepo(t)
	defer os.RemoveAll(gitDir)

	fakeHome := setupGlobalLiza(t)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)
	if err := os.Chdir(gitDir); err != nil {
		t.Fatal(err)
	}

	testhelpers.CreateSpecFile(t, gitDir, "vision.md", "# Vision\n")

	// Pre-create CLAUDE.md as the correct symlink (absolute to global)
	globalDir := filepath.Join(fakeHome, ".liza")
	correctTarget := filepath.Join(globalDir, "CORE.md")
	claudePath := filepath.Join(gitDir, "CLAUDE.md")
	if err := os.Symlink(correctTarget, claudePath); err != nil {
		t.Fatal(err)
	}

	// Run init
	if err := InitCommand("Test goal", "specs/vision.md", nil); err != nil {
		t.Fatalf("InitCommand failed: %v", err)
	}

	// CLAUDE.md should still point to the same target (untouched)
	target, err := os.Readlink(claudePath)
	if err != nil {
		t.Fatalf("CLAUDE.md is no longer a symlink: %v", err)
	}
	if target != correctTarget {
		t.Errorf("CLAUDE.md target changed; got %q, want %q", target, correctTarget)
	}
}

func TestInitCommand_DoesNotOverwriteWithoutConsent(t *testing.T) {
	gitDir := setupGitRepo(t)
	defer os.RemoveAll(gitDir)

	setupGlobalLiza(t)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)
	if err := os.Chdir(gitDir); err != nil {
		t.Fatal(err)
	}

	testhelpers.CreateSpecFile(t, gitDir, "vision.md", "# Vision\n")

	// Pre-create CLAUDE.md as a regular file
	existingContent := "# Custom contract\n"
	claudePath := filepath.Join(gitDir, "CLAUDE.md")
	if err := os.WriteFile(claudePath, []byte(existingContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Run init — stdin is not interactive in tests, so the prompt
	// will fail to read and the file should be left untouched.
	if err := InitCommand("Test goal", "specs/vision.md", nil); err != nil {
		t.Fatalf("InitCommand failed: %v", err)
	}

	// CLAUDE.md should be untouched (no consent given)
	content, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("Failed to read CLAUDE.md: %v", err)
	}
	if string(content) != existingContent {
		t.Errorf("CLAUDE.md was modified without consent; got %q, want %q", string(content), existingContent)
	}

	// AGENTS.md and GEMINI.md should still be created as symlinks
	for _, name := range []string{"AGENTS.md", "GEMINI.md"} {
		linkPath := filepath.Join(gitDir, name)
		if _, err := os.Readlink(linkPath); err != nil {
			t.Errorf("Symlink %s not created: %v", name, err)
		}
	}
}

func TestInitCommand_WritesClaudeSettings(t *testing.T) {
	// Create temporary git repo
	gitDir := setupGitRepo(t)
	defer os.RemoveAll(gitDir)

	setupGlobalLiza(t)

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)
	if err := os.Chdir(gitDir); err != nil {
		t.Fatal(err)
	}

	// Setup
	testhelpers.CreateSpecFile(t, gitDir, "vision.md", "# Vision\n")

	// Run init
	err = InitCommand("Test goal", "specs/vision.md", nil)
	if err != nil {
		t.Fatalf("InitCommand failed: %v", err)
	}

	// Verify .claude directory was created
	claudeDir := filepath.Join(gitDir, ".claude")
	if _, err := os.Stat(claudeDir); os.IsNotExist(err) {
		t.Errorf(".claude directory not created")
	}

	// Verify settings.json was created
	settingsPath := filepath.Join(claudeDir, "settings.json")
	info, err := os.Stat(settingsPath)
	if os.IsNotExist(err) {
		t.Fatalf("settings.json not created")
	}

	// Verify file permissions
	if info.Mode().Perm() != 0644 {
		t.Errorf("settings.json has wrong permissions: got %o, want 0644", info.Mode().Perm())
	}

	// Read and parse JSON
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(content, &settings); err != nil {
		t.Fatalf("settings.json is not valid JSON: %v", err)
	}

	// Verify permissions structure exists
	perms, ok := settings["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("permissions field missing or invalid type")
	}

	// Verify defaultMode
	if _, ok := perms["defaultMode"]; !ok {
		t.Errorf("permissions.defaultMode field missing")
	}

	// Verify allow array exists
	allow, ok := perms["allow"].([]any)
	if !ok {
		t.Fatalf("permissions.allow field missing or not an array")
	}

	// Verify allow array has some permissions
	if len(allow) == 0 {
		t.Errorf("permissions.allow array is empty")
	}

	// Verify key liza MCP tools are in allow array (explicit tool format)
	foundLizaMCP := false
	for _, perm := range allow {
		permStr := perm.(string)
		// Check for explicit tool format: mcp__liza__liza_add_tasks
		if strings.HasPrefix(permStr, "mcp__liza__") {
			foundLizaMCP = true
			break
		}
	}
	if !foundLizaMCP {
		t.Errorf("Expected liza MCP tools in allow array (e.g., mcp__liza__liza_add_tasks)")
	}

	// Verify .claude/hooks/enforce-init.sh was deployed
	hookPath := filepath.Join(claudeDir, "hooks", "enforce-init.sh")
	hookInfo, hookErr := os.Stat(hookPath)
	if os.IsNotExist(hookErr) {
		t.Error(".claude/hooks/enforce-init.sh not created during workspace init")
	} else if hookErr == nil && hookInfo.Mode()&0111 == 0 {
		t.Errorf("enforce-init.sh should be executable, got %o", hookInfo.Mode())
	}
}

// validPipelineYAML is a minimal valid pipeline config for testing.
const validPipelineYAML = `pipeline:
  roles:
    code-planner:
      type: doer
      display-name: "Code Planner"
    code-plan-reviewer:
      type: reviewer
      display-name: "Code Plan Reviewer"
    coder:
      type: doer
      display-name: "Coder"
    code-reviewer:
      type: reviewer
      display-name: "Code Reviewer"

  role-pairs:
    code-planning-pair:
      doer: code-planner
      reviewer: code-plan-reviewer
      states:
        initial: DRAFT_CODING_PLAN
        executing: CODE_PLANNING
        submitted: CODING_PLAN_TO_REVIEW
        reviewing: REVIEWING_CODING_PLAN
        approved: CODING_PLAN_APPROVED
        rejected: CODING_PLAN_REJECTED

    coding-pair:
      doer: coder
      reviewer: code-reviewer
      states:
        initial: DRAFT_CODE
        executing: IMPLEMENTING_CODE
        submitted: CODE_READY_FOR_REVIEW
        reviewing: REVIEWING_CODE
        approved: CODE_APPROVED
        rejected: CODE_REJECTED

  sub-pipelines:
    coding-subpipeline:
      steps:
        - code-planning-pair
        - coding-pair
      transitions:
        - name: code-plan-to-coding
          from: code-planning-pair.approved
          to: coding-pair.initial
          trigger: manual
          cardinality: per-subtask

  entry-points:
    detailed-spec: coding-subpipeline.code-planning-pair
`

func writePipelineConfig(t *testing.T, dir, content string) string {
	t.Helper()
	configPath := filepath.Join(dir, "pipeline.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return configPath
}

func TestInitCommandWithConfig_FreezesPipeline(t *testing.T) {
	tmpDir := setupGitRepo(t)
	defer os.RemoveAll(tmpDir)
	setupGlobalLiza(t)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n")
	configPath := writePipelineConfig(t, tmpDir, validPipelineYAML)

	err = InitCommandWithConfig(InitParams{
		Description: "Pipeline goal",
		SpecRef:     "specs/vision.md",
		ConfigPath:  configPath,
	})
	if err != nil {
		t.Fatalf("InitCommandWithConfig() error = %v", err)
	}

	// Verify .liza/pipeline.yaml exists and is identical to input
	frozenPath := filepath.Join(tmpDir, ".liza", "pipeline.yaml")
	frozen, err := os.ReadFile(frozenPath)
	if err != nil {
		t.Fatalf("Failed to read frozen pipeline.yaml: %v", err)
	}
	if string(frozen) != validPipelineYAML {
		t.Errorf("Frozen pipeline.yaml differs from input.\nGot:\n%s\nWant:\n%s", string(frozen), validPipelineYAML)
	}

	// Verify state.yaml has pipeline_version: 2
	bb := db.New(filepath.Join(tmpDir, ".liza", "state.yaml"))
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if state.PipelineVersion != 2 {
		t.Errorf("state.PipelineVersion = %d, want 2", state.PipelineVersion)
	}
}

func TestInitCommandWithConfig_EntryPoint(t *testing.T) {
	tmpDir := setupGitRepo(t)
	defer os.RemoveAll(tmpDir)
	setupGlobalLiza(t)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n")
	configPath := writePipelineConfig(t, tmpDir, validPipelineYAML)

	err = InitCommandWithConfig(InitParams{
		Description: "Pipeline goal",
		SpecRef:     "specs/vision.md",
		ConfigPath:  configPath,
		EntryPoint:  "detailed-spec",
	})
	if err != nil {
		t.Fatalf("InitCommandWithConfig() error = %v", err)
	}

	// Verify goal.entry_point is set
	bb := db.New(filepath.Join(tmpDir, ".liza", "state.yaml"))
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if state.Goal.EntryPoint != "detailed-spec" {
		t.Errorf("state.Goal.EntryPoint = %q, want %q", state.Goal.EntryPoint, "detailed-spec")
	}
}

func TestInitCommandWithConfig_NoConfigAutoFreezes(t *testing.T) {
	tmpDir := setupGitRepo(t)
	defer os.RemoveAll(tmpDir)
	setupGlobalLiza(t)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n")

	// Init without --config auto-freezes embedded pipeline
	err = InitCommand("Legacy goal", "specs/vision.md", nil)
	if err != nil {
		t.Fatalf("InitCommand() error = %v", err)
	}

	// Verify pipeline.yaml is auto-frozen from embedded config
	frozenPath := filepath.Join(tmpDir, ".liza", "pipeline.yaml")
	if _, err := os.Stat(frozenPath); os.IsNotExist(err) {
		t.Errorf("pipeline.yaml should be auto-frozen from embedded config")
	}

	// Verify pipeline_version is set
	bb := db.New(filepath.Join(tmpDir, ".liza", "state.yaml"))
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if state.PipelineVersion != 2 {
		t.Errorf("state.PipelineVersion = %d, want 2", state.PipelineVersion)
	}

	// Verify no entry_point (not specified)
	if state.Goal.EntryPoint != "" {
		t.Errorf("state.Goal.EntryPoint = %q, want empty", state.Goal.EntryPoint)
	}
}

func TestInitCommandWithConfig_InvalidConfig(t *testing.T) {
	tmpDir := setupGitRepo(t)
	defer os.RemoveAll(tmpDir)
	setupGlobalLiza(t)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n")

	// Write invalid pipeline config (missing required fields)
	invalidYAML := `pipeline:
  role-pairs: {}
`
	configPath := writePipelineConfig(t, tmpDir, invalidYAML)

	err = InitCommandWithConfig(InitParams{
		Description: "Bad config goal",
		SpecRef:     "specs/vision.md",
		ConfigPath:  configPath,
	})
	if err == nil {
		t.Fatal("Expected error for invalid config, got nil")
	}
	testhelpers.AssertErrorContains(t, err, "invalid pipeline config")

	// Verify .liza was not created (early validation)
	lizaDir := filepath.Join(tmpDir, ".liza")
	if _, statErr := os.Stat(lizaDir); !os.IsNotExist(statErr) {
		t.Errorf(".liza directory should not exist after config validation failure")
	}
}

func TestInitCommandWithConfig_NonexistentEntryPoint(t *testing.T) {
	tmpDir := setupGitRepo(t)
	defer os.RemoveAll(tmpDir)
	setupGlobalLiza(t)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n")
	configPath := writePipelineConfig(t, tmpDir, validPipelineYAML)

	err = InitCommandWithConfig(InitParams{
		Description: "Goal",
		SpecRef:     "specs/vision.md",
		ConfigPath:  configPath,
		EntryPoint:  "nonexistent",
	})
	if err == nil {
		t.Fatal("Expected error for nonexistent entry-point, got nil")
	}
	testhelpers.AssertErrorContains(t, err, "entry-point")
	testhelpers.AssertErrorContains(t, err, "not found")
}

func TestInitCommandWithConfig_PostWorktreeCmd(t *testing.T) {
	tmpDir := setupGitRepo(t)
	defer os.RemoveAll(tmpDir)
	setupGlobalLiza(t)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n")

	err = InitCommandWithConfig(InitParams{
		Description:     "Goal with post-worktree-cmd",
		SpecRef:         "specs/vision.md",
		PostWorktreeCmd: "make sync-embedded",
	})
	if err != nil {
		t.Fatalf("InitCommandWithConfig() error = %v", err)
	}

	// Verify post_worktree_cmd is set in state
	bb := db.New(filepath.Join(tmpDir, ".liza", "state.yaml"))
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if state.Config.PostWorktreeCmd == nil {
		t.Fatal("state.Config.PostWorktreeCmd is nil, want non-nil")
	}
	if *state.Config.PostWorktreeCmd != "make sync-embedded" {
		t.Errorf("state.Config.PostWorktreeCmd = %q, want %q", *state.Config.PostWorktreeCmd, "make sync-embedded")
	}
}

func TestInitCommandWithConfig_PostWorktreeCmdOmittedWhenEmpty(t *testing.T) {
	tmpDir := setupGitRepo(t)
	defer os.RemoveAll(tmpDir)
	setupGlobalLiza(t)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n")

	err = InitCommandWithConfig(InitParams{
		Description: "Goal without post-worktree-cmd",
		SpecRef:     "specs/vision.md",
	})
	if err != nil {
		t.Fatalf("InitCommandWithConfig() error = %v", err)
	}

	// Verify post_worktree_cmd is nil in state
	bb := db.New(filepath.Join(tmpDir, ".liza", "state.yaml"))
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if state.Config.PostWorktreeCmd != nil {
		t.Errorf("state.Config.PostWorktreeCmd = %q, want nil", *state.Config.PostWorktreeCmd)
	}
}

// --- InitPairingCommand tests ---

func TestInitPairingCommand_Claude(t *testing.T) {
	gitDir := setupGitRepo(t)
	defer os.RemoveAll(gitDir)
	fakeHome := setupGlobalLiza(t)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(gitDir)

	err := InitPairingCommand(InitPairingParams{
		Agents: []string{"claude"},
	})
	if err != nil {
		t.Fatalf("InitPairingCommand failed: %v", err)
	}

	// CLAUDE.md should be a symlink to ~/.liza/CORE.md
	target, err := os.Readlink(filepath.Join(gitDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("CLAUDE.md not a symlink: %v", err)
	}
	expected := filepath.Join(fakeHome, ".liza", "CORE.md")
	if target != expected {
		t.Errorf("CLAUDE.md → %q, want %q", target, expected)
	}

	// .liza/ directory should NOT exist
	if _, err := os.Stat(filepath.Join(gitDir, ".liza")); !os.IsNotExist(err) {
		t.Error(".liza/ directory should not be created in pairing mode")
	}

	// .claude/settings.json should be written
	settingsPath := filepath.Join(gitDir, ".claude", "settings.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Error(".claude/settings.json should be created for --claude pairing")
	}

	// .claude/hooks/enforce-init.sh should be deployed
	hookPath := filepath.Join(gitDir, ".claude", "hooks", "enforce-init.sh")
	hookInfo, err := os.Stat(hookPath)
	if os.IsNotExist(err) {
		t.Error(".claude/hooks/enforce-init.sh should be created for --claude pairing")
	} else if err == nil && hookInfo.Mode()&0111 == 0 {
		t.Errorf("enforce-init.sh should be executable, got %o", hookInfo.Mode())
	}

	// AGENTS.md and GEMINI.md should NOT exist (only --claude)
	for _, name := range []string{"AGENTS.md", "GEMINI.md"} {
		if _, err := os.Stat(filepath.Join(gitDir, name)); !os.IsNotExist(err) {
			t.Errorf("%s should not exist when only --claude is specified", name)
		}
	}
}

func TestInitPairingCommand_MultipleAgents(t *testing.T) {
	gitDir := setupGitRepo(t)
	defer os.RemoveAll(gitDir)
	fakeHome := setupGlobalLiza(t)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(gitDir)

	err := InitPairingCommand(InitPairingParams{
		Agents: []string{"claude", "codex", "gemini"},
	})
	if err != nil {
		t.Fatalf("InitPairingCommand failed: %v", err)
	}

	expected := filepath.Join(fakeHome, ".liza", "CORE.md")
	for _, tc := range []struct {
		agent string
		file  string
	}{
		{"claude", "CLAUDE.md"},
		{"codex", "AGENTS.md"},
		{"gemini", "GEMINI.md"},
	} {
		target, err := os.Readlink(filepath.Join(gitDir, tc.file))
		if err != nil {
			t.Errorf("%s (%s): not a symlink: %v", tc.file, tc.agent, err)
			continue
		}
		if target != expected {
			t.Errorf("%s → %q, want %q", tc.file, target, expected)
		}
	}
}

func TestInitPairingCommand_Idempotent(t *testing.T) {
	gitDir := setupGitRepo(t)
	defer os.RemoveAll(gitDir)
	fakeHome := setupGlobalLiza(t)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(gitDir)

	// Run twice
	for i := 0; i < 2; i++ {
		err := InitPairingCommand(InitPairingParams{
			Agents: []string{"claude"},
		})
		if err != nil {
			t.Fatalf("run %d: InitPairingCommand failed: %v", i+1, err)
		}
	}

	target, err := os.Readlink(filepath.Join(gitDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("CLAUDE.md not a symlink: %v", err)
	}
	expected := filepath.Join(fakeHome, ".liza", "CORE.md")
	if target != expected {
		t.Errorf("CLAUDE.md → %q, want %q", target, expected)
	}
}

func TestInitPairingCommand_Mistral(t *testing.T) {
	fakeHome := setupGlobalLiza(t)

	err := InitPairingCommand(InitPairingParams{
		Agents: []string{"mistral"},
	})
	if err != nil {
		t.Fatalf("InitPairingCommand failed: %v", err)
	}

	// ~/.vibe/prompts/liza.md should be a symlink to ~/.liza/CORE.md
	linkPath := filepath.Join(fakeHome, ".vibe", "prompts", "liza.md")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("liza.md not a symlink: %v", err)
	}
	expected := filepath.Join(fakeHome, ".liza", "CORE.md")
	if target != expected {
		t.Errorf("liza.md → %q, want %q", target, expected)
	}

	// config.toml should contain system_prompt_id = "liza"
	configPath := filepath.Join(fakeHome, ".vibe", "config.toml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config.toml not created: %v", err)
	}
	if !strings.Contains(string(content), `system_prompt_id = "liza"`) {
		t.Errorf("config.toml missing system_prompt_id = \"liza\", got:\n%s", content)
	}
}

func TestInitPairingCommand_MistralReplacesExistingPromptID(t *testing.T) {
	fakeHome := setupGlobalLiza(t)

	// Pre-create config.toml with system_prompt_id = "cli"
	vibeDir := filepath.Join(fakeHome, ".vibe")
	os.MkdirAll(vibeDir, 0755)
	configPath := filepath.Join(vibeDir, "config.toml")
	os.WriteFile(configPath, []byte("system_prompt_id = \"cli\"\nother_setting = true\n"), 0644)

	// Provide "y\n" for the config.toml overwrite prompt
	err := InitPairingCommand(InitPairingParams{
		Agents: []string{"mistral"},
		Stdin:  strings.NewReader("y\n"),
	})
	if err != nil {
		t.Fatalf("InitPairingCommand failed: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, `system_prompt_id = "liza"`) {
		t.Errorf("system_prompt_id not replaced, got:\n%s", text)
	}
	if strings.Contains(text, `system_prompt_id = "cli"`) {
		t.Errorf("old system_prompt_id = \"cli\" still present, got:\n%s", text)
	}
	if !strings.Contains(text, "other_setting = true") {
		t.Error("other settings were lost during config.toml update")
	}
}

func TestInitPairingCommand_MistralDeclinesOverwrite(t *testing.T) {
	fakeHome := setupGlobalLiza(t)

	// Pre-create config.toml with system_prompt_id = "cli"
	vibeDir := filepath.Join(fakeHome, ".vibe")
	os.MkdirAll(vibeDir, 0755)
	configPath := filepath.Join(vibeDir, "config.toml")
	os.WriteFile(configPath, []byte("system_prompt_id = \"cli\"\n"), 0644)

	// Decline the config.toml overwrite
	err := InitPairingCommand(InitPairingParams{
		Agents: []string{"mistral"},
		Stdin:  strings.NewReader("n\n"),
	})
	if err != nil {
		t.Fatalf("InitPairingCommand failed: %v", err)
	}

	// config.toml should still have "cli"
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `system_prompt_id = "cli"`) {
		t.Error("config.toml was modified despite user declining")
	}
}

// TestInitPairingCommand_ClaudeBothPromptsSharedReader verifies that when both
// CLAUDE.md and .claude/settings.json already exist, both prompts are answered
// from the same stdin stream (no EOF from multiple bufio.NewReader instances).
func TestInitPairingCommand_ClaudeBothPromptsSharedReader(t *testing.T) {
	gitDir := setupGitRepo(t)
	defer os.RemoveAll(gitDir)
	fakeHome := setupGlobalLiza(t)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(gitDir)

	coreFile := filepath.Join(fakeHome, ".liza", "CORE.md")

	// Pre-create CLAUDE.md as a regular file (not symlink) to trigger first prompt
	os.WriteFile(filepath.Join(gitDir, "CLAUDE.md"), []byte("existing"), 0644)

	// Pre-create .claude/settings.json to trigger second prompt (merge confirmation)
	claudeDir := filepath.Join(gitDir, ".claude")
	os.MkdirAll(claudeDir, 0755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{"existing": true}`), 0644)

	// Two "y\n" answers: first for CLAUDE.md overwrite, second for settings merge
	err := InitPairingCommand(InitPairingParams{
		Agents: []string{"claude"},
		Stdin:  strings.NewReader("y\ny\n"),
	})
	if err != nil {
		t.Fatalf("InitPairingCommand failed: %v", err)
	}

	// CLAUDE.md should now be a symlink to ~/.liza/CORE.md
	target, err := os.Readlink(filepath.Join(gitDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("CLAUDE.md should be a symlink after accepting overwrite: %v", err)
	}
	if target != coreFile {
		t.Errorf("CLAUDE.md → %q, want %q", target, coreFile)
	}

	// settings.json should have been merged (contains both existing and liza keys)
	settingsData, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatalf("failed to read settings.json: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(settingsData, &settings); err != nil {
		t.Fatalf("settings.json is not valid JSON: %v", err)
	}
	// Should contain existing user key (preserved during merge)
	if _, ok := settings["existing"]; !ok {
		t.Error("settings.json lost existing user key during merge")
	}
}

func TestInitCommandWithConfig_EntryPointWithoutConfig(t *testing.T) {
	tmpDir := setupGitRepo(t)
	defer os.RemoveAll(tmpDir)
	setupGlobalLiza(t)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	testhelpers.CreateSpecFile(t, tmpDir, "vision.md", "# Vision\n")

	// --entry-point without --config now succeeds because embedded pipeline
	// is auto-loaded and "detailed-spec" exists in the embedded config
	err = InitCommandWithConfig(InitParams{
		Description: "Goal",
		SpecRef:     "specs/vision.md",
		EntryPoint:  "detailed-spec",
	})
	if err != nil {
		t.Fatalf("InitCommandWithConfig() error = %v", err)
	}

	// Verify entry_point is set
	bb := db.New(filepath.Join(tmpDir, ".liza", "state.yaml"))
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	if state.Goal.EntryPoint != "detailed-spec" {
		t.Errorf("state.Goal.EntryPoint = %q, want %q", state.Goal.EntryPoint, "detailed-spec")
	}
}
