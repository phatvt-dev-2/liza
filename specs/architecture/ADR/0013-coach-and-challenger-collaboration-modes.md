# 13 - Coach and Challenger Collaboration Modes

## Context and Problem Statement

Agents are trained to be agreeable. Two critical collaborative postures — questioning *why* and pushing back on weak plans — require active prompting to overcome this default. Without dedicated modes, these postures are diluted across general collaboration, making them easy to skip.

The Intent Gate (Rule 2, DoR) catches agents rushing to execution without clear intent. But there was no symmetric mechanism catching humans rushing to specification without clear purpose. Similarly, Rule 13 (Constructive Contrarian) asks agents to challenge assumptions, but spreading this across all modes dilutes its force — agents default to being helpful rather than adversarial.

## Considered Options

1. **Rely on existing modes** — True Pairing and Rule 13 cover these needs implicitly
2. **More systematic prompting within existing modes** — Strengthen the contrarian triggers in Rule 13
3. **Dedicated collaboration modes** — First-class modes with explicit activation/exit criteria

## Decision Outcome

Chose **Option 3**: Add Coach and Challenger as distinct collaboration modes in the Pairing Mode contract.

### Architecture

**Coach Mode** — Socratic, not adversarial. The agent questions purpose, not implementation.

| Aspect | Design |
|--------|--------|
| Symmetric to | Intent Gate (Rule 2, DoR) |
| Activation | Agent-proposed when WHAT is clear but WHY is not |
| Agent behavior | Ask questions only — never propose solutions |
| Exit | When human can state intent unambiguously |

The Intent Gate is only meaningful when the intent has a purpose. Coach mode is the mechanism that achieve this.

**Challenger Mode** — Adversarial complement to Coach. Stress-tests a finalized plan.

| Aspect | Design |
|--------|--------|
| Concentrates | Rule 13 (Constructive Contrarian) into a dedicated mode |
| Activation | Human-initiated, or agent-proposed at the execution gate |
| Questions | "What's the strongest argument against this? What evidence would change your mind?" |
| Exit | Plan defended or revised to address weaknesses found |

### Rationale

Existing modes couldn't produce these postures reliably. Agents need explicit permission and structure to ask WHY questions and to push back on ideas. A named mode with clear activation criteria makes the posture systematic rather than occasional.

Coach and Challenger are complementary: Coach operates pre-specification (clarify purpose), Challenger operates post-specification (stress-test plan). Together they bracket the planning phase.

### Consequences

**Positive:**
- Agents have explicit license to question direction, not just implementation
- The Intent Gate gains a feeder mechanism — Coach produces the clear intent that makes the gate powerful
- Rule 13 gets concentrated force when it matters most (pre-execution)

**Limitations accepted:**
- Two more modes in an already-rich collaboration table
- Agents may still default to helpful over adversarial despite the mode — the mode is necessary but may not be sufficient

---
*Reconstructed from commit 6573b99 (2026-02-14)*
