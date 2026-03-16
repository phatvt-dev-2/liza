package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestBuildBasePrompt(t *testing.T) {
	tests := []struct {
		name           string
		config         BasePromptConfig
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "basic base prompt with all required fields",
			config: BasePromptConfig{
				Role:        "code-coder",
				AgentID:     "coder-1",
				TaskID:      "task-1",
				SpecsDir:    "/project/specs",
				ProjectRoot: "/project",
				StatePath:   "/project/.liza/state.yaml",
				GoalDesc:    "Build a web API",
				GoalSpecRef: "specs/vision.md",
			},
			wantContains: []string{
				"You are a Liza code-coder agent",
				"Agent ID: coder-1",
				"ROLE: code-coder",
				"PROJECT_SPECS: /project/specs",
				"PROJECT: /project",
				"BLACKBOARD: /project/.liza/state.yaml",
				"GOAL: Build a web API",
				"APPROVED: use MCP tools with escalated permissions",
				"TWO .liza/ directories exist",
				"~/.liza/ = installed contracts & skills",
				"/project/.liza/ = runtime state & blackboard",
				"You have FULL read access to both .liza/ directories",
				"For READING state: use liza_get with targeted queries",
				"For MODIFYING state: use role-specific MCP tools",
				"Prefer MCP tools for atomicity and validation",
				"If a required operation has no MCP tool",
				"Execute commands immediately",
				"DO proceed with tool execution",
				"QUERY TOOLS",
				"liza_get",
				"liza_status",
				"liza_validate",
				"COMMUNICATION:",
				"FORBIDDEN:",
				"Do NOT attempt to claim tasks",
				"EXIT CODES:",
				"TIMESTAMPS:",
				"FIRST ACTIONS:",
				`Query your assigned task: liza_get {"query": "tasks/task-1"}`,
				"Read the goal spec: specs/vision.md",
				"lessons/agents/",
				"GUARDRAILS.md",
			},
			wantNotContain: []string{
				// Role-specific tools should NOT be in base prompt
				"liza_add_tasks",
				"liza_submit_for_review",
				"liza_submit_verdict",
				// shared_reference content should NOT be in base prompt
				"TASK STATE MACHINE:",
				"BLACKBOARD FIELDS:",
				"ANOMALY TYPES:",
				"LEASE MODEL:",
				"HELPER COMMANDS",
			},
		},
		{
			name: "role title formatting for multi-word roles",
			config: BasePromptConfig{
				Role:        "code-reviewer",
				AgentID:     "code-reviewer-1",
				SpecsDir:    "/specs",
				ProjectRoot: "/project",
				StatePath:   "/project/.liza/state.yaml",
				GoalDesc:    "Test goal",
				GoalSpecRef: "specs/test.md",
			},
			wantContains: []string{
				"You are a Liza code-reviewer agent",
				"QUERY TOOLS",
			},
		},
		{
			name: "orchestrator role formatting",
			config: BasePromptConfig{
				Role:        "orchestrator",
				AgentID:     "orchestrator-1",
				SpecsDir:    "/specs",
				ProjectRoot: "/project",
				StatePath:   "/project/.liza/state.yaml",
				GoalDesc:    "Test",
				GoalSpecRef: "specs/vision.md",
			},
			wantContains: []string{
				"You are a Liza orchestrator agent",
				"QUERY TOOLS",
				`Query workspace state: liza_get {"query": "tasks"}`,
			},
			wantNotContain: []string{
				"Query your assigned task",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := BuildBasePrompt(tt.config)
			if err != nil {
				t.Fatalf("BuildBasePrompt() error: %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("BuildBasePrompt() missing expected content:\n%q", want)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(result, notWant) {
					t.Errorf("BuildBasePrompt() contains unexpected content:\n%q", notWant)
				}
			}
		})
	}
}

func TestBuildOrchestratorContext(t *testing.T) {
	now := time.Now().UTC()
	projectRoot := setupPipelineConfig(t)

	tests := []struct {
		name         string
		state        *models.State
		config       OrchestratorContextConfig
		wantContains []string
	}{
		{
			name: "initial planning trigger (no tasks)",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{}
				return state
			}(),
			config: OrchestratorContextConfig{ProjectRoot: projectRoot, AgentID: "orchestrator-1"},
			wantContains: []string{
				"=== ORCHESTRATOR CONTEXT ===",
				"WAKE TRIGGER: INITIAL_PLANNING",
				"SPRINT STATE:",
				"- Total tasks: 0",
				"- Merged: 0",
				"- In progress: 0",
				"- Unclaimed: 0",
				"- Blocked: 0",
				"- Integration failed: 0",
				"- Hypothesis exhausted: 0",
				"- Immediate discoveries: 0",
				"ORCHESTRATOR COMMANDS:",
				"liza_add_tasks",
				"liza_assess_blocked",
				"liza_supersede_task",
				`liza_wt_delete`,
				`Tool parameters: {"task_id": "...", "agent_id": "orchestrator-1"}`,
				`liza_sprint_checkpoint — Create sprint checkpoint for human review`,
				`Tool parameters: {"agent_id": "orchestrator-1"}`,
				`liza_update_sprint_metrics — Recompute sprint metrics`,
				`Tool parameters: {"agent_id": "orchestrator-1"}`,
				"This is initial planning",
				"Classify the input document and choose the appropriate entry-point",
				"AVAILABLE ENTRY-POINTS:",
				"Exactly one task is created",
			},
		},
		{
			name: "blocked tasks trigger",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
					testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReady, now),
				}
				return state
			}(),
			config: OrchestratorContextConfig{ProjectRoot: projectRoot, AgentID: "orchestrator-1"},
			wantContains: []string{
				"WAKE TRIGGER: BLOCKED_TASKS",
				"- Total tasks: 2",
				"- Blocked: 1",
				"Tasks are BLOCKED. Analyze and resolve immediately:",
				"Read blocked tasks from blackboard",
				"liza_assess_blocked",
			},
		},
		{
			name: "hypothesis exhaustion trigger",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
				task.FailedBy = []string{"coder-1", "coder-2"}
				state.Tasks = []models.Task{task}
				return state
			}(),
			config: OrchestratorContextConfig{ProjectRoot: projectRoot, AgentID: "orchestrator-1"},
			wantContains: []string{
				"WAKE TRIGGER: HYPOTHESIS_EXHAUSTED",
				"- Hypothesis exhausted: 1",
				"Multiple coders failed on same task. Re-evaluate and act NOW:",
			},
		},
		{
			name: "immediate discovery trigger",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				// Need at least one task to avoid INITIAL_PLANNING trigger
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
				}
				state.Discovered = []models.Discovery{
					{
						ID:             "disc-1",
						By:             "coder-1",
						During:         "task-1",
						Description:    "Critical bug",
						Severity:       "critical",
						Urgency:        "immediate",
						Recommendation: "Fix immediately",
						Created:        now,
					},
				}
				return state
			}(),
			config: OrchestratorContextConfig{ProjectRoot: projectRoot, AgentID: "orchestrator-1"},
			wantContains: []string{
				"WAKE TRIGGER: IMMEDIATE_DISCOVERY",
				"- Immediate discoveries: 1",
				"Urgent discoveries need immediate action:",
			},
		},
		{
			name: "mixed task statuses (in progress calculation)",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
					testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReadyForReview, now),
					testhelpers.BuildTaskByStatus("task-3", models.TaskStatusApproved, now),
					testhelpers.BuildTaskByStatus("task-4", models.TaskStatusMerged, now),
					testhelpers.BuildTaskByStatus("task-5", models.TaskStatusReady, now),
				}
				return state
			}(),
			config: OrchestratorContextConfig{ProjectRoot: projectRoot, AgentID: "orchestrator-1"},
			wantContains: []string{
				"- Total tasks: 5",
				"- Merged: 1",
				"- In progress: 3", // IMPLEMENTING + READY_FOR_REVIEW + APPROVED
				"- Unclaimed: 1",
			},
		},
		{
			name: "planning complete trigger",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Sprint.Scope.Planned = []string{"task-plan-1", "task-code-1"}
				planningTask := testhelpers.BuildTaskByStatus("task-plan-1", models.TaskStatusMerged, now)
				planningTask.RolePair = "code-planning-pair"
				planningTask.Output = []models.OutputEntry{
					{Desc: "Implement auth", DoneWhen: "Auth works", Scope: "internal/auth"},
				}
				codingTask := testhelpers.BuildTaskByStatus("task-code-1", models.TaskStatusMerged, now)
				state.Tasks = []models.Task{planningTask, codingTask}
				return state
			}(),
			config: OrchestratorContextConfig{ProjectRoot: projectRoot, AgentID: "orchestrator-1"},
			wantContains: []string{
				"WAKE TRIGGER: PLANNING_COMPLETE",
				"- Total tasks: 2",
				"- Merged: 2",
				"Planning sprint tasks have been merged with output[] entries",
				"Pipeline transitions are handled automatically by the supervisor",
				`liza_sprint_checkpoint`,
			},
		},
		{
			name: "sprint complete trigger",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Sprint.Scope.Planned = []string{"task-1", "task-2"}
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now),
					testhelpers.BuildTaskByStatus("task-2", models.TaskStatusMerged, now),
				}
				return state
			}(),
			config: OrchestratorContextConfig{ProjectRoot: projectRoot, AgentID: "orchestrator-1"},
			wantContains: []string{
				"WAKE TRIGGER: SPRINT_COMPLETE",
				"- Total tasks: 2",
				"- Merged: 2",
				"All planned sprint tasks have reached terminal state",
				`liza_update_sprint_metrics with {"agent_id": "orchestrator-1"}`,
				`liza_sprint_checkpoint with {"agent_id": "orchestrator-1"}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := BuildOrchestratorContext(tt.state, tt.config)
			if err != nil {
				t.Fatalf("BuildOrchestratorContext() error: %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("BuildOrchestratorContext() missing expected content:\n%q", want)
				}
			}
		})
	}
}

func setupPipelineConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	lizaDir := filepath.Join(dir, ".liza")
	if err := os.MkdirAll(lizaDir, 0o755); err != nil {
		t.Fatalf("mkdir .liza: %v", err)
	}
	yaml := `pipeline:
  agent-roles:
    epic-planner: "Epic Planner"
    epic-plan-reviewer: "Epic Plan Reviewer"
    us-writer: "US Writer"
    us-reviewer: "US Reviewer"
    code-planner: "Code Planner"
    code-plan-reviewer: "Code Plan Reviewer"
    coder: "Coder"
    code-reviewer: "Code Reviewer"
  role-pairs:
    epic-planning-pair:
      doer: epic-planner
      reviewer: epic-plan-reviewer
      states:
        initial: DRAFT_EPIC_PLAN
        executing: EPIC_PLANNING
        submitted: EPIC_PLAN_TO_REVIEW
        reviewing: REVIEWING_EPIC_PLAN
        approved: EPIC_PLAN_APPROVED
        rejected: EPIC_PLAN_REJECTED
    us-writing-pair:
      doer: us-writer
      reviewer: us-reviewer
      states:
        initial: DRAFT_US
        executing: WRITING_US
        submitted: US_READY_FOR_REVIEW
        reviewing: REVIEWING_US
        approved: US_APPROVED
        rejected: US_REJECTED
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
    epic-spec-subpipeline:
      steps:
        - epic-planning-pair
        - us-writing-pair
      transitions:
        - name: epic-to-us
          from: epic-planning-pair.approved
          to: us-writing-pair.initial
          trigger: manual
          cardinality: per-subtask
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
  pipeline-transitions:
    - name: us-to-coding
      from: epic-spec-subpipeline.us-writing-pair.approved
      to: coding-subpipeline.code-planning-pair.initial
      trigger: manual
      cardinality: one-to-one
  entry-points:
    general-objective: epic-spec-subpipeline.epic-planning-pair
    detailed-spec: coding-subpipeline.code-planning-pair
`
	if err := os.WriteFile(filepath.Join(lizaDir, "pipeline.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write pipeline.yaml: %v", err)
	}
	return dir
}

func TestBuildOrchestratorContext_EntryPoints(t *testing.T) {
	tests := []struct {
		name           string
		entryPoint     string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:       "explicit entry-point general-objective dispatches to epic-planning-pair",
			entryPoint: "general-objective",
			wantContains: []string{
				"WAKE TRIGGER: INITIAL_PLANNING",
				"role_pair\": \"epic-planning-pair\"",
				"Epic Planner",
			},
			wantNotContain: []string{
				"classify",
				"code-planning-pair",
			},
		},
		{
			name:       "explicit entry-point detailed-spec dispatches to code-planning-pair",
			entryPoint: "detailed-spec",
			wantContains: []string{
				"WAKE TRIGGER: INITIAL_PLANNING",
				"role_pair\": \"code-planning-pair\"",
				"Code Planner",
			},
			wantNotContain: []string{
				"classify",
				"epic-planning-pair",
			},
		},
		{
			name:       "no entry-point shows classification instructions",
			entryPoint: "",
			wantContains: []string{
				"WAKE TRIGGER: INITIAL_PLANNING",
				"general-objective",
				"detailed-spec",
				"epic-planning-pair",
				"code-planning-pair",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectRoot := setupPipelineConfig(t)

			state := testhelpers.CreateValidState()
			state.Tasks = []models.Task{}
			state.Goal.EntryPoint = tt.entryPoint

			result, err := BuildOrchestratorContext(state, OrchestratorContextConfig{
				ProjectRoot: projectRoot,
				AgentID:     "orchestrator-1",
			})
			if err != nil {
				t.Fatalf("BuildOrchestratorContext() error: %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("missing expected content: %q\n\nFull output:\n%s", want, result)
				}
			}
			for _, notWant := range tt.wantNotContain {
				if strings.Contains(result, notWant) {
					t.Errorf("unexpected content found: %q", notWant)
				}
			}
		})
	}
}

func TestBuildCoderContext(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name         string
		task         *models.Task
		config       CoderContextConfig
		wantContains []string
	}{
		{
			name: "first iteration without rejection",
			task: func() *models.Task {
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
				task.Description = "Implement authentication"
				task.DoneWhen = "Users can login and logout"
				task.Scope = "Add auth module to backend"
				task.Iteration = 1
				worktree := ".worktrees/task-1"
				task.Worktree = &worktree
				return &task
			}(),
			config: CoderContextConfig{
				ProjectRoot: "/project",
				AgentID:     "coder-1",
			},
			wantContains: []string{
				"=== ASSIGNED TASK ===",
				"TASK ID: task-1",
				"WORKTREE: /project/.worktrees/task-1",
				"ITERATION: 1",
				"DESCRIPTION: Implement authentication",
				"DONE WHEN:",
				"Users can login and logout",
				"SCOPE:",
				"Add auth module to backend",
				"CODER STATE TRANSITIONS:",
				"IMPLEMENTING_CODE → CODE_READY_FOR_REVIEW",
				"IMPLEMENTING_CODE → BLOCKED",
				"CODER TOOLS:",
				"liza_submit_for_review",
				"liza_handoff",
				"liza_mark_blocked",
				"ANOMALY LOGGING:",
				"If context exhaustion is near (~90%)",
				"--- IMPLEMENTATION PHASE ---",
				"Work ONLY in the worktree directory. Use git -C /project/.worktrees/task-1 for all git commands.",
				"WORKTREE RULES:",
				"MUST use -C /project/.worktrees/task-1",
				"edit tool tracks reads by string",
				"CREATE new files",
				"COMMIT WORKFLOW:",
				"files were modified by this hook",
				"TDD (code tasks): Write tests FIRST",
				"Tests are MANDATORY for code tasks",
				"Mechanical test updates: For structural refactoring",
				"not trigger the TDD requirement in item 3",
				"--- SUBMISSION (MANDATORY - DO NOT SKIP) ---",
				"you MUST submit for review",
				"git -C /project/.worktrees/task-1 rev-parse HEAD",
				"liza_submit_for_review",
				"\"task_id\": \"task-1\"",
				"\"agent_id\": \"coder-1\"",
				"CODE_READY_FOR_REVIEW",
			},
		},
		{
			name: "handoff resume context is rendered",
			task: func() *models.Task {
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
				task.Description = "Continue parser hardening"
				task.DoneWhen = "All parser edge cases handled"
				task.Scope = "Parser module"
				task.Iteration = 2
				worktree := ".worktrees/task-1"
				task.Worktree = &worktree
				return &task
			}(),
			config: CoderContextConfig{
				ProjectRoot: "/project",
				AgentID:     "coder-1",
				HandoffNote: &models.HandoffNote{
					Agent:      "coder-1",
					Summary:    "Parser support added for nested objects",
					NextAction: "Add malformed payload tests",
				},
			},
			wantContains: []string{
				"=== HANDOFF RESUME CONTEXT ===",
				"FROM: coder-1",
				"SUMMARY: Parser support added for nested objects",
				"NEXT ACTION: Add malformed payload tests",
			},
		},
		{
			name: "second iteration with rejection feedback",
			task: func() *models.Task {
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
				task.Description = "Add validation"
				task.DoneWhen = "All inputs validated"
				task.Scope = "Add validation layer"
				task.Iteration = 2
				rejectionReason := "Missing edge case tests for empty strings"
				task.RejectionReason = &rejectionReason
				worktree := ".worktrees/task-1"
				task.Worktree = &worktree
				return &task
			}(),
			config: CoderContextConfig{
				ProjectRoot: "/project",
				AgentID:     "coder-1",
			},
			wantContains: []string{
				"ITERATION: 2",
				"=== PRIOR REJECTION FEEDBACK (MUST ADDRESS) ===",
				"Missing edge case tests for empty strings",
			},
		},
		{
			name: "first iteration should not show rejection section",
			task: func() *models.Task {
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
				task.Iteration = 1
				worktree := ".worktrees/task-1"
				task.Worktree = &worktree
				return &task
			}(),
			config: CoderContextConfig{
				ProjectRoot: "/project",
				AgentID:     "coder-1",
			},
			wantContains: []string{
				"ITERATION: 1",
			},
		},
		{
			name: "integration fix mode instructions",
			task: func() *models.Task {
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
				task.Description = "Fix merge conflicts in auth module"
				task.DoneWhen = "Auth module merges cleanly"
				task.Scope = "Resolve conflicts"
				task.Iteration = 1
				task.IntegrationFix = true
				worktree := ".worktrees/task-1"
				task.Worktree = &worktree
				return &task
			}(),
			config: CoderContextConfig{
				ProjectRoot:       "/project",
				AgentID:           "coder-1",
				IntegrationBranch: "integration",
			},
			wantContains: []string{
				"=== INTEGRATION FIX MODE ===",
				"failed to merge due to conflicts",
				"worktree is clean",
				"WORKFLOW:",
				"1. FETCH AND REBASE",
				"git -C /project/.worktrees/task-1 fetch /project integration",
				"git -C /project/.worktrees/task-1 rebase FETCH_HEAD",
				"2. RESOLVE CONFLICTS",
				"conflict markers (<<<<<<<, =======, >>>>>>>)",
				"git -C /project/.worktrees/task-1 add <resolved-file>",
				"git -C /project/.worktrees/task-1 rebase --continue",
				"3. IF UNRESOLVABLE",
				"git -C /project/.worktrees/task-1 rebase --abort",
				"liza_mark_blocked",
				"4. VALIDATE",
				"5. SUBMIT",
				"liza_submit_for_review",
				"\"task_id\": \"task-1\"",
				"\"agent_id\": \"coder-1\"",
				"WORKTREE RULES:",
				"edit tool tracks reads by string",
				"CREATE new files",
			},
		},
		{
			name: "integration fix uses MCP tool syntax not CLI commands",
			task: func() *models.Task {
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
				task.IntegrationFix = true
				worktree := ".worktrees/task-1"
				task.Worktree = &worktree
				return &task
			}(),
			config: CoderContextConfig{
				ProjectRoot:       "/project",
				AgentID:           "coder-1",
				IntegrationBranch: "integration",
			},
			wantContains: []string{
				// JSON params syntax for mark-blocked
				"liza_mark_blocked",
				"\"task_id\":",
				"\"agent_id\":",
				"\"reason\":",
				// Submit tool reference
				"liza_submit_for_review",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := BuildCoderContext(tt.task, tt.config)
			if err != nil {
				t.Fatalf("BuildCoderContext() error: %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("BuildCoderContext() missing expected content:\n%q", want)
				}
			}

			// For first iteration, verify rejection section is NOT present
			if tt.task.Iteration == 1 && !tt.task.IntegrationFix {
				if strings.Contains(result, "PRIOR REJECTION FEEDBACK") {
					t.Errorf("BuildCoderContext() should not contain rejection feedback for iteration 1")
				}
			}

			// Verify integration fix section only appears when IntegrationFix=true
			if tt.task.IntegrationFix {
				if !strings.Contains(result, "=== INTEGRATION FIX MODE ===") {
					t.Errorf("BuildCoderContext() should contain integration fix instructions when IntegrationFix=true")
				}
			} else {
				if strings.Contains(result, "=== INTEGRATION FIX MODE ===") {
					t.Errorf("BuildCoderContext() should NOT contain integration fix instructions when IntegrationFix=false")
				}
			}
		})
	}
}

func TestBuildReviewerContext(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name         string
		task         *models.Task
		config       ReviewerContextConfig
		wantContains []string
	}{
		{
			name: "first iteration review",
			task: func() *models.Task {
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
				task.Description = "Add authentication"
				task.DoneWhen = "Users can login"
				assignedTo := "coder-1"
				task.AssignedTo = &assignedTo
				baseCommit := "abc123"
				task.BaseCommit = &baseCommit
				reviewCommit := "def456"
				task.ReviewCommit = &reviewCommit
				task.Iteration = 1
				worktree := ".worktrees/task-1"
				task.Worktree = &worktree
				return &task
			}(),
			config: ReviewerContextConfig{
				ProjectRoot: "/project",
				AgentID:     "code-reviewer-1",
			},
			wantContains: []string{
				"=== REVIEW TASK ===",
				"TASK ID: task-1",
				"WORKTREE: /project/.worktrees/task-1",
				"BASE COMMIT: abc123",
				"REVIEW COMMIT: def456",
				"AUTHOR: coder-1",
				"ITERATION: 1",
				"DESCRIPTION: Add authentication",
				"DONE WHEN:",
				"Users can login",
				"REVIEWER STATE TRANSITIONS:",
				"REVIEWING_CODE → CODE_APPROVED",
				"REVIEWING_CODE → CODE_REJECTED",
				"REVIEWER TOOL:",
				"liza_submit_verdict",
				"ANOMALY LOGGING:",
				"INSTRUCTIONS:",
				"Early drift check",
				"Review ALL changes",
				"Run tests:",
				"Apply the code-review skill",
				"TDD QUALITY (code tasks): Tests must cover done_when criteria",
				"mechanical test updates (file-path strings, import paths) caused by",
				"REJECTION FORMAT (if rejecting):",
				"Blockers:",
				"Concerns:",
				"--- VERDICT SUBMISSION (MANDATORY - DO NOT SKIP) ---",
				"You MUST call the MCP tool liza_submit_verdict IN THIS SAME SESSION",
				"{\"task_id\": \"task-1\", \"verdict\": \"APPROVED\", \"agent_id\": \"code-reviewer-1\"}",
				"{\"task_id\": \"task-1\", \"verdict\": \"REJECTED\", \"agent_id\": \"code-reviewer-1\"",
			},
		},
		{
			name: "second iteration with prior rejection",
			task: func() *models.Task {
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
				task.Description = "Add validation"
				task.DoneWhen = "All inputs validated"
				assignedTo := "coder-1"
				task.AssignedTo = &assignedTo
				baseCommit := "abc123"
				task.BaseCommit = &baseCommit
				reviewCommit := "def456"
				task.ReviewCommit = &reviewCommit
				task.Iteration = 2
				rejectionReason := "Tests incomplete"
				task.RejectionReason = &rejectionReason
				worktree := ".worktrees/task-1"
				task.Worktree = &worktree
				return &task
			}(),
			config: ReviewerContextConfig{
				ProjectRoot: "/project",
				AgentID:     "code-reviewer-1",
			},
			wantContains: []string{
				"ITERATION: 2",
				"=== PRIOR REJECTION (iteration 1) ===",
				"Tests incomplete",
				"PRIOR FEEDBACK REVIEW (MANDATORY for iteration 2+):",
				"Which prior issues are now RESOLVED?",
				"Which prior issues are STILL PRESENT?",
				"Prior Feedback Status:",
				"- RESOLVED:",
				"- STILL PRESENT:",
				"- PARTIAL:",
			},
		},
		{
			name: "third iteration shows correct prior iteration number",
			task: func() *models.Task {
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
				assignedTo := "coder-1"
				task.AssignedTo = &assignedTo
				baseCommit := "abc123"
				task.BaseCommit = &baseCommit
				reviewCommit := "def456"
				task.ReviewCommit = &reviewCommit
				task.Iteration = 3
				rejectionReason := "Still missing tests"
				task.RejectionReason = &rejectionReason
				worktree := ".worktrees/task-1"
				task.Worktree = &worktree
				return &task
			}(),
			config: ReviewerContextConfig{
				ProjectRoot: "/project",
				AgentID:     "code-reviewer-1",
			},
			wantContains: []string{
				"ITERATION: 3",
				"=== PRIOR REJECTION (iteration 2) ===",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := BuildReviewerContext(tt.task, tt.config)
			if err != nil {
				t.Fatalf("BuildReviewerContext() error: %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("BuildReviewerContext() missing expected content:\n%q", want)
				}
			}

			// For first iteration, verify prior rejection section is NOT present
			if tt.task.Iteration == 1 {
				if strings.Contains(result, "PRIOR REJECTION") {
					t.Errorf("BuildReviewerContext() should not contain prior rejection for iteration 1")
				}
				if strings.Contains(result, "PRIOR FEEDBACK REVIEW") {
					t.Errorf("BuildReviewerContext() should not contain prior feedback review for iteration 1")
				}
			}
		})
	}
}

func TestOrchestratorPromptHasAutonomyGuidance(t *testing.T) {
	projectRoot := setupPipelineConfig(t)
	state := &models.State{
		Tasks: []models.Task{},
		Goal:  models.Goal{SpecRef: ".liza/specs/goal.md", Description: "Test goal"},
	}

	config := OrchestratorContextConfig{ProjectRoot: projectRoot, AgentID: "orchestrator-1"}
	prompt, err := BuildOrchestratorContext(state, config)
	if err != nil {
		t.Fatalf("BuildOrchestratorContext() error: %v", err)
	}

	// Verify autonomy banner present
	if !strings.Contains(prompt, "AUTONOMY NOTICE") {
		t.Error("Expected AUTONOMY NOTICE in prompt")
	}

	// Verify success criteria present
	if !strings.Contains(prompt, "SUCCESS CRITERIA") {
		t.Error("Expected SUCCESS CRITERIA in prompt")
	}

	// Verify examples present
	if !strings.Contains(prompt, "EXPECTED BEHAVIOR EXAMPLES") {
		t.Error("Expected EXPECTED BEHAVIOR EXAMPLES in prompt")
	}

	// Verify execution phase marker
	if !strings.Contains(prompt, "--- PLANNING PHASE COMPLETE. NOW EXECUTE. ---") {
		t.Error("Expected planning phase completion marker in prompt")
	}

	// Verify mandatory execution language
	if !strings.Contains(prompt, "EXECUTION PHASE (MANDATORY - DO NOT SKIP)") {
		t.Error("Expected mandatory execution phase header in prompt")
	}

	// Verify no passive language in instructions (outside of negative examples)
	// Split at "INCORRECT" example to check only the instructional part
	parts := strings.Split(prompt, "❌ INCORRECT (DO NOT DO THIS):")
	if len(parts) < 1 {
		t.Error("Expected to find INCORRECT example section in prompt")
	}
	instructionalPart := parts[0]

	passivePhrases := []string{
		"waiting for approval",
		"once approved",
		"should be added",
		"may need to",
	}
	lowerInstructions := strings.ToLower(instructionalPart)
	for _, phrase := range passivePhrases {
		if strings.Contains(lowerInstructions, phrase) {
			t.Errorf("Prompt should not contain passive phrase in instructions: %s", phrase)
		}
	}
}

// TestBasePromptRegressionGuard is a comprehensive regression test for the base prompt.
// The base prompt is the foundation for ALL agent roles. A regression here silently
// degrades every agent in the system. Each section is tested independently so failures
// pinpoint exactly what broke.
func TestBasePromptRegressionGuard(t *testing.T) {
	config := BasePromptConfig{
		Role:        "code-coder",
		AgentID:     "coder-1",
		TaskID:      "task-42",
		SpecsDir:    "/project/specs",
		ProjectRoot: "/project",
		StatePath:   "/project/.liza/state.yaml",
		GoalDesc:    "Build a web API",
		GoalSpecRef: "specs/vision.md",
	}

	prompt, err := BuildBasePrompt(config)
	if err != nil {
		t.Fatalf("BuildBasePrompt() error: %v", err)
	}

	// Helper to check a batch of required phrases with a section label
	assertSection := func(section string, phrases []string) {
		t.Helper()
		for _, phrase := range phrases {
			if !strings.Contains(prompt, phrase) {
				t.Errorf("[%s] missing: %q", section, phrase)
			}
		}
	}

	// Helper to check phrases that must NOT appear
	assertAbsent := func(section string, phrases []string) {
		t.Helper()
		for _, phrase := range phrases {
			if strings.Contains(prompt, phrase) {
				t.Errorf("[%s] must not contain: %q", section, phrase)
			}
		}
	}

	// --- BOOTSTRAP CONTEXT: template variables resolve correctly ---
	assertSection("bootstrap", []string{
		"You are a Liza code-coder agent",
		"Agent ID: coder-1",
		"ROLE: code-coder",
		"PROJECT_SPECS: /project/specs",
		"PROJECT: /project",
		"BLACKBOARD: /project/.liza/state.yaml",
		"GOAL: Build a web API",
	})

	// --- OPERATIONAL RULES: .liza/ directory disambiguation ---
	assertSection("liza-dirs", []string{
		"TWO .liza/ directories exist",
		"~/.liza/ = installed contracts & skills",
		"/project/.liza/ = runtime state & blackboard",
		"FULL read access to both .liza/ directories",
	})

	// --- STATE ACCESS: liza_get over state.yaml ---
	assertSection("state-access", []string{
		"use liza_get with targeted queries",
		"NEVER read state.yaml directly",
		"liza_get returns only the requested slice",
		"Prefer MCP tools for atomicity and validation",
	})

	// --- AUTONOMY: agents must not hesitate ---
	assertSection("autonomy", []string{
		"Your authority is pre-approved",
		"Execute commands immediately",
		"DO proceed with tool execution",
	})

	// --- BASH CONSTRAINTS: universal safety rules ---
	assertSection("bash-constraints", []string{
		"BASH CONSTRAINTS",
		"NEVER combine cd and git in one command",
		"git -C <path> <cmd>",
		"NEVER use $() command substitution",
		"ANSI-C quoting",
		"NEVER attempt to install, bootstrap, or fix system-level tooling",
		`NEVER use "git add -A" or "git add ."`,
		"stage specific files by name",
		"liza_* operations are MCP tool calls",
		"NEVER via shell commands",
	})

	// --- QUERY TOOLS: available to all roles ---
	assertSection("query-tools", []string{
		"QUERY TOOLS",
		"liza_get",
		"liza_status",
		"liza_validate",
	})

	// --- COMMUNICATION: blackboard-only ---
	assertSection("communication", []string{
		"Agents communicate via blackboard only",
		"MCP tools",
		"not direct interaction",
	})

	// --- FORBIDDEN: hard prohibitions ---
	assertSection("forbidden", []string{
		"FORBIDDEN:",
		"Do NOT attempt to claim tasks",
		"Do NOT manually modify task status",
		"Do NOT skip worktrees",
		"Do NOT make architecture decisions",
	})

	// --- EXIT CODES: supervisor protocol ---
	assertSection("exit-codes", []string{
		"EXIT CODES:",
		"Role complete",
		"Graceful abort",
		"Restart with backoff",
	})

	// --- FIRST ACTIONS: boot sequence ---
	assertSection("first-actions", []string{
		"FIRST ACTIONS:",
		`Query your assigned task: liza_get {"query": "tasks/task-42"}`,
		"Read the goal spec: specs/vision.md",
		"lessons/agents/",
		"GUARDRAILS.md",
		"Execute your role's protocol",
	})

	// --- ENVIRONMENT LESSONS ---
	assertSection("env-lessons", []string{
		"ENVIRONMENT LESSONS",
		"lesson-capture skill",
	})

	// --- CODEBASE EXPLORATION: context-saving delegation ---
	assertSection("codebase-exploration", []string{
		"CODEBASE EXPLORATION",
		"AGENT_TOOLS.md",
	})

	// --- NEGATIVE: role-specific content must NOT leak into base ---
	assertAbsent("no-role-leak", []string{
		"liza_add_tasks",
		"liza_submit_for_review",
		"liza_submit_verdict",
		"WORKTREE RULES",
		"IMPLEMENTATION PHASE",
		"REVIEW CHECKLIST",
		"VERDICT SUBMISSION",
	})
}

func TestReviewerPromptHasAutonomyGuidance(t *testing.T) {
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
	task.Description = "Add authentication"
	task.DoneWhen = "Users can login"
	assignedTo := "coder-1"
	task.AssignedTo = &assignedTo
	baseCommit := "abc123"
	task.BaseCommit = &baseCommit
	reviewCommit := "def456"
	task.ReviewCommit = &reviewCommit
	task.Iteration = 1
	worktree := ".worktrees/task-1"
	task.Worktree = &worktree

	config := ReviewerContextConfig{
		ProjectRoot: "/project",
		AgentID:     "code-reviewer-1",
	}
	prompt, err := BuildReviewerContext(&task, config)
	if err != nil {
		t.Fatalf("BuildReviewerContext() error: %v", err)
	}

	// Verify verdict submission section (factorized partial template)
	requiredPhrases := []string{
		"--- VERDICT SUBMISSION (MANDATORY - DO NOT SKIP) ---",
		"liza_submit_verdict is an MCP tool",
		"You MUST call the MCP tool liza_submit_verdict IN THIS SAME SESSION",
		"After submitting verdict, EXIT immediately",
		"FAILURE MODE:",
		"The tool call IS the deliverable",
	}

	for _, phrase := range requiredPhrases {
		if !strings.Contains(prompt, phrase) {
			t.Errorf("Expected autonomy phrase not found in reviewer prompt: %s", phrase)
		}
	}

	// Verify no passive language suggesting waiting
	passivePhrases := []string{
		"waiting for approval",
		"once approved",
		"pending approval",
	}
	for _, phrase := range passivePhrases {
		if strings.Contains(strings.ToLower(prompt), phrase) {
			t.Errorf("Reviewer prompt should not contain passive phrase: %s", phrase)
		}
	}
}

func TestCoderContext_CollectiveScopingRendered(t *testing.T) {
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
	task.Description = "Add auth module"
	task.DoneWhen = "Auth works"
	task.Scope = "Auth module only"
	task.Iteration = 1
	worktree := ".worktrees/task-1"
	task.Worktree = &worktree

	config := CoderContextConfig{
		ProjectRoot:    "/project",
		AgentID:        "coder-1",
		GoalSpecRef:    "specs/vision.md",
		TotalPlanTasks: 3,
		TaskOrdinal:    1,
		SiblingTasks: []SiblingTaskSummary{
			{ID: "task-2", Description: "Add user API", Status: "DRAFT_CODE"},
			{ID: "task-3", Description: "Add tests", Status: "IMPLEMENTING_CODE"},
		},
	}

	result, err := BuildCoderContext(&task, config)
	if err != nil {
		t.Fatalf("BuildCoderContext() error: %v", err)
	}

	wantContains := []string{
		"=== COLLECTIVE PLAN SCOPING ===",
		"1 of 3 in the current sprint",
		"specs/vision.md",
		"Your scope is LIMITED",
		"declare it in scope_extensions with justification",
		"Do NOT silently modify out-of-scope files",
		"spec_gap anomaly",
		"SIBLING TASKS (for context only",
		"task-2: Add user API [DRAFT_CODE]",
		"task-3: Add tests [IMPLEMENTING_CODE]",
	}
	for _, want := range wantContains {
		if !strings.Contains(result, want) {
			t.Errorf("BuildCoderContext() missing expected content: %q", want)
		}
	}
}

func TestCoderContext_NoScopingForSingleTask(t *testing.T) {
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)
	task.Iteration = 1
	worktree := ".worktrees/task-1"
	task.Worktree = &worktree

	config := CoderContextConfig{
		ProjectRoot:    "/project",
		AgentID:        "coder-1",
		GoalSpecRef:    "specs/vision.md",
		TotalPlanTasks: 1,
	}

	result, err := BuildCoderContext(&task, config)
	if err != nil {
		t.Fatalf("BuildCoderContext() error: %v", err)
	}

	if strings.Contains(result, "COLLECTIVE PLAN SCOPING") {
		t.Error("BuildCoderContext() should NOT contain scoping section for single task")
	}
}

func TestReviewerContext_CollectiveScopingRendered(t *testing.T) {
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
	task.Description = "Add auth module"
	task.DoneWhen = "Auth works"
	task.Iteration = 1
	assignedTo := "coder-1"
	task.AssignedTo = &assignedTo
	baseCommit := "abc123"
	task.BaseCommit = &baseCommit
	reviewCommit := "def456"
	task.ReviewCommit = &reviewCommit
	worktree := ".worktrees/task-1"
	task.Worktree = &worktree

	config := ReviewerContextConfig{
		ProjectRoot:    "/project",
		AgentID:        "code-reviewer-1",
		GoalSpecRef:    "specs/vision.md",
		TotalPlanTasks: 2,
		TaskOrdinal:    1,
		SiblingTasks: []SiblingTaskSummary{
			{ID: "task-2", Description: "Add user API", Status: "MERGED"},
		},
	}

	result, err := BuildReviewerContext(&task, config)
	if err != nil {
		t.Fatalf("BuildReviewerContext() error: %v", err)
	}

	wantContains := []string{
		"=== COLLECTIVE PLAN SCOPING ===",
		"1 of 2 in the current sprint",
		"specs/vision.md",
		"Verify the implementation stays within scope",
		"flag scope creep as a blocker",
		"SIBLING TASKS (for scope boundary awareness)",
		"task-2: Add user API [MERGED]",
	}
	for _, want := range wantContains {
		if !strings.Contains(result, want) {
			t.Errorf("BuildReviewerContext() missing expected content: %q", want)
		}
	}
}

func TestReviewerContext_ScopeExtensionsRendered(t *testing.T) {
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
	task.Description = "Add auth module"
	task.DoneWhen = "Auth works"
	task.Iteration = 1
	assignedTo := "coder-1"
	task.AssignedTo = &assignedTo
	baseCommit := "abc123"
	task.BaseCommit = &baseCommit
	reviewCommit := "def456"
	task.ReviewCommit = &reviewCommit
	worktree := ".worktrees/task-1"
	task.Worktree = &worktree
	// Add checkpoint with scope extensions to history
	task.History = append(task.History, models.TaskHistoryEntry{
		Time:  now,
		Event: "pre_execution_checkpoint",
		Agent: &assignedTo,
		Extra: map[string]any{
			"intent":          "Add auth",
			"validation_plan": "go test ./...",
			"files_to_modify": []string{"auth.go"},
			"scope_extensions": []map[string]string{
				{"file": "internal/utils/hash.go", "justification": "Need password hashing helper"},
				{"file": "go.mod", "justification": "Add bcrypt dependency"},
			},
		},
	})

	config := ReviewerContextConfig{
		ProjectRoot:    "/project",
		AgentID:        "code-reviewer-1",
		TotalPlanTasks: 2,
		TaskOrdinal:    1,
		SiblingTasks: []SiblingTaskSummary{
			{ID: "task-2", Description: "Add user API", Status: "DRAFT_CODE"},
		},
	}

	result, err := BuildReviewerContext(&task, config)
	if err != nil {
		t.Fatalf("BuildReviewerContext() error: %v", err)
	}

	wantContains := []string{
		"SCOPE EXTENSIONS (declared by coder):",
		"internal/utils/hash.go",
		"Need password hashing helper",
		"go.mod",
		"Add bcrypt dependency",
		"Evaluate each extension",
		"Reject if the extension is unjustified",
	}
	for _, want := range wantContains {
		if !strings.Contains(result, want) {
			t.Errorf("BuildReviewerContext() missing expected content: %q", want)
		}
	}
}

func TestReviewerContext_NoScopeExtensionsWhenAbsent(t *testing.T) {
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
	task.Iteration = 1
	assignedTo := "coder-1"
	task.AssignedTo = &assignedTo
	baseCommit := "abc123"
	task.BaseCommit = &baseCommit
	reviewCommit := "def456"
	task.ReviewCommit = &reviewCommit
	worktree := ".worktrees/task-1"
	task.Worktree = &worktree

	config := ReviewerContextConfig{
		ProjectRoot: "/project",
		AgentID:     "code-reviewer-1",
	}

	result, err := BuildReviewerContext(&task, config)
	if err != nil {
		t.Fatalf("BuildReviewerContext() error: %v", err)
	}

	if strings.Contains(result, "SCOPE EXTENSIONS") {
		t.Error("BuildReviewerContext() should NOT contain SCOPE EXTENSIONS when none declared")
	}
}

func TestReviewerContext_NoScopingForSingleTask(t *testing.T) {
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)
	task.Iteration = 1
	assignedTo := "coder-1"
	task.AssignedTo = &assignedTo
	baseCommit := "abc123"
	task.BaseCommit = &baseCommit
	reviewCommit := "def456"
	task.ReviewCommit = &reviewCommit
	worktree := ".worktrees/task-1"
	task.Worktree = &worktree

	config := ReviewerContextConfig{
		ProjectRoot:    "/project",
		AgentID:        "code-reviewer-1",
		GoalSpecRef:    "specs/vision.md",
		TotalPlanTasks: 1,
	}

	result, err := BuildReviewerContext(&task, config)
	if err != nil {
		t.Fatalf("BuildReviewerContext() error: %v", err)
	}

	if strings.Contains(result, "COLLECTIVE PLAN SCOPING") {
		t.Error("BuildReviewerContext() should NOT contain scoping section for single task")
	}
}

func TestCoderContext_CollectiveScopingNonFirstTask(t *testing.T) {
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-3", models.TaskStatusImplementing, now)
	task.Description = "Add tests"
	task.DoneWhen = "Tests pass"
	task.Scope = "Test module"
	task.Iteration = 1
	worktree := ".worktrees/task-3"
	task.Worktree = &worktree

	config := CoderContextConfig{
		ProjectRoot:    "/project",
		AgentID:        "coder-1",
		GoalSpecRef:    "specs/vision.md",
		TotalPlanTasks: 3,
		TaskOrdinal:    3,
		SiblingTasks: []SiblingTaskSummary{
			{ID: "task-1", Description: "Add auth", Status: "MERGED"},
			{ID: "task-2", Description: "Add user API", Status: "IMPLEMENTING_CODE"},
		},
	}

	result, err := BuildCoderContext(&task, config)
	if err != nil {
		t.Fatalf("BuildCoderContext() error: %v", err)
	}

	if !strings.Contains(result, "3 of 3 in the current sprint") {
		t.Error("BuildCoderContext() should show correct ordinal for non-first task")
	}
	if strings.Contains(result, "1 of 3") {
		t.Error("BuildCoderContext() should NOT hardcode ordinal to 1")
	}
}

func TestReviewerContext_CollectiveScopingNonFirstTask(t *testing.T) {
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReadyForReview, now)
	task.Description = "Add user API"
	task.DoneWhen = "API works"
	task.Iteration = 1
	assignedTo := "coder-1"
	task.AssignedTo = &assignedTo
	baseCommit := "abc123"
	task.BaseCommit = &baseCommit
	reviewCommit := "def456"
	task.ReviewCommit = &reviewCommit
	worktree := ".worktrees/task-2"
	task.Worktree = &worktree

	config := ReviewerContextConfig{
		ProjectRoot:    "/project",
		AgentID:        "code-reviewer-1",
		GoalSpecRef:    "specs/vision.md",
		TotalPlanTasks: 3,
		TaskOrdinal:    2,
		SiblingTasks: []SiblingTaskSummary{
			{ID: "task-1", Description: "Add auth", Status: "MERGED"},
			{ID: "task-3", Description: "Add tests", Status: "DRAFT_CODE"},
		},
	}

	result, err := BuildReviewerContext(&task, config)
	if err != nil {
		t.Fatalf("BuildReviewerContext() error: %v", err)
	}

	if !strings.Contains(result, "2 of 3 in the current sprint") {
		t.Error("BuildReviewerContext() should show correct ordinal for non-first task")
	}
	if strings.Contains(result, "1 of 3") {
		t.Error("BuildReviewerContext() should NOT hardcode ordinal to 1")
	}
}

func TestOrchestratorPromptAutonomyForAllWakeTriggers(t *testing.T) {
	now := time.Now().UTC()
	projectRoot := setupPipelineConfig(t)

	tests := []struct {
		name         string
		state        *models.State
		wantTrigger  string
		wantContains []string
	}{
		{
			name: "BLOCKED_TASKS has immediate action language",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
				}
				return state
			}(),
			wantTrigger: "BLOCKED_TASKS",
			wantContains: []string{
				"Analyze and resolve immediately",
				"execute liza_add_tasks tool NOW",
				"fallback state edit + liza_validate",
				"execute tools NOW",
				"Execute all state-modifying tools in this session",
				"Do NOT defer",
				"Do NOT call liza_sprint_checkpoint",
			},
		},
		{
			name: "HYPOTHESIS_EXHAUSTED has immediate action language",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
				task.FailedBy = []string{"coder-1", "coder-2"}
				state.Tasks = []models.Task{task}
				return state
			}(),
			wantTrigger: "HYPOTHESIS_EXHAUSTED",
			wantContains: []string{
				"Re-evaluate and act NOW",
				"execute NOW",
				"update NOW",
				"Execute changes",
				"create them all in this session",
				"All state modifications must be executed before you exit",
				"Do NOT call liza_sprint_checkpoint",
			},
		},
		{
			name: "IMMEDIATE_DISCOVERY has immediate action language",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Tasks = []models.Task{
					testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
				}
				state.Discovered = []models.Discovery{
					{
						ID:             "disc-1",
						By:             "coder-1",
						During:         "task-1",
						Description:    "Critical issue",
						Severity:       "critical",
						Urgency:        "immediate",
						Recommendation: "Fix now",
						Created:        now,
					},
				}
				return state
			}(),
			wantTrigger: "IMMEDIATE_DISCOVERY",
			wantContains: []string{
				"Urgent discoveries need immediate action",
				"execute decision NOW",
				"execute liza_add_tasks tool NOW",
				"fallback state edit + liza_validate",
				"All discovered items must be processed and all tools executed in this session",
				"Do NOT call liza_sprint_checkpoint",
			},
		},
		{
			name: "PLANNING_COMPLETE has checkpoint language",
			state: func() *models.State {
				state := testhelpers.CreateValidState()
				state.Sprint.Scope.Planned = []string{"task-plan-1", "task-code-1"}
				planningTask := testhelpers.BuildTaskByStatus("task-plan-1", models.TaskStatusMerged, now)
				planningTask.RolePair = "code-planning-pair"
				planningTask.Output = []models.OutputEntry{
					{Desc: "Implement feature", DoneWhen: "Feature works", Scope: "internal/"},
				}
				codingTask := testhelpers.BuildTaskByStatus("task-code-1", models.TaskStatusMerged, now)
				state.Tasks = []models.Task{planningTask, codingTask}
				return state
			}(),
			wantTrigger: "PLANNING_COMPLETE",
			wantContains: []string{
				"Planning sprint tasks have been merged with output[] entries",
				"Pipeline transitions are handled automatically by the supervisor",
				"FULL autonomy to execute MCP tools immediately",
				"liza_sprint_checkpoint",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := OrchestratorContextConfig{ProjectRoot: projectRoot, AgentID: "orchestrator-1"}
			prompt, err := BuildOrchestratorContext(tt.state, config)
			if err != nil {
				t.Fatalf("BuildOrchestratorContext() error: %v", err)
			}

			// Verify correct trigger
			if !strings.Contains(prompt, "WAKE TRIGGER: "+tt.wantTrigger) {
				t.Errorf("Expected wake trigger %s not found", tt.wantTrigger)
			}

			// Verify all required action-oriented phrases
			for _, phrase := range tt.wantContains {
				if !strings.Contains(prompt, phrase) {
					t.Errorf("Missing expected action-oriented phrase: %s", phrase)
				}
			}
		})
	}
}

func TestBuildCodePlannerContext(t *testing.T) {
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-plan-1", models.TaskStatusCodePlanning, now)
	task.Description = "Create implementation plan"
	task.DoneWhen = "Plan covers all required changes"
	task.Scope = "Runtime wiring"
	task.Iteration = 1
	worktree := ".worktrees/task-plan-1"
	task.Worktree = &worktree

	cfg := CodePlannerContextConfig{
		ProjectRoot: "/project",
		AgentID:     "code-planner-1",
	}

	prompt, err := BuildCodePlannerContext(&task, cfg)
	if err != nil {
		t.Fatalf("BuildCodePlannerContext() error: %v", err)
	}

	wantContains := []string{
		"ASSIGNED CODE PLANNING TASK",
		"TASK ID: task-plan-1",
		"WORKTREE: /project/.worktrees/task-plan-1",
		"liza_submit_for_review",
		"liza_write_checkpoint",
		"REFACTORING TASKS:",
		"done_when MUST distinguish behavioral test changes (not expected) from mechanical test",
		"WORKTREE RULES",
		"edit tool tracks reads by string",
		"CREATE new files",
	}
	for _, want := range wantContains {
		if !strings.Contains(prompt, want) {
			t.Errorf("BuildCodePlannerContext() missing %q", want)
		}
	}
}

func TestBuildCodePlanReviewerContext(t *testing.T) {
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-plan-1", models.TaskStatusReviewingCodingPlan, now)
	task.Description = "Create implementation plan"
	task.Iteration = 1
	reviewCommit := "abc123"
	task.ReviewCommit = &reviewCommit
	worktree := ".worktrees/task-plan-1"
	task.Worktree = &worktree
	assigned := "code-planner-1"
	task.AssignedTo = &assigned

	cfg := CodePlanReviewerContextConfig{
		ProjectRoot: "/project",
		AgentID:     "code-plan-reviewer-1",
	}

	prompt, err := BuildCodePlanReviewerContext(&task, cfg)
	if err != nil {
		t.Fatalf("BuildCodePlanReviewerContext() error: %v", err)
	}

	wantContains := []string{
		"ASSIGNED CODE PLAN REVIEW TASK",
		"TASK ID: task-plan-1",
		"WORKTREE: /project/.worktrees/task-plan-1",
		"REVIEW COMMIT: abc123",
		"liza_submit_verdict",
	}
	for _, want := range wantContains {
		if !strings.Contains(prompt, want) {
			t.Errorf("BuildCodePlanReviewerContext() missing %q", want)
		}
	}
}

func TestBuildEpicPlanReviewerContext(t *testing.T) {
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-epic-review-1", "REVIEWING_EPIC_PLAN", now)
	task.Description = "Review epic decomposition for auth system"
	task.Iteration = 1
	reviewCommit := "def456"
	task.ReviewCommit = &reviewCommit
	worktree := ".worktrees/task-epic-review-1"
	task.Worktree = &worktree
	assigned := "epic-planner-1"
	task.AssignedTo = &assigned

	cfg := EpicPlanReviewerContextConfig{
		ProjectRoot: "/project",
		AgentID:     "epic-plan-reviewer-1",
	}

	prompt, err := BuildEpicPlanReviewerContext(&task, cfg)
	if err != nil {
		t.Fatalf("BuildEpicPlanReviewerContext() error: %v", err)
	}

	wantContains := []string{
		// Header fields
		"ASSIGNED EPIC PLAN REVIEW TASK",
		"TASK ID: task-epic-review-1",
		"WORKTREE: /project/.worktrees/task-epic-review-1",
		"REVIEW COMMIT: def456",
		"AUTHOR: epic-planner-1",

		// State transitions
		"REVIEWING_EPIC_PLAN",
		"EPIC_PLAN_APPROVED",
		"EPIC_PLAN_REJECTED",

		// Tool
		"liza_submit_verdict",
		`"task_id": "task-epic-review-1"`,
		`"agent_id": "epic-plan-reviewer-1"`,

		// All 6 review gates from spec
		"Cohesive capability",
		"Right-sized scope",
		"Falsifiable done_when",
		"Persona coherence",
		"Independence",
		"Vision coverage",
	}
	for _, want := range wantContains {
		if !strings.Contains(prompt, want) {
			t.Errorf("BuildEpicPlanReviewerContext() missing %q", want)
		}
	}
}

func TestBuildEpicPlannerContext(t *testing.T) {
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("task-epic-1", models.TaskStatusImplementing, now)
	task.Description = "Decompose vision into epics"
	task.DoneWhen = "Epics cover all vision capabilities"
	task.Scope = "Epic decomposition"
	task.Iteration = 1
	worktree := ".worktrees/task-epic-1"
	task.Worktree = &worktree

	cfg := EpicPlannerContextConfig{
		ProjectRoot: "/project",
		AgentID:     "epic-planner-1",
	}

	prompt, err := BuildEpicPlannerContext(&task, cfg)
	if err != nil {
		t.Fatalf("BuildEpicPlannerContext() error: %v", err)
	}

	wantContains := []string{
		// Task details
		"ASSIGNED EPIC PLANNING TASK",
		"TASK ID: task-epic-1",
		"WORKTREE: /project/.worktrees/task-epic-1",
		"ITERATION: 1",
		"DESCRIPTION: Decompose vision into epics",

		// State transitions
		"EPIC_PLANNING → EPIC_PLAN_TO_REVIEW",
		"EPIC_PLAN_REJECTED → DRAFT_EPIC_PLAN",

		// Tools
		"liza_submit_for_review",
		"liza_write_checkpoint",
		"liza_set_task_output",
		"liza_mark_blocked",

		// All 7 granularity signals from spec
		"Epic spans multiple independent capabilities",
		"Epic would produce >8 user stories",
		"done_when requires outcomes across unrelated subsystems",
		"Epic description contains conjunctions joining unrelated capabilities",
		"Epic is a single user action with one acceptance criterion",
		"Epic can be implemented in a single coding task",
		"Epic would produce <2 meaningful user stories",

		// Right-sized epic criteria
		"one cohesive capability area",
		"3–8 user stories",

		// Worktree rules
		"WORKTREE RULES",
		"MUST use -C",
		"edit tool tracks reads by string",
		"CREATE new files",

		// Implementation phase
		"IMPLEMENTATION PHASE",
	}
	for _, want := range wantContains {
		if !strings.Contains(prompt, want) {
			t.Errorf("BuildEpicPlannerContext() missing %q", want)
		}
	}
}

func TestBuildUSWriterContext(t *testing.T) {
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("us-task-1", models.TaskStatusImplementing, now)
	task.Description = "Write user stories for authentication capability"
	task.DoneWhen = "User stories cover all authentication requirements"
	task.Scope = "Authentication capability"
	task.Iteration = 1
	worktree := ".worktrees/us-task-1"
	task.Worktree = &worktree

	cfg := USWriterContextConfig{
		ProjectRoot: "/project",
		AgentID:     "us-writer-1",
	}

	prompt, err := BuildUSWriterContext(&task, cfg)
	if err != nil {
		t.Fatalf("BuildUSWriterContext() error: %v", err)
	}

	wantContains := []string{
		"ASSIGNED US WRITING TASK",
		"TASK ID: us-task-1",
		"WORKTREE: /project/.worktrees/us-task-1",
		"ITERATION: 1",
		"DESCRIPTION: Write user stories for authentication capability",
		// State transitions
		"WRITING_US",
		"US_READY_FOR_REVIEW",
		"US_REJECTED",
		"DRAFT_US",
		// Tools
		"liza_submit_for_review",
		"liza_write_checkpoint",
		"liza_mark_blocked",
		// user-story-writing skill reference
		"user-story-writing",
		"~/.liza/skills/user-story-writing/SKILL.md",
		// SMARC criteria
		"SMARC",
		// Worktree rules
		"WORKTREE RULES",
		"MUST use -C",
		"edit tool tracks reads by string",
		"CREATE new files",
	}
	for _, want := range wantContains {
		if !strings.Contains(prompt, want) {
			t.Errorf("BuildUSWriterContext() missing %q", want)
		}
	}

	// Should NOT contain tools that don't apply to US Writer
	wantNotContain := []string{
		"liza_set_task_output", // one-to-one cardinality, no output[] needed
		"liza_submit_verdict",  // not a reviewer
	}
	for _, notWant := range wantNotContain {
		if strings.Contains(prompt, notWant) {
			t.Errorf("BuildUSWriterContext() should not contain %q", notWant)
		}
	}
}

func TestBuildUSReviewerContext(t *testing.T) {
	now := time.Now().UTC()
	task := testhelpers.BuildTaskByStatus("us-review-1", models.TaskStatusImplementing, now)
	task.Description = "Review user stories for authentication capability"
	task.DoneWhen = "User stories meet quality gates and SMARC criteria"
	task.Iteration = 1
	worktree := ".worktrees/us-review-1"
	task.Worktree = &worktree
	reviewCommit := "def456"
	task.ReviewCommit = &reviewCommit
	assignedTo := "us-writer-1"
	task.AssignedTo = &assignedTo

	cfg := USReviewerContextConfig{
		ProjectRoot: "/project",
		AgentID:     "us-reviewer-1",
	}

	prompt, err := BuildUSReviewerContext(&task, cfg)
	if err != nil {
		t.Fatalf("BuildUSReviewerContext() error: %v", err)
	}

	wantContains := []string{
		// Task details
		"ASSIGNED US REVIEW TASK",
		"TASK ID: us-review-1",
		"WORKTREE: /project/.worktrees/us-review-1",
		"REVIEW COMMIT: def456",
		"AUTHOR: us-writer-1",
		"ITERATION: 1",
		"DESCRIPTION: Review user stories for authentication capability",
		// State transitions
		"REVIEWING_US",
		"US_APPROVED",
		"US_REJECTED",
		// Tool
		"liza_submit_verdict",
		"us-review-1",
		"us-reviewer-1",
		// spec-review skill reference
		"spec-review",
		"~/.liza/skills/spec-review/SKILL.md",
		// user-story-writing skill reference
		"user-story-writing",
		"~/.liza/skills/user-story-writing/SKILL.md",
		// Anti-patterns
		"Persona Laundering",
		"Giant Story",
		"Wishful Story",
		"Hidden Coupling",
		"Assumption Burial",
		"Scope Absorption",
		"Premature Solutioning",
		"Generic Persona",
		"Valueless Story",
		// Quality gates
		"SMARC",
		"Acceptance criteria",
	}
	for _, want := range wantContains {
		if !strings.Contains(prompt, want) {
			t.Errorf("BuildUSReviewerContext() missing %q", want)
		}
	}

	// Should NOT contain tools that don't apply to US Reviewer
	wantNotContain := []string{
		"liza_submit_for_review", // not a doer
		"liza_write_checkpoint",  // not a doer
		"liza_set_task_output",   // not a doer
		"liza_mark_blocked",      // not a doer
	}
	for _, notWant := range wantNotContain {
		if strings.Contains(prompt, notWant) {
			t.Errorf("BuildUSReviewerContext() should not contain %q", notWant)
		}
	}
}
