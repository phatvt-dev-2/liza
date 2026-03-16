package commands

import (
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestAssessBlockedCommand(t *testing.T) {
	tests := []struct {
		name          string
		taskID        string
		note          string
		agentID       string
		setupState    func(*models.State)
		wantErr       bool
		wantErrMsg    string
		validateState func(*testing.T, *models.State)
	}{
		{
			name:    "Success - assess BLOCKED task",
			taskID:  "task-1",
			note:    "Cannot resolve without external API",
			agentID: "orchestrator-1",
			wantErr: false,
			setupState: func(s *models.State) {
				now := time.Now().UTC()
				s.Tasks = []models.Task{
					{
						ID:          "task-1",
						Description: "Test task",
						Status:      models.TaskStatusBlocked,
						Created:     now,
						History:     []models.TaskHistoryEntry{},
					},
				}
			},
			validateState: func(t *testing.T, s *models.State) {
				task := &s.Tasks[0]
				if task.Status != models.TaskStatusBlocked {
					t.Errorf("expected status BLOCKED, got %s", task.Status)
				}
				if len(task.History) != 1 {
					t.Fatalf("expected 1 history entry, got %d", len(task.History))
				}
				if task.History[0].Event != "orchestrator_assessment" {
					t.Errorf("expected event orchestrator_assessment, got %s", task.History[0].Event)
				}
				if task.History[0].Agent == nil || *task.History[0].Agent != "orchestrator-1" {
					t.Errorf("expected agent orchestrator-1 in history, got %v", task.History[0].Agent)
				}
				if task.History[0].Note == nil || *task.History[0].Note != "Cannot resolve without external API" {
					t.Errorf("expected note in history, got %v", task.History[0].Note)
				}
			},
		},
		{
			name:       "Error - Empty task ID",
			taskID:     "",
			agentID:    "orchestrator-1",
			wantErr:    true,
			wantErrMsg: "task ID is required",
		},
		{
			name:       "Error - Empty agent ID",
			taskID:     "task-1",
			agentID:    "",
			wantErr:    true,
			wantErrMsg: "agent ID is required",
		},
		{
			name:       "Error - Task not found",
			taskID:     "nonexistent",
			agentID:    "orchestrator-1",
			wantErr:    true,
			wantErrMsg: "task not found",
			setupState: func(s *models.State) {
				s.Tasks = []models.Task{}
			},
		},
		{
			name:       "Error - Task not in BLOCKED status",
			taskID:     "task-1",
			agentID:    "orchestrator-1",
			wantErr:    true,
			wantErrMsg: "BLOCKED status",
			setupState: func(s *models.State) {
				now := time.Now().UTC()
				s.Tasks = []models.Task{
					{
						ID:          "task-1",
						Description: "Test task",
						Status:      models.TaskStatusReady,
						Created:     now,
						History:     []models.TaskHistoryEntry{},
					},
				}
			},
		},
		{
			name:    "Success - idempotent (two calls = two entries)",
			taskID:  "task-1",
			note:    "second assessment",
			agentID: "orchestrator-1",
			wantErr: false,
			setupState: func(s *models.State) {
				now := time.Now().UTC()
				agent := "orchestrator-1"
				s.Tasks = []models.Task{
					{
						ID:          "task-1",
						Description: "Test task",
						Status:      models.TaskStatusBlocked,
						Created:     now,
						History: []models.TaskHistoryEntry{
							{
								Time:  now.Add(-10 * time.Minute),
								Event: "orchestrator_assessment",
								Agent: &agent,
							},
						},
					},
				}
			},
			validateState: func(t *testing.T, s *models.State) {
				task := &s.Tasks[0]
				assessmentCount := 0
				for _, entry := range task.History {
					if entry.Event == "orchestrator_assessment" {
						assessmentCount++
					}
				}
				if assessmentCount != 2 {
					t.Errorf("expected 2 assessment entries, got %d", assessmentCount)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

			initialState := &models.State{
				Config: models.Config{
					IntegrationBranch: "integration",
					LeaseDuration:     1800,
				},
				Tasks:  []models.Task{},
				Agents: make(map[string]models.Agent),
			}

			if tt.setupState != nil {
				tt.setupState(initialState)
			}

			bb := testhelpers.WriteInitialState(t, statePath, initialState)

			err := AssessBlockedCommand(tmpDir, tt.taskID, tt.note, tt.agentID)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else {
					testhelpers.AssertErrorContains(t, err, tt.wantErrMsg)
				}
			} else {
				testhelpers.AssertNoError(t, err)
			}

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
