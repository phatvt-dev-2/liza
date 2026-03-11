# Code Quality Assessment and Refactoring Roadmap

* Date: 2026-03-11 (commit a2912c5)
* Repository: liza
* Author: Claude Code - Opus 4.6
* Mode: Enrichment (pass 2, Complexity lens)

## Repository Metrics Dashboard

- **Production Code**: 23,776 lines of Go across 160 files *(was 22,898/149 — +878 LOC, +11 files from structural splits and strategy extraction)*
- **Test Code**: 57,809 lines across 129 test files (2.43:1 test-to-production ratio) *(was 56,058/122, 2.45:1)*
- **Test Functions**: 1,039 test cases with table-driven subtests *(was 1,015)*
- **Behavioral Contracts**: 1,944 lines across 9 core documents + 20 skill protocols (4,461 lines)
- **Specifications**: 103 Markdown files including 45 ADRs *(was 98/41)*
- **Documentation**: 23 user-facing guides
- **Dependencies**: 4 direct (cobra, yaml.v3, flock, fsnotify) — radically minimal
- **CI/CD**: Multi-platform (Linux + macOS), Codecov integration, 23 pre-commit hooks *(was 21)*, E2E tests in CI
- **Code Hygiene**: Zero TODOs, zero `nolint` directives, zero `panic()`, zero `interface{}` in production Go code; statuses, roles, and event names are typed constants

## Executive Summary

Liza is a hybrid multi-agent coding orchestrator: Go-based deterministic supervisors enforce invariants while LLM agents handle judgment. The codebase demonstrates **exceptional engineering discipline** in its core runtime — minimal dependencies, comprehensive testing, atomic state management — combined with an unusually thorough specification and contract corpus that forms the product's core IP.

**Key Strengths:**
- **Test-first culture**: 2.43:1 test-to-production ratio with race detection, parallelization enforcement, and sleep guards
- **Radical dependency minimalism**: 4 direct dependencies for the entire Go runtime
- **Pristine code hygiene**: Zero TODOs, zero `nolint`, zero `panic()`, zero untyped code in production; event names now typed constants *(pass 2)*
- **Atomic state management**: flock + temp-write + fsync + rename pattern prevents corruption
- **Specification-driven development**: 103 spec files + 45 ADRs create extraordinary traceability
- **Healthy refactoring trajectory**: Structural splits added 11 files for navigability while test count grew by 42; codebase LOC increased modestly (+878 from strategy extraction, constants, and per-file overhead) *(pass 2)*

**Areas for Improvement:**
- ~~**CLI registration monolith**: `cmd/liza/main.go` at 1,462 LOC~~ *(Resolved — split into 7 domain-specific files; `main.go` reduced to 95 LOC)*
- ~~**Design-level complexity**: boolean-flag dispatch, imperative MCP registration, untyped event names~~ *(Largely resolved — claim strategy pattern, declarative tool registration, typed event constants; some raw literals remain in new strategy file and proceed.go)*
- **Coverage reporting gap**: Codecov configured but coverage threshold not enforced in CI
- **Python layer underspecified**: Supporting Python utilities lack tests
- ~~**Hardcoded `"orchestrator-1"` identity**: 8 production call sites still default to `"orchestrator-1"` — should resolve from workspace state~~ *(Resolved — ops layer requires non-empty ID; CLI resolves via flag → env → state; MCP schema defaults removed)*

**Overall Rating: A (Excellent)**

The deduction from A+ is for: (1) ~~file-level concentration in `cmd/liza/main.go` and `git/worktree.go`~~ *(resolved — both split into domain-specific files)*, (2) remaining design-level complexity — ~~hardcoded `"orchestrator-1"` identity across 8 call sites~~ *(resolved)*, 6 orchestration functions still exceed 150 LOC, and (3) absent coverage enforcement despite strong testing culture.

---

## Detailed Subsystem Analysis

### State Machine & Models (`internal/models/`) ★★★★★

**Strengths:**
- **Explicit state machine**: 13 task states with pipeline-driven transitions via `TransitionWith()` — no implicit state changes possible
- **Pipeline-driven extensibility**: Custom state names via YAML pipeline config with `Resolver` providing runtime query interface
- **Complete audit trail**: Every task mutation appended to `History[]` with timestamps and actor IDs
- **Lease-based concurrency**: Time-bounded claims with stale detection prevent zombie agents
- **Thorough model tests**: 1,651 lines of tests for 1,090 lines of production code (1.5:1), plus new diagnostics module (200 prod + 351 test)

**Concerns:**
- ~~The distinction between hardcoded states and pipeline-declared states adds cognitive overhead for contributors~~ *(Resolved — `581d377`: hardcoded `taskTransitions` map, `CanTransition()`, `Transition()` removed; pipeline-only)*

### Operations Layer (`internal/ops/`) ★★★★☆

**Strengths:**
- **Clean service layer**: Each operation is its own file with focused responsibility (18 production files)
- **Precondition-heavy design**: Operations validate extensively before mutating, failing fast with typed errors
- **Rebase conflict handling**: `submit_review.go` detects drift and returns actionable error messages, not generic failures
- **Compare-and-swap for git refs**: Prevents lost updates during concurrent merges
- **Strong test ratio**: 12,282 test LOC for 5,984 production LOC (2.05:1)

**Concerns:**
- ~~`claim_task.go` (655 LOC) and `proceed.go` (533 LOC) are on the large side~~ *(Partially resolved — legacy code paths removed; `claim_task.go` 551 LOC with strategy extraction to `claim_task_strategy.go` 197 LOC, `proceed.go` 504 LOC)*
- ~~**Boolean-flag dispatch in `ClaimTask()`** (296 LOC)~~ *(Resolved — claim strategy pattern extracted to `claim_task_strategy.go`; `ClaimTask()` reduced to 247 LOC of strategy dispatch. Each claim path (fresh/rejected/integration-fix) is now a self-contained strategy struct with `Preconditions()`, `WorktreePhase()`, `MutateTask()`, `EventName()` methods)*
- 6 orchestration functions exceed 150 LOC: `InitCommandWithConfig` (248), `ClaimTask` (247), `MergeWorktree` (234), `SubmitForReview` (203), `SubmitVerdict` (185), `RecoverTask` (183) *(pass 2)*
- ~~**Event name string literals scattered across 17 files**~~ *(Largely resolved — 26 `TaskEventName` typed constants defined in `models/history.go`; constants used across `statevalidate`, `ops`, `commands`, and `agent` packages. A few raw literals remain in `claim_task_strategy.go` and `proceed.go`)*
- ~~**Hardcoded `"orchestrator-1"` as assumed agent ID**~~ *(Resolved — ops layer requires non-empty agent ID; CLI resolves via flag → env var → `ResolveOrchestratorFromState()`; MCP schema defaults removed; MCP handlers already used `resolveOrchestratorID` with state fallback)*

### MCP Server (`internal/mcp/`) ★★★★☆

**Strengths:**
- **Tool categorization**: Registration split into read-only, mutation, and complex operation phases
- **Error classification**: Typed errors mapped to JSON-RPC codes with sanitized messages (no implementation leaks)
- **Schema consistency tests**: Verify tool definitions match handler signatures
- **Graceful degradation**: Missing `.liza` directory returns structured errors instead of crashing
- **Handler-level middleware**: `withLogging` and `withRole` middleware in `middleware.go` (39 LOC) with dedicated tests (127 LOC)

**Concerns:**
- ~~No handler-level middleware~~ *(Resolved — `728249e`: extracted into `middleware.go`)*
- ~~**Imperative tool registration**~~ *(Resolved — `server_registration.go` (668 LOC) now uses a `toolDef` struct with declarative `[]toolDef` slices registered via `registerToolDefs()` loop. Tool definitions are data; registration ceremony collapsed to ~30 LOC of loop logic)*

### Agent Supervision (`internal/agent/`) ★★★★☆

**Strengths:**
- **Deterministic supervisor**: Go process wraps LLM agent, enforcing restart limits, heartbeat, lease renewal
- **Exit code 42 protocol**: Clean restart mechanism when agent needs fresh context
- **Context exhaustion handoff**: Structured notes enable continuation across agent instances
- **Strategy pattern**: Role-specific behavior cleanly separated into `strategy_doer.go`, `strategy_reviewer.go`, `strategy_orchestrator.go` — each with `WaitConfig` *(pass 2 — previously embedded in waitforwork.go)*
- **Work detection logic**: Sophisticated polling with configurable intervals per role

**Concerns:**
- ~~`supervisor.go` (637 LOC) and `waitforwork.go` (412 LOC) handle complex lifecycle logic~~ *(Significantly improved — `supervisor.go` 535 LOC, `waitforwork.go` 213 LOC. Strategy extraction and systemctl simplification reduced total agent package from ~3,100 to ~2,590 production LOC)*
- `RunSupervisor()` at 157 LOC interleaves restart logic with signal handling — well-tested but dense

### Git Operations (`internal/git/`) ★★★★☆

**Strengths:**
- **Merge-tree strategy**: Merges without touching the working directory — prevents dirty-state conflicts
- **Atomic ref updates**: Compare-and-swap on git refs prevents concurrent merge races
- **Selective file sync**: After merge, only changed files are synced to working tree
- **Drift calculation**: Counts commits between base and target for conflict prediction
- **Comprehensive rebase handling**: Conflict detection with structured error types

**Concerns:**
- ~~`worktree.go` (591 LOC, 35 functions) mixes 5 concerns~~ *(Resolved — split into 5 concern-based files: `worktree.go` (174 LOC, 7 functions — CRUD), `merge.go` (232 LOC, 10 functions), `rebase.go` (55 LOC, 4 functions), `query.go` (52 LOC, 4 functions), `git.go` (105 LOC, 10 functions — struct/constructor/exec helpers). Total: 618 LOC, 35 functions across 5 files)*

### State Validation (`internal/statevalidate/`) ★★★★★

**Strengths:**
- **43+ validation rules**: Every state mutation runs through comprehensive checks
- **Rule separation from ops**: Validation is a distinct package, not mixed into business logic
- **Pipeline-aware validation**: Rules adapt to custom pipeline states
- **Well-documented invariants**: Doc comments on each validation function explain the invariant it protects

**Concerns:**
- ~~Lowest test-to-production ratio in the codebase at 0.75:1~~ *(Improved — now 1.33:1 (1,029 test LOC for 774 production LOC, 24 test functions). Edge-case tests added for task validation branches in `validate_task_test.go`)*

### CLI Entry Point (`cmd/liza/`) ★★★★☆

*(pass 2 — upgraded from ★★★☆☆ after structural split)*

**Strengths:**
- **Thin delegation**: Each command's `RunE` averages 5-15 lines — parses flags and delegates to `commands` or `ops`
- **Consistent flag patterns**: All commands follow the same structure
- **Domain-based organization**: Split into 7 files — `main.go` (95 LOC), `cmd_task.go` (280), `cmd_system.go` (489), `cmd_agent.go` (241), `cmd_review.go` (183), `cmd_init.go` (127), `cmd_worktree.go` (123) *(pass 2)*

**Concerns:**
- ~~`main.go` at 1,462 LOC is the largest file in the codebase~~ *(Resolved — split into 7 domain-specific files; `main.go` reduced to 95 LOC with `main()`, `rootCmd`, shared helpers, and `init()`)*
- The `init()` function's registration block must be maintained in sync with command definitions — no compile-time enforcement prevents a command from being defined but not registered

### CLI Commands (`internal/commands/`) ★★★★☆

**Strengths:**
- **Thin command layer**: Commands delegate to ops, never contain business logic
- **Comprehensive coverage**: 75 files covering every system operation
- **Consistent patterns**: Each command follows the same structure (flag parsing → ops call → output formatting)
- **Shared rendering infrastructure**: `internal/render/` package (175 LOC) extracts common formatting

**Concerns:**
- `watch.go` (645 LOC) and `status.go` (449 LOC) are large files (watch.go is ~99% business logic; status.go delegates formatting to `internal/render/`)

### Prompt Building (`internal/prompts/`) ★★★★☆

**Strengths:**
- **Template-based agent initialization**: Structured bootstrap prompts with embedded contracts, role definitions, and task context
- `builder.go` (421 LOC) generates agent initialization context; `wake.go` (179 LOC) encapsulates wake trigger subsystem
- **Strong test ratio**: 1,927 test LOC for 635 production LOC (3.03:1)

**Concerns:**
- ~~Single large file — could be decomposed into template sections~~ *(Resolved — wake trigger subsystem extracted to `wake.go`)*

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
- Contract documents are necessarily large (CORE.md at 750 lines) — agents must read them fully, consuming context window budget
- The archived contract versions (`contracts/_archive/`) suggest rapid evolution — no migration guide between contract versions
- Skills lack versioning — when a skill protocol changes, all sessions see the new version immediately


---

## Testing & Quality Infrastructure ★★★★★

**Strengths:**
- **2.43:1 test-to-production ratio** *(was 2.45:1)*
- **Pure standard library testing**: No external test frameworks — reduces dependency surface
- **Table-driven tests throughout**: 80+ files use `t.Run()` subtests with structured test cases
- **Race detection enabled by default**: `-race` flag in all CI runs
- **Test quality enforcement**:
  - `parallel_usage_test.go`: Ratcheting minimum for `t.Parallel()` calls (currently ≥10)
  - `sleep_usage_test.go`: Prevents `time.Sleep` in tests — enforces real concurrency patterns
  - `check-testhelpers`: Pre-commit hook ensures test utilities don't leak into production
- **Integration tests**: 5 E2E test files (2,102 LOC) covering concurrent operations, lease expiry, sprint management, and full workflows — runs in CI via `make test-e2e`
- **Isolated test environments**: Every test gets `t.TempDir()` with fresh git repo and `.liza` directory
- **Per-package test health**: All 19 non-trivial packages have test ratios ≥ 0.75:1; 11 packages exceed 1.5:1 *(pass 2)*

**Concerns:**
- No coverage threshold enforced in CI — Codecov is configured for reporting only; no `.codecov.yml` exists
- Python utilities have no active test suite despite pytest being configured
- No mutation testing or fuzz testing

---

## Pre-Commit & CI Pipeline ★★★★☆

**23 pre-commit hooks covering:** *(was 21)*
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
- No binary size tracking (9.2 MB liza binary could grow unnoticed)

---

## Documentation & Specifications ★★★★★

**Extraordinary specification depth:**

| Category | Files | Contents |
|----------|-------|----------|
| Specs | 103 *(was 98)* | Vision, epics, user stories, architecture, protocols, 45 ADRs *(was 41)* |
| Contracts | 9 | Behavioral governance for agents |
| Skills | 20 | Domain-specific agent protocols |
| Docs | 23 | User guides, recipes, troubleshooting, demos |
| Lessons | 5 | Operational lessons for agents and humans |

**Highlights:**
- **45 Architecture Decision Records** — comprehensive design rationale capture *(4 new: MCP RBAC, legacy pipeline removal, role strategy, generic claim type)*
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

### Priority 2: Medium Impact / Medium Risk

#### 2.1 Enforce Coverage Threshold
- Add minimum coverage gate in CI (e.g., 70% with trend tracking)
- Prevent coverage regression on PRs
- Risk: Medium — may block PRs that touch hard-to-test code paths
- Impact: Formalizes the already-strong testing culture

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

#### 2.4 Improve `statevalidate` Test Ratio — ✅ DONE *(pass 2)*
- Test ratio improved from 0.75:1 to 1.33:1 (1,029 test LOC for 774 production LOC). `validate_task_test.go` grew from ~130 to 446 LOC with edge-case coverage for task validation branches. Test functions: 20 → 24.
- Commit: `3a54845`

#### 2.5 Eliminate Magic Literals *(pass 2)*

**2.5a Event name constants** — ✅ DONE (Low risk):
- 26 `TaskEventName` typed constants defined in `models/history.go`. Constants replaced across `statevalidate`, `ops`, `commands`, and `agent` packages. A few raw literals remain in `claim_task_strategy.go` and `proceed.go`.
- Commits: `e2baae9`, `0451f44`, `5b4cd5f`, `e08d3f5`, `4e4baed`

**2.5b Resolve hardcoded `"orchestrator-1"` identity** — ✅ DONE:
- Ops layer (`AddTask`, `AddTasks`, `SupersedeTask`) now returns error on empty agent ID instead of silently defaulting. CLI commands (`add-task`, `supersede-task`) resolve via `resolveOrchestratorID`: flag → env var → `ResolveOrchestratorFromState()`. MCP schema `Default` values removed; descriptions updated to "auto-resolved from registered orchestrator". MCP handlers already used `resolveOrchestratorID` with state-based fallback.
- **Impact**: Prevents identity mismatch bugs in multi-orchestrator or renamed-orchestrator scenarios.

#### 2.6 Claim Strategy Pattern — ✅ DONE *(pass 2)*

- `claimStrategy` interface extracted to `claim_task_strategy.go` (197 LOC) with `Preconditions()`, `WorktreePhase()`, `MutateTask()`, `EventName()` methods. Three implementations: `freshClaimStrategy`, `rejectedClaimStrategy`, `integrationFixClaimStrategy`. `ClaimTask()` reduced from 296 to 247 LOC of strategy dispatch. Boolean flags eliminated.
- Commit: `9d68a78`

#### 2.7 Declarative MCP Tool Registration — ✅ DONE *(pass 2)*
- `toolDef` struct with `registerToolDefs()` loop replaces imperative registration. `server_registration.go` (668 LOC) now defines tools as `[]toolDef` data slices. Schema consistency tests preserved.
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
- The embedded contracts + skills + settings already constitute significant binary content
- Impact: Prevents gradual size creep as the contract and skill corpus grows

---

## Summary

Liza is a technically rigorous project that practices what it preaches. The behavioral contracts that govern LLM agents are themselves enforced by well-tested Go code with atomic state management, comprehensive validation, and race-free concurrency patterns. The 2.43:1 test-to-production ratio, zero TODOs, zero `nolint`, zero `panic()`, and 4-dependency runtime reflect deliberate engineering discipline.

The project's primary challenge is not code quality but **cognitive surface area**: 31,000+ lines of specifications, contracts, and skills create an extraordinary knowledge base that also presents a steep learning curve. The code itself is well-factored at the package level; the remaining concerns are: (1) 6 orchestration functions still exceed 150 LOC and (2) absent coverage enforcement. The pass 2 refactoring sprint resolved the structural issues (CLI split, git/worktree split) and the design-level concerns (claim strategy pattern, declarative MCP registration, typed event constants, dynamic orchestrator identity resolution).

**Overall Rating: A (Excellent)**

The deduction from A+ is for: (1) 6 orchestration functions >150 LOC (the longest two at ~248 LOC each) and (2) absent coverage enforcement despite strong testing culture. All P1 items from both pass 1 and pass 2 are resolved. P2 items 2.4–2.7 are resolved including orchestrator identity (2.5b); remaining P2 items are coverage enforcement (2.1) and Python tests (2.3). The codebase quality is improving between assessments.
