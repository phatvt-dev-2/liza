# 42 - Generic Claim-Type Vocabulary (Doer/Reviewer)

## Context and Problem Statement

ADR-0035 (Declarative Sub-Pipelines) introduced the doer/reviewer taxonomy as the adversarial load-bearing principle: every sub-pipeline is an adversarial pair with a doer who produces work and a reviewer who validates it. Yet the original code — dating from when only the coding role pair existed — still used `coder`/`code-reviewer` as claim-type selectors in the CLI, MCP, and ops layers.

This created confusion: a code-planner had to pass `"coder"` to release its claim, because "coder" didn't mean the coder role — it meant "the doer slot." The vocabulary didn't match the concept it represented.

Doer and reviewer are not roles. They are the two sides of the adversarial pair — a structural concept that predates and outlives any specific role.

## Considered Options

1. **Rename to `doer`/`reviewer`** — vocabulary matches the adversarial concept directly.

No alternatives were considered. The vocabulary should reflect the foundational concept, not a historical artifact.

## Decision Outcome

Chose **Option 1**: rename claim-type selectors from `coder`/`code-reviewer` to `doer`/`reviewer`/`both` across all layers.

### Changes

- `ClaimDoer`, `ClaimReviewer`, `ClaimBoth` constants added to `internal/roles/` as single source of truth
- CLI `--role` flag on `liza release-claim`: `"coder"` → `"doer"`, `"code-reviewer"` → `"reviewer"`
- MCP `role` parameter on `liza_release_claim`: same rename
- Ops layer: `ReleaseClaim` uses the new constants throughout
- Default changed from `"code-reviewer"` to `"reviewer"`

### Rationale

The adversarial pair is a founding principle of Liza's architecture. The vocabulary at every layer should reflect this: doer produces, reviewer validates. Using role-specific names (`coder`/`code-reviewer`) for a role-agnostic concept created a false coupling between the claim mechanism and the coding role pair.

### Consequences

**Positive:**
- Vocabulary matches the adversarial concept — no more "pass 'coder' even though you're a planner"
- Constants as single source of truth (consistent with ADR-0024)
- CLI, MCP, and ops layers use identical vocabulary

**Limitations accepted:**
- None — Liza is at development stage; breaking changes are expected

**BREAKING CHANGE:** The `--role` flag on `liza release-claim` and the `role` parameter on `liza_release_claim` MCP tool now accept `"doer"`/`"reviewer"` instead of `"coder"`/`"code-reviewer"`.

**Completes:** The vocabulary generalization arc: ADR-0033 (Orchestrator Rename) generalized the supervisor name, ADR-0024 (Unified Role Constants) centralized role strings, and this ADR aligns the claim-type vocabulary with the adversarial principle from ADR-0035.

---
*Reconstructed from commit 0a53748 (2026-03-10)*
