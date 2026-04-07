# Liza v0.6.1

Multi-provider MCP tool naming fix and TUI layout improvements.

Agent prompts now use provider-correct MCP tool names everywhere,
fixing tool discovery failures on Codex and eliminating mixed
prefixed/bare naming that caused agents to fail in repos without
rich Liza codebase context. The TUI no longer overflows or crushes
panels at narrow terminal widths.

5 commits since v0.6.0 (2026-04-07).

---

## Fixes

| Fix | Impact |
|-----|--------|
| Make MCP tool naming provider-aware | Tool names in agent prompts were hardcoded to Claude Code's `mcp__liza__` prefix, causing failures on Codex (which uses bare names). Worse, prompts mixed prefixed names (in ToolSearch hints) with bare names (in tool descriptions and instructions) — agents in non-Liza repos couldn't infer the correct names when ToolSearch failed. All tool references now use `{{.ToolPrefix}}` computed from `MCPToolPrefix(cliName)`, threaded through `BasePromptConfig`, `RoleContextData`, and wake template data. |
| Fix TUI layout overflow and agent panel crushing | `Panel Width(m.width)` produced lines 2 chars wider than the terminal, causing bubbletea to drop the header bar and right border. In tight mode, agents were allocated only 1/4 of space while terminal tasks consumed the rest. Fixed width calculation and rewrote layout to shed terminal tasks first and split space evenly. |

---

## Documentation

- README: add contract thoughtfulness example and frontier MAS label
- Usage guide: fix stale phase count (9 not 8), typo, and missing CLI commands (`liza delete`, `liza set-discovery-disposition`)

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
- **Sprint Analyzer** role — analyze agent logs via lesson-capture
