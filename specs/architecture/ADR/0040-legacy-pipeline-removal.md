# 40 - Legacy Pipeline Removal

## Context and Problem Statement

ADR-0035 introduced declarative sub-pipelines with a dual-path design: operations checked for a pipeline resolver and fell back to hardcoded legacy paths when absent. This was always intended as temporary scaffolding — the legacy paths existed to avoid a flag-day migration, not as a permanent feature.

With the pipeline model stable in production and an end-to-end sprint sequence test (`ae9ae1c`) validating the full workflow, the dual paths had become pure maintenance cost. Every operation carried `if resolver != nil { pipeline } else { legacy }` branches, doubling the code surface to test and reason about.

## Considered Options

1. **Keep dual paths longer** — maintain backward compatibility for workspaces without `pipeline.yaml`.
2. **Remove legacy code paths** — make `pipeline.yaml` mandatory, delete all fallback branches.

## Decision Outcome

Chose **Option 2**: remove all legacy (non-pipeline) code paths and make pipeline configuration mandatory.

### Changes

**Production:**
- `LoadFrozen` returns error when `pipeline.yaml` missing (was `nil, nil`)
- `liza init` auto-freezes embedded `pipeline.yaml` when `--config` absent
- Removed `Transition()`, `CanTransition()`, and the hardcoded `taskTransitions` map
- Removed legacy branches from all ops: claim, release, submit, verdict, mark_blocked, proceed, supersede, clear_stale, claim_reviewer, merge
- `IsClaimable`, `IsApprovedForMerge`, `IsExecutingStatus`: pipeline-only implementations
- `RolePair` required on all tasks; `AddTask` validates before pipeline load
- Supervisor and work-detection log warnings on resolver load failure instead of silently no-oping

**Preparation:** `61063cf` aligned Go constants and templates to pipeline state names (6 coding-pair constants, ~250 string literals across ~55 test files), removing the naming gap between code and configuration.

**Result:** 68 files changed, net ~400 lines deleted. The codebase now has a single code path per operation.

### Rationale

The dual-path design served its purpose during the transition period. Removing it:
- Eliminates an entire class of "which path are we on?" bugs
- Reduces test surface (deleted legacy-specific tests)
- Makes the declarative pipeline the only mental model developers need
- The e2e sprint test provided confidence that the pipeline path handles the full lifecycle

The direction is to make the workflow as declarative as possible (extending ADR-0019 and ADR-0020).

### Consequences

**Positive:**
- Single code path per operation — simpler reasoning, fewer tests
- ~400 lines of net deletion
- Pipeline configuration is the only mental model

**Limitations accepted:**
- Existing workspaces without `.liza/pipeline.yaml` must re-run `liza init`
- Tasks without `role_pair` are rejected by all ops

**BREAKING CHANGE:** Existing workspaces must re-run `liza init`. Tasks without `role_pair` are rejected.

**Completes:** ADR-0035 (Declarative Sub-Pipelines) — the dual-path design was always temporary.
**Extends:** ADR-0019 (Task Lifecycle State Machine Evolution), ADR-0020 (Explicit Task Workflow Contract) — continuing the direction toward fully declarative workflows.

---
*Reconstructed from commits 61063cf..4bc048f (2026-03-10)*
