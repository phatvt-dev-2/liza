# Architecture Plan: <Short Title>

Status: draft/review/approved

## Goal

One sentence. The structural vision for implementing the goal spec.

## Context

How this change fits in the broader system. What exists today. Why structural decisions are
needed beyond what the spec prescribes.

### References
- Goal spec: <path>
- Parent tasks: <task IDs of upstream deliverables>
- Codebase: <relevant files/packages explored>

### Constraints
Existing architectural constraints that bound the design: invariants, conventions, integration
points that cannot change.

### Assumptions
- **ASM-001**: <architectural assumption> — *Why*: <reasoning> — Confidence: HIGH | MEDIUM | LOW

### Open Questions
- **OQ-001**: <structural question> — *Impact*: <what stays ambiguous for code-planners>

---

## Components

### <Name> (`<path/>`)

**Responsibility:** One sentence — what this component owns.

**Boundaries:**
- Exposes: <what other components can access>
- Depends on: <what this component requires>

**Key decisions:**
- <decision>: <rationale>

---

## Interfaces

### <Component A> → <Component B>

**Contract:** What crosses the boundary, in what form.
**Direction:** Who calls whom; data flow direction.
**Invariants:** What must always be true.

---

## Data Flow

```
Input → Component A → [interface] → Component B → Output
```

Annotate transformations at each stage.

---

## Cross-Cutting Concerns

| Concern | Approach |
|---------|----------|
| Error handling | <how errors propagate across components> |
| Observability | <logging, metrics, tracing> |
| Configuration | <what's configurable, where it lives> |
| Testing | <integration boundaries, what's mockable> |

---

## Decomposition

Each scope becomes a code-planning child task.

### Scope 1: <title>

**Component(s):** <which components>
**Boundary:** <in scope / out of scope>
**Done when:** <falsifiable criterion>
**Depends on:** <scope numbers, if any>

### Scope 2: <title>
...

### Spec Coverage

| Spec Requirement | Scope |
|------------------|-------|
| <FR/feature from goal spec> | Scope N |
| ... | ... |

Every requirement in the goal spec must map to at least one scope. Unmapped requirements are gaps.
