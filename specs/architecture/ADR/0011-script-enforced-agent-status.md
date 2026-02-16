# 11 - Script-Enforced Agent Status Transitions

> **Note:** Enforcement mechanism changed from bash scripts to Go CLI commands per [ADR-0012](0012-go-cli-replaces-bash-scripts.md). The principle (structural enforcement over behavioral compliance) is preserved.

## Context and Problem Statement

Agent status in the blackboard (WORKING, WAITING, IDLE, HANDOFF) was managed by agents themselves via contract compliance. This created a reliability gap: agents could complete operations (submit for review, submit verdict) without updating their own status, leaving the blackboard in an inconsistent state. The general direction is to move to scripts what can be moved, so agents don't do things partially.

## Considered Options

1. **Stronger contract language** — More explicit rules requiring agents to set status
2. **Validation-only** — Watcher detects stale/inconsistent status after the fact
3. **Script-enforced transitions** — State-modifying scripts atomically set agent status alongside task status

## Decision Outcome

Chose **Option 3**: Scripts that modify task state also atomically set the acting agent's status.

### Rationale

Structural enforcement is more reliable than behavioral compliance. Moving status management into scripts reduces cognitive load on agents and eliminates a class of partial-update bugs. If the script runs, the status is correct — no agent cooperation needed beyond calling the script.

### Architecture

**Scripts now set agent status atomically:**

| Script | Sets Agent Status |
|--------|-------------------|
| `liza-agent.sh` (planner setup) | `WORKING` |
| `liza-agent.sh` (planner teardown) | `IDLE` |
| `liza-submit-for-review.sh` | `WAITING` |
| `liza-submit-verdict.sh` | `IDLE` |
| `liza-handoff.sh` (new) | `HANDOFF` |

**New script — `liza-handoff.sh`:**
Handles context exhaustion for Coders. Atomically sets `handoff_pending`, records summary and next action in task history, and sets agent status to HANDOFF. Supervisor spawns replacement Coder with handoff context.

**New shared helper — `require_task_exists()`:**
Added to `liza-common.sh`. All state-modifying scripts validate task existence before proceeding.

### Consequences

**Positive:**
- Agent status always consistent with last operation performed
- Reduces cognitive load — agents focus on task work, not bookkeeping
- Atomic updates prevent partial blackboard states
- Context exhaustion handoff now has a structured mechanism

**Limitations accepted:**
- Scripts become the authoritative status managers — any new status transition requires a script update
- Agents that bypass scripts (direct yq manipulation) would create inconsistency

---
*Reconstructed from commit 7af6035 (2026-02-03), status enforcement portion only*
