# Code Quality Assessment and Refactoring Roadmap

* Date: 2026-03-09 (commit 972d6c26, updated post-P1 refactoring at 26c2dd2, relevance check at 6534641)
* Repository: liza
* Author: Claude Code - Opus 4.6

Prompt:
> Read the XXXX project code quality assessments (Claude and Codex versions).
> Produce a similar report for Liza, adapting the structure to the specificities of this project.

## Repository Metrics Dashboard

- **Production Code**: 23,424 lines of Go across 138 files
- **Test Code**: 56,430 lines across 120 test files (2.41:1 test-to-production ratio)
- **Test Functions**: 997+ test cases with table-driven subtests
- **Behavioral Contracts**: 1,944 lines across 9 core documents + 20 skill protocols (4,461 lines)
- **Specifications**: 98 Markdown files including 41 ADRs
- **Documentation**: 23 user-facing guides
- **Dependencies**: 4 direct (cobra, yaml.v3, flock, fsnotify) — radically minimal
- **CI/CD**: Multi-platform (Linux + macOS), Codecov integration, 21 pre-commit hooks, E2E tests in CI
- **Binaries**: liza (9.2 MB) + liza-mcp (7.2 MB) with embedded contracts/skills

## Executive Summary

Liza is a hybrid multi-agent coding orchestrator: Go-based deterministic supervisors enforce invariants while LLM agents handle judgment. The codebase demonstrates **exceptional engineering discipline** in its core runtime — minimal dependencies, comprehensive testing, atomic state management — combined with an unusually thorough specification and contract corpus that forms the product's core IP.

**Key Strengths:**
- **Test-first culture**: 2.41:1 test-to-production ratio with race detection, parallelization enforcement, and sleep guards
- **Radical dependency minimalism**: 4 direct dependencies for the entire Go runtime
- **Zero TODOs in production Go code**: Clean codebase with no deferred maintenance
- **Atomic state management**: flock + temp-write + fsync + rename pattern prevents corruption
- **Specification-driven development**: 98 spec files + 41 ADRs create extraordinary traceability

**Areas for Improvement:**
- **Coverage reporting gap**: Codecov configured but coverage threshold not enforced in CI
- **Python layer underspecified**: Supporting Python utilities lack tests

**Overall Rating: A (Excellent)**

---

## Detailed Subsystem Analysis

### State Machine & Models (`internal/models/`) ★★★★★

**Strengths:**
- **Explicit state machine**: 13 task states with forbidden transitions hardcoded — no implicit state changes possible
- **Pipeline-aware extensibility**: Custom state names via YAML pipeline config, not just hardcoded enums
- **Complete audit trail**: Every task mutation appended to `History[]` with timestamps and actor IDs
- **Lease-based concurrency**: Time-bounded claims with stale detection prevent zombie agents
- **Thorough model tests**: 1,788 lines of tests for 937 lines of production code (1.9:1)

**Concerns:**
- The distinction between hardcoded states and pipeline-declared states adds cognitive overhead for contributors

### Operations Layer (`internal/ops/`) ★★★★☆

**Strengths:**
- **Clean service layer**: Each operation (claim, submit, merge, proceed) is its own file with focused responsibility
- **Precondition-heavy design**: Operations validate extensively before mutating, failing fast with typed errors
- **Rebase conflict handling**: `submit_review.go` detects drift and returns actionable error messages, not generic failures
- **Compare-and-swap for git refs**: Prevents lost updates during concurrent merges

**Concerns:**
- `claim_task.go` (655 LOC) and `proceed.go` (533 LOC) are on the large side — both handle multiple code paths (pipeline vs legacy, various role types)
- The proceed operation's complexity reflects genuine domain complexity (multi-phase pipeline transitions) but could benefit from method extraction

### MCP Server (`internal/mcp/`) ★★★★☆

**Strengths:**
- **Tool categorization**: Registration split into read-only, mutation, and complex operation phases
- **Error classification**: Typed errors mapped to JSON-RPC codes with sanitized messages (no implementation leaks)
- **Schema consistency tests**: Verify tool definitions match handler signatures
- **Graceful degradation**: Missing `.liza` directory returns structured errors instead of crashing

**Concerns:**
- No handler-level middleware (logging, timing, validation could be extracted)

### Agent Supervision (`internal/agent/`) ★★★★☆

**Strengths:**
- **Deterministic supervisor**: Go process wraps LLM agent, enforcing restart limits, heartbeat, lease renewal
- **Exit code 42 protocol**: Clean restart mechanism when agent needs fresh context
- **Context exhaustion handoff**: Structured notes enable continuation across agent instances
- **Work detection logic**: Sophisticated polling with configurable intervals per role

**Concerns:**
- `supervisor.go` (637 LOC) and `waitforwork.go` (412 LOC) handle complex lifecycle logic that could be decomposed
- The supervisor's restart logic interleaves with signal handling — edge cases are well-tested but the code is dense

### Git Operations (`internal/git/`) ★★★★★

**Strengths:**
- **Merge-tree strategy**: Merges without touching the working directory — prevents dirty-state conflicts
- **Atomic ref updates**: Compare-and-swap on git refs prevents concurrent merge races
- **Selective file sync**: After merge, only changed files are synced to working tree
- **Drift calculation**: Counts commits between base and target for conflict prediction
- **Comprehensive rebase handling**: Conflict detection with structured error types

### State Validation (`internal/statevalidate/`) ★★★★★

**Strengths:**
- **43+ validation rules**: Every state mutation runs through comprehensive checks
- **Rule separation from ops**: Validation is a distinct package, not mixed into business logic
- **Pipeline-aware validation**: Rules adapt to custom pipeline states

**Concerns:**
None

### CLI Commands (`internal/commands/`) ★★★★☆

**Strengths:**
- **Thin command layer**: Commands delegate to ops, never contain business logic
- **Comprehensive coverage**: 75 files covering every system operation
- **Consistent patterns**: Each command follows the same structure (flag parsing → ops call → output formatting)

**Concerns:**
- `watch.go` (645 LOC) and `status.go` (444 LOC) contain significant presentation logic
- Some commands have grown to include formatting helpers that could live in a `render` or `display` package

### Prompt Building (`internal/prompts/`) ★★★★☆

**Strengths:**
- **Template-based agent initialization**: Structured bootstrap prompts with embedded contracts, role definitions, and task context
- `builder.go` (598 LOC) generates complete agent initialization context

**Concerns:**
- Single large file — could be decomposed into template sections (role prompt, task prompt, tool config)

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
- The most project-specific failure mode is cross-surface drift: contract/spec text, prompt construction, embedded assets, and installed copies must all stay aligned

---

## Testing & Quality Infrastructure ★★★★★

**Strengths:**
- **2.41:1 test-to-production ratio**: Exceptionally thorough coverage
- **Pure standard library testing**: No external test frameworks — reduces dependency surface
- **Table-driven tests throughout**: 80+ files use `t.Run()` subtests with structured test cases
- **Race detection enabled by default**: `-race` flag in all CI runs
- **Test quality enforcement**:
  - `parallel_usage_test.go`: Ratcheting minimum for `t.Parallel()` calls (currently ≥10)
  - `sleep_usage_test.go`: Prevents `time.Sleep` in tests — enforces real concurrency patterns
  - `check-testhelpers`: Pre-commit hook ensures test utilities don't leak into production
- **Integration tests**: 5 E2E test files covering concurrent operations, lease expiry, sprint management, and full workflows — runs in CI via `make test-e2e`
- **Isolated test environments**: Every test gets `t.TempDir()` with fresh git repo and `.liza` directory

**Concerns:**
- No coverage threshold enforced in CI — Codecov is configured for reporting only
- Python utilities have no active test suite despite pytest being configured
- No mutation testing or fuzz testing

---

## Pre-Commit & CI Pipeline ★★★★☆

**21 pre-commit hooks covering:**
| Category | Hooks |
|----------|-------|
| **Go quality** | go-fmt, goimports, go-vet, staticcheck, go-mod-tidy |
| **Python quality** | ruff (lint + format), mypy, debug-statements |
| **Cross-language** | jscpd (duplicate detection), check-testhelpers |
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

| Category | Files | Lines | Contents |
|----------|-------|-------|----------|
| Specs | 98 | 13,824 | Vision, epics, user stories, architecture, protocols, ADRs |
| Contracts | 9 | 1,944 | Behavioral governance for agents |
| Skills | 20 | 4,461 | Domain-specific agent protocols |
| Docs | 23 | 7,028 | User guides, recipes, troubleshooting, demos |
| Lessons | 5 | — | Operational lessons for agents and humans |

**Highlights:**
- **41 Architecture Decision Records** — comprehensive design rationale capture
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

### Priority 2: Medium Impact / Medium Risk

#### 2.1 Enforce Coverage Threshold
- Add minimum coverage gate in CI (e.g., 70% with trend tracking)
- Prevent coverage regression on PRs
- Risk: Medium — may block PRs that touch hard-to-test code paths
- Impact: Formalizes the already-strong testing culture

#### 2.2 Extract Command Presentation Logic
- Move formatting/rendering helpers from `watch.go` (645 LOC) and `status.go` (444 LOC) into a `render` or `display` package
- Risk: Medium — touches widely-used commands
- Impact: Establishes clear boundary between business logic and presentation

#### 2.3 Python Test Coverage
- Add tests for Python utilities (markdown processing, analysis scripts)
- pytest is already configured; the gap is in actual test files
- Risk: Low — additive only
- Impact: Prevents silent breakage in supporting tooling

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

Liza is a technically rigorous project that practices what it preaches. The behavioral contracts that govern LLM agents are themselves enforced by well-tested Go code with atomic state management, comprehensive validation, and race-free concurrency patterns. The 2.41:1 test-to-production ratio, zero TODOs in production code, and 4-dependency runtime reflect deliberate engineering discipline.

The project's primary challenge is not code quality but **cognitive surface area**: 31,000+ lines of specifications, contracts, and skills create an extraordinary knowledge base that also presents a steep learning curve. That cost is not only onboarding friction; it is also a change-safety problem, because multiple surfaces must remain aligned for behavior to stay trustworthy. The code itself is well-factored at the package level; the remaining structural concerns are within-file (splitting a few 600-900 line files) rather than architectural.

**Overall Rating: A (Excellent)**

The P1 refactoring resolved MCP handler, state model, and validation file-level concentration concerns. Several files remain in the 500-655 LOC range (`claim_task.go`, `supervisor.go`, `watch.go`, `builder.go`, `proceed.go`) but reflect genuine domain complexity. The remaining improvement areas (coverage enforcement, Python test coverage, fuzz testing) are P2/P3 items that don't affect the core runtime's quality.
