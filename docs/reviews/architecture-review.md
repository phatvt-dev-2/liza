# Architecture Review — Liza

**Date:** 2026-02-20
**Mode:** Enrichment (pass 4, Coverage lens)
**Reviewer:** software-architecture-review skill

---

## Phase 1: Discovery

### 1.1 Overview

Liza is a peer-supervised multi-agent coding system built in Go. Agents (Planner, Coder, Code Reviewer) coordinate through a shared YAML blackboard file, with each agent operating in its own terminal via a supervisor loop.

```
Human Input    →    Planner    →    Coder(s)    →    Code Reviewer    →    Merge
     ↓                ↓               ↓                  ↓                 ↓
  vision.md       state.yaml     git worktrees     review verdicts    integration branch
```

**Two binaries:** `liza` (CLI with 25+ cobra commands) and `liza-mcp` (MCP JSON-RPC server over stdio).

**Source size:** ~15,400 LOC production Go, ~30,800 LOC test Go (2:1 test-to-code ratio).

### 1.2 Component Walkthrough

#### models (`internal/models/`) — 673 LOC

**Purpose:** Core domain model. Task lifecycle state machine, agent state, sprint tracking.

**Observations:**
- `State` struct is the central data type — serialized to/from `state.yaml`
- `Task` struct has 30+ fields covering full lifecycle
- `TaskType` → role workflow registry (`taskWorkflows` map)
- `IsClaimable()` encodes claiming rules with dependency checking
- 12 task statuses with `IsValid()`, `IsTerminal()` methods
- ~~No explicit `CanTransition()`~~ *(pass 2: resolved)* — `taskTransitions` map with `CanTransition()` and `Transition()` methods now exist (state.go:107-137), used consistently across 13 command call sites
- No `FindTask(taskID)` method on `State` — task lookups are duplicated inline across the codebase *(pass 2, Complexity lens)*
- Pure leaf package: zero internal imports, zero external imports — clean domain boundary *(pass 3, Boundaries lens)*
- `diagnostics.go` (127 LOC) has no corresponding test file — functions `CountClaimableTasks`, `GetCoderWorkDiagnostics`, `GetReviewerWorkDiagnostics` are used by supervisor for work detection *(pass 4, Coverage lens)*

#### db (`internal/db/`) — 864 LOC

**Purpose:** Thread-safe YAML state access with file locking via `internal/filelock`.

**Pattern:** Repository pattern — `Blackboard` wraps file I/O with atomic read-modify-write.

**Observations:**
- `Read()`, `ReadCached()` (mtime-based), `Write()`, `Modify()` (atomic closure)
- Atomic write via temp file + fsync + rename — correct durability pattern
- Stale lock detection with PID checking
- `LockError` with 5 classified types (Timeout, Permission, DiskFull, Filesystem, Stale)
- `Watcher` uses fsnotify on directory (not file) to catch atomic renames
- `Metrics` for lock acquisition timing
- `GetTask()` and `UpdateTask()` exist but are underutilized — most commands bypass them for inline lookups *(pass 2, Complexity lens)*

#### agent (`internal/agent/`) — 1,667 LOC

**Purpose:** Supervisor loop, heartbeat, work detection, logging.

**Observations:**
- `supervisor.go` (1,428 LOC, 33 functions, 19% branch density) — largest production file, confirmed god file *(pass 2: metrics added)*
- `RunSupervisor()` (186 LOC): checkAbort → waitWhilePaused → handleApprovedMerges → waitForWork → claimTask → buildPrompt → executeAgent → handleExitCode
- `CLIExecutor` interface enables mock testing (supports claude, codex, gemini, vibe)
- `waitForWorkEventDriven()` (116 LOC) with fsnotify + polling fallback
- `verifyPlannerStateChanges()` (137 LOC) — 6 switch cases with repetitive before/after counting structure *(pass 2, Complexity lens)*
- `heartbeat.go`: independent Blackboard instance, 60s tick, extends lease
- `workdetection.go` (117 LOC): 6 planner wake trigger types
- `logging.go`: package-level singleton `slog.Logger`, hardcoded to stdout
- **Upward dependency on commands**: supervisor calls `commands.ClaimTaskCommand()`, `commands.WtMergeCommand()`, `commands.ClearStaleReviewClaimsCommand()` directly — orchestration layer depends on CLI handler layer *(pass 3, Boundaries lens)*
- **Core execution paths untested**: `Execute()`, `ExecuteInteractive()`, `handleApprovedMerges()`, `logTaskSubmissionIfCompleted()` at 0% statement coverage; `resumeHandoffTask()` at 11.4%. These are the actual agent loop entry points — tested indirectly via `TestSupervisorBasicLoop` with mock executor but not at statement level *(pass 4, Coverage lens)*

#### commands (`internal/commands/`) — ~4,550 LOC

**Purpose:** CLI command implementations. Each command is a standalone function.

**Pattern:** Function-per-command, each creates its own `Blackboard` instance, validates preconditions, mutates state under lock.

**Observations:**
- 25+ command implementations (add-task, claim-task, submit-review, submit-verdict, wt-merge, watch, validate, status, etc.)
- `watch.go` (516 LOC): 11 health checks with alert deduplication, comprehensive monitoring
- `validate.go` (457 LOC): 9 validators checking all state invariants, largest function `validateTaskInvariants` at 142 LOC *(pass 2)*
- `wt_merge.go` (356 LOC): merge workflow with rollback on test failure
- `format.go` (164 LOC): centralized JSON/YAML/table formatting
- Templates in `commands/templates/`: status_dashboard, agent_value, metrics_value
- **Monolithic command functions** — 4 commands exceed 180 LOC as single functions: `WtMergeCommand` (319), `ClaimTaskCommand` (310), `DeleteTaskCommand` (220), `SubmitForReviewCommand` (183). Each mixes validation, state read, business logic, state mutation, and error handling with no internal decomposition. *(pass 2, Complexity lens)*
- **Presentation+logic coupling** — 40+ `fmt.Print*` calls to stdout/stderr in non-test code; 5+ direct `os.Stdin` reads (`setup.go`, `init.go`, `delete_task.go`, `delete_agent.go`). Commands are consumed by three callers (CLI, MCP handlers, supervisor) but embed terminal I/O assumptions *(pass 3, Boundaries lens)*
- **Self-constructing infrastructure** — each command function creates fresh `paths.New()`, `db.New()`, `git.New()` instances internally; no dependency injection. When supervisor calls commands, it creates parallel Blackboard instances that don't share cache *(pass 3, Boundaries lens)*

#### cmd (`cmd/`) — 1,344 LOC

**Purpose:** Binary entry points.

**Observations:**
- `cmd/liza/main.go` (1,275 LOC, 5 functions): 1,100+ lines of inline cobra command `var` blocks + 111-line `init()` for flag registration. Business logic correctly delegates to `commands` package — complexity is organizational, not behavioral. *(pass 2, Complexity lens)*
- `cmd/liza-mcp/main.go` (69 LOC): thin stdio transport launcher. Cross-assigns version info via mutable package globals: `mcp.Version = embedded.Version` *(pass 3, Boundaries lens)*

#### mcp (`internal/mcp/`) — 1,399 LOC

**Purpose:** MCP JSON-RPC server exposing tools and resources to AI agents.

**Observations:**
- `server.go` (704 LOC): tool/resource registration, request dispatch. `registerMutationTools()` is 242 LOC of declarative tool schema definitions — LOC is mostly boilerplate, not algorithmic complexity *(pass 2, Complexity lens)*
- `handlers.go` (547 LOC, 29 functions): tool implementations delegating to `commands` package. 14% branch density — each handler is thin. *(pass 2)*
- `protocol/` subpackage (232 LOC): clean DTO types, stdio transport, error codes
- 4 registration categories: read-only tools, read-only resources, mutation tools, complex operations
- Clean adapter boundary: mcp translates JSON-RPC into commands calls, adds error classification, holds no business logic *(pass 3, Boundaries lens)*
- **Server dispatch layer untested**: `server_test.go` has only 4 tests (initialization/registration). The entire request dispatch layer — `HandleRequest`, `Run`, `classifyError`, `handleToolCall`, `handleResourceRead`, `handleNotification` — is at 0% coverage. Handlers tested directly via `handlers_test.go` (1,298 LOC), but the routing/error-classification layer has no tests *(pass 4, Coverage lens)*
- `protocol/` entirely untested: all 6 error constructors and the stdio transport (`NewStdioTransport`, `ReadRequest`, `WriteResponse`, `WriteError`) at 0% — hardwired `os.Stdin`/`os.Stdout` prevents testing *(pass 4, Coverage lens)*

#### git (`internal/git/`) — 351 LOC

**Purpose:** Git worktree and branch operations.

**Observations:**
- `CreateWorktree()`, `RemoveWorktree()`, `MergeBranch()` (ff then no-ff), `RebaseOnto()`
- Centralized `runGit()` / `runGitCombined()` helpers
- `CalculateDrift()` for worktree-to-main divergence measurement

#### prompts (`internal/prompts/`) — 258 LOC + 14 templates

**Purpose:** Role-specific prompt generation using Go `text/template`.

**Observations:**
- Template-driven: all text in `.tmpl` files, clean logic/text separation
- 14 templates: base prompt, 3 role contexts, 6 wake triggers, shared reference, integration fix
- `executeTemplate()` panics on error rather than returning it
- `PlannerContextConfig` is empty struct (placeholder)
- Template execution pattern (embed.FS + funcMap + template.Must + executeTemplate) is duplicated nearly identically in `commands/templates.go` *(pass 3, Boundaries lens)*

#### embedded (`internal/embedded/`) — 460 LOC

**Purpose:** `go:embed` for contracts and skills, Claude/MCP settings management.

**Observations:**
- Synced from source via `make sync-embedded` before build
- `WriteClaudeSettings()` and `WriteMCPSettings()` merge with existing settings
- Frontmatter management for CLAUDE.md files
- Reads `os.Stdin` directly for merge confirmation prompts (2 locations) — couples to terminal interaction *(pass 3, Boundaries lens)*
- `WriteMCPSettings()`, `mergeMCPSettings()`, `PlanGlobalFiles()` all at 0% coverage — the stdin coupling is the direct cause *(pass 4, Coverage lens)*

#### paths (`internal/paths/`) — 257 LOC

**Purpose:** Path resolution with worktree awareness.

**Observations:**
- `GetProjectRoot()` via `git rev-parse --show-toplevel`
- `ValidateTaskID()` with path traversal protection
- All standard `.liza/` paths centralized

#### Other leaf packages

- `log/` (179 LOC): YAML append log with flock (via shared `filelock` package)
- `filelock/` (new): Shared file-locking with flock, PID-based stale detection, error classification, metrics
- `analysis/` (260 LOC): Circuit breaker pattern detection (6 patterns)
- `identity/` (140 LOC): Agent ID resolution and validation
- `errors/` (45 LOC): Exit codes and `NotFoundError` type
- `testhelpers/` (733 LOC): Fixtures, git setup, assertions, utilities

### 1.3 Dependency Map

```
models/ (stable, leaf)              paths/ (stable, leaf)
   ↑                                   ↑
   ├── db/                             ├── db/
   ├── agent/                          ├── agent/
   ├── commands/                       ├── commands/
   ├── prompts/                        ├── git/
   ├── analysis/                       └── embedded/
   └── testhelpers/

errors/ (stable, leaf)              log/ (stable, leaf)
   ↑                                   ↑
   └── commands/                       └── commands/

filelock/ (stable, leaf)            mcp/protocol/ (stable, leaf)
   ↑                                   ↑
   ├── db/                             └── mcp/server
   └── log/

git/ (volatile)                     prompts/ (stable)
   ↑                                   ↑
   └── commands/                       └── agent/

db/ (stable core)
   ↑
   ├── agent/
   ├── commands/
   └── testhelpers/

commands/ (volatile, high-level)
   ↑
   ├── agent/supervisor (upward dependency — see Boundaries findings)
   └── mcp/handlers (adapter — clean boundary)
```

**No import cycles.** Dependency graph is a clean DAG. Leaf packages: `models`, `paths`, `errors`, `filelock`, `identity`, `mcp/protocol`.

**Three consumers of `commands`** *(pass 3, Boundaries lens)*: CLI (`cmd/liza`), MCP server (`mcp/handlers`), and supervisor (`agent/supervisor`). Each has different I/O expectations — terminal, JSON-RPC stdio, and background process — but commands embed terminal assumptions.

### 1.4 Coverage Checkpoint

**What exists that shouldn't?**
- `PlannerContextConfig` is an empty struct — premature abstraction or placeholder
- `commands/format.go` has bubble-sort for map keys (functional but O(n^2); `sort.Strings` exists)
- `dashboardSection` type with `"table"` format case is a no-op (line 155: just appends empty string)
- `findTaskByID()` duplicated identically in `supervisor.go:1080` and `inspect_agents.go:143`; `findTask()` (same logic, different name) in `validate.go:450` *(pass 2, Complexity lens)*

**What's implicit that should be explicit?**
- ~~Task state transitions~~ *(pass 2: resolved — explicit `taskTransitions` map now exists)*
- The "Blackboard must remain stateless beyond cache" constraint (documented in architectural-issues.md)
- Default lease duration (1800 seconds) — exists as magic number, not named constant
- The relationship between `GracePeriod` (60s in validate.go) and `LeaseGracePeriod` (120s in watch.go)
- Missing `State.FindTask(taskID)` domain method — forces 55+ inline lookups *(pass 2, Complexity lens)*
- The contract between `commands` and its consumers — commands assume terminal I/O but serve three different transports *(pass 3, Boundaries lens)*

**What's missing from the walkthrough?**
- `db/metrics.go` (113 LOC): lock timing metrics — read and noted
- `commands/status.go` (469 LOC): status dashboard rendering — read via templates

**What requires cross-file comparison?**
- ~~Flock locking pattern in db/ vs log/ (duplicated — confirmed by cross-file analysis)~~ *(resolved: extracted to `internal/filelock`)*
- `leaseDuration = 1800` fallback in supervisor.go (x2) and claim_task.go (x1) (duplicated)
- `NotFoundError` structured type vs ad-hoc `fmt.Errorf("task not found: %s")` — **25+ instances** of the ad-hoc form in non-test code *(pass 2: quantified)*
- `derefString()` in prompts/builder.go duplicates `deref` template function
- Inline task-lookup loop duplicated 55+ times across commands, agent, db packages *(pass 2, Complexity lens)*
- Template execution pattern in `commands/templates.go` vs `prompts/templates.go` — nearly identical: embed.FS + funcMap with `deref` + template.Must + executeTemplate that panics *(pass 3, Boundaries lens)*
- `os.Stdin` reads across `embedded` (2), `commands/setup` (2), `commands/init` (1), `commands/delete_task` (2), `commands/delete_agent` (1) — all tested by monkey-patching `os.Stdin` *(pass 3, Boundaries lens)*

**Coverage lens statement-level data** *(pass 4)*:

| File/Area | Statement Coverage | Notes |
|-----------|-------------------|-------|
| **Total** | **75.3%** | From `go tool cover -func` |
| `supervisor.Execute/ExecuteInteractive` | 0% | Core agent loop entry points |
| `supervisor.handleApprovedMerges` | 0% | Merge orchestration |
| `supervisor.resumeHandoffTask` | 11.4% | Complex handoff logic |
| `supervisor.RunSupervisor` | 54.4% | Main loop |
| ~~`mcp/server.HandleRequest`~~ | ~~0%~~ | ~~Request dispatch~~ *(resolved: `server_dispatch_test.go`)* |
| `mcp/server.Run` | 0% | Server main loop |
| ~~`mcp/server.classifyError`~~ | ~~0%~~ | ~~Error code mapping~~ *(resolved: all 5 branches tested)* |
| `mcp/protocol/*` | 0% | All error constructors + stdio |
| ~~`models/diagnostics.go`~~ | ~~0%~~ | ~~No test file exists~~ *(resolved: `diagnostics_test.go`)* |
| `embedded.WriteMCPSettings` | 0% | Stdin-coupled |
| `validate.validateAnomalies` | 13.3% | Only first branch tested |
| `validate.validateHandoff` | 33.3% | |
| `inspect_field.getSprintMetricsField` | 29.4% | |

**Files without any test file** *(pass 4, Coverage lens)*:
- `cmd/liza/main.go` (1,275 LOC) — CLI wiring
- ~~`internal/models/diagnostics.go` (127 LOC) — work detection logic~~ *(resolved)*
- `cmd/liza-mcp/main.go` (69 LOC) — MCP entry point
- `internal/mcp/protocol/errors.go` (68 LOC) — error constructors
- `internal/prompts/templates.go` (34 LOC) — template execution

**Coverage report reliability** *(pass 4)*: `coverage.out` shows 0% for `CanTransition`/`Transition`/`IsTerminal` despite dedicated test functions (`TestCanTransition` with 20+ table cases, `TestTaskTransition`, `TestTerminalStatesHaveNoTransitions`). These functions were added in commit `2b5d236` — the coverage report may predate that change or was generated from a partial run. Recommend re-running `make test` to regenerate.

**Complexity lens metrics** *(pass 2)*:

| File | LOC | Functions | Branch Density | Longest Function |
|------|-----|-----------|---------------|-----------------|
| supervisor.go | 1,428 | 33 | 19% | RunSupervisor (186) |
| main.go | 1,275 | 5 | 9% | init (111) |
| server.go | 704 | 22 | 5% | registerMutationTools (242) |
| handlers.go | 547 | 29 | 14% | — |
| state.go | 543 | 19 | 9% | — |
| watch.go | 516 | 15 | 14% | — |
| **claim_task.go** | **328** | **1** | — | **ClaimTaskCommand (310)** |
| **wt_merge.go** | **356** | **3** | — | **WtMergeCommand (319)** |

**Boundaries lens import analysis** *(pass 3)*:

| Package | Internal Imports | External Imports | Consumers |
|---------|-----------------|------------------|-----------|
| `models` | 0 | 0 | 6 packages |
| `paths` | 0 | 0 | 6 packages |
| `errors` | 0 | 0 | 1 package |
| `identity` | 0 | 0 | 1 binary |
| `mcp/protocol` | 0 | 0 | 1 package |
| `filelock` | 0 | flock | 2 packages |
| `log` | filelock | yaml.v3 | 1 package |
| `db` | models, filelock | fsnotify, yaml.v3 | 3 packages |
| `git` | paths | 0 | 1 package |
| `embedded` | paths | 0 | 2 (commands, liza-mcp) |
| `analysis` | models | yaml.v3 | 1 package |
| `prompts` | models | 0 | 1 package |
| `commands` | **8 packages** | yaml.v3 | 3 (agent, mcp, liza) |
| `agent` | **5 packages** (incl. commands) | 0 | 1 binary |
| `mcp` | commands, mcp/protocol, paths | 0 | 1 binary |

---

## Phase 2: Analysis

### 2.1 Analysis Framework

| # | Question | Assessment |
|---|----------|------------|
| 1 | **Problem being solved?** | Multi-agent coordination for coding tasks with peer review and human oversight |
| 2 | **Change vectors?** | New task types (stable: models, volatile: commands/supervisor), new agent providers (volatile: supervisor.CLIExecutor), new MCP tools (volatile: handlers) |
| 3 | **Constraints?** | Solo developer, Go CLI, filesystem-based state, no external services |
| 4 | **Cost of being wrong?** | State corruption is costly (manual recovery). Code changes are reversible (git). Agent misconfiguration wastes compute. |
| 5 | **Error handling?** | Errors propagate via Go conventions. Lock errors classified. State mutation errors surface to supervisor. Partial failures can leave state stuck but not corrupt (flock protection). |
| 6 | **Expected lifespan?** | Active development, evolving. v1 with accepted limitations documented. |
| 7 | **Concurrency model?** | Multiple supervisor processes, single shared file with flock. No in-process concurrency beyond heartbeat goroutine. |
| 8 | **Data ownership?** | `state.yaml` owned by Blackboard. Git state owned by worktree operations. Prompts are read-only derived. |
| 9 | **Boundaries?** | Domain layer (`models`) clean. Persistence layer (`db`) clean. Transport layers (`mcp`, `cmd`) clean. **Orchestration/command boundary is blurred**: `commands` mixes business logic with terminal I/O, and `agent` reaches down into CLI handlers rather than a service layer. *(pass 3: updated)* |
| 10 | **Runtime constraints?** | Filesystem I/O bound. Lock contention under concurrent agents. Git operations can be slow on large repos. |

### 2.2 Strengths

#### Clean Dependency Architecture

Dependencies flow inward toward stability. No import cycles. Leaf packages (`models`, `paths`, `errors`) have zero internal dependencies. The `commands` package is correctly positioned as a high-level orchestrator. This enables confident refactoring — changing a leaf package has bounded impact.

#### Excellent Test Infrastructure

2:1 test-to-code ratio with consistent patterns: table-driven tests, filesystem isolation via `t.TempDir()`, real git repos for integration, lightweight hand-written mocks. The `testhelpers` package (733 LOC) eliminates duplication across test files. Integration tests in `internal/integration/` verify complete workflows. All `internal/` packages have tests.

#### Atomic State Persistence

The `Blackboard.Modify()` pattern (read-lock-mutate-write-unlock) combined with temp file + fsync + rename provides correct durability guarantees. The `ReadCached()` mtime-based invalidation avoids unnecessary file reads. This is the right level of complexity for a file-based coordination mechanism.

#### Comprehensive Health Monitoring

`watch.go` implements 11 distinct health checks (expired leases, blocked tasks, orphaned rejections, review loops, integration failures, hypothesis exhaustion, reassignment, approaching limits, stall detection, stale drafts, immediate discoveries) with alert deduplication and throttling. This provides operational visibility appropriate for a system that runs unattended.

#### Template-Driven Prompt Generation

All prompt text lives in `.tmpl` files, cleanly separated from Go logic. 14 templates cover all role contexts and wake trigger types. Adding new prompt content requires no Go code changes. The `prompts` package depends only on `models` — minimal coupling.

#### Well-Classified Error System

The `LockError` taxonomy (5 types with `classifyLockError()` mapping syscall errors) and the MCP error code mapping provide actionable error information at system boundaries. Error wrapping with `%w` is used consistently.

#### Explicit Task State Machine *(pass 2)*

The `taskTransitions` map (state.go:109-122) with `CanTransition()` and `Transition()` methods provides a complete, explicit state machine. All production code uses `Transition()` (13 call sites); only test fixtures set status directly. This was a known gap resolved since pass 1.

#### Clean MCP Adapter Boundary *(pass 3, Boundaries lens)*

The `mcp` package is a textbook adapter: it translates JSON-RPC wire format into `commands` function calls, maps errors to protocol error codes, and holds zero business logic. The `protocol/` sub-package cleanly separates wire types from handler logic. This is the correct structural pattern for a transport adapter.

### 2.3 Smells

#### Smell: God class in `supervisor.go`

**Signal:** 1,428 LOC, 33 functions, 19% branch density (highest in codebase), mixes agent lifecycle, work detection, task claiming, prompt building, CLI execution, merge handling, and planner verification. *(pass 2: metrics added)*

**Impact:** Difficult to understand, modify, or test any single concern in isolation. The file is the primary change target for most new features.

**Direction:** Extract cohesive subsystems: `claimManager` (claiming logic), `mergeHandler` (approved merge workflow), `plannerVerifier` (post-execution verification). The existing `CLIExecutor` interface shows the pattern.

#### Smell: Monolithic command functions *(pass 2, Complexity lens)*

**Signal:** 4 command functions exceed 180 LOC with no internal decomposition: `WtMergeCommand` (319 LOC), `ClaimTaskCommand` (310 LOC), `DeleteTaskCommand` (220 LOC), `SubmitForReviewCommand` (183 LOC). Each mixes validation, state read, business logic, state mutation, and error handling in a single function body.

**Impact:** Functions this long resist comprehension, make code review difficult, and inhibit targeted testing of individual phases (e.g., cannot test the validation phase of ClaimTask independently). More pervasive than supervisor.go because it affects 4+ files, not one.

**Direction:** Decompose into phases. `ClaimTaskCommand` already documents its three-phase structure in comments ("Phase 1: Validate Under Lock", "Phase 2: Handle worktree outside lock", "Phase 3: Re-validate and commit under lock") — extract each phase into a named function. Apply same pattern to WtMerge, DeleteTask, SubmitForReview.

#### Smell: Pervasive task-lookup duplication *(pass 2, Complexity lens)*

**Signal:** The pattern `for i := range state.Tasks { if state.Tasks[i].ID == taskID { task = &state.Tasks[i]; break } }` appears **55+ times** in non-test code. `findTaskByID()` is duplicated identically in `supervisor.go:1080` and `inspect_agents.go:143`. `findTask()` (same logic, different name) exists in `validate.go:450`. Meanwhile, `Blackboard.GetTask()` and `Blackboard.UpdateTask()` exist but are barely used.

**Impact:** Bug fixes to task-lookup logic (e.g., adding validation) require 55+ changes. The `State` type lacks a `FindTask()` method, forcing every consumer to reimplement the same 6-line loop. This is the largest single DRY violation in the codebase.

**Direction:** Add `State.FindTask(taskID string) *Task` method to `internal/models/`. Migrate inline lookups incrementally. Remove duplicated `findTaskByID`/`findTask` helpers.

#### Smell: Commands as presentation+logic hybrid *(pass 3, Boundaries lens)*

**Signal:** The `commands` package serves three consumers with different I/O expectations: CLI (terminal), MCP server (JSON-RPC over stdio), and supervisor (background process). Yet command functions contain **40+ `fmt.Print*` calls** to stdout/stderr and **5+ direct `os.Stdin` reads** in non-test production code. Functions like `ClaimTaskCommand()` print success messages, `SetupCommand()` prompts for confirmation, and `DeleteTaskCommand()` reads interactive input.

**Impact:**
- MCP server: commands called via `handlers.go` print to stdout, which is the JSON-RPC transport channel — stdout writes from commands could corrupt the protocol stream
- Supervisor: `commands.ClaimTaskCommand()` and `commands.WtMergeCommand()` called from `supervisor.go` print to the supervisor's terminal, mixing operational output with supervisor logs
- Testability: tests must monkey-patch `os.Stdin` (observed in 8+ test files replacing `os.Stdin` with pipe readers) rather than injecting I/O dependencies

**Direction:** Separate business logic from presentation. Options: (1) command functions return structured results, callers handle presentation; (2) inject an `io.Writer` for output; (3) use a `Writer` field on a command context struct. The MCP adapter already does this partially — `handleStatus()` calls `StatusCommand()` which returns a string. Extend this pattern to mutation commands.

#### Smell: Upward dependency — agent → commands *(pass 3, Boundaries lens)*

**Signal:** The supervisor (`internal/agent/supervisor.go`) directly calls three `commands` package functions: `ClaimTaskCommand()` (line 926), `WtMergeCommand()` (line 1250), `ClearStaleReviewClaimsCommand()` (line 383). It also imports `commands.IntegrationFailedError` for error type checking (line 1253).

**Impact:** The `commands` package was designed as CLI handlers (evidenced by doc.go: "Each command corresponds to a subcommand in the liza CLI"). Having the orchestration layer (`agent`) depend on the CLI layer creates a conceptual inversion: the higher-abstraction supervisor depends on lower-abstraction CLI plumbing. This also inherits the presentation coupling — when the supervisor calls `ClaimTaskCommand()`, it gets terminal output as a side effect.

**Direction:** Extract the business logic from `ClaimTaskCommand`, `WtMergeCommand`, and `ClearStaleReviewClaimsCommand` into functions that return structured results (no I/O). Both `commands` and `agent` can call these functions. This aligns with the monolithic command function decomposition — the "phases" extracted from commands become the shared service layer.

#### Smell: Interactive stdin in library packages *(pass 3, Boundaries lens)*

**Signal:** Direct `os.Stdin` reads via `bufio.NewReader(os.Stdin)` or `bufio.NewScanner(os.Stdin)` in:
- `embedded/embedded.go`: `WriteClaudeSettings()` (line 253), `WriteMCPSettings()` (line 389)
- `commands/setup.go`: setup confirmation (line 58), per-file overwrite (line 76)
- `commands/init.go`: worktree conflict resolution (line 110)
- `commands/delete_task.go`: deletion confirmation (lines 91, 126)
- `commands/delete_agent.go`: force-delete confirmation (line 76)

Tests work around this by replacing `os.Stdin` with pipe readers (8+ test files use `os.Stdin = r` / `defer func() { os.Stdin = oldStdin }()` pattern).

**Impact:** Functions with hardwired stdin cannot be used non-interactively (e.g., from MCP server or automated scripts) without the monkey-patching workaround. The `os.Stdin` swap pattern in tests is fragile and not concurrency-safe.

**Direction:** Accept an `io.Reader` parameter (or a `Confirmer` interface/callback) for interactive prompts. Default to `os.Stdin` at the call site in `cmd/liza/main.go`.

#### ~~Smell: Duplicated file locking mechanism~~ *(resolved)*

**Signal:** `db/blackboard.go` and `log/logger.go` independently defined identical `DefaultLockTimeout` (10s) and `LockCheckInterval` (100ms) constants, and implemented structurally identical flock polling loops.

**Fix:** Extracted to `internal/filelock` package with the db package's enriched version (stale lock recovery, PID tracking, error classification, metrics) as the basis. Both `db.Blackboard` and `log.Logger` now delegate to `filelock.FileLock`. The log package gained stale lock recovery and error classification it previously lacked. No external consumers of the old `db.LockError` types existed, so no aliases were needed.

#### Smell: Hardcoded configuration — magic number 1800

**Signal:** `leaseDuration = 1800` appears as a fallback default in `supervisor.go` (lines 469, 860) and `claim_task.go` (line 104), all with identical `if leaseDuration <= 0 { leaseDuration = 1800 }` pattern.

**Impact:** Changing the default requires finding and updating 3 locations. The `heartbeat.go` package already defines `DefaultLeaseDuration = 30 * time.Minute` as a named constant but in `time.Duration` form, creating a semantic split.

**Direction:** Define `DefaultLeaseDurationSeconds = 1800` alongside `Config` in models or use `heartbeat.DefaultLeaseDuration` consistently.

#### ~~Smell: MCP handlers bypass Blackboard locking~~ *(resolved)*

**Signal:** `mcp/handlers.go` read `state.yaml` directly via `os.ReadFile()` (for the `liza://state` resource) instead of going through `db.Blackboard.Read()`.

**Fix:** Added `Blackboard.ReadRaw()` method that reads raw bytes under flock. `Server` struct now holds a `*db.Blackboard` instance. `readStateResource()` uses `s.bb.ReadRaw()` instead of `os.ReadFile()`. `ReadRaw` (rather than `Read` + re-marshal) avoids the YAML round-trip data loss issue.

#### Smell: Inconsistent "not found" error types

**Signal:** `internal/errors` defines `NotFoundError` struct, but **25+ call sites** use `fmt.Errorf("task not found: %s", id)`. The structured type is used only by inspect commands. *(pass 2: quantified — 25+ instances vs "most call sites")*

**Impact:** Callers cannot reliably distinguish "not found" from other errors without string matching. Scale of the inconsistency (25+) means adoption requires systematic migration.

**Direction:** Use `NotFoundError` consistently across db and commands packages. Consider a `State.FindTask()` method that returns `NotFoundError` by default.

#### Smell: `executeTemplate` panics on error

**Signal:** `prompts/templates.go:31` and `commands/templates.go:28` both call `panic("template: " + err.Error())` on template execution failure.

**Impact:** In the supervisor's long-running process, a malformed template crashes the entire agent rather than surfacing a recoverable error. *(pass 3: noted that `commands/templates.go` has the same pattern)*

**Direction:** Return error, let caller handle.

#### Smell: Non-injectable stdio in MCP transport

**Signal:** `NewStdioTransport()` hardwires `os.Stdin`/`os.Stdout`. Cannot inject readers/writers for testing.

**Impact:** Testing requires OS-level pipe manipulation instead of simple interface injection.

**Direction:** Accept `io.Reader`/`io.Writer` parameters.

#### Smell: Scattered poll/wait magic numbers

**Signal:** `getRoleWaitConfig()` in supervisor.go has 6 inline fallback values (30, 60, 1800) for poll intervals and max wait durations.

**Impact:** Configuration defaults are invisible to code review and maintenance.

**Direction:** Define named constants alongside the Config struct.

#### ~~Smell: Untested critical execution paths~~ *(pass 4, Coverage lens — partially resolved)*

**Signal:** The system's most critical runtime paths have 0% statement coverage:
- `supervisor.Execute()` and `ExecuteInteractive()` — the actual agent execution entry points that build `exec.Cmd`, set stdin/stdout, run the CLI, and handle exit codes
- `supervisor.handleApprovedMerges()` — orchestrates post-approval merge workflow
- `supervisor.logTaskSubmissionIfCompleted()` — completion logging
- ~~`mcp/server.HandleRequest()` — JSON-RPC request dispatch~~ *(resolved: `server_dispatch_test.go`)*
- `mcp/server.Run()` — the MCP server main loop (read request → dispatch → write response)
- ~~`mcp/server.classifyError()` — maps Go errors to JSON-RPC error codes~~ *(resolved: all 5 branches tested)*
- All `mcp/protocol/` functions — error constructors and stdio transport
- ~~`models/diagnostics.go` — all 4 functions~~ *(resolved: `diagnostics_test.go`)*

**Impact:** The tested code (helpers, validators, work detection) is exercised thoroughly, but the code that wires it all together at runtime has no direct tests. This creates a "tested parts, untested whole" pattern. ~~If `classifyError` misclassifies an error, agents get wrong retry behavior. If `HandleRequest` routing breaks, all MCP tools fail.~~ If `Execute` mishandles an exit code, the supervisor loop misbehaves. The remaining untested paths are I/O-coupled functions requiring injection seams.

The root cause is I/O coupling: functions at 0% are precisely those with hardwired `os.Stdin`, `os.Stdout`, or `os/exec.Command`. The `CLIExecutor` interface in supervisor was the right move — but it was the only such seam created.

**Direction:** ~~For `mcp/server`, the dispatch layer is pure logic~~ (done). ~~For `classifyError`, same: pure function~~ (done). ~~For `diagnostics.go`: pure functions on `*State`~~ (done). For `supervisor.Execute`/`ExecuteInteractive`: already abstracted behind `CLIExecutor` interface, which is mocked in `TestSupervisorBasicLoop`, but the `DefaultCLIExecutor` concrete implementation is untested. For `mcp/server.Run` and `protocol/stdio`: require I/O injection (see "Non-injectable stdio" smell).

#### Smell: No interface-based seams beyond CLIExecutor *(pass 3, Boundaries lens)*

**Signal:** The entire production codebase has exactly **one interface**: `CLIExecutor` in `agent/supervisor.go`. All other cross-package dependencies use concrete types: `*db.Blackboard`, `*git.Git`, `*log.Logger`, `paths.LizaPaths`. There is one test-only interface (`testingT` in `testhelpers/assertions.go`).

**Impact:** This is a deliberate simplicity choice appropriate for v1 scope. However, it means testing any package that uses `db.Blackboard` requires real file I/O (creating temp directories, writing YAML files). The `testhelpers` package exists specifically to manage this overhead. If the system grows, introducing seams at the `db` and `git` boundaries would enable faster, more isolated tests.

**Direction:** No action for v1 — the current approach works. If test suite time becomes a concern, introduce interfaces at package boundaries (particularly `db` and `git`) to enable in-memory test doubles.

### 2.4 Patterns

| Pattern | Where Used | Purpose |
|---------|------------|---------|
| Repository (Blackboard) | `internal/db/` | Encapsulates file-based state persistence with locking |
| Strategy (CLIExecutor) | `internal/agent/` | Pluggable agent CLI backend (claude, codex, gemini, vibe) |
| Command | `internal/commands/` | Each CLI command is an independent function with uniform interface |
| Template Method | `internal/prompts/` | Role-specific prompts built from shared templates |
| Observer (Watcher) | `internal/db/watcher.go` | Event-driven state change notification via fsnotify |
| Registry | `internal/models/` | Task type → role workflow mapping |
| State Machine | `internal/models/` | Explicit `taskTransitions` map with `CanTransition()`/`Transition()` *(pass 2: added)* |
| Circuit Breaker | `internal/analysis/` | Pattern detection on anomalies triggers system pause |
| Heartbeat/Lease | `internal/agent/heartbeat.go` | Agent liveness detection via periodic lease extension |
| Embed | `internal/embedded/` | Contract/skill files embedded in binary via `go:embed` |
| Adapter | `internal/mcp/` | Translates JSON-RPC wire format into commands calls *(pass 3: identified)* |

### 2.5 Test Coverage

**Overall:** ~15,400 source LOC, ~30,800 test LOC. 2:1 ratio. **75.3% statement coverage** (from `go tool cover -func`). *(pass 4: statement-level data added)*

**Well-covered:**
| Package | Ratio | Notes |
|---------|-------|-------|
| prompts | 3.9:1 | Highest |
| identity | 3.0:1 | |
| db | 2.8:1 | Includes concurrency tests |
| commands | 2.1:1 | 35 test files, 14K test LOC |
| git | 2.5:1 | Real git repos in tests |
| embedded | 2.1:1 | |
| agent | 2.2:1 | MockCLIExecutor, supervisor tests |
| models | 1.5:1 | |

**Gaps:**
- `cmd/liza-mcp/main.go` (69 LOC): zero tests
- `cmd/liza/main.go` (1,275 LOC): only 376 LOC tests (0.3:1) — CLI wiring, hard to unit test
- `internal/models/diagnostics.go` (127 LOC): zero tests, no test file — work detection functions used by supervisor *(pass 4)*
- `internal/mcp/protocol/errors.go` (68 LOC): zero tests, no test file *(pass 4)*

**Critical 0% coverage paths** *(pass 4, Coverage lens)*:
The 24.7% uncovered code concentrates in two patterns:
1. **Runtime orchestration** — `supervisor.Execute/ExecuteInteractive`, `mcp/server.HandleRequest/Run`, `mcp/server.classifyError` — the integration seams that wire tested components together
2. **I/O-coupled functions** — `embedded.WriteMCPSettings/mergeMCPSettings`, `mcp/protocol/stdio`, `DefaultCLIExecutor` — hardwired to OS-level I/O

**Partially covered functions of concern** *(pass 4)*:
| Function | Coverage | Why it matters |
|----------|----------|---------------|
| `supervisor.resumeHandoffTask` | 11.4% | Complex handoff resumption logic |
| `validate.validateAnomalies` | 13.3% | Only first branch of 5 type-specific validators tested |
| `inspect_field.getSprintMetricsField` | 29.4% | Sprint metric retrieval |
| `validate.validateHandoff` | 33.3% | Handoff invariant checking |
| `inspect_field.getConfigField` | 41.2% | Configuration field retrieval |

**I/O coupling as testability barrier** *(pass 4, Coverage lens)*: Functions at 0% coverage strongly correlate with hardwired I/O — this is the Coverage lens perspective on the Boundaries smell (pass 3). The `CLIExecutor` interface demonstrates the solution pattern: abstracting one I/O boundary enabled comprehensive supervisor testing. Applying the same pattern to `StdioTransport`, `WriteMCPSettings`, and `DeleteTaskCommand` prompts would unlock testing for the remaining 0% paths.

**Integration tests:** 4 files in `internal/integration/` (1,397 LOC) covering concurrent operations, sprint/merge workflows, e2e command sequences, lease expiry. No build tags — run with regular `go test`.

**Test patterns:** Table-driven (dominant), filesystem isolation, hand-written mocks (no frameworks), real git operations. No property-based or fuzz testing. *(pass 3: `os.Stdin` monkey-patching pattern noted as testing boundary smell — 8+ test files)*

**`check-testhelpers` build guard** *(pass 4)*: Makefile target prevents `testhelpers` import in production code — good practice for maintaining test/production boundary.

---

## Phase 3: Recommendations

| Priority | Issue | Rationale | Action |
|----------|-------|-----------|--------|
| **High** | `supervisor.go` god file (1,428 LOC, 19% branch density) | Primary change target, mixes 5+ concerns, highest complexity density | Extract `claimManager`, `mergeHandler`, `plannerVerifier` subsystems |
| **High** | Monolithic command functions (310-319 LOC single functions) *(pass 2)* | 4+ commands resist comprehension, review, and targeted testing | Decompose into named phase functions (validate → execute → commit) |
| **High** | Duplicated flock mechanism (db + log) | Constants and logic can diverge silently | Extract `internal/filelock` shared package |
| ~~**High**~~ | ~~MCP handler bypasses Blackboard locking~~ *(resolved: `Blackboard.ReadRaw()` added, `readStateResource()` uses flock-protected read)* | | |
| ~~**High**~~ | ~~Untested MCP server dispatch + `classifyError`~~ *(pass 4 — resolved: `server_dispatch_test.go` covers `HandleRequest` routing, `classifyError` all 5 branches, `handleToolCall`, `handleResourceRead`, `handleNotification`)* | | |
| ~~**High**~~ | ~~Untested `models/diagnostics.go`~~ *(pass 4 — resolved: `diagnostics_test.go` covers all 4 functions with table-driven tests)* | | |
| **Medium** | Commands presentation+logic coupling *(pass 3)* | 3 consumers with incompatible I/O expectations; MCP stdout corruption risk | Commands return structured results; callers handle presentation |
| **Medium** | agent → commands upward dependency *(pass 3)* | Orchestration depends on CLI layer; inherits presentation side effects | Extract shared business logic from commands into service functions |
| **Medium** | Task-lookup duplication (55+ inline loops) *(pass 2)* | Largest DRY violation; bug fixes require 55+ changes | Add `State.FindTask()` method, migrate incrementally |
| **Medium** | Magic number 1800 (lease default) | Scattered across 3 files, easy to make inconsistent | Define named constant in one location |
| **Medium** | `executeTemplate` panics (2 locations) | Crashes long-running supervisor process | Return error instead |
| **Medium** | Inconsistent `NotFoundError` usage (25+ ad-hoc instances) | Prevents reliable programmatic error distinction | Adopt `NotFoundError` consistently, pair with `State.FindTask()` |
| **Medium** | Interactive stdin in library packages *(pass 3)* | 8 locations hardwired to terminal; tests use fragile monkey-patching | Accept `io.Reader`/callback for prompts |
| **Medium** | Poll/wait fallback magic numbers | 6 invisible defaults in `getRoleWaitConfig` | Define named constants |
| **Medium** | `validate.validateAnomalies` at 13.3% coverage *(pass 4)* | Only 1 of 5 anomaly type validators exercised; relates to "Anomaly Detail Validation Incomplete" issue | Add test cases for all 5 anomaly type branches |
| **Medium** | `supervisor.resumeHandoffTask` at 11.4% coverage *(pass 4)* | Complex handoff resumption with minimal test exercising | Increase test coverage for handoff scenarios |
| **Low** | `StdioTransport` not injectable | Testing workaround needed | Accept `io.Reader`/`io.Writer` params |
| **Low** | `PlannerContextConfig` empty struct | Premature abstraction | Remove or document intent |
| **Low** | Duplicated template execution pattern *(pass 3)* | `commands/templates.go` and `prompts/templates.go` near-identical | Extract shared template infrastructure or accept as coincidental similarity |
| **Low** | `derefString` duplicated | In builder.go and templates.go funcMap | Use template func only |
| **Low** | `cmd/liza/main.go` (1,275 LOC) *(pass 2)* | Organizational god file; behavioral complexity is low | Consider splitting cobra definitions into per-command files if growth continues |
| **Low** | No interface-based seams *(pass 3)* | Deliberate simplicity; acceptable for v1 | Monitor test suite time; introduce seams if needed |
| **Low** | Mutable package-level version variables *(pass 3)* | `mcp.Version = embedded.Version` cross-assignment | Consider constructor parameter or build-time injection |
| **Low** | Regenerate `coverage.out` *(pass 4)* | Report shows 0% for functions with thorough tests; may predate recent commits | Run `make test` to update |
| **None** | `formatKeyValue` bubble sort | Works, small data sets, not perf-sensitive | Not worth changing |
| **None** | Global logger singleton | Acceptable for CLI scope | Not worth changing for v1 |

---

## Summary

Liza's architecture is well-suited to its constraints: a file-based multi-agent coordination system for solo developers. The dependency graph is clean with no cycles. Test coverage is excellent (2:1 ratio) with consistent patterns and strong helper infrastructure. The atomic state persistence via flock and fsync+rename is correctly implemented. Health monitoring is comprehensive. The task state machine is now explicit with a complete transition map.

**Pass 2 (Complexity lens)** added two structural findings: (1) Monolithic command functions — `WtMergeCommand` and `ClaimTaskCommand` at 310-319 LOC each are the system's longest single functions, resisting comprehension and targeted testing. (2) Task-lookup duplication — the same 6-line loop appears 55+ times, making it the largest DRY violation in the codebase.

**Pass 3 (Boundaries lens)** reveals the `commands` package as the system's central boundary concern. It serves three consumers (CLI, MCP, supervisor) with incompatible I/O expectations but embeds terminal assumptions (40+ stdout writes, 5+ stdin reads). The supervisor's upward dependency on `commands` compounds this — orchestration logic inherits CLI presentation side effects. The MCP adapter layer is clean (textbook adapter pattern), and the domain/persistence boundaries are well-drawn. The recommended path forward is separating business logic from presentation in the commands layer, which would simultaneously resolve the agent→commands coupling, enable clean MCP delegation, and eliminate the stdin monkey-patching test pattern.

**Pass 4 (Coverage lens)** adds quantitative depth: 75.3% statement coverage overall, with the uncovered 24.7% concentrated in two patterns — runtime orchestration code (supervisor Execute, MCP server dispatch) and I/O-coupled functions. The most actionable finding: `mcp/server.classifyError` and `HandleRequest` are pure logic at 0% that can be tested trivially without any refactoring. Similarly, `models/diagnostics.go` (127 LOC, 4 functions, no test file) is critical work-detection logic that's entirely untested despite being pure functions on `*State`. I/O coupling (already flagged as a Boundaries smell) is now quantitatively confirmed as the primary driver of untested critical paths — functions with hardwired `os.Stdin`/`os.Stdout`/`os/exec` account for the majority of the 0% coverage.

The primary structural concerns in priority order: (1) `supervisor.go` god file, (2) commands presentation+logic coupling, (3) monolithic command functions, ~~(4) MCP handler bypassing Blackboard locking~~ (resolved), ~~(5) untested MCP dispatch and diagnostics~~ (resolved). Items 2 and 3 are synergistic — decomposing commands into phases naturally creates the service layer that resolves the boundary issue.

---

## Appendix: File Reference

| Component | Location |
|-----------|----------|
| Domain model | `internal/models/` |
| State persistence | `internal/db/` |
| Agent supervisor | `internal/agent/` |
| CLI commands | `internal/commands/` |
| MCP server | `internal/mcp/` |
| MCP protocol types | `internal/mcp/protocol/` |
| Git operations | `internal/git/` |
| Prompt generation | `internal/prompts/` |
| Embedded assets | `internal/embedded/` |
| Path utilities | `internal/paths/` |
| Logging | `internal/log/` |
| Pattern analysis | `internal/analysis/` |
| Identity resolution | `internal/identity/` |
| Error types | `internal/errors/` |
| Test helpers | `internal/testhelpers/` |
| CLI entry point | `cmd/liza/` |
| MCP entry point | `cmd/liza-mcp/` |
| Integration tests | `internal/integration/` |
