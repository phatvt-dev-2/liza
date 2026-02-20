# Spec Templates

Consult during spec generation (step 5). Use the template matching the spec type being created.

## Spec Location

```
specs/
  build/
    0.md           # Vision
    1.1.md         # Epic
    1.1.1.md       # User Story
    ...
    changelog.md
  functional/
    0.md           # Product description
    1.1.md         # Domain
    1.1.1.md       # Feature
    ...
    changelog.md
  spec-mapping.yaml         # Code ↔ spec mapping (permanent)
  spec-backfill-state.yaml  # Session state (temporary)
  _archive/                 # Superseded specs (preserve history)
```

One document per node. One changelog per hierarchy.

## Changelog Format

```markdown
# Changelog

## [YYYY-MM-DD]
- Added: Feature 1.2.3 (Email notifications)
- Modified: Feature 1.1.1 (Login) — added MFA requirement
- Deprecated: Feature 1.3.1 (Legacy export)

## [YYYY-MM-DD]
- ...
```

## Build Hierarchy

### Vision (build/0.md)

```markdown
# Vision

## Purpose

[Why this product exists]

## Target Users

[Who this is for]

## Core Value Proposition

[What problem it solves]

## Success Metrics

[How we measure success]

---
*Status: active*
*Last verified: YYYY-MM-DD*
```

### Epic (build/1.x.md)

```markdown
# [Epic Name]

## Goal

[What user-facing outcome this achieves]

## User Stories

- 1.x.1 — [Story title]
- 1.x.2 — [Story title]

## Success Criteria

[How we know this epic is complete]

## Related ADRs

- ADR-NNNN — [Decision title]

---
*Status: active*
*Last verified: YYYY-MM-DD*
```

### User Story (build/1.x.x.md)

```markdown
# [Story Title]

## User Story

As a [role], I want [capability] so that [benefit].

## Acceptance Criteria

- [ ] Criterion 1
- [ ] Criterion 2

## Implementation

- `src/path/to/module.py` — [what it does]

---
*Status: active*
*Last verified: YYYY-MM-DD*
*Parent epic: 1.x*
```

## Functional Hierarchy

### Product Description (functional/0.md)

```markdown
# [Product Name]

## Overview

[What the product is and does]

## Domains

- 1.1 — [Domain name]
- 1.2 — [Domain name]

## Key Integrations

[External systems this connects to]

---
*Status: active*
*Last verified: YYYY-MM-DD*
```

### Domain (functional/1.x.md)

```markdown
# [Domain Name]

## Description

[What this bounded context covers]

## Features

- 1.x.1 — [Feature name]
- 1.x.2 — [Feature name]

## Boundaries

[What's in scope vs. out of scope]

---
*Status: active*
*Last verified: YYYY-MM-DD*
```

### Feature (functional/1.x.x.md)

```markdown
# [Feature Name]

## Status

- Status: active
- Last verified: YYYY-MM-DD
- Linked ADRs: ADR-NNNN

## Description

[What this feature does for users]

## Behavior

[Observable behavior, rules, constraints]

## Code References

- `src/path/to/module.py` — [what it implements]

---
*Parent domain: 1.x*
```
