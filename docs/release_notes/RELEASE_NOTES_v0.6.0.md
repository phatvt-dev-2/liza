# Liza v0.6.0

Two new sub-pipeline capabilities: architecture and integration.
- many-to-one transitions consolidate N sibling tasks into one architecture task,
- architect and architecture-reviewer roles produce design documents before coding,
- an integration sub-pipeline scans the full branch diff after coding completes
  to catch cross-task issues invisible to task-scoped reviews,
- arch_ref propagates through the pipeline so downstream coders and reviewers
  see the architectural context,
- and an architecture-planning skill guides the architect role's workflow.

51 commits since v0.5.5 (2026-04-03).

---

## Highlights

- **Many-to-one transition engine** — A new transition cardinality allows N
  approved sibling tasks (e.g. user stories) to consolidate into a single
  downstream task. The engine includes deterministic child ID generation,
  cohort detection via shared parent, crash recovery for partial transitions,
  topological sort respecting inter-cohort dependencies, and inherited
  dependency propagation across cohorts. The orchestrator detects
  `MANY_TO_ONE_READY` cohorts and fires transitions automatically.

- **Architecture step** — Two new roles (architect, architecture-reviewer)
  sit between story-writing and code-planning in the pipeline. The architect
  produces a design document covering component structure, interfaces, and
  trade-offs. The architecture-reviewer validates the design before coding
  begins. Both roles have dedicated prompt templates, context sections, and
  pipeline states (`DRAFT_ARCHITECTURE`, `ARCHITECTURE_SUBMITTED`,
  `REVIEWING_ARCHITECTURE`, `ARCHITECTURE_APPROVED`,
  `ARCHITECTURE_REJECTED`).

- **Integration sub-pipeline** — A third automated sub-pipeline runs after
  all coding tasks reach MERGED. An integration-analyst scans `git diff
  goal.base_commit..HEAD` for mechanical integration issues (type
  misalignment, serialization tag inconsistencies, API surface drift,
  misplaced code). Findings become fix tasks via auto-triggered per-subtask
  transitions that flow through the standard coding-pair lifecycle. Clean
  scans terminate at `INTEGRATION_ANALYSIS_CLEAN` without spawning children.

- **arch_ref field** — A new `arch_ref` field on Task and OutputEntry
  propagates the architecture document reference through transitions.
  Downstream code-planners, coders, and reviewers receive the arch_ref
  in their prompt context, ensuring design decisions inform implementation.

---

## Features

**Many-to-One Transitions**
- `many-to-one` cardinality accepted in pipeline config validation
- Core transition engine with cohort detection, deterministic child IDs, and `transitions_executed` marking on all cohort members
- Crash recovery: idempotent re-execution patches existing children and creates missing ones
- Inherited dependency propagation across cohorts (with intra-cohort self-dep prevention)
- `CountReadyManyToOneCohorts` helper for orchestrator wake detection
- `MANY_TO_ONE_READY` trigger in `liza status`
- Orchestrator end-to-end path: detect ready cohorts, fire transition, advance sprint

**Architecture Step**
- Architect and architecture-reviewer roles registered with timeouts, context-sections, skills, and allowed-operations
- Architecture-pair with `DRAFT_ARCHITECTURE` through `ARCHITECTURE_APPROVED` state graph
- `us-to-coding` pipeline transition wired as many-to-one from us-writing-pair to architecture-pair
- `arch-to-code-planning` transition from architecture-pair to code-planning-pair

**arch_ref Propagation**
- `ArchRef` field on `Task` and `OutputEntry` model structs
- Validation: arch_ref required on architecture-pair output entries
- `set-task-output` MCP tool accepts and persists arch_ref
- `proceed.go` propagates arch_ref through per-subtask and one-to-one transitions
- Downstream prompt templates render arch_ref for code-planner, coder, and reviewer roles

**Prompt Templates**
- `parent-tasks-context` template block renders upstream task details for architect context
- `ArchRef` and `ParentTaskContexts` added to `RoleContextData`
- Task type resolved dynamically from pipeline config (replaces hardcoded "coding")
- Architect and architecture-reviewer branches in all template files

**ParentTasks Multi-Parent**
- `ParentTasks []string` field replaces single `ParentTask` (deprecated, retained for backward compatibility)
- `CohortParentID()` encapsulates the parents[0] convention for cohort detection
- `EffectiveParentTasks()` provides migration bridge

**Integration Sub-Pipeline**
- Integration-analyst and integration-reviewer roles with dedicated state graph (`DRAFT_INTEGRATION_ANALYSIS` through `INTEGRATION_ANALYSIS_APPROVED`)
- `INTEGRATION_ANALYSIS_CLEAN` terminal state when no findings (empty output[])
- `integration-to-fix` auto-transition with per-subtask cardinality: each finding becomes a coding-pair fix task
- `goal.BaseCommit` snapshot records integration branch HEAD when first coding task is created; used as diff base
- `CodingComplete` wake trigger spawns integration-analyst after all coding tasks merge
- Merge-before-fan-out: integration tasks follow approved → merged → fan-out lifecycle, preventing orphaned worktrees

**Architecture-Planning Skill**
- Skill guides architect agents through design document creation
- Wired into pipeline config context-sections for architect role

**RTK Guard Hook**
- `rtk-guard` PreToolUse hook prevents agents from bypassing RTK without prior failure evidence

---

## Fixes

| Fix | Impact |
|-----|--------|
| Prevent self-dependency in many-to-one inherited deps | Intra-cohort `depends_on` no longer produces circular self-references on the consolidated child task |
| Provision Claude Code config into worktrees | Agent worktrees receive `.claude/` settings, preventing bare-config failures |
| Merge integration tasks before fan-out | Integration tasks merged before creating downstream children, preventing orphaned worktrees |
| Propagate ParentTasks, ParentTask, and ArchRef during replan | Replan preserves multi-parent relationships and arch_ref on regenerated tasks |
| Instruct integration analyst to run tests before analysis | Integration analysis prompt now requires test execution before reporting |
| Require e2e test and doc update tasks in code plans | Code-planner prompt enforces test and documentation deliverables |

---

## Refactoring

- Extract `CohortParentID()` method to encapsulate parents[0] convention
- Extract `CountReadyManyToOneCohorts` to ops package (was inline in agent)
- Use typed errors and extract `InitProject`/`SetAutoResume` from ops

---

## Documentation

- ADR-0055 (integration sub-pipeline) and ADR-0056 (architecture step)
- Many-to-one transition engine semantics
- Architecture step pipeline schema and docs
- arch_ref field schema documentation
- parent-tasks-context section and architect context docs
- Integration sub-pipeline specs and user docs
- TECH_DEBT.md with ParentTask deprecation entry
- RTK anti-abuse rules strengthened in AGENT_TOOLS contract

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
