# 41 - RoleStrategy Pattern Extraction

## Context and Problem Statement

`RunSupervisor` had become a god function. As Phase 2 roles (ADR-0038) were added, role-specific behavior accumulated as 9-way switch/if-else chains spread across `supervisor.go`, `prompt.go`, and `waitforwork.go`. Each new role required touching multiple switch statements in multiple files. With more roles planned, this was unsustainable.

The Strategy pattern fits naturally: each role category (doer, reviewer, orchestrator) has distinct behavior for work detection, prompt building, and result handling, but the supervisor loop itself is role-agnostic.

## Considered Options

1. **Keep switch chains with pipeline awareness** — maintain the existing structure, just add more cases.
2. **Extract RoleStrategy interface** — each role category implements its own behavior; supervisor delegates through the interface.

## Decision Outcome

Chose **Option 2**: extract a `RoleStrategy` interface with three category implementations.

### Architecture

**Interface:**
```go
type RoleStrategy interface {
    WaitForWork(ctx) (*models.Task, error)
    BuildContext(task) (string, error)
    HandleResult(task, result) error
}
```

**Implementations:**
- `strategy_doer.go` — doer category (coder, code-planner, us-writer, epic-planner)
- `strategy_reviewer.go` — reviewer category (code-reviewer, code-plan-reviewer, us-reviewer, epic-plan-reviewer)
- `strategy_orchestrator.go` — orchestrator

**Factory:** A single factory function maps role constants to strategy instances. Adding a new role within an existing category requires only a role constant, a context builder function, and a factory case.

**Supervisor loop:** `RunSupervisor` is now role-agnostic — it calls `strategy.WaitForWork()`, `strategy.BuildContext()`, and `strategy.HandleResult()` without knowing which role it's running.

### Rationale

The Strategy pattern provides both extensibility and readability. The supervisor loop becomes a simple orchestration of strategy methods, and each role category's behavior is self-contained in its own file. This is both easier to extend (new roles) and easier to understand (each file is focused).

### Consequences

**Positive:**
- Adding a new role within an existing category is trivial (constant + context builder + factory case)
- Supervisor loop is role-agnostic — no more switch chains
- Each role category's behavior is self-contained and testable in isolation
- 10 files changed, +929/-591 lines — more code but much better organized

**Limitations accepted:**
- None identified — the indirection is justified by the number of roles

**Depends on:** ADR-0038 (Phase 2 Roles) — the problem that motivated extraction.
**Enabled by:** ADR-0040 (Legacy Pipeline Removal) — clearing the legacy paths simplified the extraction.

---
*Reconstructed from commit ebc3f5b (2026-03-10)*
