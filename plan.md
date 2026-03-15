# Declarative Role Definitions — Phase 2 Implementation Plan

Spec: `specs/build/3 - Declarative Role Definitions.md#phase-2-composable-prompt-sections`

## Spec Requirements (Phase 2)

| # | Requirement | Task(s) |
|---|-------------|---------|
| R1 | Decompose existing 9 role templates into modular blocks | CP-2-3 |
| R2 | Implement generic `BuildRoleContext()` using `context-sections` list | CP-2-3 |
| R3 | Remove per-role config structs and builder functions | CP-2-5 |
| R4 | Wire `mandatory-docs` and `skills` into prompt assembly | CP-2-4 |
| — | Add `context-sections`, `skills`, `mandatory-docs` resolution to Resolver | CP-2-1 |
| — | Define unified template data struct for all blocks | CP-2-2 |

## Current State Analysis

### Prompt Builder Architecture

The current prompt assembly has three layers:

1. **Per-role config structs** (`internal/prompts/builder.go`): 9 `*ContextConfig` structs (CoderContextConfig, ReviewerContextConfig, etc.) plus 9 internal `*contextData` structs
2. **Per-role builder functions** (`internal/prompts/builder.go`): 9 `Build*Context()` functions (BuildCoderContext, BuildReviewerContext, etc.)
3. **Per-role templates** (`internal/prompts/templates/`): 9 monolithic `.tmpl` files (coder_context.tmpl, code_reviewer_context.tmpl, etc.)

Assembly flow:
- `internal/agent/prompt.go` defines 9 `contextBuilderFunc` implementations that extract data from task/state/config → populate `*ContextConfig` → call `Build*Context()`
- `internal/agent/strategy.go` maintains a `contextBuilders` map (8 entries) from role name → `contextBuilderFunc`
- `doerStrategy.BuildPrompt()` and `reviewerStrategy.BuildPrompt()` call `buildPromptWithContext()` which invokes the mapped function
- `orchestratorStrategy.BuildPrompt()` calls `buildOrchestratorPromptContext()` directly

### Template Overlap

Within each role type, templates share ~80% of their structure:
- **Doers** share: assigned-task, prior-rejection, worktree-rules, doer-state-transitions, doer-tools
- **Reviewers** share: review-task, prior-rejection, reviewer-state-transitions, reviewer-tools, anomaly-logging, worktree-rules, review-instructions, rejection-format, verdict-submission
- **Orchestrator** is unique

### Pipeline YAML State (Post-Phase 1)

`RoleDef` struct already has `ContextSections []string`, `Skills []string`, `MandatoryDocs []string` fields. The YAML has `skills` and `mandatory-docs` populated for all 9 roles but does NOT yet have `context-sections`. No Resolver methods exist for these three fields.

## Architecture Decisions

### BuildRoleContext Signature

```go
BuildRoleContext(role string, sectionNames []string, data *RoleContextData) (string, error)
```

Takes section names as a parameter rather than looking them up via a resolver. This keeps the `prompts` package decoupled from the `pipeline` package. The caller in `internal/agent/` looks up `resolver.ContextSections(role)` and passes the result.

### Template Block Convention

Each block is a `.tmpl` file in `internal/prompts/templates/blocks/` using Go `{{define "block-name"}}...{{end}}` syntax. `BuildRoleContext()` iterates over `sectionNames` and executes each named template, concatenating output.

Blocks that vary by role use conditionals on `{{.Role}}` within the template.

### RoleContextData Struct

A single struct in `internal/prompts/role_context.go` covers all role types. Fields unused by a particular role are zero-valued. Key field categories:

- **Identity**: Role, AgentID, RoleType
- **Task**: TaskID, Description, DoneWhen, Scope, SpecRef, Worktree, IterationNum, AttemptNum, PriorRejection
- **Review**: ReviewCycles, ScopeExtensions
- **Plan scoping**: GoalSpecRef, SiblingTasks, TotalPlanTasks, TaskOrdinal
- **Coder-specific**: IntegrationBranch, HandoffNote
- **Orchestrator**: DashboardOutput, WakeInstruction, AgentStates, SprintMetrics, ActivePolicies, BlockedTasks, CheckpointSummary, PipelineConfig
- **Config/State**: ProjectRoot, StatePath, SpecsDir, GoalDesc
- **Declarative** (from pipeline YAML): MandatoryDocs, Skills

## Context-Sections for All 9 Roles

Source of truth for `context-sections` in `internal/embedded/pipeline.yaml`. Every role includes `mandatory-docs` and `skills-affinity` as the final two entries.

### coder
```yaml
context-sections:
  - assigned-task
  - collective-plan-scoping
  - handoff-resume
  - integration-fix
  - prior-rejection
  - doer-state-transitions
  - doer-tools
  - anomaly-logging
  - blocking-protocol
  - worktree-rules
  - commit-workflow
  - implementation-phase
  - submission-phase
  - mandatory-docs
  - skills-affinity
```

### code-reviewer
```yaml
context-sections:
  - review-task
  - collective-plan-scoping
  - scope-extensions
  - prior-rejection
  - reviewer-state-transitions
  - reviewer-tools
  - anomaly-logging
  - worktree-rules
  - review-instructions
  - rejection-format
  - verdict-submission
  - mandatory-docs
  - skills-affinity
```

### orchestrator
```yaml
context-sections:
  - orchestrator-dashboard
  - wake-instructions
  - mandatory-docs
  - skills-affinity
```

### code-planner
```yaml
context-sections:
  - assigned-task
  - collective-plan-scoping
  - prior-rejection
  - doer-state-transitions
  - doer-tools
  - worktree-rules
  - task-decomposition
  - implementation-phase
  - mandatory-docs
  - skills-affinity
```

### code-plan-reviewer
```yaml
context-sections:
  - review-task
  - collective-plan-scoping
  - prior-rejection
  - reviewer-state-transitions
  - reviewer-tools
  - anomaly-logging
  - worktree-rules
  - review-instructions
  - rejection-format
  - verdict-submission
  - mandatory-docs
  - skills-affinity
```

### epic-planner
```yaml
context-sections:
  - assigned-task
  - prior-rejection
  - doer-state-transitions
  - doer-tools
  - worktree-rules
  - capability-scoping
  - implementation-phase
  - mandatory-docs
  - skills-affinity
```

### epic-plan-reviewer
```yaml
context-sections:
  - review-task
  - prior-rejection
  - reviewer-state-transitions
  - reviewer-tools
  - anomaly-logging
  - worktree-rules
  - review-instructions
  - rejection-format
  - verdict-submission
  - mandatory-docs
  - skills-affinity
```

### us-writer
```yaml
context-sections:
  - assigned-task
  - collective-plan-scoping
  - prior-rejection
  - doer-state-transitions
  - doer-tools
  - worktree-rules
  - capability-scoping
  - implementation-phase
  - mandatory-docs
  - skills-affinity
```

### us-reviewer
```yaml
context-sections:
  - review-task
  - collective-plan-scoping
  - prior-rejection
  - reviewer-state-transitions
  - reviewer-tools
  - anomaly-logging
  - worktree-rules
  - review-instructions
  - rejection-format
  - verdict-submission
  - mandatory-docs
  - skills-affinity
```

### Unique Block Names (26 total)

| Block | Used By | Notes |
|-------|---------|-------|
| assigned-task | coder, code-planner, epic-planner, us-writer | Parameterized: renders task header per role |
| review-task | code-reviewer, code-plan-reviewer, epic-plan-reviewer, us-reviewer | Parameterized: renders review header per role |
| collective-plan-scoping | coder, code-reviewer, code-planner, code-plan-reviewer, us-writer, us-reviewer | Shared: plan context + sibling tasks |
| handoff-resume | coder | Coder-only: handoff context |
| integration-fix | coder | Existing coder_integration_fix.tmpl content |
| prior-rejection | all except orchestrator | Shared: renders prior rejection feedback |
| doer-state-transitions | coder, code-planner, epic-planner, us-writer | Parameterized by role |
| doer-tools | coder, code-planner, epic-planner, us-writer | Parameterized by role |
| reviewer-state-transitions | code-reviewer, code-plan-reviewer, epic-plan-reviewer, us-reviewer | Parameterized by role |
| reviewer-tools | code-reviewer, code-plan-reviewer, epic-plan-reviewer, us-reviewer | Parameterized by role |
| anomaly-logging | coder, all reviewers | Shared |
| blocking-protocol | coder | Coder-only |
| worktree-rules | all except orchestrator | Shared |
| commit-workflow | coder | Coder-only |
| implementation-phase | coder, code-planner, epic-planner, us-writer | Parameterized by role |
| submission-phase | coder | Coder-only |
| scope-extensions | code-reviewer | Reviewer scope extensions |
| review-instructions | code-reviewer, code-plan-reviewer, epic-plan-reviewer, us-reviewer | Parameterized by role |
| rejection-format | code-reviewer, code-plan-reviewer, epic-plan-reviewer, us-reviewer | Shared |
| verdict-submission | code-reviewer, code-plan-reviewer, epic-plan-reviewer, us-reviewer | Existing verdict_submission.tmpl content |
| orchestrator-dashboard | orchestrator | Orchestrator-only |
| wake-instructions | orchestrator | Orchestrator-only (includes wake sub-templates) |
| task-decomposition | code-planner | Code-planner-only |
| capability-scoping | epic-planner, us-writer | Shared: spec-phase capability scoping |
| mandatory-docs | all 9 roles | New: renders mandatory doc list from YAML (CP-2-4) |
| skills-affinity | all 9 roles | New: renders skills list from YAML (CP-2-4) |

## Task Decomposition

### CP-2-1: Add context-sections, skills, and mandatory-docs resolution to Resolver and pipeline YAML

**Intent**: Pipeline Resolver provides methods to query context-sections, skills, and mandatory-docs for any role, and all 9 roles in pipeline.yaml have context-sections populated.

**Approach**:
- Add three methods to `pipeline.Resolver`:
  - `ContextSections(name string) ([]string, error)` — returns ordered context-sections list
  - `Skills(name string) ([]string, error)` — returns skills list
  - `MandatoryDocs(name string) ([]string, error)` — returns mandatory-docs list
- `RoleDef` already has `ContextSections`, `Skills`, `MandatoryDocs` fields (added in Phase 1)
- Populate `context-sections` for all 9 roles in `internal/embedded/pipeline.yaml` matching the lists in this plan's "Context-Sections for All 9 Roles" section
- Add tests for the three new methods

**Files**: `internal/pipeline/resolver.go`, `internal/pipeline/resolver_test.go`, `internal/embedded/pipeline.yaml`

**desc**: Add ContextSections(), Skills(), and MandatoryDocs() methods to the pipeline Resolver, and populate context-sections in pipeline YAML for all 9 roles

**done_when**: Resolver.ContextSections("coder") returns the 15-element list matching the plan's coder context-sections; Resolver.Skills("coder") returns ["debugging", "testing", "clean-code"]; Resolver.MandatoryDocs("coder") returns []; all 9 roles in internal/embedded/pipeline.yaml have context-sections matching the plan's "Context-Sections for All 9 Roles" section including mandatory-docs and skills-affinity as final entries for every role; tests in internal/pipeline/resolver_test.go verify the three new methods for at least coder, code-reviewer, and orchestrator; existing pipeline tests pass unchanged

**scope**: internal/pipeline/resolver.go, internal/pipeline/resolver_test.go, internal/embedded/pipeline.yaml

**spec_ref**: specs/build/3 - Declarative Role Definitions.md#phase-2-composable-prompt-sections

**Depends on**: none

---

### CP-2-2: Define RoleContextData struct as the unified template data type for all role template blocks

**Intent**: A single RoleContextData struct serves as the template data type for all modular blocks across all role types.

**Approach**:
- Define `RoleContextData` struct in `internal/prompts/role_context.go` as a superset of all existing `*contextData` structs
- Field categories: identity (Role, AgentID, RoleType), task (TaskID, Description, DoneWhen, Scope, SpecRef, Worktree, IterationNum, AttemptNum, PriorRejection), review (ReviewCycles, ScopeExtensions), plan scoping (GoalSpecRef, SiblingTasks, TotalPlanTasks, TaskOrdinal), coder-specific (IntegrationBranch, HandoffNote), orchestrator (DashboardOutput, WakeInstruction, AgentStates, SprintMetrics, ActivePolicies, BlockedTasks, CheckpointSummary, PipelineConfig), config/state (ProjectRoot, StatePath, SpecsDir, GoalDesc), and declarative (MandatoryDocs, Skills)
- Callers construct the struct directly — no factory function required (construction logic stays in `internal/agent/` where task/state/config are available)
- Add tests verifying struct can be populated for representative role types

**Files**: `internal/prompts/role_context.go`, `internal/prompts/role_context_test.go`

**desc**: Define RoleContextData struct as the unified template data type for all role template blocks

**done_when**: RoleContextData struct exists in internal/prompts/role_context.go with exported fields covering identity (Role, AgentID, RoleType), task (TaskID, Description, DoneWhen, Scope, SpecRef, Worktree, IterationNum, AttemptNum, PriorRejection), review (ReviewCycles, ScopeExtensions), plan scoping (GoalSpecRef, SiblingTasks, TotalPlanTasks, TaskOrdinal), coder-specific (IntegrationBranch, HandoffNote), orchestrator (DashboardOutput, WakeInstruction, AgentStates, SprintMetrics, ActivePolicies, BlockedTasks, CheckpointSummary, PipelineConfig), config/state (ProjectRoot, StatePath, SpecsDir, GoalDesc), and declarative (MandatoryDocs, Skills); tests in internal/prompts/role_context_test.go verify struct population for coder, code-reviewer, and orchestrator with type-appropriate field values

**scope**: internal/prompts/role_context.go, internal/prompts/role_context_test.go

**spec_ref**: specs/build/3 - Declarative Role Definitions.md#phase-2-composable-prompt-sections

**Depends on**: none

---

### CP-2-3: Decompose templates into modular blocks and implement BuildRoleContext()

**Intent**: Existing monolithic role templates are decomposed into reusable blocks, and BuildRoleContext() assembles them by iterating over the context-sections list.

**Approach**:
- Create block `.tmpl` files in `internal/prompts/templates/blocks/` for 24 section names (all from the plan's unique block names table except mandatory-docs and skills-affinity which are CP-2-4)
- Each block uses `{{define "block-name"}}...{{end}}` and receives `*RoleContextData`
- Extract content from existing monolithic templates into the corresponding blocks
- Blocks with role-varying content (doer-state-transitions, doer-tools, reviewer-tools, implementation-phase, review-instructions) use conditionals on `.Role`
- Implement `BuildRoleContext(role string, sectionNames []string, data *RoleContextData) (string, error)` in `internal/prompts/builder.go` — iterates sectionNames, executes each named template with data, concatenates results
- Add equivalence tests: for each of the 9 roles, verify that BuildRoleContext() with the correct section list and populated RoleContextData produces output containing the same key content strings as existing Build*Context() output
- Preserve old monolithic template files (deletion is CP-2-5)

**Files**: `internal/prompts/templates/blocks/` (24 new .tmpl files), `internal/prompts/builder.go` (BuildRoleContext function), `internal/prompts/builder_test.go` (equivalence tests)

**desc**: Decompose existing 9 monolithic role templates into 24 modular block .tmpl files and implement BuildRoleContext(role string, sectionNames []string, data *RoleContextData) that assembles them

**done_when**: 24 block .tmpl files exist in internal/prompts/templates/blocks/ covering all section names from the plan's unique block names table except mandatory-docs and skills-affinity; BuildRoleContext(role string, sectionNames []string, data *RoleContextData) (string, error) exists in internal/prompts/builder.go; equivalence tests in internal/prompts/builder_test.go verify that for all 9 roles BuildRoleContext() output contains the same key content strings as existing Build*Context() output; old monolithic template files in internal/prompts/templates/ are preserved; no existing builder_test.go tests are broken

**scope**: internal/prompts/templates/blocks/ (24 new .tmpl files), internal/prompts/builder.go (BuildRoleContext function), internal/prompts/builder_test.go (equivalence tests)

**spec_ref**: specs/build/3 - Declarative Role Definitions.md#phase-2-composable-prompt-sections

**Depends on**: CP-2-1, CP-2-2

---

### CP-2-4: Create mandatory-docs and skills-affinity template blocks

**Intent**: New template blocks render the declarative mandatory-docs and skills lists from pipeline YAML into the prompt output.

**Approach**:
- Create `internal/prompts/templates/blocks/mandatory_docs.tmpl`: renders "=== MANDATORY DOCUMENTS ===" header and lists each doc path when `MandatoryDocs` is non-empty; renders nothing when empty
- Create `internal/prompts/templates/blocks/skills_affinity.tmpl`: renders "=== SKILLS AFFINITY ===" header and lists each skill name when `Skills` is non-empty; renders nothing when empty
- Both blocks receive `*RoleContextData` and read the `.MandatoryDocs` / `.Skills` fields
- Add tests verifying rendering with populated and empty lists

**Files**: `internal/prompts/templates/blocks/mandatory_docs.tmpl`, `internal/prompts/templates/blocks/skills_affinity.tmpl`, `internal/prompts/builder_test.go`

**desc**: Create mandatory-docs and skills-affinity template blocks that render declarative lists from RoleContextData into prompt output

**done_when**: mandatory_docs.tmpl exists in internal/prompts/templates/blocks/ and renders a MANDATORY DOCUMENTS section when MandatoryDocs is non-empty and renders empty string when MandatoryDocs is empty; skills_affinity.tmpl exists in internal/prompts/templates/blocks/ and renders a SKILLS AFFINITY section when Skills is non-empty and renders empty string when Skills is empty; tests in internal/prompts/builder_test.go verify rendering with populated lists and with empty lists for both blocks

**scope**: internal/prompts/templates/blocks/mandatory_docs.tmpl, internal/prompts/templates/blocks/skills_affinity.tmpl, internal/prompts/builder_test.go

**spec_ref**: specs/build/3 - Declarative Role Definitions.md#phase-2-composable-prompt-sections

**Depends on**: CP-2-2, CP-2-3

---

### CP-2-5: Migrate callers to BuildRoleContext() and remove per-role builder code

**Intent**: All prompt assembly uses BuildRoleContext() via resolver context-sections. Per-role builder infrastructure is removed.

**Approach**:
- Modify `buildPromptWithContext()` in `internal/agent/prompt.go`: replace `contextBuilderFunc` invocation with lookup of `resolver.ContextSections(role)`, construction of `RoleContextData`, and call to `BuildRoleContext()`
- Modify `buildOrchestratorPromptContext()`: same pattern using orchestrator's context-sections
- Strategy BuildPrompt() methods pass resolver to the updated prompt assembly functions
- Remove from `internal/agent/strategy.go`: `contextBuilders` map
- Remove from `internal/agent/prompt.go`: `contextBuilderFunc` type, all 9 named context builder functions (coderContext, codePlannerContext, epicPlannerContext, usWriterContext, codeReviewerContext, codePlanReviewerContext, epicPlanReviewerContext, usReviewerContext, orchestratorContext)
- Remove from `internal/prompts/builder.go`: all 9 `*ContextConfig` structs (OrchestratorContextConfig, CoderContextConfig, ReviewerContextConfig, CodePlannerContextConfig, CodePlanReviewerContextConfig, EpicPlannerContextConfig, USWriterContextConfig, USReviewerContextConfig, EpicPlanReviewerContextConfig), all 9 `Build*Context()` functions, all 9 internal `*contextData` structs
- Delete from `internal/prompts/templates/`: 9 old monolithic `*_context.tmpl` files (orchestrator_context.tmpl, coder_context.tmpl, code_reviewer_context.tmpl, code_planner_context.tmpl, code_plan_reviewer_context.tmpl, epic_planner_context.tmpl, epic_plan_reviewer_context.tmpl, us_writer_context.tmpl, us_reviewer_context.tmpl)
- Update tests with mechanical adjustments (function signatures, imports) preserving test intent

**Files**: `internal/agent/strategy.go`, `internal/agent/prompt.go`, `internal/agent/prompt_test.go`, `internal/prompts/builder.go`, `internal/prompts/builder_test.go`, `internal/prompts/templates/` (delete 9 old files), `internal/agent/strategy_doer.go`, `internal/agent/strategy_reviewer.go`, `internal/agent/strategy_orchestrator.go`

**desc**: Migrate callers from per-role Build*Context() to BuildRoleContext() and remove per-role builder code

**done_when**: buildPromptWithContext() in internal/agent/prompt.go calls BuildRoleContext() via resolver.ContextSections() instead of contextBuilderFunc; buildOrchestratorPromptContext() calls BuildRoleContext() via resolver.ContextSections() instead of orchestratorContext(); contextBuilders map no longer exists in internal/agent/strategy.go; contextBuilderFunc type no longer exists in internal/agent/prompt.go; all 9 named context builder functions no longer exist in internal/agent/prompt.go; all 9 *ContextConfig structs no longer exist in internal/prompts/builder.go; all 9 Build*Context() functions no longer exist in internal/prompts/builder.go; 9 old monolithic *_context.tmpl files are deleted from internal/prompts/templates/; go build ./... succeeds; all tests pass with mechanical updates where needed

**scope**: internal/agent/strategy.go, internal/agent/prompt.go, internal/agent/prompt_test.go, internal/prompts/builder.go, internal/prompts/builder_test.go, internal/prompts/templates/ (delete 9 old files), internal/agent/strategy_doer.go, internal/agent/strategy_reviewer.go, internal/agent/strategy_orchestrator.go

**spec_ref**: specs/build/3 - Declarative Role Definitions.md#phase-2-composable-prompt-sections

**Depends on**: CP-2-3, CP-2-4

---

## Dependency Graph

```
CP-2-1 (resolver + YAML)     CP-2-2 (RoleContextData)
    └─────────┬─────────────────┘
              ▼
       CP-2-3 (blocks + BuildRoleContext)
              │
              ▼
       CP-2-4 (mandatory-docs + skills-affinity)
              │
              ▼
       CP-2-5 (migration + cleanup)
```

## Execution Order

Parallelizable groups:
1. **CP-2-1** + **CP-2-2** (independent foundations)
2. **CP-2-3** (depends on CP-2-1 and CP-2-2)
3. **CP-2-4** (depends on CP-2-2 and CP-2-3)
4. **CP-2-5** (depends on CP-2-3 and CP-2-4)

## Out of Scope

- Review quorum (`review-policy`, `PARTIALLY_APPROVED` state) — Phase 3
- Dual name elimination (`Workflow*` constants, `ToWorkflow()`/`ToRuntime()`) — Phase 4
- Custom template blocks (user-defined blocks in project root) — Open Question #1
