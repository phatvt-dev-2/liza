# 38 - Phase 2 Roles

## Context and Problem Statement

Liza operated with 3 roles: orchestrator, coder, and code-reviewer. This was sufficient for the coding sub-pipeline but not for a disciplined system with human review gates between phases. Adding spec-writing, planning, and story-writing phases required specialized roles — not just different prompts, but dedicated supervisor handling for each role's mechanical requirements.

## Considered Options

1. **Reuse coder/reviewer with different prompts** — same supervisor logic, only prompt templates change. Insufficient: a new role is not just a new prompt but also a specific supervisor that handles the mechanical part of the role (merge handling, submission logging, wake triggers, timeout defaults).
2. **New dedicated roles** — each role gets its own prompt template, context builder, and supervisor dispatch path.

## Decision Outcome

Chose **Option 2**: introduce 6 new roles, expanding from 3 to 9 total.

### Architecture

**New roles:**

| Runtime | Workflow | Sub-pipeline | Pair role |
|---------|----------|-------------|-----------|
| `code-planner` | `code_planner` | coding | doer |
| `code-plan-reviewer` | `code_plan_reviewer` | coding | reviewer |
| `epic-planner` | `epic_planner` | epic-spec | doer |
| `epic-plan-reviewer` | `epic_plan_reviewer` | epic-spec | reviewer |
| `us-writer` | `us_writer` | epic-spec | doer |
| `us-reviewer` | `us_reviewer` | epic-spec | reviewer |

**Supervisor generalization:**
The per-role if/else chains are replaced with category helpers:
```go
func isDoerRuntime(role string) bool     // coder, code-planner, epic-planner, us-writer
func isReviewerRuntime(role string) bool  // code-reviewer, code-plan-reviewer, epic-plan-reviewer, us-reviewer
```

Used for: timeout defaults, merge handling, task claiming, submission logging, `waitForWork` dispatch, `buildPrompt` dispatch.

**Prompt templates (4 new):**

| Template | Builder | Key content |
|----------|---------|-------------|
| `epic_planner_context.tmpl` | `BuildEpicPlannerContext()` | 7 granularity signals, right-sized epic criteria (3-8 stories), `set_task_output` integration |
| `epic_plan_reviewer_context.tmpl` | `BuildEpicPlanReviewerContext()` | 6 decomposition quality gates: cohesive capability, right-sized scope, falsifiable done_when, persona coherence, independence, vision coverage |
| `us_writer_context.tmpl` | `BuildUSWriterContext()` | References user-story-writing skill (ADR-0034), SMARC criteria, canonical story form |
| `us_reviewer_context.tmpl` | `BuildUSReviewerContext()` | Story quality review gates |

Code-planner and code-plan-reviewer templates were added in earlier commits as part of the pipeline work.

**MCP role authorization:**
Role checks extended to include all new roles — `us-reviewer` can call `submit_verdict` and `wt_merge`; pipeline doer roles can call `write_checkpoint`, `submit_for_review`, `set_task_output`.

**Wake triggers:**
`PLANNING_COMPLETE` trigger added for planning-to-coding transitions, restricted to code-planning-pair tasks.

### Rationale

Liza is a disciplined system steered by humans. Each phase has distinct quality criteria, review gates, and mechanical requirements. The adversarial doer/reviewer pair pattern (ADR-0035) scales to any activity — epic decomposition, story writing, coding — but each pair needs role-specific expertise in its prompt and role-specific handling in the supervisor.

### Consequences

**Positive:**
- Each role has focused expertise — prompts contain only relevant methodology
- Supervisor handles mechanical differences (timeouts, merge paths, wake triggers) per role category
- MCP authorization prevents role boundary violations
- Pattern scales to future roles (architecture-reviewer, security-reviewer, etc.)

**Limitations accepted:**
- 9 roles means 9 prompt templates to maintain
- New roles require both template AND supervisor wiring
- `isDoerRuntime`/`isReviewerRuntime` must be updated for each new role

**Depends on:** ADR-0033 (Orchestrator rename freed "planner" name), ADR-0034 (skills consumed by new roles), ADR-0035 (sub-pipelines provide the structural framework).

---
*Reconstructed from commits 8a29623, fd4ddac, 1d71c1b, 9d9254b, b70af75, 1cce045, 1707c90, 15324c6 (2026-03-04 to 2026-03-07)*
