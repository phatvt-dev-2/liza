# 6 - Supervisor-Assigns-Work Model

## Context and Problem Statement

In a multi-agent system, who claims tasks? Two models are possible:
1. Agents compete to claim available work
2. Supervisors pre-assign work before spawning agents

The initial implementation had agents discovering and claiming their own work. **Agents were not reliably using the scripts** — they tended to imitate them, partially. An agent might try to claim a task by writing YAML directly instead of calling `liza-claim-task.sh`, getting the format slightly wrong or missing the lock.

## Considered Options

1. **Agent self-service** — Agents poll blackboard, claim available tasks
2. **Central dispatcher** — Single coordinator assigns all work
3. **Supervisor pre-assignment** — Each agent's supervisor claims before spawn

## Decision Outcome

Chose **Option 3**: Supervisors claim tasks/reviews BEFORE spawning agents.

### Rationale

**Simpler agent bootstrap.** Agents receive pre-claimed work in their bootstrap prompt:
- Coder: `=== ASSIGNED TASK ===` with task ID, worktree, spec reference
- Reviewer: `=== REVIEW TASK ===` with commit SHA, author, done_when criteria
- Planner: `=== PLANNING CONTEXT ===` with wake trigger, sprint state

The agent doesn't need logic to discover, evaluate, and claim work. It just executes what it's given.

**Eliminates race conditions.** With self-service claiming, two agents might try to claim the same task simultaneously. Pre-assignment by supervisor (which holds a lock) prevents this.

**Clearer responsibility.** "Supervisor" = the enclosing bash loop per agent, not a singleton. Each supervisor is responsible for exactly one agent's work assignment.

### Architecture

```
┌──────────────────┐
│ liza-agent.sh    │ (supervisor)
│                  │
│ 1. Poll blackboard
│ 2. Find claimable work
│ 3. Claim with lock     ──────────────▶  .liza/state.yaml
│ 4. Prepare worktree                     (task.status = IMPLEMENTING)
│ 5. Build bootstrap prompt
│ 6. Spawn Claude         ──────────────▶  claude --print ...
│ 7. Wait for completion
│ 8. Handle result
└──────────────────┘
```

**Bootstrap prompt structure:**
```
=== ASSIGNED TASK ===
TASK ID: implement-hello-cli
WORKTREE: .worktrees/implement-hello-cli
SPEC_REF: specs/vision.md
DONE_WHEN: |
  - python -m hello prints "Hello, World!"
  - python -m hello --name Alice prints "Hello, Alice!"
...
```

### Consequences

**Positive:**
- Agent prompts are simpler — no claiming logic needed
- No race conditions on task claiming
- Clear audit trail — supervisor logged the claim before agent saw it
- Agents can focus on their actual work (coding, reviewing)

**Limitations accepted:**
- Supervisor must understand task eligibility (dependencies, role matching)
- If supervisor crashes after claim but before spawn, task is orphaned (addressed by lease expiry)

---
*Reconstructed from commits 30892c8, 42d32fa (2026-01-20 to 2026-01-21)*
