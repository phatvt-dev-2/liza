# 23 - Crash Recovery Commands

## Context and Problem Statement

Agent crashes — most commonly from LLM provider quota exhaustion — left the system in a state requiring manual multi-step recovery: release the task claim, remove the worktree, delete the branch, delete the agent registration, and optionally respawn. This 5-6 command sequence was error-prone and painful during active sprints, where multiple agents could crash in quick succession.

Some failure causes (quota exhaustion, provider outages, OOM) cannot be auto-recovered by the supervisor, so a general human-initiated recovery mechanism was needed rather than automatic detection.

## Considered Options

1. **Keep manual multi-step recovery** — document the procedure, accept the operational cost.
2. **Add automated supervisor-side crash recovery** — detect and recover without human intervention.
3. **Add single-command recovery tools** — human-initiated but automated cleanup.

## Decision Outcome

Chose **Option 3**: two complementary commands covering both entry points.

### Architecture

**`liza recover-agent <agent-id>`** (implemented first):
- Auto-detects role from blackboard state
- Role-specific cleanup: coder (IMPLEMENTING → READY, clear worktree/branch, delete agent), reviewer (REVIEWING → READY_FOR_REVIEW, delete agent)
- PID liveness check (refuses if agent still alive; `--force` overrides)
- Optional respawn via `--cli` flag using `syscall.Exec`
- Idempotent — safe to run twice

**`liza recover-task <task-id>`** (added to cover the gap):
- `recover-agent` left the *task* stuck — agent was cleaned up but task remained claimed
- Releases claims, removes worktree/branch, recovers claiming agent(s)
- Normal mode: requires task in state, checks PID liveness
- Force mode: cleans git artifacts even when task is absent from state (orphaned worktrees after hard crashes)
- Audited via `human_notes`

Both commands follow the ops service layer pattern (ADR-0021).

### Rationale

Human-initiated recovery was chosen over auto-recovery because many crash causes are external (quota, provider outage) and may require human judgment about whether to respawn or wait. A general-purpose tool is more valuable than heuristic auto-detection that only handles some failure modes.

Two commands emerged because the natural entry points differ: operators sometimes know the agent ID, sometimes the task ID (visible in `liza status` or `.worktrees/`). `recover-agent` was implemented first but left a gap — the task remained stuck — which `recover-task` filled by also handling the task-side state.

### Consequences

**Positive:**
- Recovery from crash reduced from 5-6 manual commands to one
- Idempotent and PID-aware — safe even under uncertainty
- Role-aware cleanup handles coder/reviewer differences automatically
- Force mode handles orphaned state after hard crashes

**Limitations accepted:**
- Human must identify *which* agent or task crashed (not auto-detected)
- Respawn via `--cli` requires knowing the correct CLI binary path

---
*Reconstructed from commits d769aab..7f7f130 (2026-02-23 to 2026-02-26)*
