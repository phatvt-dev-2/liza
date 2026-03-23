# Plan: Implement First-Class Attempt Model

## Context

The task iteration model has drifted across intent, specs, and code. The original intent:
- **Attempt** is a structural lifecycle unit (max 2, hard-coded — no config knob), independent of agent identity
- Each attempt gets fresh caps: 10 iterations, 5 review cycles
- Hitting a cap → new attempt (worktree deleted, counters reset). Attempt #2 cap → BLOCKED
- Agent identity is irrelevant — all agents are fresh sessions

Scope note: this attempt model governs the iteration/review-cycle exhaustion path for the
existing doer/reviewer loop. It is orthogonal to the multi-phase planning flow introduced in
`20260323-multi-phase-planning.md`: a planner may BLOCK immediately on PHASE CONSISTENCY
without consuming an attempt or triggering `new_attempt`.
The specs conflate "attempt" with "coder reassignment." The code never implemented attempts at all (the `Attempted []string` field is dead code — defined but never written to). This plan realigns both.

## Design

### Core mechanic

Cap hit triggers attempt transition:
```
Attempt 1, iteration 10 (or review_cycles 5)
  → state transition (Attempt=2, reset counters, initial status)
  → worktree cleanup (best-effort, outside lock)
  → task claimable again

Attempt 2, iteration 10 (or review_cycles 5)
  → BLOCKED (current behavior)
```

Two trigger paths, both calling the same `TransitionToNewAttempt()`:
- **Review cycle cap** (during `SubmitVerdict`): reviewer rejects, review_cycles_current reaches 5
- **Iteration cap** (during `ClaimTask`): coder tries to claim, iteration already at 10

### Key design decisions

1. **`Attempt int` is a new field, `Attempted []string` is removed separately** — `Attempted` was documentary text for the BLOCKED protocol (never written to — dead code). `Attempt int` is a structural counter (0=unset, 1=first, 2=second). The documentary convention ("what was tried before blocking") is preserved in roles.md blocking protocol guidance as freeform text within `blocked_reason`, not as a typed field. The `Attempted` field removal is a separate dead-code cleanup, not part of the attempt model.

2. **Identity-free attempt boundaries** — Within an attempt, ALL claims preserve the worktree regardless of coder identity. The `previousAssignee == agentID` branching in `rejectedClaimStrategy` is removed entirely. Worktree deletion happens ONLY in `TransitionToNewAttempt()`.

   **Within-attempt counter inheritance is intentional.** If coder-1 uses 4/5 review cycles and a different coder-2 claims within the same attempt, coder-2 inherits the remaining 1 cycle. The attempt — not the agent — is the resource boundary. The remaining budget reflects how much the approach can still be tried. Within-attempt reassignment is rare (requires coder crash/expiry while another coder is available), and the inherited budget accurately reflects approach exhaustion.

3. **`TransitionToNewAttempt()` uses 3-phase flow with sentinel guard** — REJECTED is a claimable status, so the task must be made non-claimable during cleanup. A sentinel `AssignedTo` value blocks concurrent claims without introducing a new status:

   - **Phase 1 (bb.Modify)**: Set `Attempt=2`, reset counters. Set `AssignedTo = "$transitioning"` (sentinel — blocked by the new `IsClaimable()` and `ClaimTask` sentinel checks added in Change #1b). Worktree/BaseCommit fields kept. Task keeps current status.
   - **Phase 2 (git ops, outside lock)**: Delete worktree and branch. Safe because sentinel blocks any concurrent claim from creating a new `task/<id>` branch.
   - **Phase 3 (bb.Modify)**: Re-check `AssignedTo == "$transitioning"` (if not, a concurrent operation modified the task — abort and log error). Clear `AssignedTo`, `Worktree`, `BaseCommit`. Transition to initial pipeline status (now claimable).

   **Why sentinel over new status**: Adding a transient status requires pipeline YAML changes, transition map updates, and prompt handling for every role pair. The sentinel is invisible to the pipeline — it's an `AssignedTo` value that no real agent can match (prefixed with `$`).

   **Required guard**: Neither `IsClaimable()` nor `ClaimTask` currently check `AssignedTo`. The sentinel only works if a check is added. Two places need it:
   - `IsClaimable()` in `task.go`: return false when `AssignedTo` starts with `$` — this prevents the task from appearing in candidate lists at all.
   - `ClaimTask` Phase 1 validation (before strategy dispatch): reject with "task is in transition" when `AssignedTo` starts with `$` — defense-in-depth against direct claim calls.

   **Failure modes:**
   - Phase 2 fails (git error): Phase 3 still runs, clears sentinel and Worktree. Next `ClaimTask` hits stale-resource cleanup.
   - Phase 3 fails (bb error): Task stuck with sentinel `AssignedTo`. Stale agent recovery (`recover_agent.go`) won't catch it — `$transitioning` is not a registered agent. Recovery requires explicit task-side detection (see Change #12, `checkStaleSentinels` in watch.go).
   - Phase 3 re-check fails (sentinel replaced): Another operation modified the task between phases. Phase 3 aborts with error. The worktree may or may not be deleted (Phase 2 ran). The task is in whatever state the concurrent operation left it — no corruption, just a logged anomaly.

   This preserves the invariant from INVARIANTS.md (state lock never held during filesystem ops) AND closes the claim-before-cleanup race.

4. **Prior attempt context in prompt** — At attempt boundary, `RejectionReason` is cleared (fresh approach). BUT the attempt transition reason is surfaced in the prompt as a separate "Prior Attempt Outcome" block (distinct from `PriorRejection`, which is within-attempt feedback). This prevents repeating the same flawed approach while keeping the fresh-start semantics.

   Template: `blocks/prior_attempt.tmpl` — rendered when `AttemptNum == 2`, showing the `new_attempt` history entry's reason (e.g., "review cycle limit reached after 5 rejections").

5. **Hypothesis exhaustion stays orthogonal** — `FailedBy` tracks integration failures, not cap-triggered blocks. `TransitionToNewAttempt()` and cap-triggered BLOCKED do NOT write to `failed_by`. Cap-triggered BLOCKED at attempt 2 uses standard BLOCKED machinery (blocked_reason, blocked_questions). The orchestrator detects it via normal BLOCKED task handling.

   **Also orthogonal to phase-consistency blocking.** Multi-phase planning allows later planning
   phases to BLOCK immediately when they cannot reconcile with prior phases. That path is a spec
   conflict / planning-scope escalation, not attempt exhaustion, and does not increment `Attempt`
   or append `TaskEventNewAttempt`.
6. **`mark_blocked` unchanged** — Manual blocks are voluntary (ambiguity, external blockers). Not affected by attempt number. Agents can block at any time. Manual blocks do not populate `failed_by` (unchanged).

7. **Concurrency safety** — The sentinel `AssignedTo = "$transitioning"` blocks claims from Phase 1 through Phase 3. The task becomes claimable only after Phase 3 clears the sentinel and transitions to initial status. After the function returns, any agent can safely claim.

---

## Changes

Spec changes should be implemented first so coders don't anchor on obsolete specs.

### 1. Model — `internal/models/task.go`

- **Remove** `Attempted []string` field (line 192) — dead code, never written to. Separate cleanup from attempt model, but done in same PR since the field name collides conceptually.
- **Add** `Attempt int` field (`yaml:"attempt,omitempty"`) — 0=unset, 1=first attempt, 2=second
- **Add** helper `EffectiveAttempt() int` — returns `max(Attempt, 1)` for backward compat with existing tasks
- **Set** `Attempt = 1` on first claim in `freshClaimStrategy.mutateTask()` — clean lifecycle (0→1→2, no gaps)

### 1b. Claimability guard — `internal/models/task.go` + `internal/ops/claim_task.go`

- **`IsClaimable()`**: Add early return `false` when `AssignedTo != nil && strings.HasPrefix(*AssignedTo, "$")`. Prevents sentinel-guarded tasks from appearing in candidate lists.
- **`ClaimTask` Phase 1** (before strategy dispatch, ~line 90): Add check — reject with `PreconditionError("task is in transition")` when `AssignedTo` starts with `$`. Defense-in-depth.

### 2. History event — `internal/models/history.go`

- **Add** `TaskEventNewAttempt TaskEvent = "new_attempt"`

### 3. New operation — `internal/ops/transition_attempt.go` (new file)

Single function for attempt boundary, called from both trigger paths:

```go
func TransitionToNewAttempt(projectRoot, taskID, reason string) (*TransitionAttemptResult, error)
```

**3-phase flow with sentinel guard (see Design Decision #3):**

Phase 1 — bb.Modify (mark attempt boundary, block claims via sentinel):
1. Read task, verify `task.EffectiveAttempt() == 1` (precondition)
2. Capture worktree path for Phase 2
3. State mutations:
   - `task.Attempt = 2`
   - `task.Iteration = 0` (next claim increments to 1)
   - `task.ReviewCyclesCurrent = 0` (next rejection increments to 1)
   - `task.LeaseExpires = nil`
   - `task.AssignedTo = ptr("$transitioning")` (sentinel — blocks concurrent claims)
   - Append history: `TaskEventNewAttempt` with reason string
   - Release previous agent from state.Agents if was assigned
   - Task keeps current status, Worktree/BaseCommit kept
   - **RejectionReason kept** (cleared in Phase 3 — avoids invalid REJECTED state without reason)

Phase 2 — Git ops (outside lock):
- Delete worktree and branch via git wrapper using path captured in Phase 1
- Failure is logged as warning, not fatal

Phase 3 — bb.Modify (release sentinel, make task claimable):
1. Re-check: `task.AssignedTo == "$transitioning"` — if not, abort with error (concurrent modification)
2. `task.AssignedTo = nil`
3. `task.RejectionReason = nil` (cleared here, not Phase 1 — keeps REJECTED state valid during transition)
4. `task.Worktree = nil`, `task.BaseCommit = nil`
5. Transition to initial pipeline status (now claimable)

Note: `ReviewCyclesTotal` is NOT reset — it's the audit-trail counter.

### 4. Limit enforcement — `internal/ops/iteration_limits.go`

`classifyLimitEscalation` becomes the **single decision function** for both call sites:
- **Change** signature: add `attempt int` parameter
- **New return**: add `Action` field to `limitEscalation` struct:
  - `LimitActionNewAttempt` (attempt 1 + cap hit)
  - `LimitActionBlocked` (attempt 2 + cap hit)
- Callers branch on action instead of just the bool

### 5. Verdict submission — `internal/ops/submit_verdict.go`

In rejection handling (around line 276):
- Pass `task.EffectiveAttempt()` to `classifyLimitEscalation()`
- `LimitActionNewAttempt` → call `TransitionToNewAttempt(projectRoot, taskID, reason)`. Return normally (task is now in initial status).
- `LimitActionBlocked` → current BLOCKED behavior (existing inline code)

### 6. Claim task — `internal/ops/claim_task.go`

**Refactoring**: `classifyLimitEscalation()` becomes the single decision point. The check at line 151 (`task.Iteration >= maxCoderIterations`) delegates to it with `attempt` parameter.

- `LimitActionNewAttempt` → call `TransitionToNewAttempt()`, return specific error to abort claim. Task is now in initial status with Attempt=2, claimable on next supervisor loop.
- `LimitActionBlocked` → delegate to `enforceRejectedIterationLimit()` (preserves its atomic re-check inside bb.Modify for race protection).

### 7. Claim strategy simplification — `internal/ops/claim_task_strategy.go`

**`rejectedClaimStrategy`:**

- **`handleWorktree()`**: Remove same-coder vs different-coder distinction. Replace with `ensureRejectedWorktreeExists()`:
  - If worktree + branch exist: validate and preserve (current `validateRejectedSameCoderWorktree` logic)
  - If worktree/branch missing: recreate from integration branch (recovery path, replaces the deleted different-coder branch)
  - This preserves existing work when available, and recovers from missing/corrupt worktrees without requiring agent identity.
- **`mutateTask()`**: Remove the `previousAssignee != agentID` branch. No counter resets, no worktree/baseCommit updates. Within an attempt, all claims are equivalent regardless of coder.
- **`validate()`**: Keep `previousAssignee` capture for history events only (reclaimed vs reassigned), but it has no behavioral effect.

### 8. Prompt building — `internal/agent/prompt.go`

- **Line 132**: `AttemptNum: task.EffectiveAttempt()` (was `len(task.Attempted) + 1`)
- **Line 150**: Prior rejection gate `task.Iteration > 1` — unchanged. At attempt 2 iteration 1, there's no within-attempt prior rejection. At iteration 2+, prior rejection from within attempt 2 is shown. (Note: line shifted from 148 due to `DependsOn`/`TaskRolePair` fields added by multi-phase planning.)
- **Add**: Extract `new_attempt` history entry reason for `PriorAttemptOutcome` field when `AttemptNum == 2`.

### 9. Prompt context — `internal/prompts/role_context.go`

- **Add** `PriorAttemptOutcome string` field to `RoleContextData`

### 10. Inspect tasks — `internal/commands/inspect_tasks.go`

- **Line 94**: `AttemptNum: task.EffectiveAttempt()` (was `len(task.Attempted) + 1`)

### 11. Watch command — `internal/commands/watch.go`

- **`checkReassigned()` (lines 430-465)**: Replace identity-based reassignment warning with attempt-aware messaging. Warn on attempt 2 tasks ("attempt 2, final attempt").
- **`checkApproachingLimits()` (lines 468-506)**: Add attempt context to warning messages: `"task-1 — attempt 1, iteration 8/10"` vs `"task-1 — attempt 2 (final), iteration 8/10"`.
- **`checkApproachingLimits()` (lines 493-502)**: Remove "coder failures" warning tied to `FailedBy` + `ReviewCyclesCurrent` (identity-coupled). Hypothesis exhaustion remains separate via `checkHypothesisExhaustion()`.
- **`checkOrphanedRejected()` (~line 331)**: Exempt `$`-prefixed `AssignedTo` — a sentinel is not an orphaned assignment, it's a transition in progress. Follow the same pattern as the `WORKING` agent exemption at line 351: `delete(cache, "orphaned:"+task.ID)` then `continue`. The cache deletion is required — without it, a stale `orphaned:` cache entry from before the transition survives, and the next real orphaned rejection on the same task ID skips the grace period and alerts immediately.
- **Add `checkStaleSentinels()`**: Scan tasks for `AssignedTo` starting with `$`. Alert if present for >2 minutes (Phase 2+3 should complete in seconds). This is the recovery detection for stuck sentinels — stale agent recovery (`recover_agent.go`) won't catch them since `$transitioning` is not a registered agent. Alert level: critical, message: "task X stuck in transition — manual repair needed".

### 12. State validation — `internal/statevalidate/validate_task.go`

- **Remove** `Attempted` validation (field removed)
- **Add** `task.Attempt` must be 0, 1, or 2
- **Add** cross-check: if `Attempt == 2` and task in initial status, iteration and review_cycles_current must be 0

Note: the current validator does NOT check `AssignedTo` → `state.Agents` referential integrity (it checks status/field combinations and duplicate assignments). No sentinel exemption needed — there is no agent-existence check to exempt.

### 13. Prompt templates — `internal/prompts/templates/`

- **`blocks/assigned_task.tmpl`**: Add `ATTEMPT: {{.AttemptNum}}` line. Add conditional when AttemptNum == 2: "FINAL ATTEMPT — task will be BLOCKED if limits are reached."
- **`blocks/review_task.tmpl`**: Same attempt display.
- **`blocks/prior_attempt.tmpl`** (new): Render `PriorAttemptOutcome` when AttemptNum == 2. Shows why attempt 1 was exhausted. Included in doer prompts only.
- **`blocks/doer_state_transitions.tmpl` (line 10)**: Update "task escalates to BLOCKED when exceeded" → "task starts fresh attempt when limits reached; BLOCKED on second attempt exhaustion."

---

## Spec Updates

### `specs/build/1 - Vision.md` (lines 128-130)

| Current | Updated |
|---------|---------|
| "2 coder failures" | "2 attempts" |
| "10 iterations total" | "10 iterations per attempt" |
| "Two coders fail, then rescope" | "Two attempts exhausted, then block" |

### `specs/architecture/blackboard-schema.md`

- **Remove** `attempted: [...]` documentation
- **Add** `attempt: int` field lifecycle section:
  - Task created: 0 (unset)
  - First claim: set to 1
  - Cap hit (iteration 10 or review_cycles 5), attempt 1: set to 2, counters reset, worktree deleted
  - Cap hit, attempt 2: BLOCKED
- **Preserve** the newer sub-pipeline documentation added by `20260323-multi-phase-planning.md`,
  especially `Auto-inherited DependsOn` and `transition_cycle_blocked`; this plan only updates
  the task-attempt fields and the iteration/review-cycle semantics.
- **Iteration Field Lifecycle**: Remove "Task reassigned (different coder) → Reset to 1". Replace with "New attempt triggered → Reset to 0"
- **Review Cycles Split**: Remove "Reset on Reassign". Replace with "Reset on new attempt". Update rationale — budget reset is about approach exhaustion, not personnel change.

### `specs/protocols/task-lifecycle.md`

- **Iteration Limits**: Add "per attempt" to make scope explicit
- **Preserve existing Multi-Phase Planning section** added by `20260323-multi-phase-planning.md`
  and add one explicit orthogonality note: PHASE CONSISTENCY blocking for later planning phases
  is separate from attempt exhaustion.
- **Add** new section "Attempt Transitions": the two-attempt model, trigger conditions, and
  relationship to hypothesis exhaustion / phase-consistency blocking (both orthogonal)
- **Early Warning**: Remove "Coder failures" row. Replace with "Attempt: warning at attempt 2 start"
- **Hypothesis Exhaustion**: Add note that this tracks integration failures via `failed_by`,
  orthogonal to cap-triggered attempt transitions. Cap-triggered paths do not write to `failed_by`.

### `specs/architecture/roles.md` (line 256)

- Remove `attempted` typed field from blocking protocol table. Keep documentary guidance: fold "what approaches were tried" into `blocked_reason` guidance text.

---

## Test Updates

| File | Changes |
|------|---------|
| `internal/ops/transition_attempt_test.go` (new) | Precondition checks, field resets, worktree deletion (and failure tolerance), history entry, status transition, attempt 2 rejection, agent release, sentinel set/cleared, RejectionReason kept until Phase 3 |
| `internal/ops/iteration_limits_test.go` | Add `attempt` param. Cases: attempt 1 cap → NewAttempt, attempt 2 cap → Blocked |
| `internal/ops/claim_task_test.go` | Add: iteration cap at attempt 1 → new attempt. Add: attempt 2 cap → BLOCKED. Replace same-coder/different-coder worktree tests with ensure-worktree-exists tests (present → preserve, missing → recreate). Add: ClaimTask rejects with PreconditionError when AssignedTo is sentinel. Add: Phase 3 re-check failure when sentinel replaced by concurrent op |
| `internal/ops/submit_verdict_test.go` | Add: review cap at attempt 1 → new attempt. Add: attempt 2 → BLOCKED |
| `internal/models/task_test.go` | Add: IsClaimable returns false when AssignedTo is sentinel |
| `internal/commands/inspect_tasks_test.go` | Update AttemptNum computation. Fix test at line 593 (hardcoded `AttemptNum: 2` now reachable via Attempt field) |
| `internal/commands/watch_test.go` | Update approaching-limits tests for attempt context. Replace reassignment alert tests with attempt-aware tests. Add: checkStaleSentinels detects stuck sentinel. Add: sentinel clears orphaned cache entry and does not inherit old grace timer |
| `internal/agent/prompt_test.go` | Update AttemptNum verification. Add PriorAttemptOutcome test |
| `internal/statevalidate/validate_task_test.go` | Remove Attempted validation. Add Attempt range validation |

---

## Verification

1. **Unit tests**: `go test ./internal/ops/... ./internal/models/... ./internal/commands/... ./internal/agent/... ./internal/statevalidate/...`
2. **Integration test flow**:
   - Claim → iterate to cap (10) → verify: Attempt=2, worktree deleted, iteration=0, review_cycles_current=0, task in initial status
   - Claim attempt 2 → iterate to cap → verify: BLOCKED
   - Same flow with review cycle cap (5) instead of iteration cap
   - Mixed: attempt 1 hits review cap → attempt 2 hits iteration cap → BLOCKED
   - Worktree missing on rejected claim → verify: recreated from integration branch (not stuck)
   - Worktree deletion failure during TransitionToNewAttempt → verify: state transitions correctly, orphaned worktree handled on next claim
3. **Backward compat**: Existing state.yaml with no `attempt` field → `EffectiveAttempt()` returns 1
4. **Inspect display**: `liza inspect tasks` shows correct `Attempt.Iteration` format
5. **Watch alerts**: Verify approaching-limits shows attempt context, no identity-based reassignment warnings
6. **Prompt content**: Attempt 2 prompt includes "Prior Attempt Outcome" block with reason
7. **Pre-commit**: All touched files pass
8. **Multi-phase orthogonality**: Planner BLOCKED via PHASE CONSISTENCY RULE does not increment
   `Attempt` and does not append `new_attempt`
