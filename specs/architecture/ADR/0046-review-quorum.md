# 46 - Review Quorum

## Context and Problem Statement

Liza's review model was single-reviewer: one reviewer claims, approves or rejects, and the task merges. In pairing mode, the human systematically uses two reviewers — Claude and Codex — for both plan and code on every change. The multi-reviewer pattern catches issues that single-reviewer misses: different models have different blind spots, and adversarial diversity is more valuable than adversarial quantity.

The goal was to replicate this practice in MAS mode: configurable multi-reviewer approval with provider-diversity constraints, proportional to change impact.

### On reviewer quality

The mainstream position about code review is to use weaker/cheaper models. Experience shows the opposite: review is complex. In real life, you don't assign a junior to review the PR of a senior engineer. Why would you do it with agents? Quality review requires the same capability as quality production — sometimes more, because the reviewer must catch what the producer missed.

## Considered Options

1. **Configurable review quorum via `review-policy` in pipeline YAML** — quorum requirements, impact-based overrides, provider-diversity constraints, all declarative.

No alternatives were considered. The declarative pipeline model (ADR-0035, ADR-0045) provides the natural extension point.

## Decision Outcome

Chose **Option 1**: add review quorum as a first-class pipeline concept with impact-based escalation and provider-diversity merge gate.

### Architecture

**Review-policy schema (pipeline YAML):**
```yaml
role-pairs:
  coding-pair:
    review-policy:
      quorum: 1                    # default: single reviewer
      provider-diversity: preferred # base-level: applies at all impact levels
      significant-change:
        quorum: 2                  # override for significant changes
        provider-diversity: preferred
      architecture-impact:
        quorum: 2
        provider-diversity: preferred
```

`provider-diversity` is supported at the base level (works with quorum: 1) and at override levels. Override values take precedence; when absent, the base-level value applies.

**Impact classification:**
- Three levels: `standard` (default) < `significant` < `architecture`
- Coder declares impact floor at checkpoint (`write-checkpoint --impact`)
- Reviewer may upgrade impact at verdict (`submit-verdict --impact`) — escalation only, no downgrade
- Effective impact resolved from checkpoint + verdict history since last rejection
- No mechanical heuristics — reviewer judgment is the right backstop

**Quorum state machine:**
```
submitted → reviewing → approved (quorum=1)
                      → partially_approved (quorum>1, first approval)
                           → reviewing_2 (second reviewer claims)
                                → approved (quorum met)
                                → rejected (clears all approvals)
```

- `partially_approved` and `reviewing_2` are optional states declared per role-pair — only present when any override has quorum > 1
- Rejection at any stage clears all approvals — both reviewers must re-review after revision

**Approval tracking:**
- New `Approval` struct: `Agent`, `Provider`, `Timestamp`
- `Task.Approvals []Approval` replaces scalar `ApprovedBy` as canonical model
- `ApprovedBy` retained for backward compatibility (populated from last approval)
- Helper methods: `ApprovalCount()`, `HasProviderDiversity()`, `ClearApprovals()`, `LastApprover()`

**Provider-diversity merge gate:**
- Evaluated at merge time when configured as `"preferred"`
- Best-effort: if diversity is not satisfied (e.g., only one provider available), merge proceeds with a warning in merge history
- Defense-in-depth: merge gate also verifies `ApprovalCount >= effective quorum`

**Doer-provider diversity (claim-time blocking):**
- When `provider-diversity: preferred` is configured, a reviewer sharing the doer's provider is **blocked** from claiming the task if a reviewer from a different provider is registered (even if busy)
- Fallback: if no different-provider reviewer is registered, or the doer's agent is no longer in state, the block is skipped — the same-provider reviewer may claim
- This applies at all quorum levels (including quorum: 1) and to both first and second reviews

**Reviewer claim priority:**
- `PARTIALLY_APPROVED` tasks claimed before `SUBMITTED` tasks (second review shouldn't wait)
- When provider-diversity is configured, prefer reviewers from a different provider than first approver (soft preference within the candidates that survive doer-diversity blocking)

**Resolver methods added:**
- `ReviewPolicy(rolePair)` — returns policy config
- `EffectiveQuorum(rolePair, impact)` — resolves quorum + diversity for given impact
- `PartiallyApprovedStatus(rolePair)`, `Reviewing2Status(rolePair)` — optional state accessors
- `LoadEffectiveQuorum()`, `LoadReviewPolicyDiversity()` — ops-level accessors for agent package

### Rationale

Multi-reviewer approval with provider diversity replicates the human practice of seeking multiple perspectives. Impact-based escalation ensures the overhead is proportional — trivial changes don't need two reviewers, but architectural changes benefit from diverse review. Making this declarative in the pipeline YAML keeps the pattern consistent with how Liza configures everything else (ADR-0035, ADR-0045).

### Consequences

**Positive:**
- Multi-reviewer approval catches issues that single-reviewer misses
- Provider diversity ensures different model blind spots don't compound
- Impact-based escalation keeps overhead proportional to risk
- Fully declarative — quorum policy is visible and reviewable in YAML
- Backward compatible — quorum defaults to 1, existing workflows unchanged

**Limitations accepted:**
- Provider diversity at claim time is blocking when alternatives exist, but falls back gracefully when no different-provider reviewer is registered — environments with a single provider still function
- Impact classification relies on reviewer judgment — no automated heuristics
- Rejection clears all approvals — conservative but simple; partial re-review would add complexity

**Extends:** ADR-0035 (Declarative Sub-Pipelines) — new states and transitions. ADR-0045 (Declarative Role Definitions) — provider metadata enables diversity constraints.

---
*Reconstructed from commits 8663ae3..76db525 (2026-03-16), extended with 1af2159 (2026-03-21)*
