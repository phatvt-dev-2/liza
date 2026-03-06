package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
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

			state := testhelpers.CreateValidState()
			state.Tasks = tt.tasks

			testhelpers.WriteInitialState(t, statePath, state)

			bb := db.New(statePath)

			// Test hasPendingMerges (legacy — no pipeline.yaml)
			result := hasPendingMerges(bb, tt.agentID, tmpDir)
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := testhelpers.CreateValidState()
			state.Tasks = tt.tasks
			testhelpers.WriteInitialState(t, statePath, state)

			bb := db.New(statePath)
			result := hasPendingMerges(bb, tt.agentID, tmpDir)
			if result != tt.expected {
				t.Errorf("hasPendingMerges() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// findPipelineTestdata locates the pipeline testdata YAML in the repo.
func findPipelineTestdata(t *testing.T) string {
	t.Helper()
	return filepath.Join(testhelpers.FindRepoRoot(t), "internal", "pipeline", "testdata", "valid-coding-subpipeline.yaml")
}
