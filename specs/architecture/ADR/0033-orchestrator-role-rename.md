# 33 - Planner to Orchestrator Role Rename

## Context and Problem Statement

The "Planner" role name was a misnomer inherited from Liza's first iteration. The role orchestrates work — task decomposition, monitoring blocked states, rescoping, unblocking — rather than producing plans. This was always intended as a monolithic first iteration to prove the system worked; the separation of concerns between orchestration and planning was planned from the start.

With the introduction of actual planning roles (code-planner, epic-planner), keeping "Planner" for the orchestrating role would create confusion about which agent does what.

## Considered Options

1. **Keep "Planner"** — accept the naming ambiguity.
2. **Rename to "Orchestrator"** — matches actual responsibilities (coordination, not plan creation).
3. **Rename to "Supervisor"** — already used for the process-level supervisor loop.
4. **Rename to "Coordinator" or "Dispatcher"** — the role does more than dispatching; it decomposes, rescopes, and unblocks.

## Decision Outcome

Chose **Option 2**: rename to "Orchestrator" across the entire codebase.

### Architecture

**Scope:** 67 files across 2 commits. Pure semantic rename, zero behavioral changes.

**What changed:**
- Runtime constant: `"planner"` → `"orchestrator"`
- Workflow constant: `"planner"` → `"orchestrator"`
- Go identifiers: `BuildPlannerContext` → `BuildOrchestratorContext`, `PlannerPollInterval` → `OrchestratorPollInterval`, etc.
- Template: `planner_context.tmpl` → `orchestrator_context.tmpl`
- MCP default agent ID: `"planner-1"` → `"orchestrator-1"`
- Contracts, skills, docs: all references updated

**What "planner" now means:**
The term is exclusively used for domain-specific planning roles that produce artifacts:
- `code-planner` / `code-plan-reviewer` — coding plan pairs
- `epic-planner` / `epic-plan-reviewer` — epic decomposition pairs
- `us-writer` / `us-reviewer` — user story writing pairs

### Rationale

Clean separation of concerns: the Orchestrator coordinates work across the system; Planners produce domain-specific plans within their sub-pipeline. This distinction scales — future sub-pipelines (architecture, integration, security) will add more specialized roles without naming collisions.

### Consequences

**Positive:**
- Unambiguous terminology — each role name describes its actual function
- Frees "planner" for the growing family of planning roles
- Zero behavioral risk — purely mechanical rename

**Limitations accepted:**
- Breaking change to existing blackboard files (agent IDs change from `planner-*` to `orchestrator-*`)

---
*Reconstructed from commits 532cb37, 6692dad (2026-03-02)*
