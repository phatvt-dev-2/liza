# Liza v0.7.0

MCP server removal, CLI-native access control, and per-project agent configuration.

The MCP server — a redundant parallel surface over the ops layer — is
removed entirely. All agent-to-system interaction now goes through the
CLI with `--json` structured output and `--agent-id` role validation.
Per-project Claude environment overrides land via `claude.env`.

26 commits since v0.6.2 (2026-04-13).

---

## Highlights

**MCP server removed** — The MCP server added token overhead, reliability
issues under memory pressure, and a two-surface maintenance burden. The
CLI fallback work in v0.6.2 made the redundancy undeniable. All 16
state-mutating commands now validate agent identity natively (`--agent-id`
positional arg or auto-resolved orchestrator). Structured output via
`--json` replaces MCP tool responses. See ADR-0057.

**`claude.env` for per-project overrides** — An optional `claude.env`
file at the project root injects environment variables into spawned
Claude CLI processes (cf https://code.claude.com/docs/en/env-vars). Secrets from `claude.env` are automatically added to the
output masker.

**Configurable default CLI** — The default CLI for agent spawning was
hardcoded to `claude`, silently breaking Codex-only setups. A new
`config.default_cli` field resolves via: explicit `--cli` flag >
`state.yaml` config > `LIZA_DEFAULT_CLI` env > `"claude"`. Set at init
time with `liza init --default-cli codex` or toggle in the TUI (`s` uses
default, `S` picks).

---

## Breaking Changes

- **MCP server removed** — The `liza-mcp` binary and `cmd/liza-mcp/`
  package no longer exist. Agents that relied on MCP tools must use CLI
  commands with `--json` instead. All MCP tool names are mapped to CLI
  equivalents in the agent base prompt.

---

## Features

| Feature | Description |
|---------|-------------|
| MCP server removal + CLI `--json` mode | All agent interaction through CLI; `--json` flag on state-reading commands for structured output |
| CLI-native access control (`--agent-id`) | Role validation on state-mutating commands replaces MCP RBAC middleware |
| `claude.env` support | Per-project environment overrides for Claude CLI agent processes, with secret masking |
| Configurable `default_cli` | Resolution chain: `--cli` > `state.yaml` > env var > `"claude"` constant (#17) |
| `.claudeignore` template | Embedded universal `.claudeignore` (Node, Go, Rust, Python, Java) installed during `liza init`; prompts if file exists |
| Union-merge `additionalDirectories` | Settings merge now unions Liza-required dirs (`~/.liza`, `/tmp`) with user-added dirs instead of overwriting |
| Role-specific tool blocks | Per-role prompt tool sections replace shared doer/reviewer blocks, aligning documented CLI tools with runtime allowed operations |

---

## Fixes

| Fix | Impact |
|-----|--------|
| TOCTOU race in abortTicker | Agent checks for work before aborting, preventing premature shutdown when tasks arrive between ticks |
| Goal completion loops | System stops on goal completion instead of looping empty sprints |
| Watch alerts on terminal tasks | Watch daemon skips terminal-state tasks, reducing noise |
| Claude quota detection tightened | More precise pattern matching for quota exhaustion messages |
| Orchestrator re-wake loop | Self-healing PostExecution calls sprint_checkpoint when agent fails to; spinning detection stops system after N identical state signatures |
| Pipeline-aware sprint carry-forward | `advanceSprint` now uses pipeline-defined terminal states, preventing rapid sprint-advance loops when tasks like INTEGRATION_ANALYSIS_CLEAN were incorrectly carried forward |
| Domain-based decomposition | Architect/planner prompts reframed: decompose by domain boundaries, not implementation steps — prevents unnecessary planning overhead for cohesive scopes |
| Missing TaskTypes for epic/US roles | `TaskTypeForRole` fell back to `TaskTypeCoding` for epic-planner and us-writer, causing TDD enforcement to reject non-code planning tasks |
| Empty output in SetTaskOutput | Removes precondition requiring at least one output entry, allowing agents to set empty output lists |

---

## Documentation

- ADR-0057: MCP server removal and CLI-native access control (supersedes ADR-0039, ADR-0043)
- Architecture health check: walkthroughs for 6 new packages (tui, interactive, render, jsonout, process, gitenv), updated LOC figures (+44% production, +57% test)
- Emphasize `/liza-logs` for friction identification in README and usage guide
- Worktree build prerequisite guidance (lessons)
- `.claudeignore` best practices in README
- Remove deprecated `console.sh`
- Claude settings documentation update

---

## Installation

**Quick install (macOS/Linux):**
```bash
curl -fsSL https://raw.githubusercontent.com/liza-mas/liza/main/install.sh | bash
```

**From source:**
```bash
make install
```

---

## What's Next

- **Sprint Analyzer** role -- analyze agent logs at sprint boundaries via lesson-capture
- **Context handoff as blackboard event** -- structured positive/negative findings on every task completion
- **Deterministic pre/post hooks** at role transitions
