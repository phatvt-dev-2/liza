package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// TestHasPendingMerges tests the hasPendingMerges function
func TestHasPendingMerges(t *testing.T) {
	tests := []struct {
		name     string
		tasks    []models.Task
		agentID  string
		expected bool
	}{
		{
			name:     "no tasks returns false",
			tasks:    []models.Task{},
			agentID:  "code-reviewer-1",
			expected: false,
		},
		{
			name: "approved task with merge_commit set returns false",
			tasks: []models.Task{
				{
					ID:          "task-1",
					Status:      models.TaskStatusApproved,
					ApprovedBy:  testhelpers.StringPtr("code-reviewer-1"),
					MergeCommit: testhelpers.StringPtr("abc123"),
				},
			},
			agentID:  "code-reviewer-1",
			expected: false,
		},
		{
			name: "approved task by different agent returns false",
			tasks: []models.Task{
				{
					ID:          "task-1",
					Status:      models.TaskStatusApproved,
					ApprovedBy:  testhelpers.StringPtr("code-reviewer-2"),
					MergeCommit: nil,
				},
			},
			agentID:  "code-reviewer-1",
			expected: false,
		},
		{
			name: "approved task by this agent without merge_commit returns true",
			tasks: []models.Task{
				{
					ID:          "task-1",
					Status:      models.TaskStatusApproved,
					ApprovedBy:  testhelpers.StringPtr("code-reviewer-1"),
					MergeCommit: nil,
				},
			},
			agentID:  "code-reviewer-1",
			expected: true,
		},
		{
			name: "multiple tasks, one pending returns true",
			tasks: []models.Task{
				{
					ID:          "task-1",
					Status:      models.TaskStatusApproved,
					ApprovedBy:  testhelpers.StringPtr("code-reviewer-1"),
					MergeCommit: testhelpers.StringPtr("abc123"),
				},
				{
					ID:          "task-2",
					Status:      models.TaskStatusApproved,
					ApprovedBy:  testhelpers.StringPtr("code-reviewer-1"),
					MergeCommit: nil,
				},
			},
			agentID:  "code-reviewer-1",
			expected: true,
		},
		{
			name: "coding_plan_approved task by this agent without merge_commit returns true",
			tasks: []models.Task{
				{
					ID:          "task-1",
					Status:      models.TaskStatusCodingPlanApproved,
					ApprovedBy:  testhelpers.StringPtr("code-reviewer-1"),
					MergeCommit: nil,
				},
			},
			agentID:  "code-reviewer-1",
			expected: true,
		},
		{
			name: "integration_failed task returns false",
			tasks: []models.Task{
				{
					ID:          "task-1",
					Status:      models.TaskStatusIntegrationFailed,
					ApprovedBy:  testhelpers.StringPtr("code-reviewer-1"),
					MergeCommit: nil,
				},
			},
			agentID:  "code-reviewer-1",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test blackboard with tasks (no pipeline config = legacy mode)
			tmpDir := t.TempDir()
			statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
			testhelpers.SetupPipelineConfig(t, tmpDir)

			state := testhelpers.CreateValidState()
			state.Tasks = tt.tasks

			testhelpers.WriteInitialState(t, statePath, state)

			bb := db.New(statePath)

			// Test hasPendingMerges (legacy — no pipeline.yaml)
			result := hasPendingMerges(bb, tt.agentID, nil)
			if result != tt.expected {
				t.Errorf("hasPendingMerges() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestHasPendingMerges_Pipeline tests pipeline-aware merge detection
func TestHasPendingMerges_Pipeline(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	// Install frozen pipeline config
	src, err := os.ReadFile(findPipelineTestdata(t))
	if err != nil {
		t.Fatalf("Failed to read pipeline testdata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".liza", "pipeline.yaml"), src, 0644); err != nil {
		t.Fatalf("Failed to write frozen pipeline config: %v", err)
	}

	tests := []struct {
		name     string
		tasks    []models.Task
		agentID  string
		expected bool
	}{
		{
			name: "CODE_APPROVED pipeline task returns true",
			tasks: []models.Task{
				{
					ID:         "task-1",
					Status:     "CODE_APPROVED",
					RolePair:   "coding-pair",
					ApprovedBy: testhelpers.StringPtr("code-reviewer-1"),
				},
			},
			agentID:  "code-reviewer-1",
			expected: true,
		},
		{
			name: "CODE_APPROVED pipeline task by different agent returns false",
			tasks: []models.Task{
				{
					ID:         "task-1",
					Status:     "CODE_APPROVED",
					RolePair:   "coding-pair",
					ApprovedBy: testhelpers.StringPtr("code-reviewer-2"),
				},
			},
			agentID:  "code-reviewer-1",
			expected: false,
		},
		{
			name: "CODING_PLAN_APPROVED pipeline task returns true",
			tasks: []models.Task{
				{
					ID:         "task-1",
					Status:     "CODING_PLAN_APPROVED",
					RolePair:   "code-planning-pair",
					ApprovedBy: testhelpers.StringPtr("code-plan-reviewer-1"),
				},
			},
			agentID:  "code-plan-reviewer-1",
			expected: true,
		},
		{
			name: "legacy APPROVED with no role_pair still works with pipeline config",
			tasks: []models.Task{
				{
					ID:         "task-1",
					Status:     models.TaskStatusApproved,
					ApprovedBy: testhelpers.StringPtr("code-reviewer-1"),
				},
			},
			agentID:  "code-reviewer-1",
			expected: true,
		},
	}

	pr := ops.LoadResolverForModels(tmpDir)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := testhelpers.CreateValidState()
			state.Tasks = tt.tasks
			testhelpers.WriteInitialState(t, statePath, state)

			bb := db.New(statePath)
			result := hasPendingMerges(bb, tt.agentID, pr)
			if result != tt.expected {
				t.Errorf("hasPendingMerges() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestHasPendingMerges_Phase2Pipeline tests pipeline-aware merge detection with Phase 2 config
// (epic-planning-pair, us-writing-pair, code-planning-pair, coding-pair).
func TestHasPendingMerges_Phase2Pipeline(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	// Install Phase 2 frozen pipeline config
	src, err := os.ReadFile(findPhase2PipelineTestdata(t))
	if err != nil {
		t.Fatalf("Failed to read Phase 2 pipeline testdata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".liza", "pipeline.yaml"), src, 0644); err != nil {
		t.Fatalf("Failed to write frozen pipeline config: %v", err)
	}

	tests := []struct {
		name     string
		tasks    []models.Task
		agentID  string
		expected bool
	}{
		{
			name: "US_APPROVED us-writing-pair task returns true",
			tasks: []models.Task{
				{
					ID:         "task-1",
					Status:     "US_APPROVED",
					RolePair:   "us-writing-pair",
					ApprovedBy: testhelpers.StringPtr("us-reviewer-1"),
				},
			},
			agentID:  "us-reviewer-1",
			expected: true,
		},
		{
			name: "EPIC_PLAN_APPROVED epic-planning-pair task returns true",
			tasks: []models.Task{
				{
					ID:         "task-1",
					Status:     "EPIC_PLAN_APPROVED",
					RolePair:   "epic-planning-pair",
					ApprovedBy: testhelpers.StringPtr("epic-plan-reviewer-1"),
				},
			},
			agentID:  "epic-plan-reviewer-1",
			expected: true,
		},
		{
			name: "US_APPROVED by different agent returns false",
			tasks: []models.Task{
				{
					ID:         "task-1",
					Status:     "US_APPROVED",
					RolePair:   "us-writing-pair",
					ApprovedBy: testhelpers.StringPtr("us-reviewer-2"),
				},
			},
			agentID:  "us-reviewer-1",
			expected: false,
		},
		{
			name: "US_APPROVED already merged returns false",
			tasks: []models.Task{
				{
					ID:          "task-1",
					Status:      "US_APPROVED",
					RolePair:    "us-writing-pair",
					ApprovedBy:  testhelpers.StringPtr("us-reviewer-1"),
					MergeCommit: testhelpers.StringPtr("abc123"),
				},
			},
			agentID:  "us-reviewer-1",
			expected: false,
		},
		{
			name: "legacy APPROVED with no role_pair still works with Phase 2 config",
			tasks: []models.Task{
				{
					ID:         "task-1",
					Status:     models.TaskStatusApproved,
					ApprovedBy: testhelpers.StringPtr("code-reviewer-1"),
				},
			},
			agentID:  "code-reviewer-1",
			expected: true,
		},
		{
			name: "CODE_APPROVED coding-pair still works in Phase 2 config",
			tasks: []models.Task{
				{
					ID:         "task-1",
					Status:     "CODE_APPROVED",
					RolePair:   "coding-pair",
					ApprovedBy: testhelpers.StringPtr("code-reviewer-1"),
				},
			},
			agentID:  "code-reviewer-1",
			expected: true,
		},
	}

	pr := ops.LoadResolverForModels(tmpDir)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := testhelpers.CreateValidState()
			state.Tasks = tt.tasks
			testhelpers.WriteInitialState(t, statePath, state)

			bb := db.New(statePath)
			result := hasPendingMerges(bb, tt.agentID, pr)
			if result != tt.expected {
				t.Errorf("hasPendingMerges() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestLogTaskSubmissionIfCompleted_Phase2Pipeline tests that Phase 2 pipeline statuses
// are correctly recognized by logTaskSubmissionIfCompleted.
func TestLogTaskSubmissionIfCompleted_Phase2Pipeline(t *testing.T) {
	tmpDir := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	// Install Phase 2 frozen pipeline config
	src, err := os.ReadFile(findPhase2PipelineTestdata(t))
	if err != nil {
		t.Fatalf("Failed to read Phase 2 pipeline testdata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".liza", "pipeline.yaml"), src, 0644); err != nil {
		t.Fatalf("Failed to write frozen pipeline config: %v", err)
	}

	tests := []struct {
		name    string
		task    models.Task
		wantErr bool
	}{
		{
			name: "US_READY_FOR_REVIEW recognized as submitted",
			task: models.Task{
				ID:       "task-1",
				Status:   "US_READY_FOR_REVIEW",
				RolePair: "us-writing-pair",
			},
		},
		{
			name: "WRITING_US recognized as executing",
			task: models.Task{
				ID:       "task-2",
				Status:   "WRITING_US",
				RolePair: "us-writing-pair",
			},
		},
		{
			name: "EPIC_PLAN_TO_REVIEW recognized as submitted",
			task: models.Task{
				ID:       "task-3",
				Status:   "EPIC_PLAN_TO_REVIEW",
				RolePair: "epic-planning-pair",
			},
		},
		{
			name: "EPIC_PLANNING recognized as executing",
			task: models.Task{
				ID:       "task-4",
				Status:   "EPIC_PLANNING",
				RolePair: "epic-planning-pair",
			},
		},
		{
			name: "legacy READY_FOR_REVIEW still works",
			task: models.Task{
				ID:     "task-5",
				Status: models.TaskStatusReadyForReview,
			},
		},
	}

	pr := ops.LoadResolverForModels(tmpDir)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := testhelpers.CreateValidState()
			state.Tasks = []models.Task{tt.task}
			testhelpers.WriteInitialState(t, statePath, state)

			bb := db.New(statePath)
			err := logTaskSubmissionIfCompleted(bb, tt.task.ID, "agent-1", pr)
			if (err != nil) != tt.wantErr {
				t.Errorf("logTaskSubmissionIfCompleted() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// findPipelineTestdata locates the Phase 1 pipeline testdata YAML in the repo.
func findPipelineTestdata(t *testing.T) string {
	t.Helper()
	return filepath.Join(testhelpers.FindRepoRoot(t), "internal", "pipeline", "testdata", "valid-coding-subpipeline.yaml")
}

// findPhase2PipelineTestdata locates the Phase 2 pipeline testdata YAML in the repo.
func findPhase2PipelineTestdata(t *testing.T) string {
	t.Helper()
	return filepath.Join(testhelpers.FindRepoRoot(t), "internal", "pipeline", "testdata", "valid-phase2-full.yaml")
}
