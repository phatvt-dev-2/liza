# Cluster 0021 - Ops Service Layer for Mutations

## Commit Set
- `c7e98d7` — initial ops extraction for core mutation paths
- `bfe179d` — complete extraction for MCP-exposed mutations
- `e7d020d` — extract DeleteTask into ops pattern

## Intent Hypothesis
Separate mutation business logic from command presentation and establish `internal/ops` as the single mutation service layer for command, agent, and MCP callers.

## Architectural Signals
- New package boundary: `internal/ops`
- Command wrappers reduced to presentation responsibilities
- MCP mutation flow shifted to ops direct calls
- High test expansion in ops package with typed result structs

## User Context Captured
- Trigger: command-level coupling and MCP protocol safety
- Rationale: single implementation of mutations + testability
- Tradeoffs: additional abstraction layer to maintain

## Confidence
0.90 (high)
