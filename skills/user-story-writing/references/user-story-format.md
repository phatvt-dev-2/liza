# User Stories: <Short Descriptive Title>

Status: draft/review/approved/superseded

## Goal
One sentence. What this set of stories achieves when implemented. Measurable.

## Parent Epic
<path to epic document — capability CAP-NNN, or "none" if written without an epic>

## Context
Why this matters. How it fits in the broader system. Dependencies on other story documents or existing components. Keep it brief — the Coder needs orientation, not a lecture.

## Personas
- **<Persona name>**: <one-line description of who they are and what they care about>
- ...

## General information

Applies to: the entire scope (all stories).

### References
- <ref-type>: <path or link> — <section/line range if applicable>
- ...

### Non-Functional Requirements
- NFR-000-1: <requirement> — architectural constraints, performance, security, observability, technology mandates, compatibility requirements, etc.
- ...

### Related External Components
Summary of all the external components referenced by this document:
- Component C-002 - <component name>

### Interfaces *(include only when this document defines component boundaries)*

Summary of all the external interfaces referenced by this document:
- I-002-001 - <interface name> (Interface 001 of Component C-002): <protocol/contract description>
- ...

### Out of Scope
Explicit list of what this document does NOT cover. Adjacent concerns the Coder must not drift into.

### Assumptions
Items where the source material was ambiguous and you made a judgment call. Each assumption is:
- **ASM-000-1**: <what you assumed> — *Why*: <reasoning> — ⚠️ Confidence: HIGH | MEDIUM | LOW
- ...

LOW confidence assumptions are blocking: the human must resolve them before stories move to coding.

### Open Questions
Questions you cannot resolve by assumption. These MUST be answered by a human before stories are coded.
- **OQ-000-1**: <question> — *Impact if unresolved*: <what breaks or stays ambiguous>
- ...

---

## Story ST-001 - <story name>

### References
- <ref-type>: <path or link> — <section/line range if applicable>
- ...

### User Story
**As a** <persona>, **I want to** <action>, **so that** <outcome/value>.

### Acceptance Criteria
- AC-001-1: Given <context>, when <action>, then <outcome>
- AC-001-1b: edge case of AC-001-1
- AC-001-2: ...

### Depends on:
Run time coupling:
- Interface I-002-001 - <interface name>
- ...

### Out of Scope
Explicit list of what this story does NOT cover. Adjacent concerns the Coder must not drift into.

### Assumptions
Items where the source material was ambiguous and you made a judgment call. Each assumption is:
- **ASM-001-1**: <what you assumed> — *Why*: <reasoning> — ⚠️ Confidence: HIGH | MEDIUM | LOW

LOW confidence assumptions are blocking: the human must resolve them before this story moves to coding.

### Open Questions
Questions you cannot resolve by assumption. These MUST be answered by a human before this story is coded.
- **OQ-001-1**: <question> — *Impact if unresolved*: <what breaks or stays ambiguous>

---

## Story ST-002 - <story name>
...

### Depends on:
Implementation ordering:
- Story ST-001 - <story name>
...
