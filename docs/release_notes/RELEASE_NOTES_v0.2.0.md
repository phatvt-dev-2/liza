# Liza v0.2.0

The bash-script orchestrator is replaced by a single Go binary. Contracts gain tiered context degradation and a third mode (Subagent). The task lifecycle is hardened with role-neutral states. Deployment splits into global setup + project init.

77 commits since v0.1.1 (2026-01-29).

---

## Breaking Changes

- **Bash scripts removed**: All `scripts/liza-*.sh` entrypoints are gone. Replace with `liza <subcommand>` (e.g., `liza-agent.sh` → `liza agent`, `liza-watch.sh` → `liza agent planner --watch`). Any existing automation calling the old scripts must be updated.
- **Task states renamed**: UNCLAIMED → READY, CLAIMED → IMPLEMENTING. New REVIEWING state added. Existing blackboard files need state values updated.
- **`liza init` slimmed**: no longer copies contracts/skills into projects. Run `liza setup` first to install contracts globally to `~/.liza/`.
- **`agent-runtime-reference.md` removed**: operational context is now inlined into agent prompts at build time. Agents no longer need to read an external file.

---

## Highlights

- **Go CLI replaces bash scripts** (ADR-0012) — Two binaries (`liza` CLI + `liza-mcp` server) replace 18+ bash scripts. Type-safe blackboard operations, built-in MCP server, internal locking, comprehensive test suite.
- **Two-step deployment** (ADR-0018) — `liza setup` installs contracts to `~/.liza/`, `liza init` scaffolds project with symlinks. Eliminates per-project duplication, fixes cross-reference paths.
- **Task lifecycle hardened** (ADR-0019) — Role-neutral states (READY, IMPLEMENTING, REVIEWING) support planned multi-role expansion. REVIEWING prevents concurrent reviewers. SPRINT_COMPLETE trigger closes the planner loop.

---

## Features

**Go CLI**
- Two binaries: `liza` (CLI) and `liza-mcp` (MCP server), replacing 18+ bash scripts
- Built-in MCP server (`liza-mcp`) — agents call liza tools directly
- Supervisor with heartbeat, lease management, and priority-based work detection
- Interactive mode (`-i`) launches CLI for prompt input
- `--alias` flag to pass agent fabrications
- Handoff command for context exhaustion mid-task
- SPRINT_COMPLETE wake trigger so planner checkpoints finished sprints

**Contracts**
- Coach and Challenger collaboration modes (ADR-0013)
- Tiered context degradation: Full → Working Set → Kernel (ADR-0014)
- Subagent Mode as first-class contract mode (ADR-0015)
- Loop detection self-abort and review exhaustion protocols (ADR-0010)
- Script-enforced agent status transitions (ADR-0011)
- Unrelated working tree changes protection in Git Protocol
- Fail-fast guidance for missing environment tools in coder prompt

**Prompt Architecture**
- Embedded template architecture — `.tmpl` files compiled into binary via `embed.FS` (ADR-0016)
- Shared reference template inlined into all agent prompts at build time
- Runtime reference eliminated — single source of truth per concern

**Deployment**
- `liza setup` for global contract/skill installation to `~/.liza/` (ADR-0018)
- User-customizable file protection with `.bak` backup on overwrite
- Release infrastructure — GoReleaser, CI pipeline, curl-pipe-sh installer (ADR-0017)

**Skills**
- Clean-code skill made language-agnostic with plugin architecture (Go + Python)
- Spec-backfill skill for reconciling spec/code drift
- Lesson-capture skill for project-specific operational knowledge
- Kimi (Moonshot) provider support

---

## Fixes

| Fix | Impact |
|-----|--------|
| Force agents to use MCP or CLI to update blackboard | Prevents direct file edits bypassing validation |
| Release agent state on task lifecycle transitions | 4 call sites consolidated into `ReleaseAgent()` |
| Normalize commit SHAs to prevent short/full comparison | Blackboard consistency |
| Force worktree removal after merge | Prevents stale worktree accumulation |
| Preserve skill frontmatter fields during `liza setup` | Downstream consumers retain metadata |
| Address 8 cross-document inconsistencies in contracts | Contract coherence |
| Protect user-customizable files with backup on overwrite | Prevents accidental loss of customizations |
| Align subagent mode with skill on vague goal handling | Consistent behavior across contract and skill |

---

## Refactoring

- 12-commit clean code pass across all Go packages (post-rewrite)
- Go code duplication reduced below jscpd 5% threshold
- Prompt builder reduced from ~700 lines of string formatting to ~100 lines of template orchestration
- MULTI_AGENT_MODE.md trimmed to behavioral-only (operational tables moved to templates)
- Consolidated redundant philosophy content to canonical sources
- Centralized agent runtime reference documentation (then eliminated it)

---

## Documentation

- Supervision model — supervisor vs MCP tool responsibility matrix
- Operational docs imported from liza-go: configuration, performance, recipes, testing
- Updated REPOSITORY.md for Go rewrite
- ADRs 0010–0019 backfilled

---

## ADRs Added

| ADR | Title |
|-----|-------|
| 0010 | Loop Detection Self-Abort |
| 0011 | Script-Enforced Agent Status |
| 0012 | Go CLI Replaces Bash Scripts |
| 0013 | Coach and Challenger Collaboration Modes |
| 0014 | Tiered Context Degradation |
| 0015 | Subagent Mode First-Class |
| 0016 | Embedded Template Architecture |
| 0017 | Release Infrastructure |
| 0018 | Two-Step Deployment |
| 0019 | Task Lifecycle State Machine Evolution |

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

**First-time setup:**
```bash
liza setup        # Install contracts to ~/.liza/
liza init          # Initialize project
```

---

## Known Limitations

- Sequential sprints only — after a sprint completes, the blackboard must be removed (`rm -rf .liza`) and re-initialized before the next sprint. The planner does not auto-detect changes to vision.md.
- One agent instance per role (no parallel coders)
- Terminal-first; no IDE integration or web UI
- Prompt changes require binary rebuild (no hot-reload)
- `liza setup` required before `liza init` — extra step for new users

---

## What's Next

- More agent pairs (Spec Writer / Spec Reviewer, Architect / Architecture Reviewer)
- Parallel coder support
- Automated circuit breaker — `liza analyze` exists for manual pattern detection; next step is continuous monitoring via `liza watch`
- Multi-sprint continuity — planner detects scope changes and plans incrementally without blackboard reset
