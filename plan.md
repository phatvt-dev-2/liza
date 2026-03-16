# Implementation Plan: Dual Name Elimination (Phase 4)

Spec: specs/build/3 - Declarative Role Definitions.md#phase-4-dual-name-elimination

## Context

Today, roles have two name forms:
- **Runtime** (hyphenated): `code-reviewer`, `code-planner`, `epic-plan-reviewer`
- **Workflow** (underscore): `code_reviewer`, `code_planner`, `epic_plan_reviewer`

The workflow form exists because task definitions originally used underscore names while
the CLI used hyphenated names. With declarative roles (Phases 1-3), the canonical name
is the YAML key (hyphenated). Phase 4 eliminates the workflow form entirely.

### Current dual-name machinery

| Location | What | Purpose |
|----------|------|---------|
| `internal/roles/roles.go:20-30` | `Workflow*` constants (9) | Underscore-form role names |
| `internal/roles/roles.go:33-56` | `runtimeToWorkflow` / `workflowToRuntime` maps | Bidirectional mapping |
| `internal/roles/roles.go:60-74` | `ToWorkflow()` / `ToRuntime()` | Conversion functions |
| `internal/roles/roles.go:77-90` | `IsValidWorkflow()` / `AllWorkflow()` | Validation/enumeration |
| `internal/models/task.go:20-29` | `Role*` constants aliased to `roles.Workflow*` | Task-model role names |
| `internal/agent/strategy.go:55-61` | `workflowRole` derivation via `ToWorkflow` | Strategy creation |
| `internal/agent/strategy_doer.go:17` | `workflowRole` field | Doer strategy state |
| `internal/agent/strategy_reviewer.go:21` | `workflowRole` field | Reviewer strategy state |
| `internal/agent/claiming.go:15,83` | `workflowRole` parameters | Doer/reviewer claiming |
| `internal/ops/claim_reviewer_task.go:25,52-63` | `WorkflowRole` field + `ToWorkflow` call | Reviewer task claiming |
| `internal/models/task.go:298-303` | `roles.ToRuntime()` call in `isClaimablePipeline` | Converts workflow->runtime for pipeline comparison |
| `internal/models/diagnostics.go:164` | `roles.ToWorkflow()` call | Converts runtime->workflow for role filtering |
| `internal/commands/watch.go:628-635` | `roles.ToWorkflow()` calls (2) | Converts runtime->workflow for `IsClaimable` |
| `internal/ops/assess_blocked.go:34` | `roles.WorkflowOrchestrator` reference | Role validation |
| `internal/ops/assess_hypothesis_exhausted.go:34` | `roles.WorkflowOrchestrator` reference | Role validation |

### Key insight: workflow names are never persisted to state.yaml

All workflow-form role names are in-memory only:
- `Agent.Role` stores runtime (hyphenated) form — set from CLI agent IDs
- `Task.RolePair` stores hyphenated form (e.g., "coding-pair")
- No other task/agent field stores role name strings

The strategy -> claiming -> IsClaimable call chain passes workflow names, but they
don't reach state.yaml. Migration tooling is a safety net for edge cases (manual
edits, older scripts) — not for correcting production data.

---

## Tasks

### CP1: Unify role constant values to single hyphenated form

**Intent:** Change all `Workflow*` constant values from underscore form (e.g., `"code_reviewer"`)
to hyphenated form (e.g., `"code-reviewer"`). After this change, `ToWorkflow`/`ToRuntime` become
identity functions — all in-memory role comparisons use the unified hyphenated form.

**Changes:**
- `internal/roles/roles.go`: Change `WorkflowCodeReviewer` value from `"code_reviewer"` to `"code-reviewer"`,
  and similarly for all 6 multi-word Workflow* constants. Update both mapping maps to become identity
  (same key and value). Single-word constants (`WorkflowCoder`, `WorkflowOrchestrator`) are already
  identical in both forms — no value change needed.
- `internal/roles/roles_test.go`: Update `TestWorkflowConstants` expected values from underscore to
  hyphenated. Update `TestToWorkflow` and `TestToRuntime` expected mappings (now identity). Update
  `TestIsValidWorkflow` test cases (underscore inputs become invalid, hyphenated inputs become valid).
  Update `TestAllWorkflow` expected values.
- `internal/models/state_test.go`: Update hardcoded assertions: `RoleCodeReviewer != "code_reviewer"`
  -> `"code-reviewer"`, `RoleCodePlanner != "code_planner"` -> `"code-planner"`,
  `RoleCodePlanReviewer != "code_plan_reviewer"` -> `"code-plan-reviewer"`.
  Update test name strings (e.g., `"code_reviewer can claim"` -> `"code-reviewer can claim"`).
- `internal/ops/claim_reviewer_task_test.go`: Update comment at line 309 from
  `"code_reviewer"` to `"code-reviewer"`.

**Scope:** `internal/roles/roles.go`, `internal/roles/roles_test.go`, `internal/models/state_test.go`,
`internal/ops/claim_reviewer_task_test.go`

**Done when:** All `Workflow*` constants in `internal/roles/roles.go` have hyphenated values.
`TestWorkflowConstants` in `roles_test.go` asserts `WorkflowCodeReviewer == "code-reviewer"` (and
similarly for all multi-word roles). `go test ./internal/roles/ ./internal/models/ ./internal/ops/
./internal/agent/ ./internal/commands/ ./cmd/liza/` passes. No test uses hardcoded underscore role
strings for assertion comparison.

**Spec ref:** specs/build/3 - Declarative Role Definitions.md#phase-4-dual-name-elimination

---

### CP2: Remove workflowRole from agent strategy layer

**Intent:** Eliminate the `workflowRole` field from `doerStrategy` and `reviewerStrategy` and the
`ToWorkflow` call in `NewRoleStrategy`. All callers use the runtime `role` directly (which now
equals the unified constant values after CP1).

**Changes:**
- `internal/agent/strategy.go`: Remove lines 55-61 (`workflowRole` derivation via `roles.ToWorkflow`).
  Pass `role` directly where `workflowRole` was passed to strategy constructors.
- `internal/agent/strategy_doer.go`: Remove `workflowRole` field (line 17). Replace all `s.workflowRole`
  references with `s.role`.
- `internal/agent/strategy_reviewer.go`: Remove `workflowRole` field (line 21). Replace all
  `s.workflowRole` references with `s.role`.
- `internal/agent/claiming.go`: Rename parameter `workflowRole` to `role` in `claimDoerTask` (line 15)
  and `claimReviewerTaskForRole` (line 83). No behavioral change — parameter values are identical
  after CP1. Update `mergeGateInput.reviewerRole` doc comment (line 122) if it says "workflow role".
- `internal/agent/strategy_test.go`: Remove `wantWorkflow` field from test table (line 292-306).
  Remove assertions on `workflowRole` field (lines 326-327, 340-341). Update custom-role test
  (line 435-437) to not check workflowRole.

**Scope:** `internal/agent/strategy.go`, `internal/agent/strategy_doer.go`,
`internal/agent/strategy_reviewer.go`, `internal/agent/claiming.go`,
`internal/agent/strategy_test.go`

**Done when:** The string `workflowRole` does not appear as a struct field in `strategy_doer.go` or
`strategy_reviewer.go`. `NewRoleStrategy` does not call `roles.ToWorkflow`. `go test ./internal/agent/`
passes.

**Depends on:** CP1

**Spec ref:** specs/build/3 - Declarative Role Definitions.md#phase-4-dual-name-elimination

---

### CP3: Remove ToRuntime conversion from isClaimablePipeline

**Intent:** `isClaimablePipeline` calls `roles.ToRuntime(role)` to convert workflow-form role names
to runtime form for pipeline comparison. After CP1, the role parameter is already in runtime
(hyphenated) form — the conversion is an identity no-op. Remove it.

**Changes:**
- `internal/models/task.go`: In `isClaimablePipeline` (line 298), remove the `roles.ToRuntime(role)`
  call and the associated error check (lines 300-303). Use the `role` parameter directly in the
  `switch` statement (line 314). Update the doc comment on `IsClaimable` (line 286) to say
  "The role parameter uses runtime form (e.g. `"code-reviewer"`)" instead of workflow form.
  Remove `"github.com/liza-mas/liza/internal/roles"` import if no longer needed.

**Scope:** `internal/models/task.go`

**Done when:** `isClaimablePipeline` does not call `roles.ToRuntime`. The `IsClaimable` doc comment
references runtime form. `go test ./internal/models/` passes.

**Depends on:** CP1

**Spec ref:** specs/build/3 - Declarative Role Definitions.md#phase-4-dual-name-elimination

---

### CP4: Remove WorkflowRole from ClaimReviewerTaskInput

**Intent:** `ClaimReviewerTaskInput.WorkflowRole` and the `roles.ToWorkflow` call in
`ClaimReviewerTask`'s inference code are obsolete — the runtime role extracted from agent ID
is now the canonical name form. Remove the field and use runtime role directly.

**Changes:**
- `internal/ops/claim_reviewer_task.go`: Rename `WorkflowRole` field to `Role` in
  `ClaimReviewerTaskInput` (line 25). Rename the local variable `workflowRole` to `role`. In the
  inference block (lines 52-64), when inferring from agent ID, use the extracted role directly
  (no `roles.ToWorkflow` call). Default to `models.RoleCodeReviewer` if extraction fails.
  Pass `role` to `IsClaimable` (line 85).
  Remove `"github.com/liza-mas/liza/internal/roles"` import if no longer needed.
- `internal/ops/claim_reviewer_task_test.go`: Update any test that sets `WorkflowRole` field to use
  `Role` instead. Update comments referencing workflow form.
- `internal/agent/claiming.go`: Update `ClaimReviewerTaskInput` construction at line 89 to use `Role`
  field instead of `WorkflowRole`.

**Scope:** `internal/ops/claim_reviewer_task.go`, `internal/ops/claim_reviewer_task_test.go`,
`internal/agent/claiming.go`

**Done when:** `ClaimReviewerTaskInput` has a `Role` field (not `WorkflowRole`). No `roles.ToWorkflow`
call exists in `claim_reviewer_task.go`. `go test ./internal/ops/ ./internal/agent/` passes.

**Depends on:** CP1, CP2

**Spec ref:** specs/build/3 - Declarative Role Definitions.md#phase-4-dual-name-elimination

---

### CP5: Remove ToWorkflow calls from diagnostics and watch

**Intent:** `diagnostics.go` and `watch.go` call `roles.ToWorkflow()` to convert pipeline-returned
runtime role names to workflow form for comparison/IsClaimable. After CP1, the workflow form equals
the runtime form — remove the conversions and use runtime names directly.

**Changes:**
- `internal/models/diagnostics.go`: At line 164, replace `roles.ToWorkflow(reviewerRole)` + comparison
  with `RoleCodeReviewer` -> compare `reviewerRole` directly with `RoleCodeReviewer` (both now
  hyphenated). Remove the `err` check since no conversion is needed. Remove
  `"github.com/liza-mas/liza/internal/roles"` import if no longer needed.
- `internal/commands/watch.go`: At lines 628-635, remove both `roles.ToWorkflow()` calls.
  Pass `doerRuntime` and `reviewerRuntime` directly to `task.IsClaimable()` (they are already
  in the correct hyphenated form). Remove `roles` import if no longer needed.

**Scope:** `internal/models/diagnostics.go`, `internal/commands/watch.go`

**Done when:** Neither `diagnostics.go` nor `watch.go` calls `roles.ToWorkflow`. `go test
./internal/models/ ./internal/commands/` passes.

**Depends on:** CP1, CP3

**Spec ref:** specs/build/3 - Declarative Role Definitions.md#phase-4-dual-name-elimination

---

### CP6: Remove deprecated dual-name machinery from roles package

**Intent:** All callers of `ToWorkflow`, `ToRuntime`, `IsValidWorkflow`, `AllWorkflow` have been
updated (CP2-CP5). Remove the mapping functions, maps, and `Workflow*` constants from the roles
package. Rename remaining exports for clarity.

**Changes:**
- `internal/roles/roles.go`:
  - Remove `Workflow*` constants (lines 20-30). Replace with unified constants:
    `Coder = "coder"`, `CodeReviewer = "code-reviewer"`, `Orchestrator = "orchestrator"`,
    `CodePlanner = "code-planner"`, `CodePlanReviewer = "code-plan-reviewer"`,
    `EpicPlanner = "epic-planner"`, `EpicPlanReviewer = "epic-plan-reviewer"`,
    `USWriter = "us-writer"`, `USReviewer = "us-reviewer"`.
  - Remove `runtimeToWorkflow` and `workflowToRuntime` maps.
  - Remove `ToWorkflow()`, `ToRuntime()`, `IsValidWorkflow()`, `AllWorkflow()` functions.
  - Add `IsValid(role string) bool` — checks against a set of known role names.
  - Add `All() []string` — returns all known role names.
- `internal/roles/roles_test.go`: Rewrite tests: remove TestToWorkflow, TestToRuntime,
  TestIsValidWorkflow, TestAllWorkflow, TestRoundTrip, TestExtractAndConvert. Add tests
  for new constants, `IsValid()`, and `All()`. Update constant tests to use new names.
- `internal/models/task.go`: Update aliases from `roles.WorkflowCoder` -> `roles.Coder`,
  `roles.WorkflowCodeReviewer` -> `roles.CodeReviewer`, etc.
- `internal/ops/assess_blocked.go`: Replace `roles.WorkflowOrchestrator` -> `roles.Orchestrator`.
- `internal/ops/assess_hypothesis_exhausted.go`: Same.

**Scope:** `internal/roles/roles.go`, `internal/roles/roles_test.go`, `internal/models/task.go`,
`internal/ops/assess_blocked.go`, `internal/ops/assess_hypothesis_exhausted.go`

**Done when:** `roles.go` exports unified constants (`Coder`, `CodeReviewer`, etc.) with no
`Workflow*` prefix. `ToWorkflow`, `ToRuntime`, `runtimeToWorkflow`, `workflowToRuntime` do not
exist. `IsValid` and `All` functions work. `go test ./internal/roles/ ./internal/models/
./internal/ops/` passes.

**Depends on:** CP2, CP3, CP4, CP5

**Spec ref:** specs/build/3 - Declarative Role Definitions.md#phase-4-dual-name-elimination

---

### CP7: Add liza migrate CLI command for state normalization

**Intent:** Provide `liza migrate` CLI command that normalizes any underscore-form role names in
state.yaml to hyphenated form. This is a safety net for manually-edited state files or edge cases —
current production code does not persist underscore role names.

**Changes:**
- `internal/commands/migrate.go` (new file): Implement `MigrateCommand(statePath string) error`.
  Read state.yaml, walk `Agent.Role` fields, apply `NormalizeRoleName(name string) string`
  (converts known underscore patterns to hyphenated: `code_reviewer` -> `code-reviewer`, etc.).
  If any changes were made, write back and report. If no changes needed, report "already migrated".
- `internal/roles/roles.go`: Add `NormalizeRoleName(name string) string` — canonical normalization
  function. Replaces underscores with hyphens for known multi-word role names. Returns input
  unchanged for unknown names (no silent mutation of arbitrary strings).
- `cmd/liza/cmd_init.go`: Register `migrateCmd` with `rootCmd.AddCommand(migrateCmd)`. Command
  takes optional state-file path argument (defaults to `.liza/state.yaml`).
- `internal/commands/migrate_test.go` (new file): Test migration with underscore Agent.Role values.
  Test idempotency (running twice produces no changes). Test unknown role names pass through unchanged.
- `internal/roles/roles_test.go`: Add `TestNormalizeRoleName` with cases for all 9 roles.

**Scope:** `internal/commands/migrate.go`, `internal/commands/migrate_test.go`,
`internal/roles/roles.go`, `internal/roles/roles_test.go`, `cmd/liza/cmd_init.go`

**Done when:** `liza migrate` command exists and is registered. Running it on a state.yaml with
`Agent.Role: "code_reviewer"` normalizes to `"code-reviewer"`. Running on an already-migrated
state reports no changes. `TestNormalizeRoleName` in `roles_test.go` passes for all 9 roles.
`go test ./internal/commands/ ./internal/roles/ ./cmd/liza/` passes.

**Depends on:** CP6

**Spec ref:** specs/build/3 - Declarative Role Definitions.md#phase-4-dual-name-elimination

---

### CP8: Add unmigrated state detection to liza validate and read-path normalization

**Intent:** `liza validate` should detect underscore-form role names in state.yaml and surface a
guided fix pointing to `liza migrate`. Read paths should normalize underscore names in-memory
(no implicit writes) as a safety net.

**Changes:**
- `internal/statevalidate/validate_roles.go` (new file): Implement
  `validateRoleNames(state *models.State, projectRoot string, skipSpecFileCheck bool) error`.
  Walk `state.Agents` checking `Agent.Role` for underscore-form names using
  `roles.NormalizeRoleName`. If normalized differs from original, return error:
  `"agent %s has unmigrated role name %q — run 'liza migrate' to fix"`.
- `internal/statevalidate/validate.go`: Add `validateRoleNames` to the validators slice.
- `internal/statevalidate/validate_roles_test.go` (new file): Test detection of underscore role
  names. Test that hyphenated names pass validation.
- `internal/db/blackboard.go` (or equivalent read path): After deserializing state, apply
  `roles.NormalizeRoleName` to each `Agent.Role` value in-memory. This ensures runtime code
  works correctly even if state.yaml has unmigrated names. No write-back — normalization is
  in-memory only.
- `internal/db/blackboard_test.go`: Add test verifying read-path normalization: write a state
  with underscore role, read it back, verify Agent.Role is hyphenated in the returned struct.

**Scope:** `internal/statevalidate/validate_roles.go`, `internal/statevalidate/validate_roles_test.go`,
`internal/statevalidate/validate.go`, `internal/db/blackboard.go`, `internal/db/blackboard_test.go`

**Done when:** `liza validate` on a state.yaml with `Agent.Role: "code_reviewer"` returns an error
mentioning `liza migrate`. `liza validate` on a state.yaml with `Agent.Role: "code-reviewer"` passes
(no role-name error). Reading state.yaml with underscore Agent.Role returns a struct with hyphenated
Agent.Role. `go test ./internal/statevalidate/ ./internal/db/` passes.

**Depends on:** CP7

**Spec ref:** specs/build/3 - Declarative Role Definitions.md#phase-4-dual-name-elimination

---

## Dependency Graph

```
CP1  (unify constants)           -- no dependencies
CP2  (strategy workflowRole)     -- depends on CP1
CP3  (isClaimablePipeline)       -- depends on CP1
CP4  (ClaimReviewerTaskInput)    -- depends on CP1, CP2
CP5  (diagnostics/watch)         -- depends on CP1, CP3
CP6  (remove machinery)          -- depends on CP2, CP3, CP4, CP5
CP7  (liza migrate)              -- depends on CP6
CP8  (validate + normalize)      -- depends on CP7
```

CP2, CP3 can run in parallel after CP1.
CP4 depends on CP2 (both modify claiming.go).
CP5 depends on CP3 (IsClaimable signature change).
CP6 is the convergence point — all caller updates must be done.
CP7 and CP8 are serial (CP8 uses CP7's NormalizeRoleName).

## Spec Coverage Mapping

| Spec Requirement | Task(s) |
|-----------------|---------|
| Migrate all internal code to single name form (runtime/hyphenated) | CP1, CP2, CP3, CP4, CP5 |
| Update task model to use runtime names | CP1 (constants), CP3 (IsClaimable doc), CP6 (model aliases) |
| Remove ToWorkflow()/ToRuntime() and associated constants | CP6 |
| liza migrate CLI command for explicit conversion | CP7 |
| Read paths normalize in-memory only (no implicit writes) | CP8 |
| liza validate detects unmigrated state | CP8 |
| liza validate surfaces guided fix pointing to liza migrate | CP8 |
