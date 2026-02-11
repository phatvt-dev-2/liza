# 10 - Loop Detection Self-Abort & Review Exhaustion

## Context and Problem Statement

Agents in MAS can enter infinite loops — repeating the same commands or close variations without progress. During the Mistral demo trace, a Code Reviewer ran unittest variations endlessly without issuing a verdict, consuming tokens rapidly with no useful output. No mechanism existed to detect or interrupt this behavior.

## Considered Options

1. **Timeout-only** — Kill agents after fixed duration
2. **Supervisor-side detection** — Watcher monitors command patterns and kills looping agents
3. **Agent self-abort with role-specific actions** — Agent detects own repetition and takes structured exit

## Decision Outcome

Chose **Option 3**: Agents self-detect loops and abort with role-appropriate actions.

### Rationale

Token consumption is the primary cost. A fast-pace loop burns tokens much faster than wall-clock time suggests — an immediate abort is needed, not a timeout. Supervisor-side detection would be more complex to implement and introduces latency between loop onset and intervention. Self-abort is the simplest mechanism that addresses the failure mode.

This is an initial attempt with heuristic thresholds. May be reconsidered based on future experience. If self-detection proves insufficient, supervisor-side detection remains a viable escalation.

### Architecture

**Loop Detection (all roles):**

| Trigger | Threshold |
|---------|-----------|
| Same command repeated | >3 times |
| Close variations (same base, different flags/pipes) | >5 times total |

"Meaningful progress" = new information that changes next action.

**Role-Specific Abort Actions:**

| Role | Log As | Then |
|------|--------|------|
| Coder | `retry_loop` | Mark task BLOCKED with diagnosis |
| Code Reviewer | `reviewer_loop` | Issue REJECTED with "insufficient information to complete review" |
| Planner | `spec_gap` | Pause for human input |

All roles exit with code 42 after logging.

**Review Exhaustion Protocol:**
When 2 different Code Reviewers fail to issue a verdict on the same task:
- Task marked BLOCKED with `review_exhaustion` reason
- Planner evaluates: spec unclear? done_when untestable? missing context?

**New anomaly types added to blackboard schema:**
- `reviewer_loop` — details: `count`, `command_pattern`
- `review_exhaustion` — details: `reviewers_failed`, `common_blocker`

**Enforcement:** Originally added to `specs/architecture/roles.md`, subsequently promoted to `contracts/MULTI_AGENT_MODE.md` (circuit breaker section) for higher enforcement authority.

### Consequences

**Positive:**
- Prevents unbounded token waste from agent loops
- Structured escalation preserves diagnostic information for Planner
- Review exhaustion catches systemic review failures, not just individual loops
- Builds on existing circuit breaker pattern (specs/protocols/circuit-breaker.md)

**Limitations accepted:**
- 3x/5x thresholds are heuristic — may cause false positives on legitimate retries
- Self-detection relies on agent compliance (same limitation as all contract rules)

---
*Reconstructed from commit 72bc374 (2026-01-31), with subsequent promotion to contract in 7af6035 (2026-02-03)*
