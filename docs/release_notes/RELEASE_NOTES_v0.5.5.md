# Liza v0.5.5

Codex MCP compatibility, agent lifecycle hardening, and multi-agent
robustness fixes from production sprint runs.

15 commits since v0.5.4 (2026-04-02).

---

## Highlights

**MCP tool annotations for Codex exec mode** — Codex 0.117.0+ introduced
MCP elicitation prompts that auto-cancel in non-interactive mode
([openai/codex#16685](https://github.com/openai/codex/issues/16685)).
All liza-mcp tools now declare `destructiveHint=false` via MCP annotations,
replacing the `--dangerously-bypass-approvals-and-sandbox` workaround with
`--full-auto` to keep the OS-enforced sandbox active.

---

## Features

| Feature | Description |
|---------|-------------|
| Allow supersede-task without replacement IDs | Orchestrator can supersede tasks completed externally without requiring replacement task IDs |

---

## Fixes

| Fix | Impact |
|-----|--------|
| MCP tool annotations for Codex compatibility | All 24 liza-mcp tools annotated with `destructiveHint=false`; `--full-auto` replaces `--dangerously-bypass-approvals-and-sandbox`; CI guardrail ensures new tools have annotations |
| Force C locale for all git subprocesses (#14) | Git output parsing no longer breaks on non-English locales |
| SIGTERM agent process on delete | Agent deletion now sends SIGTERM to the process (both CLI/TUI and MCP paths), preventing orphaned agent processes |
| Chunk await blocking to fit Codex transport timeout | Long-polling MCP calls (await_verdict, await_resubmission) chunked to avoid Codex transport timeouts |
| Handle race conditions in await ops | Concurrent await operations on the same task no longer panic or return stale state |
| Handle already-escalated tasks in await_resubmission | Reviewers waiting for resubmission on tasks that were already re-escalated now get correct state |
| Clear review ownership on new attempt transition | Review claims properly reset when a task enters a new review cycle |
| Skip review-cycle warning for terminal tasks | Watcher no longer emits spurious review-cycle warnings for completed/superseded tasks |
| Surface quota exhaustion as alert | Provider quota exhaustion surfaces as a structured alert instead of silent failure |
| Clear provider quota signals on resume | TUI resume clears stale quota-exhaustion signals from previous run |
| Surface ops.AddTask warnings in TUI | Validation warnings from add-task now visible in TUI result messages |

---

## Refactoring

- Extract shared process spawning and alert line parsing into reusable helpers

---

## Codex Version Requirement

When using `--cli codex`: requires Codex CLI > 0.118.0 (or 0.116.0).
Versions 0.117.0-0.118.0 have a regression that cancels MCP tool calls
in exec mode regardless of annotations.

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
