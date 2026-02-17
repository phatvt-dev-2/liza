package commands

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestHandoffCommand(t *testing.T) {
	tests := []struct {
		name          string
		taskID        string
		summary       string
		nextAction    string
		agentID       string
		setupState    func(*models.State)
		wantErr       bool
		wantErrSubstr string
		validateState func(*testing.T, *models.State)
	}{
		{
			name:       "successful handoff",
			taskID:     "task-1",
			summary:    "Implemented parsing and validation",
			nextAction: "Finish edge case tests",
			agentID:    "coder-1",
			setupState: func(s *models.State) {
				now := time.Now().UTC()
				assigned := "coder-1"
				lease := now.Add(30 * time.Minute)
				s.Tasks = []models.Task{
					{
						ID:           "task-1",
						Description:  "Test task",
						Status:       models.TaskStatusImplementing,
						AssignedTo:   &assigned,
						LeaseExpires: &lease,
						Created:      now,
						SpecRef:      "README.md",
						DoneWhen:     "Done",
						Scope:        "Scope",
						History:      []models.TaskHistoryEntry{},
					},
				}
				s.Agents["coder-1"] = models.Agent{
					Role:      "coder",
					Status:    models.AgentStatusWorking,
					Heartbeat: now,
				}
			},
			validateState: func(t *testing.T, s *models.State) {
				task := &s.Tasks[0]
				if !task.HandoffPending {
					t.Fatalf("expected handoff_pending=true")
				}
				if len(task.History) != 1 {
					t.Fatalf("expected one history entry, got %d", len(task.History))
				}
				if task.History[0].Event != "handoff_initiated" {
					t.Fatalf("expected handoff_initiated event, got %s", task.History[0].Event)
				}
				if task.History[0].Note == nil || !strings.Contains(*task.History[0].Note, "next_action") {
					t.Fatalf("expected history note to include next_action, got %v", task.History[0].Note)
				}

				note, ok := s.Handoff["task-1"]
				if !ok {
					t.Fatalf("expected handoff note for task-1")
				}
				if note.Agent != "coder-1" {
					t.Fatalf("expected handoff agent coder-1, got %s", note.Agent)
				}
				if note.Summary != "Implemented parsing and validation" {
					t.Fatalf("unexpected handoff summary: %s", note.Summary)
				}
				if note.NextAction != "Finish edge case tests" {
					t.Fatalf("unexpected handoff next_action: %s", note.NextAction)
				}

				agent, ok := s.Agents["coder-1"]
				if !ok {
					t.Fatalf("expected coder-1 agent in state")
				}
				if agent.Status != models.AgentStatusHandoff {
					t.Fatalf("expected agent status HANDOFF, got %s", agent.Status)
				}
				if agent.CurrentTask == nil || *agent.CurrentTask != "task-1" {
					t.Fatalf("expected agent current_task task-1, got %v", agent.CurrentTask)
				}
			},
		},
		{
			name:          "missing task ID",
			taskID:        "",
			summary:       "summary",
			nextAction:    "next",
			agentID:       "coder-1",
			wantErr:       true,
			wantErrSubstr: "task ID is required",
		},
		{
			name:          "missing summary",
			taskID:        "task-1",
			summary:       "",
			nextAction:    "next",
			agentID:       "coder-1",
			wantErr:       true,
			wantErrSubstr: "summary is required",
		},
		{
			name:          "missing next action",
			taskID:        "task-1",
			summary:       "summary",
			nextAction:    "",
			agentID:       "coder-1",
			wantErr:       true,
			wantErrSubstr: "next action is required",
		},
		{
			name:          "missing agent",
			taskID:        "task-1",
			summary:       "summary",
			nextAction:    "next",
			agentID:       "",
			wantErr:       true,
			wantErrSubstr: "LIZA_AGENT_ID is required",
		},
		{
			name:       "task not found",
			taskID:     "missing",
			summary:    "summary",
			nextAction: "next",
			agentID:    "coder-1",
			setupState: func(s *models.State) {
				s.Tasks = []models.Task{}
			},
			wantErr:       true,
			wantErrSubstr: "task not found",
		},
		{
			name:       "task not claimed",
			taskID:     "task-1",
			summary:    "summary",
			nextAction: "next",
			agentID:    "coder-1",
			setupState: func(s *models.State) {
				now := time.Now().UTC()
				s.Tasks = []models.Task{
					{
						ID:          "task-1",
						Description: "Test",
						Status:      models.TaskStatusReady,
						Created:     now,
						SpecRef:     "README.md",
						DoneWhen:    "Done",
						Scope:       "Scope",
						History:     []models.TaskHistoryEntry{},
					},
				}
			},
			wantErr:       true,
			wantErrSubstr: "is not IMPLEMENTING",
		},
		{
			name:       "task assigned to different coder",
			taskID:     "task-1",
			summary:    "summary",
			nextAction: "next",
			agentID:    "coder-1",
			setupState: func(s *models.State) {
				now := time.Now().UTC()
				assigned := "coder-2"
				s.Tasks = []models.Task{
					{
						ID:          "task-1",
						Description: "Test",
						Status:      models.TaskStatusImplementing,
						AssignedTo:  &assigned,
						Created:     now,
						SpecRef:     "README.md",
						DoneWhen:    "Done",
						Scope:       "Scope",
						History:     []models.TaskHistoryEntry{},
					},
				}
			},
			wantErr:       true,
			wantErrSubstr: "is not assigned to agent coder-1",
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
				Tasks:   []models.Task{},
				Agents:  make(map[string]models.Agent),
				Handoff: make(map[string]models.HandoffNote),
			}

			if tt.setupState != nil {
				tt.setupState(initialState)
			}

			bb := testhelpers.WriteInitialState(t, statePath, initialState)

			err := HandoffCommand(tmpDir, tt.taskID, tt.summary, tt.nextAction, tt.agentID)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErrSubstr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if tt.validateState != nil {
				state, readErr := bb.Read()
				if readErr != nil {
					t.Fatalf("failed to read state: %v", readErr)
				}
				tt.validateState(t, state)
			}
		})
	}
}
