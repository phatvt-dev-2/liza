# Liza v0.5.0

Roles become declarative, reviews gain quorum:
- pipeline YAML roles replace hardcoded Go constants — type, timeouts, context-sections,
  allowed-operations, skills, and mandatory-docs are all configurable per role,
- composable prompt templates decompose 9 monolithic templates into 24 reusable blocks
  assembled via BuildRoleContext from pipeline YAML context-sections,
- review quorum enables multi-reviewer approval with impact-based overrides
  and best-effort provider-diversity enforcement,
- structured handoff events replace flat summary/nextAction with typed audit trails,
- multi-phase planning lets the orchestrator partition complex goals into sequential
  planning tasks with phase-gate dependency propagation and topo-sorted execution,
- a first-class attempt model replaces the dead Attempted field with structured lifecycle
  transitions, attempt-aware escalation, and watch monitoring,
- brownfield-safe initialization detects existing contract files and falls back
  to global config directories,
- and mechanical enforcement hooks (enforce-init.sh, git-guard) prevent agent drift
  without relying on compliance alone.

249 commits since v0.4.0 (2026-03-12).

---

## Breaking Changes

- **`agent-roles` pipeline YAML section removed** — absorbed into the declarative `roles` section. Existing pipeline.yaml files with `agent-roles` must be regenerated via `liza setup --force` or `liza init`.
- **Underscore role names eliminated** — `code_reviewer`, `code_planner`, etc. replaced by hyphenated `code-reviewer`, `code-planner` everywhere. Run `liza migrate` to normalize existing state.yaml files. Read paths auto-normalize for backward compatibility.
- **`State.Handoff` map removed** — replaced by `Task.HandoffEvents[]` array. The `liza_handoff` MCP tool now accepts structured fields (`succeeded`, `failed`, `hypothesis`, `key_files`, `dead_ends`) instead of flat `summary`/`next_action`.
- **Runtime role constants removed** — `roles.Runtime*` constants, `IsValidRuntime()`, `AllRuntime()`, `ToWorkflow()`, `ToRuntime()` deleted. All role references use single hyphenated form. Consumer code must use `pipeline.Resolver` for role queries.
- **Pipeline config mandatory for new fields** — `roles` section with `type` field required. `liza validate` detects underscore role names and directs to `liza migrate`.
- **`--log` flag removed from `liza agent`** — agent output logging is now enabled by default. Use `--no-log` to disable. Scripts passing `--log` must remove the flag.

---

## Highlights

- **Declarative Role Definitions** (Phases 1–4) — Roles are now first-class objects in pipeline YAML with type (doer/reviewer/orchestrator), display-name, timeouts, context-sections, allowed-operations, skills, and mandatory-docs. The ops layer, MCP authorization, supervisor strategy, and prompt builder all derive behavior from YAML instead of hardcoded constants. Custom roles (e.g. `data-engineer`) work without modifying Go code.
- **Composable Prompt Templates** (Phase 2) — 9 monolithic role-context templates decomposed into 24 modular block `.tmpl` files. `BuildRoleContext(role, sectionNames, data)` renders and concatenates blocks per the pipeline YAML context-sections list. Unified `RoleContextData` struct replaces 9 per-role config structs.
- **Review Quorum** (Phase 3) — Multi-reviewer approval with `Approvals[]` array, quorum states (`partially_approved`, `reviewing_2`), impact-based quorum overrides (standard/significant/architecture), and best-effort provider-diversity enforcement. Reviewers sharing the doer's provider are blocked when a different-provider reviewer is available.
- **Structured Handoff Events** — `HandoffEvent` struct with trigger constants (`context_exhaustion`, `submission`, `completion`), structured fields (`succeeded`, `failed`, `hypothesis`, `next_step`, `key_files`, `dead_ends`), and validation for post-submission/merged tasks.
- **Multi-Phase Planning** — Orchestrator can create N sequential planning tasks for complex goals. Phase-gate dependency propagation ensures downstream phase children depend on all upstream phase children. ExecuteAvailableTransitions uses topological sort (Kahn's algorithm) with crash recovery and precise cycle detection (Tarjan's SCC distinguishes true cycle members from downstream-of-cycle tasks).
- **First-Class Attempt Model** — `Attempt` int field replaces dead `Attempted []string`. `TransitionToNewAttempt()` implements a 3-phase operation (sentinel → limit reset → status reset) with sentinel claimability guard. Attempt-aware escalation in `SubmitVerdict` and `ClaimTask`. Watch command updated for attempt-aware monitoring.
- **Brownfield-Safe Initialization** — `liza init` detects existing non-Liza CLAUDE.md/AGENTS.md/GEMINI.md and places contract symlinks in the CLI's global config directory instead of overwriting.

---

## Features

**Declarative Roles (Pipeline YAML)**
- `RoleDef` and `TimeoutDef` structs with type, display-name, timeouts, context-sections, allowed-operations, skills, mandatory-docs
- Resolver methods: `RoleType`, `DoerRoleNames`, `ReviewerRoleNames`, `AllRoleNames`, `AllowedOperations`, `RoleTimeouts`, `ContextSections`, `Skills`, `MandatoryDocs`, `RoleDisplayName`
- Timeout resolution hierarchy: state.yaml config > YAML role values > type defaults
- YAML-driven MCP authorization replacing hardcoded role checks (`operationChecker` + `typeChecker`)
- `LoadFrozen` patches missing allowed-operations from embedded config into older frozen configs
- Orchestrator singularity enforced by resolved type, not literal role key
- `liza migrate` CLI command for state.yaml role name normalization
- `liza validate` detects unmigrated underscore role names
- Read-path normalization via `NormalizeRoleName` lookup table

**Composable Prompts**
- 24 block templates in `templates/blocks/` composable via pipeline YAML context-sections
- `BuildRoleContext(role, sectionNames, data)` renders and concatenates block templates
- Unified `RoleContextData` struct with identity, task, review, plan, coder-specific, orchestrator-specific fields
- `mandatory_docs.tmpl` and `skills_affinity.tmpl` block templates
- Worktree-rules rendered before read-bearing blocks (regression guard test)
- Fixture-matches-embedded-pipeline sync test

**Review Quorum**
- `Approval` struct (Agent, Provider, Timestamp) with helpers: `ApprovalCount`, `HasProviderDiversity`, `ClearApprovals`, `LastApprover`
- Review-policy schema with base quorum and impact overrides (significant-change, architecture-impact)
- Quorum states: `partially_approved` → `reviewing_2` → `approved`/`rejected`
- `SubmitVerdict` evaluates effective quorum using resolved impact; insufficient quorum transitions to `partially_approved`
- `ResolveEffectiveImpact` scans checkpoint/verdict history since last rejection
- Stale review claim clearing extended for `reviewing_2` → `partially_approved`
- Claim priority: `partially_approved` preferred over `submitted`
- Provider-diversity soft preference for candidate selection (fresh submissions and partial approvals)
- Best-effort provider-diversity merge gate with diversity Extra fields in merge history
- Impact field on `write-checkpoint` operation (standard/significant/architecture)
- Reviewer diagnostics for quorum states
- Doer-provider diversity blocking at claim time when `provider-diversity: preferred` configured

**Structured Handoff Events**
- `HandoffEvent` struct with Timestamp, Agent, Trigger, Succeeded, Failed, Hypothesis, NextStep, KeyFiles, DeadEnds
- Trigger constants: `context_exhaustion`, `submission`, `completion`
- Events written on: `ops.Handoff()`, `SubmitForReview`, `MergeWorktree`
- `liza_handoff` MCP tool extended with structured fields (backward-compatible)
- `State.Handoff` map removed; prompt builder reads from `task.HandoffEvents`
- State validation: handoff event field completeness, required events per task status

**Multi-Phase Planning**
- Multi-phase planning guidance in orchestrator wake template
- `computeInheritedDeps` derives phase-gate dependencies from upstream transitions
- `ExecuteAvailableTransitions` restructured: collect → topo sort (Kahn's) → execute
- Crash recovery via phase 1b (patches missing inherited deps on existing children)
- Cycle detection via Tarjan's SCC with per-component `transition_cycle_blocked` events and dynamic downstream blocking via `HasCycleBlockedDependency`
- Phase-consistency blocking rule for multi-phase planners
- `liza replan` command with DependsOn preservation and downstream retargeting
- Shared child ID derivation functions (`perSubtaskChildID`, `oneToOneChildID`)

**Attempt Model**
- `Attempt` int field with `EffectiveAttempt()` helper, `Attempted []string` removed
- Sentinel claimability guard in `IsClaimable` and `ClaimTask`
- `TaskEventNewAttempt` history event constant
- `TransitionToNewAttempt` 3-phase operation (sentinel → limits → status)
- Attempt-aware `classifyLimitEscalation` with `LimitAction` return type
- Wired into `SubmitVerdict` (review cap) and `ClaimTask` (iteration cap)
- `rejectedClaimStrategy` simplified: identity-based branching removed
- Attempt field validation rules in `statevalidate`
- Watch: `checkStaleSentinels` for stuck transition detection, attempt context in `checkApproachingLimits`, sentinel exemption from `checkOrphanedRejected`
- Prompt LIMITS text updated for fresh attempt model

**New Commands & Tools**
- `liza cancel-task` / `liza_cancel_task` — transition tasks to ABANDONED with reason
- `liza replan` — re-invoke planner after plan amendment at CHECKPOINT
- `liza migrate` — normalize underscore role names to hyphenated form
- `liza assess-hypothesis-exhausted` / `liza_assess_hypothesis_exhausted` — prevent orchestrator re-wake loops for hypothesis-exhausted tasks
- `liza_set_discovery_disposition` — set converted_to_task on discovery entries (replaces last state.yaml edit exception)
- Auto-assign agent ID when `--agent-id` omitted (retry on collision)

**Initialization & Setup**
- Brownfield-safe symlinks with global config fallback for existing CLAUDE.md/AGENTS.md/GEMINI.md
- Unified agent flag handling (`--claude`, `--codex`, `--gemini`, `--mistral`) for both init and setup
- Agent startup warning when no contract symlink found
- Auto-suggest `post_worktree_cmd` for Node.js projects (detects package.json + lockfile)
- `--agent-tools` flag for `liza setup` to install custom AGENT_TOOLS.md
- Windows symlink failure messages with Developer Mode guidance
- `liza setup --force` updates stale pipeline.yaml with backup

**Agent Reliability**
- enforce-init.sh PreToolUse hook mechanically blocks non-Read tools until mandatory docs read; sentinel-file tracking (race-condition-free), fail-open on mkdir failure
- git-guard hook blocks force push, hard reset, clean -f in agent mode
- Prompts prohibit direct state.yaml edits by agents
- `review_commit` invariant: reject reviewer claims when ReviewCommit is nil
- recover-task detects and resets corrupted tasks to initial status
- `plan_ref` propagation and `validation_plan` surfaced to reviewers
- Spec compliance matrix required in code-planner plans
- Scope extensions guidance for out-of-scope files
- Reviewer worktree immutability (no stash, reset, etc.)
- Check-before-act gate for orchestrator supersede workflows

**Prompt Improvements**
- MCP tool name resolution hints (ToolSearch select: queries) per role
- Skill invocation via Skill tool instead of hardcoded file paths
- Standardized plan file output path (`specs/plans/<task-id>.md`) with timestamp prefix
- Shared-file dependency rule for code planner decomposition
- Git commit heredoc pattern (`git commit -F -`) for multi-line messages
- Output[] parity checks strengthened for planners
- FORBIDDEN items made role-conditional
- Block boundary normalization in BuildRoleContext

**Other Features**
- `TaskTypePlanning` for non-coding task types (TDD gate auto-exempt)
- Provider metadata persisted on agent registration, visible in `liza_get agents/<id>`
- Pipeline transitions gated on planning checkpoint acknowledgment (two-wake model)
- `LIZA_AGENT_ID` exported to child processes
- Superseded task branches preserved for successor access
- `DependsOn` field on `OutputEntry` for inter-task dependencies
- Console dashboard three-column layout
- `gocover-cobertura` pinned as project tool dependency
- Agent output logging enabled by default (`--no-log` to disable); `-i` mode implicitly disables logging
- Secret values masked in persisted agent logs

---

## Fixes

| Fix | Impact |
|-----|--------|
| Prompt piped via stdin instead of CLI arg | Eliminates Windows 32K ARG_MAX limit for 4/5 CLIs |
| Sibling task descriptions truncated to 200 chars | Prevents ARG_MAX on large sprints (137+ tasks) |
| Task lease renewed alongside agent lease in heartbeat | Prevents task theft from live agents after 30min |
| Concurrent worktree corruption via targeted metadata cleanup | Eliminates cross-worktree interference from global prune |
| Per-task claim cooldown (60s window) | Prevents reviewer spin loops (26 events/15s observed) |
| Actionable MCP error messages via `OperationalError` type | Stops blind agent retries (14 turns, ~722K tokens wasted) |
| `IntegrationFailedError` classified in MCP handler | Agents get recovery guidance instead of "internal error" |
| `ReleaseAgent` guarded against stale task references | Prevents blowing active agents to IDLE on merge |
| `liza_get` format "text" normalized to "value" | Fixes -32002 validation errors (36 agent occurrences) |
| ~70 ops validation errors converted to typed `PreconditionError` | Agents see specific messages instead of generic "validation failed" |
| Orchestrator assess-blocked actionability filtering | Prevents $16 re-wake burn on single blocked task |
| Hypothesis-exhausted actionability filtering | Prevents repeated wake for already-triaged tasks |
| Worktree-rules rendered before read-bearing blocks | Prevents agent drift out of worktree |
| Lease refreshed on system-initiated state transitions | Agents keep tasks during INTEGRATION_FAILED/REJECTED recovery |
| ToolSearch deferred to after initialization | Eliminates 42% session init failures (33/79 hook blocks) |
| Secret words detected in early text blocks (up to 5) | Correct detection after enforce-init.sh retries |
| spec_ref/plan_ref semantics corrected for epic planner | Downstream agents retain goal spec traceability |
| Default max-wait increased from 30min to 2 hours | Prevents premature supervisor exits |
| Git fallback for spec/plan ref validation on integration branch | Files merged to integration branch pass validation |
| Pipeline fixture sync fixture with prior-attempt change | Test consistency after attempt model changes |
| Quorum requires fail-hard for missing partially-approved state | Defense-in-depth against config bypass |
| ApprovedBy cleared on rejection for approvals[] consistency | Prevents stale legacy field after approval list cleared |
| Skill files use absolute paths | Skills resolve correctly on user projects |
| Only prepend frontmatter to Markdown files | Prevents shebang corruption in .py/.sh scripts |
| Watch mutations removed (read-only monitoring) | Eliminates infinite resume→checkpoint→resume loop |
| npm/npx added to default agent permissions | JS project agents can run tests |
| `Bash(git:*)` permission covers all git flag patterns | Eliminates ~600+ permission blocks per sprint |
| Parallel tool-call cascade from permission denial | `Bash(test:*)` allowed, two-step pattern documented |
| State.yaml edit prohibition enforced in contract and prompts | Prevents agent corruption via direct state surgery |
| Precise cycle detection with Tarjan's SCC replacing Kahn's leftover set | Downstream-of-cycle tasks no longer incorrectly marked as cycle members; independent cycles no longer merged |
| Heartbeat moved from per-CLI-execution to supervisor lifetime | IDLE agents no longer lose lease during wait-for-work, preventing auto-assigned ID collision with new agents |
| enforce-init.sh race condition: sed+mv replaced with atomic sentinel files | Concurrent hook invocations no longer corrupt shared state file |
| enforce-init.sh fail-open on mkdir failure | Hook logs diagnostic and exits 0 instead of permanently blocking all tools with misleading errors |
| Test timing margins widened for CI stability | Lease expiry boundary buffer 1ms→100ms, waitforwork timeout 200ms→2s |

---

## Refactoring

- **Dual Name Elimination** (Phase 4) — `ToWorkflow`/`ToRuntime` removed, all constants hyphenated, `workflowRole` field removed from strategy layer, `ClaimReviewerTaskInput.WorkflowRole` → `Role`.
- **Agent-roles absorption** — `AgentRoles` field removed from `Pipeline` struct; validation and display name lookups via `Resolver` methods.
- **Role strategy via resolver** — 9-way role name switch replaced with 3-way type dispatch via `resolver.RoleType()`.
- **Per-role builder removal** — 9 `Build*Context()` functions, 9 `*ContextConfig` structs, 9 `*ContextData` structs, 9 monolithic `*_context.tmpl` files deleted.
- **Hardcoded constants replaced** — `Runtime*` constants, `IsDoerRole`/`IsReviewerRole`, `DoerRoles`/`ReviewerRoles` all replaced with resolver-based queries.
- **Impact validation deduplicated** — `IsValidImpact` derived from single `impactOrder` source.
- **Merge loop resolver hoist** — pipeline YAML loaded once per cycle instead of 2N disk reads.
- **Child ID derivation extracted** — `perSubtaskChildID`/`oneToOneChildID` shared functions replace inline `fmt.Sprintf`.
- **Embedded config mastering** — `claude-settings.json` and `mcp.json` mastered directly in `internal/embedded/`, eliminating repo-root copies and `sync-embedded` for these files.
- **Epic methodology delegation** — inline epic planner guidance replaced with skill reference, matching US Writer pattern.

---

## Tests

- Orchestrator singularity test for misconfigured max-instances (resolver coerces to 1)
- `TestResolver_WorktreeRulesBeforeReadBearingBlocks` ordering invariant across all roles
- `TestResolver_FixtureMatchesEmbeddedPipeline` prevents silent test/production drift
- Quorum diagnostic scenarios including mixed states and stale reviewing_2 leases
- Fresh-submission diversity tests rewritten with mock resolver for meaningful exercise
- HandoffEvent YAML round-trip serialization and Task-level serialization
- 19 handoff event validation tests (valid, missing-field, missing-event)
- EAT crash recovery patches existing children with inherited deps
- Tarjan's SCC cycle detection: independent cycles correctly separated, downstream-of-cycle tasks excluded
- Sprint advance carries forward cycle-blocked planning tasks
- `checkStaleSentinels` repeated-alert subtest for stuck transitions
- TransitionToNewAttempt failure propagation test
- AttemptNum uses EffectiveAttempt in buildTaskRoleContextData
- Full sprint e2e test drives real checkpoint-resume-advance flow

---

## Documentation

- ADRs 0044–0051 backfilled (Waves 5–6)
- ADR index (`specs/architecture/ADR/README.md`) with linked table of all ADRs
- Declarative Role Definitions spec (`3 - Declarative Role Definitions.md`)
- Sprint retrospectives spec
- Multi-phase planning specs (sub-pipelines, state machines, task lifecycle, sprint governance, blackboard schema)
- Attempt model specs (vision, blackboard-schema, task-lifecycle, roles)
- Code quality reassessment (A → A- after 50 feature commits)
- Hardening measures inventory (`docs/liza-hardened-mas.md`) — defense-in-depth architecture across five enforcement layers with industry context analysis
- Adversarial architecture review (INVARIANTS.md vs code reality)
- System invariants reference (`INVARIANTS.md`) with cross-reference protection matrix
- Replan workflow documented in usage guide
- Stale status names updated in troubleshooting/recipes/configuration
- Agent lessons: large test file reads, settings master editing, worktree path consistency
- Contract and prompt conciseness guardrail (G2.2)
- Structured handoff fields documented in agent prompts
- Parallel tool-call guard in AGENT_TOOLS.md
- Attempt model realignment plan and spec reconciliation with multi-phase planning

---

## ADRs Added

| ADR | Title |
|-----|-------|
| 0044 | Task Event Constants |
| 0045 | Declarative Role Definitions in Pipeline YAML |
| 0046 | Review Quorum |
| 0047 | Dual Name Elimination |
| 0048 | Multi-Phase Planning |
| 0049 | Structured Handoff Events |
| 0050 | Brownfield-Safe Initialization |
| 0051 | First-Class Attempt Model |

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
liza setup --force   # Re-install updated contracts and pipeline config
liza migrate         # Normalize underscore role names in existing state
```

---

## Known Limitations

- Terminal-first; no IDE integration or web UI
- Prompt changes require binary rebuild (no hot-reload)
- `liza setup` required before `liza init` — extra step for new users
- `TaskTypePlanning` only covers code-planning workflow; other planning types default to `TaskTypeCoding`
- Quorum review states only defined for coding-pair; spec-phase role-pairs use quorum: 1

---

## Resolved from v0.4.0

- ~~Context handoff as blackboard event~~ — Structured `HandoffEvent` struct with typed trigger constants and structured fields
- ~~Deterministic pre/post hooks at role transitions~~ — enforce-init.sh PreToolUse hook and git-guard hook deployed via `liza init`

---

## What's Next

- **Architecture role pair** — define from the specs the architecture to be used by the coders
- **Integration sub-pipeline** — validate a batch of commits so it can be safely merged to main
- **Sprint Analyzer** role — analyze all agent logs at end of sprint, use lesson-capture to capitalize on patterns
- **Planner-routed model selection** — assign tasks to models based on estimated complexity
- **Phase 2 composable prompt enhancements** — content composition (not just section selection) from pipeline YAML
