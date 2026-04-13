# Code Quality Assessment and Refactoring Roadmap

* Date: 2026-04-13 (commit 672a7d95) *(reassessment 2026-04-13)*
* Previous: 2026-03-24 (commit e94eb6f) — 50 commits since
* Repository: liza
* Author: Claude Code - Opus 4.6
* Mode: Reassessment (after 2026-03-24 / e94eb6f)

## Repository Metrics Dashboard

| Metric | Previous | Current | Delta |
|--------|----------|---------|-------|
| Production LOC | 28,201 (200 files) | 32,788 (195 files) | +4,587 (+16%), -5 files |
| Test LOC | 75,737 | 86,153 (166 files) | +10,416 (+14%) |
| Test-to-production ratio | 2.69:1 | 2.63:1 | -0.06 |
| Test functions | 1,327 | 1,649 | +322 (+24%) |
| Files >500 LOC | 9 | 12 (10 non-declarative) | +3 |
| Non-decl functions >150 LOC | 9 | 16 | +7 |
| Specifications | 130 (51 ADRs) | 192 (60 ADRs) | +62 (+9 ADRs) |
| Contracts | 11 | 9 | -2 (MCP removal) |
| Skills | 23 | 22 | -1 |
| Documentation | 24 guides | 36 guides | +12 |
| Lessons | 5 | 6 | +1 |
| Dependencies | 4 direct | 9 direct | +5 (Charmbracelet TUI) |
| Code Hygiene | Zero across all | Zero across all | — |

**Dependencies**: cobra, yaml.v3, flock, fsnotify — unchanged; +bubbles, bubbletea, huh, lipgloss, go-runewidth (Charmbracelet TUI stack) *(reassessment 2026-04-13)*
**CI/CD**: Multi-platform (Linux + macOS), Codecov integration, 23 pre-commit hooks, E2E tests in CI — unchanged
**Code Hygiene**: Zero TODOs, zero `nolint` directives, zero `panic()`, zero `interface{}` in production Go code; statuses, roles, and event names are typed constants — maintained through 50 more commits *(reassessment 2026-04-13)*

**Note on file count**: Production files decreased 200→195 despite +4,587 LOC because the MCP server removal (ADR-0057) deleted ~15 files while new subsystems added ~10. Net production growth accounting for MCP removal is ~7,269 LOC of new code. *(reassessment 2026-04-13)*

## Executive Summary

Liza is a hybrid multi-agent coding orchestrator: Go-based deterministic supervisors enforce invariants while LLM agents handle judgment. The codebase demonstrates **exceptional engineering discipline** in its core runtime — minimal dependencies, comprehensive testing, atomic state management — combined with an unusually thorough specification and contract corpus that forms the product's core IP.

The most significant architectural change this period was the **removal of the MCP server** (ADR-0057), eliminating a redundant parallel surface that caused reliability issues under memory pressure. This was replaced by CLI-native `--json` mode and `--agent-id` RBAC — a genuine simplification that reduced maintenance burden and agent context overhead.

**Key Strengths:**
- **Architectural courage**: MCP server removal deleted ~8,500 LOC (prod + test) of working but redundant code, replacing it with ~425 LOC of CLI-native access control. The decision was documented in ADR-0057 with clear rationale *(reassessment 2026-04-13)*
- **Test discipline holding under growth**: 2.63:1 test-to-production ratio (was 2.69:1), with +10K test LOC tracking +4.6K production LOC across 50 commits of feature work (TUI, integration pipeline, quota detection, many-to-one transitions) *(reassessment 2026-04-13)*
- **Pristine code hygiene**: Zero TODOs, zero `nolint`, zero `panic()`, zero untyped code in production — maintained through 50 commits including a major subsystem removal and TUI addition *(reassessment 2026-04-13)*
- **Radical dependency minimalism** (qualified): 9 direct dependencies, up from 4. The 5 new deps are a single cohesive TUI framework (Charmbracelet). Still exceptional for Go ecosystem. *(reassessment 2026-04-13)*
- **Specification-driven development**: 192 spec files + 60 ADRs — extraordinary traceability *(reassessment 2026-04-13)*

**Areas for Improvement:**
- **Structural complexity acceleration unaddressed**: P1 decomposition recommendations from prior assessment (proceed.go, init.go) were not executed while those files grew further — proceed.go hit 1,200 LOC (+47%), init.go hit 854 (+13%). 12 files now exceed 500 LOC (was 9). 16 non-declarative functions exceed 150 LOC (was 9). The structural debt is compounding *(reassessment 2026-04-13)*
- **supervisor.go approaching god-file territory**: 588→831 LOC (+41%) mixing restart tracking (exit-42, crash, spinning), CLI execution, and the supervisor loop *(reassessment 2026-04-13)*
- **Coverage reporting gap**: Codecov configured but coverage threshold not enforced in CI — unchanged
- **Test ratio slight decline**: 2.69→2.63 — production LOC grew slightly faster than test LOC for the first time *(reassessment 2026-04-13)*

**Overall Rating: A- (Excellent with concerns)** *(unchanged — reassessment 2026-04-13)*

The A- holds because architectural simplification (MCP removal), code hygiene, and test discipline counterbalance the structural complexity growth. The deduction from A is strengthened: files >500 LOC grew 9→12, functions >150 LOC nearly doubled (9→16), and all three P1 decomposition targets from the prior assessment grew further without being addressed. A further assessment without P1 progress would warrant a downgrade to B+.

---

## Detailed Subsystem Analysis

### State Machine & Models (`internal/models/`) ★★★★★

**Strengths:**
- **Explicit state machine**: Task states with pipeline-driven transitions via `TransitionWith()` — no implicit state changes possible
- **Pipeline-driven extensibility**: Custom state names via YAML pipeline config with `Resolver` providing runtime query interface
- **Complete audit trail**: Every task mutation appended to `History[]` with timestamps and actor IDs
- **Lease-based concurrency**: Time-bounded claims with stale detection prevent zombie agents
- **Thorough model tests**: 3,344 test LOC for 1,411 production LOC (2.37:1) *(reassessment 2026-04-13)*
- **31 focused methods on Task**: task.go at 558 LOC but averaging 18 LOC per function — appropriate domain model density, not god-class behavior *(reassessment 2026-04-13)*

**Concerns:**
- None at this time. task.go exceeds 500 LOC nominally but the 31 small methods are well-factored domain logic.

### Operations Layer (`internal/ops/`) ★★★★☆

**Strengths:**
- **Clean service layer**: Each operation is its own file with focused responsibility — now 49 production files
- **Precondition-heavy design**: Operations validate extensively before mutating, failing fast with typed errors
- **Strong test ratio**: 24,824 test LOC for 9,917 production LOC (2.50:1) *(reassessment 2026-04-13)*
- **Claim strategy pattern**: `claimStrategy` interface eliminates boolean-flag dispatch
- **Typed event constants**: `TaskEventName` typed constants used across all packages
- **Integration pipeline**: many-to-one transitions, architecture step, cohort management *(reassessment 2026-04-13)*

**Concerns:**
- `proceed.go` grew from 816→1,200 LOC (+47%) — **the largest production file in the codebase by a wide margin**. Contains 26 functions spanning: transition execution, many-to-one cohort management, phase-gate dependency propagation, topo-sorted execution, crash recovery, cycle detection (SCC algorithm), and child task construction. P1.8 decomposition was recommended in the prior assessment and not addressed *(reassessment 2026-04-13)*
- `claim_task.go` (563 LOC) and `wt_merge.go` (520 LOC) both exceed 500 LOC — stable but not improving
- 7 ops functions exceed 150 LOC: `InitCommandWithConfig` (315), `SubmitVerdict` (309), `MergeWorktree` (294), `ClaimTask` (282), `SubmitForReview` (221), `RecoverTask` (221), `ExecuteAvailableTransitions` (210) *(reassessment 2026-04-13)*

### Agent Supervision (`internal/agent/`) ★★★★☆

**Strengths:**
- **Deterministic supervisor**: Go process wraps LLM agent, enforcing restart limits, heartbeat, lease renewal
- **Exit code 42 protocol**: Clean restart mechanism with exponential backoff tracking
- **Strategy pattern**: Role-specific behavior cleanly separated into strategy files — each with `WaitConfig`
- **Strong test ratio**: 11,454 test LOC for 3,874 production LOC (2.96:1) *(reassessment 2026-04-13)*
- **Quota detection**: New `quota.go` (160 LOC) detects provider quota exhaustion *(reassessment 2026-04-13)*
- **System control**: New `systemctl.go` (311 LOC) for system start/stop/pause/resume *(reassessment 2026-04-13)*

**Concerns:**
- `supervisor.go` grew from 588→831 LOC (+41%) with 24 functions. Now mixes: exit-42 restart tracking, crash restart tracking, spinning detection, CLI execution (`DefaultCLIExecutor`), and the supervisor loop (`RunSupervisor` at 287 LOC, was 171). These are distinct responsibilities that share minimal state *(reassessment 2026-04-13)*
- The restart tracking subsystem alone (exit42, crash, spinning trackers) accounts for ~300 LOC and is self-contained — extraction would be straightforward *(reassessment 2026-04-13)*

### Terminal UI (`internal/tui/`) ★★★★☆ *(new subsystem — reassessment 2026-04-13)*

**Strengths:**
- **Charmbracelet bubbletea architecture**: Clean Model-Update-View separation following the Elm pattern
- **Well-organized**: 6 files with clear responsibilities — model.go (types), view.go (rendering), update.go (state transitions), commands.go (side effects), styles.go (theming), keymap.go (bindings)
- **Tested**: 3,872 test LOC for 2,265 production LOC (1.71:1) with tests covering view rendering and update logic

**Concerns:**
- `view.go` (688 LOC) and `update.go` (643 LOC) both exceed 500 LOC already. For a new subsystem, this is early-stage structural debt — the same trajectory that other subsystems followed before becoming decomposition candidates
- 1.71:1 test ratio is below project average (2.63:1), though reasonable for a UI layer
- `renderTaskPanel` (206 LOC) and `Update` (164 LOC) are the two largest functions

### Git Operations (`internal/git/`) ★★★★☆

**Strengths:**
- **Merge-tree strategy**: Merges without touching the working directory — prevents dirty-state conflicts
- **Atomic ref updates**: Compare-and-swap on git refs prevents concurrent merge races
- **Concern-based file organization**: `worktree.go` (CRUD), `merge.go`, `rebase.go`, `query.go`, `git.go` (helpers)

**Concerns:**
- `merge.go` at 314 LOC — within acceptable range but above the 300 LOC attention threshold

### State Validation (`internal/statevalidate/`) ★★★★★

**Strengths:**
- **Comprehensive validation rules**: Every state mutation runs through extensive checks
- **Rule separation from ops**: Validation is a distinct package, not mixed into business logic
- **Pipeline-aware validation**: Rules adapt to custom pipeline states
- **Improved test ratio**: 1,844 test LOC for 903 production LOC (2.04:1) *(reassessment 2026-04-13)*

**Concerns:**
- None at this time

### CLI Entry Point (`cmd/liza/`) ★★★★☆

**Strengths:**
- **Thin delegation**: Each command's `RunE` averages 5-15 lines — parses flags and delegates to `commands` or `ops`
- **CLI-native RBAC**: `--agent-id` validation on state-mutating commands replaces MCP RBAC (ADR-0057) *(reassessment 2026-04-13)*
- **JSON output wiring**: `--json` flag on read commands for structured output *(reassessment 2026-04-13)*

**Concerns:**
- `cmd_task.go` (910 LOC) and `cmd_system.go` (683 LOC) contain monolithic `init()` functions with all Cobra command registrations — declarative data, not control flow, so LOC is structural rather than concerning
- The `init()` registration blocks must be maintained in sync with command definitions — no compile-time enforcement prevents a command from being defined but not registered

### CLI Commands (`internal/commands/`) ★★★★☆

**Strengths:**
- **Thin command layer**: Commands delegate to ops, never contain business logic
- **Exceptional test ratio**: 19,521 test LOC for 5,297 production LOC (3.68:1) *(reassessment 2026-04-13)*
- **Inspect subsystem**: Well-factored reporting with 5 dedicated inspect files (tasks, agents, metrics, anomalies, field) *(reassessment 2026-04-13)*
- **Setup command**: New `setup.go` (403 LOC) for post-init configuration *(reassessment 2026-04-13)*

**Concerns:**
- `init.go` grew from 753→854 LOC (+13%) with `InitCommandWithConfig` at 315 LOC (was 294). P1.9 not addressed *(reassessment 2026-04-13)*
- `watch.go` grew from 751→846 LOC (+13%). P2.8 not addressed *(reassessment 2026-04-13)*

### Pipeline Configuration (`internal/pipeline/`) ★★★★☆

**Strengths:**
- **Declarative pipeline definition**: YAML-driven state machine configuration with role-pair schemas
- **Improved test ratio**: 3,613 test LOC for 1,167 production LOC (3.10:1, was 2.75:1) *(reassessment 2026-04-13)*
- **Migration support**: `MigrateOperations` (146 LOC) for pipeline config evolution *(reassessment 2026-04-13)*

**Concerns:**
- `resolver.go` at 642 LOC (was 571, +12%) — approaching the point where concern separation would help readability
- `config.go` at 488 LOC — new file nearing 500 LOC threshold *(reassessment 2026-04-13)*

### Prompt Building (`internal/prompts/`) ★★★★☆

**Strengths:**
- **Template-based agent initialization**: Structured bootstrap prompts with embedded contracts, role definitions, and task context
- **Strong test ratio**: 2,524 test LOC for 600 production LOC (4.21:1, was 3.70:1) *(reassessment 2026-04-13)*
- **Provider-aware naming**: MCP tool naming adapts to CLI provider *(reassessment 2026-04-13)*

**Concerns:**
- None at this time

### Supporting Infrastructure

Focused infrastructure packages maintain the project's quality discipline:

| Package | Prod LOC | Test LOC | Ratio | Purpose |
|---------|----------|----------|-------|---------|
| `internal/testhelpers/` | 784 | 1,031 | 1.32:1 | Test fixtures and helpers |
| `internal/db/` | 531 | 2,566 | 4.83:1 | Blackboard state persistence *(reassessment 2026-04-13)* |
| `internal/filelock/` | 489 | 884 | 1.81:1 | File locking with metrics and typed errors |
| `internal/paths/` | 346 | 522 | 1.51:1 | Path management *(new — reassessment 2026-04-13)* |
| `internal/interactive/` | 238 | 109 | 0.46:1 | Init wizard *(new — reassessment 2026-04-13)* ⚠️ |
| `internal/analysis/` | 224 | 404 | 1.80:1 | Pattern analysis *(new — reassessment 2026-04-13)* |
| `internal/log/` | 211 | 678 | 3.21:1 | Structured agent logging |
| `internal/render/` | 175 | 234 | 1.34:1 | Shared CLI formatting |
| `internal/jsonout/` | 149 | 275 | 1.85:1 | JSON output *(new — reassessment 2026-04-13)* |
| `internal/identity/` | 123 | 473 | 3.85:1 | Orchestrator identity resolution |
| `internal/roles/` | 90 | 160 | 1.78:1 | Role type definitions and validation |
| `internal/errors/` | 70 | 121 | 1.73:1 | Shared error types |
| `internal/process/` | 67 | 0 | — | Process attributes *(new, no tests — reassessment 2026-04-13)* ⚠️ |
| `internal/gitenv/` | 28 | 0 | — | Git locale env *(new, no tests — reassessment 2026-04-13)* ⚠️ |

⚠️ `interactive/` (0.46:1 ratio), `process/` (no tests), and `gitenv/` (no tests) break the project's testing pattern. All are small but noteworthy. *(reassessment 2026-04-13)*

---

## Behavioral Contracts & Skills ★★★★★

**Strengths:**
- **Failure-mode-driven design**: 55+ documented LLM failure modes mapped to specific countermeasures in `CONTRACT_FAILURE_MODE_MAP.md`
- **Tiered rule architecture**: Tier 0 (inviolable) through Tier 3 (preferences) with explicit degradation protocol
- **Execution state machine**: States with forbidden transitions, model activation points, and stop triggers
- **22 composable skills**: Domain-specific protocols that agents load on demand *(reassessment 2026-04-13)*
- **Three collaboration modes**: Pairing (human-supervised), Multi-Agent (peer-supervised), Subagent (delegated) — each with explicit gate semantics
- **Anti-gaming clause**: "Technically compliant is not compliant" — closes the most common loophole in agent governance

**Concerns:**
- Contract documents are necessarily large (CORE.md at 750 lines) — agents must read them fully, consuming context window budget
- Skills lack versioning — when a skill protocol changes, all sessions see the new version immediately

---

## Testing & Quality Infrastructure ★★★★★

**Strengths:**
- **2.63:1 test-to-production ratio** — slight decline from 2.69:1, first time production LOC grew faster than test LOC, but still excellent for Go ecosystem *(reassessment 2026-04-13)*
- **1,649 test functions** (was 1,327) — +322 test functions tracking feature growth *(reassessment 2026-04-13)*
- **Pure standard library testing**: No external test frameworks — reduces dependency surface
- **Table-driven tests throughout**: Files use `t.Run()` subtests with structured test cases
- **Race detection enabled by default**: `-race` flag in all CI runs
- **Test quality enforcement**:
  - `parallel_usage_test.go`: Ratcheting minimum for `t.Parallel()` calls
  - `sleep_usage_test.go`: Prevents `time.Sleep` in tests — enforces real concurrency patterns
  - `check-testhelpers`: Pre-commit hook ensures test utilities don't leak into production
- **Integration tests expanded**: 7 E2E test files covering concurrent operations, lease expiry, sprint management, await workflows, and full end-to-end pipelines *(reassessment 2026-04-13)*

**Concerns:**
- No coverage threshold enforced in CI — Codecov is configured for reporting only
- No mutation testing or fuzz testing
- Three new packages (`process/`, `gitenv/`, `interactive/`) break the project's testing pattern *(reassessment 2026-04-13)*

---

## Pre-Commit & CI Pipeline ★★★★☆

**23 pre-commit hooks covering:**
| Category | Hooks |
|----------|-------|
| **Go quality** | go-fmt, goimports, go-vet, staticcheck, go-mod-tidy |
| **Python quality** | ruff (lint + format), mypy, debug-statements |
| **Cross-language** | jscpd (duplicate detection), check-testhelpers |
| **Git hygiene** | commitizen (Conventional Commits), check-merge-conflict, check-useless-excludes |
| **File hygiene** | check-yaml, check-toml, check-json, end-of-file-fixer, trailing-whitespace, forbid-crlf, remove-crlf, check-hooks-apply |

**CI pipeline:**
- Multi-platform: ubuntu-latest + macos-latest
- Sequential: lint → test (unit + e2e) → build
- Coverage uploaded to Codecov (ubuntu only)
- E2E tests run via `make test-e2e`

**Changes:** `check-embedded` hook removed (was for MCP embedded file consistency) *(reassessment 2026-04-13)*

**Concerns:**
- Python pre-commit hooks still configured despite no Python source files in the repo — harmless but vestigial *(reassessment 2026-04-13)*
- No binary size tracking

---

## Documentation & Specifications ★★★★★

**Extraordinary specification depth:**

| Category | Previous | Current | Delta |
|----------|----------|---------|-------|
| Specs | 130 (51 ADRs) | 192 (60 ADRs) | +62 (+9 ADRs) |
| Contracts | 11 | 9 | -2 (MCP removal) |
| Skills | 23 | 22 | -1 |
| Docs | 24 | 36 | +12 |
| Lessons | 5 | 6 | +1 |

*(reassessment 2026-04-13)*

**Highlights:**
- **60 Architecture Decision Records** — including ADR-0057 (MCP removal) documenting the most significant architectural change since inception *(reassessment 2026-04-13)*
- **C4 diagrams** at context, container, and component levels
- **Failure mode map** connecting each contract clause to the specific LLM failure it prevents
- **Agent testimony** and **demo benchmarks** — real session transcripts showing the system in action
- **Lessons system** — operational insights organized by audience (agents vs humans)

**Concerns:**
- The sheer volume of documentation (192 specs + 36 docs + 9 contracts + 22 skills + 6 lessons) could overwhelm new contributors
- Spec-to-implementation drift checking remains manual beyond the automated `consistency_test.go` for embedded artifacts

---

## Refactoring Recommendations by Priority

### Completed (P1 — all resolved, prior assessments)

| # | Title | Commit(s) |
|---|-------|-----------|
| 1.1 | Decompose MCP handler monolith | `3544574`, `fd145e9` |
| 1.2 | Split state model | `82258fe` |
| 1.3 | Group validation rules | `d53a2f0` |
| 1.4 | Add artifact consistency checks | `47e5597`, `bab9a78` |
| 1.5 | Split CLI entry point | `7ac5ac8` |
| 1.6 | Group `git/worktree.go` by concern | `35e5c6d` |
| 1.7 | Replace raw event literals | `a815895` |

### Completed (P2 — resolved items, prior assessments)

| # | Title | Commit(s) |
|---|-------|-----------|
| 2.2 | Extract command presentation logic | `internal/render/` package |
| 2.4 | Improve `statevalidate` test ratio (0.75:1 → 2.04:1) | `3a54845` et al. |
| 2.5a | Event name constants (26 typed constants) | `e2baae9` et al. |
| 2.5b | Resolve hardcoded `"orchestrator-1"` identity | `40f407c` et al. |
| 2.6 | Claim strategy pattern | `9d68a78` |
| 2.7 | Declarative MCP tool registration | `5350e71` (subsystem since removed) |

### Stale (removed)

| # | Title | Reason |
|---|-------|--------|
| 2.3 | Python test coverage | No Python source files remain in repo *(reassessment 2026-04-13)* |

### Priority 1: High Impact / Low Risk

#### 1.8 Decompose `proceed.go` *(escalated — reassessment 2026-04-13)*
- **What**: `proceed.go` at 1,200 LOC (was 816 at prior assessment, +47%) with 26 functions. Split by concern: transition execution (`Proceed`, `proceedInner`, `proceedManyToOneInner`), cohort management (`findManyToOneCohort`, `buildManyToOneChild`), child task construction (`buildChildTask`, `buildOneToOneChild`, `patchInheritedDeps`), graph algorithms (`topoSortPending`, `findSCCs`, `hasSelfLoop`), crash recovery (`recoverCrashedTransition`, `isTransitionIncomplete`), and the available-transitions engine (`ExecuteAvailableTransitions`, `buildTransitionDefFromPipeline`).
- **Risk**: Low — concerns are already functionally separated within the file; extraction is structural
- **Impact**: The largest production file is 2.4x the Go ecosystem norm (500 LOC). Six distinct responsibility clusters identified — each could be its own file.
- **Depends on**: None
- **Priority escalation**: This was P1 at the prior assessment. The file grew 47% further without decomposition. Next assessment without progress here directly impacts the grade.

#### 1.9 Decompose `init.go`
- **What**: `init.go` at 854 LOC (was 753, +13%) with `InitCommandWithConfig` at 315 LOC (was 294). Split by phase: project detection, symlink setup, config generation, and interactive prompts.
- **Risk**: Low — phases are sequential with clear data boundaries
- **Impact**: Second-largest commands file
- **Depends on**: None

#### 1.10 Decompose `supervisor.go` *(new — reassessment 2026-04-13)*
- **What**: `supervisor.go` at 831 LOC (was 588, +41%) with 24 functions. Extract restart tracking subsystem (~300 LOC: `exit42RestartTracker`, `crashRestartTracker`, `spinningTracker` + their helpers) into `restart_tracking.go`. Extract `DefaultCLIExecutor` and related CLI functions (~150 LOC) into a dedicated file.
- **Risk**: Low — restart trackers are self-contained structs with no shared mutable state; CLI executor is already an independent type
- **Impact**: Reduces supervisor.go from 831 to ~380 LOC; makes restart tracking independently testable and navigable
- **Depends on**: None

### Priority 2: Medium Impact / Medium Risk (open)

#### 2.1 Enforce Coverage Threshold
- **What**: Add minimum coverage gate in CI (e.g., 70% with trend tracking); prevent coverage regression on PRs
- **Risk**: Medium — may block PRs that touch hard-to-test code paths
- **Impact**: Formalizes the already-strong testing culture

#### 2.8 Decompose `watch.go`
- **What**: `watch.go` at 846 LOC (was 751, +13%). Contains stale sentinel detection, orphaned rejection checks, approaching-limits warnings, and reassignment messaging. Extract check functions into a `checks.go` or similar.
- **Risk**: Low — check functions are self-contained with clear inputs/outputs
- **Impact**: Brings watch below 500 LOC; makes individual checks testable in isolation

#### 2.9 Address TUI file sizes *(new — reassessment 2026-04-13)*
- **What**: `view.go` (688 LOC) and `update.go` (643 LOC) both exceed 500 LOC in a new subsystem. Evaluate whether view rendering can be split by panel (task panel, agent panel, log panel) and whether update handlers can be grouped by message type.
- **Risk**: Medium — bubbletea's Model-Update-View pattern centralizes naturally; splitting may require interface changes
- **Impact**: Prevents the TUI from following the same growth trajectory as ops/commands
- **Depends on**: None

#### 2.10 Test coverage for new packages *(new — reassessment 2026-04-13)*
- **What**: Add tests for `process/` (67 LOC, 0 tests), `gitenv/` (28 LOC, 0 tests), and improve `interactive/` (238 LOC, 0.46:1 ratio)
- **Risk**: Low — additive only
- **Impact**: Restores the project's pattern of universal test coverage; prevents silent breakage in process management and locale handling

### Priority 3: Strategic / Long-term

#### 3.1 Spec-Code Consistency Automation — partially done
- `consistency_test.go` verifies byte-exact match of embedded artifacts vs repo masters
- Higher-level drift detection (e.g., blackboard schema spec vs actual YAML structure) remains manual

#### 3.2 Fuzz Testing for State Mutations
- The atomic YAML state management is critical — fuzz testing concurrent reads/writes would surface edge cases
- Go's built-in fuzzing (`go test -fuzz`) is well-suited for this

#### 3.3 Binary Size Tracking (revised)
- Track liza binary size in CI to detect bloat from embedded content growth
- `console.sh` was removed *(reassessment 2026-04-13)* but contracts + skills + settings are still embedded *(reassessment 2026-04-13)*

#### 3.4 `proceed.go` design-level review
- After P1.8 structural decomposition, evaluate whether `ExecuteAvailableTransitions` (210 LOC) and the graph algorithm cluster would benefit from a strategy or visitor pattern
- Contingent on P1.8 — assess after decomposition reveals the actual complexity shape

#### 3.5 Clean up vestigial Python tooling *(new — reassessment 2026-04-13)*
- **What**: Remove Python pre-commit hooks (ruff, mypy, debug-statements) and Python-related config now that no Python source files exist in the repo
- **Risk**: Low — hooks are no-ops on zero matching files
- **Impact**: Reduces pre-commit configuration noise; removes false signal that the project has a Python component

---

## Files >500 LOC *(updated — reassessment 2026-04-13)*

| File | LOC | Delta | Nature |
|------|-----|-------|--------|
| `internal/ops/proceed.go` | 1,200 | +384 (+47%) | Transition engine, cohort, graph algos, crash recovery |
| `cmd/liza/cmd_task.go` | 910 | *(new in list)* | Declarative Cobra init() — command registrations |
| `internal/commands/init.go` | 854 | +101 (+13%) | Brownfield init, symlinks, Node.js detection |
| `internal/commands/watch.go` | 846 | +95 (+13%) | Stale sentinels, attempt-aware messaging |
| `internal/agent/supervisor.go` | 831 | +243 (+41%) | Restart tracking, CLI execution, supervisor loop |
| `internal/tui/view.go` | 688 | *(new)* | TUI rendering — task/agent/log panels |
| `cmd/liza/cmd_system.go` | 683 | +146 (+27%) | Declarative Cobra init() — command registrations |
| `internal/tui/update.go` | 643 | *(new)* | TUI state transitions — message handlers |
| `internal/pipeline/resolver.go` | 642 | +71 (+12%) | Pipeline query interface |
| `internal/ops/claim_task.go` | 563 | +21 | Stable |
| `internal/models/task.go` | 558 | *(new in list)* | 31 small domain methods (avg 18 LOC each) |
| `internal/ops/wt_merge.go` | 520 | 0 | Stable |

**Removed from list:** `internal/mcp/server_registration.go` (800 LOC) — subsystem deleted *(reassessment 2026-04-13)*

---

## Summary

Liza continues to demonstrate technically rigorous engineering with an unusually disciplined approach to testing, code hygiene, and specification. The most significant change this period was the **removal of the MCP server** — a courageous architectural simplification that deleted ~8,500 LOC of working but redundant code in favor of CLI-native access control. This was accompanied by a **new TUI subsystem** using the Charmbracelet stack, bringing direct dependencies from 4 to 9.

The project's primary challenge remains **structural complexity accumulation**. Files exceeding 500 LOC grew from 9→12 (net, after MCP removal), non-declarative functions >150 LOC nearly doubled (9→16), and **all three P1 decomposition targets from the prior assessment grew further** without being addressed: proceed.go (+47%), init.go (+13%), watch.go (+13%). The test-to-production ratio dipped slightly (2.69→2.63) for the first time — production code grew 16% while test code grew 14%. Code hygiene remains perfect.

**Overall Rating: A- (Excellent with concerns)** *(unchanged — reassessment 2026-04-13)*

The A- holds on the strength of architectural simplification, pristine hygiene, and strong testing culture. The gap to A has widened: structural complexity is compounding while decomposition recommendations remain unaddressed. **A further assessment without P1 progress would warrant a downgrade to B+.** The gap to B+ is held by the consistent quality of new code (TUI, jsonout, paths, analysis all arrived tested), the MCP removal demonstrating willingness to simplify, and the 60 ADRs showing continued design discipline.
