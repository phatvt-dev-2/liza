# Liza v0.5.3

Session-persistent review flow: blocking await primitives eliminate cold
restarts across review cycles, restoring the original vision of
attempt-scoped agent sessions. Prompt and contract hardening across the board.

26 commits since v0.5.2 (2026-03-29).

---

## Highlights

- **Blocking Await Primitives** — Two new MCP tools (`liza_await_verdict`, `liza_await_resubmission`) let doer and reviewer agents block in-session while waiting for the review cycle to complete. Eliminates the cold-restart penalty where agents lost all session context between submit and re-work. Doers call `liza_await_verdict` after submission and wake when the verdict lands; reviewers call `liza_await_resubmission` after requesting changes and wake when the doer resubmits.
- **ReviewingBy Ownership Guard** — `isClaimablePipeline` now rejects reviewer claims when another reviewer holds `ReviewingBy` ownership, preventing double-review races during await windows.
- **Prompt Hardening** — Eight prompt fixes tighten agent behavior: TDD colocation rule, parallelism objective in task decomposition, reviewer test triage, worktree path resolution, bash `!` history expansion warning, reviewer build-system guardrail, agent lifecycle clarification, and init-ordering fix.

---

## Features

- `liza_await_verdict` — doer-side blocking MCP tool for submit-then-wait flow (#8)
- `liza_await_resubmission` — reviewer-side blocking MCP tool for session persistence across re-review cycles (#9)
- `ReviewingBy` guard in `isClaimablePipeline` prevents double-review claims
- `ClearStaleReviewClaims` extended to recover from reviewer crashes during await
- Parallelism objective added to task decomposition prompts
- TDD colocation rule added to task decomposition prompts

---

## Fixes

| Fix | Impact |
|-----|--------|
| Resolve PlanRef relative to worktree path | Agents in worktrees could not find their plan files |
| Read contract files before `liza_get` in FIRST ACTIONS | Init ordering prevented agents from having contract context during first tool calls |
| Prevent worktree rules from overriding pre-resolved init paths | Worktree agents re-resolved paths incorrectly |
| Tighten reviewer test triage and fix worktree cd pattern | Reviewers ran tests from wrong directory |
| Whitelist Glob in enforce-init gate | Glob tool calls were blocked during initialization |
| Warn agents about bash history expansion with `!` | Agents using `!` in bash commands triggered unexpected history expansion |
| Add reviewer guardrail against build-system investigation | Reviewers wasted cycles investigating build tooling outside their scope |
| Clarify agent lifecycle — stop, don't exit | Agents attempted to exit instead of stopping cleanly |
| Remove duplicate HEARTBEAT and dead CONTEXT columns, widen ROLE/CURRENT_TASK | TUI displayed redundant columns and truncated useful ones |
| Update regression guard for renamed exit codes heading | Test assertion matched stale heading text |

---

## Contracts & Prompts

- RTK proxy instructions hardened across three iterations (tighter wording, allow `rtk proxy` in settings)
- MCP tool preferences strengthened from "default" to "required" in AGENT_TOOLS.md
- Stop agents re-reading CORE.md already loaded as system prompt
- Clean-code and code-review skills compressed without signal loss (−116 lines net)

---

## Documentation

- ADR-0053: Supervisor Resilience — Automated Failure Detection
- ADR-0054: Blocking Await Primitives for Review Flow
- Troubleshooting aids migrated from retired agent-runtime-reference to SUPPORT.md
- Goal guide vendor-neutralized and review step added
- L4 positioning quote added to README
- TUI screenshot updated

---

## Installation

**Quick install (macOS/Linux):**
```bash
curl -fsSL https://raw.githubusercontent.com/liza-mas/liza/main/install.sh | bash
```

**From source:**
```bash
go install ./cmd/liza/ ./cmd/liza-mcp/
```

---

## What's Next

- **TUI Phase 2** — panel navigation, item selection, log filtering
- **Integration sub-pipeline** — validate commit batches before merge to main
- **Sprint Analyzer** role — analyze agent logs via lesson-capture
