# 45 - Declarative Role Definitions in Pipeline YAML

## Context and Problem Statement

ADR-0035 (Declarative Sub-Pipelines) externalized pipeline structure into YAML — state names, transitions, and sub-pipeline composition became configuration rather than code. But roles themselves remained hardcoded: role types (doer/reviewer/orchestrator), display names, and agent-role mappings lived in Go constants, switch statements, and the now-removed `agent-roles` map on the pipeline struct.

Adding Phase 2 roles (ADR-0038) made this painful — each new role required changes across multiple Go files. The declarative philosophy that worked for pipelines should extend to roles: making things declarative is always the plan, hardcoding is an intermediate step toward maintainability and extensibility.

## Considered Options

1. **Add a `roles` section to pipeline YAML** — roles become first-class declarative objects alongside role-pairs and sub-pipelines.

No alternatives were considered. Declarative configuration is the established direction (ADR-0035).

## Decision Outcome

Chose **Option 1**: extend the pipeline YAML with a `roles` section that defines role properties declaratively.

### Architecture

**Pipeline YAML `roles` section:**
```yaml
roles:
  coder:
    type: doer
    display-name: Coder
  code-reviewer:
    type: reviewer
    display-name: Code Reviewer
  orchestrator:
    type: orchestrator
    display-name: Orchestrator
  code-planner:
    type: doer
    display-name: Code Planner
  # ... all 9 roles
```

**Key properties per role:**
- `type`: doer | reviewer | orchestrator — the adversarial category (ADR-0042)
- `display-name`: human-readable label for CLI output

**Resolver methods added:**
- `RoleType(role)` — returns doer/reviewer/orchestrator
- `RoleDisplayName(role)` — returns display name
- `IsDoerRole(role)`, `IsReviewerRole(role)`, `IsOrchestratorRole(role)` — classification predicates
- Role classification replaces hardcoded role lists throughout the codebase

**Agent provider metadata:**
- `Agent.Provider` persisted at registration (from `--cli` flag value, e.g. `"claude"`, `"codex"`)
- Threaded through agent query/inspect for observability
- Foundation for provider-diversity constraints (ADR-0046)

**`agent-roles` absorption:**
The legacy `AgentRoles` map on the pipeline struct (which mapped role names to display names) was absorbed into the `roles` section. `proceed.go` and `wake.go` now use `Resolver.RoleDisplayName()` instead of map lookups.

### Rationale

Roles are structural metadata — their type determines claiming behavior, their display name drives CLI output, and their properties will grow (timeouts, allowed-operations, context-sections, skills, mandatory-docs are planned). Encoding this in YAML makes the structure visible, reviewable, and extensible without code changes. The same principle that motivated declarative sub-pipelines applies: generic mechanisms tested once beat special-cased code duplicated per role.

### Consequences

**Positive:**
- Adding a new role requires YAML, not Go code changes for classification
- Role type is declared once, not inferred from scattered switch statements
- Provider metadata enables downstream decisions (diversity constraints, model selection)
- `agent-roles` map eliminated — single source of truth for role display names

**Limitations accepted:**
- Only `type` and `display-name` are implemented so far — future properties (timeouts, skills, mandatory-docs) are designed but deferred
- Config validation remains at load time, not compile time (consistent with ADR-0035)

**Extends:** ADR-0035 (Declarative Sub-Pipelines) — same declarative philosophy, applied to roles. **Depends on:** ADR-0038 (Phase 2 Roles), ADR-0042 (Doer/Reviewer Vocabulary).

---
*Reconstructed from commits f0d1f26..ad6ef20 (2026-03-13 to 2026-03-15)*
