# 21 - Ops Service Layer for State Mutations

## Context and Problem Statement

Mutation logic lived primarily in CLI command handlers, mixing business rules, state mutation, and presentation. This coupling created duplication across command, agent, and MCP paths and increased risk for protocol issues in MCP contexts where stdout/stderr contamination can break JSON-RPC streams.

The architecture review also identified large command surfaces (notably `delete_task`) where validation, mutation, and interactive behavior were interleaved, making testing and concurrency reasoning harder.

## Considered Options

1. **Keep command-centric mutation logic** and incrementally refactor hotspots only.
2. **Extract a subset of mutations** to shared helpers, leaving many state changes command-bound.
3. **Move mutation business logic into a dedicated ops layer** used by commands, MCP handlers, and supervisors.

## Decision Outcome

Chose **Option 3**.

### Architecture

- Introduced `internal/ops` as business-logic service layer with typed results and no terminal I/O.
- Converted command handlers into thin presentation wrappers over ops calls.
- Directed MCP mutation handlers to call ops directly; retained command path mainly for query/presentation flows.
- Completed extraction for MCP-exposed mutations and follow-up extraction for `DeleteTask`:
  - pre-check + mutation split
  - lock-safe revalidation in mutation path
  - dependency/worktree safeguards moved into ops logic.
- Added/expanded operation-level tests in `internal/ops/*_test.go` and updated architecture issue tracking to reflect resolved coupling concerns.

### Rationale

Separating presentation from mutation logic gives one canonical implementation for state changes, reduces divergence between entry points, and makes behavior testable without terminal interaction.

This also removes an integration hazard: MCP mutation execution no longer depends on command wrappers that might emit user-facing text in contexts expecting strict machine-readable responses.

### Consequences

**Positive:**
- Shared, testable mutation semantics across CLI, agent, and MCP surfaces.
- Lower risk of JSON-RPC stream corruption from mutation paths.
- Smaller, clearer command wrappers focused on UX concerns.
- Better concurrency correctness in mutation code through centralized lock/revalidation patterns.

**Limitations accepted:**
- More files and abstraction boundaries to maintain (`commands` + `ops`).
- Requires discipline to keep new mutation features in ops first, not command-first.

---
*Reconstructed from commits c7e98d7..e7d020d (2026-02-20 to 2026-02-21)*
