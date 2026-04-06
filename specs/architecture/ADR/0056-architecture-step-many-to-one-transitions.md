# 56 - Architecture Step with Many-to-One Transitions

## Context and Problem Statement

The coding sub-pipeline started directly with code planning: the orchestrator created code-planning tasks, and each code-planner independently made architectural decisions within its scope. Architecture responsibility lacked cohesion — the orchestrator implicitly did part of it when decomposing work, then each code-planner did the rest in sequence, with no shared structural vision.

This had concrete consequences. During a recent implementation, the orchestrator created a mix of coding and code-planning tasks simultaneously, attempting architecture and planning at the same time. Several tasks collided structurally and had to be superseded. Without an explicit architecture consolidation point, parallel code-planners could diverge on component boundaries, interfaces, and data flow — producing individually reasonable but collectively incoherent plans, pushing structural realignment to the integration pipeline where it is both late and expensive.

Adding the architecture step required a fundamentally new pipeline cardinality: `many-to-one` (fan-in). Previously all transitions were `one-to-one` or `per-subtask` (fan-out). The architecture task consolidates N approved upstream tasks into 1 architectural plan, then fans out to M coding plans.

## Considered Options

1. **Architecture step as first step of coding sub-pipeline**, with a new `many-to-one` transition cardinality for fan-in, new `arch_ref` field for context propagation, and multi-parent task linkage.

No alternatives were considered — the architecture consolidation point was part of the initial vision. The `many-to-one` cardinality is the natural mechanism for fan-in within the declarative pipeline model.

## Decision Outcome

Chose **Option 1**: architecture-pair inserted as first step of coding-subpipeline, with a new `many-to-one` cardinality type and `arch_ref` propagation chain.

### Architecture

**Fan-in / fan-out pattern:**

```
Path 1 (general-objective): N x US_APPROVED  ──many-to-one──> 1 x architecture task ──per-subtask──> M x coding plans
Path 2 (detailed-spec):     1 x goal         ──────────────> 1 x architecture task ──per-subtask──> M x coding plans
```

**New roles:**

| YAML Key | Type | Description | Skills |
|----------|------|-------------|--------|
| `architect` | doer | Defines component boundaries, interfaces, structural decisions. Output: coding plan task definitions. | software-architecture-review |
| `architecture-reviewer` | reviewer | Reviews architectural coherence, interface contracts, structural soundness | systemic-thinking |

**Updated pipeline structure:**

```yaml
coding-subpipeline:
  steps:
    - architecture-pair       # NEW
    - code-planning-pair
    - coding-pair
  transitions:
    - name: architecture-to-code-plan
      from: architecture-pair.approved
      to: code-planning-pair.initial
      trigger: manual
      cardinality: per-subtask

pipeline-transitions:
  - name: us-to-coding
    from: epic-spec-subpipeline.us-writing-pair.approved
    to: coding-subpipeline.architecture-pair.initial
    trigger: manual
    cardinality: many-to-one    # NEW

entry-points:
  detailed-spec: coding-subpipeline.architecture-pair    # was: code-planning-pair
```

**`many-to-one` transition semantics:**

| Aspect | Design |
|--------|--------|
| Cohort boundary | Sibling tasks sharing a `parent_task`. US tasks share a parent (created via per-subtask from epic plan), so they form a natural cohort. |
| Trigger | All siblings matching the `from` status reach approved. New wake detection in `advance_sprint.go` / `workdetection.go`. |
| Action | Create one child task linked to all N parent tasks in the cohort. |
| Child ID | Deterministic from the cohort (sorted parent IDs + transition name). |
| Crash recovery | Detects partially-created many-to-one children and completes or rolls back. `transitions_executed` bookkeeping across the cohort. |
| `spec_ref` | Inherited from parent tasks (all share the same goal spec). |

**Multi-parent task model:** New `ParentTasks []string` field on `Task`. `ParentTask` (singular) retained for backward compatibility. All callsites reading parent references migrated to handle both. The architect's context section iterates over parent tasks to render their descriptions and outputs.

**`arch_ref` — durable architecture artifact:**

The architecture document (`specs/arch-plan/<goal-slug>.md`) propagates through the pipeline as a reference field, parallel to `spec_ref` and `plan_ref`:

| Task | `spec_ref` | `arch_ref` | `plan_ref` |
|------|-----------|-----------|-----------|
| Architecture | goal spec | — (produces via `output[]`) | — |
| Code-planning | from `output[]` | from architecture `output[]` | — |
| Coding | from `output[]` | inherited from parent code-planner | from code-planner `output[]` |

End-to-end path:
1. **Write**: Architect creates design document, commits via standard submit/merge
2. **Store**: `set-task-output` stores `arch_ref` on `output[]` entries (normalized by `NormalizeSpecRef`)
3. **Propagate**: `proceed.go` copies `arch_ref` to child tasks (first hop from output entry, second hop from parent task field)
4. **Expose**: Prompt templates render `arch_ref` for code-planner, coder, and reviewer context sections

Validation follows the same pattern as `plan_ref`: no file-existence check on `output[]` entries, `checkSpecFileExists` on task fields (checks project root then integration branch via `git cat-file -e`).

**New context section:** `parent-tasks-context` renders linked parent tasks' descriptions and outputs for the architect role — many-to-one-specific but reusable at other pipeline boundaries.

### Rationale

The architecture step is a consolidation point that prevents structural divergence before it happens. Without it, each code-planner makes independent architectural decisions, and the integration pipeline catches the resulting inconsistencies after all code is written — late and expensive. With it, one architect defines component boundaries and interface contracts, and M code-planners work within that shared structural vision.

The `many-to-one` cardinality keeps the pipeline fully declarative — no special-case orchestrator logic for fan-in. The same YAML configuration vocabulary (`cardinality: many-to-one`) makes the topology explicit and reviewable.

### Consequences

**Positive:**
- Eliminates structural divergence between parallel code-planners
- `arch_ref` propagation gives every downstream agent access to the architectural plan
- `many-to-one` is a generic pipeline primitive — reusable beyond the architecture step
- Multi-parent linkage generalizes task provenance for any fan-in scenario
- Pipeline topology remains fully declarative

**Limitations accepted:**
- ~~No purpose-built architecture skill yet~~ — resolved: `architecture-planning` skill created
- `ParentTasks` field adds complexity to parent reference handling across all callsites
- Detailed-spec path creates architecture task via `add-task` (no upstream transition) — direct entry point

**Extends:** ADR-0035 (Declarative Sub-Pipelines) — new cardinality type. ADR-0038 (Phase 2 Roles) — two new roles. ADR-0036 (Structured Task Output) — `arch_ref` on `output[]`.

---
*Reconstructed from commits cabf1ae..325809d (2026-04-05 to 2026-04-06)*
