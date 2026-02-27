# State Machines

## Task State Machine

| State | Description | Valid Transitions |
|-------|-------------|-------------------|
| DRAFT | Task being defined by planner | → READY |
| READY | Task ready, no agent assigned | → IMPLEMENTING |
| IMPLEMENTING | Coder assigned, work in progress | → READY_FOR_REVIEW, BLOCKED |
| READY_FOR_REVIEW | Coder done, awaiting Code Reviewer | → REVIEWING |
| REVIEWING | Reviewer assigned, review in progress | → APPROVED, REJECTED, READY_FOR_REVIEW (stale lease) |
| REJECTED | Code Reviewer rejected, feedback provided | → IMPLEMENTING (supervisor reclaims for coder) |
| APPROVED | Code Reviewer approved, merge eligible | → MERGED, INTEGRATION_FAILED |
| MERGED | Successfully merged to integration | Terminal |
| BLOCKED | Cannot proceed, awaiting escalation | → SUPERSEDED, ABANDONED |
| SUPERSEDED | Replaced by rescoped task(s) | Terminal |
| ABANDONED | Planner killed task | Terminal |
| INTEGRATION_FAILED | Merge conflict or integration test failure | → IMPLEMENTING (integration-fix scope) |

### Task State Diagram

```
     ┌──────────────────────────────────────────┐
     │              DRAFT                       │
     │  (Planner writing, coders must ignore)   │
     └──────────────────┬───────────────────────┘
                        │ finalize
                        ▼
     ┌──────────────────────────────────────────┐
     │              READY                       │
     │       (Available for claim)              │
     └──────────────────┬───────────────────────┘
                        │ claim
                        ▼
     ┌──────────────────────────────────────────┐
     │           IMPLEMENTING                   │
     │       (Coder working)                    │
     └─────────┬────────────────────┬───────────┘
               │                    │
     request_review             blocked
               │                    │
               ▼                    ▼
     ┌─────────────────┐    ┌──────────────┐
     │ READY_FOR_REVIEW│    │   BLOCKED    │
     └────────┬────────┘    └──────┬───────┘
              │                    │
       assign_reviewer       ┌──────┴──────┐
              │               │             │
              ▼             rescope     abandon
     ┌─────────────────┐     │             │
     │   REVIEWING     │     ▼             ▼
     │ (Reviewer active)│  ┌──────────┐  ┌──────────┐
     └────────┬────────┘  │SUPERSEDED│  │ABANDONED │
              │           └──────────┘  └──────────┘
       ┌──────┴──────┐     Terminal      Terminal
       │             │
    approve       reject
       │             │
       ▼             ▼
┌──────────┐  ┌──────────┐
│ APPROVED │  │ REJECTED │
└────┬─────┘  └────┬─────┘
     │             │
     │      resume (IMPLEMENTING)
     │
     ├─────────────┐
     │             │
   merge      conflict
     │             │
     ▼             ▼
┌──────────┐  ┌───────────────────┐
│  MERGED  │  │ INTEGRATION_FAILED│
└──────────┘  └─────────┬─────────┘
   Terminal             │
                   claim (integration-fix)
                        │
                        ▼
                    IMPLEMENTING

Note: REVIEWING → READY_FOR_REVIEW (stale lease recovery, not shown)
```

### Type-Aware Claimability

Each task has a `type` field (default: `"coding"`) that determines which roles participate in its lifecycle. The workflow registry maps each type to an ordered role sequence:

| Type | Role Workflow | Coder Claims | Code Reviewer Claims |
|------|--------------|--------------|---------------------|
| `coding` | coder → code_reviewer | READY, REJECTED, INTEGRATION_FAILED | READY_FOR_REVIEW |

Claimability rule:
```
claimable(task, role) =
    task.effective_type().has_role(role)
    AND status in claimable_statuses_for(role)
    AND (depends_on is empty OR all depends_on are MERGED)
```

When new task types are added (e.g., `specification`, `architecture`), they define their own role workflow in the registry. The supervisor and work detection derive behavior from the registry rather than hardcoding role checks.

### Forbidden Transitions

- DRAFT → IMPLEMENTING (coders cannot claim drafts)
- IMPLEMENTING → MERGED (skipping review)
- IMPLEMENTING → APPROVED (self-approval)
- READY_FOR_REVIEW → APPROVED (must go through REVIEWING)
- READY_FOR_REVIEW → REJECTED (must go through REVIEWING)
- REJECTED → APPROVED (without addressing feedback)
- BLOCKED → READY (in current implementation, blocked tasks are resolved via SUPERSEDED/ABANDONED)
- Any terminal state → Any other state (MERGED, ABANDONED, SUPERSEDED are final)

### Transition Requirements

| Transition | Must Set | Must Preserve | Notes |
|------------|----------|---------------|-------|
| READY_FOR_REVIEW → REVIEWING | `reviewing_by`, `review_lease_expires`, status=REVIEWING | `review_commit`, `assigned_to`, `worktree` | Supervisor assigns reviewer |
| REVIEWING → APPROVED | `approved_by`, status=APPROVED | `review_commit` | Clear `reviewing_by`, `review_lease_expires` |
| REVIEWING → REJECTED | `rejection_reason`, status=REJECTED | `review_commit` | Clear `reviewing_by`, `review_lease_expires`; increment review cycles |
| REVIEWING → READY_FOR_REVIEW | status=READY_FOR_REVIEW | `review_commit`, `assigned_to`, `worktree` | Stale lease recovery: clear `reviewing_by`, `review_lease_expires` |
| REJECTED → IMPLEMENTING (same coder) | `lease_expires` (new) | `worktree`, `review_cycles_current`, `review_cycles_total` | Supervisor reclaims for same coder to address feedback |
| REJECTED → IMPLEMENTING (different coder) | `lease_expires`, `assigned_to`, `review_cycles_current: 0` | `review_cycles_total` | Worktree reset: delete old, create fresh |
| INTEGRATION_FAILED → IMPLEMENTING | `lease_expires`, `integration_fix: true` | `worktree` | Any coder may claim; keeps worktree for conflict resolution |
| BLOCKED → SUPERSEDED | `superseded_by`, `rescope_reason`, status=SUPERSEDED | `failed_by` | Planner links blocked task to replacement task(s) |
| BLOCKED → ABANDONED | status=ABANDONED | `failed_by` | Planner abandons blocked task when no viable continuation exists |
| READY → IMPLEMENTING (reassignment) | `lease_expires`, `assigned_to`, `review_cycles_current: 0` | `failed_by`, `review_cycles_total` | Fresh worktree created |
| Any → MERGED | — | — | Must clear `worktree` (cleanup) |

**Review Cycles on Reassignment:**
- `review_cycles_current` resets to 0 when `assigned_to` changes — new coder gets full budget
- `review_cycles_total` never resets — preserves audit trail for retrospectives
- Limit checks (deadlock detection) use `review_cycles_current`

**Worktree on Reassignment:**
- Same coder continuing: worktree preserved (iterating on own work)
- Different coder taking over: worktree deleted and recreated fresh
- Rationale: salvaging failed work often costs more than restarting from spec (see vision.md)
- Exception: INTEGRATION_FAILED keeps worktree (conflict resolution requires seeing the failed merge state)

**Validation Note:** `liza validate` should verify that IMPLEMENTING tasks always have valid `lease_expires`, regardless of prior state.

---

## Review Lease Lifecycle

Tasks in READY_FOR_REVIEW state have an independent lease for the Code Reviewer:

| Condition | Review Claimable? |
|-----------|-------------------|
| `reviewing_by` is null | Yes |
| `reviewing_by` set, `review_lease_expires` in past | Yes (stale claim) |
| `reviewing_by` set, `review_lease_expires` in future | No (active claim) |

```
READY_FOR_REVIEW
    │
    ├── reviewing_by: null ──► Supervisor assigns review (sets reviewing_by, review_lease_expires)
    │                              │
    │                              ▼
    │                         reviewing_by: code-reviewer-1
    │                         review_lease_expires: future
    │                              │
    │                    ┌─────────┴─────────┐
    │                    │                   │
    │               verdict              lease expires
    │                    │                   │
    │                    ▼                   ▼
    │            APPROVED/REJECTED      reviewing_by: null
    │            (lease cleared)        (reclaimable)
    │
    └── Supervisor may reassign if lease expired
```

**Key invariants:**
- Supervisor claims review before spawning Code Reviewer (prevents duplicate reviews)
- Code Reviewer must extend lease with heartbeats during long reviews
- On verdict: clear `reviewing_by` and `review_lease_expires`
- On approval: set `approved_by` to Code Reviewer agent ID
- On merge: set `merge_commit` to integration commit SHA
- Stale review lease allows supervisor to reassign (Code Reviewer crash recovery)

See [Blackboard Schema — Lease Model](blackboard-schema.md#lease-model) for field details.

---

## Agent State Machine

| State | Description | Valid Transitions |
|-------|-------------|-------------------|
| STARTING | Agent registered, initializing | → IDLE |
| IDLE | No task assigned | → WORKING (Coder), REVIEWING (Code Reviewer) |
| WORKING | Coder actively implementing task | → WAITING, IDLE, HANDOFF |
| REVIEWING | Code Reviewer actively reviewing task | → IDLE (verdict done), HANDOFF |
| WAITING | Coder waiting for review result or escalation | → WORKING (continue after feedback), IDLE (task done) |
| HANDOFF | Context exhaustion, preparing handoff notes | → (agent terminates, supervisor restarts fresh) |

**Role-specific states:**
- WORKING and WAITING: Coder only
- REVIEWING: Code Reviewer only
- STARTING, IDLE, HANDOFF: All roles

### Agent State Diagram

```
┌───────────┐
│ STARTING  │
│           │
└─────┬─────┘
      │ registration complete
      ▼
┌───────────┐
│   IDLE    │◄──────────────────────────────┐
│           │                               │
└─────┬─────┘                               │
      │                                     │
      ├── assigned task (Coder)             │
      │         ▼                           │
      │   ┌───────────┐                     │
      │   │  WORKING  │─────────────────────┤
      │   │  (Coder)  │    task done/       │
      │   └─────┬─────┘    lost lease       │
      │         │                           │
      │         │ submitted for review      │
      │         ▼                           │
      │   ┌───────────┐      REJECTED       │
      │   │  WAITING  │◄────────────────────┤
      │   │  (Coder)  │                     │
      │   └─────┬─────┘                     │
      │         │                           │
      │         ├── APPROVED ───────────────┤
      │         │                           │
      │         │ context exhaustion        │
      │         ▼                           │
      │   ┌───────────┐                     │
      │   │  HANDOFF  │─────────────────────┘
      │   │           │    (terminated)
      │   └───────────┘
      │
      └── assigned review (Code Reviewer)
                ▼
          ┌───────────┐
          │ REVIEWING │─────────────────────┐
          │(Reviewer) │    verdict done/    │
          └─────┬─────┘    lost lease       │
                │                           │
                │ context exhaustion        │
                ▼                           │
          ┌───────────┐                     │
          │  HANDOFF  │─────────────────────┘
          │           │    (terminated)
          └───────────┘
```

**Registration Failure:** If agent registration fails (ID collision with active lease), the agent process exits immediately with error—it never enters the state machine. See [Roles — Agent Identity Protocol](roles.md#agent-identity-protocol).

---

## Goal State Machine

| State | Description | Valid Transitions |
|-------|-------------|-------------------|
| IN_PROGRESS | Goal active, sprints ongoing | → COMPLETED, ABORTED |
| COMPLETED | All planned work done | Terminal |
| ABORTED | Human stopped goal | Terminal |

Goals span sprints. Unlike sprints, goals have no CHECKPOINT state — checkpoint is a sprint-level concern.

---

## Sprint State Machine

| State | Description | Valid Transitions |
|-------|-------------|-------------------|
| IN_PROGRESS | Sprint active, work ongoing | → CHECKPOINT, COMPLETED, ABORTED |
| CHECKPOINT | Mandatory human review | → IN_PROGRESS (continue), ABORTED |
| COMPLETED | All planned tasks terminal | Terminal |
| ABORTED | Human or circuit breaker stopped | Terminal |

### Sprint Transition Triggers

| Trigger | Target State | Notes |
|---------|--------------|-------|
| All planned tasks terminal | COMPLETED | Normal completion |
| Calendar deadline reached | CHECKPOINT | Human decides continue/abort |
| `liza checkpoint` | CHECKPOINT | Manual review request |
| Circuit breaker triggered | ABORTED | Systemic issue detected |
| `liza stop` | ABORTED | Manual termination |

**From CHECKPOINT:**
- `liza resume` (planned tasks NOT all terminal) → IN_PROGRESS (continue same sprint)
- `liza resume` (all planned tasks terminal) → archives sprint, creates new sprint (IN_PROGRESS)
- `liza stop` → ABORTED (stop)

---

## Exit Codes

| Code | Meaning | Supervisor Action |
|------|---------|-------------------|
| 0 | Role-complete (see below) | Stop supervisor |
| 42 | Graceful abort (context exhaustion, lease lost, invariant) | Restart immediately |
| Other | Crash | Restart with backoff |

### Exit Code 0 Semantics

Exit 0 signals "this agent type has no more work to do." Supervisor should stop (not restart).

| Role | Exit 0 When |
|------|-------------|
| Planner | All tasks in terminal state, no blocked tasks, goal complete |
| Coder | No READY tasks AND no REJECTED tasks assigned to this agent |
| Code Reviewer | No READY_FOR_REVIEW tasks |

**Note:** Exit 0 does NOT mean "task done" (use state transitions for that). It means "role complete for this goal/sprint."

### DRAFT Task Waiting

When a Coder or Code Reviewer finds no work and would exit 0, the supervisor first checks for DRAFT tasks:

```
if agent_exit_code == 0 and count(DRAFT tasks) > 0:
    log("Agent found no work, but N DRAFT task(s) exist. Waiting for Planner...")
    sleep(coder_poll_interval)  # Configurable, default 30s
    restart_agent()  # Re-check after Planner may have finalized
```

**Rationale:** DRAFT tasks will become READY when the Planner finalizes them. Rather than stopping the supervisor (exit 0 behavior), we wait briefly and re-check. This avoids a race where:
1. Coder sees no READY tasks → exit 0
2. Planner finalizes DRAFT → READY
3. No Coder running to claim the newly-available task

The delay is configurable via `config.coder_poll_interval` (default 30s) — long enough for Planner to finalize without busy-waiting, short enough for reasonable responsiveness.

### Supervisor Backoff Timing

| Scenario | Delay | Rationale |
|----------|-------|-----------|
| Exit 42 (graceful abort) | 2s | Brief pause before restart |
| Crash (any exit code except 0, 42) | 5s | Recovery delay for unexpected failures |
| Exit 0 with DRAFT tasks | `coder_poll_interval` (30s) | Wait for Planner to finalize, then re-check |

**v1 Implementation (`f15cd61`):** Per-task `exit42RestartTracker` applies capped exponential backoff (2s, 4s, 8s, ... up to `exit42_max_backoff_seconds`, default 60s). Progress detection via task-state signature comparison resets the counter when meaningful state changes occur between restarts. After `exit42_restart_threshold` (default 5) consecutive restarts without progress, the task transitions to BLOCKED with diagnostic reason and questions. Configurable via `config.exit42_restart_threshold` and `config.exit42_max_backoff_seconds`.

### Graceful Abort Triggers (Exit 42)

Agent should exit with code 42 when:
- Context reaches 90% capacity (set `handoff_pending: true` first)
- Lease expired while working
- Tier 0 invariant violated
- PAUSE file detected
- ABORT file detected
- CHECKPOINT file detected

### Exit 42 with Handoff

When exit 42 is triggered by context exhaustion:

1. Agent sets `handoff_pending: true` on task
2. Agent writes handoff notes to blackboard
3. Agent exits with code 42
4. Supervisor restarts agent process (fresh LLM context)
5. On startup, restarted agent checks `handoff_pending`:
   - `true` → read handoff, clear flag, resume work
   - `false` → normal startup (exit was for pause/checkpoint, not context)

**Distinction:** Exit 42 always restarts the agent process. The `handoff_pending` flag tells the restarted agent whether to read handoff notes or proceed normally.

---

## Validation Rules

```yaml
task_states:
  valid:
    - DRAFT
    - READY
    - IMPLEMENTING
    - READY_FOR_REVIEW
    - REVIEWING
    - REJECTED
    - APPROVED
    - MERGED
    - BLOCKED
    - ABANDONED
    - SUPERSEDED
    - INTEGRATION_FAILED
  terminal:
    - MERGED
    - ABANDONED
    - SUPERSEDED

agent_states:
  - STARTING
  - IDLE
  - WORKING
  - REVIEWING  # Code Reviewer only
  - WAITING
  - HANDOFF

invariants:
  - "DRAFT task cannot have assigned_to"
  - "IMPLEMENTING task must have assigned_to"
  - "IMPLEMENTING task must have worktree"
  - "IMPLEMENTING task must have valid lease_expires"
  - "READY_FOR_REVIEW task must have review_commit"
  - "REVIEWING task must have reviewing_by"
  - "REVIEWING task must have review_lease_expires"
  - "REVIEWING task must have review_commit"
  - "REJECTED task must have rejection_reason"
  - "BLOCKED task must have blocked_reason"
  - "SUPERSEDED task must have superseded_by and rescope_reason"
  - "MERGED task must not have worktree"
  - "Agent WORKING must have task"
  - "Agent WORKING should have lease_expires in future (warning if expired beyond grace period of 60s — may indicate long-running operation)"
  - "No two agents assigned to same task"
  - "Task with integration_fix:true must have prior INTEGRATION_FAILED in history"
```

## Related Documents

- [Task Lifecycle](../protocols/task-lifecycle.md) — operational flow
- [Blackboard Schema](blackboard-schema.md) — state.yaml structure
- [Roles](roles.md) — who can trigger which transitions
