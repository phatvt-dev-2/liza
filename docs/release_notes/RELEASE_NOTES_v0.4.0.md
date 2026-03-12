# Liza v0.4.0

The orchestrator becomes pipeline-driven:
- declarative sub-pipelines replace hardcoded role-pair logic with YAML configuration,
- six new roles enable a full specification phase before coding,
- rebase conflict detection catches integration failures at submission time,
- structured task output lets planning agents pass typed deliverables downstream,
- a 26-batch clean-code campaign refactors 122 Go files,
- and a second refactoring pass decomposes 6 monolithic files, extracts the RoleStrategy pattern,
  and replaces all raw event string literals with 26 typed constants.

A cross-cutting theme is pipeline generalization — the ops layer, supervisor, claiming, validation,
and sprint-terminal checks all become pipeline-aware, making Liza's runtime agnostic to the specific
role workflow driving each task. The second half of the release removes legacy (non-pipeline) code paths
entirely, making pipeline config mandatory.

211 commits since v0.3.0 (2026-03-02).

---

## Breaking Changes

- **`liza_add_task` renamed to batch `liza_add_tasks`** — the MCP endpoint now accepts multiple tasks. External automation referencing the old singular name must update.
- **Planner role renamed to Orchestrator** (ADR-0033) — across contracts, skills, docs, and role constants. Prompt templates and role references updated throughout.
- **BREAKING CHANGE footer required** — commit messages for breaking changes must now include a `BREAKING CHANGE:` footer stating what breaks and the migration path, per Conventional Commits.
- **Claim-type selectors renamed from `coder/code-reviewer` to `doer/reviewer`** (ADR-0042) — `--role` flag on `liza release-claim` accepts `doer`/`reviewer` instead of `coder`/`code-reviewer`. `ClaimDoer`/`ClaimReviewer`/`ClaimBoth` constants replace string literals.
- **Legacy (non-pipeline) code paths removed** — pipeline config is now mandatory. Workspaces without `.liza/pipeline.yaml` must re-run `liza init`. Tasks without `role_pair` are rejected. `Transition()`/`CanTransition()` dual-path patterns removed.

---

## Highlights

- **Declarative sub-pipelines** (ADR-0035) — YAML-driven pipeline configuration replaces hardcoded role-pair logic. Each sub-pipeline declares its own status graph, role assignments, and transitions. The resolver uses 3-part ref validation (sub-pipeline, status, transition) and the entire ops layer routes through it.
- **Specification-phase roles** (ADR-0038) — Six new roles (epic-planner, epic-plan-reviewer, us-writer, us-reviewer, code-planner, code-plan-reviewer) enable a specification sub-pipeline before coding sprints. The supervisor loop recognizes and dispatches all spec-phase roles.
- **Rebase conflict detection** (ADR-0037) — `SubmitForReview` catches rebase conflicts at submission time and transitions the task to `INTEGRATION_FAILED`, providing actionable feedback instead of silent merge failures.
- **Structured task output** (ADR-0036) — `liza_set_task_output` MCP tool lets planning agents persist typed deliverables (desc, done_when, scope, spec_ref) for downstream consumption. Scope extensions in checkpoints give reviewers visibility into out-of-scope modifications.
- **Clean-code campaign** — 122 Go files refactored across 26 batches: removed what-comments, extracted helpers, flattened nested conditionals, eliminated pointer intermediates, and standardized error handling patterns.
- **Refactoring pass 2** — 6 monolithic files decomposed, RoleStrategy pattern extracted from supervisor, claim task strategies separated, MCP middleware and declarative registration introduced, and all raw event string literals replaced with 26 typed constants.

---

## Features

**Declarative Pipelines**
- Pipeline config package with YAML-driven sub-pipeline definitions (ADR-0035)
- Pipeline-transitions with 3-part ref validation and uniqueness checks
- Pipeline-aware ops layer: claim, release, checkpoint, submit, BLOCKED escalation, sprint-terminal checks
- Auto-execute pipeline transitions in supervisor
- `liza proceed` command for inter-pair transitions with one-to-one cardinality enforcement
- `--config` and `--entry-point` flags for `liza init`
- Embedded spec-phase pipeline config with default `--config` flag
- Unknown fields rejected in pipeline config YAML (strict parsing)
- AvailableTransitions filtered to manual-trigger only

**Specification-Phase Roles**
- Epic-planner, epic-plan-reviewer, us-writer, us-reviewer, code-planner, code-plan-reviewer roles (ADR-0038)
- Code-planning pair data model with rejection loop to `DRAFT_CODING_PLAN`
- PLANNING_COMPLETE wake trigger for spec-to-coding transition
- Entry-point-aware initial planning prompts
- Single-intent task decomposition enforcement in spec pipeline

**Prompt Templates**
- Epic planner context template and builder
- Epic plan reviewer context template with 6 review gates
- US Writer prompt template with user-story-writing skill integration
- Output[] instructions in planner and reviewer templates
- Collective plan scoping for coder and reviewer prompts
- Tool guidance consolidated into `AGENT_TOOLS.md`
- Coder prompt: commit workflow with pre-commit recovery, forbid bare git commands and `git add -A`
- Concrete task ID rendered in base prompt with safe charset enforcement
- Worktree file path consistency guidance in doer templates (addresses Read/Edit path mismatches)
- Pre-submit self-check gating submission in code planning prompt
- Reviewer checklist aligned with planner output field-level identity

**Quality & Runtime**
- Rebase conflict detection at submission time → `INTEGRATION_FAILED` transition (ADR-0037)
- `liza_set_task_output` MCP tool for structured output entries (ADR-0036)
- Scope extensions in checkpoints and reviewer workflow (ADR-0036) enable negotiation between coder and reviewer
- Configurable `post_worktree_cmd` replacing hardcoded `syncEmbedded` (ADR-0031)
- Auto-checkpoint when all non-terminal planned tasks are BLOCKED
- Randomized task claiming with worktree pre-check for reviewers
- Task inspect exposes output, done_when, scope, spec_ref, rejection_reason
- ATTEMPT column (attempt.round) in `liza get tasks --format table` output
- `console.sh` embedded in binary and deployed on `liza init` for blackboard introspection
- E2e full sprint sequence test exercising epic-planning → US-writing → code-planning → coding

**Skills**
- **detailed-spec-writing** — transforms requirements into precise specifications using PRD format (ADR-0034)
- **user-story-writing** — transforms requirements into user stories with structured story format (ADR-0034)
- **liza-logs** — analysis tools for agent CLI logs (Claude, Codex, etc.) produced during multi-agent sprints
- **code-quality-assessment** — quantitative code quality assessment with star ratings, letter grades, refactoring priorities, and 5 analysis modes (Full, Targeted, Reassessment, Enrichment, Quick Health Check)

**Contracts**
- BREAKING CHANGE footer required in commit messages
- Project-specific guardrails via `GUARDRAILS.md` with tier system (ADR-0032)
- Secret word canaries to enforce full contract reading — agents recite collected words at session start to prove they read essential docs

---

## Fixes

| Fix | Impact |
|-----|--------|
| `rejected→executing` transition allowed in pipeline transition map | Unblocks rejection recovery in pipeline workflows |
| PostWorktreeCmd runs on rejection reclaims including same-coder | Consistent hook execution on all worktree paths |
| PostWorktreeCmd runs after worktree creation in claim and recovery | Hooks fire reliably in all entry paths |
| Missing worktree handled in MergeWorktree | Graceful error instead of panic |
| us-reviewer added to submit_verdict and wt_merge role checks | Spec-phase reviewers can submit verdicts and merge |
| Role checks broadened for pipeline agents, errors surfaced | Clear role error messages for new roles |
| Planning-task detection generalized beyond hardcoded code-planning-pair | Correct behavior for all pipeline types |
| Entry-point resolution hardened, redundant pipeline loads reduced | Faster startup, fewer false errors |
| PLANNING_COMPLETE restricted to code-planning-pair tasks | Prevents spurious planning transitions |
| Sprint-complete re-wake guarded after checkpoint/completed | No infinite re-wake loops |
| Working tree synced after update-ref in MergeWorktree | Merged content visible immediately |
| Pipeline config wired into IsClaimable, work detection, claiming | Pipeline-driven task routing works end-to-end |
| Pipeline-aware sprint-terminal checks wired to all callers | Correct sprint advancement for all pipeline types |
| TOCTOU race eliminated in AddTask duplicate check | No duplicate tasks under concurrent adds |
| Role authorization added to handleReleaseClaim | Prevents unauthorized claim releases |
| Transition name uniqueness validated across sub-pipelines | No ambiguous transition resolution |
| Liza MCP server registered for Codex sessions | Codex agents can use liza tools |
| Slash-delimited query split in liza_get handler | `liza_get tasks/T1` works correctly |
| Task claim released in resetAgentAfterExit before clearing CurrentTask | Clean state on agent exit |
| BaseCommit updated to rebase target on submit-for-review | Correct rebase base for review diffs |
| Fail fast on STOPPED before sprint branching in Resume | Clear error instead of partial state |
| Coder integration fix template simplified (89→30 lines) | Reduced coder context pressure |
| CLI, MCP, and ops made pipeline-aware for all spec-phase roles | Spec-phase roles (epic-planner, us-writer, etc.) no longer rejected |
| Access control added to 7 unprotected MCP handlers | State-mutating admin/worktree ops require role validation |
| Release-vs-claim worktree race condition eliminated | Concurrent release/claim no longer deletes freshly created worktrees |
| MCP server no longer exits on missing `.liza/` | Returns actionable error from handlers instead of crashing connection |
| wt-create made pipeline-aware for executing status check | Pipeline tasks (IMPLEMENTING_CODE, CODE_PLANNING) accepted by CreateWorktree |
| Agent ID patterns derived from `roles.AllRuntime()` | Spec-phase role prefixes auto-recognized in CLI |
| `spec_ref` normalized at write time to strip worktree prefixes | No worktree-relative paths persisted in state |
| Merged planning tasks with unconsumed output carried during sprint advance | Orchestrator no longer idles on pending planning output |
| BLOCKED tasks prevented from freezing sprint via checkpoint | Sprint doesn't stall when all planned tasks are blocked |
| Mechanical test updates clarified in refactoring task prompts | Agents stop conflating test modifications with behavioral changes |
| Timeout added to integration test execution in MergeWorktree | Hanging test scripts no longer block merge pipeline (10min limit) |
| Main working tree cleaned up after worktree merge | No stale synced files on non-integration branches |
| Worktree cleaned up when task is superseded | No orphaned worktrees on superseded tasks |
| Type assertion replaced with interface method for dependency recheck | Full encapsulation via `requiresDependencyRecheck()` interface method |
| `sync-embedded` removed from `check-embedded` Makefile target | CI catches master/embedded drift instead of masking it |

---

## Refactoring

- **Clean-code campaign** — 122 Go files across 26 batches: ops (6 batches), commands (8 batches), agent (2 batches), prompts, mcp, models, infrastructure, testhelpers, protocol, errors, log, filelock, statevalidate. Transformations: remove what-comments, extract helpers, flatten nested conditionals, eliminate pointer intermediates, use tagged switch, simplify nil checks, use `errors.Is` for sentinel comparison.
- **Planner → Orchestrator rename** (ADR-0033) — role constant, contracts, skills, docs, and all references updated in a single coordinated rename.
- **Tool guidance consolidation** — role-specific tool instructions merged into a single `AGENT_TOOLS.md`, referenced from all prompt templates.
- **Worktree merge refactoring** — merge logic cleaned up for pipeline-aware operation.
- **RoleStrategy pattern** (ADR-0041) — replaces 9-way switch/if-else chains in RunSupervisor with a `RoleStrategy` interface and 3 category implementations (doer, reviewer, orchestrator). Supervisor loop becomes role-agnostic. WaitConfig moved into strategy.
- **Claim task strategies** — ClaimTask's threaded claim-type booleans replaced with dedicated strategy implementations for fresh, rejection, and integration-fix claims. Interface method for dependency recheck replaces type assertion.
- **MCP middleware and declarative registration** (ADR-0043) — `withLogging`/`withRole` handler-level middleware eliminates repeated role validation across 15 handlers. Mutation and complex-operation tools registered from declarative metadata definitions.
- **File decomposition** — 6 monolithic files split into concern-based modules:
  - `cmd/liza/main.go` (1,462 LOC → 6 domain files: init, task, worktree, agent, review, system)
  - `internal/git/worktree.go` (591 LOC → 5 files: git, worktree, merge, rebase, query)
  - `internal/models/state.go` (938 LOC → 5 files: task, agent, sprint, config, history)
  - `internal/mcp/handlers.go` (918 LOC → 4 files: helpers, readonly, mutation, complex)
  - `internal/mcp/server.go` → protocol + registration files
  - `internal/statevalidate/validate.go` (658 LOC → 5 files: task, agent, deps, entity, sprint)
- **Event constants** — `TaskEventName` type with 26 constants replaces all raw string literals across ops, agent, commands, and statevalidate packages.
- **Orchestrator identity** — hardcoded "orchestrator-1" eliminated from 8 CLI call sites and 17 templates. `FindOrchestratorID` resolves from state; registration guard prevents dual orchestrators.
- **Legacy removal** — non-pipeline code paths (`Transition()`/`CanTransition()` dual-path, `cfg==nil` guards) removed. Pipeline config mandatory.
- **Naming alignment** — `review_deadlock` → `review_budget_exhausted`; `coder/code-reviewer` → `doer/reviewer` (ADR-0042); coding-pair constants aligned to pipeline state names (e.g. `READY` → `DRAFT_CODE`).
- **Wake trigger extraction** — 163-line wake trigger logic moved from builder.go to wake.go.
- **Render package** — shared CLI formatting (FormatJSON, FormatYAML, FormatValue) extracted into `internal/render`.
- **Dead code cleanup** — stale legacy comments, dead `cfg==nil` guard, and inlined single-use `initialTaskStatusWithResolver` removed.

---

## Tests

- Validate task edge case coverage — table-driven invariant tests for status field rules, completion requirements, integration fix history, parent task references, output completeness
- Artifact consistency checks — byte-compares repo masters (contracts, skills, settings) against embedded copies; `check-embedded` Makefile target wired into lint
- E2e full sprint sequence test — exercises complete supervisor pipeline (epic-planning → US-writing → code-planning → coding) with SmartMockCLIExecutor; `make test-e2e` target, ~40s, gated with `//go:build e2e`

---

## Documentation

- ADRs 0031–0043 backfilled (see below)
- C4 architecture diagrams (context, container, component, code-level views)
- Architecture review updated to pass 17 with data flow analysis lenses
- Code quality assessment updated for pass 2 refactors (grade: A)
- Sprint lifecycle and human gates section added to usage docs
- CLI reference completed
- Pipeline-configured `liza init` example
- LLM-assisted log analysis method documented
- Spec-phase sub-pipeline implementation plan
- User stories for clean-code batches (business logic, infrastructure, protocol)
- Codex liza-mcp config documentation
- MAS usage and demo instructions updated
- Contract reading hardened
- Skill activation docs corrected

---

## ADRs Added

| ADR | Title |
|-----|-------|
| 0031 | Configurable Post-Worktree Command |
| 0032 | Project-Specific Guardrails (GUARDRAILS.md) |
| 0033 | Planner → Orchestrator Role Rename |
| 0034 | Spec-Writing and User-Story-Writing Skills |
| 0035 | Declarative Sub-Pipelines |
| 0036 | Structured Task Output and Scope Extensions |
| 0037 | Rebase Conflict Detection at Submission |
| 0038 | Specification-Phase Roles |
| 0039 | MCP Role-Based Access Control |
| 0040 | Legacy Code Path Removal |
| 0041 | RoleStrategy Pattern |
| 0042 | Claim-Type Vocabulary (doer/reviewer) |
| 0043 | MCP Middleware and Declarative Registration |

---

## Installation

**Quick install (macOS/Linux):**
```bash
curl -fsSL https://raw.githubusercontent.com/liza-mas/liza/main/install.sh | bash
```

**From source:**
```bash
go install ./cmd/liza/ ./cmd/liza-mcp/
```

**Upgrade:**
```bash
liza setup        # Re-install updated contracts to ~/.liza/
```

---

## Known Limitations

- Terminal-first; no IDE integration or web UI
- Prompt changes require binary rebuild (no hot-reload)
- `liza setup` required before `liza init` — extra step for new users
- Spec-phase entry point selection relies on orchestrator auto-classification when `--entry-point` is not specified

---

## Resolved from v0.3.0

- ~~Specification phase pipeline~~ — Declarative sub-pipelines with spec-phase roles (epic-planner, epic-plan-reviewer, us-writer, us-reviewer, code-planner, code-plan-reviewer) (ADR-0035, ADR-0038)
- ~~Context handoff as blackboard event~~ — Structured task output via `liza_set_task_output` (ADR-0036)

---

## What's Next

- **Architecture role pair** — define from the specs the architecture to be used by the coders
- **Integration sub-pipeline** — validate a batch of commits so it can be safely merged to main
- **Context handoff as blackboard event** — structured positive/negative findings on every task completion
- **Sprint Analyzer** role — analyze all agent logs at end of sprint, use lesson-capture to capitalize on patterns and feed back into the planner's context
- **Deterministic pre/post hooks** at role transitions — mechanical checks before spawning agents and before their handoff
- **Planner-routed model selection** — assign tasks to models based on estimated complexity
