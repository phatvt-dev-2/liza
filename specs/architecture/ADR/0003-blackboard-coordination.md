# 3 - Blackboard Coordination Architecture

## Context and Problem Statement

Multiple agents need to coordinate autonomously. The behavioral contract (ADR-0001) disciplines individual agents, but agents need a mechanism to:
- Claim and hand off tasks
- Communicate approval/rejection decisions
- Make all state visible for human observation
- Support the gate mechanism without human-in-loop

## Considered Options

1. **Direct agent-to-agent communication** — Agents message each other
2. **Central orchestrator** — A meta-agent routes all communication
3. **Shared file-based blackboard** — Agents read/write shared state files
4. **Database-backed state** — PostgreSQL or similar for coordination

## Decision Outcome

Chose **Option 3**: Shared file-based blackboard in `.liza/` directory.

### Rationale

**No conversation between agents.** Read state, do work, write state. This eliminates the failure mode where agents negotiate, compromise, or drift collectively through dialogue.

**Everything visible, everything auditable.** The blackboard is human-readable YAML. You can `cat .liza/state.yaml` at any moment. No hidden handshakes between agents.

**Gates without human bottleneck.** The contract requires externalized reasoning before action. In pairing mode, that goes to the human. In MAS mode, it writes to the blackboard — same forcing function, different audience.

**Simplicity for POC.** File-based coordination is simple to implement, debug, and understand. The decision to use bash scripts (ADR-0005) reinforces this — both choices prioritize quick iteration over infrastructure complexity.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         .liza/                              │
├─────────────────────────────────────────────────────────────┤
│  state.yaml    │ Current state (atomic read-modify-write)  │
│  log.yaml      │ Activity history (append-only)            │
│  alerts.log    │ Watcher alerts (append-only)              │
│  archive/      │ Completed tasks (periodic pruning)        │
└─────────────────────────────────────────────────────────────┘
                              │
          ┌───────────────────┼───────────────────┐
          ▼                   ▼                   ▼
    ┌───────────┐        ┌──────────┐        ┌──────────┐
    │ Planner   │        │  Coder   │        │ Reviewer │
    │           │        │          │        │          │
    │ Reads     │        │ Claims   │        │ Reads    │
    │ goal,     │        │ tasks,   │        │ commits, │
    │ writes    │        │ writes   │        │ writes   │
    │ tasks     │        │ status   │        │ verdicts │
    └───────────┘        └──────────┘        └──────────┘
```

**Key state transitions in blackboard:**
- `READY` → `IMPLEMENTING` (Coder claims task)
- `IMPLEMENTING` → `READY_FOR_REVIEW` (Coder submits work)
- `READY_FOR_REVIEW` → `APPROVED` / `REJECTED` (Reviewer decides)
- `APPROVED` → `MERGED` (Supervisor merges)

**Atomic operations:** Scripts use `liza-lock.sh` for read-modify-write to prevent race conditions.

### Consequences

**Positive:**
- Full audit trail — every state change logged with timestamp and agent
- Human can observe, pause (`PAUSE` file), or intervene at any moment
- Agents are stateless — "every restart is a new mind with old artifacts"
- Simple debugging — `git diff .liza/state.yaml` shows exactly what changed

**Limitations accepted:**
- File-based locking is adequate for POC but may need revisiting at scale
- No real-time notification — agents poll the blackboard
- YAML can become unwieldy for large task lists (addressed by archiving)

---
*Reconstructed from commit 9f91405 (2026-01-19)*
