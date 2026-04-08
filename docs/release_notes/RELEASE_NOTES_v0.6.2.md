# Liza v0.6.2

Brownfield onboarding, CLI resilience, and MCP error quality.

Agents now fall back to CLI commands when MCP tools are unavailable,
`liza init` handles brownfield projects with a `CLAUDE.local.md` option,
and MCP error messages expose actionable detail instead of generic
"precondition not met" failures.

9 commits since v0.6.1 (2026-04-07).

---

## Highlights

**CLI fallbacks for all MCP-only tools** — When the MCP server can't
start (e.g. OOM on memory-constrained machines), agents no longer burn
~20 turns guessing flag syntax. The base prompt now includes concrete
CLI examples for `liza get`, `liza status`, `liza add-task`, and all
other MCP-only operations, so agents degrade gracefully to shell commands.

**CLAUDE.local.md for brownfield projects** — When `CLAUDE.md` already
exists at repo root, `liza init` now offers a "local" option that creates
`CLAUDE.local.md` (Claude Code's gitignored project instructions) as a
symlink to the contract instead of renaming or falling back to global config.

---

## Features

| Feature | Description |
|---------|-------------|
| CLAUDE.local.md init option | Brownfield projects can keep their existing `CLAUDE.md` while enabling Liza's contract via `CLAUDE.local.md` |
| Claude quota exhaustion detection | Supervisor detects `"You're out of extra usage"` from Claude Code and triggers graceful shutdown |
| CLI fallbacks for all MCP tools | Agent prompts include CLI syntax for every MCP-only operation, eliminating flag-guessing spirals |

---

## Fixes

| Fix | Impact |
|-----|--------|
| Expose actionable MCP errors (#16) | `InputShapeError` surfaces the real parsing failure instead of generic "precondition not met"; all 14 array properties now have `Items` schemas |
| Add MCP tool discovery guidance | Base prompt guides agents to discover available tools when MCP prefixes vary |
| CLI fallback syntax for get, add-task, status | Orchestrator's most-used operations no longer cause multi-turn guessing |
| Warn when Node.js deps not installed | `liza init` checks for `package.json` without `node_modules/` and warns before agents hit missing binaries |
| Log warnings from task claim and worktree cleanup | `post_worktree_cmd` failures and worktree cleanup issues now visible in supervisor logs |

---

## Documentation

- Add Key Concepts section (Goal, Checkpoints, Worktrees) to usage guide
- Highlight committing spec file before `liza init` (worktrees need it)
- Add interactive diagnosis guidance (use a regular coding agent to diagnose)
- Update support doc: reference `.liza/agent-prompts/` for crash diagnosis
- Update support doc: allow direct `state.yaml` edits as last resort with guardrails

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
