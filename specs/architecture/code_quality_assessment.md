# Code Quality Assessment and Refactoring Roadmap

* Date: 2026-03-11 (commit a815895)
* Repository: liza
* Author: Claude Code - Opus 4.6
* Mode: Reassessment (after 2026-03-11 / a2912c5)

## Repository Metrics Dashboard

- **Production Code**: 24,134 lines of Go across 163 files *(was 23,776/160 — +358 LOC, +3 files)*
- **Test Code**: 58,656 lines across 131 test files (2.43:1 test-to-production ratio) *(was 57,809/129)*
- **Test Functions**: 1,037 test cases with table-driven subtests *(was 1,039 — minor consolidation)*
- **Behavioral Contracts**: 1,945 lines across 9 core documents + 20 skill protocols (4,523 lines) *(skills +62 LOC)*
- **Specifications**: 105 Markdown files including 46 ADRs *(was 103/45 — +1 ADR: MCP middleware and declarative registration)*
- **Documentation**: 23 user-facing guides
- **Dependencies**: 4 direct (cobra, yaml.v3, flock, fsnotify) — radically minimal
- **CI/CD**: Multi-platform (Linux + macOS), Codecov integration, 23 pre-commit hooks, E2E tests in CI
- **Code Hygiene**: Zero TODOs, zero `nolint` directives, zero `panic()`, zero `interface{}` in production Go code; statuses, roles, and event names are typed constants

## Executive Summary

Liza is a hybrid multi-agent coding orchestrator: Go-based deterministic supervisors enforce invariants while LLM agents handle judgment. The codebase demonstrates **exceptional engineering discipline** in its core runtime — minimal dependencies, comprehensive testing, atomic state management — combined with an unusually thorough specification and contract corpus that forms the product's core IP.

**Key Strengths:**
- **Test-first culture**: 2.43:1 test-to-production ratio with race detection, parallelization enforcement, and sleep guards — ratio stable across 27 commits of feature and refactoring work *(reassessment 2026-03-11)*
- **Radical dependency minimalism**: 4 direct dependencies for the entire Go runtime
- **Pristine code hygiene**: Zero TODOs, zero `nolint`, zero `panic()`, zero untyped code in production; all event names are typed constants — zero raw event literals remain *(reassessment 2026-03-11)*
- **Atomic state management**: flock + temp-write + fsync + rename pattern prevents corruption
- **Specification-driven development**: 105 spec files + 46 ADRs create extraordinary traceability
- **Consistent quality across new code**: 5 supporting infrastructure packages (`filelock`, `identity`, `log`, `roles`, `errors`) all maintain test ratios between 1.8:1 and 3.9:1 *(reassessment 2026-03-11)*

**Areas for Improvement:**
- **Long orchestration functions trending upward**: 8 functions exceed 150 LOC (was 6), longest at 257 LOC; modest growth in existing functions (`InitCommandWithConfig` 248→257, `MergeWorktree` 234→251) *(reassessment 2026-03-11)*
- **Coverage reporting gap**: Codecov configured but coverage threshold not enforced in CI *(verified still absent)*
- **Python layer underspecified**: Supporting Python utilities lack tests *(verified still absent)*
- ~~**3 raw event literals remain in `claim_task_strategy.go`**~~ *(Resolved — `a815895`: all 4 raw event literals replaced with typed constants)*

**Overall Rating: A (Excellent)**

The deduction from A+ is for: (1) 8 orchestration functions >150 LOC with a slow upward trend in count and size, and (2) absent coverage enforcement despite strong testing culture.

---

## Detailed Subsystem Analysis

### State Machine & Models (`internal/models/`) ★★★★★

**Strengths:**
- **Explicit state machine**: 13 task states with pipeline-driven transitions via `TransitionWith()` — no implicit state changes possible
- **Pipeline-driven extensibility**: Custom state names via YAML pipeline config with `Resolver` providing runtime query interface
- **Complete audit trail**: Every task mutation appended to `History[]` with timestamps and actor IDs
- **Lease-based concurrency**: Time-bounded claims with stale detection prevent zombie agents
- **Thorough model tests**: 2,049 test LOC for 1,154 production LOC (1.78:1) *(was 1.5:1 — improved by +64 prod, +47 test; reassessment 2026-03-11)*

**Concerns:**
- ~~The distinction between hardcoded states and pipeline-declared states adds cognitive overhead for contributors~~ *(Resolved — `581d377`: hardcoded `taskTransitions` map, `CanTransition()`, `Transition()` removed; pipeline-only)*

### Operations Layer (`internal/ops/`) ★★★★☆

**Strengths:**
- **Clean service layer**: Each operation is its own file with focused responsibility (36 production files) *(reassessment 2026-03-11)*
- **Precondition-heavy design**: Operations validate extensively before mutating, failing fast with typed errors
- **Rebase conflict handling**: `submit_review.go` detects drift and returns actionable error messages, not generic failures
- **Compare-and-swap for git refs**: Prevents lost updates during concurrent merges
- **Strong test ratio**: 12,543 test LOC for 6,061 production LOC (2.07:1) *(was 2.05:1; reassessment 2026-03-11)*
- **Worktree lifecycle cleanup**: Worktrees now cleaned up on both merge and supersede *(reassessment 2026-03-11)*

**Concerns:**
- ~~`claim_task.go` (655 LOC) and `proceed.go` (533 LOC) are on the large side~~ *(Partially resolved — `claim_task.go` 551 LOC with strategy extraction to `claim_task_strategy.go` 210 LOC, `proceed.go` 504 LOC)*
- ~~**Boolean-flag dispatch in `ClaimTask()`** (296 LOC)~~ *(Resolved — claim strategy pattern extracted; `ClaimTask()` reduced to 248 LOC of strategy dispatch)*
- 8 orchestration functions exceed 150 LOC: `InitCommandWithConfig` (257), `MergeWorktree` (251), `ClaimTask` (248), `SubmitForReview` (211), `SubmitVerdict` (185), `RecoverTask` (183), `RunSupervisor` (157), `AddTask` (153). Count was 6 at pass 2; +2 new entries (`RunSupervisor` previously noted in agent subsystem, `AddTask` newly crossed threshold). `InitCommandWithConfig` and `MergeWorktree` each grew ~15 LOC from feature additions *(reassessment 2026-03-11)*
- ~~**Event name string literals scattered across 17 files**~~ *(Resolved — 26 `TaskEventName` typed constants used across all packages. Last 4 raw literals in `claim_task_strategy.go` replaced in `a815895`)*
- ~~**Hardcoded `"orchestrator-1"` as assumed agent ID**~~ *(Resolved)*

### MCP Server (`internal/mcp/`) ★★★★☆

**Strengths:**
- **Tool categorization**: Registration split into read-only, mutation, and complex operation phases
- **Error classification**: Typed errors mapped to JSON-RPC codes with sanitized messages (no implementation leaks)
- **Schema consistency tests**: Verify tool definitions match handler signatures
- **Graceful degradation**: Missing `.liza` directory returns structured errors instead of crashing
- **Handler-level middleware**: `withLogging` and `withRole` middleware in `middleware.go` (39 LOC) with dedicated tests (127 LOC)
- **Declarative registration**: `toolDef` struct with `[]toolDef` data slices — tool definitions are data, not code

**Concerns:**
- ~~No handler-level middleware~~ *(Resolved)*
- ~~**Imperative tool registration**~~ *(Resolved)*
- `server_registration.go` at 667 LOC contains two registration functions >250 LOC each — these are declarative data definitions (tool schemas), not control flow, so the LOC is structural rather than concerning *(reassessment 2026-03-11)*

### Agent Supervision (`internal/agent/`) ★★★★☆

**Strengths:**
- **Deterministic supervisor**: Go process wraps LLM agent, enforcing restart limits, heartbeat, lease renewal
- **Exit code 42 protocol**: Clean restart mechanism when agent needs fresh context
- **Context exhaustion handoff**: Structured notes enable continuation across agent instances
- **Strategy pattern**: Role-specific behavior cleanly separated into `strategy_doer.go`, `strategy_reviewer.go`, `strategy_orchestrator.go` — each with `WaitConfig`
- **Work detection logic**: Sophisticated polling with configurable intervals per role

**Concerns:**
- ~~`supervisor.go` (637 LOC) and `waitforwork.go` (412 LOC) handle complex lifecycle logic~~ *(Resolved — `supervisor.go` 535 LOC, `waitforwork.go` 213 LOC)*
- `RunSupervisor()` at 157 LOC interleaves restart logic with signal handling — well-tested but dense *(verified unchanged; reassessment 2026-03-11)*

### Git Operations (`internal/git/`) ★★★★☆

**Strengths:**
- **Merge-tree strategy**: Merges without touching the working directory — prevents dirty-state conflicts
- **Atomic ref updates**: Compare-and-swap on git refs prevents concurrent merge races
- **Selective file sync**: After merge, only changed files are synced to working tree
- **Drift calculation**: Counts commits between base and target for conflict prediction
- **Comprehensive rebase handling**: Conflict detection with structured error types

**Concerns:**
- ~~`worktree.go` (591 LOC, 35 functions) mixes 5 concerns~~ *(Resolved — split into 5 concern-based files)*
- `merge.go` grew from 232 to 314 LOC due to worktree cleanup logic added for merge and supersede operations. Still within acceptable range but approaching the 300 LOC attention threshold *(reassessment 2026-03-11)*

### State Validation (`internal/statevalidate/`) ★★★★★

**Strengths:**
- **43+ validation rules**: Every state mutation runs through comprehensive checks
- **Rule separation from ops**: Validation is a distinct package, not mixed into business logic
- **Pipeline-aware validation**: Rules adapt to custom pipeline states
- **Well-documented invariants**: Doc comments on each validation function explain the invariant it protects

**Concerns:**
- ~~Lowest test-to-production ratio in the codebase at 0.75:1~~ *(Resolved — 1.33:1: 1,029 test LOC for 774 production LOC. Verified stable; reassessment 2026-03-11)*

### CLI Entry Point (`cmd/liza/`) ★★★★☆

**Strengths:**
- **Thin delegation**: Each command's `RunE` averages 5-15 lines — parses flags and delegates to `commands` or `ops`
- **Consistent flag patterns**: All commands follow the same structure
- **Domain-based organization**: Split into 7 files — `main.go` (95 LOC), `cmd_task.go` (275), `cmd_system.go` (489), `cmd_agent.go` (241), `cmd_review.go` (183), `cmd_init.go` (127), `cmd_worktree.go` (123) *(reassessment 2026-03-11)*

**Concerns:**
- ~~`main.go` at 1,462 LOC is the largest file in the codebase~~ *(Resolved)*
- The `init()` function's registration block must be maintained in sync with command definitions — no compile-time enforcement prevents a command from being defined but not registered

### CLI Commands (`internal/commands/`) ★★★★☆

**Strengths:**
- **Thin command layer**: Commands delegate to ops, never contain business logic
- **Comprehensive coverage**: 75 files covering every system operation
- **Consistent patterns**: Each command follows the same structure (flag parsing → ops call → output formatting)
- **Shared rendering infrastructure**: `internal/render/` package (175 LOC) extracts common formatting

**Concerns:**
- `watch.go` (645 LOC) and `status.go` (449 LOC) are large files (watch.go is ~99% business logic; status.go delegates formatting to `internal/render/`) *(verified unchanged; reassessment 2026-03-11)*
- `init.go` grew to 386 LOC with `InitCommandWithConfig` at 257 LOC — the longest function in the commands package, handling project initialization with multiple interactive prompts. Growth from `console.sh` embedding feature *(reassessment 2026-03-11)*

### Prompt Building (`internal/prompts/`) ★★★★☆

**Strengths:**
- **Template-based agent initialization**: Structured bootstrap prompts with embedded contracts, role definitions, and task context
- `builder.go` (424 LOC) generates agent initialization context; `wake.go` (179 LOC) encapsulates wake trigger subsystem *(reassessment 2026-03-11)*
- **Strong test ratio**: 1,940 test LOC for 643 production LOC (3.02:1) *(was 3.03:1; reassessment 2026-03-11)*

**Concerns:**
- ~~Single large file — could be decomposed into template sections~~ *(Resolved — wake trigger subsystem extracted to `wake.go`)*

### Supporting Infrastructure *(reassessment 2026-03-11)*

Five focused infrastructure packages demonstrate the same quality discipline as the core subsystems:

| Package | Prod LOC | Test LOC | Ratio | Purpose |
|---------|----------|----------|-------|---------|
| `internal/filelock/` | 489 | 884 | 1.81:1 | File locking with metrics and typed errors |
| `internal/log/` | 211 | 678 | 3.21:1 | Structured agent logging |
| `internal/roles/` | 141 | 375 | 2.66:1 | Role type definitions and validation |
| `internal/identity/` | 108 | 418 | 3.87:1 | Orchestrator identity resolution |
| `internal/errors/` | 45 | 121 | 2.69:1 | Shared error types |

All five follow the project's patterns: typed constants, precondition validation, table-driven tests.

---

## Behavioral Contracts & Skills ★★★★★

This is Liza's core IP and most distinctive feature.

**Strengths:**
- **Failure-mode-driven design**: 55+ documented LLM failure modes mapped to specific countermeasures in `CONTRACT_FAILURE_MODE_MAP.md`
- **Tiered rule architecture**: Tier 0 (inviolable) through Tier 3 (preferences) with explicit degradation protocol
- **Execution state machine**: 10 states with forbidden transitions, model activation points, and stop triggers
- **20 composable skills**: Domain-specific protocols (debugging, testing, code review, architecture) that agents load on demand
- **Three collaboration modes**: Pairing (human-supervised), Multi-Agent (peer-supervised), Subagent (delegated) — each with explicit gate semantics
- **Anti-gaming clause**: "Technically compliant is not compliant" — closes the most common loophole in agent governance

**Concerns:**
- Contract documents are necessarily large (CORE.md at 750 lines) — agents must read them fully, consuming context window budget *(verified unchanged; reassessment 2026-03-11)*
- The archived contract versions (`contracts/_archive/`) suggest rapid evolution — no migration guide between contract versions
- Skills lack versioning — when a skill protocol changes, all sessions see the new version immediately

---

## Testing & Quality Infrastructure ★★★★★

**Strengths:**
- **2.43:1 test-to-production ratio** — stable across 27 commits of mixed feature and refactoring work *(reassessment 2026-03-11)*
- **Pure standard library testing**: No external test frameworks — reduces dependency surface
- **Table-driven tests throughout**: 80+ files use `t.Run()` subtests with structured test cases
- **Race detection enabled by default**: `-race` flag in all CI runs
- **Test quality enforcement**:
  - `parallel_usage_test.go`: Ratcheting minimum for `t.Parallel()` calls (currently ≥10)
  - `sleep_usage_test.go`: Prevents `time.Sleep` in tests — enforces real concurrency patterns
  - `check-testhelpers`: Pre-commit hook ensures test utilities don't leak into production
- **Integration tests**: 5 E2E test files (2,102 LOC) covering concurrent operations, lease expiry, sprint management, and full workflows — runs in CI via `make test-e2e`
- **Isolated test environments**: Every test gets `t.TempDir()` with fresh git repo and `.liza` directory
- **Per-package test health**: All 22 non-trivial packages have test ratios ≥ 1.33:1; 13 packages exceed 1.5:1 *(was 19 packages ≥ 0.75:1, 11 ≥ 1.5:1; improved by infrastructure package additions; reassessment 2026-03-11)*

**Concerns:**
- No coverage threshold enforced in CI — Codecov is configured for reporting only; no `.codecov.yml` exists *(verified still absent; reassessment 2026-03-11)*
- Python utilities have no active test suite despite pytest being configured *(verified still absent; reassessment 2026-03-11)*
- No mutation testing or fuzz testing

---

## Pre-Commit & CI Pipeline ★★★★☆

**23 pre-commit hooks covering:**
| Category | Hooks |
|----------|-------|
| **Go quality** | go-fmt, goimports, go-vet, staticcheck, go-mod-tidy |
| **Python quality** | ruff (lint + format), mypy, debug-statements |
| **Cross-language** | jscpd (duplicate detection), check-testhelpers, check-embedded |
| **Git hygiene** | commitizen (Conventional Commits), check-merge-conflict, check-useless-excludes |
| **File hygiene** | check-yaml, check-toml, check-json, end-of-file-fixer, trailing-whitespace, forbid-crlf, remove-crlf |

**CI pipeline:**
- Multi-platform: ubuntu-latest + macos-latest
- Sequential: lint → test (unit + e2e) → build
- Coverage uploaded to Codecov (ubuntu only)
- E2E tests run via `make test-e2e`

**Concerns:**
- No binary size tracking (liza binary could grow unnoticed from embedded content) *(verified still absent; reassessment 2026-03-11)*

---

## Documentation & Specifications ★★★★★

**Extraordinary specification depth:**

| Category | Files | Contents |
|----------|-------|----------|
| Specs | 105 *(was 103)* | Vision, epics, user stories, architecture, protocols, 46 ADRs *(was 45)* |
| Contracts | 9 | Behavioral governance for agents |
| Skills | 20 | Domain-specific agent protocols |
| Docs | 23 | User guides, recipes, troubleshooting, demos |
| Lessons | 4 | Operational lessons for agents *(reassessment 2026-03-11)* |

**Highlights:**
- **46 Architecture Decision Records** — comprehensive design rationale capture *(+1: ADR-0043 MCP middleware and declarative registration; reassessment 2026-03-11)*
- **C4 diagrams** at context, container, and component levels
- **Failure mode map** connecting each contract clause to the specific LLM failure it prevents
- **Agent testimony** and **demo benchmarks** — real session transcripts showing the system in action
- **Lessons system** — operational insights organized by audience (agents vs humans)

**Concerns:**
- Some specs reference implementation details that may have drifted (normal for spec-heavy projects)
- Artifact consistency (embedded copies vs repo masters) is now automated via `consistency_test.go`; higher-level spec-to-implementation drift checking remains manual
- The sheer volume of documentation (31,000+ lines) could overwhelm new contributors
- The main cost of this documentation volume is not just onboarding; it also increases the chance of locally correct but systemically incomplete changes

---

## Refactoring Recommendations by Priority

### Priority 1: High Impact / Low Risk

#### 1.1 Decompose MCP Handler Monolith — ✅ DONE
- `handlers.go` (918 LOC) → `handlers_helpers.go` (303), `handlers_readonly.go` (131), `handlers_mutation.go` (291), `handlers_complex.go` (217). Original deleted.
- `server.go` (887 LOC) → `server.go` (130), `server_protocol.go` (243), `server_registration.go` (567)
- Commits: `3544574`, `fd145e9`

#### 1.2 Split State Model — ✅ DONE
- `state.go` (937 LOC) → `state.go` (43), `task.go` (431), `agent.go` (51), `sprint.go` (137), `config.go` (132), `history.go` (163)
- Commit: `82258fe`

#### 1.3 Group Validation Rules — ✅ DONE
- `validate.go` (658 LOC) → `validate.go` (114, orchestration + shared helpers), `validate_task.go` (372), `validate_agent.go` (42), `validate_deps.go` (84), `validate_entity.go` (75), `validate_sprint.go` (88)
- Doc comments added to each validation function explaining the invariant it protects
- Commit: `d53a2f0`

#### 1.4 Add Artifact Consistency Checks — ✅ DONE
- `internal/embedded/consistency_test.go` (126 LOC): byte-exact comparison of repo masters vs embedded copies (contracts, skills, claude-settings.json, mcp.json)
- `make check-embedded` target added, wired into `make lint`
- Commits: `47e5597`, `bab9a78`

#### 1.5 Split CLI Entry Point — ✅ DONE *(pass 2)*
- `cmd/liza/main.go` (1,462 LOC) → 7 domain-specific files: `main.go` (95 LOC), `cmd_task.go` (280), `cmd_system.go` (489), `cmd_agent.go` (241), `cmd_review.go` (183), `cmd_init.go` (127), `cmd_worktree.go` (123)
- Commit: `7ac5ac8`

#### 1.6 Group `git/worktree.go` by Concern — ✅ DONE *(pass 2)*
- `worktree.go` (591 LOC, 35 functions) → 5 concern-based files: `worktree.go` (174 LOC, 7 functions — CRUD), `merge.go` (232, 10 functions), `rebase.go` (55, 4 functions), `query.go` (52, 4 functions), `git.go` (105, 10 functions — struct/constructor/exec)
- Commit: `35e5c6d`

#### 1.7 Replace Raw Event Literals in `claim_task_strategy.go` — ✅ DONE *(reassessment 2026-03-11)*
- 4 raw event string literals replaced with typed `TaskEventName` constants (`TaskEventClaimed`, `TaskEventReclaimedAfterRejection`, `TaskEventReassignedAfterRejection`, `TaskEventClaimedForIntegrationFix`). Completes the typed event name migration.
- Commit: `a815895`

### Priority 2: Medium Impact / Medium Risk

#### 2.1 Enforce Coverage Threshold
- Add minimum coverage gate in CI (e.g., 70% with trend tracking)
- Prevent coverage regression on PRs
- Risk: Medium — may block PRs that touch hard-to-test code paths
- Impact: Formalizes the already-strong testing culture
- *(verified still absent; reassessment 2026-03-11)*

#### 2.2 Extract Command Presentation Logic ✅
- Extracted shared formatting infrastructure (`FormatJSON`, `FormatYAML`, `FormatValue`, `FormatTable`, `ExecuteTemplate`, `FormatDuration`) into `internal/render/` package
- Templates moved from `commands/templates/` to `render/templates/`
- Domain-specific helpers (`formatKeyValue`, `formatDashboard`, `dashboardSection`) remain in `commands/format_helpers.go` (no production callers outside commands)
- `watch.go` analyzed and intentionally left alone: ~99% business logic, only `alert.String()` is presentation (a one-liner)
- Risk: Medium — touched 16 files across commands package with phased execution and compile gates
- Impact: Establishes clear boundary between business logic and presentation

#### 2.3 Python Test Coverage
- Add tests for Python utilities (markdown processing, analysis scripts)
- pytest is already configured; the gap is in actual test files
- Risk: Low — additive only
- Impact: Prevents silent breakage in supporting tooling
- *(verified still absent; reassessment 2026-03-11)*

#### 2.4 Improve `statevalidate` Test Ratio — ✅ DONE *(pass 2)*
- Test ratio improved from 0.75:1 to 1.33:1 (1,029 test LOC for 774 production LOC). `validate_task_test.go` grew from ~130 to 446 LOC with edge-case coverage for task validation branches. Test functions: 20 → 24.
- Commit: `3a54845`

#### 2.5 Eliminate Magic Literals *(pass 2)*

**2.5a Event name constants** — ✅ DONE (Low risk):
- 26 `TaskEventName` typed constants defined in `models/history.go`. Constants replaced across all packages. Last 4 raw literals in `claim_task_strategy.go` resolved in P1 item 1.7 *(reassessment 2026-03-11)*
- Commits: `e2baae9`, `0451f44`, `5b4cd5f`, `e08d3f5`, `4e4baed`

**2.5b Resolve hardcoded `"orchestrator-1"` identity** — ✅ DONE:
- Ops layer (`AddTask`, `AddTasks`, `SupersedeTask`) now returns error on empty agent ID instead of silently defaulting. CLI commands (`add-task`, `supersede-task`) resolve via `resolveOrchestratorID`: flag → env var → `ResolveOrchestratorFromState()`. MCP schema `Default` values removed; descriptions updated to "auto-resolved from registered orchestrator". MCP handlers already used `resolveOrchestratorID` with state-based fallback.
- **Impact**: Prevents identity mismatch bugs in multi-orchestrator or renamed-orchestrator scenarios.

#### 2.6 Claim Strategy Pattern — ✅ DONE *(pass 2)*

- `claimStrategy` interface extracted to `claim_task_strategy.go` (210 LOC) with `Preconditions()`, `WorktreePhase()`, `MutateTask()`, `EventName()` methods. Three implementations: `freshClaimStrategy`, `rejectedClaimStrategy`, `integrationFixClaimStrategy`. `ClaimTask()` reduced from 296 to 248 LOC of strategy dispatch. Boolean flags eliminated. *(reassessment 2026-03-11: file grew 197→210 LOC)*
- Commit: `9d68a78`

#### 2.7 Declarative MCP Tool Registration — ✅ DONE *(pass 2)*
- `toolDef` struct with `registerToolDefs()` loop replaces imperative registration. `server_registration.go` (667 LOC) now defines tools as `[]toolDef` data slices. Schema consistency tests preserved. *(reassessment 2026-03-11: file 668→667 LOC, stable)*
- Commit: `5350e71`

### Priority 3: Strategic / Long-term

#### 3.1 Spec-Code Consistency Automation — ✅ PARTIALLY DONE
- `consistency_test.go` verifies byte-exact match of embedded artifacts vs repo masters (contracts, skills, settings)
- Higher-level drift detection (e.g., blackboard schema spec vs actual YAML structure) remains manual
- Impact: Artifact layer covered; semantic spec-to-code consistency remains a gap

#### 3.2 Fuzz Testing for State Mutations
- The atomic YAML state management is critical — fuzz testing concurrent reads/writes would surface edge cases
- Go's built-in fuzzing (`go test -fuzz`) is well-suited for this
- Impact: Strengthens the most critical subsystem

#### 3.3 Binary Size Tracking
- Track liza/liza-mcp binary sizes in CI to detect bloat from embedded content growth
- The embedded contracts + skills + settings already constitute significant binary content; `console.sh` embedding adds to this *(reassessment 2026-03-11)*
- Impact: Prevents gradual size creep as the contract and skill corpus grows

---

## Reassessment Delta Summary *(2026-03-11)*

**27 commits since previous assessment (a2912c5 → 1ef4ef4):**

| Area | Change | Assessment Impact |
|------|--------|-------------------|
| Production LOC | +358 (23,776 → 24,134) | Modest growth consistent with feature additions |
| Test LOC | +847 (57,809 → 58,656) | Test growth outpaced production growth 2.4:1 |
| Test ratio | 2.43:1 → 2.43:1 | Perfectly stable — discipline maintained |
| Functions >150 LOC | 6 → 8 | Upward trend. `AddTask` (153) and `RunSupervisor` (157) newly counted |
| Largest function | 248 → 257 | `InitCommandWithConfig` grew from `console.sh` embedding |
| Event literals | "few remaining" → ✅ resolved | All raw literals replaced with typed constants in `a815895` |
| Package count | ~19 assessed | 22 non-trivial, all ≥ 1.33:1 test ratio |
| ADRs | 45 → 46 | ADR-0043 backfilled for MCP middleware |
| Bug fixes | — | Worktree cleanup on merge + supersede; type assertion → interface |
| Coverage enforcement | absent | Still absent |
| Python tests | absent | Still absent |

**Trend**: Codebase quality is stable-to-improving. New code maintains the same discipline as existing code. The main watch item is the slow upward trend in long function count — not yet problematic (all are complex orchestration with validated preconditions) but worth monitoring.

---

## Summary

Liza is a technically rigorous project that practices what it preaches. The behavioral contracts that govern LLM agents are themselves enforced by well-tested Go code with atomic state management, comprehensive validation, and race-free concurrency patterns. The 2.43:1 test-to-production ratio, zero TODOs, zero `nolint`, zero `panic()`, and 4-dependency runtime reflect deliberate engineering discipline — and these metrics held stable across 27 commits of mixed feature and refactoring work.

The project's primary challenge is not code quality but **cognitive surface area**: 31,000+ lines of specifications, contracts, and skills create an extraordinary knowledge base that also presents a steep learning curve. The code itself is well-factored at the package level; the remaining concerns are: (1) 8 orchestration functions exceeding 150 LOC with a slow upward trend, and (2) absent coverage enforcement. All P1 items from all passes are resolved, including the new P1 (1.7) for event literal cleanup.

**Overall Rating: A (Excellent)**

The deduction from A+ is for: (1) 8 orchestration functions >150 LOC (the longest at 257 LOC, trend is slowly upward), and (2) absent coverage enforcement despite strong testing culture. The grade is unchanged from the previous assessment — the codebase continues to demonstrate excellent engineering at the same consistent quality level. All P1 items from all passes are now resolved.
