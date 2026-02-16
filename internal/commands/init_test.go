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

func TestInitCommand(t *testing.T) {
	tests := []struct {
		name        string
		description string
		specRef     string
		setup       func(t *testing.T, tmpDir string)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary git repo
			tmpDir := setupGitRepo(t)
			defer os.RemoveAll(tmpDir)

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
			err = InitCommand(tt.description, tt.specRef)

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
	if err := InitCommand("Test goal", "specs/vision.md"); err != nil {
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
}

func TestInitCommandIntegrationBranch(t *testing.T) {
	tmpDir := setupGitRepo(t)
	defer os.RemoveAll(tmpDir)

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
	if err := InitCommand("Test goal", "specs/vision.md"); err != nil {
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
	if state.Goal.SpecRef != specRef {
		t.Errorf("state.Goal.SpecRef = %q, want %q", state.Goal.SpecRef, specRef)
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

func TestInitCommand_WritesEmbeddedFiles(t *testing.T) {
	// Create temporary git repo
	gitDir := setupGitRepo(t)
	defer os.RemoveAll(gitDir)

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
	err = InitCommand("Test goal", "specs/vision.md")
	if err != nil {
		t.Fatalf("InitCommand failed: %v", err)
	}

	// Verify contracts directory and files
	contractsDir := filepath.Join(gitDir, ".liza", "contracts")
	if _, err := os.Stat(contractsDir); os.IsNotExist(err) {
		t.Errorf("contracts directory not created: %s", contractsDir)
	}

	contractFiles := []string{"CORE.md", "PAIRING_MODE.md", "MULTI_AGENT_MODE.md", "AGENT_TOOLS.md", "COLLABORATION_CONTINUITY.md"}
	for _, file := range contractFiles {
		filePath := filepath.Join(contractsDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Contract file not created: %s", file)
		}
	}

	// Verify skills directory and files
	skillsDir := filepath.Join(gitDir, ".liza", "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		t.Errorf("skills directory not created: %s", skillsDir)
	}

	skillDirs := []string{"clean-code", "code-review", "debugging",
		"software-architecture-review", "spec-review", "systemic-thinking", "testing"}
	for _, dir := range skillDirs {
		skillFile := filepath.Join(skillsDir, dir, "SKILL.md")
		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			t.Errorf("Skill file not created: %s/SKILL.md", dir)
		}
	}

	// Verify specs directory and files
	specsDir := filepath.Join(gitDir, ".liza", "specs")
	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		t.Errorf("specs directory not created: %s", specsDir)
	}

	// Check root specs files (vision.md is a project file created by users, not embedded)
	readmePath := filepath.Join(specsDir, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		t.Errorf("Spec file not created: README.md")
	}

	// Check subdirectory structure
	specSubdirs := []string{"architecture", "implementation", "protocols"}
	for _, dir := range specSubdirs {
		dirPath := filepath.Join(specsDir, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			t.Errorf("Spec subdirectory not created: %s", dir)
		}
	}
}

func TestInitCommand_FrontmatterPresent(t *testing.T) {
	// Create temporary git repo
	gitDir := setupGitRepo(t)
	defer os.RemoveAll(gitDir)

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
	err = InitCommand("Test goal", "specs/vision.md")
	if err != nil {
		t.Fatalf("InitCommand failed: %v", err)
	}

	// Read a sample embedded file
	coreFile := filepath.Join(gitDir, ".liza", "contracts", "CORE.md")
	content, err := os.ReadFile(coreFile)
	if err != nil {
		t.Fatalf("Failed to read CORE.md: %v", err)
	}

	contentStr := string(content)

	// Verify frontmatter is present
	if !strings.HasPrefix(contentStr, "---\n") {
		t.Errorf("CORE.md missing frontmatter prefix")
	}

	// Verify frontmatter fields
	if !strings.Contains(contentStr, "liza_version:") {
		t.Errorf("CORE.md missing liza_version field")
	}
	if !strings.Contains(contentStr, "liza_git_commit:") {
		t.Errorf("CORE.md missing liza_git_commit field")
	}
	if !strings.Contains(contentStr, "liza_build_date:") {
		t.Errorf("CORE.md missing liza_build_date field")
	}
}

func TestInitCommand_WritesClaudeSettings(t *testing.T) {
	// Create temporary git repo
	gitDir := setupGitRepo(t)
	defer os.RemoveAll(gitDir)

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
	err = InitCommand("Test goal", "specs/vision.md")
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

	// Verify _comment field contains metadata
	comment, ok := settings["_comment"].([]any)
	if !ok {
		t.Fatalf("_comment field is missing or not an array")
	}

	commentStr := ""
	for _, line := range comment {
		commentStr += line.(string) + "\n"
	}

	if !strings.Contains(commentStr, "liza_version:") {
		t.Errorf("Metadata missing liza_version in _comment")
	}
	if !strings.Contains(commentStr, "liza_git_commit:") {
		t.Errorf("Metadata missing liza_git_commit in _comment")
	}
	if !strings.Contains(commentStr, "liza_build_date:") {
		t.Errorf("Metadata missing liza_build_date in _comment")
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
		// Check for explicit tool format: mcp__liza__liza_add_task
		if strings.HasPrefix(permStr, "mcp__liza__") {
			foundLizaMCP = true
			break
		}
	}
	if !foundLizaMCP {
		t.Errorf("Expected liza MCP tools in allow array (e.g., mcp__liza__liza_add_task)")
	}
}
