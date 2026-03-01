# 28 - Multi-Sprint Support with Archive Safety

## Context and Problem Statement

Liza operated with a single-sprint model: one batch of tasks from planning to checkpoint. Introducing Spec Writer / Reviewer roles requires a human review gate between spec production and coding — sprints are the natural concept to model these distinct work batches. Without multi-sprint support, transitioning from spec work to coding work would require manual state manipulation.

## Considered Options

1. **Manual state reset between sprints** — clear tasks, restart from scratch.
2. **Multi-sprint with history preservation** — sprints advance automatically, previous sprint summaries remain in state for planner context.

## Decision Outcome

Chose **Option 2**: automatic sprint advancement with lightweight history in state.

### Architecture

**State model additions:**
- `Sprint.Number` field (legacy `0` normalized to `1` for backward compatibility)
- `SprintSummary` — lightweight completed sprint record (number, task count, outcomes)
- `SprintHistory` — list of summaries in state.yaml, available to planner

**`AdvanceSprint` op** — plan/apply separation:
1. `planSprintAdvance` — read-only validation + snapshot (all tasks must be terminal)
2. `writeSprintArchive` — freeze full sprint detail to archive file
3. `applySprintAdvance` — state mutation (increment number, reset tasks, append summary)

All three steps execute inside a single `Modify` closure: archive write failure aborts before state mutation — no partial state.

**Integration:**
- `liza resume` auto-advances when state is at CHECKPOINT with all planned tasks terminal
- Status display and planner context include sprint number and history
- `SprintArchivePath` takes `int` (not string) to prevent path traversal
- Validation rules for sprint number consistency and history entries

### Rationale

Sprint history in state gives the planner context from previous sprints without needing to parse archive files. The archive-before-mutate pattern ensures full sprint detail is frozen before state is modified — easy access to the complete record of each sprint. The auto-advance on `liza resume` makes the transition seamless: checkpoint → human review → resume → next sprint.

### Consequences

**Positive:**
- Enables role expansion (Spec Writer / Reviewer) by modeling distinct work phases as sprints
- Planner has previous sprint context for continuity
- Sprint archive preserves full detail, state carries lightweight summaries
- Atomic operation: archive failure prevents state mutation

**Limitations accepted:**
- Sprint history grows in state.yaml — not a concern at current scale
- Sprint advance is tied to `liza resume` — no standalone advance command yet

---
*Reconstructed from commit 9765a4e (2026-02-27)*
