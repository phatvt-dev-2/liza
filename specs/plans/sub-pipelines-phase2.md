# Sub-pipelines Phase 2 — Implementation Plan

## Overview

Phase 2 adds the epic-spec sub-pipeline (epic-planning-pair + us-writing-pair),
extends the pipeline YAML with `pipeline-transitions` for cross-sub-pipeline linking,
adds `one-to-one` cardinality support in `liza proceed`, and updates the Orchestrator
to dispatch based on entry-points.

**Spec reference:** `specs/build/2 - Sub-pipelines and spec writing.md` §Phase 2

## Architecture Impact

### Current State (Phase 1)

- Pipeline config: 2 role-pairs (code-planning-pair, coding-pair), 1 sub-pipeline, 1 entry-point
- Transitions: intra-sub-pipeline only (`code-plan-to-coding` within coding-subpipeline)
- Roles: 5 (orchestrator, code-planner, code-plan-reviewer, coder, code-reviewer)
- `liza proceed`: `per-subtask` cardinality only; 2-part refs (`role-pair.phase`)
- Supervisor: hardcoded role switches for 5 roles

### Target State (Phase 2)

- Pipeline config: 4 role-pairs, 2 sub-pipelines, 2 entry-points, 1 pipeline-transition
- Transitions: intra-sub-pipeline + cross-sub-pipeline (`us-to-coding` between sub-pipelines)
- Roles: 9 (+ epic-planner, epic-plan-reviewer, us-writer, us-reviewer)
- `liza proceed`: `per-subtask` + `one-to-one` cardinality; 2-part and 3-part refs
- Supervisor: extended for 9 roles
- Orchestrator: entry-point-aware dispatch (LLM classification or `--entry-point` override)

### Key Design Decisions

1. **3-part refs for pipeline-transitions**: Cross-sub-pipeline transitions use
   `<sub-pipeline>.<role-pair>.<phase>` to unambiguously reference states across
   sub-pipeline boundaries. The sub-pipeline prefix is stripped during status resolution
   (the role-pair name is already globally unique per the spec constraint).

2. **`one-to-one` cardinality**: For `us-to-coding`, the parent US task IS the input
   (no `output[]` needed). The child task's `desc` and `scope` are generated from the
   parent; `spec_ref` points to the parent's artifact; `parent_task` links back.

3. **Role switches extended, not generalized**: Phase 1 established the pattern of
   role-specific switches in supervisor. Phase 2 adds 4 new cases following the same
   pattern. Full generalization (deriving doer/reviewer from pipeline config at runtime)
   is noted as tech debt but not required for Phase 2 correctness.

4. **Status check generalization**: `handleApprovedMerges` and related functions
   currently hardcode `TaskStatusApproved || TaskStatusCodingPlanApproved`. Phase 2
   generalizes these to use the pipeline resolver, avoiding N hardcoded checks per new
   role-pair. This is the ONE generalization that's required (otherwise every future
   role-pair addition breaks merge handling).

---

## Task Decomposition

### Task 1: Add `pipeline-transitions` to YAML schema and validation

**Intent:** Pipeline config supports cross-sub-pipeline transitions with 3-part refs.

**Files to modify:**
- `internal/pipeline/config.go`
- `internal/pipeline/config_test.go`

**Changes:**
- Add `PipelineTransitions []TransitionDef` field to `Pipeline` struct with YAML tag `pipeline-transitions`
- Add `parse3PartRef(ref string) (subPipeline, rolePair, phase string, err error)` for refs like `epic-spec-subpipeline.us-writing-pair.approved`
- Add `validatePipelineTransition(t TransitionDef, p *Pipeline) error`:
  - Parse `from` as 3-part ref; validate sub-pipeline exists, role-pair is a step of that sub-pipeline, phase is valid
  - Parse `to` as 3-part ref; same validation
  - Validate `from` and `to` reference DIFFERENT sub-pipelines (cross-sub-pipeline constraint)
  - Validate trigger and cardinality (same as `validateTransition`)
- Extend transition name uniqueness check in `validate()` to include pipeline-transitions
- Add validation tests with Phase 2 YAML fixture

**Done when:**
- `Load()` successfully parses the Phase 2 YAML (with `pipeline-transitions` section)
- Validation rejects: invalid 3-part refs, same sub-pipeline in from/to, duplicate transition names across sub-pipeline and pipeline-transitions
- Tests pass

**Scope:** `internal/pipeline/config.go`, `internal/pipeline/config_test.go`

**Depends:** none

---

### Task 2: Extend Resolver to search pipeline-transitions

**Intent:** Resolver finds and resolves transitions declared in `pipeline-transitions` (not just sub-pipeline transitions).

**Files to modify:**
- `internal/pipeline/resolver.go`
- `internal/pipeline/resolver_test.go`

**Changes:**
- `Transition(name)`: after searching sub-pipeline transitions, also search `config.Pipeline.PipelineTransitions`
- `AvailableTransitions(status, executed)`: also iterate `config.Pipeline.PipelineTransitions` for manual transitions matching the given status. For 3-part refs, strip the sub-pipeline prefix before resolving to a status.
- `SprintTerminalStates()`: also consider pipeline-transition sources. A role-pair whose approved state is the `from` of a pipeline-transition is a transition source → its approved state is sprint-terminal.
- Add `resolve3PartPhase(ref string) models.TaskStatus` helper: parses 3-part ref, extracts role-pair and phase, calls `resolvePhase()`.
- Tests verify: Transition() finds `us-to-coding`, AvailableTransitions returns it for US_APPROVED status, SprintTerminalStates includes US_APPROVED

**Done when:**
- `resolver.Transition("us-to-coding")` returns the pipeline-transition definition
- `resolver.AvailableTransitions(US_APPROVED, nil)` returns `["us-to-coding"]`
- `resolver.SprintTerminalStates()` includes `US_APPROVED` and `EPIC_PLAN_APPROVED`
- Tests pass

**Scope:** `internal/pipeline/resolver.go`, `internal/pipeline/resolver_test.go`

**Depends:** Task 1

---

### Task 3: Add Phase 2 roles (epic-planner, epic-plan-reviewer, us-writer, us-reviewer)

**Intent:** Role infrastructure recognizes the 4 new roles with correct runtime/workflow mapping.

**Files to modify:**
- `internal/roles/roles.go`
- `internal/roles/roles_test.go`
- `internal/models/state.go`

**Changes:**
- `roles.go`: Add constants:
  ```
  RuntimeEpicPlanner      = "epic-planner"
  RuntimeEpicPlanReviewer  = "epic-plan-reviewer"
  RuntimeUSWriter          = "us-writer"
  RuntimeUSReviewer        = "us-reviewer"
  WorkflowEpicPlanner      = "epic_planner"
  WorkflowEpicPlanReviewer  = "epic_plan_reviewer"
  WorkflowUSWriter          = "us_writer"
  WorkflowUSReviewer        = "us_reviewer"
  ```
- Update `runtimeToWorkflow`, `workflowToRuntime` maps
- Update `AllRuntime()`, `AllWorkflow()` lists
- `state.go`: Add model role constants:
  ```
  RoleEpicPlanner      = roles.WorkflowEpicPlanner
  RoleEpicPlanReviewer = roles.WorkflowEpicPlanReviewer
  RoleUSWriter         = roles.WorkflowUSWriter
  RoleUSReviewer       = roles.WorkflowUSReviewer
  ```
- Add test coverage for new role conversions

**Done when:**
- `ToWorkflow("epic-planner")` → `"epic_planner"` (and all 4 pairs)
- `IsValidRuntime("us-writer")` → true
- `AllRuntime()` returns 9 roles
- Tests pass

**Scope:** `internal/roles/roles.go`, `internal/roles/roles_test.go`, `internal/models/state.go`

**Depends:** none

---

### Task 4: Implement `one-to-one` cardinality and 3-part ref resolution in proceed

**Intent:** `liza proceed` handles `one-to-one` cardinality and resolves 3-part refs for cross-sub-pipeline transitions.

**Files to modify:**
- `internal/ops/proceed.go`
- `internal/ops/proceed_test.go`

**Changes:**
- `resolvePhaseRef()`: Handle both 2-part (`role-pair.phase`) and 3-part (`sub-pipeline.role-pair.phase`) refs. For 3-part refs, extract the last two parts as role-pair and phase.
- `Proceed()`: Add `one-to-one` branch:
  - No `output[]` required (validation: reject if `output[]` is non-empty for one-to-one? Or just ignore it — spec says "no output[] needed")
  - Child ID: `<parent-id>-<transition-name>` (no index suffix)
  - Child task fields:
    - `desc`: `"<target-role-pair-doer-display-name> task for: <parent.Description>"` (e.g., "Code Planner task for: User authentication story")
    - `done_when`: inherited from transition semantics (generic for role-pair)
    - `scope`: `"Based on parent task <parent-id>"`
    - `spec_ref`: parent task's `spec_ref`
    - `parent_task`: source task ID
    - `role_pair`: target role-pair
    - `status`: target pair's initial state
  - Crash recovery: check if child already exists
- `allTransitionNames()`: also collect from `cfg.Pipeline.PipelineTransitions`
- Update `proceed` command help text to be dynamic or remove hardcoded transition list
- Tests for one-to-one: creates single child, correct ID format, crash recovery

**Done when:**
- `liza proceed <us-task> us-to-coding` creates one child task with correct fields
- Child ID follows `<parent>-us-to-coding` pattern (no index)
- `resolvePhaseRef` handles 3-part ref `epic-spec-subpipeline.us-writing-pair.approved`
- Crash recovery: re-running returns "already executed" if child exists
- Tests pass

**Scope:** `internal/ops/proceed.go`, `internal/ops/proceed_test.go`, `cmd/liza/main.go` (help text)

**Depends:** Task 2

---

### Task 5: Generalize approved/submitted status checks to pipeline-aware

**Intent:** Merge handling and submission logging work for any pipeline role-pair without hardcoded status lists.

**Files to modify:**
- `internal/agent/claiming.go`
- `internal/agent/claiming_test.go` (if exists)

**Changes:**
- `handleApprovedMerges()`: Replace hardcoded `task.Status == TaskStatusApproved || task.Status == TaskStatusCodingPlanApproved` with pipeline-aware check:
  ```go
  isApproved := false
  if task.RolePair != "" && pr != nil {
      approvedStatus, err := pr.ApprovedStatus(task.RolePair)
      if err == nil && task.Status == approvedStatus {
          isApproved = true
      }
  } else {
      // Legacy fallback
      isApproved = task.Status == TaskStatusApproved || task.Status == TaskStatusCodingPlanApproved
  }
  ```
- `hasPendingMerges()`: Same generalization pattern
- `logTaskSubmissionIfCompleted()`: Generalize submitted status check using `pr.SubmittedStatus(task.RolePair)`
- Load pipeline resolver once at the top of each function (or pass as parameter)
- Tests with pipeline-configured tasks using Phase 2 role-pairs

**Done when:**
- A task with `role_pair: "us-writing-pair"` and status `US_APPROVED` is picked up by `handleApprovedMerges`
- Legacy tasks (no role_pair) still work via fallback
- Tests pass

**Scope:** `internal/agent/claiming.go`

**Depends:** Tasks 2, 3

---

### Task 6: Add Epic Planner prompt template

**Intent:** Epic Planner agents receive proper context with state transitions, tools, and decomposition guidance.

**Files to modify:**
- `internal/prompts/templates/epic_planner_context.tmpl` (new)
- `internal/prompts/builder.go`

**Changes:**
- Create template with:
  - State transitions: `EPIC_PLANNING → EPIC_PLAN_TO_REVIEW`, `EPIC_PLAN_REJECTED → DRAFT_EPIC_PLAN`
  - Tools: `liza_submit_for_review`, `liza_write_checkpoint`, `liza_mark_blocked`
  - Decomposition guidance from spec §Phase 2 §1 (granularity heuristics table, right-sized epic criteria)
  - Bash constraints block (same as code_planner)
  - Implementation phase instructions
- Add `EpicPlannerContextConfig` struct (ProjectRoot, AgentID)
- Add `BuildEpicPlannerContext(task, config)` function
- Tests for template rendering

**Done when:**
- `BuildEpicPlannerContext()` renders template with task details and decomposition heuristics
- Template includes all 7 granularity signals from spec
- Tests pass

**Scope:** `internal/prompts/templates/epic_planner_context.tmpl`, `internal/prompts/builder.go`

**Depends:** Task 3

---

### Task 7: Add Epic Plan Reviewer prompt template

**Intent:** Epic Plan Reviewer agents receive review checklist with decomposition quality gates.

**Files to modify:**
- `internal/prompts/templates/epic_plan_reviewer_context.tmpl` (new)
- `internal/prompts/builder.go`

**Changes:**
- Create template with:
  - State transitions: `REVIEWING_EPIC_PLAN → EPIC_PLAN_APPROVED`, `REVIEWING_EPIC_PLAN → EPIC_PLAN_REJECTED`
  - Tool: `liza_submit_verdict`
  - Review checklist from spec §Phase 2 §1 (6 gates: cohesive capability, right-sized scope, falsifiable done_when, persona coherence, independence, vision coverage)
  - Review phase instructions
- Add `EpicPlanReviewerContextConfig` struct
- Add `BuildEpicPlanReviewerContext(task, config)` function
- Tests

**Done when:**
- Template includes all 6 review gates from spec
- `BuildEpicPlanReviewerContext()` renders correctly
- Tests pass

**Scope:** `internal/prompts/templates/epic_plan_reviewer_context.tmpl`, `internal/prompts/builder.go`

**Depends:** Task 3

---

### Task 8: Add US Writer prompt template

**Intent:** US Writer agents receive context integrating the `user-story-writing` skill.

**Files to modify:**
- `internal/prompts/templates/us_writer_context.tmpl` (new)
- `internal/prompts/builder.go`

**Changes:**
- Create template with:
  - State transitions: `WRITING_US → US_READY_FOR_REVIEW`, `US_REJECTED → DRAFT_US`
  - Tools: `liza_submit_for_review`, `liza_write_checkpoint`, `liza_mark_blocked`
  - Reference to `user-story-writing` skill (SMARC criteria, persona quality, acceptance criteria)
  - Instruction to read skill file `~/.liza/skills/user-story-writing/SKILL.md` at session start
  - Bash constraints block
  - Implementation phase instructions
- Add `USWriterContextConfig` struct
- Add `BuildUSWriterContext(task, config)` function
- Tests

**Done when:**
- Template references `user-story-writing` skill
- `BuildUSWriterContext()` renders correctly
- Tests pass

**Scope:** `internal/prompts/templates/us_writer_context.tmpl`, `internal/prompts/builder.go`

**Depends:** Task 3

---

### Task 9: Add US Reviewer prompt template

**Intent:** US Reviewer agents receive review context with spec-review and user-story anti-patterns.

**Files to modify:**
- `internal/prompts/templates/us_reviewer_context.tmpl` (new)
- `internal/prompts/builder.go`

**Changes:**
- Create template with:
  - State transitions: `REVIEWING_US → US_APPROVED`, `REVIEWING_US → US_REJECTED`
  - Tool: `liza_submit_verdict`
  - Reference to `spec-review` skill
  - Anti-patterns from `user-story-writing` skill (Persona Laundering, Giant Story, Wishful Story, etc.)
  - Review phase instructions
- Add `USReviewerContextConfig` struct
- Add `BuildUSReviewerContext(task, config)` function
- Tests

**Done when:**
- Template references both `spec-review` and `user-story-writing` skills
- Anti-pattern checklist included
- `BuildUSReviewerContext()` renders correctly
- Tests pass

**Scope:** `internal/prompts/templates/us_reviewer_context.tmpl`, `internal/prompts/builder.go`

**Depends:** Task 3

---

### Task 10: Extend supervisor dispatch for Phase 2 roles

**Intent:** Supervisor loop recognizes and dispatches for all Phase 2 roles (claim, wait, prompt, timeouts).

**Files to modify:**
- `internal/agent/supervisor.go`
- `internal/agent/waitforwork.go`
- `internal/agent/prompt.go`

**Changes:**
- `supervisor.go` → `RunSupervisor()`:
  - Execution timeout: add epic-planner, us-writer to doer defaults (2h); epic-plan-reviewer, us-reviewer to reviewer defaults (30min)
  - Claim dispatch: add epic-planner, us-writer to doer branch; epic-plan-reviewer, us-reviewer to reviewer branch
  - Post-exit handling: add new reviewer roles to merge handler check; add new doer roles to submission logger
- `waitforwork.go`:
  - Add `waitForEpicPlannerWork()`, `waitForUSWriterWork()` (same pattern as `waitForCodePlannerWork`)
  - Add `waitForEpicPlanReviewerWork()`, `waitForUSReviewerWork()` (same pattern as `waitForCodePlanReviewerWork`)
  - Extend `waitForWork()` switch with 4 new cases
  - Extend `getRoleWaitConfig()` with new reviewer/doer role mappings
  - Extend `isResumableHandoff()` to check for new doer executing statuses (pipeline-aware — already handled by existing pipeline resolver check)
- `prompt.go` → `buildPrompt()`:
  - Add cases for `RuntimeEpicPlanner`, `RuntimeEpicPlanReviewer`, `RuntimeUSWriter`, `RuntimeUSReviewer`
  - Each calls the corresponding `Build*Context()` from tasks 6-9
- Tests

**Done when:**
- `liza run --role epic-planner` successfully claims, builds prompt, and dispatches
- All 4 new roles work in the supervisor loop
- Tests pass

**Scope:** `internal/agent/supervisor.go`, `internal/agent/waitforwork.go`, `internal/agent/prompt.go`

**Depends:** Tasks 3, 5, 6, 7, 8, 9

---

### Task 11: Update Orchestrator entry-point dispatch

**Intent:** Orchestrator dispatches to the correct entry-point based on `--entry-point` flag or LLM classification.

**Files to modify:**
- `internal/prompts/templates/wake_initial_planning.tmpl`
- `internal/prompts/builder.go`

**Changes:**
- Make `wake_initial_planning.tmpl` entry-point-aware:
  - If `GoalEntryPoint` is set (human provided `--entry-point`), use it directly:
    - Resolve to role-pair from pipeline config
    - Create task with that role-pair
  - If `GoalEntryPoint` is not set, provide classification instructions:
    - List available entry-points with descriptions
    - `general-objective` → `epic-planning-pair` (vision documents, broad objectives)
    - `detailed-spec` → `code-planning-pair` (structured requirements, user stories)
    - Default: `general-objective` (skipping spec phase is riskier than redundant refinement)
  - Template data: add `GoalEntryPoint`, `EntryPoints` (map of name → role-pair), `RolePairLabels` (map of role-pair → display name)
- Update `BuildOrchestratorContext()`:
  - Load pipeline config to get entry-points
  - Pass entry-point data to template
  - Pass `state.Goal.EntryPoint` to template
- Remove hardcoded `"code-planning-pair"` from template; derive from entry-point
- Parameterize task ID generation (currently hardcoded `code-planning-1` → derive from entry-point role-pair)
- Tests: verify template renders correctly with and without explicit entry-point

**Done when:**
- With `--entry-point general-objective`: creates task with `role_pair: "epic-planning-pair"`
- With `--entry-point detailed-spec`: creates task with `role_pair: "code-planning-pair"`
- Without `--entry-point`: template instructs LLM to classify and choose
- Tests pass

**Scope:** `internal/prompts/templates/wake_initial_planning.tmpl`, `internal/prompts/builder.go`

**Depends:** Task 2

---

## Dependency Graph

```
Task 1 (YAML schema) ─────────┐
                               ├─→ Task 2 (Resolver) ─────┬─→ Task 4 (one-to-one + 3-part refs)
                               │                          ├─→ Task 5 (status generalization) ──┐
Task 3 (Roles) ────────────────┤                          └─→ Task 11 (Orchestrator dispatch)  │
                               ├─→ Task 6 (Epic Planner template) ───────────────────────┐     │
                               ├─→ Task 7 (Epic Plan Reviewer template) ─────────────────┤     │
                               ├─→ Task 8 (US Writer template) ─────────────────────────┤     │
                               └─→ Task 9 (US Reviewer template) ───────────────────────┤     │
                                                                                         │     │
                                                          Task 10 (Supervisor dispatch) ←┴─────┘
```

**Parallelizable groups:**
- Group A (no deps): Tasks 1, 3
- Group B (after A): Tasks 2, 6, 7, 8, 9
- Group C (after B): Tasks 4, 5, 11
- Group D (after C): Task 10

## Phase 2 YAML Fixture

The complete Phase 2 pipeline YAML (for test fixtures and documentation):

```yaml
pipeline:
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
```

## Tech Debt Notes

1. **Supervisor role switches**: The supervisor dispatch uses a role-name switch that grows
   linearly with each new role-pair. The pipeline config already contains doer/reviewer
   information — a future task could derive this at runtime, reducing the switches to a
   doer/reviewer binary dispatch. Not required for Phase 2 correctness.

2. **`one-to-one` child field generation**: The child task's `desc`, `done_when`, and `scope`
   are generated in code from the parent task. If more `one-to-one` transitions are added,
   consider adding template fields to `TransitionDef` to make this declarative.

3. **Hardcoded status references**: Several places in the codebase still reference hardcoded
   status constants (e.g., `TaskStatusImplementing`, `TaskStatusReadyForReview`). Task 5
   generalizes the most critical ones (merge handling). Remaining references in exit-42
   tracking and logging are low-risk (they affect error messages, not correctness) and can
   be addressed incrementally.
