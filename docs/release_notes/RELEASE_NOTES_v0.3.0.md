# Liza v0.3.0

The orchestrator hardens its runtime:
- an ops service layer decouples business logic from the MCP transport,
- role boundaries are enforced at the tool level,
- pre-execution checkpoints and TDD gates raise the quality floor,
- crash recovery commands replace manual workflows,
- and multi-sprint support lets projects run continuously without blackboard resets.

A cross-cutting theme is context pressure reduction — role-specific prompt templates cut per-agent prompt size by 58%,
contract compression saves 69 lines per session, and new log analysis tools make agent behavior observable for the first time.

147 commits since v0.2.0 (2026-02-18).

---

## Breaking Changes

- **`liza_checkpoint` renamed to `liza_sprint_checkpoint`** across MCP tools, CLI, and prompt templates. Any external automation referencing the old name must update.
- **Reviewer naming normalized**: `reviewer-1`/`reviewer` → `code-reviewer-1`/`code-reviewer` across the entire codebase, including `release-claim` CLI role vocabulary.
- **MCP role enforcement**: Agents can no longer invoke tools outside their role boundary (e.g., coders cannot call `liza_wt_merge` or `liza_submit_verdict`).
- **Pre-execution checkpoint required**: Coders must write a checkpoint via `liza_write_checkpoint` before submitting for review.
- **TDD enforcement**: Code task submissions without test files are rejected unless a `tdd_not_required` waiver is provided. Detection covers Go, Python, JS/TS, Shell, Ruby, Java, Kotlin, and Rust.
- **TodoWrite forbidden in Multi-Agent Mode**: Agents using TodoWrite will now be rejected — blackboard already tracks task state.
- **MCP request size limit**: JSON-RPC requests exceeding 10MB are rejected.

---

## Highlights

- **Ops service layer** (ADR-0021) — All 17 MCP-exposed mutation commands extracted into `internal/ops/` with typed result structs. Handlers became thin presentation wrappers (~20–50 LOC each), eliminating stdout writes that could corrupt the JSON-RPC stream. Package coverage: 0.8% → 81.7%.
- **Crash recovery commands** (ADR-0023) — `liza recover-agent` and `liza recover-task` replace a manual 5–6 step workflow. Auto-detect role, perform role-specific cleanup, check PID liveness, optionally respawn.
- **Multi-sprint support** (ADR-0028) — Sprints advance automatically on `liza resume` when at CHECKPOINT with all planned tasks terminal. Archive-before-mutate safety guarantees no state mutation if archive write fails.
- **Code-enforced agent guardrails** (ADR-0030) — Pre-execution checkpoints, TDD gates, and role-boundary enforcement at the MCP handler level.
- **Context pressure reduction and observability** (ADR-0026, 0027, 0029) — Role-specific prompt templates cut per-agent size by 58% (ADR-0026), contract compression saves 69 lines per MAM session (ADR-0027), and agent log analysis tools provide first-time visibility into token usage, tool frequency, struggle sequences, and context growth (ADR-0029).

---

## Features

**Runtime Hardening**
- Ops service layer decouples business logic from MCP transport (ADR-0021)
- Singleton blackboard — `db.For(statePath)` returns shared instance per path, replacing ~30 independent `db.New()` calls (ADR-0022)
- MCP role-boundary enforcement via `requireRole` on 8 handlers (ADR-0030)
- Bounded MCP request size (10MB) on stdio transport
- Iteration/review limit enforcement — exhausted REJECTED loops transition to BLOCKED with escalation metadata
- Parallel merge CAS safety — `wt-merge` uses compare-and-swap `update-ref` retries under contention

**Crash Recovery**
- `liza recover-agent <agent-id>` — one-command crash recovery, idempotent, PID-liveness check, optional `--cli` to respawn (ADR-0023)
- `liza recover-task <task-id>` — task-oriented complement: releases claims, removes worktree/branch, recovers claiming agent(s)

**Multi-Sprint**
- Sprint.Number field, SprintSummary, SprintHistory for lightweight completed-sprint records in `state.yaml` (ADR-0028)
- Automatic advancement on `liza resume` at CHECKPOINT with all planned tasks terminal
- Legacy Number=0 normalized to 1 for backward compatibility

**Quality Gates**
- Pre-execution checkpoint: intent, validation plan, files to modify, assumptions, risks (ADR-0030)
- TDD enforcement gate in SubmitForReview — rejects code submissions without test files (Go, Python, JS/TS, Shell, Ruby, Java, Kotlin, Rust)
- `tdd_not_required` waiver for legitimate exceptions (cosmetic fixes, comment edits)

**Agent Log Analysis** (ADR-0029)
- `--log` flag for `liza agent` persists stdout/stderr to `.liza/agent-outputs/`
- `scripts/analyze-log.py` — stdlib-only Python CLI for batch session analysis
- `liza-session-analyzer.html` — drag-and-drop browser app with charts
- Reports: token usage, tool frequency, context growth, struggle sequences, efficiency insights

**Operational**
- Auto-sync embedded assets on worktree creation (`make sync-embedded`)
- Circuit breaker watch automation with hardened error handling
- Heartbeat interval wired from config with bounds validation
- Exit-42 restart backoff — capped exponential (2s, 4s, 8s…), exhausted tasks transition to BLOCKED

**Contracts**
- CORE.md compressed for MAM context: −69 lines (799 → 730) (ADR-0027)
- Subagent READ-WRITE variant — explicit opt-in for state-modifying delegation
- Doc Impact Declaration now requires search before declaring "none"
- Conventional Commits standard added to Git Protocol
- Post-compaction re-read instruction for context recovery

**Skills**
- **black-box-red-testing** — hypothesis-driven bug hunting in configurable rounds; only red tests survive
- **white-box-red-testing** — target-driven adversarial testing from commits, files, or coverage gaps
- **have-you-considered** — mentor-posture skill that surfaces alternatives, gated by demonstrable benefit
- **code-spec-backfill** — backfills function-level contracts (docstrings, type annotations); SHA-based incremental state
- **clean-code** updated — commit-SHA input mode; coverage-gated transformations in Liza mode
- **code-review** updated — default scope narrowed to staged files, diff-first principle
- **spec-backfill** restructured — reduced cognitive load, added incremental state and report format

---

## Fixes

| Fix | Impact |
|-----|--------|
| Submit-review spin loop — clear CurrentTask after submit | Prevents claim spin loop |
| Submit-review rebase — don't abort so conflict instructions remain | Actionable merge conflict messages |
| Submit-for-review SHA contract aligned between MCP and ops | Consistent commit validation |
| Claim-task worktree recovery on rejected reassignment | Clean recovery path |
| Worktree/branch cleanup on ReleaseClaim | Prevents stale worktree accumulation |
| YAML round-trip preservation via inline map | Unknown fields survive Modify cycles |
| MCP parse-error write failures made terminal | No silent continuation on protocol errors |
| Signal cleanup restored (lost in bash-to-Go rewrite) | Clean process shutdown |
| Lease-expiry warnings use injected warnWriter | Proper warning output |
| Delete-task defers git cleanup until commit | No orphaned artifacts on failure |
| Init stores spec_ref as absolute path | Correct path resolution |
| Clear stale review leases before reviewer wait | Unblocks reviewer scheduling |
| wt-merge branch cleanup made idempotent | Safe re-runs |
| Template panic replaced with error return | Graceful error handling |
| npm EACCES fix for jscpd pre-commit in sandboxed agents | Unblocks agent pre-commit |
| Post-write add-task validation enforced | Catches invalid tasks at creation |
| Prompt disambiguation: ~/.liza/ vs project .liza/ | Correct path references in bootstrap |

---

## Refactoring

- **Supervisor decomposition** — god file (1,426 LOC / 31 functions) split into 10 cohesive files: supervisor.go, registration.go, waitforwork.go, claiming.go, prompt.go, systemctl.go, heartbeat.go, logging.go, output.go, workdetection.go
- **Role-specific prompt templates** (ADR-0026) — deleted shared_reference.tmpl, distributed to role-specific templates; per-role prompt size: coder 348→90, reviewer 399→109, planner 277→65 (−74% per role)
- **Task state machine** — explicit transition graph as `taskTransitions` map with `CanTransition()` / `Transition()` methods; all 14 transition sites migrated from direct assignment
- **Task type → role workflow mapping** — claimability derived from (type, role) registry instead of hardcoded status checks (ADR-0020)
- **Unified role constants** (ADR-0024) — `internal/roles` package resolves `code-reviewer` vs `code_reviewer` divergence; all production code uses constants
- **State validation extraction** (ADR-0025) — validation logic extracted from blackboard into dedicated package
- **FindTask/FindTaskIndex consolidation** — 35 inline lookup loops across 17 files replaced with `State.FindTask()` / `State.FindTaskIndex()`
- **NotFoundError consistency** — 25+ ad-hoc `fmt.Errorf` calls replaced with structured `NotFoundError{Entity, ID, Field}`
- **File locking extraction** — extracted from db/ and log/ into `internal/filelock`
- **Named config defaults** — scattered magic numbers (1800/30/60) replaced with named constants
- **Append-only log writes** — tail timestamp reads for sub-linear stall detection as logs grow

---

## Documentation

- ADRs 0020–0030 backfilled (see below)
- Multiple architecture review passes (passes 5–7 plus adversarial)
- Cross-model architectural findings documented
- Comprehensive build story backfill across all functional areas (specs 1.1–1.6)
- `spec-mapping.yaml` — 1,694-line mapping reconciling specs to implementation
- Troubleshooting: canonicalized worktree recovery examples
- Lessons added for agents (CLI/MCP surface sync, edit-tool-destroys-symlinks) and humans
- Architectural issues categorized into "Accepted v1 Limitations" section

---

## ADRs Added

| ADR | Title |
|-----|-------|
| 0020 | Explicit Task Workflow Contract (Type-Aware Claiming and Limit Escalation) |
| 0021 | Ops Service Layer for State Mutations |
| 0022 | Concurrency Hardening: Singleton Blackboard and CAS Merges |
| 0023 | Crash Recovery Commands |
| 0024 | Unified Role Constants Package |
| 0025 | State Validation Extraction |
| 0026 | Role-Specific Prompt Templates |
| 0027 | Contract Compression for MAM Context |
| 0028 | Multi-Sprint Support with Archive Safety |
| 0029 | Agent Log Analysis Tools |
| 0030 | Code-Enforced Agent Guardrails |

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

**Upgrade:**
```bash
liza setup        # Re-install updated contracts to ~/.liza/
```

---

## Known Limitations

- Terminal-first; no IDE integration or web UI
- Prompt changes require binary rebuild (no hot-reload)
- `liza setup` required before `liza init` — extra step for new users

---

## Resolved from v0.2.0

- ~~Sequential sprints only~~ — Multi-sprint support with automatic advancement (ADR-0028)
- ~~Automated circuit breaker~~ — Watch escalation automated with hardened error handling
- ~~One agent instance per role~~ — Parallel coders and reviewers supported, with CAS-safe merges

---

## What's Next

- **Specification phase** pipeline (Requirement Planner → Spec Writer → Spec Reviewer) before coding sprints
- **Context handoff** as blackboard event — structured positive/negative findings on every task completion
- **Sprint Analyzer** role — analyze all agent logs at end of sprint, use lesson-capture to capitalize on patterns and feed back into the planner's context, providing decision material to the user at sprint boundaries
- **Deterministic pre/post hooks** at role transitions — mechanical checks before spawning agents and before their handoff
- **Planner-routed model selection** — assign tasks to models based on estimated complexity
