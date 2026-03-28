# Liza v0.5.2

Fix Codex provider support broken by upstream approval-policy change,
harden the supervisor loop against runaway restarts and quota exhaustion,
and ship TUI agent-management keybindings.

32 commits since v0.5.1 (2026-03-27).

---

## Highlights

- **Codex Non-Interactive Fix** — Codex CLI began denying custom MCP tool calls under `on-failure` approval policy in exec mode, causing 100% failure rate for Liza MCP calls across 74 reviewer sessions. Fix: override `approval_policy="never"` for Liza-launched autonomous agents.
- **Supervisor Hardening** — Crash-restart tracker, spinning safety net, and quota-exhaustion detection prevent the 9-hour claim/release loop observed in production. Quota signal files coordinate graceful shutdown across co-located supervisors.
- **Auto-Resume ("Yolo Mode")** — `--auto-resume` flag and `[y]` TUI keybinding for fully unattended sprint progression through CHECKPOINT and COMPLETED states.
- **Legacy Attempt Migration** — The deprecated `attempted:` list field is now migrated on disk and normalized on read, completing the transition to the structured `attempt:` integer model.

---

## Features

- Crash-restart and spinning safety nets for supervisor loop — blocks tasks after configurable thresholds (5 crash-restarts, 10 same-task re-executions)
- Provider quota exhaustion detection with signal-file coordination and `liza resume` to clear
- Auto-resume option (`--auto-resume`, `[y]` TUI toggle) for hands-off sprint progression
- `[S]` spawn-with-CLI keybinding — two-phase role → CLI provider selection flow
- `[t]` terminate keybinding — force-delete agents with tab-completion and confirmation
- Active tasks partitioned before terminal tasks in TUI task panel
- Legacy `attempted:` field migration on disk (`liza migrate`) and read-normalization

---

## Fixes

| Fix | Impact |
|-----|--------|
| Set `approval_policy="never"` for Codex non-interactive agents | **Critical** — unblocks all Codex reviewer sessions |
| Derive stall detection from task history instead of state.yaml mtime | Heartbeat writes no longer mask stalled tasks |
| Surface prior rejection feedback in attempt-2 prompts | Agents see actual reviewer feedback, not just escalation reason |
| Generalize escalation enforcement to all blocked types | Tasks with exhausted review cycles no longer re-claimable indefinitely |
| Preserve rejection feedback in new_attempt history entry | Reviewer feedback survives attempt transition |
| MigrateAttemptedField returns true for all key deletions | Migration persists cleanup-only removals |
| MigrateAttemptedField and EffectiveAttempt capped at 2 | Prevents invalid attempt values from legacy data |
| Re-arm watcher subscription on errMsg | Reactive updates continue after transient fsnotify errors |
| Surface log.yaml parse errors instead of swallowing | Persistent corruption visible in activity feed |
| Surface WriteAlert errors instead of discarding | Alerts.log write failures now visible |
| Use build tags for `SysProcAttr` in TUI spawn | **Release** — goreleaser Windows build failed, blocking all v0.5.2 binary assets |
| Relabel TIME_ON_TASK to LAST_HEARTBEAT in agent panel | Column semantics match displayed data |
| Eliminate wasted vertical space and widen task ID column | Content-sized panels, compound IDs fit |
| Auto-fallback to headless mode when stdin is not a TTY | CI/cron/pipe contexts no longer crash |
| Forward cobra flag defaults through wizard init path | Wizard and CLI paths produce identical config |
| Reject workspace flags silently swallowed by wizard | `--branch`, `--config`, etc. no longer dropped |

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
