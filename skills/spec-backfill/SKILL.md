---
name: spec-backfill
description: Backfill missing specifications, reconcile spec/code drift, maintain changelog.
---

# Objective

Reconstruct specifications from a repository's codebase, existing documentation, and git history.
You're doing archaeology — finding the *features* buried in code and docs, then surfacing them as structured specs.

A spec is warranted when code implements user-facing functionality or business logic. Not every module needs a spec.
Your job is to find the functional boundaries and document them.

# Process

1. **Gather** — Read existing specs, docs, code, git history, and mapping state
2. **Classify code** — Distinguish functional code (needs specs) from architectural (needs ADRs) from utility (needs neither)
3. **Map code to specs** — Identify existing coverage, gaps, and conflicts
4. **Ask** — Surface unknowns, request clarification. No guessing.
5. **Generate** — Produce/amend specs only after confirmation
6. **Verify** — Ensure no relevant legacy content dropped
7. **Archive** — Move superseded specs to `_archive` after confirmation
8. **Persist** — Update mapping and changelog

Maintain state in files so work isn't lost if the conversation ends.

# 1. Code Classification

## Functional (needs spec coverage)

- User-facing features and workflows
- Business logic and domain rules
- API endpoints that serve external consumers
- Data transformations with business meaning
- Integration points with business significance

## Architectural (needs ADR coverage, separate skill)

- Infrastructure decisions (deployment, CI/CD)
- Technology choices (frameworks, libraries)
- Cross-cutting concerns (auth, logging, caching)
- System boundaries and module structure

## Utility (needs neither)

- Helper functions and utilities
- Generated code
- Test infrastructure
- Dev tooling
- Pure technical plumbing

## Judgment calls

When uncertain, ask: "If a product manager asked 'what does this do for users?', would there be a meaningful answer?" If yes, it's functional.

# 2. Hierarchy Relationship

## Build hierarchy (source of truth, time-oriented)

```
specs/build/
  0.md           # Vision — why the product exists
  1.1.md         # Epic — large user-facing goal
  1.1.1.md       # User Story — specific user need
  changelog.md
```

Build captures *intent* — what we set out to do and when.

## Functional hierarchy (derived view, navigation-oriented)

```
specs/functional/
  0.md           # Product description — what it is today
  1.1.md         # Domain — bounded context
  1.1.1.md       # Feature — specific capability
  changelog.md
```

Functional captures *current state* — what exists now and how it's organized.

## Relationship

- Build **generates** Functional: epics/stories become domains/features when implemented
- Functional gaps **trigger** build investigation: missing feature → check epic history
- During backfill: **populate build first**, derive functional from it
- Both can exist independently, but functional without build loses historical context

## Numbering convention

Hierarchical numbering reflects parent-child relationships:
- `1.1.md` = first child of root
- `1.1.1.md` = first child of 1.1
- `1.2.md` = second child of root (sibling of 1.1)

# 3. Coverage Analysis

## Mapping code to specs

For each functional code path, determine:
- Does a spec exist? → Check alignment
- No spec exists? → Flag as gap
- Spec exists but conflicts with code? → Flag as conflict

## Coverage report structure

```
Coverage Report
===============
Covered:     src/billing/invoice.py → specs/functional/1.2.1.md (Feature: Invoicing)
Gap:         src/notifications/email.py → No spec (appears to be email notifications)
Conflict:    src/auth/login.py → specs/functional/1.1.1.md (code has MFA, spec doesn't mention it)
```

## Conflict types

- **Spec ahead of code**: Spec describes feature not yet implemented
- **Code ahead of spec**: Code has functionality not in spec
- **Semantic mismatch**: Both exist but describe different behavior
- **Stale spec**: Spec describes removed functionality

# 4. Gates (when to stop and ask)

**Code classification ambiguity**
> "I'm unsure whether `src/lib/pricing_engine.py` is functional (business logic) or utility (calculation helper).
> It computes discounts based on customer tier. How do you see it?"

**Spec mapping unclear**
> "Code in `src/orders/fulfillment.py` handles shipping, returns, and tracking. Should this map to one feature or three?"

**Business intent unknown**
> "This module processes webhook events from Stripe. I can see *what* it does but not *why* — what business need does this serve?"

**Conflict resolution**
> "Spec says users can have max 5 projects. Code enforces max 10. Which is correct — should I update the spec or flag this as a bug?"

**Gap prioritization**
> "I found 12 code paths without specs. Should I generate specs for all, or which are highest priority?"

Always ask before generating a spec. Present your analysis, let the user confirm or correct.

# 5. Spec Generation

## From code (gap filling)

When generating a spec for undocumented code:

1. Extract observable behavior from code
2. Identify inputs, outputs, side effects
3. Note business rules and constraints
4. **Ask user** for: purpose, user story, acceptance criteria
5. Generate spec only after confirmation

## From existing docs (migration)

When migrating legacy docs to spec structure:

1. Identify content that maps to features
2. Preserve all relevant information
3. **Ask user** to confirm nothing important is dropped
4. Archive original in `specs/_archive`

# 6. Changelog Reconstruction

## From git history

Parse git history of spec documents to rebuild changelog:

1. Identify creation dates (first commit)
2. Track significant modifications (not typo fixes)
3. Note status transitions (draft → active, active → deprecated)
4. **Ask for clarification** on ambiguous commits

## Changelog format

```markdown
# Changelog

## [YYYY-MM-DD]
- Added: Feature 1.2.3 (Email notifications)
- Modified: Feature 1.1.1 (Login) — added MFA requirement
- Deprecated: Feature 1.3.1 (Legacy export)

## [YYYY-MM-DD]
- ...
```

## Ambiguous changes

When commit message doesn't clarify intent:
> "Commit abc123 modified specs/functional/1.2.1.md significantly but message just says 'updates'. What changed and why?"

# 7. Verification

Before finalizing any run:

1. **Coverage check** — All functional code has spec mapping
2. **Content preservation** — No relevant legacy content dropped
3. **Consistency check** — No internal contradictions in specs
4. **Link integrity** — All cross-references (ADRs, code paths) valid

## Legacy content handling

When archiving old specs:
1. Diff old vs. new content
2. Surface any content in old not in new
3. **Ask user**: "This content exists in legacy but not in new spec: [content]. Should it be added, or is it obsolete?"
4. Only archive after confirmation

# 8. Location

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

---

# Quality Bar

A good backfilled spec:
- Could have been written before the code was built
- Describes *what* and *for whom*, not implementation details
- Stands alone (reader doesn't need to read the code)
- Is honest about what's confirmed vs. inferred
- Links to relevant ADRs for *why* decisions were made

A bad backfilled spec:
- Just describes the code structure
- Invents requirements the user didn't confirm
- Mixes multiple features in one document
- Contains implementation details instead of behavior
- Conflicts with the actual code

# Anti-patterns to Avoid

- **Inventing requirements**: If you don't know the business need, ask. Don't fabricate.
- **Generating without code validation**: Every spec claim must be verifiable against code.
- **Guessing business intent**: Code shows *what*, not *why for users*. Ask.
- **Dropping legacy content silently**: Always surface what's being removed for confirmation.
- **Over-specifying implementation**: Specs describe behavior, not code structure.
- **Ignoring conflicts**: Surface mismatches explicitly, don't paper over them.

---

# Getting Started

When the user invokes this skill:

1. Check for existing state (`specs/spec-backfill-state.yaml`), offer to resume
2. **Detect staleness** — compare stored SHAs in `spec-mapping.yaml` against current HEAD:
   - `code_sha` stale, `spec_sha` current → code-ahead-of-spec (new behavior undocumented)
   - `code_sha` current, `spec_sha` stale → spec edited (intentional or drift?)
   - Both stale → full reconciliation needed
   - New code files (not in mapping) → flag as candidates for classification
   - Deleted code files → flag for mapping cleanup
   - If nothing stale/new → report "all current" and stop
3. Surface stale mappings and new files to user
4. If starting fresh or extending, begin with code classification
5. Show the user your classification for validation
6. Map existing specs to code, surface gaps and conflicts
7. **Ask** which gaps to prioritize
8. Generate specs one at a time, confirming each before proceeding
9. Verify no legacy content lost before archiving
10. Update mapping (including SHAs), changelog, and output report per `references/report-format.md`

---

# Reference

## Mapping schema (permanent)

```yaml
# specs/spec-mapping.yaml
version: 1
repository: "git@github.com:org/repo.git"
last_updated: "2024-01-15T14:22:00Z"

code_classification:
  # path → {type: functional | architectural | utility, confidence: confirmed | inferred}
  "src/billing/invoice.py": {type: functional, confidence: confirmed}
  "src/lib/kafka_client.py": {type: architectural, confidence: confirmed}
  "src/utils/formatting.py": {type: utility, confidence: inferred}

mappings:
  # code_path → spec mapping
  # Staleness: compare code_sha and spec_sha against current HEAD
  #   code_sha stale, spec_sha current → code-ahead-of-spec
  #   code_sha current, spec_sha stale → spec edited (intentional or drift?)
  #   both stale → full reconciliation needed
  "src/billing/invoice.py":
    spec_path: "specs/functional/1.2.1.md"
    spec_title: "Invoicing"
    code_sha: a1b2c3d
    spec_sha: x9y8z7w
    status: aligned  # aligned | gap | conflict (stale is computed from SHA comparison, not stored)
    last_verified: "2024-01-15"
    confidence: confirmed  # confirmed | inferred-pending-review
    conflict_details: null
  "src/notifications/email.py":
    spec_path: null
    spec_title: null
    code_sha: d4e5f6a
    spec_sha: null
    status: gap
    last_verified: "2024-01-15"
    confidence: inferred-pending-review
    inferred_feature: "Email notifications"
  "src/auth/login.py":
    spec_path: "specs/functional/1.1.1.md"
    spec_title: "User Login"
    code_sha: b7c8d9e
    spec_sha: f3g4h5i
    status: conflict
    last_verified: "2024-01-15"
    confidence: confirmed
    conflict_details: "Code has MFA, spec doesn't mention it"

specs:
  # spec_path → metadata
  "specs/functional/1.2.1.md":
    title: "Invoicing"
    status: active  # draft | active | deprecated | removed
    last_verified: "2024-01-15"
    linked_adrs: ["ADR-0012", "ADR-0015"]
    code_paths: ["src/billing/invoice.py", "src/billing/line_items.py"]
  "specs/build/1.2.md":
    title: "Billing Epic"
    status: active
    last_verified: "2024-01-15"
    linked_adrs: ["ADR-0012"]
    child_specs: ["specs/build/1.2.1.md", "specs/build/1.2.2.md"]

archive:
  # archived_path → metadata
  "specs/_archive/old-billing-spec.md":
    original_path: "docs/billing.md"
    archived_at: "2024-01-15"
    reason: "Migrated to specs/functional/1.2.x"
    content_verified: true
```

## Session state schema (temporary)

```yaml
# specs/spec-backfill-state.yaml
version: 1
started_at: "2024-01-15T10:30:00Z"
last_updated: "2024-01-15T14:22:00Z"

processing_cursor:
  step: "mapping"  # classification | mapping | gap_analysis | generation | verification | archiving
  current_path: "src/notifications/email.py"
  substep: "awaiting_user_input"

configuration:
  functional_roots: ["src/"]
  exclude_patterns: ["**/tests/**", "**/migrations/**"]
  spec_output_dir: "specs"
  require_confirmation: true
```

## Spec templates

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
