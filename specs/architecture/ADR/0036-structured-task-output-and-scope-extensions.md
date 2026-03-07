# 36 - Structured Task Output and Scope Extensions

## Context and Problem Statement

The declarative sub-pipeline architecture (ADR-0035) introduced `per-subtask` cardinality for transitions — one child task per planned item. But there was no mechanism for a planning agent to persist structured task definitions that downstream transitions could consume.

Separately, coders encountered rejection loops when they legitimately needed to modify files outside their declared scope. In real-world development, no plan is 100% accurate — scope negotiation between implementer and reviewer is normal. The system needed a way to formalize this.

## Considered Options

### Task Output
1. **Free-form task descriptions** — the orchestrator creates tasks with prose descriptions. Loses structure; downstream roles can't validate completeness.
2. **Structured `output[]` on tasks** — planning agents persist typed entries that transitions read to create child tasks.

### Scope Extensions
1. **Reject and re-plan** — any out-of-scope file modification triggers rejection. Too rigid; blocks legitimate work.
2. **Scope extensions in checkpoint** — coder declares out-of-scope files with justification; reviewer evaluates.

## Decision Outcome

Chose structured mechanisms for both: `output[]` for inter-pipeline data flow, `scope_extensions` for scope negotiation.

### Architecture

**Task Output:**
```go
type OutputEntry struct {
    Desc     string `yaml:"desc"`
    DoneWhen string `yaml:"done_when"`
    Scope    string `yaml:"scope"`
    SpecRef  string `yaml:"spec_ref"`  // optional
}
```

- Stored directly on `Task.Output []OutputEntry`
- Written via `liza_set_task_output` MCP tool
- Validated: task must be in executing status; agent must be assigned; all required fields non-empty
- Overwrites atomically (idempotent)
- Consumed by `liza proceed` with `per-subtask` cardinality — one child task per output entry

**Scope Extensions:**
```go
type ScopeExtensionEntry struct {
    File          string
    Justification string
}
```

- Added to `WriteCheckpointInput.ScopeExtensions`
- Serialized into checkpoint history entry's `Extra["scope_extensions"]`
- Rendered in reviewer template (`{{- if .ScopeExtensions}}` block)
- Reader helper `GetLatestScopeExtensions()` extracts from latest checkpoint

### Rationale

**Output[]** provides the generic data channel that makes `per-subtask` transitions work. Without it, the orchestrator would need to manually create and configure each child task — defeating the purpose of declarative transitions.

**Scope extensions** mirror how real teams work: a developer discovers a legitimate need to touch an adjacent file, documents why, and the reviewer evaluates. Blocking this entirely causes rejection loops; allowing it silently removes reviewer oversight. Explicit declaration with justification is the right middle ground.

### Consequences

**Positive:**
- Planning agents produce structured, validated task definitions
- Transitions consume output[] automatically — no manual orchestrator intervention
- Scope negotiation is explicit and auditable
- Reviewers have context to make informed decisions about out-of-scope changes

**Limitations accepted:**
- Output[] entries must match the expected schema — no extensibility beyond the four fields
- Scope extensions are advisory — reviewer can still reject

---
*Reconstructed from commits 45150db, 8e75dee, df10fe7 (2026-03-05 to 2026-03-06)*
