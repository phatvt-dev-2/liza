# Phase 2: Composable Prompt Sections — Implementation Plan

**Spec:** `specs/build/3 - Declarative Role Definitions.md#phase-2-composable-prompt-sections`

## Current State (Post-Phase 1)

Phase 1 established:
- `roles` section in pipeline YAML with 9 roles (type, timeouts, allowed-operations, skills, mandatory-docs)
- `RoleDef` struct has `ContextSections []string` field (defined but not populated in YAML)
- `Resolver` has `RoleType()`, `AllowedOperations()`, `RoleTimeouts()` methods
- `NewRoleStrategy()` dispatches by role type (doer/reviewer/orchestrator) using resolver
- MCP handler authorization uses `allowed-operations` from YAML

### What Phase 2 Must Change

Phase 2 replaces per-role prompt construction with composable template blocks:

**Before:** 9 `*ContextConfig` structs → 9 `Build*Context()` functions → 9 monolithic `.tmpl` files, each with ~80% structural overlap within role type.

**After:** 1 unified data struct → 1 `BuildRoleContext()` function → modular `.tmpl` block files composed per `context-sections` list from YAML.

## Architecture Analysis

### Template Content Categorization

Analysis of the 9 existing role templates reveals three categories of content:

**1. Shared blocks (identical or nearly identical across all roles that use them):**
- `anomaly-logging` — anomaly event table (doer and reviewer variants)
- `blocking-protocol` — blocking protocol rules (doers only)
- `worktree-rules` — worktree git/file rules (doer variant with read/edit rules, reviewer variant with git-only rules)
- `verdict-submission` — already extracted as shared block (reviewers only)
- `commit-workflow` — git commit workflow (coder only, but reusable if other doers need it)

**2. Parameterized blocks (same structure, content varies by role type or resolved state):**
- `assigned-task` — doer task header (ID, worktree, iteration, description, done_when, scope)
- `review-task` — reviewer task header (ID, worktree, commits, author, iteration, description, done_when)
- `collective-plan-scoping` — doer variant ("LIMITED to scope") vs reviewer variant ("flag scope creep")
- `prior-rejection` — doer variant ("MUST ADDRESS") vs reviewer variant ("iteration N")
- `doer-state-transitions` — rendered from resolved pipeline state names per role-pair
- `doer-tools` — rendered from allowed-operations per role
- `reviewer-state-transitions` — rendered from resolved pipeline state names
- `reviewer-tools` — rendered from allowed-operations per role

**3. Role-specific blocks (unique to one role or a small subset):**
- `handoff-resume` — coder only (handoff note context)
- `integration-fix` — coder only (already exists as `coder_integration_fix.tmpl`)
- `scope-extensions` — code-reviewer only (scope extension evaluation)
- `rejection-format` — code-reviewer (structured rejection template)
- `coder-implementation` — coder implementation phase (TDD, testing, clean-code)
- `coder-submission` — coder submission phase (mandatory submit-for-review)
- `code-planner-implementation` — code-planner implementation (task decomposition, set-task-output, self-check)
- `epic-planner-implementation` — epic-planner implementation (epic-writing skill, set-task-output)
- `us-writer-implementation` — US writer implementation (user-story-writing skill, capability scoping)
- `code-reviewer-review` — code-reviewer review instructions
- `code-plan-reviewer-review` — code-plan reviewer instructions + checklist
- `epic-plan-reviewer-review` — epic-plan reviewer instructions + checklist
- `us-reviewer-review` — US reviewer instructions + quality gates + capability scoping
- `orchestrator-dashboard` — orchestrator sprint dashboard + commands
- `wake-instructions` — orchestrator wake-trigger instructions

### Unified Data Structure Design

All blocks render from a single `RoleContextData` struct. Key design decisions:

1. **Superset approach:** The struct contains all fields needed by any block. Blocks use only the fields they need. Fields irrelevant to a role type are zero-valued.

2. **Resolved pipeline state names:** The struct includes resolved state names (ExecutingStatus, SubmittedStatus, etc.) for the current role's pair, enabling `doer-state-transitions` and `reviewer-state-transitions` to render dynamically.

3. **Orchestrator data:** Nested `*OrchestratorData` field (nil for non-orchestrator roles). The orchestrator's complex dashboard data (sprint metrics, wake triggers) is isolated in this sub-struct.

4. **Package boundary:** `RoleContextData` lives in `internal/prompts/` (no dependency on `internal/agent.SupervisorConfig`). The agent layer maps from `SupervisorConfig` to `RoleContextData` fields.

### Context-Sections for All 9 Roles

The spec provides explicit sections for coder, code-reviewer, and orchestrator. Sections for the remaining 6 roles are derived from template analysis:

**Coder:** (per spec)
assigned-task, collective-plan-scoping, handoff-resume, integration-fix, prior-rejection, doer-state-transitions, doer-tools, anomaly-logging, blocking-protocol, worktree-rules, commit-workflow, coder-implementation, coder-submission

**Code-planner:**
assigned-task, collective-plan-scoping, prior-rejection, doer-state-transitions, doer-tools, worktree-rules, code-planner-implementation

**Epic-planner:**
assigned-task, prior-rejection, doer-state-transitions, doer-tools, worktree-rules, epic-planner-implementation

**US-writer:**
assigned-task, collective-plan-scoping, prior-rejection, doer-state-transitions, doer-tools, worktree-rules, us-writer-implementation

**Code-reviewer:** (per spec)
review-task, collective-plan-scoping, scope-extensions, prior-rejection, reviewer-state-transitions, reviewer-tools, anomaly-logging, worktree-rules, code-reviewer-review, rejection-format, verdict-submission

**Code-plan-reviewer:**
review-task, collective-plan-scoping, prior-rejection, reviewer-state-transitions, reviewer-tools, worktree-rules, code-plan-reviewer-review, verdict-submission

**Epic-plan-reviewer:**
review-task, prior-rejection, reviewer-state-transitions, reviewer-tools, worktree-rules, epic-plan-reviewer-review, verdict-submission

**US-reviewer:**
review-task, collective-plan-scoping, prior-rejection, reviewer-state-transitions, reviewer-tools, worktree-rules, us-reviewer-review, verdict-submission

**Orchestrator:** (per spec)
orchestrator-dashboard, wake-instructions

### BuildRoleContext() Function

```
BuildRoleContext(role string, resolver *pipeline.Resolver, data *RoleContextData) (string, error)
```

1. Calls `resolver.ContextSections(role)` to get section list
2. For each section name, renders the named template block with `data`
3. Concatenates results in order
4. Returns the complete context string

The caller (`buildPromptWithContext` in `internal/agent/prompt.go`) handles: base prompt + task lookup + `RoleContextData` assembly + `BuildRoleContext()` call + resume suffix.

### Mandatory-Docs and Skills Blocks

Two new template blocks render declarative YAML fields into the prompt:

- **`mandatory-docs`** — Lists project-specific documentation paths that the agent must read. Renders only when `MandatoryDocs` is non-empty.
- **`skills-affinity`** — Lists skills with affinity to the role. Advisory context ("consider these skills first"), not enforcement. Renders only when `Skills` is non-empty.

These blocks are inserted into context-sections for all roles (after the role header, before implementation/review instructions).

## Task Decomposition

### CP-2-1: Add resolver accessor methods and populate context-sections in pipeline YAML

**Desc:** Add ContextSections(), Skills(), and MandatoryDocs() methods to the pipeline Resolver, and populate context-sections in pipeline YAML for all 9 roles.

**Done when:**
- `Resolver.ContextSections(name)` returns the ordered section list for any declared role
- `Resolver.Skills(name)` returns the skills list for any declared role
- `Resolver.MandatoryDocs(name)` returns the mandatory-docs list for any declared role
- All 9 roles in `internal/embedded/pipeline.yaml` have `context-sections` populated matching the lists in the "Context-Sections for All 9 Roles" section of this plan
- Tests in `internal/pipeline/resolver_test.go` verify the three new methods
- Existing pipeline tests pass unchanged

**Scope:** `internal/pipeline/resolver.go`, `internal/pipeline/resolver_test.go`, `internal/embedded/pipeline.yaml`

**Spec ref:** `specs/build/3 - Declarative Role Definitions.md#phase-2-composable-prompt-sections`

### CP-2-2: Define unified RoleContextData struct and assembler

**Desc:** Create a unified RoleContextData struct used by all template blocks, and a factory function that populates it from task, state, and prompt-building parameters.

**Done when:**
- `RoleContextData` struct exists in `internal/prompts/` with fields covering all three role types: task data (ID, worktree, iteration, description, done_when, scope, spec_ref), reviewer data (base_commit, review_commit, assigned_to, scope_extensions), doer data (handoff_note, integration_branch), plan scoping data (goal_spec_ref, sibling_tasks, total_plan_tasks, task_ordinal), resolved pipeline states (executing, submitted, reviewing, approved, rejected, initial status strings), role metadata (role name, role type, display name, allowed operations, skills, mandatory docs), and orchestrator data (nested sub-struct, nil for non-orchestrator)
- Factory function `NewRoleContextData(...)` exists and correctly populates the struct
- Unit tests in `internal/prompts/role_context_test.go` verify field population for at least one doer, one reviewer, and orchestrator

**Scope:** `internal/prompts/role_context.go`, `internal/prompts/role_context_test.go`

**Spec ref:** `specs/build/3 - Declarative Role Definitions.md#phase-2-composable-prompt-sections`

**Depends on:** none

### CP-2-3: Decompose templates into modular blocks and implement BuildRoleContext()

**Desc:** Extract modular template blocks from the 9 monolithic role templates and implement the generic BuildRoleContext() function that composes them per context-sections list.

**Done when:**
- Modular block `.tmpl` files exist in `internal/prompts/templates/` for every section name referenced in any role's context-sections (approximately 25-28 block files)
- `BuildRoleContext(role string, sectionNames []string, data *RoleContextData) (string, error)` function exists in `internal/prompts/`
- For all 9 roles: `BuildRoleContext()` produces output that is functionally equivalent to the existing `Build*Context()` functions (verified by tests comparing key content strings)
- Old monolithic template files are NOT deleted (preserved for CP-2-5 migration)
- No existing `builder_test.go` tests are broken

**Scope:** `internal/prompts/templates/` (new block files), `internal/prompts/builder.go` or `internal/prompts/role_context.go` (BuildRoleContext function), `internal/prompts/builder_test.go` (equivalence tests)

**Spec ref:** `specs/build/3 - Declarative Role Definitions.md#phase-2-composable-prompt-sections`

**Depends on:** CP-2-1, CP-2-2

### CP-2-4: Add mandatory-docs and skills-affinity template blocks

**Desc:** Create template blocks that render mandatory-docs and skills-affinity from YAML configuration into the prompt, and add these sections to each role's context-sections list.

**Done when:**
- `mandatory-docs` template block exists: renders "MANDATORY DOCUMENTS" listing when MandatoryDocs is non-empty, renders nothing when empty
- `skills-affinity` template block exists: renders "SKILLS AFFINITY" listing when Skills is non-empty, renders nothing when empty
- Both blocks are added to context-sections for all 9 roles in `internal/embedded/pipeline.yaml` (positioned after role header, before implementation/review instructions)
- Tests verify correct rendering with populated and empty lists
- Test confirms empty mandatory-docs (current default) produces no output

**Scope:** `internal/prompts/templates/mandatory_docs.tmpl`, `internal/prompts/templates/skills_affinity.tmpl`, `internal/embedded/pipeline.yaml` (add sections), `internal/prompts/builder_test.go` (tests)

**Spec ref:** `specs/build/3 - Declarative Role Definitions.md#phase-2-composable-prompt-sections`

**Depends on:** CP-2-3

### CP-2-5: Migrate callers to BuildRoleContext() and remove per-role code

**Desc:** Switch the strategy layer from per-role context builder functions to the generic BuildRoleContext(), then remove all per-role config structs, builder functions, named context builder functions, and old monolithic template files.

**Done when:**
- `doerStrategy.BuildPrompt()` and `reviewerStrategy.BuildPrompt()` call `BuildRoleContext()` (via an updated `buildPromptWithContext` or replacement) instead of per-role `contextBuilderFunc`
- `orchestratorStrategy.BuildPrompt()` calls `BuildRoleContext()` instead of `buildOrchestratorPromptContext()`
- The `contextBuilders` map in `internal/agent/strategy.go` is removed
- The `contextBuilderFunc` type in `internal/agent/prompt.go` is removed
- All 9 named context builder functions in `internal/agent/prompt.go` (coderContext, codePlannerContext, etc.) are removed
- All 9 `*ContextConfig` structs in `internal/prompts/builder.go` are removed
- All 9 `Build*Context()` functions in `internal/prompts/builder.go` are removed
- All 9 old monolithic `*_context.tmpl` files are deleted (replaced by modular blocks from CP-2-3)
- All existing tests pass (tests updated to use new API where needed)
- Mechanical test updates (import paths, function signatures) are in-scope; behavioral test changes are not expected

**Scope:** `internal/agent/strategy.go`, `internal/agent/prompt.go`, `internal/prompts/builder.go`, `internal/prompts/builder_test.go`, `internal/prompts/templates/` (delete old files), `internal/agent/strategy_doer.go`, `internal/agent/strategy_reviewer.go`, `internal/agent/strategy_orchestrator.go`

**Spec ref:** `specs/build/3 - Declarative Role Definitions.md#phase-2-composable-prompt-sections`

**Depends on:** CP-2-3, CP-2-4

## Dependency Graph

```
CP-2-1 ──────────┐
                  ├──► CP-2-3 ──► CP-2-4 ──► CP-2-5
CP-2-2 ──────────┘
```

CP-2-1 and CP-2-2 are independent of each other and can execute in parallel.
CP-2-3 depends on both CP-2-1 and CP-2-2.
CP-2-4 depends on CP-2-3 (needs BuildRoleContext and block infrastructure).
CP-2-5 depends on CP-2-3 and CP-2-4 (needs everything working before removing old code).

## Design Decisions for Implementers

1. **Parameterized blocks vs role-specific blocks:** Blocks like `doer-state-transitions` and `doer-tools` should render dynamically using resolved pipeline state names and allowed-operations from `RoleContextData`. This avoids 9 near-identical template files. Use Go template conditionals (`{{if eq .RoleType "doer"}}`) where doer/reviewer variants differ.

2. **Tool documentation rendering:** The `doer-tools` and `reviewer-tools` blocks should render tool documentation for each allowed operation. This requires a mapping from operation names (e.g., "submit-for-review") to documentation snippets (parameters, descriptions). Consider a `FuncMap` helper or nested template calls.

3. **Resolver access in BuildRoleContext:** The function needs the section list from the resolver. Options: (a) pass section list directly as a parameter, (b) pass the resolver, (c) store resolver in strategy struct. Option (a) is simplest and avoids coupling `internal/prompts` to `internal/pipeline`.

4. **Orchestrator special case:** The orchestrator has no task and uses a completely different data structure (sprint metrics, wake triggers). The unified `RoleContextData` accommodates this via a nested `*OrchestratorData` field. The `orchestrator-dashboard` and `wake-instructions` blocks render from this nested struct.

5. **Behavioral equivalence vs byte-identical:** The new system produces functionally equivalent output, not byte-identical. Acceptable differences include: section ordering matching `context-sections` YAML order (may differ from current template order), tool documentation for all allowed-operations (current templates under-document some tools), dynamically resolved state names instead of hardcoded strings.

## Spec Coverage Verification

| Spec Requirement | Covered By |
|---|---|
| Decompose existing 9 role templates into modular blocks | CP-2-3 |
| Implement generic `BuildRoleContext()` using `context-sections` list | CP-2-3 |
| Remove per-role config structs and builder functions | CP-2-5 |
| Wire `mandatory-docs` and `skills` into prompt assembly | CP-2-4 |
| Context-sections list in YAML schema (needed for composition) | CP-2-1 |
| Shared data structure for all blocks | CP-2-2 |
