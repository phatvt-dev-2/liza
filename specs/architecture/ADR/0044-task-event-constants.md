# 44 - Task Event Constants

## Context and Problem Statement

ADR-0024 (Unified Role Constants) centralized role names into a single `internal/roles` package, replacing scattered string literals with typed constants. Task event names — the strings recorded in task history entries like `"submitted"`, `"verdict_approved"`, `"checkpoint_written"` — had the same problem: raw string literals scattered across `internal/ops`, `internal/commands`, and `internal/agent` packages with no single source of truth.

The same principle that motivated role constants applied here: string literals are fragile, not grep-able as a category, and silently diverge when copy-pasted across files.

## Considered Options

1. **Define `TaskEventName` type alias and constants in `internal/models/history.go`** — same package where `HistoryEntry` lives, same pattern as role constants.

No alternatives were considered. The approach mirrors ADR-0024 directly.

## Decision Outcome

Chose **Option 1**: introduce `TaskEventName` type alias with 26 constants covering all task lifecycle events.

### Architecture

**`internal/models/history.go`:**
- `TaskEventName` type alias (`string`)
- 15 initial constants (non-claim events): `TaskEventSubmitted`, `TaskEventVerdictApproved`, `TaskEventVerdictRejected`, `TaskEventCheckpointWritten`, `TaskEventHandedOff`, `TaskEventBlocked`, `TaskEventSuperseded`, `TaskEventMerged`, `TaskEventSprintCompleted`, `TaskEventSprintArchived`, `TaskEventResumedHandoff`, `TaskEventIntegrationFailed`, `TaskEventAddedToSprint`, `TaskEventRecoveredTask`, `TaskEventProceed`
- 11 remaining constants (claim-path events): `TaskEventInitialization`, `TaskEventCreated`, `TaskEventClaimed`, `TaskEventAbandoned`, `TaskEventClaimedForIntegrationFix`, `TaskEventClaimReleased`, `TaskEventReclaimedAfterRejection`, `TaskEventReassignedAfterRejection`, `TaskEventWorktreeRecovered`, `TaskEventDoerClaimReleased`, `TaskEventReviewClaimReleased`

**Rollout:** Two commits — first the non-claim events (higher risk of drift, more call sites), then the claim-path events. Table-driven test covers all 26 constants to guard against value drift.

### Rationale

Same principle as ADR-0024: constants make event names grep-able, compiler-adjacent (Go won't catch typos in string literals but IDE tooling will flag unused constants), and centrally enumerable. The `TaskEventName` type alias provides documentation intent without runtime overhead.

### Consequences

**Positive:**
- Single source of truth for all 26 task event names
- New events require adding one constant, not hunting for insertion points
- Table-driven test prevents value drift

**Limitations accepted:**
- Type alias (not distinct type) — Go won't enforce at compile time, but constants still provide grep-ability and IDE support

**Extends:** ADR-0024 (Unified Role Constants) — same pattern, different domain.

---
*Reconstructed from commits e7ba1fe..a815895 (2026-03-11)*
