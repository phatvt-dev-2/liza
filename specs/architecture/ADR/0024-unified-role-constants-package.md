# 24 - Unified Role Constants Package

## Context and Problem Statement

Role names existed as string literals scattered across agent, cmd, and ops packages. Two naming conventions coexisted: runtime format (`code-reviewer`, hyphenated, written by the supervisor to agent state) and workflow format (`code_reviewer`, underscored, used in task lifecycle logic). During code review of the crash recovery commands (ADR-0023), gaps in systematic role renaming were noticed — a symptom of fragile string-literal coupling.

## Considered Options

1. **Keep string literals** — rely on tests and reviews to catch inconsistencies.
2. **Standardize on one naming convention** — eliminate the dual-convention entirely.
3. **Introduce a role constants package** with explicit mapping between conventions.

## Decision Outcome

Chose **Option 3**: a new `internal/roles` package providing constants and bidirectional mapping.

### Architecture

**`internal/roles/roles.go`:**
- Runtime constants: `RuntimeCoder`, `RuntimeCodeReviewer`, `RuntimePlanner`
- Workflow constants: `WorkflowCoder`, `WorkflowCodeReviewer`, `WorkflowPlanner`
- `ToWorkflow()` and `ToRuntime()` mapping functions
- `IsValidRuntime()`, `IsValidWorkflow()` validation functions
- `AllRuntime()`, `AllWorkflow()` list functions

**Follow-up** (`e173f71`): Extended to planner role with `WorkflowPlanner` constant, added `models.RolePlanner` alias, and replaced imperative planner wake-trigger branching with declarative trigger specs.

All production code now uses constants from the roles package — no raw role string literals remain.

### Rationale

Constants make role references grep-able and compiler-checked. The dual naming convention (hyphen vs underscore) is an artifact of different contexts (CLI/runtime vs YAML/workflow) and wasn't worth homogenizing in this pass — the important thing was making the mapping explicit and centralized rather than implicit and scattered.

### Consequences

**Positive:**
- Single source of truth for role names
- Mapping between runtime and workflow conventions is explicit and tested
- New roles require updating one package, not hunting for string literals
- Declarative planner wake triggers replace imperative branching

**Limitations accepted:**
- Two naming conventions still coexist — homogenization deferred
- Adding a new role requires updating the roles package mappings

---
*Reconstructed from commits a60c72e..e173f71 (2026-02-23 to 2026-02-24)*
