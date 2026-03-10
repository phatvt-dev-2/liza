package commands

import (
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestReleaseClaimCommand(t *testing.T) {
	tests := []struct {
		name          string
		taskID        string
		role          string
		force         bool
		reason        string
		agentID       string
		setupState    func(*models.State)
		wantErr       bool
		wantErrMsg    string
		validateState func(*testing.T, *models.State)
	}{
		{
			name:    "release expired reviewer claim",
			taskID:  "t1",
			role:    "code-reviewer",
			force:   false,
			reason:  "manual release",
			agentID: "human",
			setupState: func(s *models.State) {
				reviewingBy := "code-reviewer-1"
				reviewLeaseExpires := time.Now().UTC().Add(-1 * time.Hour) // expired
				reviewCommit := "abc123"
				s.Tasks = []models.Task{
					{
						ID:                 "t1",
						Description:        "Test task",
						Status:             models.TaskStatusReviewing,
						ReviewingBy:        &reviewingBy,
						ReviewLeaseExpires: &reviewLeaseExpires,
						ReviewCommit:       &reviewCommit,
						Created:            time.Now().UTC(),
						History:            []models.TaskHistoryEntry{},
					},
				}
			},
			validateState: func(t *testing.T, s *models.State) {
				task := &s.Tasks[0]
				if task.Status != models.TaskStatusReadyForReview {
					t.Errorf("expected status READY_FOR_REVIEW, got %s", task.Status)
				}
				if task.ReviewingBy != nil {
					t.Errorf("expected reviewing_by to be nil, got %v", task.ReviewingBy)
				}
				if task.ReviewLeaseExpires != nil {
					t.Errorf("expected review_lease_expires to be nil, got %v", task.ReviewLeaseExpires)
				}
				if len(task.History) != 1 {
					t.Fatalf("expected 1 history entry, got %d", len(task.History))
				}
				if task.History[0].Event != "review_claim_released" {
					t.Errorf("expected event review_claim_released, got %s", task.History[0].Event)
				}
			},
		},
		{
			name:    "release expired coder claim",
			taskID:  "t2",
			role:    "coder",
			force:   false,
			reason:  "manual release",
			agentID: "human",
			setupState: func(s *models.State) {
				assignedTo := "coder-1"
				leaseExpires := time.Now().UTC().Add(-1 * time.Hour) // expired
				s.Tasks = []models.Task{
					{
						ID:           "t2",
						Description:  "Test task",
						Status:       models.TaskStatusImplementing,
						AssignedTo:   &assignedTo,
						LeaseExpires: &leaseExpires,
						Created:      time.Now().UTC(),
						History:      []models.TaskHistoryEntry{},
					},
				}
			},
			validateState: func(t *testing.T, s *models.State) {
				task := &s.Tasks[0]
				if task.AssignedTo != nil {
					t.Errorf("expected assigned_to to be nil, got %v", task.AssignedTo)
				}
				if task.LeaseExpires != nil {
					t.Errorf("expected lease_expires to be nil, got %v", task.LeaseExpires)
				}
				if task.Status != models.TaskStatusReady {
					t.Errorf("expected status READY, got %s", task.Status)
				}
				if len(task.History) != 1 {
					t.Fatalf("expected 1 history entry, got %d", len(task.History))
				}
				if task.History[0].Event != "coder_claim_released" {
					t.Errorf("expected event coder_claim_released, got %s", task.History[0].Event)
				}
			},
		},
		{
			name:    "release both claims",
			taskID:  "t3",
			role:    "both",
			force:   false,
			reason:  "reset task",
			agentID: "admin",
			setupState: func(s *models.State) {
				assignedTo := "coder-1"
				leaseExpires := time.Now().UTC().Add(-1 * time.Hour)
				reviewingBy := "code-reviewer-1"
				reviewLeaseExpires := time.Now().UTC().Add(-1 * time.Hour)
				s.Tasks = []models.Task{
					{
						ID:                 "t3",
						Description:        "Test task",
						Status:             models.TaskStatusImplementing,
						AssignedTo:         &assignedTo,
						LeaseExpires:       &leaseExpires,
						ReviewingBy:        &reviewingBy,
						ReviewLeaseExpires: &reviewLeaseExpires,
						Created:            time.Now().UTC(),
						History:            []models.TaskHistoryEntry{},
					},
				}
			},
			validateState: func(t *testing.T, s *models.State) {
				task := &s.Tasks[0]
				if task.AssignedTo != nil {
					t.Errorf("expected assigned_to to be nil, got %v", task.AssignedTo)
				}
				if task.ReviewingBy != nil {
					t.Errorf("expected reviewing_by to be nil, got %v", task.ReviewingBy)
				}
				if len(task.History) != 2 {
					t.Fatalf("expected 2 history entries, got %d", len(task.History))
				}
			},
		},
		{
			name:       "error on valid reviewer lease without force",
			taskID:     "t4",
			role:       "code-reviewer",
			force:      false,
			reason:     "manual release",
			agentID:    "human",
			wantErr:    true,
			wantErrMsg: "review lease still valid",
			setupState: func(s *models.State) {
				reviewingBy := "code-reviewer-1"
				reviewLeaseExpires := time.Now().UTC().Add(1 * time.Hour) // still valid
				reviewCommit := "abc123"
				s.Tasks = []models.Task{
					{
						ID:                 "t4",
						Description:        "Test task",
						Status:             models.TaskStatusReviewing,
						ReviewingBy:        &reviewingBy,
						ReviewLeaseExpires: &reviewLeaseExpires,
						ReviewCommit:       &reviewCommit,
						Created:            time.Now().UTC(),
						History:            []models.TaskHistoryEntry{},
					},
				}
			},
		},
		{
			name:    "force release valid reviewer lease",
			taskID:  "t5",
			role:    "code-reviewer",
			force:   true,
			reason:  "emergency release",
			agentID: "admin",
			setupState: func(s *models.State) {
				reviewingBy := "code-reviewer-1"
				reviewLeaseExpires := time.Now().UTC().Add(1 * time.Hour) // still valid
				reviewCommit := "abc123"
				s.Tasks = []models.Task{
					{
						ID:                 "t5",
						Description:        "Test task",
						Status:             models.TaskStatusReviewing,
						ReviewingBy:        &reviewingBy,
						ReviewLeaseExpires: &reviewLeaseExpires,
						ReviewCommit:       &reviewCommit,
						Created:            time.Now().UTC(),
						History:            []models.TaskHistoryEntry{},
					},
				}
			},
			validateState: func(t *testing.T, s *models.State) {
				task := &s.Tasks[0]
				if task.Status != models.TaskStatusReadyForReview {
					t.Errorf("expected status READY_FOR_REVIEW, got %s", task.Status)
				}
				if task.ReviewingBy != nil {
					t.Errorf("expected reviewing_by to be nil, got %v", task.ReviewingBy)
				}
			},
		},
		{
			name:       "error on valid coder lease without force",
			taskID:     "t6",
			role:       "coder",
			force:      false,
			reason:     "manual release",
			agentID:    "human",
			wantErr:    true,
			wantErrMsg: "coder lease still valid",
			setupState: func(s *models.State) {
				assignedTo := "coder-1"
				leaseExpires := time.Now().UTC().Add(1 * time.Hour) // still valid
				s.Tasks = []models.Task{
					{
						ID:           "t6",
						Description:  "Test task",
						Status:       models.TaskStatusImplementing,
						AssignedTo:   &assignedTo,
						LeaseExpires: &leaseExpires,
						Created:      time.Now().UTC(),
						History:      []models.TaskHistoryEntry{},
					},
				}
			},
		},
		{
			name:    "force release valid coder lease",
			taskID:  "t7",
			role:    "coder",
			force:   true,
			reason:  "emergency release",
			agentID: "admin",
			setupState: func(s *models.State) {
				assignedTo := "coder-1"
				leaseExpires := time.Now().UTC().Add(1 * time.Hour) // still valid
				s.Tasks = []models.Task{
					{
						ID:           "t7",
						Description:  "Test task",
						Status:       models.TaskStatusImplementing,
						AssignedTo:   &assignedTo,
						LeaseExpires: &leaseExpires,
						Created:      time.Now().UTC(),
						History:      []models.TaskHistoryEntry{},
					},
				}
			},
			validateState: func(t *testing.T, s *models.State) {
				task := &s.Tasks[0]
				if task.AssignedTo != nil {
					t.Errorf("expected assigned_to to be nil, got %v", task.AssignedTo)
				}
			},
		},
		{
			name:       "error when no claims to release",
			taskID:     "t8",
			role:       "both",
			force:      false,
			reason:     "manual release",
			agentID:    "human",
			wantErr:    true,
			wantErrMsg: "no claims to release",
			setupState: func(s *models.State) {
				s.Tasks = []models.Task{
					{
						ID:          "t8",
						Description: "Test task",
						Status:      models.TaskStatusReady,
						Created:     time.Now().UTC(),
						History:     []models.TaskHistoryEntry{},
					},
				}
			},
		},
		{
			name:       "missing task ID",
			taskID:     "",
			role:       "coder",
			force:      false,
			reason:     "manual release",
			agentID:    "human",
			wantErr:    true,
			wantErrMsg: "task ID is required",
		},
		{
			name:       "invalid role",
			taskID:     "t1",
			role:       "invalid",
			force:      false,
			reason:     "manual release",
			agentID:    "human",
			wantErr:    true,
			wantErrMsg: "role must be code-reviewer, coder, or both",
		},
		{
			name:       "task not found",
			taskID:     "nonexistent",
			role:       "coder",
			force:      false,
			reason:     "manual release",
			agentID:    "human",
			wantErr:    true,
			wantErrMsg: "task not found",
			setupState: func(s *models.State) {
				s.Tasks = []models.Task{}
			},
		},
		{
			name:    "coder claim release doesn't change READY_FOR_REVIEW status",
			taskID:  "t9",
			role:    "coder",
			force:   false,
			reason:  "manual release",
			agentID: "human",
			setupState: func(s *models.State) {
				assignedTo := "coder-1"
				leaseExpires := time.Now().UTC().Add(-1 * time.Hour)
				s.Tasks = []models.Task{
					{
						ID:           "t9",
						Description:  "Test task",
						Status:       models.TaskStatusReadyForReview,
						AssignedTo:   &assignedTo,
						LeaseExpires: &leaseExpires,
						Created:      time.Now().UTC(),
						History:      []models.TaskHistoryEntry{},
					},
				}
			},
			validateState: func(t *testing.T, s *models.State) {
				task := &s.Tasks[0]
				if task.Status != models.TaskStatusReadyForReview {
					t.Errorf("expected status READY_FOR_REVIEW to be preserved, got %s", task.Status)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for test
			tmpDir := t.TempDir()
			statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)
			testhelpers.SetupPipelineConfig(t, tmpDir)

			// Initialize state
			initialState := &models.State{
				Config: models.Config{
					IntegrationBranch: "integration",
					LeaseDuration:     1800,
				},
				Tasks:  []models.Task{},
				Agents: make(map[string]models.Agent),
			}

			// Setup state if provided
			if tt.setupState != nil {
				tt.setupState(initialState)
			}

			// Write initial state
			bb := testhelpers.WriteInitialState(t, statePath, initialState)

			// Execute command
			err := ReleaseClaimCommand(tmpDir, tt.taskID, tt.role, tt.force, tt.reason, tt.agentID)

			// Check error
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else {
					testhelpers.AssertErrorContains(t, err, tt.wantErrMsg)
				}
			} else {
				testhelpers.AssertNoError(t, err)
			}

			// Validate state if no error expected
			if !tt.wantErr && tt.validateState != nil {
				state, err := bb.Read()
				if err != nil {
					t.Fatalf("failed to read state: %v", err)
				}
				tt.validateState(t, state)
			}
		})
	}
}
