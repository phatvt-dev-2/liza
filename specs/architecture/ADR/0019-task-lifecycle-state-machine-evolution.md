# 19 - Task Lifecycle State Machine Evolution

## Context and Problem Statement

The blackboard coordination model (ADR-0003) used generic state names — UNCLAIMED and CLAIMED — that don't clearly express what an agent is doing with a task. As the system plans to expand beyond coder/code-reviewer pairs to include spec writers, architecture reviewers, UX reviewers, and other worker/reviewer types, role-neutral but activity-descriptive state names were needed.

Additionally, no state existed to track active review — a task went from READY_FOR_REVIEW directly to APPROVED or REJECTED. Without a REVIEWING state, concurrent reviewer assignment couldn't be prevented at the state machine level.

## Considered Options

1. **Keep existing names, add REVIEWING** — Minimal change. UNCLAIMED/CLAIMED naming remains confusing alongside IMPLEMENTING/REVIEWING.
2. **Rename states + add REVIEWING + enforce transitions** — Clean slate with self-documenting names. All states describe the task's current activity.

## Decision Outcome

Chose **Option 2**:

### Architecture

**State renames:**
- `UNCLAIMED` → `READY` — task is available for any worker type to claim
- `CLAIMED` → `IMPLEMENTING` — a worker is actively working on the task

**New state:**
- `REVIEWING` — a reviewer has claimed the task and is actively reviewing it

**Full lifecycle:**
```
DRAFT → READY → IMPLEMENTING → READY_FOR_REVIEW → REVIEWING → APPROVED → MERGED
                     ↑                                  ↓
                     └──────────── REJECTED ←───────────┘
```

**Supporting changes:**
- `State.ReleaseAgent()` — canonical pattern for agent release (IDLE + clear current_task + clear lease), extracted to eliminate duplication across 4 call sites
- `State.AllPlannedTasksTerminal()` — shared predicate for sprint completion detection
- `SPRINT_COMPLETE` wake trigger — lowest-priority trigger that fires when all planned tasks reach terminal state (MERGED/ABANDONED/SUPERSEDED), prompting the planner to checkpoint and pause

### Rationale

Role-neutral naming (READY, IMPLEMENTING, REVIEWING) supports planned expansion to multiple worker/reviewer pair types without renaming again. "READY" and "IMPLEMENTING" describe task state, not agent role — a spec writer implementing a spec and a coder implementing code both put tasks in IMPLEMENTING.

The REVIEWING state is preventive — concurrent reviewers haven't been observed, but the state machine should prevent it structurally rather than relying on timing.

### Consequences

**Positive:**
- Self-documenting states — log output and dashboards immediately readable
- Concurrent reviewer prevention at the state machine level
- Clean agent release pattern reduces bugs in lifecycle transitions
- Sprint completion detection closes the planner lifecycle loop

**Limitations accepted:**
- Breaking change to existing blackboard files — any in-progress sprints need state values updated
- 93 files touched in the rename commit — broad surface area, mitigated by comprehensive test coverage

---
*Reconstructed from commits 97d7687..421ceef (2026-02-17)*
