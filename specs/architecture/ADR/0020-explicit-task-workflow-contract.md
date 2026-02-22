# 20 - Explicit Task Workflow Contract (Type-Aware Claiming and Limit Escalation)

## Context and Problem Statement

Task workflow rules were becoming distributed across command handlers and supervisor logic: claimability checks, allowed status transitions, and iteration/review ceiling behavior were all implemented in multiple places. This created a risk of drift between documented lifecycle rules and runtime enforcement.

At the same time, role expansion required task semantics that were explicit about which roles can claim which tasks. The existing model was effectively coder/reviewer specific, while the architecture direction was role-aware task workflows.

Finally, repeated reject/retry loops needed deterministic runtime behavior when limits were reached. Detecting exhaustion without enforcing a transition left the system vulnerable to prolonged loops.

## Considered Options

1. **Keep workflow logic distributed** across commands/supervisor and rely on tests/docs for consistency.
2. **Add task type mapping only** and keep transitions/limit enforcement mostly local to commands.
3. **Centralize workflow primitives in the model** and enforce exhaustion behavior in runtime ops.

## Decision Outcome

Chose **Option 3**.

### Architecture

- Added a `TaskType` model with a workflow registry (`taskWorkflows`) and default effective type (`coding`) for backward compatibility.
- Added explicit task transition graph (`taskTransitions`) in `internal/models/state.go` with `CanTransition()` and `Task.Transition()`.
- Updated claimability/work diagnostics to derive from role-aware rules and transition capability rather than hardcoded status checks.
- Added runtime escalation for exhausted loops:
  - `effectiveCoderIterationLimit()` and `effectiveReviewCycleLimit()`
  - escalation classification to `BLOCKED` with structured reason/questions
  - enforcement in claim/verdict paths (`internal/ops/claim_task.go`, `internal/ops/submit_verdict.go`).
- Updated specifications to reflect type-aware claimability and lifecycle behavior (`specs/architecture/blackboard-schema.md`, `specs/architecture/state-machines.md`, `specs/protocols/task-lifecycle.md`).

### Rationale

A declared transition graph and typed workflow model make lifecycle behavior inspectable and enforceable in one place. This reduces the chance that a command-level change silently introduces invalid paths.

Runtime limit escalation converts previously advisory loop ceilings into structural behavior: when thresholds are reached, the task is moved to `BLOCKED` with escalation metadata for planner intervention, instead of continuing churn.

### Consequences

**Positive:**
- Single authoritative transition model for task lifecycle rules.
- Role-aware workflow semantics that align with planned multi-role expansion.
- Deterministic behavior when review/iteration limits are exhausted.
- Better alignment between runtime behavior and architecture/spec docs.

**Limitations accepted:**
- Task type abstraction is still partial: role participation is centralized in `taskWorkflows`, but claimable-status mapping remains partly role-switch based in `internal/models/state.go` (`Task.IsClaimable`).
- More lifecycle behavior now depends on consistency between config defaults and runtime limit enforcement.

---
*Reconstructed from commits ec635c5..7fc449d (2026-02-19 to 2026-02-21)*
