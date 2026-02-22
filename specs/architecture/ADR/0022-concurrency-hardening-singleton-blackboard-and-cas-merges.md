# 22 - Concurrency Hardening: Singleton Blackboard and CAS Merges

## Context and Problem Statement

As parallel agent execution increased, two concurrency risks became material:

1. **In-process blackboard instance fragmentation.** Multiple independent `db.New()` calls across layers created separate `Blackboard` instances per path, which was acceptable while state remained filesystem-centric but risky if in-process state/caching responsibilities expanded.
2. **Concurrent merge races.** Parallel reviewer merges required a safe strategy for advancing the integration branch without relying on a mutable working tree or single-writer assumptions.

## Considered Options

1. **Keep independent instances and optimistic merge updates,** relying on existing locking and low contention assumptions.
2. **Add coarse global serialization** around merge and blackboard access.
3. **Harden concurrency with per-path singleton blackboard instances and CAS-based merge updates with retries.**

## Decision Outcome

Chose **Option 3**.

### Architecture

- Added `db.For(statePath)` process-level singleton constructor keyed by cleaned path.
- Replaced production `db.New()` callsites with `db.For()` to ensure shared instance behavior per process/path; retained `db.New()` for isolated test scenarios.
- Documented singleton instance model in `internal/db/doc.go`.
- Hardened merge path in `ops/wt_merge`:
  - working-tree-less merge flow (`merge-tree` + `commit-tree`)
  - `git update-ref` compare-and-swap semantics
  - explicit `RefConflictError` and bounded CAS retry loop (`maxMergeRetries`).
- Added deterministic CAS conflict/retry tests and updated user/spec docs for multi-agent parallel operation behavior.

### Rationale

Per-path singleton blackboard access removes a silent constraint that blackboard instances must remain effectively stateless beyond file I/O and transient cache behavior.

CAS-based ref updates provide an explicit correctness mechanism for concurrent merges: ref advancement succeeds only if the expected prior SHA still matches, otherwise the operation retries from fresh integration HEAD.

### Consequences

**Positive:**
- Stronger in-process coherence for blackboard access.
- Explicit conflict detection and safe retry behavior for parallel merges.
- Working-tree-less merge strategy enables concurrent reviewer merges without shared working tree contention.
- Clearer failure semantics for high-contention integration updates.

**Limitations accepted:**
- CAS retry loop is bounded; sustained high contention still fails fast after retry budget.
- A separate supervisor-level pending-merge retry loop remains in `internal/agent/supervisor.go`, in addition to CAS retries in `internal/ops/wt_merge.go`; consolidation is a potential cleanup follow-up.
- `db.For()` introduces global process-level instance lifecycle that tests must account for (via reset/isolated paths).

---
*Reconstructed from commits 9d1890c..17e94f7 (2026-02-21 to 2026-02-22)*
