# Code Quality Assessment and Refactoring Roadmap

* Date: 2026-03-24 (commit e94eb6f) *(reassessment 2026-03-24)*
* Previous: 2026-03-11 (commit a815895) — 50 commits since
* Repository: liza
* Author: Claude Code - Opus 4.6
* Mode: Reassessment (after 2026-03-11 / a815895)

## Repository Metrics Dashboard

| Metric | Previous | Current | Delta |
|--------|----------|---------|-------|
| Production LOC | 24,134 (163 files) | 28,201 (200 files) | +4,067 (+17%) |
| Test LOC | 58,656 (131 files) | 75,737 | +17,081 (+29%) |
| Test-to-production ratio | 2.43:1 | 2.69:1 | +0.26 |
| Test functions | 1,037 | 1,327 | +290 (+28%) |
| Files >500 LOC | 4 | 9 | +5 |
| Functions >150 LOC | 8 | 9 non-declarative (+2 MCP declarative) | +3 |
| Specifications | 105 (46 ADRs) | 130 (51 ADRs) | +25 (+5 ADRs) |
| Contracts | 9 | 11 | +2 |
| Skills | 20 | 23 | +3 |
| Documentation | 23 guides | 24 guides | +1 |
| Lessons | 4 | 5 | +1 |
| Dependencies | 4 direct | 4 direct | — |
| Code Hygiene | Zero across all | Zero across all | — |

**Dependencies**: cobra, yaml.v3, flock, fsnotify — unchanged
**CI/CD**: Multi-platform (Linux + macOS), Codecov integration, 23 pre-commit hooks, E2E tests in CI — unchanged
**Code Hygiene**: Zero TODOs, zero `nolint` directives, zero `panic()`, zero `interface{}` in production Go code; statuses, roles, and event names are typed constants — unchanged

## Executive Summary

Liza is a hybrid multi-agent coding orchestrator: Go-based deterministic supervisors enforce invariants while LLM agents handle judgment. The codebase demonstrates **exceptional engineering discipline** in its core runtime — minimal dependencies, comprehensive testing, atomic state management — combined with an unusually thorough specification and contract corpus that forms the product's core IP.

**Key Strengths:**
- **Test discipline improving under growth**: 2.69:1 test-to-production ratio (was 2.43:1), with +17K test LOC outpacing +4K production LOC across 50 commits of feature work (multi-phase planning, quorum reviews, attempt model, replan) *(reassessment 2026-03-24)*
- **Radical dependency minimalism**: 4 direct dependencies for the entire Go runtime
- **Pristine code hygiene**: Zero TODOs, zero `nolint`, zero `panic()`, zero untyped code in production; all event names are typed constants — maintained through 50 commits of feature growth *(reassessment 2026-03-24)*
- **Atomic state management**: flock + temp-write + fsync + rename pattern prevents corruption
- **Specification-driven development**: 130 spec files + 51 ADRs create extraordinary traceability *(reassessment 2026-03-24)*
- **Consistent quality across new code**: Supporting infrastructure packages maintain test ratios between 1.3:1 and 4.5:1

**Areas for Improvement:**
- **File and function size growth accelerating**: 9 files exceed 500 LOC (was 4); 9 non-declarative functions exceed 150 LOC (was 8), longest at 294 LOC (was 257). `proceed.go` grew from 504→816 (+62%), `init.go` from 386→753 (+95%). Growth is feature-driven (multi-phase planning, quorum, brownfield init) but the trajectory is structural debt *(reassessment 2026-03-24)*
- **Coverage reporting gap**: Codecov configured but coverage threshold not enforced in CI
- **Python layer underspecified**: Supporting Python utilities lack tests

**Overall Rating: A- (Excellent with concerns)** *(was A — reassessment 2026-03-24)*

The deduction from A is for the acceleration in structural complexity: files >500 LOC more than doubled (4→9), `proceed.go` grew 62% in 50 commits, and the long-function trend is no longer "slow upward" but consistent across the ops and commands layers. Testing discipline improved (ratio 2.43→2.69), code hygiene is pristine, and all growth is feature-driven — but the structural debt is accumulating faster than it's being addressed.

---

## Detailed Subsystem Analysis

### State Machine & Models (`internal/models/`) ★★★★★

**Strengths:**
- **Explicit state machine**: Task states with pipeline-driven transitions via `TransitionWith()` — no implicit state changes possible
- **Pipeline-driven extensibility**: Custom state names via YAML pipeline config with `Resolver` providing runtime query interface
- **Complete audit trail**: Every task mutation appended to `History[]` with timestamps and actor IDs
- **Lease-based concurrency**: Time-bounded claims with stale detection prevent zombie agents
- **Thorough model tests**: 2,822 test LOC for 1,300 production LOC (2.17:1) *(was 1.78:1 — reassessment 2026-03-24)*
- **Attempt model**: New `Attempt` field with `EffectiveAttempt` helper for retry-aware orchestration *(reassessment 2026-03-24)*

**Concerns:**
- None at this time

### Operations Layer (`internal/ops/`) ★★★★☆

**Strengths:**
- **Clean service layer**: Each operation is its own file with focused responsibility
- **Precondition-heavy design**: Operations validate extensively before mutating, failing fast with typed errors
- **Rebase conflict handling**: `submit_review.go` detects drift and returns actionable error messages, not generic failures
- **Compare-and-swap for git refs**: Prevents lost updates during concurrent merges
- **Improved test ratio**: 19,926 test LOC for 7,946 production LOC (2.51:1) *(was 2.07:1 — reassessment 2026-03-24)*
- **Worktree lifecycle cleanup**: Worktrees cleaned up on both merge and supersede
- **Claim strategy pattern**: `claimStrategy` interface eliminates boolean-flag dispatch
- **Typed event constants**: `TaskEventName` typed constants used across all packages
- **Multi-phase planning**: `proceed.go` gained topo-sorted transition execution, crash recovery, and phase-gate dependency propagation *(reassessment 2026-03-24)*
- **Quorum reviews**: `submit_verdict.go` gained quorum evaluation and impact upgrade logic *(reassessment 2026-03-24)*
- **Replan operation**: New `replan.go` for re-invoking planner after plan modifications *(reassessment 2026-03-24)*

**Concerns:**
- `proceed.go` grew from 504→816 LOC (+62%) — the largest production file in the codebase. Growth driven by multi-phase planning (topo sort, crash recovery, cycle detection, inherited deps). The complexity is feature-driven but the file now contains multiple distinct concerns *(reassessment 2026-03-24)*
- `claim_task.go` (542 LOC) and `wt_merge.go` (520 LOC) both exceed 500 LOC
- `claim_reviewer_task.go` at 438 LOC is a substantial new file (quorum + diversity blocking) *(reassessment 2026-03-24)*
- 7 ops functions exceed 150 LOC: `SubmitVerdict` (275, was 185), `MergeWorktree` (271, was 251), `ClaimTask` (270, was 248), `SubmitForReview` (213), `RecoverTask` (209, was 183), `Replan` (175, new), `ExecuteAvailableTransitions` (157, new) *(reassessment 2026-03-24)*

### MCP Server (`internal/mcp/`) ★★★★☆

**Strengths:**
- **Tool categorization**: Registration split into read-only, mutation, and complex operation phases
- **Error classification**: Typed errors mapped to JSON-RPC codes with sanitized messages (no implementation leaks)
- **Schema consistency tests**: Verify tool definitions match handler signatures
- **Graceful degradation**: Missing `.liza` directory returns structured errors instead of crashing
- **Handler-level middleware**: `withLogging` and `withRole` middleware in `middleware.go` with dedicated tests
- **Declarative registration**: `toolDef` struct with `[]toolDef` data slices — tool definitions are data, not code
- **Improved test ratio**: 5,892 test LOC for 2,682 production LOC (2.20:1) *(reassessment 2026-03-24)*

**Concerns:**
- `server_registration.go` at 800 LOC (was 667) contains two registration functions (373 + 309 LOC) — these are declarative data definitions (tool schemas), not control flow, so the LOC is structural rather than concerning *(reassessment 2026-03-24)*

### Agent Supervision (`internal/agent/`) ★★★★☆

**Strengths:**
- **Deterministic supervisor**: Go process wraps LLM agent, enforcing restart limits, heartbeat, lease renewal
- **Exit code 42 protocol**: Clean restart mechanism when agent needs fresh context
- **Context exhaustion handoff**: Structured notes enable continuation across agent instances
- **Strategy pattern**: Role-specific behavior cleanly separated into strategy files — each with `WaitConfig`
- **Work detection logic**: Sophisticated polling with configurable intervals per role
- **Strong test ratio**: 9,286 test LOC for 3,154 production LOC (2.94:1) *(reassessment 2026-03-24)*
- **Registration subsystem**: `registration.go` (358 LOC) and `claiming.go` (356 LOC) manage agent lifecycle *(reassessment 2026-03-24)*

**Concerns:**
- `supervisor.go` at 588 LOC (was ~450) with `RunSupervisor()` at 171 LOC — well-tested but growing *(reassessment 2026-03-24)*

### Git Operations (`internal/git/`) ★★★★☆

**Strengths:**
- **Merge-tree strategy**: Merges without touching the working directory — prevents dirty-state conflicts
- **Atomic ref updates**: Compare-and-swap on git refs prevents concurrent merge races
- **Selective file sync**: After merge, only changed files are synced to working tree
- **Drift calculation**: Counts commits between base and target for conflict prediction
- **Comprehensive rebase handling**: Conflict detection with structured error types
- **Concern-based file organization**: `worktree.go` (CRUD), `merge.go`, `rebase.go`, `query.go`, `git.go` (helpers)

**Concerns:**
- `merge.go` at 314 LOC — within acceptable range but above the 300 LOC attention threshold

### State Validation (`internal/statevalidate/`) ★★★★★

**Strengths:**
- **Comprehensive validation rules**: Every state mutation runs through extensive checks
- **Rule separation from ops**: Validation is a distinct package, not mixed into business logic
- **Pipeline-aware validation**: Rules adapt to custom pipeline states
- **Well-documented invariants**: Doc comments on each validation function explain the invariant it protects
- **Improved test ratio**: 1,726 test LOC for 893 production LOC (1.93:1, was 1.33:1) *(reassessment 2026-03-24)*
- **Attempt validation**: New rules for `Attempt` field integrity *(reassessment 2026-03-24)*

**Concerns:**
- None at this time

### CLI Entry Point (`cmd/liza/`) ★★★★☆

**Strengths:**
- **Thin delegation**: Each command's `RunE` averages 5-15 lines — parses flags and delegates to `commands` or `ops`
- **Consistent flag patterns**: All commands follow the same structure
- **Domain-based organization**: Split into 7 files

**Concerns:**
- `cmd_system.go` grew to 537 LOC (was 489) *(reassessment 2026-03-24)*
- The `init()` function's registration block must be maintained in sync with command definitions — no compile-time enforcement prevents a command from being defined but not registered

### CLI Commands (`internal/commands/`) ★★★★☆

**Strengths:**
- **Thin command layer**: Commands delegate to ops, never contain business logic
- **Comprehensive coverage**: Files covering every system operation
- **Consistent patterns**: Each command follows the same structure (flag parsing → ops call → output formatting)
- **Shared rendering infrastructure**: `internal/render/` package (175 LOC) extracts common formatting
- **Exceptional test ratio**: 18,639 test LOC for 4,918 production LOC (3.79:1) *(reassessment 2026-03-24)*

**Concerns:**
- `init.go` grew from 386→753 LOC (+95%) with `InitCommandWithConfig` at 294 LOC (was 257) — brownfield symlinks, Node.js detection, agent flags. This is the largest single-file growth in the codebase *(reassessment 2026-03-24)*
- `watch.go` grew from 645→751 LOC (+16%) — stale sentinel detection, attempt-aware messaging *(reassessment 2026-03-24)*
- `status.go` at 449 LOC — unchanged

### Pipeline Configuration (`internal/pipeline/`) ★★★★☆ *(new subsystem section — reassessment 2026-03-24)*

**Strengths:**
- **Declarative pipeline definition**: YAML-driven state machine configuration with role-pair schemas
- **Resolver pattern**: `resolver.go` (571 LOC) provides runtime query interface for pipeline state
- **Strong test ratio**: 3,013 test LOC for 1,095 production LOC (2.75:1)
- **Quorum and review policy**: Schema extensions for multi-reviewer workflows

**Concerns:**
- `resolver.go` at 571 LOC — a single file providing the full query interface. Functional but approaching the point where concern separation would help readability

### Prompt Building (`internal/prompts/`) ★★★★☆

**Strengths:**
- **Template-based agent initialization**: Structured bootstrap prompts with embedded contracts, role definitions, and task context
- **Strong test ratio**: 1,932 test LOC for 522 production LOC (3.70:1)
- **Prior attempt context**: New `prior_attempt.tmpl` template for attempt-aware agent bootstrap *(reassessment 2026-03-24)*

**Concerns:**
- None at this time

### Supporting Infrastructure

Focused infrastructure packages maintain the project's quality discipline:

| Package | Prod LOC | Test LOC | Ratio | Purpose |
|---------|----------|----------|-------|---------|
| `internal/filelock/` | 489 | 884 | 1.81:1 | File locking with metrics and typed errors |
| `internal/db/` | 520 | 2,350 | 4.52:1 | Blackboard state persistence *(reassessment 2026-03-24)* |
| `internal/log/` | 211 | 678 | 3.21:1 | Structured agent logging |
| `internal/render/` | 175 | 234 | 1.34:1 | Shared CLI formatting |
| `internal/identity/` | 123 | 473 | 3.85:1 | Orchestrator identity resolution |
| `internal/roles/` | 76 | 146 | 1.92:1 | Role type definitions and validation |
| `internal/errors/` | 62 | 121 | 1.95:1 | Shared error types |

All follow the project's patterns: typed constants, precondition validation, table-driven tests.

---

## Behavioral Contracts & Skills ★★★★★

This is Liza's core IP and most distinctive feature.

**Strengths:**
- **Failure-mode-driven design**: 55+ documented LLM failure modes mapped to specific countermeasures in `CONTRACT_FAILURE_MODE_MAP.md`
- **Tiered rule architecture**: Tier 0 (inviolable) through Tier 3 (preferences) with explicit degradation protocol
- **Execution state machine**: States with forbidden transitions, model activation points, and stop triggers
- **23 composable skills**: Domain-specific protocols that agents load on demand *(was 20 — reassessment 2026-03-24)*
- **Three collaboration modes**: Pairing (human-supervised), Multi-Agent (peer-supervised), Subagent (delegated) — each with explicit gate semantics
- **Anti-gaming clause**: "Technically compliant is not compliant" — closes the most common loophole in agent governance

**Concerns:**
- Contract documents are necessarily large (CORE.md at 750 lines) — agents must read them fully, consuming context window budget
- The archived contract versions (`contracts/_archive/`) suggest rapid evolution — no migration guide between contract versions
- Skills lack versioning — when a skill protocol changes, all sessions see the new version immediately

---

## Testing & Quality Infrastructure ★★★★★

**Strengths:**
- **2.69:1 test-to-production ratio** — improved from 2.43:1 across 50 commits of feature work; test LOC grew 29% while production LOC grew 17% *(reassessment 2026-03-24)*
- **Pure standard library testing**: No external test frameworks — reduces dependency surface
- **Table-driven tests throughout**: Files use `t.Run()` subtests with structured test cases
- **Race detection enabled by default**: `-race` flag in all CI runs
- **Test quality enforcement**:
  - `parallel_usage_test.go`: Ratcheting minimum for `t.Parallel()` calls
  - `sleep_usage_test.go`: Prevents `time.Sleep` in tests — enforces real concurrency patterns
  - `check-testhelpers`: Pre-commit hook ensures test utilities don't leak into production
- **Integration tests**: E2E test files covering concurrent operations, lease expiry, sprint management, and full workflows — runs in CI via `make test-e2e`
- **Isolated test environments**: Every test gets `t.TempDir()` with fresh git repo and `.liza` directory
- **1,327 test functions** (was 1,037) — +290 test functions tracking feature growth *(reassessment 2026-03-24)*

**Concerns:**
- No coverage threshold enforced in CI — Codecov is configured for reporting only; no `.codecov.yml` exists
- Python utilities have no active test suite despite pytest being configured
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
- No binary size tracking (liza binary could grow unnoticed from embedded content)

---

## Documentation & Specifications ★★★★★

**Extraordinary specification depth:**

| Category | Previous | Current | Delta |
|----------|----------|---------|-------|
| Specs | 105 (46 ADRs) | 130 (51 ADRs) | +25 (+5 ADRs) |
| Contracts | 9 | 11 | +2 |
| Skills | 20 | 23 | +3 |
| Docs | 23 | 24 | +1 |
| Lessons | 4 | 5 | +1 |

*(reassessment 2026-03-24)*

**Highlights:**
- **51 Architecture Decision Records** — comprehensive design rationale capture
- **C4 diagrams** at context, container, and component levels
- **Failure mode map** connecting each contract clause to the specific LLM failure it prevents
- **Agent testimony** and **demo benchmarks** — real session transcripts showing the system in action
- **Lessons system** — operational insights organized by audience (agents vs humans)

**Concerns:**
- Some specs reference implementation details that may have drifted (normal for spec-heavy projects)
- Artifact consistency (embedded copies vs repo masters) is automated via `consistency_test.go`; higher-level spec-to-implementation drift checking remains manual
- The sheer volume of documentation could overwhelm new contributors
- The main cost of this documentation volume is not just onboarding; it also increases the chance of locally correct but systemically incomplete changes

---

## Refactoring Recommendations by Priority

### Completed (P1 — all resolved)

| # | Title | Commit(s) |
|---|-------|-----------|
| 1.1 | Decompose MCP handler monolith | `3544574`, `fd145e9` |
| 1.2 | Split state model | `82258fe` |
| 1.3 | Group validation rules | `d53a2f0` |
| 1.4 | Add artifact consistency checks | `47e5597`, `bab9a78` |
| 1.5 | Split CLI entry point | `7ac5ac8` |
| 1.6 | Group `git/worktree.go` by concern | `35e5c6d` |
| 1.7 | Replace raw event literals | `a815895` |

### Completed (P2 — resolved items)

| # | Title | Commit(s) |
|---|-------|-----------|
| 2.2 | Extract command presentation logic | `internal/render/` package |
| 2.4 | Improve `statevalidate` test ratio (0.75:1 → 1.93:1) | `3a54845` et al. *(updated — reassessment 2026-03-24)* |
| 2.5a | Event name constants (26 typed constants) | `e2baae9` et al. |
| 2.5b | Resolve hardcoded `"orchestrator-1"` identity | `40f407c` et al. |
| 2.6 | Claim strategy pattern | `9d68a78` |
| 2.7 | Declarative MCP tool registration | `5350e71` |

### Priority 1: High Impact / Low Risk *(new — reassessment 2026-03-24)*

#### 1.8 Decompose `proceed.go`
- **What**: Split `proceed.go` (816 LOC) by concern: transition execution, phase-gate dependency computation, crash recovery, and cycle detection are distinct responsibilities. Extract `computeInheritedDeps` and topo-sort logic into dedicated files.
- **Risk**: Low — each concern is already functionally separated within the file; extraction is structural
- **Impact**: Brings the largest production file back under 500 LOC; makes multi-phase planning logic navigable
- **Depends on**: None

#### 1.9 Decompose `init.go`
- **What**: Split `init.go` (753 LOC) by concern: project detection, symlink setup, config generation, and interactive prompts are distinct phases. `InitCommandWithConfig` (294 LOC) orchestrates them sequentially — extract each phase into its own function/file.
- **Risk**: Low — phases are sequential with clear data boundaries
- **Impact**: Brings the second-largest commands file under 500 LOC; simplifies brownfield init evolution
- **Depends on**: None

### Priority 2: Medium Impact / Medium Risk (open)

#### 2.1 Enforce Coverage Threshold
- **What**: Add minimum coverage gate in CI (e.g., 70% with trend tracking); prevent coverage regression on PRs
- **Risk**: Medium — may block PRs that touch hard-to-test code paths
- **Impact**: Formalizes the already-strong testing culture

#### 2.3 Python Test Coverage
- **What**: Add tests for Python utilities (markdown processing, analysis scripts); pytest is already configured
- **Risk**: Low — additive only
- **Impact**: Prevents silent breakage in supporting tooling

#### 2.8 Decompose `watch.go` *(new — reassessment 2026-03-24)*
- **What**: `watch.go` (751 LOC) contains stale sentinel detection, orphaned rejection checks, approaching-limits warnings, and reassignment messaging. Extract check functions into a `checks.go` or similar.
- **Risk**: Low — check functions are self-contained with clear inputs/outputs
- **Impact**: Brings watch below 500 LOC; makes individual checks testable in isolation

### Priority 3: Strategic / Long-term

#### 3.1 Spec-Code Consistency Automation — partially done
- `consistency_test.go` verifies byte-exact match of embedded artifacts vs repo masters
- Higher-level drift detection (e.g., blackboard schema spec vs actual YAML structure) remains manual

#### 3.2 Fuzz Testing for State Mutations
- The atomic YAML state management is critical — fuzz testing concurrent reads/writes would surface edge cases
- Go's built-in fuzzing (`go test -fuzz`) is well-suited for this

#### 3.3 Binary Size Tracking
- Track liza/liza-mcp binary sizes in CI to detect bloat from embedded content growth
- The embedded contracts + skills + settings + `console.sh` already constitute significant binary content

#### 3.4 `proceed.go` design-level review *(new — reassessment 2026-03-24)*
- After P1.8 structural decomposition, evaluate whether `ExecuteAvailableTransitions` (157 LOC) would benefit from a strategy or visitor pattern for transition types, rather than the current sequential-conditional approach
- This is contingent on P1.8 — assess after decomposition reveals the actual complexity shape

---

## Files >500 LOC *(new section — reassessment 2026-03-24)*

| File | LOC | Delta | Nature |
|------|-----|-------|--------|
| `internal/ops/proceed.go` | 816 | +312 (+62%) | Multi-phase planning, topo sort, crash recovery |
| `internal/mcp/server_registration.go` | 800 | +133 (+20%) | Declarative tool schemas (not control flow) |
| `internal/commands/init.go` | 753 | +367 (+95%) | Brownfield symlinks, Node.js, agent flags |
| `internal/commands/watch.go` | 751 | +106 (+16%) | Stale sentinels, attempt-aware messaging |
| `internal/agent/supervisor.go` | 588 | ~+138 | Agent lifecycle, restart logic |
| `internal/pipeline/resolver.go` | 571 | new subsystem growth | Pipeline query interface |
| `internal/ops/claim_task.go` | 542 | -9 | Stable |
| `cmd/liza/cmd_system.go` | 537 | +48 | New commands |
| `internal/ops/wt_merge.go` | 520 | ~+120 | Worktree cleanup, merge logic |

---

## Summary

Liza is a technically rigorous project that practices what it preaches. The behavioral contracts that govern LLM agents are themselves enforced by well-tested Go code with atomic state management, comprehensive validation, and race-free concurrency patterns. The test-to-production ratio improved from 2.43:1 to 2.69:1 across 50 commits of substantial feature work (multi-phase planning, quorum reviews, attempt model, replan), and code hygiene metrics (zero TODOs, zero `nolint`, zero `panic()`, 4 dependencies) held at zero throughout. *(reassessment 2026-03-24)*

The project's primary challenge has shifted from "cognitive surface area" to **structural complexity accumulation**. Files exceeding 500 LOC more than doubled (4→9), with `proceed.go` (+62%) and `init.go` (+95%) as the steepest growth areas. All growth is feature-driven — not bloat — but the pattern indicates that feature velocity is outpacing structural decomposition. The new P1 items (decompose `proceed.go` and `init.go`) target the two largest growth areas. *(reassessment 2026-03-24)*

**Overall Rating: A- (Excellent with concerns)** *(was A — reassessment 2026-03-24)*

The deduction from A is for: (1) files >500 LOC more than doubled (4→9) with two files growing 62-95% in 50 commits, indicating structural debt is accumulating faster than feature velocity warrants, and (2) absent coverage enforcement. The improved test ratio (2.43→2.69) and pristine hygiene prevent a further deduction.
