# Liza

A disciplined peer-supervised multi-agent coding system.

## Overview

Liza combines four ideas:
- **Behavioral contracts** for per-agent discipline—Tier 0 invariants are never violated
- **Specification system** for durable context—agents are stateless, specs persist understanding across restarts
- **Blackboard coordination** for visible state—all coordination happens through a shared file humans can observe
- **Externally validated completion** with Ralph-like loops—Coders cannot self-certify; Code Reviewers issue binding verdicts

## Domains

- 1.1 — [Contract System](1.1.md) — Behavioral rules governing agent conduct
- 1.2 — [Multi-Agent Coordination](1.2.md) — Blackboard, roles, state machines
- 1.3 — [Task Management](1.3.md) — Claim, iterate, review, merge flow
- 1.4 — [Worktree Isolation](1.4.md) — Per-task git worktrees
- 1.5 — [Circuit Breaker](1.5.md) — Systemic failure detection and recovery
- 1.6 — [Skills](1.6.md) — Domain-specific agent protocols

## Key Integrations

| System | Role |
|--------|------|
| Claude Code CLI | Agent runtime — executes agents with mode-based prompting |
| Bash/Shell | Script execution — all Liza mechanics are shell scripts |
| YAML + flock | Blackboard persistence — atomic read-modify-write coordination |
| Git worktrees | Isolation — each task gets its own working directory |
| Git | Version control — standard operations, merge protocol |

## MVP Scope (v1)

**In scope:**
- Single goal, single sprint at a time
- One Planner, one Coder, one Code Reviewer
- Terminal-based observation
- YAML blackboard with file locking
- Shell script tooling
- Human-triggered circuit breaker

**Explicit out of scope:**

| Feature | Rationale |
|---------|-----------|
| IDE integration | Terminal workflow sufficient; IDE adds complexity |
| Web dashboard | `tail -f` and `watch` sufficient for observation |
| Multi-repo coordination | Single repo focus for v1 |
| Parallel coders | Validate sequential first |
| Real-time circuit breaker | Human-triggered analysis sufficient |
| Token budget tracking | API doesn't expose; calendar time proxy |
| SQLite backend | YAML sufficient at v1 scale |
| MCP server integration | Nice-to-have, not essential |

## Risks and Assumptions

See [Vision](../build/0%20-%20Vision.md) for detailed risk analysis.

Key assumptions:
- Claude Code CLI supports mode-based prompting
- Agents can reliably call shell scripts
- YAML + flock sufficient for coordination
- Specs substantially complete before work begins

---
*Status: active*
*Last verified: 2026-02-02*
