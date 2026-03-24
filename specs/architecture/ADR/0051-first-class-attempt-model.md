# 51 - First-Class Attempt Model

## Context and Problem Statement

The vision spec defined an attempt model where tasks could transition to a second attempt with fresh budgets when the first approach was exhausted. The actual implementation had drifted silently and significantly: the `Attempted` field was a string slice tracking agent identities, reassignment logic used identity-based branching (same-coder vs different-coder), and iteration/review limits were never scoped per-attempt. Console output always showed attempts as 1.N, never 2.N.

This was discovered by observing the console — the drift from the initial vision was huge, silent, and undocumented. The worst consequence: the identity-based logic deleted the worktree at every review cycle when the coder changed, which could happen randomly because there is no task-doer affinity. This ADR is a realignment on the vision.

## Considered Options

1. **First-class Attempt lifecycle with 3-phase transition** — replace identity-based logic with a structural lifecycle counter, sentinel-guarded transitions, and independent per-attempt budgets.

No alternatives were considered. The vision still applies — the implementation needed to match it.

## Decision Outcome

Chose **Option 1**: first-class attempt model with `TransitionToNewAttempt` 3-phase operation.

### Architecture

**Attempt as lifecycle unit:**
- `Task.Attempt int` replaces `Attempted []string` — explicit lifecycle counter (0=unset/legacy, 1=first, 2=second/final)
- `EffectiveAttempt()` helper: `max(Attempt, 1)` for backward compatibility
- Each attempt gets independent iteration and review budgets — counters reset on transition
- Identity-independent: within an attempt, all claims share the same budget regardless of which agent picks it up

**3-phase TransitionToNewAttempt:**

```
Phase 1 (with lock):
  Attempt = 2
  Iteration = 0, ReviewCyclesCurrent = 0  (budget reset)
  AssignedTo = "$transitioning"             (sentinel blocks claims)
  Release previous agent
  Append TaskEventNewAttempt to history

Phase 2 (outside lock):
  Delete worktree (best-effort, non-blocking)

Phase 3 (with lock):
  Verify sentinel still "$transitioning"    (detect concurrent modification)
  Clear AssignedTo, RejectionReason, Worktree, BaseCommit
  Transition status → initial status for role-pair
```

**Sentinel claimability guard:**
- Tasks with `AssignedTo` matching `$*` pattern are rejected by `IsClaimable()`
- Prevents claim races during the transition window between Phase 1 and Phase 3
- `checkStaleSentinels()` in watch: alerts critical if sentinel persists >2 minutes (stuck transition)

**Escalation integration:**
- `classifyLimitEscalation()` returns `LimitActionNewAttempt` (attempt 1) or `LimitActionBlocked` (attempt 2)
- Wired into both `SubmitVerdict` (review cap) and `ClaimTask` (iteration cap)
- Attempt 1 exhaustion → new attempt with fresh budgets
- Attempt 2 exhaustion → task marked BLOCKED (human intervention required)

**Prior-attempt context in prompts:**
- `prior_attempt.tmpl` renders when `AttemptNum == 2` with prior attempt's outcome
- Inserted into all doer role pipelines (epic-planner, us-writer, code-planner, coder)
- Signals: "Do NOT repeat the same approach — try a fundamentally different strategy"

**Watch integration:**
- `checkAttempt2()` replaces `checkReassigned()` — checks `EffectiveAttempt() == 2` directly (no history scan, no identity comparison)
- Alert category `ATTEMPT`: marks final-attempt tasks for human attention

**Simplification:**
- Removed identity-based branching from `rejectedClaimStrategy` — no more same-coder vs different-coder logic
- `mutateTask()` in rejected strategy emptied (no counter resets on different-coder claims)
- Watch alerting simplified from history-scanning identity comparison to field check

### Rationale

The vision specified attempts as independent lifecycle units with fresh budgets. The implementation had drifted to identity-based heuristics that never actually triggered attempt transitions. Realigning on the vision required replacing the identity-based model with a structural one — the 3-phase transition with sentinel guards provides safe concurrent operation while keeping the model simple (attempt 1 or 2, no more).

### Implementation Notes

The attempt model spec was complex enough to require multi-phase planning (ADR-0048) for its own implementation by Liza agents. The spec spanned 5 implementation phases: spec alignment, core model, limit enforcement, prompts/display, and watch/validation.

### Consequences

**Positive:**
- Attempt transitions actually happen (vision realignment)
- Independent per-attempt budgets prevent cascading exhaustion
- Sentinel guards prevent claim races during transition
- Prior-attempt context helps agents avoid repeating failed approaches
- Simplified claim strategy — identity-independent logic is cleaner and more correct
- Stale sentinel detection catches stuck transitions

**Limitations accepted:**
- Hard-capped at 2 attempts — sufficient for current needs, extensible later if needed
- Worktree deletion in Phase 2 is best-effort — a failed deletion doesn't block the transition
- `EffectiveAttempt()` backward compatibility means legacy tasks (Attempt=0) are treated as attempt 1

**Realigns:** Vision spec (attempt model). **Extends:** ADR-0020 (Task Workflow Contract) — attempt-aware escalation. ADR-0030 (Code-Enforced Guardrails) — sentinel enforcement.

---
*Reconstructed from commits b1f82b5..e94eb6f (2026-03-23 to 2026-03-24)*
