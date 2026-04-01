# Liza v0.5.4

Compatibility fix for Claude Code CLI hook protocol change, dependency
resolution bugfix, and documentation improvements.

4 commits since v0.5.3 (2026-03-31).

---

## Fixes

| Fix | Impact |
|-----|--------|
| Redirect enforce-init block message to stderr | Claude Code CLI now requires exit 2 hook messages on stderr; the hook was writing to stdout, causing "No stderr output" errors that blocked all non-Read tool calls before init gate cleared |
| Treat SUPERSEDED as satisfied dependency (#5, #10) | Tasks with SUPERSEDED dependencies were stuck in DRAFT_CODE because dependency checks only accepted MERGED; fixed in all four check locations (validate_deps, task, claim_task, diagnostics) |

---

## Documentation

- CONTRIBUTING.md added with setup, development workflow, and contribution guidelines
- README: ANTHROPIC_API_KEY warning for Claude subscription users

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
