# User Stories: Clean Code — Business Logic

Status: draft

## Goal

Apply the clean-code skill in full-file mode to all 71 non-test `.go` files across `internal/ops` (32 files, ~5,800 lines) and `internal/commands` (39 files, ~4,262 lines), leaving the code cleaner, more readable, and fully passing tests and pre-commit hooks.

## Context

The `internal/ops` package is the shared service layer implementing core task workflow operations as pure business logic. Functions return structured results with no terminal I/O — consumed by both CLI commands (`internal/commands`) and the agent supervisor (`internal/agent`). Operations span task claiming, state transitions, review submission, worktree management, sprint lifecycle, and agent recovery.

The `internal/commands` package implements all CLI command handlers. Each command corresponds to a `liza` subcommand: thin wrappers that parse arguments, delegate to `ops`, and format output for the terminal. Larger commands (inspect, status, watch, init) contain significant display logic, formatting, and interactive behavior.

These packages were built incrementally as Liza's feature set grew. A dedicated clean-code pass ensures naming, function size, duplication, and structure meet Uncle Bob's Clean Code principles before the codebase grows further.

This is one of six batched clean-code tasks spanning `internal/` and `cmd/`. Sibling tasks handle foundation packages, infrastructure, protocol, and lifecycle. This task covers business logic only.

## Personas

- **Liza Codebase Maintainer**: A Go developer who contributes to or reviews changes in the Liza multi-agent orchestrator. Reads `internal/ops` and `internal/commands` regularly, expects idiomatic Go, clear function names, small focused functions, and minimal duplication. Relies on `go test` and pre-commit hooks for regression safety.

## General information

### References

- Goal spec: `clean-code.md` — "Apply the clean-code skill to every .go file in internal/ and cmd/ (excluding tests)."
- Skill: `skills/clean-code/SKILL.md` — full-file mode, Go language profile, principle catalog, Liza mode behavior
- Go language profile: `skills/clean-code/languages/go.md` — Go-specific tool map and idiom patterns
- Sibling story: `stories/us-infrastructure-2.md` — conventions for clean-code story structure
- Sibling story: `stories/us-protocol-3-clean-code-protocol.md` — conventions for clean-code story structure
- Source (ops, 32 files): `internal/ops/add_tasks.go` (246), `internal/ops/advance_sprint.go` (188), `internal/ops/analyze.go` (114), `internal/ops/claim_reviewer_task.go` (152), `internal/ops/claim_task.go` (638), `internal/ops/clear_stale_review_claims.go` (140), `internal/ops/delete_agent.go` (132), `internal/ops/delete_task.go` (215), `internal/ops/doc.go` (8), `internal/ops/handoff.go` (106), `internal/ops/helpers.go` (23), `internal/ops/iteration_limits.go` (88), `internal/ops/mark_blocked.go` (104), `internal/ops/mode_change.go` (190), `internal/ops/pipeline_ops.go` (197), `internal/ops/precondition_error.go` (14), `internal/ops/proceed.go` (406), `internal/ops/recover_agent.go` (159), `internal/ops/recover_task.go` (229), `internal/ops/release_claim.go` (254), `internal/ops/resume_handoff.go` (155), `internal/ops/set_task_output.go` (82), `internal/ops/sprint_checkpoint.go` (230), `internal/ops/submit_review.go` (291), `internal/ops/submit_verdict.go` (228), `internal/ops/supersede_task.go` (97), `internal/ops/test_detection.go` (91), `internal/ops/update_sprint_metrics.go` (127), `internal/ops/write_checkpoint.go` (193), `internal/ops/wt_create.go` (126), `internal/ops/wt_delete.go` (82), `internal/ops/wt_merge.go` (495)
- Source (commands, 39 files): `internal/commands/add_task.go` (62), `internal/commands/analyze.go` (29), `internal/commands/claim_task.go` (51), `internal/commands/clear_stale_review_claims.go` (11), `internal/commands/delete_agent.go` (50), `internal/commands/delete_task.go` (81), `internal/commands/doc.go` (3), `internal/commands/format.go` (164), `internal/commands/handoff.go` (20), `internal/commands/init.go` (382), `internal/commands/inspect.go` (197), `internal/commands/inspect_agents.go` (235), `internal/commands/inspect_anomalies.go` (140), `internal/commands/inspect_field.go` (275), `internal/commands/inspect_metrics.go` (228), `internal/commands/inspect_tasks.go` (309), `internal/commands/inspect_time.go` (92), `internal/commands/mark_blocked.go` (19), `internal/commands/pause.go` (22), `internal/commands/proceed.go` (32), `internal/commands/recover_agent.go` (75), `internal/commands/recover_task.go` (51), `internal/commands/release_claim.go` (24), `internal/commands/resume.go` (32), `internal/commands/setup.go` (157), `internal/commands/sprint_checkpoint.go` (27), `internal/commands/start.go` (36), `internal/commands/status.go` (473), `internal/commands/stop.go` (21), `internal/commands/submit_review.go` (21), `internal/commands/submit_verdict.go` (36), `internal/commands/supersede_task.go` (21), `internal/commands/templates.go` (32), `internal/commands/update_sprint_metrics.go` (42), `internal/commands/validate.go` (28), `internal/commands/watch.go` (650), `internal/commands/wt_create.go` (31), `internal/commands/wt_delete.go` (29), `internal/commands/wt_merge.go` (74)
- Tests (ops, 28 files): `internal/ops/*_test.go` — comprehensive test suite covering all operations
- Tests (commands, 36 files): `internal/commands/*_test.go` — comprehensive test suite including integration tests

### Non-Functional Requirements

- NFR-000-1: **Behavior preservation** — every transformation batch must preserve existing behavior. All existing tests must pass after each batch. No behavioral changes are permitted.
- NFR-000-2: **Pre-commit compliance** — pre-commit hooks must pass on all touched files after all transformations complete.
- NFR-000-3: **Go idioms** — transformations must follow Go conventions as defined in the Go language profile (`skills/clean-code/languages/go.md`).
- NFR-000-4: **Idempotence** — re-running the clean-code skill on already-cleaned files must produce no further changes.

### Related External Components

- Component C-001 - clean-code skill: The refactoring methodology and principle catalog applied during this work.

### Out of Scope

- Test files (`*_test.go`) in either package — excluded per task scope.
- Files in other packages — handled by sibling tasks (`us-foundation-1`, `us-infrastructure-2`, `us-protocol-3`, `us-lifecycle-4`).
- Bug fixes discovered during cleaning — flag in analysis output per the skill protocol, do not auto-fix.
- New features, behavior changes, or API additions.
- Changing public API signatures without propagating to all callers within the worktree.
- Template files (`internal/commands/templates/*.tmpl` or similar) — only `.go` files are in scope.
- Restructuring operations (cross-module moves, file splits) — if analysis identifies restructuring opportunities, flag them per the skill's restructuring protocol for future consideration. Do not auto-execute.
- Formatting-only changes — delegated to pre-commit hooks per clean-code skill anti-patterns.

### Assumptions

- **ASM-000-1**: The Go language profile at `skills/clean-code/languages/go.md` provides all required tool map variables (`$TEST_CMD`, `$COVERAGE_CMD`, etc.) for the pre-flight phase. — *Why*: verified by sibling tasks that used the same profile. — ⚠️ Confidence: HIGH
- **ASM-000-2**: All existing tests in both packages pass before clean-code is applied (pre-flight baseline). — *Why*: standard pre-flight requirement per clean-code skill. — ⚠️ Confidence: HIGH
- **ASM-000-3**: Rename propagation from cleaned files to other in-scope files (within the same worktree) is permitted without scope extension, since all 71 files share the same task worktree. — *Why*: the clean-code skill allows auto-propagation within worktree scope in Liza mode. — ⚠️ Confidence: HIGH
- **ASM-000-4**: `doc.go` files (8 and 3 lines respectively, package-level documentation only) will require no code transformations since they contain only package comments with no executable code. The Coder should still include them in the analysis for completeness. — *Why*: the clean-code principle catalog applies to code constructs, not documentation comments. — ⚠️ Confidence: HIGH

### Open Questions

None — the task scope, skill protocol, and source files are well-defined.

---

## Story ST-001 — Clean ops task state machine core

### References

- Source: `internal/ops/claim_task.go` (638 lines), `internal/ops/claim_reviewer_task.go` (152 lines), `internal/ops/proceed.go` (406 lines), `internal/ops/release_claim.go` (254 lines), `internal/ops/pipeline_ops.go` (197 lines), `internal/ops/helpers.go` (23 lines), `internal/ops/precondition_error.go` (14 lines), `internal/ops/doc.go` (8 lines)
- Tests: `internal/ops/claim_task_test.go`, `internal/ops/claim_reviewer_task_test.go`, `internal/ops/proceed_test.go`, `internal/ops/release_claim_test.go`, `internal/ops/pipeline_ops_test.go`

### User Story

**As a** Liza codebase maintainer, **I want** the ops package's task state machine files (`claim_task.go`, `claim_reviewer_task.go`, `proceed.go`, `release_claim.go`, `pipeline_ops.go`) and shared support files (`helpers.go`, `precondition_error.go`, `doc.go`) cleaned to follow Clean Code principles, **so that** the core claiming, progression, and release logic — the most frequently modified and reviewed ops code — is easier to navigate, review, and extend when adding new task states or role-based claiming rules.

### Acceptance Criteria

- AC-001-1: Given the clean-code skill is run in full-file mode on all 8 files listed in this story, when the analysis and transformation loop completes, then all existing tests in the `ops` package pass.
- AC-001-1b: Given a file has no violations (e.g., `doc.go`, `precondition_error.go`), when analysis completes for that file, then the file is reported as already clean and no changes are made.
- AC-001-2: Given transformations are applied, when pre-commit hooks are run on touched files, then all hooks pass.
- AC-001-3: Given the clean-code skill identifies violations, when the analysis phase completes, then each violation is classified by principle (from the Principle Catalog) and addressed or documented as a remaining violation.
- AC-001-4: Given any bugs are spotted during analysis, when the analysis output is produced, then bugs are flagged separately and not auto-fixed.
- AC-001-5: Given any function extraction transformations are applied, when the Test Maintenance phase runs, then new unit tests are added for extracted functions and redundant indirect tests are removed.

### Depends on

None — this story establishes the shared support baseline for the ops package.

### Out of Scope

- Test files in `internal/ops/` — excluded per task scope.
- Modifying `PreconditionError`'s public API (used across the ops package and by external callers).

### Assumptions

- **ASM-001-1**: `claim_task.go` (638 lines) is the largest ops file and likely the richest source of clean-code violations (long functions, complex conditionals). The Coder should expect multiple transformation batches for this file. — *Why*: files above 400 lines consistently yield the most violations in clean-code passes. — ⚠️ Confidence: HIGH

### Open Questions

None.

---

## Story ST-002 — Clean ops review and delivery pipeline

### References

- Source: `internal/ops/submit_review.go` (291 lines), `internal/ops/submit_verdict.go` (228 lines), `internal/ops/write_checkpoint.go` (193 lines), `internal/ops/set_task_output.go` (82 lines), `internal/ops/handoff.go` (106 lines), `internal/ops/resume_handoff.go` (155 lines), `internal/ops/iteration_limits.go` (88 lines), `internal/ops/test_detection.go` (91 lines)
- Tests: `internal/ops/submit_review_test.go`, `internal/ops/submit_verdict_test.go`, `internal/ops/write_checkpoint_test.go`, `internal/ops/set_task_output_test.go`, `internal/ops/handoff_test.go`, `internal/ops/resume_handoff_test.go`, `internal/ops/test_detection_test.go`

### User Story

**As a** Liza codebase maintainer, **I want** the ops review and delivery files (`submit_review.go`, `submit_verdict.go`, `write_checkpoint.go`, `set_task_output.go`, `handoff.go`, `resume_handoff.go`, `iteration_limits.go`, `test_detection.go`) cleaned to follow Clean Code principles, **so that** the submission, review verdict, checkpoint, and handoff logic is easier to understand and modify when adding new review policies or iteration constraints.

### Acceptance Criteria

- AC-002-1: Given the clean-code skill is run in full-file mode on all 8 files listed in this story, when the analysis and transformation loop completes, then all existing tests in the `ops` package pass.
- AC-002-1b: Given a transformation batch fails tests, when the Coder detects the failure, then the batch is reverted and the failure is reported (no silent test regressions).
- AC-002-2: Given transformations are applied, when pre-commit hooks are run on touched files, then all hooks pass.
- AC-002-3: Given the clean-code skill identifies violations, when the analysis phase completes, then each violation is classified by principle and addressed or documented as a remaining violation.
- AC-002-4: Given any bugs are spotted during analysis, when the analysis output is produced, then bugs are flagged separately and not auto-fixed.
- AC-002-5: Given any function extraction transformations are applied, when the Test Maintenance phase runs, then new unit tests are added for extracted functions and redundant indirect tests are removed.

### Depends on

None.

### Out of Scope

- Test files in `internal/ops/` — excluded per task scope.
- Changes to the checkpoint data model or validation rules — these are behavioral, not clean-code.

### Assumptions

None beyond general assumptions.

### Open Questions

None.

---

## Story ST-003 — Clean ops task and agent lifecycle management

### References

- Source: `internal/ops/add_tasks.go` (246 lines), `internal/ops/delete_task.go` (215 lines), `internal/ops/delete_agent.go` (132 lines), `internal/ops/supersede_task.go` (97 lines), `internal/ops/mark_blocked.go` (104 lines), `internal/ops/recover_agent.go` (159 lines), `internal/ops/recover_task.go` (229 lines)
- Tests: `internal/ops/add_tasks_test.go`, `internal/ops/delete_task_test.go`, `internal/ops/delete_agent_test.go`, `internal/ops/supersede_task_test.go`, `internal/ops/mark_blocked_test.go`, `internal/ops/recover_agent_test.go`, `internal/ops/recover_task_test.go`

### User Story

**As a** Liza codebase maintainer, **I want** the ops task and agent lifecycle files (`add_tasks.go`, `delete_task.go`, `delete_agent.go`, `supersede_task.go`, `mark_blocked.go`, `recover_agent.go`, `recover_task.go`) cleaned to follow Clean Code principles, **so that** the CRUD and recovery operations for tasks and agents are easier to understand, review, and maintain when adding new lifecycle transitions or recovery policies.

### Acceptance Criteria

- AC-003-1: Given the clean-code skill is run in full-file mode on all 7 files listed in this story, when the analysis and transformation loop completes, then all existing tests in the `ops` package pass.
- AC-003-1b: Given a transformation batch fails tests, when the Coder detects the failure, then the batch is reverted and the failure is reported (no silent test regressions).
- AC-003-2: Given transformations are applied, when pre-commit hooks are run on touched files, then all hooks pass.
- AC-003-3: Given the clean-code skill identifies violations, when the analysis phase completes, then each violation is classified by principle and addressed or documented as a remaining violation.
- AC-003-4: Given any bugs are spotted during analysis, when the analysis output is produced, then bugs are flagged separately and not auto-fixed.

### Depends on

None.

### Out of Scope

- Test files in `internal/ops/` — excluded per task scope.
- Changes to task/agent model structures in `internal/models` — out of package scope.

### Assumptions

None beyond general assumptions.

### Open Questions

None.

---

## Story ST-004 — Clean ops worktree, sprint management, and support

### References

- Source: `internal/ops/wt_create.go` (126 lines), `internal/ops/wt_delete.go` (82 lines), `internal/ops/wt_merge.go` (495 lines), `internal/ops/sprint_checkpoint.go` (230 lines), `internal/ops/advance_sprint.go` (188 lines), `internal/ops/update_sprint_metrics.go` (127 lines), `internal/ops/analyze.go` (114 lines), `internal/ops/mode_change.go` (190 lines), `internal/ops/clear_stale_review_claims.go` (140 lines)
- Tests: `internal/ops/wt_create_test.go`, `internal/ops/wt_delete_test.go`, `internal/ops/wt_merge_test.go`, `internal/ops/sprint_checkpoint_test.go`, `internal/ops/advance_sprint_test.go`, `internal/ops/update_sprint_metrics_test.go`, `internal/ops/analyze_test.go`, `internal/ops/mode_change_test.go`, `internal/ops/clear_stale_review_claims_test.go`

### User Story

**As a** Liza codebase maintainer, **I want** the ops worktree, sprint management, and ancillary operations files (`wt_create.go`, `wt_delete.go`, `wt_merge.go`, `sprint_checkpoint.go`, `advance_sprint.go`, `update_sprint_metrics.go`, `analyze.go`, `mode_change.go`, `clear_stale_review_claims.go`) cleaned to follow Clean Code principles, **so that** worktree lifecycle, sprint transitions, and housekeeping operations are easier to navigate and maintain when extending the sprint model or adding new worktree operations.

### Acceptance Criteria

- AC-004-1: Given the clean-code skill is run in full-file mode on all 9 files listed in this story, when the analysis and transformation loop completes, then all existing tests in the `ops` package pass.
- AC-004-1b: Given a transformation batch fails tests, when the Coder detects the failure, then the batch is reverted and the failure is reported (no silent test regressions).
- AC-004-2: Given transformations are applied, when pre-commit hooks are run on touched files, then all hooks pass.
- AC-004-3: Given the clean-code skill identifies violations, when the analysis phase completes, then each violation is classified by principle and addressed or documented as a remaining violation.
- AC-004-4: Given any bugs are spotted during analysis, when the analysis output is produced, then bugs are flagged separately and not auto-fixed.
- AC-004-5: Given any function extraction transformations are applied, when the Test Maintenance phase runs, then new unit tests are added for extracted functions and redundant indirect tests are removed.

### Depends on

None.

### Out of Scope

- Test files in `internal/ops/` — excluded per task scope.
- Changes to `internal/git/` worktree plumbing functions — out of package scope (handled by sibling task `us-infrastructure-2`).
- Changes to sprint model structures in `internal/models` — out of package scope.

### Assumptions

- **ASM-004-1**: `wt_merge.go` (495 lines) is the second-largest ops file and likely contains long functions mixing git operations with state management. The Coder should expect multiple transformation batches. — *Why*: merge operations combine multiple concerns (git plumbing, state transitions, error recovery) in a single flow. — ⚠️ Confidence: HIGH

### Open Questions

None.

---

## Story ST-005 — Clean commands thin operation wrappers

### References

- Source (27 files): `internal/commands/add_task.go` (62), `internal/commands/analyze.go` (29), `internal/commands/claim_task.go` (51), `internal/commands/clear_stale_review_claims.go` (11), `internal/commands/delete_agent.go` (50), `internal/commands/delete_task.go` (81), `internal/commands/doc.go` (3), `internal/commands/handoff.go` (20), `internal/commands/mark_blocked.go` (19), `internal/commands/pause.go` (22), `internal/commands/proceed.go` (32), `internal/commands/recover_agent.go` (75), `internal/commands/recover_task.go` (51), `internal/commands/release_claim.go` (24), `internal/commands/resume.go` (32), `internal/commands/sprint_checkpoint.go` (27), `internal/commands/start.go` (36), `internal/commands/stop.go` (21), `internal/commands/submit_review.go` (21), `internal/commands/submit_verdict.go` (36), `internal/commands/supersede_task.go` (21), `internal/commands/templates.go` (32), `internal/commands/update_sprint_metrics.go` (42), `internal/commands/validate.go` (28), `internal/commands/wt_create.go` (31), `internal/commands/wt_delete.go` (29), `internal/commands/wt_merge.go` (74)
- Tests: `internal/commands/add_task_test.go`, `internal/commands/analyze_test.go`, `internal/commands/claim_task_test.go`, `internal/commands/clear_stale_review_claims_test.go`, `internal/commands/delete_agent_test.go`, `internal/commands/delete_task_test.go`, `internal/commands/handoff_test.go`, `internal/commands/mark_blocked_test.go`, `internal/commands/pause_test.go`, `internal/commands/recover_agent_test.go`, `internal/commands/release_claim_test.go`, `internal/commands/resume_test.go`, `internal/commands/sprint_checkpoint_test.go`, `internal/commands/start_test.go`, `internal/commands/stop_test.go`, `internal/commands/submit_review_test.go`, `internal/commands/submit_verdict_test.go`, `internal/commands/supersede_task_test.go`, `internal/commands/update_sprint_metrics_test.go`, `internal/commands/validate_test.go`, `internal/commands/watch_test.go`, `internal/commands/wt_create_test.go`, `internal/commands/wt_delete_test.go`, `internal/commands/wt_merge_test.go`

### User Story

**As a** Liza codebase maintainer, **I want** the 27 thin CLI command wrappers in `internal/commands/` (each under ~82 lines) cleaned to follow Clean Code principles, **so that** the consistent arg-parsing-to-ops-delegation pattern across these files is uniform, uses clear names, and any duplicated error handling or formatting patterns are visible for potential consolidation.

### Acceptance Criteria

- AC-005-1: Given the clean-code skill is run in full-file mode on all 27 files listed in this story, when the analysis and transformation loop completes, then all existing tests in the `commands` package pass.
- AC-005-1b: Given a file has no violations (e.g., `doc.go`), when analysis completes for that file, then the file is reported as already clean and no changes are made.
- AC-005-2: Given transformations are applied, when pre-commit hooks are run on touched files, then all hooks pass.
- AC-005-3: Given the clean-code skill identifies violations, when the analysis phase completes, then each violation is classified by principle and addressed or documented as a remaining violation.
- AC-005-3b: Given the clean-code skill identifies a duplicated pattern across multiple wrapper files (e.g., identical error formatting), when the Coder encounters it, then the pattern is flagged per the skill's DRY analysis. The Coder evaluates whether extraction to a shared helper improves clarity or adds unnecessary indirection.
- AC-005-4: Given any bugs are spotted during analysis, when the analysis output is produced, then bugs are flagged separately and not auto-fixed.

### Depends on

None.

### Out of Scope

- Test files in `internal/commands/` — excluded per task scope.
- Changes to ops function signatures that these wrappers call — out of package scope.
- The larger command files (`init.go`, `status.go`, `watch.go`, `inspect*.go`, `format.go`, `setup.go`) — covered by ST-006 and ST-007.

### Assumptions

- **ASM-005-1**: Many of these 27 thin wrappers follow a near-identical pattern (parse args → call ops function → format output → return error). Most will have few or no clean-code violations due to their small size and simple structure. — *Why*: files under 50 lines rarely accumulate significant violations. — ⚠️ Confidence: HIGH

### Open Questions

None.

---

## Story ST-006 — Clean commands inspection and display

### References

- Source: `internal/commands/inspect.go` (197 lines), `internal/commands/inspect_tasks.go` (309 lines), `internal/commands/inspect_field.go` (275 lines), `internal/commands/inspect_agents.go` (235 lines), `internal/commands/inspect_metrics.go` (228 lines), `internal/commands/inspect_anomalies.go` (140 lines), `internal/commands/inspect_time.go` (92 lines), `internal/commands/format.go` (164 lines), `internal/commands/status.go` (473 lines)
- Tests: `internal/commands/inspect_test.go`, `internal/commands/inspect_tasks_test.go`, `internal/commands/inspect_field_test.go`, `internal/commands/inspect_agents_test.go`, `internal/commands/inspect_metrics_test.go`, `internal/commands/inspect_anomalies_test.go`, `internal/commands/inspect_time_test.go`, `internal/commands/format_test.go`, `internal/commands/status_test.go`, `internal/commands/status_integration_test.go`

### User Story

**As a** Liza codebase maintainer, **I want** the inspection and display commands (`inspect.go`, `inspect_tasks.go`, `inspect_field.go`, `inspect_agents.go`, `inspect_metrics.go`, `inspect_anomalies.go`, `inspect_time.go`, `format.go`, `status.go`) cleaned to follow Clean Code principles, **so that** the query routing, data formatting, and multi-format output logic (JSON, YAML, table, value) is easier to navigate, review, and extend when adding new inspect subcommands or output formats.

### Acceptance Criteria

- AC-006-1: Given the clean-code skill is run in full-file mode on all 9 files listed in this story, when the analysis and transformation loop completes, then all existing tests in the `commands` package pass.
- AC-006-1b: Given a transformation batch fails tests, when the Coder detects the failure, then the batch is reverted and the failure is reported (no silent test regressions).
- AC-006-2: Given transformations are applied, when pre-commit hooks are run on touched files, then all hooks pass.
- AC-006-3: Given the clean-code skill identifies violations, when the analysis phase completes, then each violation is classified by principle and addressed or documented as a remaining violation.
- AC-006-3b: Given a principle conflict is identified (e.g., DRY vs readability for structurally similar inspect handlers), when the Coder encounters it, then the conflict is flagged with both options per the skill's principle conflict resolution guidance rather than silently choosing one.
- AC-006-4: Given any bugs are spotted during analysis, when the analysis output is produced, then bugs are flagged separately and not auto-fixed.
- AC-006-5: Given any function extraction transformations are applied, when the Test Maintenance phase runs, then new unit tests are added for extracted functions and redundant indirect tests are removed.

### Depends on

None.

### Out of Scope

- Test files in `internal/commands/` — excluded per task scope.
- Changes to `internal/models` display or serialization methods — out of package scope.
- Adding new inspect subcommands or output formats — this is refactoring only.

### Assumptions

- **ASM-006-1**: The `inspect_*.go` files likely share structural patterns (read state, filter/select, format output). Coincidental similarity does not warrant DRY extraction — each handler has distinct query logic even if the surrounding boilerplate is similar. — *Why*: same reasoning as ASM-003-2 in the sibling protocol story — each inspect handler filters and formats different data. — ⚠️ Confidence: HIGH

### Open Questions

None.

---

## Story ST-007 — Clean commands project initialization and watch

### References

- Source: `internal/commands/init.go` (382 lines), `internal/commands/setup.go` (157 lines), `internal/commands/watch.go` (650 lines)
- Tests: `internal/commands/init_test.go`, `internal/commands/setup_test.go`, `internal/commands/watch_test.go`

### User Story

**As a** Liza codebase maintainer, **I want** the project initialization and watch commands (`init.go`, `setup.go`, `watch.go`) cleaned to follow Clean Code principles, **so that** the interactive initialization flow, project setup logic, and the continuous file-watching loop with alert detection are easier to understand, debug, and extend when adding new initialization options or watch behaviors.

### Acceptance Criteria

- AC-007-1: Given the clean-code skill is run in full-file mode on all 3 files listed in this story, when the analysis and transformation loop completes, then all existing tests in the `commands` package pass.
- AC-007-1b: Given a transformation batch fails tests, when the Coder detects the failure, then the batch is reverted and the failure is reported (no silent test regressions).
- AC-007-2: Given transformations are applied, when pre-commit hooks are run on touched files, then all hooks pass.
- AC-007-3: Given the clean-code skill identifies violations, when the analysis phase completes, then each violation is classified by principle and addressed or documented as a remaining violation.
- AC-007-4: Given any bugs are spotted during analysis, when the analysis output is produced, then bugs are flagged separately and not auto-fixed.
- AC-007-5: Given any function extraction transformations are applied, when the Test Maintenance phase runs, then new unit tests are added for extracted functions and redundant indirect tests are removed.

### Depends on

None.

### Out of Scope

- Test files in `internal/commands/` — excluded per task scope.
- Changes to `internal/analysis/` pattern detection — out of package scope (handled by sibling task `us-protocol-3`).
- Changes to `internal/db/` watcher — out of package scope (handled by sibling task `us-infrastructure-2`).
- Adding new initialization prompts or watch alert categories — this is refactoring only.

### Assumptions

- **ASM-007-1**: `watch.go` (650 lines) is the largest commands file and contains multiple concerns (file watching loop, alert detection, threshold checks, terminal output). The Coder should expect multiple transformation batches and likely function extraction candidates. — *Why*: files above 400 lines consistently yield the most violations in clean-code passes. — ⚠️ Confidence: HIGH

### Open Questions

None.
