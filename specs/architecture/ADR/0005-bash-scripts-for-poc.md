# 5 - Bash Scripts for POC Orchestration

> **Status: SUPERSEDED by [ADR-0012](0012-go-cli-replaces-bash-scripts.md)** — Bash scripts replaced by Go CLI binary (`liza`).

## Context and Problem Statement

The multi-agent system needs orchestration: spawning agents, managing worktrees, coordinating through the blackboard, handling agent lifecycle. How should this be implemented?

## Considered Options

1. **Python orchestrator** — Type-safe, testable, familiar to most developers
2. **Bash scripts** — Direct, transparent, minimal dependencies
3. **Existing framework** — Prefect, Airflow, or similar workflow orchestrators
4. **Custom agent runtime** — Purpose-built coordinator

## Decision Outcome

Chose **Option 2**: Bash scripts for the proof of concept.

### Rationale

**Quick iteration for POC.** Bash is direct — you can read exactly what happens. No abstraction layers, no framework conventions to learn. When debugging agent coordination, `cat` and `grep` are your friends.

**Minimal dependencies.** The only requirements are bash, yq (for YAML manipulation), and git. No Python environment to configure, no packages to install.

**Transparency.** Every script is readable. `liza-agent.sh` is 774 lines, but it's linear — you can trace exactly how an agent gets spawned, claims work, and hands off.

**To be reconsidered.** This is explicitly a POC decision. As Liza matures, the orchestration layer may need:
- Better error handling and recovery
- Type safety for blackboard operations
- Testability for coordination logic
- Performance at scale

### Architecture

```
scripts/
├── liza-agent.sh          # Agent supervisor (spawns Claude, restarts on abort)
├── liza-watch.sh          # Watcher for blackboard changes
├── liza-lock.sh           # Atomic blackboard operations
├── liza-claim-task.sh     # Claim task for agent
├── liza-init.sh           # Initialize .liza/ directory
├── liza-validate.sh       # Validate state.yaml
├── wt-create.sh           # Create worktree for task
├── wt-merge.sh            # Merge completed task
└── wt-delete.sh           # Clean up worktree
```

**Key patterns:**
- Supervisor loop: `liza-agent.sh` wraps Claude invocations, handles restarts
- File locking: `liza-lock.sh` provides atomic read-modify-write
- Polling: Agents poll blackboard state rather than receiving notifications

### Consequences

**Positive:**
- Working POC in days, not weeks
- Easy to modify and experiment
- No runtime dependencies beyond bash/yq/git
- Full visibility into orchestration logic

**Limitations accepted:**
- Limited error handling sophistication
- No type checking on blackboard operations
- Testing is manual rather than automated
- May not scale to many concurrent agents

### Reconsideration Triggers

This decision should be revisited when **testability and maintainability** become priorities — not shell scripts' strengths.

An orchestrator framework may be overkill at Liza's targeted scale, where the human stays engaged. Most likely evolution: **Python** — hence the early `pyproject.toml` setup (commit 8b04128) establishing the project as Python-based despite bash orchestration.

---
*Reconstructed from commit 0875f71 (2026-01-20)*
