# 27 - Contract Compression for MAM Context

## Context and Problem Statement

Agents in multi-agent mode were under context pressure partly because CORE.md contained pairing-specific instructions that MAM agents loaded but never used. CORE.md is loaded at system prompt level — it has the highest authority — but that authority came at the cost of context budget for every agent, regardless of mode. With prompt templates also being compressed (ADR-0026), the contract itself was the next target.

The tension: CORE.md content has system-prompt-level authority, while PAIRING_MODE.md and MULTI_AGENT_MODE.md do not. Moving load-bearing instructions out of CORE weakens their enforcement. But keeping pairing-specific content in CORE wastes context for MAM agents that outnumber pairing sessions.

## Considered Options

1. **Keep CORE.md as-is** — accept the context cost for MAM agents.
2. **Compress CORE.md** — move pairing-specific content to PAIRING_MODE.md, eliminate duplication, keep universal safety rules in CORE.
3. **Split CORE into mandatory/optional sections** — more aggressive decomposition by agent role.

## Decision Outcome

Chose **Option 2**: targeted compression preserving all universal safety rules in CORE.

### Architecture

**Removed from CORE.md** (-69 lines, 799→730):
- Runtime Kernel appendix (duplicated Tier 0 and state machine content)
- Assumption Comfort Levels table (compressed into the assumption budget rule)
- Quick Self-Check (folded into Rule 7 — Think Before Acting)

**Relocated to PAIRING_MODE.md:**
- Struggle Protocol template with mode-switch prompt (MAM uses BLOCKED instead)
- Rule 12 peer collaboration dynamics
- Rule 13 behavioral framing (Constructive Contrarian philosophy)

**Retained in CORE (universal):**
- Rule 12 Professional Judgment — Mechanical Triggers and Key Questions
- All Tier 0-1 rules
- State machine, gate semantics, security protocol

**Cross-reference updates:** MULTI_AGENT_MODE.md, CONTRACT_FAILURE_MODE_MAP.md, contracts README.

### Rationale

CORE.md is load-bearing — it has system-prompt authority that mode contracts lack. The compression targets content that is either pairing-specific (Struggle Protocol template, Contrarian framing) or duplicated (Runtime Kernel appendix restated Tier 0 rules already present). Universal principles (professional judgment, mechanical triggers) stay in CORE; mode-specific expression of those principles moves to the mode contract.

Option 3 (role-specific CORE splits) was rejected because dispatching content across both modes *and* agent roles would create a maintenance matrix that's harder to reason about than the context cost it saves.

### Consequences

**Positive:**
- 69 fewer lines loaded by every MAM agent (~9% reduction)
- Pairing-specific instructions no longer clutter MAM agent context
- Duplication eliminated (Runtime Kernel appendix)

**Limitations accepted:**
- Pairing-specific instructions (Struggle Protocol, Contrarian framing) lose system-prompt-level authority — they're now in PAIRING_MODE.md
- Further compression is constrained: most remaining CORE content is universal and load-bearing

---
*Reconstructed from commit 3553ffb (2026-02-27)*
