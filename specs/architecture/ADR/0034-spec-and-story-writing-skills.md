# 34 - Spec-Writing and User-Story-Writing Skills

## Context and Problem Statement

Phase 2 pipeline roles (ADR-0038) need structured methodologies for producing specifications and user stories. These methodologies — formats, quality criteria, anti-patterns — are domain know-how that should be reusable across contexts, not embedded in prompt templates.

The skills were created as a preparatory step before introducing the roles themselves, establishing the artifact layer that planning agents would consume.

## Considered Options

1. **Embed methodology in prompt templates** — simpler, but mixes instructions (what to do) with know-how (how to do it); harder to maintain and reuse.
2. **Separate skills** — follows the established skills architecture (ADR-0002): skills = know-how, prompts = instructions. Allows humans to use it too.

## Decision Outcome

Chose **Option 2**: create two new skills following the existing skills-as-lean-prompts architecture.

### Architecture

**`detailed-spec-writing/`:**
- SMARC methodology (Specific, Measurable, Achievable, Relevant, Context-bound)
- PRD format template with traceability IDs: FR (functional req), NFR (non-functional), AC (acceptance criteria), ASM (assumption), OQ (open question)
- Dependency tracking distinguishing runtime coupling from implementation ordering
- Output: git-tracked markdown document

**`user-story-writing/`:**
- SMARC criteria applied to user stories
- Canonical story form with persona, action, value
- Two-tier scope discipline: upfront reads task references and same-domain stories; broader material only on reviewer feedback
- Anti-pattern catalog
- Output: structured story document mapping 1:1 to coder-sized tasks

**Consumption:**
- US Writer role loads `user-story-writing` skill
- Spec-phase agents load `detailed-spec-writing` skill
- Skills are loose-coupled by design: agents load them when needed, keeping base context lean

### Rationale

Skills and prompts serve different purposes. Skills encode reusable domain expertise that persists across roles and contexts. Prompts encode role-specific instructions for a particular pipeline stage. This separation makes both independently maintainable and testable.

### Consequences

**Positive:**
- Consistent methodology across all spec/story-producing roles
- Reusable in pairing mode (human invokes skill directly)
- Independent evolution — skills can be improved without touching prompt templates

**Limitations accepted:**
- Loose coupling means agents must explicitly load the skill — not guaranteed by code

---
*Reconstructed from commits f73b8a6, 816e8f3 (2026-03-02 to 2026-03-03)*
