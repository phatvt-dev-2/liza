# Refactoring Pass 2 — Coding Plan

Implementation plan for the structural and quality improvements defined in
`to-fix.md` and assessed in `specs/architecture/code_quality_assessment.md`.

All tasks are pure refactoring or additive — no behavioral changes.

---

## Dependency Graph

```
CP4 (typed event constants in models)
├── CP5 (adopt constants in statevalidate)
├── CP6 (adopt constants in ops, excluding claim_task)
├── CP7 (adopt constants in commands)
├── CP8 (adopt constants in agent)
└── CP9 (claim strategy — finishes claim-path event adoption)

CP1 (CLI split)          — independent
CP2 (git/worktree split) — independent
CP3 (statevalidate tests)— independent
CP10 (MCP registration)  — independent
CP11 (assessment update) — after CP1–CP10 merged
```

---

## CP1: Split CLI Entry Point

**Desc:** Split the CLI entrypoint into domain-specific command files without changing CLI behavior

**Done when:** cmd/liza/main.go retains only root wiring/shared helpers, the planned cmd_*.go files own the command definitions, behavioral CLI tests still pass, and any test edits are mechanical only

**Scope:** Refactor cmd/liza/main.go into cmd_task.go, cmd_worktree.go, cmd_agent.go, cmd_review.go, cmd_system.go, and cmd_init.go plus only the CLI tests needed to preserve existing command/flag/help behavior. No command semantics changes.

**Risk:** Low — pure structural split, zero behavior change

**Spec ref:** to-fix.md §1.5

---

## CP2: Reorganize git/worktree.go by Concern

**Desc:** Reorganize internal/git/worktree.go into concern-based files without changing git behavior

**Done when:** worktree CRUD helpers remain in worktree.go, merge helpers live in merge.go, rebase helpers live in rebase.go, query helpers live in query.go, Git stays defined in git.go, and targeted internal/git tests pass unchanged in behavior

**Scope:** Refactor internal/git/worktree.go and related internal/git tests only. Preserve receiver semantics and existing behavior; no new git features or workflow changes.

**Risk:** Low — all methods on the same receiver; grouping is purely organizational

**Spec ref:** to-fix.md §1.6

---

## CP3: Add statevalidate Edge-Case Tests

**Desc:** Add edge-case tests for task validation branches in internal/statevalidate

**Done when:** new table-driven tests exercise status-specific required field enforcement, completion-field requirements, integration_fix history linkage, duplicate failed_by rejection, parent-task referential integrity, and output-entry completeness in validate_task.go, and go test ./internal/statevalidate passes

**Scope:** Modify internal/statevalidate test files only, primarily validate_task_test.go plus adjacent package test helpers if needed. No production validation logic changes in this task.

**Risk:** Low — additive test-only change

**Spec ref:** to-fix.md §2.4

---

## CP4: Define Typed Task-Event Constants

**Desc:** Define typed task-event constants in internal/models

**Done when:** internal/models defines a typed task-event name and constants for the existing task-history vocabulary in scope, history model code preserves serialized event values, and go test ./internal/models passes

**Scope:** Modify internal/models/history.go and only the internal/models tests needed to cover the new typed event constants. No producer or consumer adoption outside internal/models in this task.

**Risk:** Low — additive definition, no consumer changes

**Spec ref:** to-fix.md §2.5a

**Depends on:** none

---

## CP5: Adopt Event Constants in statevalidate

**Desc:** Replace task-event literals in internal/statevalidate production validation logic with shared constants

**Done when:** internal/statevalidate/validate_task.go uses the internal/models task-event constant for integration_failed instead of a raw literal, related statevalidate tests preserve the same validation behavior, and go test ./internal/statevalidate passes

**Scope:** Modify internal/statevalidate/validate_task.go and only the statevalidate tests needed to keep production constant adoption covered. No broader test-ratio expansion or unrelated validation-rule changes.

**Risk:** Low — mechanical find-and-replace

**Spec ref:** to-fix.md §2.5a

**Depends on:** CP4

---

## CP6: Adopt Event Constants in ops (Non-Claim)

**Desc:** Replace non-claim task-event literals in internal/ops with shared constants

**Done when:** the non-claim event producers and consumers in internal/ops, including sprint-metrics history consumers in update_sprint_metrics.go, use internal/models task-event constants instead of duplicated raw literals, internal/ops/claim_task.go remains out of scope, and go test ./internal/ops passes

**Scope:** Touch only non-claim internal/ops event sites such as write_checkpoint.go, submit_verdict.go, submit_review.go, mark_blocked.go, handoff.go, proceed.go, wt_merge.go, supersede_task.go, resume_handoff.go, add_tasks.go, update_sprint_metrics.go, and related internal/ops tests. Do not refactor claim-path control flow.

**Risk:** Low — mechanical find-and-replace

**Spec ref:** to-fix.md §2.5a

**Depends on:** CP4

---

## CP7: Adopt Event Constants in commands

**Desc:** Replace task-event literals in internal/commands with shared constants

**Done when:** the command-side history comparisons, including task timing/status-duration calculations, and emitted initialization events in inspect_time.go, inspect_tasks.go, watch.go, and init.go use internal/models task-event constants instead of raw literals, command behavior stays unchanged, and go test ./internal/commands passes

**Scope:** Modify internal/commands/inspect_time.go, internal/commands/inspect_tasks.go, internal/commands/watch.go, internal/commands/init.go, and only the related internal/commands tests needed to preserve existing behavior. No CLI semantic changes.

**Risk:** Low — mechanical find-and-replace

**Spec ref:** to-fix.md §2.5a

**Depends on:** CP4

---

## CP8: Adopt Event Constants in agent

**Desc:** Replace task-event literals in internal/agent with shared constants

**Done when:** the agent-side history comparisons and emitted events in registration.go, supervisor.go, and worktree_check.go use internal/models task-event constants instead of raw literals, agent behavior stays unchanged, and go test ./internal/agent passes

**Scope:** Modify internal/agent/registration.go, internal/agent/supervisor.go, internal/agent/worktree_check.go, and only the related internal/agent tests needed to preserve existing behavior. No supervision-policy changes.

**Risk:** Low — mechanical find-and-replace

**Spec ref:** to-fix.md §2.5a

**Depends on:** CP4

---

## CP9: Claim Strategy Pattern

**Desc:** Refactor ClaimTask to use a strategy abstraction instead of threaded claim-type booleans

**Done when:** fresh, rejection, and integration-fix claim paths are represented by a strategy abstraction with encapsulated preconditions, worktree handling, state mutation, and event naming, existing claim behaviors are preserved, and targeted claim-task tests pass

**Scope:** Refactor internal/ops/claim_task.go, its tests, and any minimal supporting ops files needed for the strategy extraction. This task may finish claim-path event constant adoption introduced by the shared model constants but must not broaden into unrelated event-literal cleanup.

**Risk:** Medium — refactoring the most complex operation requires careful test verification

**Spec ref:** to-fix.md §2.6

**Depends on:** CP4 (for event constants used in claim paths)

---

## CP10: Declarative MCP Tool Registration

**Desc:** Replace imperative MCP tool registration with declarative tool definitions

**Done when:** server_registration.go registers mutation and complex-operation tools from declarative metadata definitions, schema consistency safeguards still pass, and MCP registration behavior remains unchanged

**Scope:** Modify internal/mcp/server_registration.go and only the MCP tests needed to preserve handler/schema/role-check coverage. No tool inventory changes beyond the registration structure.

**Risk:** Medium — schema definitions are currently compile-time verified via schema_consistency_test.go; declarative approach must preserve that

**Spec ref:** to-fix.md §2.7

**Depends on:** none

---

## CP11: Update Architecture Assessment

**Desc:** Update the architecture code-quality assessment after the refactors are merged

**Done when:** specs/architecture/code_quality_assessment.md reflects the completed fixes, removes or revises the resolved recommendations, and updates the relevant summary/subsystem notes consistently with the post-refactor code state

**Scope:** Update specs/architecture/code_quality_assessment.md only after the implementation tasks for the planned refactors have landed. No new refactoring scope beyond documenting the resulting code state.

**Risk:** Low — documentation-only change

**Spec ref:** to-fix.md §Assessment update

**Depends on:** CP1, CP2, CP3, CP4, CP5, CP6, CP7, CP8, CP9, CP10
