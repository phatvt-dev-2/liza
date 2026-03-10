# 35 - Declarative Sub-Pipelines

## Context and Problem Statement

Liza's state machine was hardcoded for a single coder/reviewer pair. Adding the spec-writing sub-pipeline would have required duplicating the entire state transition logic with different state names — and more sub-pipelines were already planned: architecture review (between spec and coding), integration sub-pipelines (architecture-review, systemic-thinking, security). Hardcoding each would create unmanageable code.

The system needed generic mechanisms, tested once, that could support arbitrary multi-phase workflows through configuration rather than code changes.

## Considered Options

1. **Extend the hardcoded state machine** — add more literal states for each new phase. Would create unmanageable code very quickly; every new sub-pipeline would require changes across ~15 call sites.
2. **Declarative YAML configuration** — externalize the pipeline structure into a config file; runtime resolves states and transitions through a generic resolver.

## Decision Outcome

Chose **Option 2**: introduce `internal/pipeline/` with a declarative YAML config model.

### Architecture

**Package structure:**
- `internal/pipeline/config.go` — YAML parsing, validation, struct definitions
- `internal/pipeline/resolver.go` — runtime query interface against loaded config
- `internal/embedded/pipeline.yaml` — default Phase 2 pipeline config

**Config hierarchy:**
```
PipelineConfig
  Pipeline
    AgentRoles         map[string]string        # role display names
    RolePairs          map[string]RolePairDef    # adversarial pairs
      RolePairDef
        Doer / Reviewer                         # role references
        States          RolePairStates           # 6 slots: initial/executing/submitted/reviewing/approved/rejected
    SubPipelines       map[string]SubPipeline
      SubPipeline
        Steps           []string                 # ordered role-pair names
        Transitions     []TransitionDef          # intra-sub-pipeline transitions
    PipelineTransitions []TransitionDef          # cross-sub-pipeline (3-part refs)
    EntryPoints        map[string]string         # name → "sub-pipeline.role-pair"
```

**Transition model:**
```yaml
transitions:
  - name: epic-to-us
    from: epic-planning-pair.approved
    to: us-writing-pair.initial
    trigger: manual          # human validates before proceeding
    cardinality: per-subtask # one child task per output[] entry
```

- `manual` transitions require explicit `liza proceed <task-id> <transition-name>`
- `auto` transitions execute deterministically in the supervisor loop
- `per-subtask` reads `output[]` entries from the source task (ADR-0036)
- `one-to-one` creates a single child task referencing the parent

**Design invariants:**
- The **intra-pair flow** (initial → executing → submitted → reviewing → approved|rejected) is a fixed structural invariant, never declared in YAML. Adversarial pairs are a founding principle — they mimic the coding/PR review process and scale to any activity.
- **Cross-cutting meta-states** (BLOCKED, ABANDONED, SUPERSEDED, INTEGRATION_FAILED) remain hardcoded and overlay any role-pair.
- The YAML declares only state **names** (opaque labels). The loader treats them as human-readable identifiers, not behavioral enum values.

**Runtime resolution:**
The `Resolver` wraps a loaded config and replaces all hardcoded status comparisons in the ops layer:
- `InitialStatus(rolePair)`, `ExecutingStatus(rolePair)`, `ApprovedStatus(rolePair)`
- `TransitionMap()`, `AvailableTransitions(status, executed)`
- `SprintTerminalStates()`, `TransitionTargetRolePair(name)`
- `IsClaimable()`, work detection, BLOCKED escalation

**Config lifecycle:**
1. `liza init --config pipeline.yaml` freezes the config into `.liza/pipeline.yaml`
2. All runtime lookups read from the frozen copy, never the original
3. `--entry-point` selects which sub-pipeline to start from. The Orchestrator will figure it from the input document if not provided.

**Current pipeline** (Phase 2):
```
epic-spec-subpipeline:
  epic-planning-pair → [epic-to-us] → us-writing-pair
                                           ↓ [us-to-coding]
coding-subpipeline:
  code-planning-pair → [code-plan-to-coding] → coding-pair
```

Entry points: `general-objective` (full pipeline from epic), `detailed-spec` (coding only).

### Rationale

Generic mechanisms tested once beat special-cased code duplicated per sub-pipeline. The declarative approach makes pipeline structure visible and reviewable as configuration. New sub-pipelines (architecture, integration, security) will require only YAML changes and new prompt templates, not ops layer modifications.

### Consequences

**Positive:**
- Adding new sub-pipelines requires YAML + prompt templates, not code changes
- Pipeline structure is visible, reviewable, and testable as configuration
- Resolver provides a single query interface — eliminates scattered status comparisons
- ~~Backward compatible — goals without pipeline config use hardcoded legacy paths~~ *(stale: pipeline config is now mandatory — ADR-0040)*

**Limitations accepted:**
- Config validation is at load time, not compile time — misconfigurations surface at `liza init`
- The intra-pair flow is deliberately not configurable — this is a feature, not a limitation
- ~~Legacy (non-pipeline) code paths must be maintained for backward compatibility~~ *(stale: legacy paths removed — ADR-0040)*

**Supersedes:** State machine portions of ADR-0019 (Task Lifecycle State Machine Evolution) and ADR-0020 (Explicit Task Workflow Contract).

---
*Reconstructed from commits b54dcdc..3e85eed (2026-03-02 to 2026-03-07)*
