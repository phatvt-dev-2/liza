# 49 - Structured Handoff Events

## Context and Problem Statement

Liza's handoff mechanism used a top-level `State.Handoff` map with overwrite semantics: each `ops.Handoff()` call replaced the previous entry, losing history. Only context-exhaustion triggered a handoff write — submission and completion events were not captured. The schema used flat fields (`Summary`, `NextAction`) that didn't distinguish between what succeeded and what failed.

This was a prerequisite for a planned retrospective phase that would combine handoff events with `/liza-logs` analysis and replanned task data (ADR-0048) to produce sprint retrospectives.

## Considered Options

1. **Per-task append-only HandoffEvent array with structured fields and three triggers** — replaces top-level map with task-level event array.

No alternatives were considered. Append-only per-task events are the natural fit for an audit trail.

## Decision Outcome

Chose **Option 1**: structured handoff events as per-task append-only array.

### Architecture

**HandoffEvent struct:**
```
HandoffEvent:
  Timestamp     time.Time
  Agent         string
  Trigger       HandoffTrigger  # context_exhaustion | submission | completion
  Succeeded     []string        # what worked
  Failed        []string        # what failed and why
  Hypothesis    string          # optional
  NextStep      string          # optional
  KeyFiles      []string        # optional
  DeadEnds      []string        # optional
```

**Three trigger points (mandatory writes):**
- `context_exhaustion`: Agent exhausts context window, calls `ops.Handoff()`
- `submission`: Auto-appended by `ops.SubmitForReview()` on transition to reviewing
- `completion`: Auto-appended by `ops.MergeWorktree()` on transition to merged

**Migration from old model:**
- `State.Handoff` map and `HandoffNote` struct removed
- `Task.HandoffEvents []HandoffEvent` replaces it
- MCP `liza_handoff` tool accepts structured fields (`succeeded`, `failed`, `hypothesis`, `key_files`, `dead_ends`) with backward-compatible mapping from legacy `summary`/`next_action`

**Validation:**
- Each HandoffEvent must have non-zero timestamp, non-empty agent, valid trigger
- Submitted tasks must have at least one `submission` event
- Merged tasks must have at least one `completion` event

**Prompt integration:**
- Resume context reads latest `context_exhaustion` event from `task.HandoffEvents`
- `RoleContextData.HandoffNote` type changed from `*HandoffNote` to `*HandoffEvent`

### Rationale

Append-only per-task events preserve full history across multiple agent handoffs (A exhausts → B exhausts → C completes). Structured succeeded/failed fields enable future retrospective analysis. Mandatory writes on submission and completion ensure every task lifecycle boundary is captured.

### Consequences

**Positive:**
- Full handoff history preserved (no more overwrite loss)
- Structured fields enable retrospective analysis
- Three trigger points ensure complete lifecycle coverage
- Backward-compatible MCP interface

**Extends:** ADR-0020 (Task Workflow Contract) — enriches task lifecycle events.
**Prepares:** Future sprint retrospective phase.

---
*Reconstructed from commits 69a26d2..ef80d62 (2026-03-17)*
