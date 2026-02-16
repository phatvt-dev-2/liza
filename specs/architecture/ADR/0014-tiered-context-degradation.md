# 14 - Tiered Context Degradation Protocol

## Context and Problem Statement

Agents now support graceful context compaction, enabling longer sessions. But the contract's context management was binary: full initialization or kernel fallback. When context pressure hit mid-session, the agent either kept everything (and degraded silently) or jumped straight to the bare kernel (losing operational capability).

The contract didn't adapt to compaction. A graduated recovery mechanism was needed so agents could shed weight incrementally while maintaining the most critical operational knowledge.

## Considered Options

1. **Keep binary approach** — Full or Kernel, no middle ground
2. **Automatic context summarization** — Let the agent summarize and compress its own context
3. **Tiered re-reading protocol** — Define graduated tiers with explicit re-read lists

## Decision Outcome

Chose **Option 3**: Three-tier degradation with explicit transition protocol and mode-specific re-read lists.

### Architecture

```
Full Init ──(context pressure)──→ Working Set ──(continued degradation)──→ Kernel
    │                                   │                                      │
    │ Everything per Session Init       │ CORE + mode essentials +             │ Tier 0 + state
    │                                   │ active task context                  │ transitions + self-check
```

| Tier | Name | When | What's Active |
|------|------|------|---------------|
| Full | Full Init | Fresh session | Everything per Session Initialization |
| Working | Working Set | Context pressure detected | CORE (system prompt) + mode essentials + active task context |
| Kernel | Runtime Kernel | Severe degradation | Tier 0 + state transitions + self-check (appendix) |

**Working Set re-read list (universal):**
- Runtime Kernel (already in system prompt via appendix)
- Tier 1 rules summary (added to appendix for this purpose)
- Current task intent + validation plan (from own earlier output)
- Active skill's SKILL.md (if loaded)
- Mode-specific items (defined in each mode contract's Context Recovery section)

**Transition protocol:**
- First signal → Working Set + announce: `"⚠️ WORKING SET — Context pressure. Re-reading mode essentials. Tier 2-3 best-effort."`
- Continued degradation → Kernel + offer checkpoint/reset (Pairing) or auto-checkpoint and self-terminate (MAM)

### Rationale

Binary degradation wastes the middle ground. An agent at Working Set tier retains enough to operate competently — the mode contract essentials, the task context, the critical rules. The Kernel is reserved for severe degradation where only safety invariants matter.

Each mode contract owns its Context Recovery section because what's essential differs: Pairing needs approval format and gate semantics; MAM needs role definition and blackboard protocol.

### Consequences

**Positive:**
- Agents can sustain longer sessions with graceful degradation instead of cliff-edge failure
- Degradation is explicit and announced — no silent quality loss
- Mode contracts each define their own recovery priorities

**Limitations accepted:**
- First attempt — thresholds for tier transitions are heuristic (agent self-assessment), not measured
- Each mode contract gains a Context Recovery section (more contract weight, but only one is loaded at a time)

---
*Reconstructed from commit e7fecbd (2026-02-15)*
