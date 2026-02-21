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
- ~~No `FindTask(taskID)` method on `State`~~ *(pass 2, Complexity lens — resolved)* — `FindTask(taskID)` and `FindTaskIndex(taskID)` methods now exist on `*State`, all inline lookups migrated
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
- `GetTask()` and `UpdateTask()` exist and now delegate to `State.FindTask()` internally *(pass 2, Complexity lens — resolved)*

#### agent (`internal/agent/`) — 1,667 LOC (6 files)

**Purpose:** Supervisor loop, heartbeat, work detection, logging.

**Observations:**
- ~~`supervisor.go` (1,428 LOC, 33 functions) — god file~~ *(resolved: decomposed into 6 cohesive files)*:
  - `supervisor.go` (~270 LOC) — types, interfaces, main loop
  - `registration.go` (~175 LOC) — agent identity and lifecycle
  - `waitforwork.go` (~300 LOC) — work detection (event-driven + polling)
  - `claiming.go` (~230 LOC) — task claiming and merge handling
  - `prompt.go` (~95 LOC) — prompt assembly
  - `systemctl.go` (~160 LOC) — system control, execution, planner verification
- `RunSupervisor()` (186 LOC): checkAbort → waitWhilePaused → handleApprovedMerges → waitForWork → claimTask → buildPrompt → executeAgent → handleExitCode
- `CLIExecutor` interface enables mock testing (supports claude, codex, gemini, vibe)
- `waitForWorkEventDriven()` (116 LOC) with fsnotify + polling fallback
- `verifyPlannerStateChanges()` (137 LOC) — 6 switch cases with repetitive before/after counting structure *(pass 2, Complexity lens)*
- `heartbeat.go`: independent Blackboard instance, 60s tick, extends lease
- `workdetection.go` (117 LOC): 6 planner wake trigger types
- `logging.go`: package-level singleton `slog.Logger`, hardcoded to stdout
- ~~**Upward dependency on commands**: supervisor calls `commands.ClaimTaskCommand()`, `commands.WtMergeCommand()`, `commands.ClearStaleReviewClaimsCommand()` directly — orchestration layer depends on CLI handler layer~~ *(pass 3, Boundaries lens — resolved: extracted to `internal/ops/` package, agent now imports `ops` instead of `commands`)*
- **Core execution paths untested**: `Execute()`, `ExecuteInteractive()`, `handleApprovedMerges()`, `logTaskSubmissionIfCompleted()` at 0% statement coverage; `resumeHandoffTask()` at 11.4%. These are the actual agent loop entry points — tested indirectly via `TestSupervisorBasicLoop` with mock executor but not at statement level *(pass 4, Coverage lens)*

#### ops (`internal/ops/`) — ~2,700 LOC (19 files)

**Purpose:** Pure business logic layer for all task workflow and system operations. Returns structured results with no terminal I/O side effects.

**Pattern:** Service layer — extracted from `commands` to break the agent→commands upward dependency and eliminate MCP protocol corruption risk.

**Observations:**
- 19 operations covering all mutation commands:
  - Task workflow: `ClaimTask`, `SubmitForReview`, `SubmitVerdict`, `Handoff`, `MarkBlocked`, `ReleaseClaim`, `SupersedeTask`, `AddTask`, `CheckDeleteTask`, `DeleteTask`
  - Agent lifecycle: `DeleteAgent`, `IsAgentProcessRunning`
  - System mode: `Start`, `Stop`, `Pause`, `Resume`
  - Worktree: `CreateWorktree`, `DeleteWorktree`, `MergeWorktree`
  - Sprint: `UpdateSprintMetrics`, `Checkpoint`, `Analyze`
  - Maintenance: `ClearStaleReviewClaims`
- Each function returns a typed result struct (e.g., `*VerdictResult`, `*HandoffResult`, `*ModeChangeResult`)
- Zero `fmt.Print*` or `os.Stdin` calls — verified by grep
- Three consumers: `agent/` (orchestration), `commands/` (CLI presentation), `mcp/` (JSON-RPC adapter)
- Depends on: `db`, `models`, `git`, `log`, `paths`, `analysis` — same layer as `commands` minus presentation concerns

#### commands (`internal/commands/`) — ~2,800 LOC

**Purpose:** CLI presentation wrappers over `ops/` business logic, plus read-only query commands.

**Pattern:** Thin wrapper per command: call `ops.*`, format and print result. Read-only commands (inspect, status, validate) retain their own logic since they already return structured data.

**Observations:**
- 25+ command implementations — mutation commands are thin wrappers (~20-75 LOC each), read-only commands retain logic
- `watch.go` (516 LOC): 11 health checks with alert deduplication, comprehensive monitoring
- `validate.go` (457 LOC): 9 validators checking all state invariants, largest function `validateTaskInvariants` at 142 LOC *(pass 2)*
- ~~`wt_merge.go` (356 LOC)~~ now ~60 LOC wrapper over `ops.MergeWorktree()`
- `format.go` (164 LOC): centralized JSON/YAML/table formatting
- Templates in `commands/templates/`: status_dashboard, agent_value, metrics_value
- ~~**Monolithic command functions**~~ — *(pass 2, Complexity lens — resolved)* All monolithic commands extracted to `ops/`. `DeleteTaskCommand` (220→~75 LOC) was the last to be extracted, using `ops.CheckDeleteTask()` + `ops.DeleteTask()` with interactive confirmation remaining at CLI level.
- ~~**Presentation+logic coupling**~~ — *(pass 3, Boundaries lens — resolved for MCP-exposed commands)* All mutation commands now delegate to `ops/` for business logic. Remaining `fmt.Print*` calls are legitimate presentation in thin wrappers. `os.Stdin` reads remain only in CLI-only commands (`setup.go`, `init.go`, `delete_task.go`, `delete_agent.go`) — not MCP-exposed.
- **Self-constructing infrastructure** — each command function creates fresh `paths.New()`, `db.New()`, `git.New()` instances internally; no dependency injection *(pass 3, Boundaries lens)*

#### cmd (`cmd/`) — 1,344 LOC

**Purpose:** Binary entry points.

**Observations:**
- `cmd/liza/main.go` (1,275 LOC, 5 functions): 1,100+ lines of inline cobra command `var` blocks + 111-line `init()` for flag registration. Business logic correctly delegates to `commands` package — complexity is organizational, not behavioral. *(pass 2, Complexity lens)*
- `cmd/liza-mcp/main.go` (69 LOC): thin stdio transport launcher. Cross-assigns version info via mutable package globals: `mcp.Version = embedded.Version` *(pass 3, Boundaries lens)*

#### mcp (`internal/mcp/`) — 1,399 LOC

**Purpose:** MCP JSON-RPC server exposing tools and resources to AI agents.

**Observations:**
- `server.go` (704 LOC): tool/resource registration, request dispatch. `registerMutationTools()` is 242 LOC of declarative tool schema definitions — LOC is mostly boilerplate, not algorithmic complexity *(pass 2, Complexity lens)*
- `handlers.go` (~550 LOC, 29 functions): tool implementations delegating to `ops` package for mutations, `commands` package for read-only queries. 14% branch density — each handler is thin. *(pass 2; pass 5: updated — ops import added)*
- `protocol/` subpackage (232 LOC): clean DTO types, stdio transport, error codes
- 4 registration categories: read-only tools, read-only resources, mutation tools, complex operations
- Clean adapter boundary: mcp translates JSON-RPC into `ops` calls (mutations) and `commands` calls (queries), adds error classification, holds no business logic *(pass 3, Boundaries lens; pass 5: updated — handlers now import ops directly for all mutations)*
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
   ├── ops/                            ├── ops/
   ├── prompts/                        ├── git/
   ├── analysis/                       └── embedded/
   └── testhelpers/

errors/ (stable, leaf)              log/ (stable, leaf)
   ↑                                   ↑
   └── commands/                       ├── commands/
                                       └── ops/

filelock/ (stable, leaf)            mcp/protocol/ (stable, leaf)
   ↑                                   ↑
   ├── db/                             └── mcp/server
   └── log/

git/ (volatile)                     prompts/ (stable)
   ↑                                   ↑
   ├── commands/                       └── agent/
   └── ops/

db/ (stable core)
   ↑
   ├── agent/
   ├── commands/
   ├── ops/
   └── testhelpers/

ops/ (service layer — pure logic, no I/O)
   ↑
   ├── agent/ (orchestration — uses structured results)
   ├── commands/ (CLI — adds presentation)
   └── mcp/handlers (adapter — mutations via ops)

commands/ (volatile, high-level)
   ↑
   └── mcp/handlers (adapter — read-only queries via commands)
```

**No import cycles.** Dependency graph is a clean DAG. Leaf packages: `models`, `paths`, `errors`, `filelock`, `identity`, `mcp/protocol`.

**Two consumers of `commands`** *(pass 3, Boundaries lens — updated; pass 5: updated)*: CLI (`cmd/liza`) and MCP server (`mcp/handlers` — read-only queries only). ~~Supervisor (`agent/supervisor`) was a third consumer~~ — resolved by extracting business logic to `ops/`. ~~Commands still embed terminal I/O assumptions for CLI and MCP consumers~~ — resolved: MCP handlers now call `ops` directly for all mutations; `commands` only used by MCP for read-only queries (status, inspect, validate) which already return structured data.

### 1.4 Coverage Checkpoint

**What exists that shouldn't?**
- `PlannerContextConfig` is an empty struct — premature abstraction or placeholder
- `commands/format.go` has bubble-sort for map keys (functional but O(n^2); `sort.Strings` exists)
- `dashboardSection` type with `"table"` format case is a no-op (line 155: just appends empty string)
- ~~`findTaskByID()` duplicated~~ *(pass 2, Complexity lens — resolved)* — removed all 3 duplicate helpers, replaced by `State.FindTask()`

**What's implicit that should be explicit?**
- ~~Task state transitions~~ *(pass 2: resolved — explicit `taskTransitions` map now exists)*
- The "Blackboard must remain stateless beyond cache" constraint (documented in architectural-issues.md)
- Default lease duration (1800 seconds) — exists as magic number, not named constant
- The relationship between `GracePeriod` (60s in validate.go) and `LeaseGracePeriod` (120s in watch.go)
- ~~Missing `State.FindTask(taskID)` domain method~~ *(pass 2, Complexity lens — resolved)* — `FindTask` and `FindTaskIndex` added, all inline lookups migrated
- The contract between `commands` and its consumers — commands assume terminal I/O but serve three different transports *(pass 3, Boundaries lens)*

**What's missing from the walkthrough?**
- `db/metrics.go` (113 LOC): lock timing metrics — read and noted
- `commands/status.go` (469 LOC): status dashboard rendering — read via templates

**What requires cross-file comparison?**
- ~~Flock locking pattern in db/ vs log/ (duplicated — confirmed by cross-file analysis)~~ *(resolved: extracted to `internal/filelock`)*
- ~~`leaseDuration = 1800` fallback in supervisor.go (x2) and claim_task.go (x1) (duplicated)~~ *(resolved: `models.DefaultLeaseDurationSeconds` constant)*
- `NotFoundError` structured type vs ad-hoc `fmt.Errorf("task not found: %s")` — **25+ instances** of the ad-hoc form in non-test code *(pass 2: quantified)*
- `derefString()` in prompts/builder.go duplicates `deref` template function
- ~~Inline task-lookup loop duplicated 55+ times across commands, agent, db packages~~ *(pass 2, Complexity lens — resolved: `State.FindTask()`)*
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
| ~~supervisor.go~~ | ~~1,428~~ | ~~33~~ | ~~19%~~ | ~~RunSupervisor (186)~~ *(resolved: split into 6 files, largest ~270 LOC)* |
| main.go | 1,275 | 5 | 9% | init (111) |
| server.go | 704 | 22 | 5% | registerMutationTools (242) |
| handlers.go | 547 | 29 | 14% | — |
| state.go | 543 | 19 | 9% | — |
| watch.go | 516 | 15 | 14% | — |
| ~~claim_task.go~~ | ~~328~~ | ~~1~~ | — | ~~ClaimTaskCommand (310)~~ *(resolved: business logic extracted to `ops/claim_task.go`; command is now ~55 LOC presentation wrapper)* |
| ~~wt_merge.go~~ | ~~356~~ | ~~3~~ | — | ~~WtMergeCommand (319)~~ *(resolved: business logic extracted to `ops/wt_merge.go`; command is now ~60 LOC presentation wrapper)* |

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
| `ops` | 6 packages (db, models, git, log, paths, analysis) | 0 | 3 (agent, commands, mcp) |
| `commands` | **8 packages** (incl. ops) | yaml.v3 | 2 (mcp queries, liza) |
| `agent` | **5 packages** (incl. ops) | 0 | 1 binary |
| `mcp` | ops, commands, mcp/protocol, paths | 0 | 1 binary |

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
| 9 | **Boundaries?** | Domain layer (`models`) clean. Persistence layer (`db`) clean. Transport layers (`mcp`, `cmd`) clean. Service layer (`ops`) clean — pure business logic, no I/O. ~~**Remaining boundary concern**: `commands` still mixes business logic with terminal I/O for commands not yet extracted to `ops`~~ (resolved: all MCP-exposed mutations extracted to ops; commands are thin presentation wrappers; MCP imports ops directly for mutations). ~~`agent` reaches down into CLI handlers~~ (resolved: agent imports `ops`). *(pass 3: updated; pass 5: resolved — ops extraction complete)* |
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

#### ~~Smell: God class in `supervisor.go`~~ *(resolved)*

**Signal:** 1,428 LOC, 33 functions, 19% branch density (highest in codebase), mixes agent lifecycle, work detection, task claiming, prompt building, CLI execution, merge handling, and planner verification. *(pass 2: metrics added)*

**Fix:** Decomposed into 6 cohesive files within `internal/agent/`: `supervisor.go` (types + main loop), `registration.go` (identity + lifecycle), `waitforwork.go` (work detection), `claiming.go` (task claiming + merges), `prompt.go` (prompt assembly), `systemctl.go` (system control + execution + verification). Test files split correspondingly. No signature or behavior changes.

#### ~~Smell: Monolithic command functions~~ *(pass 2, Complexity lens — resolved)*

**Signal:** ~~4 command functions exceed 180 LOC~~ *(resolved: all 4 extracted to `ops/`)*. `WtMergeCommand` (319→~60 LOC), `ClaimTaskCommand` (310→~55 LOC), `SubmitForReviewCommand` (183→~30 LOC), `DeleteTaskCommand` (220→~75 LOC) are now thin wrappers. `DeleteTaskCommand` uses `ops.CheckDeleteTask()` (pre-check returning info for interactive decisions) + `ops.DeleteTask()` (business logic), following the same two-function pattern as `DeleteAgentCommand`.

**Impact:** All 4 monolithic commands resolved. No command function exceeds ~75 LOC.

#### ~~Smell: Pervasive task-lookup duplication~~ *(pass 2, Complexity lens — resolved)*

**Signal:** The pattern `for i := range state.Tasks { if state.Tasks[i].ID == taskID { task = &state.Tasks[i]; break } }` appeared **55+ times** in non-test code. `findTaskByID()` was duplicated identically in `supervisor.go` and `inspect_agents.go`. `findTask()` (same logic, different name) existed in `validate.go`. Meanwhile, `Blackboard.GetTask()` and `Blackboard.UpdateTask()` existed but were barely used.

**Fix:** Added `State.FindTask(taskID string) *Task` and `State.FindTaskIndex(taskID string) int` methods to `internal/models/state.go`. Migrated all ~35 inline ID-lookup loops in non-test production code across `commands/`, `agent/`, `db/`, and `models/` packages. Removed all 3 duplicate private helpers (`findTaskByID` in supervisor.go and inspect_agents.go, `findTask` in validate.go). `Blackboard.GetTask()` and `UpdateTask()` now delegate to `State.FindTask()` internally. Filtering loops (iterating all tasks with complex conditions) were correctly left as-is — they're a different pattern.

#### ~~Smell: Commands as presentation+logic hybrid~~ *(pass 3, Boundaries lens — resolved)*

**Signal:** The `commands` package served three consumers with different I/O expectations — CLI (terminal), MCP server (JSON-RPC over stdio), and supervisor (background process) — but embedded terminal I/O assumptions.

**Fix:** Extracted business logic from all 15 MCP-exposed mutation commands into `internal/ops/` package. Each ops function returns a typed result struct with zero I/O. Command files became thin presentation wrappers (~20-50 LOC). MCP handlers now call `ops` directly for all mutations. `handlers.go` imports `commands` only for read-only queries (InspectCommand, StatusCommand, ValidateCommand) which already return structured data. The MCP protocol corruption risk is eliminated — no `fmt.Print*` calls exist in ops functions. Remaining `os.Stdin` reads are in CLI-only interactive commands not exposed to MCP (see Interactive stdin smell).

#### ~~Smell: Upward dependency — agent → commands~~ *(pass 3, Boundaries lens — resolved)*

**Signal:** The supervisor (`internal/agent/supervisor.go`) directly calls three `commands` package functions: `ClaimTaskCommand()` (line 926), `WtMergeCommand()` (line 1250), `ClearStaleReviewClaimsCommand()` (line 383). It also imports `commands.IntegrationFailedError` for error type checking (line 1253).

**Fix:** Extracted business logic from `ClaimTaskCommand`, `WtMergeCommand`, `ClearStaleReviewClaimsCommand`, and `UpdateSprintMetricsCommand` into `internal/ops/` package. Functions return structured results (`ClaimResult`, `MergeResult`, `IntegrationFailedError`) with no terminal I/O side effects. Agent package now imports `ops` instead of `commands`. Commands package rewired as thin presentation wrappers over `ops` functions.

#### Smell: Interactive stdin in library packages *(pass 3, Boundaries lens — partially resolved)*

**Signal:** Direct `os.Stdin` reads via `bufio.NewReader(os.Stdin)` or `bufio.NewScanner(os.Stdin)` in:
- `embedded/embedded.go`: `WriteClaudeSettings()` (line 253), `WriteMCPSettings()` (line 389)
- `commands/setup.go`: setup confirmation (line 58), per-file overwrite (line 76)
- `commands/init.go`: worktree conflict resolution (line 110)
- `commands/delete_task.go`: deletion confirmation (lines 91, 126)
- `commands/delete_agent.go`: force-delete confirmation (line 76) — business logic extracted to `ops.DeleteAgent()`; stdin only at CLI wrapper level

Tests work around this by replacing `os.Stdin` with pipe readers (8+ test files use `os.Stdin = r` / `defer func() { os.Stdin = oldStdin }()` pattern).

**Partial fix:** The ops extraction resolved the MCP protocol corruption risk — MCP handlers now call `ops.*` functions with zero I/O. Remaining stdin reads are in CLI-only commands not exposed to MCP. `delete_agent.go` retains interactive confirmation at the CLI level but delegates business logic to `ops.DeleteAgent()`.

**Remaining impact:** Functions with hardwired stdin still cannot be used non-interactively. The `os.Stdin` monkey-patching test pattern remains.

**Direction:** Accept an `io.Reader` parameter (or a `Confirmer` interface/callback) for interactive prompts. Default to `os.Stdin` at the call site in `cmd/liza/main.go`.

#### ~~Smell: Duplicated file locking mechanism~~ *(resolved)*

**Signal:** `db/blackboard.go` and `log/logger.go` independently defined identical `DefaultLockTimeout` (10s) and `LockCheckInterval` (100ms) constants, and implemented structurally identical flock polling loops.

**Fix:** Extracted to `internal/filelock` package with the db package's enriched version (stale lock recovery, PID tracking, error classification, metrics) as the basis. Both `db.Blackboard` and `log.Logger` now delegate to `filelock.FileLock`. The log package gained stale lock recovery and error classification it previously lacked. No external consumers of the old `db.LockError` types existed, so no aliases were needed.

#### ~~Smell: Hardcoded configuration — magic number 1800~~ *(resolved)*

**Signal:** `leaseDuration = 1800` appeared as a fallback default in 3 locations, plus 6 more magic numbers in `getRoleWaitConfig`.

**Fix:** Defined `DefaultLeaseDurationSeconds` and `Default{Coder,Planner,Reviewer}{PollInterval,MaxWait}` constants in `internal/models/state.go` alongside `Config`. All 9 fallback sites reference named constants. `heartbeat.DefaultLeaseDuration` derives from `models.DefaultLeaseDurationSeconds`.

#### ~~Smell: MCP handlers bypass Blackboard locking~~ *(resolved)*

**Signal:** `mcp/handlers.go` read `state.yaml` directly via `os.ReadFile()` (for the `liza://state` resource) instead of going through `db.Blackboard.Read()`.

**Fix:** Added `Blackboard.ReadRaw()` method that reads raw bytes under flock. `Server` struct now holds a `*db.Blackboard` instance. `readStateResource()` uses `s.bb.ReadRaw()` instead of `os.ReadFile()`. `ReadRaw` (rather than `Read` + re-marshal) avoids the YAML round-trip data loss issue.

#### Smell: Inconsistent "not found" error types

**Signal:** `internal/errors` defines `NotFoundError` struct, but **25+ call sites** use `fmt.Errorf("task not found: %s", id)`. The structured type is used only by inspect commands. *(pass 2: quantified — 25+ instances vs "most call sites")*

**Impact:** Callers cannot reliably distinguish "not found" from other errors without string matching. Scale of the inconsistency (25+) means adoption requires systematic migration.

**Direction:** Use `NotFoundError` consistently across db and commands packages. Consider a `State.FindTask()` method that returns `NotFoundError` by default.

#### ~~Smell: `executeTemplate` panics on error~~ *(resolved)*

**Signal:** `prompts/templates.go:31` and `commands/templates.go:28` both called `panic("template: " + err.Error())` on template execution failure.

**Fix:** Both `executeTemplate` (prompts) and `executeCommandTemplate` (commands) now return `(string, error)`. Error propagated through all callers: `Build{BasePrompt,PlannerContext,CoderContext,ReviewerContext}`, `buildInstructionsForWakeTrigger`, `format{AgentValue,MetricsValue}`, and `agent/prompt.go:buildPrompt`. All callers already returned `(string, error)` — propagation required no architectural changes.

#### Smell: Non-injectable stdio in MCP transport

**Signal:** `NewStdioTransport()` hardwires `os.Stdin`/`os.Stdout`. Cannot inject readers/writers for testing.

**Impact:** Testing requires OS-level pipe manipulation instead of simple interface injection.

**Direction:** Accept `io.Reader`/`io.Writer` parameters.

#### ~~Smell: Scattered poll/wait magic numbers~~ *(resolved)*

**Signal:** `getRoleWaitConfig()` had 6 inline fallback values (30, 60, 1800).

**Fix:** Resolved together with magic number 1800 — all fallbacks now reference `models.Default*` constants.

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

**I/O coupling as testability barrier** *(pass 4, Coverage lens)*: Functions at 0% coverage strongly correlate with hardwired I/O — this is the Coverage lens perspective on the Boundaries smell (pass 3). The `CLIExecutor` interface demonstrates the solution pattern: abstracting one I/O boundary enabled comprehensive supervisor testing. ~~`DeleteTaskCommand` prompts~~ (resolved: business logic extracted to `ops.CheckDeleteTask` + `ops.DeleteTask`, both fully testable without I/O). Applying the same pattern to `StdioTransport` and `WriteMCPSettings` would unlock testing for the remaining 0% paths.

**Integration tests:** 4 files in `internal/integration/` (1,397 LOC) covering concurrent operations, sprint/merge workflows, e2e command sequences, lease expiry. No build tags — run with regular `go test`.

**Test patterns:** Table-driven (dominant), filesystem isolation, hand-written mocks (no frameworks), real git operations. No property-based or fuzz testing. *(pass 3: `os.Stdin` monkey-patching pattern noted as testing boundary smell — 8+ test files)*

**`check-testhelpers` build guard** *(pass 4)*: Makefile target prevents `testhelpers` import in production code — good practice for maintaining test/production boundary.

---

## Phase 3: Recommendations

| Priority | Issue | Rationale | Action |
|----------|-------|-----------|--------|
| ~~**Medium**~~ | ~~Magic number 1800 (lease default)~~ *(resolved)* | Named constants in `models/state.go` | ~~Define named constant in one location~~ |
| ~~**Medium**~~ | ~~`executeTemplate` panics (2 locations)~~ *(resolved)* | Both `executeTemplate` and `executeCommandTemplate` return `(string, error)` | ~~Return error instead~~ |
| **Medium** | Inconsistent `NotFoundError` usage (25+ ad-hoc instances) | Prevents reliable programmatic error distinction | Adopt `NotFoundError` consistently, pair with `State.FindTask()` |
| **Medium** | Interactive stdin in library packages *(pass 3 — partially resolved: MCP-exposed commands no longer read stdin; remaining reads in CLI-only commands)* | Remaining 7 locations hardwired to terminal; tests use fragile monkey-patching | Accept `io.Reader`/callback for prompts |
| ~~**Medium**~~ | ~~Poll/wait fallback magic numbers~~ *(resolved)* | Named constants in `models/state.go` | ~~Define named constants~~ |
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

**Pass 2 (Complexity lens)** added two structural findings: (1) Monolithic command functions — `WtMergeCommand` and `ClaimTaskCommand` at 310-319 LOC each are the system's longest single functions, resisting comprehension and targeted testing. ~~(2) Task-lookup duplication — the same 6-line loop appears 55+ times, making it the largest DRY violation in the codebase~~ (resolved: `State.FindTask()` and `FindTaskIndex()` added, all inline lookups migrated).

**Pass 3 (Boundaries lens)** reveals the `commands` package as the system's central boundary concern. It serves two consumers (CLI, MCP) with different I/O expectations but embeds terminal assumptions (40+ stdout writes, 5+ stdin reads). ~~The supervisor's upward dependency on `commands` compounded this — orchestration logic inherited CLI presentation side effects~~ (resolved: business logic extracted to `internal/ops/` service layer; agent now imports `ops` instead of `commands`). The MCP adapter layer is clean (textbook adapter pattern), and the domain/persistence boundaries are well-drawn. The remaining path forward is separating business logic from presentation in the remaining commands, which would enable clean MCP delegation and eliminate the stdin monkey-patching test pattern.

**Pass 4 (Coverage lens)** adds quantitative depth: 75.3% statement coverage overall, with the uncovered 24.7% concentrated in two patterns — runtime orchestration code (supervisor Execute, MCP server dispatch) and I/O-coupled functions. The most actionable finding: `mcp/server.classifyError` and `HandleRequest` are pure logic at 0% that can be tested trivially without any refactoring. Similarly, `models/diagnostics.go` (127 LOC, 4 functions, no test file) is critical work-detection logic that's entirely untested despite being pure functions on `*State`. I/O coupling (already flagged as a Boundaries smell) is now quantitatively confirmed as the primary driver of untested critical paths — functions with hardwired `os.Stdin`/`os.Stdout`/`os/exec` account for the majority of the 0% coverage.

The primary structural concerns in priority order: ~~(1) `supervisor.go` god file~~ (resolved — decomposed into 6 files), ~~(2) commands presentation+logic coupling~~ (resolved — all 15 MCP-exposed mutation commands extracted to `internal/ops/`; MCP handlers call ops directly; protocol corruption risk eliminated), ~~(3) monolithic command functions~~ (resolved — all 4 monolithic commands extracted to ops; `DeleteTaskCommand` was the last, using the two-function pre-check + action pattern from `DeleteAgentCommand`), ~~(4) MCP handler bypassing Blackboard locking~~ (resolved), ~~(5) untested MCP dispatch and diagnostics~~ (resolved), ~~(6) agent→commands upward dependency~~ (resolved — `internal/ops/` service layer). The ops layer now contains 19 operations (~2,700 LOC) serving 3 consumers (agent, commands, mcp). Remaining concerns: interactive stdin in CLI-only commands (partially resolved — no MCP risk), inconsistent `NotFoundError` usage.

---

## Appendix: File Reference

| Component | Location |
|-----------|----------|
| Domain model | `internal/models/` |
| State persistence | `internal/db/` |
| Agent supervisor | `internal/agent/` |
| Task operations | `internal/ops/` |
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
