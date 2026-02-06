# 2 - Skills as Lean Agent Prompts

## Context and Problem Statement

Agent prompts can become unwieldy. Every capability, every domain-specific procedure, every anti-pattern to avoid — all crammed into a single system prompt. The prompt grows, context windows fill, and agents struggle to find relevant guidance.

How should domain-specific methodologies be organized?

## Considered Options

1. **Monolithic prompts** — Everything in CLAUDE.md or role-specific prompts
2. **Inline documentation** — Agents read docs/specs as needed
3. **Skills as pluggable modules** — Separate files loaded on demand

## Decision Outcome

Chose **Option 3**: Skills are separate markdown files that agents load when relevant.

### Rationale

**Keep agent prompts lean.** The core contract (CORE.md + mode) defines *how* agents make decisions. Skills define *what* to do in specific domains. Separating them keeps both manageable.

**Support reusability.** A debugging skill applies whether you're in pairing mode or MAS, whether you're using Claude or Codex. Skills are methodology, not agent-specific.

**Enable specialization.** Some skills are explicitly used by Liza agents:
- `code-review` — Reviewer role
- `clean-code` — Coder cleanup tasks
- `systemic-thinking` — Architecture analysis

Other skills support pairing workflows but aren't Liza-specific.

**Marketplace potential.** Skills are self-contained. A future decision may extract them to a dedicated repository for broader sharing (see future ADR on marketplace extraction).

### Architecture

```
~/.liza/skills/
├── debugging/SKILL.md        # Narrowing search, not guess-and-check
├── testing/SKILL.md          # Tests encode intent; assume test is correct
├── code-review/SKILL.md      # Risk mitigation, not gatekeeping
├── clean-code/SKILL.md    # Reduce complexity without changing behavior
├── systemic-thinking/SKILL.md # Where does this break under pressure?
├── software-architecture-review/SKILL.md
├── spec-review/SKILL.md
├── feynman/SKILL.md          # Explain to understand
└── generic-subagent/SKILL.md # Subagent coordination patterns
```

**Skill structure:**
```markdown
---
name: debugging
description: Debugging Protocol
---

[Philosophy and principles]

# Triggers
When to activate this skill

# Process
Step-by-step methodology

# Anti-patterns
What NOT to do

# Escalation
When to stop and ask for help
```

**Loading mechanism:**
- Contract references skills: "MANDATORY: Before debugging, read `~/.liza/skills/debugging/SKILL.md`"
- Agents read skill file when entering relevant activity
- Skills operate within contract constraints (gates still apply)

### Consequences

**Positive:**
- Agent prompts stay focused on decision-making
- Skills can be updated independently
- Same skill works across modes and providers
- Clear separation: contract = guardrails, skills = methodology

**Limitations accepted:**
- Agents must read additional files (context cost)
- No runtime skill selection — contract/agent prompts hardcode which skills to load when

---
*Reconstructed from commit f02ac4a (2026-01-17)*
