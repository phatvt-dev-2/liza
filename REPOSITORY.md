# Repository Guide

Liza is a disciplined peer-supervised multi-agent coding system. See [README](README.md) for philosophy, approach, and architecture.

This document is a navigation aid: where to find things and why they're organized that way.

## Structure

```
├── contracts/              # Behavioral contracts governing agents
├── specs/                  # Specifications (durable agent context)
├── skills/                 # Domain-specific agent skills
├── scripts/                # Shell scripts (Liza system mechanics)
├── docs/                   # User-facing documentation
├── templates/              # Document templates
│
├── README.md               # Project overview
├── REPOSITORY.md           # This file
├── AGENTS.md               # Symlink → contracts/CORE.md (for agents, e.g. Codex)
├── CLAUDE.md               # Symlink → contracts/CORE.md (Claude Code)
├── GEMINI.md               # Symlink → contracts/CORE.md (Gemini)
├── LICENSE                 # Apache 2.0
├── pyproject.toml          # Python project config & dependencies
├── uv.lock                 # Dependency lock file
├── .pre-commit-config.yaml # Pre-commit hooks
├── mypy.ini                # Type checking config
├── .editorconfig           # Editor formatting
└── .envrc                  # Direnv environment
```

## contracts/

Behavioral contracts that turn agents into trustworthy peers by countering LLM failure modes.

| File | Purpose |
|------|---------|
| `CORE.md` | Universal rules: tier system, state machine, golden rules 1-14, protocols, security |
| `PAIRING_MODE.md` | Human-supervised mode: approval gates, collaboration modes, magic phrases |
| `MULTI_AGENT_MODE.md` | Peer-supervised Liza mode: blackboard protocol, role definitions, circuit breaker |
| `AGENT_TOOLS.md` | Tool sub-contract (MCP preferences, forbidden tools, codebase exploration) |
| `COLLABORATION_CONTINUITY.md` | Cross-session collaboration patterns (the "letter to future self") |
| `CONTRACT_FAILURE_MODE_MAP.md` | Maps every clause to the failure modes it covers |
| `README.md` | Contract navigation guide |

Deployed by symlinking `CORE.md` to `~/.liza/CORE.md`. Mode contracts and supporting files go under `~/.liza/`.

## specs/

Externalized context that survives agent restarts. Agents read specs before acting — specs prevent rediscovering requirements through failing tests.

```
specs/
├── vision.md                           # Philosophy, success criteria, MVP scope
├── README.md                           # Reading order and navigation
├── build/                              # Build specifications (system implementation)
│   └── 0.md                            # Foundation spec (v0 consolidated)
├── functional/                         # Functional domain specifications
│   ├── 0 - Liza.md                     # Liza system overview
│   └── 1.1.md - 1.6.md                 # Domain-specific specs
├── architecture/
│   ├── overview.md                     # Components, data flow, directory structure
│   ├── roles.md                        # Planner, Coder, Code Reviewer responsibilities
│   ├── state-machines.md               # Task states, agent states, exit codes
│   ├── blackboard-schema.md            # YAML state structure, locking, operations
│   └── ADR/                            # Architecture Decision Records
├── protocols/
│   ├── task-lifecycle.md               # Claim → iterate → review → merge
│   ├── sprint-governance.md            # Checkpoints, retrospectives, spec evolution
│   ├── circuit-breaker.md              # Systemic failure detection, severity levels
│   ├── worktree-management.md          # Isolated workspaces, merge protocol
│   └── agent-initialization.md         # Agent bootstrap protocol
└── implementation/
    ├── phases.md                       # Implementation roadmap
    ├── tooling.md                      # Scripts, agent-blackboard interface
    ├── validation-checklist.md         # v1 completion criteria
    └── future.md                       # v1.1+ roadmap
```

## lessons/

Project-specific operational lessons captured via the `lesson-capture` skill. Prevents recurring mistakes by encoding gotchas, patterns, and hard-won insights.

```
lessons/
├── agents/              # Lessons for AI agents (read during session init)
│   └── README.md        # Index: trigger + title for each lesson
└── humans/              # Lessons for human developers
    └── README.md        # Index: trigger + title for each lesson
```

## skills/

Specialized protocols agents load conditionally. Each contains a single `SKILL.md`.

| Skill | Trigger |
|-------|---------|
| `debugging/` | Before any debugging (mandatory) |
| `testing/` | When writing or analyzing tests (mandatory) |
| `code-review/` | When reviewing PRs or pending changes |
| `software-architecture-review/` | Implementation planning, structural concerns |
| `clean-code/` | Pre-commit refactoring (Python-focused) |
| `spec-review/` | Specification validation |
| `spec-backfill/` | Extracting specifications from existing code |
| `generic-subagent/` | Delegating read-only work to subagents |
| `systemic-thinking/` | Systemic coherence and risk analysis |
| `adversarial-testing/` | Security and edge-case testing |
| `adr-backfill/` | Extracting ADRs from git history |
| `feynman/` | Explaining complex ideas simply |
| `lesson-capture/` | Capturing project-specific operational lessons |

Skills execute within contract constraints — contract gates are non-negotiable, skill steps operate within them.

## scripts/

Shell scripts implementing Liza system mechanics. Agents invoke these; the scripts don't invoke agents.

**Initialization & validation:**
- `liza-init.sh` — Initialize `.liza/` directory with blackboard
- `liza-validate.sh` — Validate blackboard state against schema
- `liza-lock.sh` — Atomic file operations with flock

**Agent supervision:**
- `liza-agent.sh` — Agent supervisor (main entry point)
- `liza-checkpoint.sh` — Halt + generate summary
- `liza-watch.sh` — Monitor blackboard, alert on anomalies

**Task management:**
- `liza-add-task.sh` — Add new task to backlog
- `liza-claim-task.sh` — Atomically claim a task for a coder
- `liza-analyze.sh` — Analysis helper
- `liza-prompt-builders.sh` — Construct prompts from state

**Review & merge:**
- `liza-submit-for-review.sh` — Atomic review submission
- `liza-submit-verdict.sh` — Atomic review verdict
- `release-claim.sh` — Release claim on task or review
- `clear-stale-review-claims.sh` — Clean up abandoned reviews

**Shared utilities:**
- `liza-common.sh` — Common functions sourced by other scripts

**Worktree management:**
- `wt-create.sh` — Create isolated per-task worktree
- `wt-merge.sh` — Merge after reviewer approval
- `wt-delete.sh` — Clean up worktree

**Metrics:**
- `update-sprint-metrics.sh` — Sprint statistics

## docs/

User-facing documentation.

| File | Purpose |
|------|---------|
| `USAGE.md` | Quick start guide |
| `DEMO.md` | Full end-to-end walkthrough |
| `TROUBLESHOOTING.md` | Common issues and fixes |
| `release_notes/` | Version changelogs |
| `demo-benchmark/` | Multi-agent demo traces and comparisons |
| `for-agent-eyes/` | Agent-specific runtime references |
| `_archive/` | Archived documentation |

Contract activation guide: `contracts/contract-activation.md`

## templates/

Document templates for bootstrapping new artifacts.

| File | Purpose |
|------|---------|
| `vision-template.md` | Goal-level vision document (produces `specs/vision.md`) |
| `README.md` | Template usage guide and triggers |

ADR template lives at `specs/architecture/ADR/TEMPLATE.md`.

## Reading Order

For newcomers:

1. `README.md` — What Liza is and why
2. `specs/build/0 - Vision.md` — Design philosophy and success criteria
3. `specs/architecture/overview.md` — System components and data flow
4. `contracts/CORE.md` — The behavioral contract
5. `docs/USAGE.md` — How to run it
