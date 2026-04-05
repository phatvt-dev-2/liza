# Architecture Step in Coding Sub-Pipeline

## Context

The coding sub-pipeline today starts directly with code planning: the orchestrator creates code-planning tasks, and each code-planner independently makes architectural decisions within its scope. Architecture responsibility lacks cohesion — the orchestrator implicitly does a piece of it when decomposing work into code-planning tasks, then each code-planner does the rest in sequence, with no shared structural vision.

This has concrete consequences. During a recent implementation, the orchestrator created a mix of coding and code-planning tasks simultaneously, attempting to do architecture and planning at the same time. Several tasks collided structurally and had to be superseded. Without an explicit architecture consolidation point, parallel code-planners can diverge on component boundaries, interfaces, and data flow — producing individually reasonable but collectively incoherent plans, and pushing structural realignment to the integration pipeline where it is both late and expensive.

The existing `software-architecture-review` skill is designed for evaluating existing architecture, not for defining the architecture of a new feature or change. A purpose-built skill is a natural follow-up but is not structural to this design.

## Design

### Fan-In / Fan-Out Pattern

The architecture step is a consolidation point between upstream specs and downstream code planning:

```
Path 1 (general-objective): N x US_APPROVED  --> 1 x architecture task --> M x coding plans
Path 2 (detailed-spec):     1 x goal         --> 1 x architecture task --> M x coding plans
```

The N:1 fan-in is a new `many-to-one` transition cardinality: when all tasks on the `from` side reach approved status, the transition creates one child task. The architect works from the linked parent tasks plus the codebase.

The architect's `output[]` defines the M coding plan task definitions. The per-subtask transition creates one code-planning task per entry.

### New Roles

| YAML Key | Type | Display Name | Description | Skills |
|----------|------|--------------|-------------|--------|
| `architect` | doer | Architect | Defines component boundaries, interfaces, and structural decisions for a change | *(follow-up: purpose-built architecture skill)* |
| `architecture-reviewer` | reviewer | Architecture Reviewer | Reviews architectural coherence, interface contracts, structural soundness | systemic-thinking |

The architect focuses on structural decisions: component boundaries, interface contracts, data flow, cross-cutting concerns, and decomposition into coding plan scopes. The output is a set of coding plan task definitions — each with enough architectural context that code-planners can work independently without diverging.

The architecture reviewer validates structural coherence: do the proposed boundaries make sense? Are interfaces well-defined? Will the M coding plans compose into a working whole? Are there cross-cutting concerns that no single plan owns?

### New Role-Pair

```yaml
architecture-pair:
  doer: architect
  reviewer: architecture-reviewer
  states:
    initial: DRAFT_ARCHITECTURE
    executing: ARCHITECTING
    submitted: ARCHITECTURE_TO_REVIEW
    reviewing: REVIEWING_ARCHITECTURE
    approved: ARCHITECTURE_APPROVED
    rejected: ARCHITECTURE_REJECTED
```

### Updated Coding Sub-Pipeline

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
      trigger: manual         # human validates architectural decomposition
      cardinality: per-subtask
    - name: code-plan-to-coding
      from: code-planning-pair.approved
      to: coding-pair.initial
      trigger: manual
      cardinality: per-subtask
```

### Updated Entry Points and Pipeline Transitions

```yaml
entry-points:
  general-objective: epic-spec-subpipeline.epic-planning-pair
  detailed-spec: coding-subpipeline.architecture-pair    # was: code-planning-pair

pipeline-transitions:
  - name: us-to-coding
    from: epic-spec-subpipeline.us-writing-pair.approved
    to: coding-subpipeline.architecture-pair.initial      # was: code-planning-pair.initial
    trigger: manual
    cardinality: many-to-one    # NEW: N approved US → 1 architecture task
```

The `us-to-coding` transition is retained as a declared pipeline transition with the new `many-to-one` cardinality. The pipeline topology stays fully declarative.

### `many-to-one` Transition Semantics

A new cardinality type alongside `one-to-one` and `per-subtask`:

- **Cohort boundary**: sibling tasks sharing a `parent_task`. US tasks are created via `per-subtask` from an epic plan task, so they share a parent. The transition fires when all siblings matching the `from` status reach approved. Tasks with different parents form separate cohorts and produce separate architecture tasks.
- **Action**: create one child task linked to all N parent tasks in the cohort.
- **Gate**: `manual` (human checkpoint before creation, same as other transitions).
- **Child description**: generated from parent tasks (combining descriptions/outputs). The architect's context section renders the parent tasks' content at execution time, so the generated description can be lightweight.
- **`spec_ref`**: inherited from parent tasks (all share the same goal spec). Verified: `spec_ref` propagates through the pipeline chain — it's the goal spec from the start.
- **Provenance**: automatic — parent/child links tracked by the transition system.

**Multi-parent linkage**: The current task model has a singular `ParentTask` field. `many-to-one` requires N parent links. This requires a new `ParentTasks []string` field (or converting `ParentTask` to a slice). Every callsite reading `ParentTask` needs to handle the multi-parent case. See Implementation Cost item 2.

**Idempotency and crash recovery**: Current child-ID and `transitions_executed` are per-source-task. For `many-to-one`, idempotency must be cohort-level:
- **Child ID**: deterministic from the cohort (e.g. derived from sorted parent IDs or the transition name + parent task ID of the cohort).
- **`transitions_executed`**: recorded on all N parent tasks (or on the cohort as a whole). Crash recovery must detect a partially-created `many-to-one` child and either complete or roll back.

**Wake detection**: The existing `PLANNING_COMPLETE` mechanism fires for merged planning tasks with non-empty `output[]`. US-approved tasks don't match this pattern (they're approved, not merged, and don't carry `output[]`). A new wake condition is needed: "all sibling tasks from the `from` side of a `many-to-one` transition reached approved status." This is `many-to-one`-specific detection, not a general-purpose change.

For the detailed-spec path, the entry point targets `architecture-pair` directly. Since there's no upstream transition, the orchestrator creates the architecture task via `add-task` with `spec_ref` pointing to the goal spec (same as today for code-planning tasks).

### `arch_ref` — Durable Architecture Artifact

`spec_ref` is the input: "what to build" (the goal spec). `arch_ref` is the output: "how to structure it" (the architecture document). These are different concerns.

End-to-end `arch_ref` path:

1. **Write**: The architect creates the design document at `specs/arch-plan/<goal-slug>.md` in its worktree and commits it via the standard submit/merge flow. After merge, the document is repo-backed and survives worktree deletion.
2. **Store**: The architect sets `arch_ref` on its `output[]` entries via `set-task-output` — same pattern as `plan_ref` on `OutputEntry`. The value is the repo-relative path to the architecture document (normalized by `NormalizeSpecRef` to strip worktree prefixes). `arch_ref` is **not** set on the architecture task itself — it lives on `output[]` entries only, avoiding file-existence validation on the producing task before merge.
3. **Propagate**: `proceed.go` copies `arch_ref` from `output[]` entries to child code-planning tasks (first hop). For the second hop (code-planning → coding), `proceed.go` copies `arch_ref` from the parent code-planning task to child coding tasks — same parent-field inheritance pattern as `PlanRef` in `buildOneToOneChild`.
4. **Expose**: `role_context.go` and prompt templates expose `arch_ref` to downstream agents. Code-planner, coder, and reviewer context sections render `arch_ref` when present, instructing agents to read the architecture document.

**Validation semantics**: `arch_ref` follows the same validation pattern as `plan_ref`. On `output[]` entries: no file-existence check (entries are not validated against disk). On tasks: `checkSpecFileExists` validates against project root then integration branch via `git cat-file -e`. Since `proceed` creates child tasks after merge, the architecture document exists on the integration branch by the time child tasks carry `arch_ref`.

Task field summary:

| Task | `spec_ref` | `arch_ref` | `plan_ref` |
|------|-----------|-----------|-----------|
| Architecture | goal spec | — (produces it via `output[]`) | — |
| Code-planning | from `output[]` entry | from architecture task's `output[]` entry | — |
| Coding | from `output[]` entry | inherited from parent code-planning task | from code-planner's `output[]` entry |

### Fan-In Closure

The `many-to-one` transition defines cohort closure: all sibling tasks sharing a `parent_task` that match the `from` status must reach approved. The transition fires once per cohort, creating one child. Wake detection for this is new — see `many-to-one` Transition Semantics above.

### Review Quorum

The architecture-pair uses the default quorum of 1. The consolidation value comes from having an explicit architecture step at all, not from multiple reviewers. The single reviewer carries `systemic-thinking` for cross-cutting coherence.

## Implementation Cost

1. **New roles and role-pair in pipeline YAML.** Straightforward — follows established patterns. Context sections for the architect and architecture-reviewer need to be defined (likely a subset of existing doer/reviewer sections).
2. **`many-to-one` cardinality — the largest item.** New transition primitive spanning multiple subsystems:
   - **Task model**: multi-parent linkage — `ParentTasks []string` field (or `ParentTask` converted to slice). Cascades to every callsite reading `ParentTask`.
   - **`proceed.go`**: cohort detection (all siblings approved), child creation from N parents, `spec_ref` inheritance, deterministic child-ID scheme, `transitions_executed` bookkeeping across the cohort, crash recovery for partial creation.
   - **`pipeline/resolver.go`**: parse and validate `many-to-one` cardinality from YAML.
   - **Wake detection**: new condition in `advance_sprint.go` / `workdetection.go` — "all sibling tasks from the `from` side of a `many-to-one` transition reached approved." The existing `PLANNING_COMPLETE` mechanism does not cover this.
   - **Architect context section**: renders N parent tasks' descriptions and outputs. This is `many-to-one`-specific (iterates over parent list) and potentially reusable at other pipeline boundaries.
3. **Entry point and transition updates in pipeline YAML.** `detailed-spec` retargeted to `architecture-pair`. `us-to-coding` retargeted to `architecture-pair.initial` with `many-to-one` cardinality.
4. **Orchestrator prompt updates.** For the detailed-spec path, the orchestrator creates architecture tasks instead of code-planning tasks. For the general-objective path, the `many-to-one` transition handles creation — the orchestrator's role is limited to triggering `proceed` after checkpoint approval. Prompt-level changes only.
5. **`arch_ref` on `OutputEntry` and `Task` models.** New field on both `OutputEntry` (`models/task.go`) and `Task` (`models/task.go`). `set-task-output` normalizes it via `NormalizeSpecRef` (same as `plan_ref`). `validate_task.go` checks file existence on `Task.ArchRef` (same pattern as `PlanRef`). CLI and MCP handler expose it on `set-task-output` (cli-mcp-surface-sync).
6. **`arch_ref` propagation in `proceed.go`.** First hop: copy `arch_ref` from `output[]` entry to child task (same as `spec_ref`/`plan_ref`). Second hop: copy `arch_ref` from parent task to child task (same pattern as `PlanRef` in `buildOneToOneChild`).
7. **`arch_ref` in prompt context.** Add `ArchRef` to `role_context.go` struct. Update code-planner, coder, and reviewer templates to render it when present.
8. **Context sections for architect role.** The architect needs codebase access, its task description, and parent task content (US outputs for general-objective path). A new context section renders linked parent tasks' descriptions and outputs. May reuse existing doer sections (`assigned-task`, `worktree-rules`, `doer-state-transitions`, `doer-tools`) alongside.
9. **New task states.** Six new states (DRAFT_ARCHITECTURE through ARCHITECTURE_REJECTED) need to be recognized by the state machine. Since states are pipeline-driven, this should be automatic from the YAML — verify no hardcoded state lists exist that would need updating.

## Follow-Up Evolutions

1. **Purpose-built architecture skill.** The existing `software-architecture-review` is oriented toward evaluating existing architecture, not defining new feature architecture. A lighter, definition-focused skill would sharpen the architect's output. Not structural — the role works without it, guided by its prompt and context sections.
2. **Architecture skill for the reviewer.** The architecture-reviewer currently carries `systemic-thinking`. A dedicated review checklist for architectural coherence may emerge from operational experience.
