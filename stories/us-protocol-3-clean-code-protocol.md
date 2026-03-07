# User Stories: Clean Code — Protocol and Templates

Status: draft

## Goal

Apply the clean-code skill in full-file mode to all 10 non-test `.go` files across `internal/analysis`, `internal/mcp`, `internal/mcp/protocol`, `internal/prompts`, and `internal/roles`, leaving the code cleaner, more readable, and fully passing tests and pre-commit hooks.

## Context

Liza's MCP server layer (`internal/mcp/` and `internal/mcp/protocol/`) implements the JSON-RPC 2.0 protocol for tool-based agent communication. The prompt builder (`internal/prompts/`) generates role-specific bootstrap prompts for all agent types. The analysis package (`internal/analysis/`) provides circuit breaker pattern detection, and the roles package (`internal/roles/`) maps runtime and workflow role names.

These packages were built incrementally as features shipped. A dedicated clean-code pass ensures naming, function size, duplication, and structure meet Uncle Bob's Clean Code principles before the codebase grows further.

This is one of six batched clean-code tasks spanning `internal/` and `cmd/`. Sibling tasks handle foundation packages, infrastructure, lifecycle, and business logic. This task covers protocol, MCP, prompts, analysis, and roles only.

## Personas

- **Liza Codebase Maintainer**: A Go developer who contributes to or reviews changes in the Liza multi-agent orchestrator. Reads `internal/` packages regularly, expects idiomatic Go, clear function names, small focused functions, and minimal duplication. Relies on `go test` and pre-commit hooks for regression safety.

## General information

### References

- Goal spec: `clean-code.md` — "Apply the clean-code skill to every .go file in internal/ and cmd/ (excluding tests)."
- Skill: `skills/clean-code/SKILL.md` — full-file mode, Go language profile, principle catalog, Liza mode behavior
- Go language profile: `skills/clean-code/languages/go.md` — Go-specific tool map and idiom patterns
- Source files (10): `internal/analysis/patterns.go`, `internal/mcp/handlers.go`, `internal/mcp/server.go`, `internal/mcp/protocol/errors.go`, `internal/mcp/protocol/stdio.go`, `internal/mcp/protocol/testing.go`, `internal/mcp/protocol/types.go`, `internal/prompts/builder.go`, `internal/prompts/templates.go`, `internal/roles/roles.go`
- Test files (11): `internal/analysis/patterns_test.go`, `internal/mcp/handlers_test.go`, `internal/mcp/concurrency_test.go`, `internal/mcp/server_test.go`, `internal/mcp/server_dispatch_test.go`, `internal/mcp/server_run_test.go`, `internal/mcp/schema_consistency_test.go`, `internal/mcp/protocol/stdio_test.go`, `internal/mcp/protocol/types_test.go`, `internal/prompts/builder_test.go`, `internal/roles/roles_test.go`

### Non-Functional Requirements

- NFR-000-1: **Behavior preservation** — every transformation batch must preserve existing behavior. All existing tests must pass after each batch. No behavioral changes are permitted.
- NFR-000-2: **Pre-commit compliance** — pre-commit hooks must pass on all touched files after all transformations complete.
- NFR-000-3: **Go idioms** — transformations must follow Go conventions as defined in the Go language profile (`skills/clean-code/languages/go.md`).
- NFR-000-4: **Idempotence** — re-running the clean-code skill on already-cleaned files must produce no further changes.

### Out of Scope

- Test files (`*_test.go`) — excluded per task scope. Test maintenance (adding tests for extracted functions, removing redundant tests) IS in scope per the clean-code skill's Test Maintenance phase.
- Template files (`internal/prompts/templates/*.tmpl`) — only `.go` files are in scope.
- Files in other packages — handled by sibling tasks (`us-foundation-1`, `us-infrastructure-2`, `us-lifecycle-4`, `us-business-5`).
- Bug fixes discovered during cleaning — flag in analysis output per the skill protocol, do not auto-fix.
- New features, behavior changes, or API additions.
- Changing public API signatures without propagating to all callers within the worktree.

### Assumptions

- **ASM-000-1**: The Go language profile (`skills/clean-code/languages/go.md`) exists and provides `$TEST_CMD`, `$COVERAGE_CMD`, and other tool map entries needed by the clean-code skill. — *Why*: verified by listing available profiles. — Confidence: HIGH
- **ASM-000-2**: All existing tests in the 5 packages pass before clean-code is applied (pre-flight baseline). — *Why*: standard pre-flight requirement per clean-code skill. — Confidence: HIGH
- **ASM-000-3**: Rename propagation from cleaned files to other in-scope files (within the same worktree) is permitted without scope extension, since all 10 files share the same task worktree. — *Why*: the clean-code skill allows auto-propagation within worktree scope in Liza mode. — Confidence: HIGH

### Open Questions

None — the task scope, skill protocol, and source files are well-defined.

---

## Story ST-001 — Clean MCP protocol layer

### References

- Source: `internal/mcp/protocol/errors.go` (71 lines), `internal/mcp/protocol/stdio.go` (106 lines), `internal/mcp/protocol/testing.go` (41 lines), `internal/mcp/protocol/types.go` (80 lines)
- Tests: `internal/mcp/protocol/stdio_test.go`, `internal/mcp/protocol/types_test.go`
- Skill: `skills/clean-code/SKILL.md` — full-file mode, Liza mode behavior

### User Story

**As a** Liza codebase maintainer, **I want** the MCP protocol package (`internal/mcp/protocol/`) cleaned to follow Clean Code principles, **so that** JSON-RPC type definitions, error constructors, and transport code are easy to read, extend with new error codes or types, and review during future protocol changes.

### Acceptance Criteria

- AC-001-1: Given the clean-code skill is run in full-file mode on all 4 files in `internal/mcp/protocol/`, when the analysis and transformation loop completes, then all existing tests in the `protocol` package pass.
- AC-001-2: Given transformations are applied, when pre-commit hooks are run on touched files, then all hooks pass.
- AC-001-3: Given the clean-code skill identifies violations, when the analysis phase completes, then each violation is classified by principle (from the Principle Catalog) and addressed or documented as a remaining violation.
- AC-001-3b: Given the clean-code skill finds no violations in a file, when analysis completes for that file, then the file is reported as already clean and no changes are made.
- AC-001-4: Given any bugs are spotted during analysis, when the analysis output is produced, then bugs are flagged separately and not auto-fixed.

### Depends on

None.

### Out of Scope

- Changes to the `internal/mcp/` package (handlers, server) — covered by ST-002.
- Adding new protocol types or error codes.

### Assumptions

None beyond general assumptions.

### Open Questions

None.

---

## Story ST-002 — Clean MCP handlers and server

### References

- Source: `internal/mcp/handlers.go` (890 lines), `internal/mcp/server.go` (830 lines)
- Tests: `internal/mcp/handlers_test.go`, `internal/mcp/concurrency_test.go`, `internal/mcp/server_test.go`, `internal/mcp/server_dispatch_test.go`, `internal/mcp/server_run_test.go`, `internal/mcp/schema_consistency_test.go`
- Skill: `skills/clean-code/SKILL.md` — full-file mode, Liza mode behavior

### User Story

**As a** Liza codebase maintainer, **I want** the MCP handlers and server files (`internal/mcp/handlers.go`, `internal/mcp/server.go`) cleaned to follow Clean Code principles, **so that** tool handler implementations, parameter extraction, role validation, error classification, and tool registration code are easier to navigate, review, and extend when adding new MCP tools.

### Acceptance Criteria

- AC-002-1: Given the clean-code skill is run in full-file mode on `handlers.go` and `server.go`, when the analysis and transformation loop completes, then all existing tests in the `mcp` package pass (including concurrency, dispatch, run, schema, and handler tests).
- AC-002-2: Given transformations are applied, when pre-commit hooks are run on touched files, then all hooks pass.
- AC-002-3: Given the clean-code skill identifies violations, when the analysis phase completes, then each violation is classified by principle and addressed or documented as a remaining violation.
- AC-002-3b: Given a transformation would require renaming a public function or changing a function signature, when the Coder applies the transformation, then all callers within the worktree (including test files and protocol package) are updated in the same batch.
- AC-002-4: Given any bugs are spotted during analysis, when the analysis output is produced, then bugs are flagged separately and not auto-fixed.
- AC-002-5: Given any function extraction transformations are applied, when the Test Maintenance phase runs, then new unit tests are added for extracted functions and redundant indirect tests are removed.

### Depends on

Implementation ordering:
- Story ST-001 — Clean MCP protocol layer

*Rationale*: If ST-001 renames any protocol types or error constructors, `handlers.go` and `server.go` must reference the updated names. Ordering ensures the Coder works with finalized protocol identifiers.

### Out of Scope

- Changes to `internal/mcp/protocol/` files — covered by ST-001.
- Adding new MCP tools or removing deprecated tools.
- Modifying tool descriptions or JSON schema definitions (these are user-facing contracts).

### Assumptions

- **ASM-002-1**: The tool registration code in `server.go` (string literals for tool names, descriptions, schema properties) is not subject to rename transformations — only the Go code structure around it. — *Why*: tool names and descriptions are part of the MCP API contract visible to LLM agents. Renaming them would break client expectations. — Confidence: HIGH

### Open Questions

None.

---

## Story ST-003 — Clean prompt system, analysis, and roles

### References

- Source: `internal/prompts/builder.go` (616 lines), `internal/prompts/templates.go` (35 lines), `internal/analysis/patterns.go` (260 lines), `internal/roles/roles.go` (109 lines)
- Tests: `internal/prompts/builder_test.go`, `internal/analysis/patterns_test.go`, `internal/roles/roles_test.go`
- Skill: `skills/clean-code/SKILL.md` — full-file mode, Liza mode behavior

### User Story

**As a** Liza codebase maintainer, **I want** the prompt builder, template engine, analysis patterns, and role definitions cleaned to follow Clean Code principles, **so that** the repetitive `Build*Context` functions, pattern detection helpers, and role mapping code are easier to understand, maintain, and extend when new agent roles or analysis patterns are added.

### Acceptance Criteria

- AC-003-1: Given the clean-code skill is run in full-file mode on `builder.go`, `templates.go`, `patterns.go`, and `roles.go`, when the analysis and transformation loop completes, then all existing tests in the `prompts`, `analysis`, and `roles` packages pass.
- AC-003-2: Given transformations are applied, when pre-commit hooks are run on touched files, then all hooks pass.
- AC-003-3: Given the clean-code skill identifies violations, when the analysis phase completes, then each violation is classified by principle and addressed or documented as a remaining violation.
- AC-003-3b: Given a transformation in one package requires propagation to another in-scope package (e.g., a role constant rename in `roles.go` affecting `handlers.go`), when the Coder identifies the propagation, then the dependency is documented and the affected file is updated within the same worktree.
- AC-003-4: Given any bugs are spotted during analysis, when the analysis output is produced, then bugs are flagged separately and not auto-fixed.
- AC-003-5: Given any function extraction transformations are applied, when the Test Maintenance phase runs, then new unit tests are added for extracted functions and redundant indirect tests are removed.

### Depends on

None.

### Out of Scope

- Template files (`internal/prompts/templates/*.tmpl`) — only `.go` files are in scope.
- Changes to `internal/mcp/` or `internal/mcp/protocol/` — covered by ST-001 and ST-002.
- Adding new agent roles, prompt types, or analysis patterns.

### Assumptions

- **ASM-003-1**: The `Build*Context` functions in `builder.go` share a common pattern (worktree path construction, `hasPriorRejection` check, template execution) that may be a DRY candidate. The Coder should evaluate whether extracting a shared helper improves clarity or harms it per the DRY vs KISS conflict resolution. — *Why*: ~10 functions repeat the same 5-line pattern. Extraction is plausible but "DRY extraction adds indirection" conflict applies. — Confidence: HIGH
- **ASM-003-2**: The `internal/analysis/patterns.go` check functions (`checkRetryCluster`, `checkDebtAccumulation`, etc.) follow a similar structure but have distinct logic in each. Coincidental similarity does not warrant DRY extraction. — *Why*: each function filters by different type, groups by different field, and applies different thresholds. — Confidence: HIGH

### Open Questions

None.
