# Phase 3: Review Quorum — Implementation Plan

Spec ref: `specs/build/3 - Declarative Role Definitions.md`, Phase 3 section.

## Overview

Phase 3 adds review quorum as a first-class concept: role-pairs can require
multiple approvals with provider diversity constraints before merge eligibility.

**Key changes:**
1. Review-policy configuration on role-pairs (quorum, impact-based overrides)
2. New quorum states: `PARTIALLY_APPROVED`, `REVIEWING_2`
3. Impact classification on checkpoints and verdicts
4. Migration from `approved_by` (scalar) to `approvals[]` (list)
5. Multi-reviewer state machine and claim priority logic

## Current State

- Single reviewer per task: `reviewing_by` + `review_lease_expires`
- Single approval: `approved_by *string` (scalar)
- Reviewer claims from `SUBMITTED` → `REVIEWING` → `APPROVED | REJECTED`
- Merge triggered by reviewer whose `agentID == *task.ApprovedBy`
- No impact classification on checkpoints or verdicts
- `Agent.Provider` already persisted at registration (from `--cli` flag)

## Task Decomposition

### CP3-1: Add review-policy schema to pipeline role-pair config

**Intent:** Define the review-policy schema that controls quorum requirements.

**Changes:**
- Add `ReviewPolicyDef` struct: `Quorum int`, `SignificantChange *ReviewPolicyOverrideDef`,
  `ArchitectureImpact *ReviewPolicyOverrideDef`
- Add `ReviewPolicyOverrideDef` struct: `Quorum int`, `ProviderDiversity string`
- Add `ReviewPolicy *ReviewPolicyDef` field to `RolePairDef`
- Validation: quorum >= 1 (0 treated as 1), `provider-diversity` must be `"preferred"` or empty
- Add resolver method: `ReviewPolicy(rolePair string) (*ReviewPolicyDef, error)`
- Add resolver method: `EffectiveQuorum(rolePair string, impact string) (quorum int, diversity string, err error)`
  — resolves quorum + diversity for a given impact level against the review-policy hierarchy
- Update `internal/embedded/pipeline.yaml`: add `review-policy` to coding-pair with default quorum 1

**Scope:**
- `internal/pipeline/config.go` — structs, validation
- `internal/pipeline/resolver.go` — new methods
- `internal/embedded/pipeline.yaml` — default config
- `internal/pipeline/config_test.go` — validation tests
- `internal/pipeline/resolver_test.go` — resolver tests

**Depends on:** nothing

---

### CP3-2: Add quorum states to role-pair schema and transition map

**Intent:** Extend the state machine to support partially-approved and reviewing-2 states.

**Changes:**
- Add `PartiallyApproved string` and `Reviewing2 string` optional fields to `RolePairStates`
  (yaml: `partially-approved` and `reviewing-2`, `omitempty`)
- Add `"partially-approved"` and `"reviewing-2"` to `validPhases` map
- Add resolver methods: `PartiallyApprovedStatus(rolePair)`, `Reviewing2Status(rolePair)`
  — return empty TaskStatus (not error) when state is not declared (optional states)
- Add to `PipelineResolver` interface in `internal/models/task.go`:
  `PartiallyApprovedStatus(rolePair string) (TaskStatus, error)` and
  `Reviewing2Status(rolePair string) (TaskStatus, error)`
- Update mock `PipelineResolver` in test files to implement new methods
- Extend `AllDeclaredStates()` to include these when non-empty
- Validation: if any review-policy override has quorum > 1 on a role-pair, both
  `partially-approved` and `reviewing-2` states MUST be declared on that role-pair
- Extend `TransitionMap()` — when states are declared:
  - `reviewing → partially-approved` (added alongside existing `reviewing → approved`)
  - `partially-approved → reviewing-2`
  - `reviewing-2 → approved | rejected`
- Extend `BuildPipelineTransitions()` — cross-cutting for new states:
  - `reviewing-2 → partially-approved` (stale claim revert)
  - `partially-approved → abandoned | superseded`
  - `reviewing-2 → rejected` flows to same rejected state as reviewing-1
- Update `internal/embedded/pipeline.yaml`: add `partially-approved` and `reviewing-2`
  states on coding-pair (e.g., `CODE_PARTIALLY_APPROVED`, `REVIEWING_CODE_2`)

**Scope:**
- `internal/pipeline/config.go` — RolePairStates fields, validPhases, validation
- `internal/pipeline/resolver.go` — new methods, TransitionMap extension
- `internal/ops/pipeline_ops.go` — BuildPipelineTransitions extension
- `internal/models/task.go` — PipelineResolver interface extension
- `internal/embedded/pipeline.yaml` — quorum states
- Tests in all affected packages (including mock updates)

**Depends on:** CP3-1

---

### CP3-3: Add Approval struct and approvals field to Task model

**Intent:** Establish the canonical data model for tracking multiple reviewer approvals.

**Changes:**
- Add `Approval` struct: `Agent string`, `Provider string`, `Timestamp time.Time`
  (yaml: `agent`, `provider`, `timestamp`)
- Add `Approvals []Approval` to `Task` (yaml: `approvals,omitempty`)
- Add `Impact string` to `Task` (yaml: `impact,omitempty`) — current impact classification
- Keep `ApprovedBy *string` field on Task (set as derived from last approval, removed later
  in Phase 4 or after migration stabilizes)
- Helper methods on `*Task`:
  - `ApprovalCount() int`
  - `HasProviderDiversity() bool` — true if approvals contain ≥2 distinct providers
  - `ClearApprovals()` — sets Approvals to nil
  - `LastApprover() string` — returns agent ID of last approval, empty if none
- Model-level tests for Approval struct and helpers

**Scope:**
- `internal/models/task.go` — Approval struct, field, helpers
- `internal/models/task_test.go` — tests

**Depends on:** nothing

---

### CP3-4: Add impact field to write-checkpoint operation

**Intent:** Allow coders to declare change impact classification in checkpoints.

**Changes:**
- Add `Impact string` to `WriteCheckpointInput`
- Validate impact: must be empty (defaults to "standard"), "standard", "significant",
  or "architecture"
- Store impact in checkpoint history `Extra` map (`"impact": value`)
- Add helper: `GetCheckpointImpact(history []TaskHistoryEntry, agentID string) string`
  — returns impact from the latest checkpoint by the given agent
- Update MCP handler `handleWriteCheckpoint` to extract `impact` from params
- Update MCP tool description in `server_registration.go` to document impact parameter
- Update CLI if `write-checkpoint` has a CLI surface (verify — currently no CLI command
  for write-checkpoint, only MCP)

**Scope:**
- `internal/ops/write_checkpoint.go` — input, validation, storage, helper
- `internal/mcp/handlers_complex.go` — extract param
- `internal/mcp/server_registration.go` — tool description
- Tests for write_checkpoint, MCP handler

**Depends on:** nothing

---

### CP3-5: Migrate SubmitVerdict and merge gate from approved_by to approvals[]

**Intent:** Switch approval tracking from single-value approved_by to approvals[] list,
maintaining backward compatibility and updating all read sites.

**Changes:**
- In `SubmitVerdict` on APPROVED:
  - Look up reviewer's provider from agent registry in state
  - Build `Approval{Agent: agentID, Provider: provider, Timestamp: now}`
  - Append to `task.Approvals`
  - Set `task.ApprovedBy = &agentID` (backward compat, derived)
- In `SubmitVerdict` on REJECTED:
  - Set `task.Approvals = nil` (clear all approvals — spec: both reviewers re-review)
- In `handleApprovedMerges` (`internal/agent/claiming.go`):
  - Replace `task.ApprovedBy != nil && *task.ApprovedBy == agentID`
    with `task.LastApprover() == agentID`
- In `hasPendingMerges` (`internal/agent/claiming.go`):
  - Same condition update
- On merge: log provider diversity status in history entry Extra
  (e.g., `"provider_diversity": true/false, "providers": ["claude", "codex"]`)
- Update `VerdictResult` to include approval metadata
- Update CLI `printVerdictResult` and MCP response for approval info

**Scope:**
- `internal/ops/submit_verdict.go` — core migration
- `internal/agent/claiming.go` — merge gate condition
- `internal/commands/submit_verdict.go` — CLI output
- `internal/mcp/handlers_mutation.go` — MCP response (if needed)
- Tests in all affected packages

**Depends on:** CP3-3

---

### CP3-6: Implement quorum evaluation and impact upgrade in SubmitVerdict

**Intent:** When a verdict is approved, evaluate quorum from review-policy to decide
whether the task transitions to approved or partially_approved. Support impact upgrade
by reviewers.

**Changes:**
- Add `impact string` parameter to `SubmitVerdict` function signature
- In the MCP handler `handleSubmitVerdict`: extract optional `impact` from params, pass
  to `SubmitVerdict`
- In CLI `submitVerdictCmd`: add optional `--impact` flag, pass to command
- In `commands.SubmitVerdictCommand`: accept and pass impact
- In `SubmitVerdict` on APPROVED:
  1. Determine base impact: max of checkpoint impact (via `GetCheckpointImpact`),
     current `task.Impact`, and verdict impact parameter
  2. Validate: impact can only escalate (new >= current). Values ordered:
     standard < significant < architecture
  3. Store resolved impact on `task.Impact`
  4. Load review-policy via resolver: `EffectiveQuorum(task.RolePair, resolvedImpact)`
  5. If `len(task.Approvals) >= quorum`: transition to approved (existing path)
  6. If `len(task.Approvals) < quorum`: transition to `partially_approved`
     (new path — uses `PartiallyApprovedStatus` from resolver)
- In `SubmitVerdict` when task is in `reviewing_2` state (second review):
  - Validate task is in `reviewing_2` status (in addition to `reviewing`)
  - On APPROVED: append approval, check quorum → should now be met → transition to approved
  - On REJECTED: clear approvals, transition to rejected
- Update `VerdictResult` with quorum-related info (partially_approved flag, quorum progress)
- Update MCP tool description for submit-verdict to document impact parameter

**Scope:**
- `internal/ops/submit_verdict.go` — quorum evaluation, impact handling
- `internal/mcp/handlers_mutation.go` — extract impact param
- `internal/mcp/server_registration.go` — tool description
- `internal/commands/submit_verdict.go` — accept impact
- `cmd/liza/cmd_review.go` — --impact flag
- Tests: comprehensive quorum scenarios

**Depends on:** CP3-1, CP3-2, CP3-4, CP3-5

---

### CP3-7: Extend ClaimReviewerTask for partially-approved tasks

**Intent:** Allow reviewers to claim partially-approved tasks, with priority over fresh
submissions and provider diversity preference.

**Changes:**
- In `isClaimablePipeline` (reviewer case): add `partially_approved` as claimable status
  alongside `submitted`
- In `ClaimReviewerTask`:
  - When claiming from `partially_approved` state: transition to `reviewing_2`
    (not `reviewing`)
  - Detect task state to choose correct target status
- Claim priority (in candidate selection):
  - Partition candidates into two tiers: `PARTIALLY_APPROVED` and `SUBMITTED`
  - Select from `PARTIALLY_APPROVED` first (completing quorum > starting new review)
  - Within each tier, maintain existing priority-based selection
- Provider diversity preference:
  - Among candidates at same priority, prefer tasks where the claiming reviewer's
    provider differs from existing approvals' providers
  - This is a soft preference — if no diversity-satisfying task exists, claim any
  - Requires reading `Agent.Provider` from state for the claiming agent

**Scope:**
- `internal/ops/claim_reviewer_task.go` — claim logic, priority, diversity
- `internal/models/task.go` — IsClaimable extension
- Tests for all claim scenarios (partially-approved, diversity, priority)

**Depends on:** CP3-2, CP3-3

---

### CP3-8: Extend stale review claim clearing for reviewing_2 state

**Intent:** Handle expired reviewer leases on tasks in reviewing_2 by reverting to
partially_approved (not submitted).

**Changes:**
- In `ClearStaleReviewClaims`: detect tasks in `reviewing_2` state with expired
  `ReviewLeaseExpires`
- Revert `reviewing_2` → `partially_approved` (preserve existing approvals)
- Revert `reviewing` → `submitted` (existing behavior, unchanged)
- The distinction requires knowing which reviewing state the task is in and using
  the correct revert target

**Scope:**
- `internal/ops/clear_stale_review_claims.go` — extended clearing logic
- Tests for stale clearing of reviewing_2

**Depends on:** CP3-2

---

### CP3-9: Update diagnostics for quorum states

**Intent:** Ensure reviewer work detection and diagnostics account for quorum states.

**Changes:**
- `CountReviewableTasks`: include `partially_approved` tasks as claimable
  (this follows automatically from IsClaimable changes in CP3-7, but verify)
- `GetReviewerWorkDiagnostics`: report `partially_approved` and `reviewing_2` states
  in diagnostic output — add categories for "partially approved (awaiting 2nd review)"
  and "in second review"
- Ensure `liza status` output correctly displays the new states when present

**Scope:**
- `internal/models/diagnostics.go` — diagnostic functions
- `internal/ops/status.go` — status display (if quorum states need special formatting)
- Tests for diagnostics with quorum states

**Depends on:** CP3-2, CP3-7

---

## Dependency Graph

```
CP3-1 ─────────────────────────────┐
  │                                 │
  v                                 │
CP3-2 ──────────┬──────────┐       │
  │              │          │       │
  │              v          v       │
  │           CP3-7      CP3-8     │
  │              │                  │
  │              v                  │
  │           CP3-9                 │
  │                                 │
CP3-3 ──────────┬──────────┐       │
  │              │          │       │
  v              v          │       │
CP3-4         CP3-5         │       │
  │              │          │       │
  │              v          │       │
  └──────────►CP3-6◄───────┘───────┘
```

**Critical path:** CP3-1 → CP3-2 → CP3-7 → CP3-9 (state machine + claiming)
**Parallel paths:** CP3-3 and CP3-4 can start independently alongside CP3-1
**Integration:** CP3-6 requires all of CP3-1, CP3-2, CP3-4, CP3-5

## Spec Coverage Verification

| Spec Requirement | Task(s) |
|---|---|
| Add `review-policy` to role-pair schema | CP3-1 |
| Add `partially-approved` and `reviewing-2` states | CP3-2 |
| Add `impact` field to checkpoint tool | CP3-4 |
| Add `impact` field to verdict tool | CP3-6 |
| Migrate `approved_by` to `approvals[]` | CP3-3 (model), CP3-5 (ops) |
| `PARTIALLY_APPROVED → REVIEWING_2 → APPROVED/REJECTED` transitions | CP3-2 (map), CP3-6 (verdict), CP3-7 (claim) |
| Extend `reviewing_by`/`review_lease_expires` for second claim | CP3-7 |
| Rejection clears `approvals[]` | CP3-5 |
| Merge gate checks quorum + provider diversity | CP3-5 (condition), CP3-6 (quorum) |
| Reviewer claim priority: `PARTIALLY_APPROVED` first | CP3-7 |
| Provider diversity preference in claims | CP3-7 |
| Impact upgrade triggers quorum recalculation | CP3-6 |
| Stale lease handling for reviewing_2 | CP3-8 |
| Provider metadata (already exists on Agent) | — (no change needed) |

## Risks and Assumptions

**ASSUMPTION:** Phase 1 (declarative role properties) and Phase 2 (composable prompt
sections) are already implemented and merged. The current codebase reflects this
(pipeline YAML has roles section, resolver has role methods).

**ASSUMPTION:** `Agent.Provider` is reliably populated during registration. Current code
confirms this — provider is set from `--cli` flag during `registerAgent`.

**RISK:** The `reviewing_2` state introduces a second active review within the same
role-pair. Operations that assume at most one active reviewer per task (e.g., worktree
management, lease cleanup) need careful audit.

**RISK:** Impact escalation mid-review could surprise the first reviewer. The spec
explicitly accepts this trade-off (Section "Trade-off: first reviewer's scrutiny level
on impact upgrade").
