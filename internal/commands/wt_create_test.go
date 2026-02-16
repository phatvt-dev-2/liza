package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestWtCreateCommand(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		taskStatus  models.TaskStatus
		fresh       bool
		existingWT  bool
		wantErr     bool
		errContains string
	}{
		{
			name:       "create worktree for CLAIMED task",
			taskID:     "task-1",
			taskStatus: models.TaskStatusClaimed,
			fresh:      false,
			existingWT: false,
			wantErr:    false,
		},
		{
			name:        "task not CLAIMED",
			taskID:      "task-2",
			taskStatus:  models.TaskStatusUnclaimed,
			fresh:       false,
			existingWT:  false,
			wantErr:     true,
			errContains: "not CLAIMED",
		},
		{
			name:       "worktree already exists without fresh",
			taskID:     "task-3",
			taskStatus: models.TaskStatusClaimed,
			fresh:      false,
			existingWT: true,
			wantErr:    false, // Should succeed without error
		},
		{
			name:       "worktree already exists with fresh",
			taskID:     "task-4",
			taskStatus: models.TaskStatusClaimed,
			fresh:      true,
			existingWT: true,
			wantErr:    false,
		},
		{
			name:        "empty task ID",
			taskID:      "",
			taskStatus:  models.TaskStatusClaimed,
			fresh:       false,
			existingWT:  false,
			wantErr:     true,
			errContains: "task ID is required",
		},
		{
			name:        "nonexistent task",
			taskID:      "nonexistent",
			taskStatus:  models.TaskStatusClaimed,
			fresh:       false,
			existingWT:  false,
			wantErr:     true,
			errContains: "task not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory (project root)
			tmpDir := t.TempDir()

			// Setup git repo and liza directory
			testhelpers.SetupTestGitRepo(t, tmpDir)
			stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

			// Create initial state
			now := time.Now().UTC()
			agent := "coder-1"
			initialState := &models.State{
				Version: 1,
				Goal: models.Goal{
					ID:               "goal-1",
					Description:      "Test goal",
					SpecRef:          "README.md",
					Created:          now,
					Status:           models.GoalStatusInProgress,
					AlignmentHistory: []models.AlignmentHistory{},
				},
				Tasks:  []models.Task{},
				Agents: make(map[string]models.Agent),
				Sprint: models.Sprint{
					ID:      "sprint-1",
					GoalRef: "goal-1",
					Scope: models.SprintScope{
						Planned: []string{},
						Stretch: []string{},
					},
					Timeline: models.SprintTimeline{
						Started: now,
					},
					Status: models.SprintStatusInProgress,
				},
				CircuitBreaker: models.CircuitBreaker{
					Status:  "OK",
					History: []models.CircuitBreakerHistory{},
				},
				Config: models.Config{
					MaxCoderIterations: 10,
					MaxReviewCycles:    5,
					IntegrationBranch:  "integration",
				},
			}

			// Add task if not testing nonexistent task
			if tt.taskID != "nonexistent" && tt.taskID != "" {
				worktreePath := filepath.Join(".worktrees", tt.taskID)
				task := models.Task{
					ID:          tt.taskID,
					Description: "Test task",
					Status:      tt.taskStatus,
					Priority:    1,
					Created:     now,
					SpecRef:     "README.md",
					DoneWhen:    "Done",
					Scope:       "Test",
					History:     []models.TaskHistoryEntry{},
				}

				if tt.taskStatus == models.TaskStatusClaimed {
					task.AssignedTo = &agent
					task.Worktree = &worktreePath
					leaseExpires := now.Add(30 * time.Minute)
					task.LeaseExpires = &leaseExpires
				}

				initialState.Tasks = append(initialState.Tasks, task)
			}

			// Write initial state
			bb := testhelpers.WriteInitialState(t, stateFile, initialState)

			// Create existing worktree if needed
			if tt.existingWT && tt.taskID != "" && tt.taskID != "nonexistent" {
				wtDir := filepath.Join(tmpDir, ".worktrees", tt.taskID)
				if err := os.MkdirAll(wtDir, 0755); err != nil {
					t.Fatalf("Failed to create existing worktree: %v", err)
				}
				// Create a minimal git worktree
				if err := exec.Command("git", "-C", tmpDir, "worktree", "add", wtDir, "integration", "-b", "task/"+tt.taskID).Run(); err != nil {
					t.Fatalf("Failed to create existing worktree: %v", err)
				}

				// Set base_commit in state since worktree already exists
				gitWrapper := git.New(tmpDir)
				baseCommit, err := gitWrapper.GetCommitSHA("integration")
				if err != nil {
					t.Fatalf("Failed to get base commit: %v", err)
				}

				err = bb.Modify(func(state *models.State) error {
					for i := range state.Tasks {
						if state.Tasks[i].ID == tt.taskID {
							state.Tasks[i].BaseCommit = &baseCommit
							return nil
						}
					}
					return fmt.Errorf("task not found")
				})
				if err != nil {
					t.Fatalf("Failed to set base_commit: %v", err)
				}
			}

			// Run command
			err := WtCreateCommand(tmpDir, tt.taskID, tt.fresh)

			// Check error
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				testhelpers.AssertErrorContains(t, err, tt.errContains)
				return
			}

			testhelpers.AssertNoError(t, err)

			// Verify worktree was created
			wtDir := filepath.Join(tmpDir, ".worktrees", tt.taskID)
			if _, err := os.Stat(wtDir); os.IsNotExist(err) {
				t.Errorf("Worktree directory not created: %s", wtDir)
			}

			// Verify branch was created
			branchName := "task/" + tt.taskID
			cmd := exec.Command("git", "-C", tmpDir, "branch", "--list", branchName)
			output, err := cmd.Output()
			if err != nil {
				t.Errorf("Failed to check branch: %v", err)
			}
			if !strings.Contains(string(output), branchName) {
				t.Errorf("Branch %s not created", branchName)
			}

			// Verify base_commit was updated in state
			state, err := bb.Read()
			if err != nil {
				t.Fatalf("Failed to read state: %v", err)
			}

			var task *models.Task
			for i := range state.Tasks {
				if state.Tasks[i].ID == tt.taskID {
					task = &state.Tasks[i]
					break
				}
			}

			if task == nil {
				t.Fatalf("Task %s not found in state", tt.taskID)
			}

			if task.BaseCommit == nil {
				t.Errorf("base_commit not set in state")
			} else if *task.BaseCommit == "" {
				t.Errorf("base_commit is empty")
			}
		})
	}
}

func TestWtCreateCommandIntegration(t *testing.T) {
	// Create temp directory (project root)
	tmpDir := t.TempDir()

	// Setup git repo and liza directory
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	// Create initial state with CLAIMED task
	now := time.Now().UTC()
	agent := "coder-1"
	worktreePath := ".worktrees/task-integration"
	leaseExpires := now.Add(30 * time.Minute)

	initialState := &models.State{
		Version: 1,
		Goal: models.Goal{
			ID:               "goal-1",
			Description:      "Test goal",
			SpecRef:          "README.md",
			Created:          now,
			Status:           models.GoalStatusInProgress,
			AlignmentHistory: []models.AlignmentHistory{},
		},
		Tasks: []models.Task{
			{
				ID:           "task-integration",
				Description:  "Integration test task",
				Status:       models.TaskStatusClaimed,
				Priority:     1,
				Created:      now,
				SpecRef:      "README.md",
				DoneWhen:     "Done",
				Scope:        "Test",
				AssignedTo:   &agent,
				Worktree:     &worktreePath,
				LeaseExpires: &leaseExpires,
				History:      []models.TaskHistoryEntry{},
			},
		},
		Agents: make(map[string]models.Agent),
		Sprint: models.Sprint{
			ID:      "sprint-1",
			GoalRef: "goal-1",
			Scope: models.SprintScope{
				Planned: []string{"task-integration"},
				Stretch: []string{},
			},
			Timeline: models.SprintTimeline{
				Started: now,
			},
			Status: models.SprintStatusInProgress,
		},
		CircuitBreaker: models.CircuitBreaker{
			Status:  "OK",
			History: []models.CircuitBreakerHistory{},
		},
		Config: models.Config{
			MaxCoderIterations: 10,
			MaxReviewCycles:    5,
			IntegrationBranch:  "integration",
		},
	}

	bb := testhelpers.WriteInitialState(t, stateFile, initialState)

	// Run command
	if err := WtCreateCommand(tmpDir, "task-integration", false); err != nil {
		t.Fatalf("WtCreateCommand failed: %v", err)
	}

	// Verify worktree was created and is functional
	wtDir := filepath.Join(tmpDir, ".worktrees", "task-integration")

	// Check that worktree directory exists
	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		t.Fatalf("Worktree directory not created: %s", wtDir)
	}

	// Verify we can perform git operations in the worktree
	testFile := filepath.Join(wtDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content\n"), 0644); err != nil {
		t.Fatalf("Failed to write test file in worktree: %v", err)
	}

	// Verify git status works in worktree
	cmd := exec.Command("git", "-C", wtDir, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to run git status in worktree: %v", err)
	}

	if !strings.Contains(string(output), "test.txt") {
		t.Errorf("Git status should show test.txt as untracked")
	}

	// Verify base_commit is set correctly
	state, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := &state.Tasks[0]
	if task.BaseCommit == nil {
		t.Fatal("base_commit not set")
	}

	// Verify base_commit is a valid git SHA
	gitWrapper := git.New(tmpDir)
	integrationSHA, err := gitWrapper.GetCommitSHA("integration")
	if err != nil {
		t.Fatalf("Failed to get integration SHA: %v", err)
	}

	if integrationSHA != *task.BaseCommit {
		t.Errorf("base_commit %s doesn't match integration branch %s", *task.BaseCommit, integrationSHA)
	}
}
