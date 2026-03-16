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
6. Best-effort provider-diversity merge gate

## Current State

- Single reviewer per task: `reviewing_by` + `review_lease_expires`
- Single approval: `approved_by *string` (scalar)
- Reviewer claims from `SUBMITTED` → `REVIEWING` → `APPROVED | REJECTED`
- Merge triggered by reviewer whose `agentID == *task.ApprovedBy`
- No impact classification on checkpoints or verdicts
- `Agent.Provider` already persisted at registration (from `--cli` flag)

## Task Decomposition

### CP3-1: Add review-policy schema to pipeline role-pair config

**Desc:** Add review-policy schema to pipeline role-pair config

**Done When:** ReviewPolicyDef and ReviewPolicyOverrideDef structs exist in internal/pipeline/config.go. RolePairDef has ReviewPolicy field. Validation rejects quorum < 1 and invalid provider-diversity values. Resolver methods ReviewPolicy() and EffectiveQuorum() return correct values. pipeline.yaml coding-pair has review-policy with quorum: 1. TestReviewPolicyValidation and TestEffectiveQuorum pass.

**Scope:** internal/pipeline/config.go, internal/pipeline/resolver.go, internal/embedded/pipeline.yaml, config tests, resolver tests

**Spec Ref:** specs/build/3 - Declarative Role Definitions.md — Phase 3: 'Add review-policy to role-pair schema'

**Depends on:** nothing

---

### CP3-2: Add quorum states to role-pair schema and transition map

**Desc:** Add quorum states to role-pair schema and transition map

**Done When:** RolePairStates has PartiallyApproved and Reviewing2 optional string fields. validPhases includes partially-approved and reviewing-2. Resolver methods PartiallyApprovedStatus() and Reviewing2Status() exist. PipelineResolver interface in models/task.go includes both methods. Validation rejects role-pairs where the effective quorum can exceed 1 but quorum states are missing — this includes both role-pairs with base quorum > 1 AND role-pairs where any override (significant-change, architecture-impact) specifies quorum > 1, even if base quorum is 1. TransitionMap() includes reviewing → partially-approved, partially-approved → reviewing-2, reviewing-2 → approved|rejected when states declared. BuildPipelineTransitions includes reviewing-2 → partially-approved (stale revert) and partially-approved → abandoned|superseded. pipeline.yaml coding-pair declares CODE_PARTIALLY_APPROVED and REVIEWING_CODE_2. All existing tests pass plus new tests for quorum state transitions including TestQuorumStatesRequiredForOverrides (base quorum 1, override quorum 2, missing states → rejected).

**Scope:** internal/pipeline/config.go, internal/pipeline/resolver.go, internal/ops/pipeline_ops.go, internal/models/task.go (interface only), internal/embedded/pipeline.yaml, tests in all affected packages

**Spec Ref:** specs/build/3 - Declarative Role Definitions.md — Phase 3: 'Add partially-approved and reviewing-2 states to role-pair schema'

**Depends on:** CP3-1

---

### CP3-3: Add Approval struct and approvals field to Task model

**Desc:** Add Approval struct and approvals field to Task model

**Done When:** Approval struct with Agent, Provider, Timestamp fields exists in internal/models/task.go. Task has Approvals []Approval field (yaml: approvals,omitempty) and Impact string field (yaml: impact,omitempty). Helper methods ApprovalCount(), HasProviderDiversity(), ClearApprovals(), LastApprover() exist on *Task. ApprovedBy *string field retained for backward compat. TestApprovalHelpers passes covering all helpers including edge cases (empty list, single approval, diverse providers).

**Scope:** internal/models/task.go, internal/models/task_test.go

**Spec Ref:** specs/build/3 - Declarative Role Definitions.md — Phase 3: 'Migrate approved_by to approvals[] as canonical representation', New Blackboard Fields

**Depends on:** nothing

---

### CP3-4: Add impact field to write-checkpoint operation

**Desc:** Add impact field to write-checkpoint operation

**Done When:** WriteCheckpointInput has Impact string field. Validation accepts empty, standard, significant, architecture and rejects other values. Impact stored in checkpoint history Extra map. GetCheckpointImpact helper returns impact from latest checkpoint. MCP handler extracts impact param. MCP tool description includes impact parameter. TestWriteCheckpointImpact passes for valid and invalid values.

**Scope:** internal/ops/write_checkpoint.go, internal/mcp/handlers_complex.go, internal/mcp/server_registration.go, write_checkpoint tests, MCP handler tests

**Spec Ref:** specs/build/3 - Declarative Role Definitions.md — Phase 3: 'Add impact field to checkpoint and verdict tools'

**Depends on:** nothing

---

### CP3-5: Migrate SubmitVerdict and merge identity check from approved_by to approvals[]

**Desc:** Migrate SubmitVerdict and merge identity check from approved_by to approvals[]

**Done When:** SubmitVerdict on APPROVED builds Approval with agent, provider (from state.Agents), timestamp and appends to task.Approvals. Sets task.ApprovedBy as derived. SubmitVerdict on REJECTED clears task.Approvals to nil. handleApprovedMerges uses task.LastApprover() == agentID instead of task.ApprovedBy check. hasPendingMerges uses same condition. TestSubmitVerdictApprovals and TestMergeIdentityCheck pass.

**Scope:** internal/ops/submit_verdict.go, internal/agent/claiming.go, tests

**Spec Ref:** specs/build/3 - Declarative Role Definitions.md — Phase 3: 'Migrate approved_by to approvals[]', 'Rejection at any stage clears approvals[]'

**Depends on:** CP3-3

---

### CP3-6: Implement quorum evaluation and impact upgrade in SubmitVerdict

**Desc:** Implement quorum evaluation and impact upgrade in SubmitVerdict

**Done When:** SubmitVerdict accepts optional impact parameter. Impact escalation enforced (new >= current, ordered: standard < significant < architecture). Resolved impact stored on task.Impact. On APPROVED: EffectiveQuorum evaluated; if len(approvals) < quorum, task transitions to partially_approved instead of approved. On second approval (reviewing_2 state), transitions to approved when quorum met. MCP handler and CLI pass impact. MCP tool description updated. TestQuorumEvaluation passes with scenarios: quorum-1 standard path, quorum-2 both reviewers approve, impact upgrade triggers partial approval, rejection clears and restarts.

**Scope:** internal/ops/submit_verdict.go, internal/mcp/handlers_mutation.go, internal/mcp/server_registration.go, internal/commands/submit_verdict.go, cmd/liza/cmd_review.go, tests

**Spec Ref:** specs/build/3 - Declarative Role Definitions.md — Phase 3: 'Add impact field to checkpoint and verdict tools', 'Implement PARTIALLY_APPROVED → REVIEWING_2 → APPROVED/REJECTED', 'Impact upgrade triggers quorum recalculation mid-review'

**Depends on:** CP3-1, CP3-2, CP3-4, CP3-5

---

### CP3-7: Extend ClaimReviewerTask for partially-approved tasks with priority and diversity

**Desc:** Extend ClaimReviewerTask for partially-approved tasks with priority and diversity

**Done When:** isClaimablePipeline returns true for reviewer role when task status is partially_approved. ClaimReviewerTask transitions partially_approved → reviewing_2. Claim priority: partially_approved candidates selected before submitted candidates at same priority level. Provider diversity for partially-approved tasks: among equal-priority candidates, tasks where claiming reviewer's provider differs from existing approvals are preferred (soft preference). Provider diversity for fresh submissions (submitted tasks with no existing approvals): general rule — among equal-priority candidates, prefer tasks where provider diversity is satisfiable from the reviewer pool, i.e., at least one other registered reviewer for the role-pair has a provider different from the claiming reviewer's provider (soft preference); when no other registered reviewer has a different provider (all share the claiming reviewer's provider, or no other reviewers exist), diversity is not satisfiable and no preference is applied. TestClaimPartiallyApproved, TestClaimPriority, TestClaimDiversityWithApprovals, TestClaimDiversityFreshSubmissions pass. TestClaimDiversityFreshSubmissions covers: (a) single alternate reviewer with different provider — preferred, (b) single alternate reviewer with same provider — no preference, (c) multiple alternate reviewers all sharing one provider different from claiming reviewer — preferred (diversity satisfiable), (d) multiple alternate reviewers with mixed providers — preferred (diversity always satisfiable).

**Scope:** internal/ops/claim_reviewer_task.go, internal/models/task.go, tests

**Spec Ref:** specs/build/3 - Declarative Role Definitions.md — Phase 3: 'Extend reviewing_by / review_lease_expires to work for second review claim', 'Implement reviewer claim priority: PARTIALLY_APPROVED first, then diversity-satisfying tasks'

**Depends on:** CP3-2, CP3-3

---

### CP3-8: Extend stale review claim clearing for reviewing_2 state

**Desc:** Extend stale review claim clearing for reviewing_2 state

**Done When:** ClearStaleReviewClaims detects tasks in reviewing_2 status with expired ReviewLeaseExpires. Expired reviewing_2 tasks revert to partially_approved (not submitted). Expired reviewing tasks still revert to submitted (unchanged). TestClearStaleReviewingTwo passes.

**Scope:** internal/ops/clear_stale_review_claims.go, tests

**Spec Ref:** specs/build/3 - Declarative Role Definitions.md — Phase 3: 'Extend reviewing_by / review_lease_expires to work for second review claim' (stale lease handling implied)

**Depends on:** CP3-2

---

### CP3-9: Update diagnostics for quorum states

**Desc:** Update diagnostics for quorum states

**Done When:** CountReviewableTasks counts partially_approved tasks (follows from IsClaimable change in CP3-7). GetReviewerWorkDiagnostics reports partially_approved tasks as 'awaiting second review' and reviewing_2 tasks as 'in second review' in diagnostic output. TestDiagnosticsQuorumStates passes.

**Scope:** internal/models/diagnostics.go, internal/ops/status.go (if needed), diagnostics tests

**Spec Ref:** specs/build/3 - Declarative Role Definitions.md — Phase 3: state machine extension (diagnostics correctness for new states)

**Depends on:** CP3-2, CP3-7

---

### CP3-10: Implement best-effort provider-diversity merge gate

**Desc:** Implement best-effort provider-diversity merge gate

**Done When:** handleApprovedMerges loads review-policy for the task's role-pair via EffectiveQuorum. Defense-in-depth: if task.ApprovalCount() < effective quorum, logs anomaly and skips merge. When provider-diversity is 'preferred': if task.HasProviderDiversity() is true, merge proceeds and merge history Extra includes diversity_achieved: true. If task.HasProviderDiversity() is false, checks all registered reviewer agents' providers for the role-pair — if all share one provider, merge proceeds and merge history Extra includes diversity_not_achievable: true with reason (e.g. 'all reviewers use provider X'). If different providers exist but diversity was not achieved, merge proceeds and merge history Extra includes diversity_not_met: true. When provider-diversity is not configured, merge proceeds without diversity fields. TestMergeGateDiversityAchieved, TestMergeGateDiversityNotAchievable, TestMergeGateQuorumDefenseInDepth, TestMergeGateDiversityNotConfigured pass.

**Scope:** internal/agent/claiming.go, internal/ops/wt_merge.go (merge history Extra parameter), tests

**Spec Ref:** specs/build/3 - Declarative Role Definitions.md — Phase 3: 'Modify reviewer PreWork merge gate to check quorum and provider diversity (best-effort)', lines 300-306

**Depends on:** CP3-1, CP3-3, CP3-5

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
                 │
              CP3-10◄──CP3-1, CP3-3, CP3-5
```

**Critical path:** CP3-1 → CP3-2 → CP3-7 → CP3-9 (state machine + claiming)
**Parallel paths:** CP3-3 and CP3-4 can start independently alongside CP3-1
**Integration:** CP3-6 requires all of CP3-1, CP3-2, CP3-4, CP3-5
**Merge gate:** CP3-10 requires CP3-1, CP3-3, CP3-5

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
| Modify reviewer PreWork merge gate: quorum + provider diversity (best-effort) | CP3-10 |
| 'All reviewers share one provider' fallback logic | CP3-10 |
| Reviewer claim priority: `PARTIALLY_APPROVED` first | CP3-7 |
| Provider diversity preference in claims (partially-approved: vs existing approvals) | CP3-7 |
| Provider diversity preference in claims (fresh submissions: vs reviewer pool) | CP3-7 |
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
