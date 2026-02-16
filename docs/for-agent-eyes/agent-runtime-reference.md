# Agent Runtime Reference

Operational quick-reference for Liza agents. For authoritative protocols, see [MULTI_AGENT_MODE.md](~/.liza/MULTI_AGENT_MODE.md).

---

## How to Use This Document

1. Read [MULTI_AGENT_MODE.md](~/.liza/MULTI_AGENT_MODE.md) for protocols (roles, state machine, TDD, iteration, scope)
2. Read **your role section** below for logging duties and role-specific operations
3. Reference **Scripts**, **Blackboard Fields**, and **Anomaly Types** as needed

---

## CLI Commands Reference

All operations use the `liza` binary (assumed in PATH).

| Command | Purpose | Used By |
|---------|---------|---------|
| `liza get` | Read blackboard data | All |
| `liza validate [state.yaml]` | Validate blackboard state | All |
| `liza add-task --id X ...` | Add task to blackboard | Planner |
| `liza submit-for-review <task-id> <commit>` | Submit for review (sets agent status WAITING) | Coder |
| `liza submit-verdict <task-id> <verdict> [reason]` | Submit review verdict (sets agent status IDLE) | Code Reviewer |
| `liza mark-blocked <task-id> --reason "..."` | Mark task as blocked | Coder, Planner |
| `liza wt-delete <task-id>` | Delete worktree | Planner |
| `liza status` | Show system status | All |

> **Blackboard is read-only.** Agents MUST NOT edit `state.yaml` directly. All state mutations go through `liza` CLI commands or MCP tools. Direct edits bypass validation and produce invalid states.

---

## Blackboard Field Reference

Location: `.liza/state.yaml` (read via `liza get`, mutated via CLI/MCP tools only)

### Task Fields

| Field | Type | Set By | Description |
|-------|------|--------|-------------|
| `id` | string | Planner | Unique task identifier (kebab-case) |
| `description` | string | Planner | What to build (1-2 sentences) |
| `status` | enum | CLI/MCP | Current state (see MAM Task State Machine) |
| `priority` | int | Planner | 1 (highest) to 5 (lowest) |
| `spec_ref` | string | Planner | Path to spec, optionally with `#anchor` |
| `done_when` | string | Planner | Falsifiable completion criteria |
| `scope` | string | Planner | Functional area and boundaries |
| `depends_on` | array | Planner | Task IDs that must be MERGED first |
| `assigned_to` | string | Supervisor | Agent ID of assigned coder |
| `worktree` | string | Supervisor | Path to task worktree |
| `base_commit` | string | Supervisor | Integration HEAD at claim time |
| `iteration` | int | System | Current coder iteration (1-based) |
| `review_commit` | string | Coder | Commit SHA submitted for review |
| `rejection_reason` | string | Reviewer | Structured rejection feedback |
| `reviewing_by` | string | Supervisor | Agent ID of assigned reviewer |
| `review_lease_expires` | timestamp | System | Reviewer lease expiry |
| `approved_by` | string | Reviewer | Agent ID who approved |
| `blocked_reason` | string | Coder | What is blocking progress |
| `blocked_questions` | array | Coder | 1-3 questions that would unblock |
| `failed_by` | array | System | Coders who failed this task |
| `integration_fix` | bool | Supervisor | True if fixing INTEGRATION_FAILED |
| `handoff_pending` | bool | Coder | True during context exhaustion handoff |

### Agent Fields

| Field | Type | Description |
|-------|------|-------------|
| `role` | enum | `coder`, `code-reviewer`, `planner` |
| `status` | enum | `STARTING`, `IDLE`, `WORKING`, `REVIEWING`, `WAITING`, `HANDOFF` |
| `current_task` | string | Task ID currently assigned |
| `lease_expires` | timestamp | When lease expires |
| `heartbeat` | timestamp | Last heartbeat time |

### Discovery Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Discovery identifier |
| `by` | string | Agent who discovered |
| `during` | string | Task ID when discovered |
| `description` | string | What was discovered |
| `severity` | enum | `critical`, `high`, `medium`, `low` |
| `urgency` | enum | `immediate` (wakes Planner), `deferred` |
| `recommendation` | string | Suggested action |
| `converted_to_task` | string | Task ID if converted |

---

## Planner

**Protocols:** See MAM sections: Role Execution, Task State Machine (incl. Claimability Rule), Communication Protocol, Circuit Breaker (Loop Detection).

### Logging Duties

| Event | Log As |
|-------|--------|
| Two coders failed same task | `hypothesis_exhaustion` |
| Spec gap discovered | `spec_gap` |

---

## Coder

**Protocols:** See MAM sections: Role Execution, Iteration Protocol (incl. Context Exhaustion Handoff), Scope Discipline, TDD Enforcement, Circuit Breaker (Loop Detection).

### Logging Duties

| Event | Log As |
|-------|--------|
| >2 iterations on same error | `retry_loop` |
| Accepting suboptimal solution | `trade_off` |
| Spec doesn't cover case | `spec_ambiguity` |
| External service blocking | `external_blocker` |
| Spec assumption proven false | `assumption_violated` |

---

## Code Reviewer

**Protocols:** See MAM sections: Role Execution, Iteration Protocol (incl. Review Exhaustion), Scope Discipline, TDD Enforcement, Circuit Breaker (Loop Detection).

### Logging Duties

| Observation | Log As |
|-------------|--------|
| Coder retry loop visible | `retry_loop` |
| Implementation differs from spec | `scope_deviation` |
| Workaround taken | `workaround` |
| Technical debt introduced | `debt_created` |
| Spec assumption contradicted | `assumption_violated` |
| Spec changed since task creation | `spec_changed` |
| Own review stuck in command loop | `reviewer_loop` |

---

## Anomaly Types

Log anomalies as they occur using the `anomalies` section.

| Type | Logged By | Required Fields |
|------|-----------|-----------------|
| `retry_loop` | Coder, Reviewer | `count`, `error_pattern` |
| `trade_off` | Coder | `what`, `why`, `debt_created` |
| `spec_ambiguity` | Coder | — |
| `external_blocker` | Coder | `blocker_service` |
| `assumption_violated` | Coder, Reviewer | `assumption`, `reality` |
| `scope_deviation` | Reviewer | — |
| `workaround` | Reviewer | — |
| `debt_created` | Reviewer | — |
| `spec_changed` | Reviewer | — |
| `reviewer_loop` | Reviewer | `count`, `command_pattern` |
| `hypothesis_exhaustion` | Planner | — |
| `spec_gap` | Planner | — |
| `review_deadlock` | Planner | — |
| `review_exhaustion` | Planner | `reviewers_failed`, `common_blocker` |
| `system_ambiguity` | Any | `protocol_section`, `question` |

---

## Quick Reference

### Lease Model

- Lease duration: 5 minutes (default)
- Heartbeat interval: 60 seconds
- Extend lease before long operations
- If lease expires, task becomes reclaimable

### Exit Codes

| Code | Meaning | Supervisor Action |
|------|---------|-------------------|
| 0 | Role complete (no more work) | Stop agent |
| 42 | Graceful abort (handoff, loop detected) | Restart immediately |
| Other | Crash | Restart with backoff |

### Timestamps

Format: `YYYY-MM-DDTHH:MM:SSZ` (ISO 8601 UTC)
Generate: `date -u +%Y-%m-%dT%H:%M:%SZ`
