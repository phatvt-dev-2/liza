package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestSubmitVerdict_Validation(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		verdict     string
		reason      string
		agentID     string
		errContains string
	}{
		{
			name: "empty task ID", verdict: "APPROVED", agentID: "r1",
			errContains: "task ID is required",
		},
		{
			name: "empty verdict", taskID: "t1", agentID: "r1",
			errContains: "verdict is required",
		},
		{
			name: "empty agent ID", taskID: "t1", verdict: "APPROVED",
			errContains: "LIZA_AGENT_ID is required",
		},
		{
			name: "invalid verdict", taskID: "t1", verdict: "MAYBE", agentID: "r1",
			errContains: "must be APPROVED or REJECTED",
		},
		{
			name: "rejection without reason", taskID: "t1", verdict: "REJECTED", agentID: "r1",
			errContains: "rejection reason is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SubmitVerdict("/nonexistent", tt.taskID, tt.verdict, tt.reason, tt.agentID)
			testhelpers.RequireErrorContains(t, err, tt.errContains)
		})
	}
}

func TestSubmitVerdict_VerdictNormalization(t *testing.T) {
	// Lowercase "approved" should be accepted and normalized
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now),
	}
	state.Agents["reviewer-1"] = models.Agent{
		Role:   "reviewer",
		Status: models.AgentStatusWorking,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SubmitVerdict(tmpDir, "task-1", "approved", "", "reviewer-1")
	if err != nil {
		t.Fatalf("SubmitVerdict() error: %v", err)
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "APPROVED")
	}
}

func TestSubmitVerdict_Approved(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now),
	}
	state.Agents["reviewer-1"] = models.Agent{
		Role:   "reviewer",
		Status: models.AgentStatusWorking,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SubmitVerdict(tmpDir, "task-1", "APPROVED", "", "reviewer-1")
	if err != nil {
		t.Fatalf("SubmitVerdict() error: %v", err)
	}

	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "APPROVED")
	}

	// Verify state
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := readState.FindTask("task-1")
	if task == nil {
		t.Fatal("Task not found")
	}
	if task.Status != models.TaskStatusApproved {
		t.Errorf("Status = %v, want APPROVED", task.Status)
	}
	if task.ApprovedBy == nil || *task.ApprovedBy != "reviewer-1" {
		t.Error("ApprovedBy should be reviewer-1")
	}
	if task.RejectionReason != nil {
		t.Error("RejectionReason should be nil after approval")
	}
	if task.ReviewingBy != nil {
		t.Error("ReviewingBy should be cleared")
	}
	if task.ReviewLeaseExpires != nil {
		t.Error("ReviewLeaseExpires should be cleared")
	}

	lastHistory := task.History[len(task.History)-1]
	if lastHistory.Event != "approved" {
		t.Errorf("History event = %q, want %q", lastHistory.Event, "approved")
	}
}

func TestSubmitVerdict_Rejected(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now),
	}
	state.Agents["reviewer-1"] = models.Agent{
		Role:   "reviewer",
		Status: models.AgentStatusWorking,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SubmitVerdict(tmpDir, "task-1", "REJECTED", "Missing error handling", "reviewer-1")
	if err != nil {
		t.Fatalf("SubmitVerdict() error: %v", err)
	}

	if result.Verdict != "REJECTED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "REJECTED")
	}
	if result.Reason != "Missing error handling" {
		t.Errorf("Reason = %q, want %q", result.Reason, "Missing error handling")
	}
	if result.EscalatedToBlocked {
		t.Error("EscalatedToBlocked = true, want false for normal rejection")
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := readState.FindTask("task-1")
	if task == nil {
		t.Fatal("Task not found")
	}
	if task.Status != models.TaskStatusRejected {
		t.Errorf("Status = %v, want REJECTED", task.Status)
	}
	if task.RejectionReason == nil || *task.RejectionReason != "Missing error handling" {
		t.Error("RejectionReason not set correctly")
	}
	if task.ReviewCyclesCurrent != 1 {
		t.Errorf("ReviewCyclesCurrent = %d, want 1", task.ReviewCyclesCurrent)
	}
	if task.ReviewCyclesTotal != 1 {
		t.Errorf("ReviewCyclesTotal = %d, want 1", task.ReviewCyclesTotal)
	}

	lastHistory := task.History[len(task.History)-1]
	if lastHistory.Event != "rejected" {
		t.Errorf("History event = %q, want %q", lastHistory.Event, "rejected")
	}
}

func TestSubmitVerdict_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitVerdict(tmpDir, "nonexistent", "APPROVED", "", "reviewer-1")
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestSubmitVerdict_WrongStatus(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitVerdict(tmpDir, "task-1", "APPROVED", "", "reviewer-1")
	testhelpers.RequireErrorContains(t, err, "not REVIEWING")
}

func TestSubmitVerdict_AgentReleased(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now),
	}
	taskRef := "task-1"
	state.Agents["reviewer-1"] = models.Agent{
		Role:        "reviewer",
		Status:      models.AgentStatusWorking,
		CurrentTask: &taskRef,
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SubmitVerdict(tmpDir, "task-1", "APPROVED", "", "reviewer-1")
	if err != nil {
		t.Fatalf("SubmitVerdict() error: %v", err)
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	agent := readState.Agents["reviewer-1"]
	if agent.Status != models.AgentStatusIdle {
		t.Errorf("Agent status = %v, want idle", agent.Status)
	}
	if agent.CurrentTask != nil {
		t.Error("Agent CurrentTask should be nil after verdict")
	}
}

func TestSubmitVerdict_RejectedLimitEscalationTransitionsToBlocked(t *testing.T) {
	tests := []struct {
		name               string
		rejectionReason    string
		configureStateTask func(*models.State, *models.Task)
		wantReasonContains string
		wantQuestionHint   string
		wantReviewCurrent  int
		wantReviewTotal    int
	}{
		{
			name:            "review cycle limit",
			rejectionReason: "Still failing",
			configureStateTask: func(state *models.State, task *models.Task) {
				state.Config.MaxReviewCycles = 2
				task.ReviewCyclesCurrent = 1
				task.ReviewCyclesTotal = 1
			},
			wantReasonContains: "review deadlock",
			wantQuestionHint:   "review deadlock",
			wantReviewCurrent:  2,
			wantReviewTotal:    2,
		},
		{
			name:            "task iteration limit",
			rejectionReason: "Needs redesign",
			configureStateTask: func(state *models.State, task *models.Task) {
				state.Config.MaxReviewCycles = 5
				state.Config.MaxCoderIterations = 10
				task.Iteration = 2
				task.MaxIterations = 2
			},
			wantReasonContains: "max iterations",
			wantQuestionHint:   "max iterations were exhausted",
			wantReviewCurrent:  1,
			wantReviewTotal:    1,
		},
		{
			name:            "combined limits",
			rejectionReason: "Needs rescope",
			configureStateTask: func(state *models.State, task *models.Task) {
				state.Config.MaxReviewCycles = 2
				state.Config.MaxCoderIterations = 10
				task.ReviewCyclesCurrent = 1
				task.ReviewCyclesTotal = 4
				task.Iteration = 2
				task.MaxIterations = 2
			},
			wantReasonContains: "review deadlock and max iterations",
			wantQuestionHint:   "both review cycles and iterations",
			wantReviewCurrent:  2,
			wantReviewTotal:    5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

			now := time.Now().UTC()
			state := testhelpers.CreateValidState()
			task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
			tt.configureStateTask(state, &task)
			state.Tasks = []models.Task{task}

			taskRef := "task-1"
			state.Agents["coder-1"] = models.Agent{
				Role:        "coder",
				Status:      models.AgentStatusWaiting,
				CurrentTask: &taskRef,
			}
			state.Agents["reviewer-1"] = models.Agent{
				Role:        "reviewer",
				Status:      models.AgentStatusReviewing,
				CurrentTask: &taskRef,
			}

			testhelpers.WriteInitialState(t, stateFile, state)

			result, err := SubmitVerdict(tmpDir, "task-1", "REJECTED", tt.rejectionReason, "reviewer-1")
			if err != nil {
				t.Fatalf("SubmitVerdict() error: %v", err)
			}
			if !result.EscalatedToBlocked {
				t.Error("EscalatedToBlocked = false, want true")
			}
			if !strings.Contains(result.BlockedReason, tt.wantReasonContains) {
				t.Errorf("BlockedReason = %q, want to contain %q", result.BlockedReason, tt.wantReasonContains)
			}

			bb := db.New(stateFile)
			readState, err := bb.Read()
			if err != nil {
				t.Fatalf("Failed to read state: %v", err)
			}

			blockedTask := readState.FindTask("task-1")
			if blockedTask == nil {
				t.Fatal("Task not found")
			}
			if blockedTask.Status != models.TaskStatusBlocked {
				t.Errorf("Status = %v, want BLOCKED", blockedTask.Status)
			}
			if blockedTask.BlockedReason == nil || !strings.Contains(*blockedTask.BlockedReason, tt.wantReasonContains) {
				t.Errorf("BlockedReason = %v, want to contain %q", blockedTask.BlockedReason, tt.wantReasonContains)
			}
			if len(blockedTask.BlockedQuestions) == 0 || !strings.Contains(blockedTask.BlockedQuestions[0], tt.wantQuestionHint) {
				t.Errorf("BlockedQuestions = %v, want first question to contain %q", blockedTask.BlockedQuestions, tt.wantQuestionHint)
			}
			if blockedTask.ReviewCyclesCurrent != tt.wantReviewCurrent {
				t.Errorf("ReviewCyclesCurrent = %d, want %d", blockedTask.ReviewCyclesCurrent, tt.wantReviewCurrent)
			}
			if blockedTask.ReviewCyclesTotal != tt.wantReviewTotal {
				t.Errorf("ReviewCyclesTotal = %d, want %d", blockedTask.ReviewCyclesTotal, tt.wantReviewTotal)
			}
			if blockedTask.AssignedTo != nil {
				t.Error("AssignedTo should be cleared after escalation")
			}
			if blockedTask.ReviewingBy != nil || blockedTask.ReviewLeaseExpires != nil {
				t.Error("Review lease fields should be cleared")
			}

			assertReleasedAgent(t, readState, "coder-1")
			assertReleasedAgent(t, readState, "reviewer-1")
		})
	}
}

func assertReleasedAgent(t *testing.T, state *models.State, agentID string) {
	t.Helper()

	agent := state.Agents[agentID]
	if agent.Status != models.AgentStatusIdle || agent.CurrentTask != nil {
		t.Errorf("%s should be released to IDLE, got status=%v current_task=%v", agentID, agent.Status, agent.CurrentTask)
	}
}
