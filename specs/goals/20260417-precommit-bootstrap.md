# Pre-Commit Bootstrap for Greenfield Projects

## Problem Statement

`internal/prompts/templates/blocks/commit_workflow.tmpl:3` instructs coders to run pre-commit on every commit. The instruction assumes `.pre-commit-config.yaml` exists. In a greenfield project (`liza init` + vision doc, no pre-existing tooling), the file is absent.

**Depends on `20260417-precommit-hook-composition.md`** — without hook chaining in `.liza-hooks/pre-commit`, the config file is never auto-executed by the worktree commit hook. Bootstrap is a no-op without that spec landing first.

### Evidence

This is a **proactive hardening** change. The failure modes are mechanical consequences of an absent file plus an instruction that assumes its presence — they do not require empirical reproduction to justify the goal-level design.

**Rule 5 waiver (goal spec only):** the implementation spec must reproduce a greenfield run and capture observed failure modes before specifying remediation. At the goal level, hypothesized modes drive scope:
- Coders inventing `.pre-commit-config.yaml` ad-hoc inside an unrelated coding task.
- Parallel coders producing conflicting configs.
- Coders silently skipping pre-commit when it errors with "no config".
- DoD step "Pre-commit passes on touched files" (CORE.md Rule 3) becoming vacuous.

If reproduction shows the actual failure mode is different from any of the above, the implementation spec re-scopes — this spec does not bind it.

## Preconditions

### Prompt wording to narrow

`internal/prompts/templates/base_prompt.tmpl:36` says: *"NEVER attempt to install, bootstrap, or fix system-level tooling."* The line sits under the BASH CONSTRAINTS block (alongside "NEVER use $()…", "NEVER combine cd and git…") — the original intent targets system package installs, language runtimes, and IDE setup, not project config files. Implementation spec narrows the wording to make the system/project boundary explicit. `base_prompt.tmpl` is on the implementation surface.

### Composition with post-submit commit guard

The post-submit commit guard (`fcb7edc7`, `f840debe`) and project pre-commit are not orthogonal — both want to run on `git commit` in a worktree. Composition is owned by `20260417-precommit-hook-composition.md`. This spec assumes that composition is in place.

### `pre-commit` binary install

The chain spec assumes `pre-commit` is on PATH; bootstrap's `done_when` (`pre-commit run --all-files`) requires the same. Greenfield projects may not have it installed. Implementation spec must resolve this — recommended approach: extend the `base_prompt.tmpl:36` narrowing to explicitly permit `pre-commit` install (e.g. `pip install pre-commit` / `uv tool install pre-commit` / `brew install pre-commit`) inside the bootstrap coding task. This keeps `liza init`'s preconditions minimal and avoids forking another goal spec, at the cost of one carve-out in the no-system-tooling rule. Alternatives: (a) document `pre-commit` as a `liza init` precondition, or (b) defer to a separate goal.

## Design

The architecture step (ADR-0056) is the first agent step where the project's tech stack is decided. Bootstrap of stack-dependent tooling can therefore be planned no earlier than the architecture step.

### Q1: Architect emits bootstrap as `output[0]`

The architect's `output[]` becomes M code-planning task definitions per ADR-0036. The bootstrap entry produces a code-planning task → coding task chain dedicated to `.pre-commit-config.yaml`. All other architect `output[]` entries declare `depends_on: ["0"]`. **Phase-gate inheritance** (ADR-0048, `computeInheritedDeps` in `internal/ops/proceed.go:1076`) auto-propagates the dependency from code-planning to coding tasks — no new primitive needed.

- Uses existing `output[]` channel and existing dependency-inheritance machinery.
- Architect has full stack context (sees `arch_ref`).
- Normal review gates apply.

**Cost (accepted):** two extra review pairs (code-plan-review, code-review) for what is ultimately a config file. The trade-off is accepted because the alternatives below require new pipeline primitives — paying ceremony cost for a known design vs. paying engineering cost for a new one.

**Ruled-out alternatives** (preserved here so the rationale survives):
- *Code-planner emits bootstrap.* ADR-0056 created the architecture step explicitly to prevent code-planners from making cross-scope structural decisions. Bootstrap is repo-wide, not per-scope. M parallel code-planners each emitting bootstrap reintroduces the failure mode ADR-0056 fixed.
- *Pipeline-injected bootstrap (new task-creation path in `proceed.go`).* Would require a new task-creation primitive: review parity for a task without `output[]` provenance, `parent_task` linkage, `transitions_executed` bookkeeping, checkpoint visibility, replan handling, and a deviation from the ADR-0036 invariant that tasks come from agent `output[]`. Substantially heavier than reusing `output[]` + phase-gate inheritance.

### Q2: Idempotency — plan-time and execution-time

Detection happens at two points:
- **Plan-time** (architect): the architect's prompt context must include a rendered field (e.g. `.PreCommitConfigExists`) populated by `internal/prompts/role_context.go` from a resolver-side check against integration-branch HEAD. When true, the architect omits the bootstrap entry entirely. Implementation spec specifies the resolver primitive; precedent for "exists" semantics is `checkSpecFileExists` in `internal/statevalidate/validate.go:92`.
- **Execution-time** (coder): when claiming the bootstrap task, re-check worktree disk + integration branch. If the file appeared after planning (human added it, or another in-flight task merged), the coder calls `liza mark-blocked` with reason "config already present at execution time". The planner-orchestrator handles via standard rescope flow (`SUPERSEDED`, per `specs/architecture/roles.md:111`). Coders cannot mark tasks SUPERSEDED themselves (`internal/prompts/templates/base_prompt.tmpl:54`).

"Exists" rule: file presence is sufficient. Content adequacy is the project's responsibility — empty or malformed config still counts as "present". Coders fail fast on `pre-commit run`, surfacing the bad config as a normal task failure rather than silently re-bootstrapping.

### Q3: Hook content

**Option α — Stack-derived.** Architect narrates the hooks in the `output[0]` description (free-form, based on `arch_ref` ecosystem). **Constraint:** hook content must be narrated by the architect at plan time, never templated in prompts (G1.1 — no Liza-specific stack hardcoding).
- Pro: useful immediately.
- Con: relies on architect prompt judgment; bad picks land in the config until reviewed.

**Option β — Minimal universal only.** Bootstrap emits `trailing-whitespace`, `end-of-file-fixer`, `check-yaml`, `check-merge-conflict`. Stack-specific hooks added later by the first task that exercises the stack.
- Pro: stack-agnostic.
- Con: defers stack-specific protection indefinitely.

**Option γ — Hybrid.** Universal at bootstrap; architect emits a second `output[]` entry `add-stack-precommit-hooks` at index 1 with `depends_on: ["0"]`. **Indexing impact:** if γ is chosen, all other architect entries shift to indices 2+ and must declare `depends_on: ["0", "1"]` (both bootstrap entries are gating). Implementation spec must specify this explicitly to avoid silently relying on `["0"]` only.

Implementation spec picks one and justifies.

### Replan interaction

If the goal is re-planned after bootstrap is approved but before integration merge, the second architect invocation re-runs the plan-time existence check. The check reads integration-branch HEAD; if the bootstrap task's worktree hasn't merged yet, integration HEAD still lacks the config, and the architect re-emits `output[0]`. The orchestrator must deduplicate by detecting the existing in-flight bootstrap task — implementation spec specifies the dedup mechanism (likely a check on sibling tasks in the cohort with matching scope).

### Brownfield indexing

When bootstrap is omitted (config already exists), no synthetic `depends_on: ["0"]` is added to other entries. Indices renumber from 0; inter-scope dependencies revert to whatever the architect explicitly emits. The "bootstrap-first" gating is a property of the bootstrap entry's existence, not a structural assumption other entries depend on.

### Bootstrap failure semantics

If the bootstrap coding task fails (e.g. `pre-commit run --all-files` fails on the seed config):
- Standard BLOCKED escalation per `specs/protocols/task-lifecycle.md:158` (Coder BLOCKED → planner notified → may rescope). No new escalation primitive.
- Phase-gate inheritance already prevents downstream tasks from claiming until bootstrap is approved.

No degraded "no-hooks" mode. The contract requires pre-commit; failing to bootstrap it is a real failure, not a soft skip (Rule 14).

## Acceptance Criteria

The implementation spec is "done" when:
1. Greenfield run reproduced; observed failure modes captured (replaces hypothesized list above).
2. Q3 (hook content) has a chosen design with rationale.
3. Resolver primitive for plan-time existence check is specified, including which integration-branch ref is read and how the rendered field reaches the architect prompt context.
4. Orchestrator dedup mechanism for replan-while-in-flight is specified. **This is a new primitive** — no current sibling-task lookup at architect emission time exists. Implementation spec must scope this explicitly (sibling-task scan? scope-based dedup? cohort-scoped lock?), not treat it as trivial. Cross-goal parallelism (two goals racing on bootstrap) is a flavor of the same dedup problem and must be covered.
5. `base_prompt.tmpl:36` wording narrowed with a concrete edit; touch list explicitly includes that file plus the architect template and the bootstrap task's `done_when` phrasing.
6. Worked example for one greenfield stack (e.g. Python: vision doc → architect output → bootstrap task → resulting `.pre-commit-config.yaml`).
7. Statement of dependency on `20260417-precommit-hook-composition.md`.

## Out of Scope

- Worktree hook composition — covered by `20260417-precommit-hook-composition.md`.
- Choice of specific hook *versions* per ecosystem (defer to hook-selection task or ADR).
- Brownfield migration of partial/broken existing `.pre-commit-config.yaml`. Brownfield with any config no-ops.
- Bootstrap of other contract-assumed tooling (`.editorconfig`, lint config). The pattern may generalize; not addressed here.

## Status

Draft 2026-04-17. Pending implementation spec.
