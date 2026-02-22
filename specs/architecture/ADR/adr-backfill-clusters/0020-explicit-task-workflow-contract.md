# Cluster 0020 - Explicit Task Workflow Contract

## Commit Set
- `ec635c5` — task type to role workflow mapping
- `2b5d236` — explicit validated task transition graph
- `7fc449d` — runtime iteration/review limit enforcement to BLOCKED

## Intent Hypothesis
Establish an explicit, role-aware, and enforceable workflow contract for tasks, replacing distributed lifecycle rules and advisory-only loop limits.

## Architectural Signals
- New model primitive: `TaskType` + workflow registry
- Explicit transition map + validated transition API (`CanTransition` / `Transition`)
- Enforcement logic for exhaustion-induced `BLOCKED` transition
- Spec updates in blackboard schema, state machine, and task lifecycle protocol

## User Context Captured
- Trigger: role expansion and lifecycle consistency pressure
- Rationale: structural enforcement over implicit behavior
- Tradeoffs: partial abstraction remains around role→claimable-status mapping

## Confidence
0.90 (high)
