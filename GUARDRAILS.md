# Project Guardrails

Project-specific constraints for Liza agents.
Uses the tier system from the core contract (CORE.md).

## Tier 0 (Inviolable)
<!-- Constraints that must NEVER be violated. Triggers mandatory halt (RESET). -->

## Tier 1 (Hard Constraints)
<!-- Suspended only with explicit waiver. -->

### G1.1: No Liza-specific hardcoding

Liza is a **stack-agnostic** multi-agent orchestrator. Projects using Liza may be written in any language or framework — Python, TypeScript, Rust, Java, etc. Liza itself happens to be written in Go, but that is irrelevant to its users.

**Never** hardcode Liza-specific tooling, paths, commands, or assumptions into Liza's runtime behavior. Examples of violations:

- Hardcoding `make sync-embedded` or any Liza build command into ops/commands
- Assuming a `Makefile`, `go.mod`, or any specific build system exists in the target project
- Referencing Liza-internal paths (e.g. `internal/embedded/`) from runtime code that executes in user worktrees
- Embedding Go-specific test or lint commands as defaults

**Instead:** Use configuration fields (stored in `state.yaml` via `Config`) that users set during `liza init` or can modify later. If a behavior needs to vary per project, it must be configurable — not assumed.

**Test:** Before adding any command, path, or tool reference that touches the user's project, ask: "Would this work for a Python project with no Makefile?" If not, it must be behind a config field.

## Tier 2 (Strong Defaults)

### G2.1: Lessons - Agents

Operational lessons from project experience. Read when a trigger matches.

| Trigger | File                                                                            |
|---------|---------------------------------------------------------------------------------|
| Editing files under `~/.liza/` or symlink paths | [edit-tool-destroys-symlinks.md](lessons/agents/edit-tool-destroys-symlinks.md) |
| Modifying CLI commands, flags, validation, or vocabulary that MCP handlers also expose | [cli-mcp-surface-sync.md](lessons/agents/cli-mcp-surface-sync.md)                              |
| Modifying `internal/embedded/claude-settings.json`, `internal/embedded/mcp.json`, `internal/embedded/hooks/`, or any file with master/derived copies | [settings-master-not-derived.md](lessons/agents/settings-master-not-derived.md)                |
| Reading, editing, or creating files in a worktree | [worktree-file-path-consistency.md](lessons/agents/worktree-file-path-consistency.md) |

## Tier 3 (Preferences)

### G3.1: ADR awareness for architectural changes

When planning a change with architectural impact, read `specs/architecture/ADR/README.md` to understand prior decisions that may constrain or inform the design.

---

Secret word: On-rails
