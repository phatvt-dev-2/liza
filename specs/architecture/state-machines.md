# State Machines

## Task State Machine

| State | Description | Valid Transitions |
|-------|-------------|-------------------|
| DRAFT | Task being defined by orchestrator | → READY |
| READY | Task ready, no agent assigned | → IMPLEMENTING |
| IMPLEMENTING | Coder assigned, work in progress | → READY_FOR_REVIEW, BLOCKED |
| READY_FOR_REVIEW | Coder done, awaiting Code Reviewer | → REVIEWING |
| REVIEWING | Reviewer assigned, review in progress | → APPROVED, REJECTED, READY_FOR_REVIEW (stale lease) |
| REJECTED | Code Reviewer rejected, feedback provided | → IMPLEMENTING (supervisor reclaims for coder) |
| APPROVED | Code Reviewer approved, merge eligible | → MERGED, INTEGRATION_FAILED |
| MERGED | Successfully merged to integration | Terminal |
| BLOCKED | Cannot proceed, awaiting escalation | → SUPERSEDED, ABANDONED |
| SUPERSEDED | Replaced by rescoped task(s) | Terminal |
| ABANDONED | Orchestrator killed task | Terminal |
| INTEGRATION_FAILED | Merge conflict or integration test failure | → IMPLEMENTING (integration-fix scope) |

### Task State Diagram

```
     ┌──────────────────────────────────────────┐
     │              DRAFT                       │
     │  (Orchestrator writing, coders must ignore)   │
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

> **Note:** The `depends_on` terminal condition (`all depends_on are MERGED`) applies to the current
> hardcoded system. The [Sub-pipelines spec](../build/2%20-%20Sub-pipelines and spec writing.md) generalizes this:
> a dependency is satisfied when it reaches its role-pair's **successful** sprint-terminal state
> (e.g., CODING_PLAN_APPROVED for code-planning-pair, US_APPROVED for us-writing-pair, MERGED for
> coding-pair). The `role_pair` field on each dependency task determines which terminal applies.
> ABANDONED and SUPERSEDED do **not** satisfy dependencies — they indicate the upstream work was
> dropped or replaced. When a dependency is SUPERSEDED, the dependent task should be re-evaluated
> against the replacement task(s) referenced in `superseded_by`.

> **Phase-gate dependency propagation:** When pipeline transitions create child tasks from
> parent tasks that have `depends_on`, the children automatically inherit dependencies on
> upstream parents' children (from the same transition). This propagates the phase-gate
> barrier: if plan-2 depends on plan-1, then plan-2's coding children depend on plan-1's
> coding children. The inherited deps are appended after sibling deps (from `output[]` DependsOn
> indices). This is transition-specific — only the same transition name contributes.

When new task types are added (e.g., `specification`, `architecture`), they define their own role workflow in the registry. The supervisor and work detection derive behavior from the registry rather than hardcoding role checks.

> **Note:** The `task.type` field and type registry are superseded by the `role_pair` field
> for claimability and state resolution — see [Sub-pipelines spec](../build/2%20-%20Sub-pipelines and spec writing.md)
> §Task model extension. The `type` field may remain as a human-readable category.

### Code-Planning Pair State Machine

The code-planning pair introduces a parallel state cycle for plan creation and review,
analogous to the coding pair (IMPLEMENTING → READY_FOR_REVIEW → REVIEWING → APPROVED/REJECTED).

| State | Description | Valid Transitions |
|-------|-------------|-------------------|
| DRAFT_CODING_PLAN | Task created, awaiting Code Planner claim | → CODE_PLANNING |
| CODE_PLANNING | Code Planner working | → CODING_PLAN_TO_REVIEW, BLOCKED, DRAFT_CODING_PLAN |
| CODING_PLAN_TO_REVIEW | Code Planner done, awaiting Code Plan Reviewer | → REVIEWING_CODING_PLAN |
| REVIEWING_CODING_PLAN | Code Plan Reviewer active | → CODING_PLAN_APPROVED, CODING_PLAN_REJECTED, CODING_PLAN_TO_REVIEW (stale lease) |
| CODING_PLAN_REJECTED | Code Plan Reviewer rejected, feedback provided | → DRAFT_CODING_PLAN (supervisor reclaims for planner) |
| CODING_PLAN_APPROVED | Code Plan Reviewer approved | Sprint-terminal (transition to coding pair via orchestrator after planning checkpoint) |

```
     ┌────────────────────────────────────────────┐
     │          DRAFT_CODING_PLAN                  │
     │  (Orchestrator created, planners claim)     │
     └──────────────────┬─────────────────────────┘
                        │ claim
                        ▼
     ┌────────────────────────────────────────────┐
     │           CODE_PLANNING                     │
     │       (Code Planner working)                │
     └─────────┬────────────────────┬─────────────┘
               │                    │
     submit_plan              blocked
               │                    │
               ▼                    ▼
     ┌─────────────────────┐ ┌──────────────┐
     │CODING_PLAN_TO_REVIEW│ │   BLOCKED    │
     └────────┬────────────┘ └──────────────┘
              │
       assign_reviewer
              │
              ▼
     ┌────────────────────────┐
     │ REVIEWING_CODING_PLAN  │
     └────────┬───────────────┘
       ┌──────┴──────┐
       │             │
    approve       reject
       │             │
       ▼             ▼
┌─────────────────────┐  ┌─────────────────────┐
│CODING_PLAN_APPROVED │  │CODING_PLAN_REJECTED │
└─────────────────────┘  └─────────┬───────────┘
  Sprint-terminal                  │
  (→ coding pair via               resume (DRAFT_CODING_PLAN)
   orchestrator after checkpoint)
```

**Sprint-terminal:** CODING_PLAN_APPROVED is terminal for sprint completion (alongside MERGED, ABANDONED, SUPERSEDED).
The transition CODING_PLAN_APPROVED → DRAFT (coding pair) is executed by the orchestrator after the human resumes a `PLANNING_COMPLETE` checkpoint. See [Planning Transition Gate](../protocols/sprint-governance.md#planning-transition-gate).

**Claimability:**

| Role | Claims | States |
|------|--------|--------|
| Code Planner (`code_planner`) | DRAFT_CODING_PLAN | Doer role (supervisor transitions CODING_PLAN_REJECTED → DRAFT_CODING_PLAN first) |
| Code Plan Reviewer (`code_plan_reviewer`) | CODING_PLAN_TO_REVIEW | Reviewer role |

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
| BLOCKED → SUPERSEDED | `superseded_by`, `rescope_reason`, status=SUPERSEDED | `failed_by` | Orchestrator links blocked task to replacement task(s) |
| BLOCKED → ABANDONED | status=ABANDONED | `failed_by` | Orchestrator abandons blocked task when no viable continuation exists |
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

> **Note:** The review lease below references READY_FOR_REVIEW and Code Reviewer specifically.
> The [Sub-pipelines spec](../build/2%20-%20Sub-pipelines and spec writing.md) generalizes this: tasks at any
> role-pair `submitted` state are reviewable by that pair's reviewer role.

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
| IDLE | No task assigned | → WORKING (doer), REVIEWING (reviewer) |
| WORKING | Doer actively executing task | → WAITING, IDLE, HANDOFF |
| REVIEWING | Reviewer actively reviewing task | → IDLE (verdict done), HANDOFF |
| WAITING | Doer waiting for review result or escalation | → WORKING (continue after feedback), IDLE (task done) |
| HANDOFF | Context exhaustion, preparing handoff notes | → (agent terminates, supervisor restarts fresh) |

**Role-specific states:**
- WORKING and WAITING: Doer roles — current: Coder; planned ([Sub-pipelines spec](../build/2%20-%20Sub-pipelines and spec writing.md)): Code Planner, Epic Planner, US Writer
- WORKING (no WAITING): Dispatcher roles — planned: Orchestrator (pipeline dispatch only — no reviewer, no output[], no sprint-terminal state; creates initial task then exits)
- REVIEWING: Reviewer roles — current: Code Reviewer; planned: Code Plan Reviewer, Epic Plan Reviewer, US Reviewer
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
| COMPLETED | All planned tasks terminal | → IN_PROGRESS (via `liza resume`: archive + new sprint) |
| ABORTED | Human or circuit breaker stopped | Terminal |

### Sprint Transition Triggers

| Trigger | Target State | Notes |
|---------|--------------|-------|
| All planned tasks terminal | COMPLETED | Normal completion |
| Calendar deadline reached | CHECKPOINT | Human decides continue/abort |
| `liza sprint-checkpoint` | CHECKPOINT | Manual review request |
| All non-terminal planned tasks BLOCKED | CHECKPOINT | Sprint stalled — human intervention needed |
| Circuit breaker triggered | ABORTED | Systemic issue detected |
| `liza stop` | ABORTED | Manual termination |

**From CHECKPOINT:**
- `liza resume` (planned tasks NOT all terminal) → IN_PROGRESS (continue same sprint)
- `liza resume` (all planned tasks terminal) → COMPLETED (marks sprint done for human review)
- `liza stop` → ABORTED (stop)

**From COMPLETED:**
- `liza proceed <task-id> <transition>` → creates child tasks from parent task's `output[]` (manual transitions)
- `liza resume` → archives sprint, creates new sprint (IN_PROGRESS) with child tasks

**Sprint advance flows:**

*Planning tasks (automatic transitions):*
1. Planning tasks merged → orchestrator checkpoints with `checkpoint_trigger: PLANNING_COMPLETE`
2. Human reviews planning output → `liza resume` → IN_PROGRESS
3. Orchestrator PreWork detects trigger + IN_PROGRESS → executes transitions → child tasks created
4. Sprint continues or completes → next `liza resume` archives sprint

*Manual transitions:*
1. CHECKPOINT + all terminal → `liza resume` → COMPLETED (human review gate)
2. Human runs `liza proceed` to create child tasks from approved plans
3. Human runs `liza resume` → archives sprint, creates new sprint with child tasks

---

## Exit Codes

> **Note:** The exit semantics below are role-specific to the current system (Orchestrator, Coder, Code Reviewer).
> The [Sub-pipelines spec](../build/2%20-%20Sub-pipelines and spec writing.md) generalizes these: "no work" detection
> will be derived from the configured role-pair states (e.g., doer exits 0 when no tasks at
> `role-pair.initial` or `role-pair.rejected`; reviewer exits 0 when no tasks at `role-pair.submitted`).
> The DRAFT Task Waiting logic generalizes to: any doer waiting for upstream pairs to produce work.

| Code | Meaning | Supervisor Action |
|------|---------|-------------------|
| 0 | Role-complete (see below) | Stop supervisor |
| 42 | Graceful abort (context exhaustion, lease lost, invariant) | Restart immediately |
| Other | Crash | Restart with backoff |

### Exit Code 0 Semantics

Exit 0 signals "this agent type has no more work to do." Supervisor should stop (not restart).

| Role | Exit 0 When |
|------|-------------|
| Orchestrator | All tasks in terminal state, no blocked tasks, goal complete |
| Coder | No READY tasks AND no REJECTED tasks assigned to this agent |
| Code Reviewer | No READY_FOR_REVIEW tasks |

**Note:** Exit 0 does NOT mean "task done" (use state transitions for that). It means "role complete for this goal/sprint."

### DRAFT Task Waiting

When a Coder or Code Reviewer finds no work and would exit 0, the supervisor first checks for DRAFT tasks:

```
if agent_exit_code == 0 and count(DRAFT tasks) > 0:
    log("Agent found no work, but N DRAFT task(s) exist. Waiting for Orchestrator...")
    sleep(coder_poll_interval)  # Configurable, default 30s
    restart_agent()  # Re-check after Orchestrator may have finalized
```

**Rationale:** DRAFT tasks will become READY when the Orchestrator finalizes them. Rather than stopping the supervisor (exit 0 behavior), we wait briefly and re-check. This avoids a race where:
1. Coder sees no READY tasks → exit 0
2. Orchestrator finalizes DRAFT → READY
3. No Coder running to claim the newly-available task

The delay is configurable via `config.coder_poll_interval` (default 30s) — long enough for Orchestrator to finalize without busy-waiting, short enough for reasonable responsiveness.

### Supervisor Backoff Timing

| Scenario | Delay | Rationale |
|----------|-------|-----------|
| Exit 42 (graceful abort) | 2s | Brief pause before restart |
| Crash (any exit code except 0, 42) | 5s | Recovery delay for unexpected failures |
| Exit 0 with DRAFT tasks | `coder_poll_interval` (30s) | Wait for Orchestrator to finalize, then re-check |

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

> **Note:** The task states below reflect the current hardcoded state machine.
> The [Sub-pipelines spec](../build/2%20-%20Sub-pipelines and spec writing.md) makes these config-driven —
> valid states, terminals, and transitions will be derived from the pipeline YAML.

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
    # Code-planning pair states
    - DRAFT_CODING_PLAN
    - CODE_PLANNING
    - CODING_PLAN_TO_REVIEW
    - REVIEWING_CODING_PLAN
    - CODING_PLAN_APPROVED
    - CODING_PLAN_REJECTED
  terminal:
    - MERGED
    - ABANDONED
    - SUPERSEDED
  sprint_terminal:
    - MERGED
    - ABANDONED
    - SUPERSEDED
    - CODING_PLAN_APPROVED

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
  # Code-planning pair invariants
  - "DRAFT_CODING_PLAN task cannot have assigned_to"
  - "CODE_PLANNING task must have assigned_to"
  - "CODE_PLANNING task must have worktree"
  - "CODE_PLANNING task must have base_commit"
  - "CODE_PLANNING task must have lease_expires"
  - "CODING_PLAN_TO_REVIEW task must have review_commit"
  - "REVIEWING_CODING_PLAN task must have reviewing_by"
  - "REVIEWING_CODING_PLAN task must have review_lease_expires"
  - "REVIEWING_CODING_PLAN task must have review_commit"
  - "CODING_PLAN_APPROVED task must have review_commit"
  - "CODING_PLAN_REJECTED task must have rejection_reason"
  - "Agent WORKING must have task"
  - "Agent WORKING should have lease_expires in future (warning if expired beyond grace period of 60s — may indicate long-running operation)"
  - "No two agents assigned to same task"
  - "Task with integration_fix:true must have prior INTEGRATION_FAILED in history"
```

## Related Documents

- [Task Lifecycle](../protocols/task-lifecycle.md) — operational flow
- [Blackboard Schema](blackboard-schema.md) — state.yaml structure
- [Roles](roles.md) — who can trigger which transitions
