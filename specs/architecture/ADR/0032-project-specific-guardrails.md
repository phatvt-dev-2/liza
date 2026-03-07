# 32 - Project-Specific Guardrails

## Context and Problem Statement

The core contract (CORE.md) is project-agnostic by design — it defines universal rules for all Liza deployments. But projects have specific constraints that need enforcement, not just documentation. The existing `lessons/` mechanism captured insights but had no enforcement power: agents might read lessons but could still violate them.

The need crystallized when the post-worktree hardcoding incident (ADR-0031) revealed that project-level rules like "no Go-specific assumptions in runtime code" needed the same tier-based enforcement as universal contract rules.

## Considered Options

1. **Add project rules to `state.yaml` config** — not persistent across goal resets; config is for runtime parameters, not constraints.
2. **Add project-specific sections to CORE.md** — violates the project-agnostic principle; CORE.md must remain universal.
3. **Separate GUARDRAILS.md with the same tier system** — project-level file with full enforcement semantics.

## Decision Outcome

Chose **Option 3**: introduce `GUARDRAILS.md` at the project root, reusing the Tier 0-3 hierarchy from CORE.md.

### Architecture

**Structure:**
```markdown
# Project Guardrails
## Tier 0 (Inviolable)
## Tier 1 (Hard Constraints)
## Tier 2 (Strong Defaults)
## Tier 3 (Preferences)
```

**Deployment:**
- Embedded template in the `liza` binary (`internal/embedded/guardrails-template.md`)
- Written on `liza init` (non-fatal, respects existing files)
- Placed at project root (visible, git-tracked, human-editable)

**Enforcement:**
- Agents read GUARDRAILS.md during initialization (step 5 of FIRST ACTIONS in `base_prompt.tmpl`)
- Re-read on context degradation alongside CORE.md (Context Management → Transition Protocol)
- Tier 0-1 violations trigger the same RESET protocol as core contract violations

**Contract reference** (CORE.md):
> If `GUARDRAILS.md` exists at the project root, read and enforce it as project-specific constraints. GUARDRAILS.md uses and extends the same tier system (Tier 0-3) defined in Rule Priority Architecture.

### Rationale

Constraints need enforcement mechanisms, not just documentation. By reusing the existing tier system, agents already understand the semantics — no new concepts to learn. Keeping it separate from CORE.md preserves the universal/project boundary cleanly.

### Consequences

**Positive:**
- Project-specific rules get the same enforcement power as universal rules
- Agents already understand the tier system — zero learning curve
- Git-tracked and human-readable — easy to review and maintain
- Embedded template reduces onboarding friction

**Limitations accepted:**
- Enforcement relies on agents reading the file (prompt-based, not code-enforced)
- Currently only one rule (G1.1) — mechanism is proven but lightly used

---
*Reconstructed from commits a29de97, 0a53d76 (2026-03-01 to 2026-03-05)*
