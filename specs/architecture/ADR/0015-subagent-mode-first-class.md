# 15 - Subagent Mode as First-Class Contract Mode

## Context and Problem Statement

Subagent Mode was embedded as a section in PAIRING_MODE.md, but subagents aren't pairing-specific — any Task-spawned agent operates as a subagent regardless of parent mode. A Liza (MAS) agent spawning a subagent loaded the same behavioral contract, which was designed for a different context.

The full contract is overkill for subagents. They're typically read-only (research, analysis), don't interact with humans, and shouldn't perform tiered context recovery. Loading the full Pairing or MAS contract wastes context budget on irrelevant sections — greeting protocols, collaboration modes, approval semantics — that a research subagent will never use.

## Considered Options

1. **Keep as Pairing Mode subsection** — Minimal contract surface, but wrong home and wrong context cost
2. **Conditional sections in existing modes** — "If subagent, skip sections X, Y, Z"
3. **First-class contract mode** — Lightweight SUBAGENT_MODE.md with its own Mode Selection Gate entry

## Decision Outcome

Chose **Option 3**: Extract to SUBAGENT_MODE.md as the third contract mode, detected via `MODE: SUBAGENT` marker in the Task tool prompt.

With the dual-mode architecture established (ADR-0004) and tiered context degradation in place (ADR-0014), adding a third mode was a natural extension of existing patterns.

### Architecture

**Mode Selection Gate (updated):**

| Detection | Mode | Action |
|-----------|------|--------|
| First prompt contains "You are a Liza ... agent" | **Liza** | Read MULTI_AGENT_MODE.md |
| First prompt contains `MODE: SUBAGENT` | **Subagent** | Read SUBAGENT_MODE.md |
| Otherwise | **Pairing** (default) | Read PAIRING_MODE.md |

**What Subagent Mode strips:**
- No user interaction (caller is interface, not human)
- No greeting or mental model ceremony
- No approval gates (internal ceremony only — Intent Gate, DoR/DoD remain)
- No tiered context recovery (return partial results instead)
- No state-modifying actions

**What Subagent Mode keeps:**
- All Tier 0 invariants
- Security Protocol
- Scope discipline
- Uncertainty reporting
- Anti-deception rules

**Task Tool Rule** (added to CORE.md): All agents spawned via Task tool are subagents. Include `MODE: SUBAGENT` in every Task tool prompt.

### Rationale

Context usage is the primary concern. The specialization into distinct, complementary contracts loaded on demand means each agent type pays only for the contract surface it needs. Subagents are typically read-only research agents — the full approval ceremony, collaboration modes, and recovery protocols are context weight with no return.

The generic-subagent skill already defined subagent behavioral expectations. Extracting Subagent Mode aligns the contract architecture with the skill's implicit assumption that subagents operate differently.

### Consequences

**Positive:**
- Subagents load CORE + ~60 lines instead of CORE + ~300 (Pairing) or CORE + ~500 (MAM)
- Mode architecture now covers all three agent types in the system
- Context budget freed for actual research work
- Clean separation: subagent behavioral constraints live in the contract, delegation methodology lives in the skill

**Limitations accepted:**
- Three modes to maintain instead of two (but subagent mode is deliberately minimal)
- `MODE: SUBAGENT` marker must be included in every Task tool prompt — structural enforcement via convention, not tooling

---
*Reconstructed from commits c7c4554, 7c24e33, b8f7faf (2026-02-15)*
