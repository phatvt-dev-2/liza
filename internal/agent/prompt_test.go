package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// TestBuildPrompt tests the buildPrompt function
func TestBuildPrompt(t *testing.T) {
	now := time.Now().UTC()
	state := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			SpecRef:     "spec.md",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Tasks: []models.Task{
			{
				ID:          "task-1",
				Description: "Test task",
				Status:      models.TaskStatusImplementing,
				Priority:    1,
				SpecRef:     "spec.md",
				DoneWhen:    "Task is complete",
				Created:     now,
			},
		},
		Agents: make(map[string]models.Agent),
		Config: models.Config{
			IntegrationBranch: "main",
		},
	}

	tests := []struct {
		name        string
		role        string
		taskID      string
		initialTask string
		wantErr     bool
		contains    []string
	}{
		{
			name:     "coder prompt",
			role:     "coder",
			taskID:   "task-1",
			wantErr:  false,
			contains: []string{"coder", "Test goal", "task-1"},
		},
		{
			name:     "code-reviewer prompt",
			role:     "code-reviewer",
			taskID:   "task-1",
			wantErr:  false,
			contains: []string{"reviewer", "Test goal", "task-1"},
		},
		{
			name:     "orchestrator prompt",
			role:     "orchestrator",
			taskID:   "",
			wantErr:  false,
			contains: []string{"orchestrator", "Test goal"},
		},
		{
			name:     "code-planner prompt",
			role:     "code-planner",
			taskID:   "task-1",
			wantErr:  false,
			contains: []string{"code-planner", "ASSIGNED CODE PLANNING TASK", "task-1"},
		},
		{
			name:     "code-plan-reviewer prompt",
			role:     "code-plan-reviewer",
			taskID:   "task-1",
			wantErr:  false,
			contains: []string{"code-plan-reviewer", "ASSIGNED CODE PLAN REVIEW TASK", "task-1"},
		},
		{
			name:     "epic-planner prompt",
			role:     "epic-planner",
			taskID:   "task-1",
			wantErr:  false,
			contains: []string{"epic-planner", "task-1"},
		},
		{
			name:     "epic-plan-reviewer prompt",
			role:     "epic-plan-reviewer",
			taskID:   "task-1",
			wantErr:  false,
			contains: []string{"epic-plan-reviewer", "task-1"},
		},
		{
			name:     "us-writer prompt",
			role:     "us-writer",
			taskID:   "task-1",
			wantErr:  false,
			contains: []string{"us-writer", "task-1"},
		},
		{
			name:     "us-reviewer prompt",
			role:     "us-reviewer",
			taskID:   "task-1",
			wantErr:  false,
			contains: []string{"us-reviewer", "task-1"},
		},
		{
			name:     "coder with non-existent task",
			role:     "coder",
			taskID:   "task-999",
			wantErr:  true,
			contains: nil,
		},
		{
			name:        "coder with initial task",
			role:        "coder",
			taskID:      "task-1",
			initialTask: "task-1",
			wantErr:     false,
			contains:    []string{"coder", "RESUME CONTEXT", "task-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			testhelpers.SetupPipelineConfig(t, tmpDir)
			config := SupervisorConfig{
				Role:        tt.role,
				AgentID:     tt.role + "-1",
				ProjectRoot: tmpDir,
				SpecsDir:    filepath.Join(tmpDir, "specs"),
				StatePath:   filepath.Join(tmpDir, "state.yaml"),
				InitialTask: tt.initialTask,
			}

			prompt, err := buildPrompt(state, config, tt.taskID)

			if (err != nil) != tt.wantErr {
				t.Errorf("buildPrompt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if !errors.IsNotFound(err) {
					t.Errorf("expected NotFoundError, got %T: %v", err, err)
				}
				return
			}

			// Check that prompt contains expected strings
			for _, expected := range tt.contains {
				if !strings.Contains(prompt, expected) {
					t.Errorf("buildPrompt() prompt should contain %q", expected)
				}
			}

			// Verify prompt is not empty
			if prompt == "" {
				t.Error("buildPrompt() returned empty prompt")
			}
		})
	}
}

// TestBuildPrompt_CollectiveScoping verifies that sibling task info flows through
// from state to the rendered coder/reviewer prompts.
func TestBuildPrompt_CollectiveScoping(t *testing.T) {
	now := time.Now().UTC()
	state := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			SpecRef:     "specs/vision.md",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Tasks: []models.Task{
			{
				ID:          "task-1",
				Description: "Add auth",
				Status:      models.TaskStatusImplementing,
				Priority:    1,
				SpecRef:     "spec.md",
				DoneWhen:    "Auth works",
				Scope:       "Auth module",
				Iteration:   1,
				Created:     now,
			},
			{
				ID:          "task-2",
				Description: "Add user API",
				Status:      models.TaskStatusReady,
				Priority:    2,
				SpecRef:     "spec.md",
				DoneWhen:    "API works",
				Scope:       "API module",
				Created:     now,
			},
			{
				ID:          "task-3",
				Description: "Add tests",
				Status:      models.TaskStatusMerged,
				Priority:    3,
				SpecRef:     "spec.md",
				DoneWhen:    "Tests pass",
				Scope:       "Test module",
				Created:     now,
			},
		},
		Sprint: models.Sprint{
			Scope: models.SprintScope{
				Planned: []string{"task-1", "task-2", "task-3"},
			},
		},
		Agents: make(map[string]models.Agent),
		Config: models.Config{
			IntegrationBranch: "main",
		},
	}

	tmpDir := t.TempDir()
	config := SupervisorConfig{
		Role:        "coder",
		AgentID:     "coder-1",
		ProjectRoot: tmpDir,
		SpecsDir:    filepath.Join(tmpDir, "specs"),
		StatePath:   filepath.Join(tmpDir, "state.yaml"),
	}

	prompt, err := buildPrompt(state, config, "task-1")
	if err != nil {
		t.Fatalf("buildPrompt() error: %v", err)
	}

	// Should contain scoping section with correct ordinal and sibling tasks
	wantContains := []string{
		"COLLECTIVE PLAN SCOPING",
		"1 of 3 in the current sprint",
		"specs/vision.md",
		"task-2: Add user API [DRAFT_CODE]",
		"task-3: Add tests [MERGED]",
	}
	for _, want := range wantContains {
		if !strings.Contains(prompt, want) {
			t.Errorf("buildPrompt() missing expected scoping content: %q", want)
		}
	}

	// Current task should NOT appear in siblings list
	if strings.Contains(prompt, "task-1: Add auth") {
		t.Error("buildPrompt() should not include current task in sibling list")
	}
}

// TestBuildPrompt_NoScopingForSinglePlannedTask verifies no scoping section
// when the sprint has only one planned task.
func TestBuildPrompt_NoScopingForSinglePlannedTask(t *testing.T) {
	now := time.Now().UTC()
	state := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			SpecRef:     "specs/vision.md",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Tasks: []models.Task{
			{
				ID:          "task-1",
				Description: "Solo task",
				Status:      models.TaskStatusImplementing,
				Priority:    1,
				SpecRef:     "spec.md",
				DoneWhen:    "Done",
				Scope:       "Everything",
				Iteration:   1,
				Created:     now,
			},
		},
		Sprint: models.Sprint{
			Scope: models.SprintScope{
				Planned: []string{"task-1"},
			},
		},
		Agents: make(map[string]models.Agent),
		Config: models.Config{
			IntegrationBranch: "main",
		},
	}

	tmpDir := t.TempDir()
	config := SupervisorConfig{
		Role:        "coder",
		AgentID:     "coder-1",
		ProjectRoot: tmpDir,
		SpecsDir:    filepath.Join(tmpDir, "specs"),
		StatePath:   filepath.Join(tmpDir, "state.yaml"),
	}

	prompt, err := buildPrompt(state, config, "task-1")
	if err != nil {
		t.Fatalf("buildPrompt() error: %v", err)
	}

	if strings.Contains(prompt, "COLLECTIVE PLAN SCOPING") {
		t.Error("buildPrompt() should NOT contain scoping for single-task sprint")
	}
}

// TestBuildPrompt_CollectiveScopingOrdinal verifies the ordinal is computed
// correctly for non-first tasks in the sprint plan.
func TestBuildPrompt_CollectiveScopingOrdinal(t *testing.T) {
	now := time.Now().UTC()
	state := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			SpecRef:     "specs/vision.md",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Tasks: []models.Task{
			{
				ID:          "task-1",
				Description: "Add auth",
				Status:      models.TaskStatusMerged,
				Priority:    1,
				SpecRef:     "spec.md",
				DoneWhen:    "Auth works",
				Scope:       "Auth module",
				Created:     now,
			},
			{
				ID:          "task-2",
				Description: "Add user API",
				Status:      models.TaskStatusImplementing,
				Priority:    2,
				SpecRef:     "spec.md",
				DoneWhen:    "API works",
				Scope:       "API module",
				Iteration:   1,
				Created:     now,
			},
			{
				ID:          "task-3",
				Description: "Add tests",
				Status:      models.TaskStatusReady,
				Priority:    3,
				SpecRef:     "spec.md",
				DoneWhen:    "Tests pass",
				Scope:       "Test module",
				Created:     now,
			},
		},
		Sprint: models.Sprint{
			Scope: models.SprintScope{
				Planned: []string{"task-1", "task-2", "task-3"},
			},
		},
		Agents: make(map[string]models.Agent),
		Config: models.Config{
			IntegrationBranch: "main",
		},
	}

	tmpDir := t.TempDir()
	config := SupervisorConfig{
		Role:        "coder",
		AgentID:     "coder-1",
		ProjectRoot: tmpDir,
		SpecsDir:    filepath.Join(tmpDir, "specs"),
		StatePath:   filepath.Join(tmpDir, "state.yaml"),
	}

	// Build prompt for task-2 (second in plan)
	prompt, err := buildPrompt(state, config, "task-2")
	if err != nil {
		t.Fatalf("buildPrompt() error: %v", err)
	}

	if !strings.Contains(prompt, "2 of 3 in the current sprint") {
		t.Error("buildPrompt() should show correct ordinal (2 of 3) for second task")
	}
	if strings.Contains(prompt, "1 of 3") {
		t.Error("buildPrompt() should NOT hardcode ordinal to 1")
	}
	// task-2 (current) should not appear in siblings
	if strings.Contains(prompt, "task-2: Add user API") {
		t.Error("buildPrompt() should not include current task in sibling list")
	}
	// task-1 and task-3 should appear as siblings
	if !strings.Contains(prompt, "task-1: Add auth [MERGED]") {
		t.Error("buildPrompt() should include task-1 as sibling")
	}
	if !strings.Contains(prompt, "task-3: Add tests [DRAFT_CODE]") {
		t.Error("buildPrompt() should include task-3 as sibling")
	}
}

// TestBuildPrompt_NoScopingForUnplannedTask verifies that mid-sprint replacement
// tasks not in Sprint.Scope.Planned do not get the scoping section.
func TestBuildPrompt_NoScopingForUnplannedTask(t *testing.T) {
	now := time.Now().UTC()
	state := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:          "goal-1",
			Description: "Test goal",
			SpecRef:     "specs/vision.md",
			Status:      models.GoalStatusInProgress,
			Created:     now,
		},
		Tasks: []models.Task{
			{
				ID:          "task-1",
				Description: "Original task",
				Status:      models.TaskStatusMerged,
				Priority:    1,
				SpecRef:     "spec.md",
				DoneWhen:    "Done",
				Scope:       "Module A",
				Created:     now,
			},
			{
				ID:          "task-2",
				Description: "Another planned task",
				Status:      models.TaskStatusReady,
				Priority:    2,
				SpecRef:     "spec.md",
				DoneWhen:    "Done",
				Scope:       "Module B",
				Created:     now,
			},
			{
				// Mid-sprint replacement — not in planned[]
				ID:          "task-3-replacement",
				Description: "Replacement for blocked task",
				Status:      models.TaskStatusImplementing,
				Priority:    1,
				SpecRef:     "spec.md",
				DoneWhen:    "Replacement done",
				Scope:       "Module C",
				Iteration:   1,
				Created:     now,
			},
		},
		Sprint: models.Sprint{
			Scope: models.SprintScope{
				Planned: []string{"task-1", "task-2"}, // task-3-replacement not here
			},
		},
		Agents: make(map[string]models.Agent),
		Config: models.Config{
			IntegrationBranch: "main",
		},
	}

	tmpDir := t.TempDir()
	config := SupervisorConfig{
		Role:        "coder",
		AgentID:     "coder-1",
		ProjectRoot: tmpDir,
		SpecsDir:    filepath.Join(tmpDir, "specs"),
		StatePath:   filepath.Join(tmpDir, "state.yaml"),
	}

	prompt, err := buildPrompt(state, config, "task-3-replacement")
	if err != nil {
		t.Fatalf("buildPrompt() error: %v", err)
	}

	if strings.Contains(prompt, "COLLECTIVE PLAN SCOPING") {
		t.Error("buildPrompt() should NOT show scoping for unplanned replacement task")
	}
	if strings.Contains(prompt, "0 of") {
		t.Error("buildPrompt() should NOT render '0 of N' for unplanned task")
	}
}

// TestPromptSaving tests prompt file creation
func TestPromptSaving(t *testing.T) {
	tmpDir := t.TempDir()
	promptDir := filepath.Join(tmpDir, "agent-prompts")

	prompt := "Test prompt content"
	agentID := "coder-1"

	filePath, err := savePrompt(promptDir, agentID, prompt)
	if err != nil {
		t.Fatalf("savePrompt() error = %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(promptDir); os.IsNotExist(err) {
		t.Error("Prompt directory should be created")
	}

	// Verify file exists and has correct content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read prompt file: %v", err)
	}

	if string(content) != prompt {
		t.Errorf("Prompt content = %q, want %q", string(content), prompt)
	}

	// Verify filename format
	filename := filepath.Base(filePath)
	if !strings.HasPrefix(filename, agentID+"-") {
		t.Errorf("Filename should start with agent ID, got %s", filename)
	}
	if !strings.HasSuffix(filename, ".txt") {
		t.Errorf("Filename should end with .txt, got %s", filename)
	}
}

// TestSavePromptMultipleCalls tests that savePrompt creates unique filenames
func TestSavePromptMultipleCalls(t *testing.T) {
	tmpDir := t.TempDir()
	promptDir := filepath.Join(tmpDir, "prompts")

	// Save multiple prompts
	path1, err := savePrompt(promptDir, "coder-1", "prompt 1")
	if err != nil {
		t.Fatalf("savePrompt() error = %v", err)
	}

	// savePrompt uses second-resolution timestamps (20060102-150405).
	// Wait until the wall-clock second advances so the next call produces
	// a distinct filename, instead of polling savePrompt in a tight loop
	// which would create dozens of duplicate files as a side effect.
	startSec := time.Now().UTC().Truncate(time.Second)
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for time.Now().UTC().Truncate(time.Second).Equal(startSec) {
		select {
		case <-deadline:
			t.Fatal("wall-clock second did not advance within timeout")
		case <-ticker.C:
		}
	}

	path2, err := savePrompt(promptDir, "coder-1", "prompt 2")
	if err != nil {
		t.Fatalf("savePrompt() error = %v", err)
	}

	// Verify paths are different
	if path1 == path2 {
		t.Error("savePrompt() should create unique filenames")
	}

	// Verify both files exist
	if _, err := os.Stat(path1); os.IsNotExist(err) {
		t.Error("First prompt file should exist")
	}
	if _, err := os.Stat(path2); os.IsNotExist(err) {
		t.Error("Second prompt file should exist")
	}
}

// TestOutputSaving tests agent output file creation
func TestOutputSaving(t *testing.T) {
	tmpDir := t.TempDir()
	outputsDir := filepath.Join(tmpDir, "agent-outputs")

	output := "Test agent output content"
	agentID := "claude-1"

	filePath, err := saveOutput(outputsDir, agentID, "txt", output)
	if err != nil {
		t.Fatalf("saveOutput() error = %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(outputsDir); os.IsNotExist(err) {
		t.Error("Outputs directory should be created")
	}

	// Verify file exists and has correct content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != output {
		t.Errorf("Output content = %q, want %q", string(content), output)
	}

	// Verify filename format
	filename := filepath.Base(filePath)
	if !strings.HasPrefix(filename, agentID+"-") {
		t.Errorf("Filename should start with agent ID, got %s", filename)
	}
	if !strings.HasSuffix(filename, ".txt") {
		t.Errorf("Filename should end with .txt, got %s", filename)
	}
}
