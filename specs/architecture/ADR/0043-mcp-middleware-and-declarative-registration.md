# 43 - MCP Middleware and Declarative Tool Registration

## Context and Problem Statement

A code quality assessment identified two structural issues in the MCP server layer:

1. **Zero observability**: MCP handlers had no logging or timing — tool calls were invisible to operators.
2. **Inline role validation**: 15 handlers repeated the same `agent_id` extraction and role-checking boilerplate, violating DRY and increasing the risk of inconsistent enforcement (as demonstrated by the gaps that led to ADR-0039).

Additionally, tool registration was procedural — each tool required a full `registerTool` call with inline schema and handler wiring, making the registration file long and ceremonial.

## Considered Options

1. **Middleware chain + declarative registration metadata** — extract cross-cutting concerns into composable middleware functions; define mutation tools as data (`toolDef` structs) registered via a shared loop.

No alternatives were considered. The middleware pattern is the standard solution for cross-cutting handler concerns, and declarative registration is the natural next step once middleware makes handler wiring uniform.

## Decision Outcome

Chose **Option 1**: middleware chain + declarative `toolDef` metadata.

### Architecture

**Middleware chain** (applied at `registerTool` time):

```
withLogging → withRole → handler
```

- `withLogging`: automatic on all handlers via `registerTool`. Logs tool name + duration to stderr via `log/slog` (stdout is reserved for JSON-RPC transport).
- `withRole`: explicit per-tool via `toolDef.roleChecker`. Extracts `agent_id` from params and validates against the required role before calling the handler.

Role validation failures are logged too (since `withLogging` wraps the entire chain).

**Declarative registration** (`toolDef` struct):

```go
type toolDef struct {
    tool        protocol.Tool
    handler     ToolHandler
    roleChecker RoleChecker  // nil = no role check (read-only tools)
}
```

Mutation and complex-operation tools are defined as `[]toolDef` slices and registered via `registerToolDefs`. Read-only tools remain procedural (simpler, fewer of them).

### Rationale

Middleware is the standard pattern for cross-cutting handler concerns. It eliminates the inline boilerplate that caused the ADR-0039 gaps in the first place — new tools get logging automatically and role checks by declaring a `roleChecker`, not by remembering to copy-paste extraction code.

Declarative registration makes tool definitions scannable as data. The schema consistency test was updated to parse both direct `registerTool` calls and `toolDef` metadata, maintaining compile-time-like safety over the tool inventory.

### Consequences

**Positive:**
- All MCP tool calls are now observable (logged with timing)
- Role validation is declarative — impossible to forget for new mutation tools
- Tool definitions are scannable data, not scattered procedural code
- Schema consistency test covers both registration patterns

**Limitations accepted:**
- Read-only tools still use procedural registration (acceptable — fewer tools, no role check needed)
- Middleware indirection adds one layer of wrapping per concern

**Extends:** ADR-0039 (MCP Role-Based Access Control) — middleware makes RBAC structural rather than per-handler. ADR-0030 (Code-Enforced Guardrails) — further shifts enforcement from prompts to code.

---
*Reconstructed from commits 728249e..ee59b10 (2026-03-11)*
