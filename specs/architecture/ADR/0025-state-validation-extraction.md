# 25 - State Validation Extraction

## Context and Problem Statement

State validation logic lived in `internal/commands/validate.go`, tightly coupled to the CLI layer. With the ops service layer (ADR-0021) providing a shared mutation path for both CLI and MCP, validation needed to be accessible from ops as well — but importing commands from ops would create an import cycle. More fundamentally, having validation only in commands meant CLI and MCP could diverge: changes to validation rules required updates in multiple locations to stay consistent.

## Considered Options

1. **Keep validation in commands** — accept the duplication risk across CLI and MCP entry points.
2. **Extract validation into a shared `internal/statevalidate` package** — accessible from both commands and ops without import cycles.

## Decision Outcome

Chose **Option 2**: a dedicated `internal/statevalidate` package.

### Architecture

- Moved ~430 lines of validation logic from `internal/commands/validate.go` to `internal/statevalidate/validate.go`
- CLI `validate` command now delegates to the shared package
- `ops.AddTask` runs post-write validation and returns a typed `PostWriteValidationError` when mutation succeeds but full-state validation fails
- MCP layer classifies `PostWriteValidationError` as `ValidationError` for proper error reporting to callers

### Rationale

Centralized validation ensures CLI and MCP apply identical rules without coordinated changes across packages. The ops layer is the natural consumer — mutations that can validate themselves are safer than mutations that rely on callers to validate afterward.

### Consequences

**Positive:**
- Single validation implementation shared by CLI, MCP, and ops
- Post-write validation catches state inconsistencies at mutation time, not later
- No import cycles between packages
- Typed error (`PostWriteValidationError`) distinguishes "mutation succeeded but state is inconsistent" from mutation failure

**Limitations accepted:**
- Validation is now split from the command that originally housed it — `commands/validate.go` becomes a thin wrapper

---
*Reconstructed from commit 6fe5bcc (2026-02-24)*
