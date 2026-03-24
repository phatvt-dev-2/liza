# 48 - Multi-Phase Planning & Phase-Gate Dependencies

## Context and Problem Statement

Liza's pipeline executed a single planning phase: one code-planner produced a task decomposition, one code-plan-reviewer validated it, then all coding tasks ran in parallel. For complex specs — particularly technical changes that couldn't go through the epic/user-story specification sub-pipeline — a single planning phase couldn't converge. The code-planner/code-plan-reviewer pair would fail to produce a coherent plan when the spec spanned too many functional areas or required phased implementation.

The solution had to work at the detailed-spec entry point (code-planning), not require a new sub-pipeline type. A requirement-based specification sub-pipeline remains a future option, but supporting multiple code-planning tasks on the existing entry point was more versatile.

## Considered Options

1. **Multi-phase planning with phase-gate dependency propagation** — orchestrator splits planning into N sequential tasks, system auto-propagates dependencies between phases' children.
2. **Requirement-based specification sub-pipeline** — new pipeline type for breaking down requirements before code-planning. Rejected as less versatile — multi-phase planning at the detailed-spec entry point covers more cases, and a requirement pipeline could still be added later.

## Decision Outcome

Chose **Option 1**: multi-phase planning with automatic phase-gate dependency propagation, planning checkpoints, and topological execution ordering.

### Architecture

**Multi-phase planning flow:**
```
Orchestrator creates:
  plan-1 (scope: core auth)
  plan-2 (scope: notifications, depends_on: plan-1)
  plan-3 (scope: reporting, depends_on: plan-2)

Each plan → code-plan-reviewer → CHECKPOINT (human review) → coding tasks

Phase-gate inheritance:
  plan-1 produces tasks 1a, 1b, 1c
  plan-2 produces tasks 2a, 2b, 2c
  → 2a, 2b, 2c auto-inherit depends_on [1a, 1b, 1c]
```

**Key mechanisms:**

- **`DependsOn` in OutputEntry**: Code planners express sibling task ordering via index references within their output. Orchestrator converts indices to task IDs during task creation.

- **`computeInheritedDeps()`**: When a source task has `depends_on` pointing to an upstream task that executed the same transition, child tasks automatically inherit upstream children as dependencies. This enforces phase-gate barriers without manual specification.

- **Topological execution ordering**: `ExecuteAvailableTransitions` uses Kahn's algorithm to topo-sort pending transitions by task `depends_on`. Upstream transitions fire first, guaranteeing upstream children exist before downstream `computeInheritedDeps` runs. Stable tie-breaking ensures deterministic ordering.

- **Cycle detection**: Circular dependencies emit a `transition_cycle_blocked` history event (idempotent). Cycles suppress `PLANNING_COMPLETE` auto-detection but don't modify task status. Watch command can surface stuck cycles.

- **Planning checkpoints**: After planning tasks merge, orchestrator detects unconsumed `output[]` and creates CHECKPOINT with trigger `PLANNING_COMPLETE`. Human reviews planning output, optionally amends plan files, then runs `liza resume` to proceed.

- **`liza replan`**: At checkpoint, human can amend a plan file and run `liza replan` to re-invoke the planner. Old planning output is marked consumed (`TransitionsExecuted["replanned"] = true`), new planning task is created. Sprint transitions back to IN_PROGRESS with checkpoint trigger cleared. Audit trail preserved.

- **`TaskTypePlanning`**: Non-coding task type for planning roles. TDD gate automatically exempt — no test requirement for plans. Derived from target role-pair doer type (code-planner → planning; others → coding).

- **Phase-consistency blocking rule**: Planners with upstream phase dependencies must mark BLOCKED if their plan conflicts with prior phases' output. Blocking is orthogonal to attempt exhaustion — doesn't increment the attempt counter.

### Rationale

The code-planner/code-plan-reviewer pair wasn't converging on complex specs because a single planning phase couldn't handle multi-area decomposition. Rather than building a new pipeline type, extending the existing code-planning entry point to support N sequential phases was more versatile. The phase-gate dependency propagation mechanism is generic — it works for any pipeline transition, not just code-planning — so future sub-pipelines get the same capability for free.

### Consequences

**Positive:**
- Complex specs can be decomposed into manageable phases
- Phase-gate dependencies are automatic — planners don't need to manually specify cross-phase ordering
- Human checkpoint between planning and coding prevents wasted coding effort on bad plans
- `liza replan` enables amendment recovery without manual worktree manipulation
- Generic mechanism — works for any pipeline transition, not just code-planning

**Limitations accepted:**
- Code complexity is significant — topo sort, cycle detection, crash recovery, 3-phase execution
- Cycle detection is passive (history event) rather than active (auto-resolution)
- Planning checkpoint requires human interaction (`liza resume`) — fully autonomous multi-phase execution is not yet supported

**Extends:** ADR-0035 (Declarative Sub-Pipelines) — phase-gate deps extend the transition mechanism. ADR-0036 (Structured Task Output) — DependsOn in OutputEntry.
**Enables:** ADR-0051 (First-Class Attempt Model) — the attempt model spec was complex enough to require multi-phase planning for its own implementation.

---
*Reconstructed from commits 1689ffb..c6ab59d (2026-03-16 to 2026-03-24)*
