# 47 - Dual Name Elimination

## Context and Problem Statement

ADR-0024 (Unified Role Constants) introduced the `internal/roles` package with explicit bidirectional mapping between runtime format (`code-reviewer`, hyphenated) and workflow format (`code_reviewer`, underscored). The dual naming was accepted as tech debt with the note: "Two naming conventions still coexist — homogenization deferred."

ADR-0042 (Generic Claim-Type Vocabulary) began the vocabulary unification arc. ADR-0045 (Declarative Role Definitions) made roles first-class YAML objects keyed by hyphenated names. With declarative roles, the workflow (underscore) form became completely redundant — the canonical name is the YAML key, and all pipeline resolution uses hyphenated form. The `ToWorkflow()`/`ToRuntime()` conversion functions were identity operations.

The tech debt from ADR-0024 was ready for payback.

## Considered Options

1. **Remove dual-name machinery entirely** — unify all constants to hyphenated form, add migration tooling as safety net.

No alternatives were considered. The duality existed only because homogenization was deferred, not because it served a purpose.

## Decision Outcome

Chose **Option 1**: eliminate the workflow name form in a systematic 8-step plan, add `liza migrate` CLI command as safety net.

### Architecture

**Phase 4 of Declarative Role Definitions spec, 8 checkpoints:**

1. **Unify constant values** — change all `Workflow*` constant values from underscore to hyphenated. `ToWorkflow`/`ToRuntime` become identity functions.
2. **Remove `workflowRole` from strategy layer** — `doerStrategy` and `reviewerStrategy` use runtime `role` directly.
3. **Remove `ToRuntime` from `isClaimablePipeline`** — the role parameter is already in the correct form.
4. **Remove `WorkflowRole` from `ClaimReviewerTaskInput`** — rename to `Role`, remove conversion.
5. **Remove `ToWorkflow` from diagnostics and watch** — use runtime names directly.
6. **Remove deprecated machinery** — delete `Workflow*` constants, `ToWorkflow()`, `ToRuntime()`, mapping maps. Rename `IsValidWorkflow`→`IsValid`, `AllWorkflow`→`All`. Introduce unified constants: `Coder`, `CodeReviewer`, `Orchestrator`, etc.
7. **`liza migrate` CLI command** — reads state.yaml, normalizes underscore Agent.Role values to hyphenated form. Safety net for manually-edited state files (production code never persisted underscore names).
8. **Unmigrated state detection** — `liza validate` detects underscore role names and surfaces guided fix. Read-path normalization in blackboard (in-memory only, no implicit writes).

**Key insight:** Workflow names were never persisted to state.yaml. `Agent.Role` stores the runtime (hyphenated) form set from CLI agent IDs. The migration tooling is a safety net for edge cases (manual edits, older scripts), not for correcting production data.

**Net deletion:** ~250 lines removed from `internal/roles/roles.go` and tests. Conversion functions, mapping maps, and `Workflow*` constants eliminated. The roles package shrinks to unified constants + `IsValid()` + `All()` + `NormalizeRoleName()`.

### Rationale

Tech debt with a clear payback trigger (ADR-0024 explicitly noted "homogenization deferred"). Declarative roles (ADR-0045) made the trigger condition true: the canonical name form is the YAML key. Every `ToWorkflow`/`ToRuntime` call was a no-op after constant unification — pure code weight with no behavioral value.

### Consequences

**Positive:**
- Single name form for every role — no more mental translation between conventions
- ~250 lines of mapping machinery eliminated
- `liza migrate` + `liza validate` provide safe migration path
- Read-path normalization prevents breakage from stale state files

**Limitations accepted:**
- `liza migrate` must be run on any state.yaml with manually-entered underscore role names (edge case)
- Read-path normalization is defense-in-depth, not a substitute for proper migration

**Completes:** The tech debt declared in ADR-0024 ("Two naming conventions still coexist — homogenization deferred"). Continues the vocabulary arc: ADR-0024 → ADR-0042 → ADR-0047.

---
*Reconstructed from commits ca6a81d..1e1f52f (2026-03-16)*
