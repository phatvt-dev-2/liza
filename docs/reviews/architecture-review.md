# Architecture Review — Liza

**Date:** 2026-02-21
**Mode:** Adversarial (after pass 12)
**Reviewer:** software-architecture-review skill

---

## Update Policy

1. `Phase 3: Recommendations` tracks open issues only.
2. When an issue is fixed, move it to `Fixed (Traceability)` with:
   - original priority
   - resolving commit(s)
   - date moved
   - concise evidence note
3. Do not delete fixed issues from this document; keep an append-only trace history.
4. Keep inline `*(resolved)*` annotations in historical sections for narrative continuity, but use `Fixed (Traceability)` as the canonical closure log.
5. If resolution evidence is incomplete, keep the issue in Recommendations until trace data is added.

---

## Table of Contents

- [Update Policy](#update-policy)
- [Phase 1: Discovery](#phase-1-discovery)
- [1.1 Overview](#11-overview)
- [1.2 Component Walkthrough](#12-component-walkthrough)
- [1.3 Dependency Map](#13-dependency-map)
- [1.4 Coverage Checkpoint](#14-coverage-checkpoint)
- [Phase 2: Analysis](#phase-2-analysis)
- [2.1 Analysis Framework](#21-analysis-framework)
- [2.2 Strengths](#22-strengths)
- [2.3 Smells](#23-smells)
- [2.4 Patterns](#24-patterns)
- [2.5 Test Coverage](#25-test-coverage)
- [Phase 3: Recommendations](#phase-3-recommendations)
- [Fixed (Traceability)](#fixed-traceability)
- [Summary](#summary)
- [Appendix: File Reference](#appendix-file-reference)

## Phase 1: Discovery

### 1.1 Overview

Liza is a peer-supervised multi-agent coding system built in Go. Agents (Planner, Coder, Code Reviewer) coordinate through a shared YAML blackboard file, with each agent operating in its own terminal via a supervisor loop.

```
Human Input    →    Planner    →    Coder(s)    →    Code Reviewer    →    Merge
     ↓                ↓               ↓                  ↓                 ↓
  vision.md       state.yaml     git worktrees     review verdicts    integration branch
```

**Two binaries:** `liza` (CLI with 25+ cobra commands) and `liza-mcp` (MCP JSON-RPC server over stdio).

**Source size:** ~15,300 LOC production Go, ~36,700 LOC test Go (2.4:1 test-to-code ratio). *(pass 7: updated from ~15,400/~30,800)*

### 1.2 Component Walkthrough

#### models (`internal/models/`) — ~680 LOC

**Purpose:** Core domain model. Task lifecycle state machine, agent state, sprint tracking.

**Observations:**
- `State` struct is the central data type — serialized to/from `state.yaml`. `state.go` is 631 LOC with 20 structs — cohesive (all YAML-serialized state types) *(pass 5: updated LOC)*
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

#### agent (`internal/agent/`) — 1,716 LOC (6 files)

**Purpose:** Supervisor loop, heartbeat, work detection, logging.

**Observations:**
- ~~`supervisor.go` (1,428 LOC, 33 functions) — god file~~ *(resolved: decomposed into 6 cohesive files)*:
  - `supervisor.go` (~460 LOC) — types, interfaces, main loop, exit-42 restart tracker with exponential backoff and circuit breaker
  - `registration.go` (~175 LOC) — agent identity and lifecycle
  - `waitforwork.go` (~300 LOC) — work detection (event-driven + polling)
  - `claiming.go` (~160 LOC) — task claiming and merge handling (reviewer claiming and handoff resumption extracted to `ops`)
  - `prompt.go` (~95 LOC) — prompt assembly
  - `systemctl.go` (~160 LOC) — system control, execution, planner verification
- `RunSupervisor()` (186 LOC, nesting depth 5): checkAbort → waitWhilePaused → handleApprovedMerges → waitForWork → claimTask → buildPrompt → executeAgent → handleExitCode *(pass 7, Complexity lens: nesting depth noted)*
- `CLIExecutor` interface enables mock testing (supports claude, codex, gemini, vibe)
- `waitForWorkEventDriven()` (116 LOC) with fsnotify + polling fallback
- `verifyPlannerStateChanges()` (137 LOC) — 6 switch cases with repetitive before/after counting structure *(pass 2, Complexity lens)*
- `heartbeat.go`: independent Blackboard instance, 60s tick, extends lease
- `workdetection.go` (~170 LOC): 6 planner wake trigger types, now declarative via `plannerWakeTriggerSpecs` (trigger → description → state predicate) replacing imperative branching
- `logging.go`: package-level singleton `slog.Logger`, hardcoded to stdout
- ~~**Upward dependency on commands**: supervisor calls `commands.ClaimTaskCommand()`, `commands.WtMergeCommand()`, `commands.ClearStaleReviewClaimsCommand()` directly — orchestration layer depends on CLI handler layer~~ *(pass 3, Boundaries lens — resolved: extracted to `internal/ops/` package, agent now imports `ops` instead of `commands`)*
- **Core execution paths untested**: `Execute()`, `ExecuteInteractive()`, `handleApprovedMerges()`, `logTaskSubmissionIfCompleted()` at 0% statement coverage; `resumeHandoffTask()` at 11.4%. These are the actual agent loop entry points — tested indirectly via `TestSupervisorBasicLoop` with mock executor but not at statement level *(pass 4, Coverage lens)*
- **`handleApprovedMerges` high nesting-to-LOC ratio**: 55 LOC but nesting depth 6 (for-range → if-status → if-approved → if-merge-commit → if-err → errors.As). The deepest nesting processes `IntegrationFailedError` fields conditionally. ~~Relatedly, `resumeHandoffTask` (63 LOC) reaches nesting depth 5 inside its `bb.Modify` closure~~ *(pass 7, Complexity lens — partially resolved: `ac4ce6f5` extracted `resumeHandoffTask` to `ops.ResumeHandoff`)*
- ~~**Role string literals instead of constants**~~ *(pass 6, Coupling lens — mostly resolved: `a60c72e`)*: `internal/roles` package introduced with `RuntimeCoder`, `RuntimeCodeReviewer`, `RuntimePlanner` constants and `ToWorkflow()`/`ToRuntime()` mapping. All agent/, cmd/, and ops/ files now import role constants. 1 residual: `claiming.go:115` still uses `Role: "coder"` literal
- **Duplicated identity validation**: `registration.go:validateIdentity()` reimplements `identity.ValidateFormat()` + `identity.ValidateRole()` — same algorithm (split on last hyphen, validate numeric suffix, check role prefix) without importing the `identity` package *(pass 6, Coupling lens)*
- **Hardcoded `"terminal-1"` and raw `1800`**: `supervisor.go:127` passes `"terminal-1"` literal and `1800` instead of `models.DefaultLeaseDurationSeconds`; `supervisor.go:221` also uses raw `1800` *(pass 6, Coupling lens)*

#### ops (`internal/ops/`) — ~3,200 LOC production, ~5,000 LOC test (21 files)

**Purpose:** Pure business logic layer for all task workflow and system operations. Returns structured results with no terminal I/O side effects.

**Pattern:** Service layer — extracted from `commands` to break the agent→commands upward dependency and eliminate MCP protocol corruption risk.

**Observations:**
- 21 operations covering all mutation commands:
  - Task workflow: `ClaimTask`, `ClaimReviewerTask`, `SubmitForReview`, `SubmitVerdict`, `Handoff`, `ResumeHandoff`, `MarkBlocked`, `ReleaseClaim`, `SupersedeTask`, `AddTask`, `CheckDeleteTask`, `DeleteTask`
  - Agent lifecycle: `DeleteAgent`, `IsAgentProcessRunning`
  - System mode: `Start`, `Stop`, `Pause`, `Resume`
  - Worktree: `CreateWorktree`, `DeleteWorktree`, `MergeWorktree`
  - Sprint: `UpdateSprintMetrics`, `Checkpoint`, `Analyze`
  - Maintenance: `ClearStaleReviewClaims`
- Each function returns a typed result struct (e.g., `*VerdictResult`, `*HandoffResult`, `*ModeChangeResult`)
- Zero `fmt.Print*` or `os.Stdin` calls — verified by grep
- Three consumers: `agent/` (orchestration), `commands/` (CLI presentation), `mcp/` (JSON-RPC adapter)
- Depends on: `db`, `models`, `git`, `log`, `paths`, `analysis` — same layer as `commands` minus presentation concerns
- ~~`claim_task.go` (299 LOC): `ClaimTask` is 265 LOC with nesting depth 6~~ *(pass 7, Complexity lens — resolved: `e86abd4`)*: Phase 2 worktree handling extracted into status-specific helper functions with a `phaseResult` struct. Dependency checking extracted to `unmetDependencies()` shared between TOCTOU phases (format-string wrapper further removed in `0158b64`). File is now 519 LOC total (helpers + main function), but `ClaimTask` itself is shorter with reduced nesting depth. Direct tests added for dependency resolution behavior.
- `wt_merge.go` (377 LOC): `MergeWorktree` — linear phased flow (validate → merge → integration tests → update state → cleanup) but the phase count and error handling paths contribute significant cognitive load *(pass 7, Complexity lens)*
- `helpers.go` provides `readTaskState()` for Read-path task lookup, but no equivalent exists for the Modify-callback path *(pass 5, Duplication lens)*
- **Structural repetition within ops** *(pass 5, Duplication lens)*: Most ops functions share an identical skeleton — input validation → `paths.New(projectRoot)` + `db.For(lp.StatePath())` → `bb.Modify(func(state) { FindTask + nil check + status check + mutate + history append })` → wrap error → return result. Quantified: `if taskID == ""` guard in 10/21 files, `FindTask + NotFoundError` inside Modify in 10 files, `task.History = append(...)` in 12 files. See Duplication smell below.
- **Inconsistent parameter conventions** *(pass 6, Coupling lens)*: Some ops functions take `projectRoot` and internally construct `paths.New()` + `db.For()` (ClaimTask, MergeWorktree, DeleteTask, SubmitReview, etc.), while others take `statePath`/`logPath` directly (AddTask). Callers must know which convention each function uses. See Coupling smell below.

#### commands (`internal/commands/`) — ~4,300 LOC *(pass 7: updated from ~2,800)*

**Purpose:** CLI presentation wrappers over `ops/` business logic, plus read-only query commands.

**Pattern:** Thin wrapper per command: call `ops.*`, format and print result. Read-only commands (inspect, status, validate) retain their own logic since they already return structured data.

**Observations:**
- 25+ command implementations — mutation commands are thin wrappers (~20-75 LOC each), read-only commands retain logic
- `watch.go` (516 LOC): 11 health checks with alert deduplication, comprehensive monitoring
- `validate.go` (448 LOC): 9 validators checking all state invariants, `validateTaskInvariants` at 142 LOC — sequential if-chain checking ~15 status-specific invariants with no early-exit grouping *(pass 2; pass 7: LOC updated, complexity note)*
- ~~`inspect_field.go` (327 LOC): Manual reflection system with 9 switch statements~~ *(pass 7, Complexity lens — resolved: `c4bd748`)*: Direct-field switch dispatch replaced with reflect-based YAML-tag walker for `getField`. File is now 275 LOC. Exhaustive tests enumerate all config/sprint tagged paths and assert `getField` can resolve every field — adding a model field with a YAML tag automatically makes it accessible via `liza inspect`. Computed-field behavior preserved. Historical note: `sprint.timeline` retains a hardcoded check for backward compatibility (`a35735e` documents rationale)
- ~~`wt_merge.go` (356 LOC)~~ now ~60 LOC wrapper over `ops.MergeWorktree()`
- `format.go` (164 LOC): centralized JSON/YAML/table formatting
- Templates in `commands/templates/`: status_dashboard, agent_value, metrics_value
- ~~**Monolithic command functions**~~ — *(pass 2, Complexity lens — resolved)* All monolithic commands extracted to `ops/`. `DeleteTaskCommand` (220→~75 LOC) was the last to be extracted, using `ops.CheckDeleteTask()` + `ops.DeleteTask()` with interactive confirmation remaining at CLI level.
- ~~**Presentation+logic coupling**~~ — *(pass 3, Boundaries lens — resolved for MCP-exposed commands)* All mutation commands now delegate to `ops/` for business logic. Remaining `fmt.Print*` calls are legitimate presentation in thin wrappers. `os.Stdin` reads remain only in CLI-only commands (`setup.go`, `init.go`, `delete_task.go`, `delete_agent.go`) — not MCP-exposed.
- **Self-constructing infrastructure** — each command function creates fresh `paths.New()`, `db.New()`, `git.New()` instances internally; no dependency injection *(pass 3, Boundaries lens)*
- **Watch thresholds hardcoded** — 10 constants (`DefaultCheckInterval`, `LeaseGracePeriod`, `StallThreshold`, etc.) with no path to `models.Config`. Operationally tunable parameters hardcoded in source *(pass 6, Coupling lens)*
- ~~**Divergent GracePeriod**~~ — *(pass 6, Coupling lens — resolved: `b9f20ff`)*: Unified into `models.LeaseExpiryGracePeriod` single `time.Duration` constant in `internal/models/lease.go`. Both `validate.go` and `watch.go` now use the shared constant. Tests verify warning behavior inside and outside the unified grace window

#### cmd (`cmd/`) — 1,344 LOC

**Purpose:** Binary entry points.

**Observations:**
- `cmd/liza/main.go` (1,275 LOC, 5 functions): 1,100+ lines of inline cobra command `var` blocks + 111-line `init()` for flag registration. Business logic correctly delegates to `commands` package — complexity is organizational, not behavioral. *(pass 2, Complexity lens)*
- `cmd/liza-mcp/main.go` (69 LOC): thin stdio transport launcher. Cross-assigns version info via mutable package globals: `mcp.Version = embedded.Version` *(pass 3, Boundaries lens)*

#### mcp (`internal/mcp/`) — 1,399 LOC

**Purpose:** MCP JSON-RPC server exposing tools and resources to AI agents.

**Observations:**
- `server.go` (704 LOC): tool/resource registration, request dispatch. `registerMutationTools()` is 242 LOC of declarative tool schema definitions — LOC is mostly boilerplate, not algorithmic complexity *(pass 2, Complexity lens)*
- `handlers.go` (~600 LOC, 29 functions): tool implementations delegating to `ops` package for mutations, `commands` package for read-only queries. 14% branch density — each handler is thin. *(pass 2; pass 5: updated LOC and ops import)*
- `protocol/` subpackage (232 LOC): clean DTO types, stdio transport, error codes
- 4 registration categories: read-only tools, read-only resources, mutation tools, complex operations
- Clean adapter boundary: mcp translates JSON-RPC into `ops` calls (mutations) and `commands` calls (queries), adds error classification, holds no business logic *(pass 3, Boundaries lens; pass 5: updated — handlers now import ops directly for all mutations)*
- **Server dispatch layer untested**: `server_test.go` has only 4 tests (initialization/registration). The entire request dispatch layer — `HandleRequest`, `Run`, `classifyError`, `handleToolCall`, `handleResourceRead`, `handleNotification` — is at 0% coverage. Handlers tested directly via `handlers_test.go` (1,298 LOC), but the routing/error-classification layer has no tests *(pass 4, Coverage lens)*
- ~~`protocol/` entirely untested~~ *(pass 4, Coverage lens — partially resolved: `c2fe02b`)*: stdio transport now has bounded request size enforcement (`MaxRequestSize` 10MB, `readLineBounded()`) with comprehensive tests (214 LOC in `stdio_test.go`). Error constructors remain untested. `RequestTooLarge` JSON-RPC error code added

#### git (`internal/git/`) — 351 LOC

**Purpose:** Git worktree and branch operations.

**Observations:**
- `CreateWorktree()`, `RemoveWorktree()`, `MergeBranch()` (ff then no-ff), `RebaseOnto()`
- Centralized `runGit()` / `runGitCombined()` helpers
- `CalculateDrift()` for worktree-to-main divergence measurement
- ~~**Hardcoded `"task/"` branch prefix**~~ *(pass 6, Coupling lens — resolved: `59a8e3e`)*: `paths.TaskBranchPrefix` constant added; all 7 production files (`git/worktree.go` ×2, `ops/claim_task.go` ×5, `ops/wt_merge.go`, `ops/delete_task.go`, `ops/submit_review.go`, `ops/wt_delete.go`) now use the constant

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
- ~~Reads `os.Stdin` directly for merge confirmation prompts~~ *(pass 3, Boundaries lens — resolved: `7a5e79c`)*: Both `WriteClaudeSettings()` and `WriteMCPSettings()` now accept `io.Reader` parameter, defaulting to `os.Stdin` when nil. Tests use `strings.NewReader` — no more monkey-patching
- `WriteMCPSettings()`, `mergeMCPSettings()`, `PlanGlobalFiles()` — previously at 0% coverage due to stdin coupling *(pass 4, Coverage lens)*; stdin coupling now resolved via `io.Reader` injection

#### paths (`internal/paths/`) — 257 LOC

**Purpose:** Path resolution with worktree awareness.

**Observations:**
- `GetProjectRoot()` via `git rev-parse --show-toplevel`
- `ValidateTaskID()` with path traversal protection
- `TaskBranchPrefix = "task/"` constant — single source of truth for branch naming *(added: `59a8e3e`)*
- All standard `.liza/` paths centralized

#### Other leaf packages

- `log/` (221 LOC): YAML append log with flock (via shared `filelock` package). Now uses append-only writes (no O(n) rewrite) and bounded tail-window `GetLastTimestamp()` for sub-linear reads *(perf: `fe8de6b`)*
- `filelock/` (new): Shared file-locking with flock, PID-based stale detection, error classification, metrics
- `analysis/` (260 LOC): Circuit breaker pattern detection (6 patterns)
- `identity/` (140 LOC): Agent ID resolution and validation
- `errors/` (55 LOC): Exit codes and `NotFoundError` type (with `Entity`, `ID`, `Field` fields)
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
   ├── db/                             ├── commands/
   ├── ops/                            └── ops/
   ├── agent/
   ├── commands/
   └── mcp/

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
- ~~Default lease duration (1800 seconds) — exists as magic number, not named constant~~ *(resolved: `models.DefaultLeaseDurationSeconds` — but supervisor.go still uses raw `1800` in 2 call sites, pass 6)*
- ~~The relationship between `GracePeriod` (60s in validate.go) and `LeaseGracePeriod` (120s in watch.go)~~ *(pass 6, Coupling lens — resolved: `b9f20ff`; unified as `models.LeaseExpiryGracePeriod`)*
- ~~The mapping between agent config roles (`"code-reviewer"`, hyphen) and task workflow role constants (`"code_reviewer"`, underscore)~~ *(pass 6, Coupling lens — resolved: `a60c72e`; `internal/roles` package provides explicit `ToWorkflow()`/`ToRuntime()` mapping)*
- ~~Missing `State.FindTask(taskID)` domain method~~ *(pass 2, Complexity lens — resolved)* — `FindTask` and `FindTaskIndex` added, all inline lookups migrated
- The contract between `commands` and its consumers — commands assume terminal I/O but serve three different transports *(pass 3, Boundaries lens)*

**What's missing from the walkthrough?**
- `db/metrics.go` (113 LOC): lock timing metrics — read and noted
- `commands/status.go` (469 LOC): status dashboard rendering — read via templates

**What requires cross-file comparison?**
- ~~Flock locking pattern in db/ vs log/ (duplicated — confirmed by cross-file analysis)~~ *(resolved: extracted to `internal/filelock`)*
- ~~`leaseDuration = 1800` fallback in supervisor.go (x2) and claim_task.go (x1) (duplicated)~~ *(resolved: `models.DefaultLeaseDurationSeconds` constant)*
- ~~`NotFoundError` structured type vs ad-hoc `fmt.Errorf("task not found: %s")` — **25+ instances** of the ad-hoc form in non-test code~~ *(resolved: all sites migrated to `NotFoundError` with `ID` field)*
- `derefString()` in prompts/builder.go duplicates `deref` template function
- ~~Inline task-lookup loop duplicated 55+ times across commands, agent, db packages~~ *(pass 2, Complexity lens — resolved: `State.FindTask()`)*
- Template execution pattern in `commands/templates.go` vs `prompts/templates.go` — nearly identical: embed.FS + funcMap with `deref` + template.Must + executeTemplate that panics *(pass 3, Boundaries lens)*
- ~~`os.Stdin` reads across `embedded` (2), `commands/setup` (2), `commands/init` (1), `commands/delete_task` (2), `commands/delete_agent` (1) — all tested by monkey-patching `os.Stdin`~~ *(pass 3, Boundaries lens — resolved: `7a5e79c`; all 8 locations now accept `io.Reader` parameter; tests use `strings.NewReader`; `os.Stdin` monkey-patching eliminated)*
- **Ops Modify-callback boilerplate** *(pass 5, Duplication lens)*: `FindTask(taskID) + nil→NotFoundError` inside `bb.Modify` callbacks appears in 10 production files. The existing `readTaskState()` helper only works outside callbacks. The guard is identical in every file — see Smell below.
- **Ops input validation** *(pass 5, Duplication lens)*: `if taskID == "" { return nil, fmt.Errorf("task ID is required") }` in 10/19 ops files; `if agentID == ""` in 7/19. Each function validates its own required parameters independently.
- **Command test harness** *(pass 5, Duplication lens)*: 82 table-driven test loops across 34 command test files, 23 `initialState := &models.State{...}` constructions. The loop body (~15 lines: tmpDir → SetupLizaDir → create state → setup → WriteInitialState → call command → check error → validate state) is structurally identical.
- **Test setup sequence** *(pass 5, Duplication lens)*: 625 occurrences of `testhelpers.{SetupLizaDir|CreateValidState|WriteInitialState}` across 55 test files. The 3-4 line setup is per-test-function, not per-file.
- **Ops parameter convention split** *(pass 6, Coupling lens)*: `ClaimTask`, `MergeWorktree`, `DeleteTask`, `SubmitReview`, `Start`, `Stop`, `Pause`, `Resume`, `CreateWorktree`, `DeleteWorktree` take `projectRoot`; `AddTask` takes `statePath`/`logPath` directly — callers must track which convention each function uses
- ~~**`"task/"` branch prefix scattered**~~ *(pass 6, Coupling lens — resolved: `59a8e3e`)*: `paths.TaskBranchPrefix` constant added; all 7 production files now use it
- **Identity validation duplicated** *(pass 6, Coupling lens)*: `agent/registration.go:validateIdentity()` reimplements `identity.ValidateFormat()` + `identity.ValidateRole()` without importing the package
- ~~**Role naming divergence**~~ *(pass 6, Coupling lens — resolved: `a60c72e`, extended: `e173f71`)*: `internal/roles` package provides unified constants (`RuntimeCoder`, `RuntimeCodeReviewer`, `RuntimePlanner`, `WorkflowCoder`, `WorkflowCodeReviewer`, `WorkflowPlanner`) with explicit bidirectional `ToWorkflow()`/`ToRuntime()` mapping. Planner now fully integrated: `WorkflowPlanner` added, `AllWorkflow()` includes planner, `IsValidRuntime()` no longer special-cases planner. `models.RolePlanner` alias added. All agent/, cmd/, and ops/ files migrated to role constants. 1 residual: `claiming.go:115` uses `Role: "coder"` literal

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

**Complexity lens metrics** *(pass 2; pass 7: updated with ops files, nesting depth, function LOC)*:

| File | LOC | Longest Function (LOC) | Max Nesting Depth | Notes |
|------|-----|----------------------|-------------------|-------|
| main.go | 1,275 | init (111) | 2 | Organizational only — flat command registry |
| server.go | 715 | registerMutationTools (242) | 2 | Declarative schema definitions |
| state.go | 631 | — | 2 | 20+ cohesive structs |
| handlers.go | 603 | — | 3 | Thin handlers, 14% branch density |
| watch.go | 516 | — | 3 | 11 health checks, well-decomposed |
| validate.go | 448 | validateTaskInvariants (142) | 3 | Sequential if-chain, no early-exit grouping |
| inspect_field.go | 327 | — | 3 | **9 switch statements** — manual reflection *(pass 7)* |
| **ops/claim_task.go** | **299** | **ClaimTask (265)** | **6** | **Highest complexity — 3-phase TOCTOU with duplicated dep check** *(pass 7)* |
| **ops/wt_merge.go** | **285** | **MergeWorktree (189)** | **4** | **Linear but many error-handling paths** *(pass 7)* |
| supervisor.go | 302 | RunSupervisor (186) | 5 | Main event loop *(pass 7: depth noted)* |
| claiming.go | 318 | resumeHandoffTask (63) | 5/6 | handleApprovedMerges: 55 LOC but depth 6 *(pass 7)* |
| ~~supervisor.go (old)~~ | ~~1,428~~ | ~~RunSupervisor (186)~~ | — | *(resolved: split into 6 files)* |
| ~~commands/claim_task.go~~ | ~~328~~ | ~~ClaimTaskCommand (310)~~ | — | *(resolved: extracted to ops)* |
| ~~commands/wt_merge.go~~ | ~~356~~ | ~~WtMergeCommand (319)~~ | — | *(resolved: extracted to ops)* |

**Boundaries lens import analysis** *(pass 3)*:

| Package | Internal Imports | External Imports | Consumers |
|---------|-----------------|------------------|-----------|
| `models` | 0 | 0 | 6 packages |
| `paths` | 0 | 0 | 6 packages |
| `errors` | 0 | 0 | 7 packages (ops, db, agent, commands, mcp, testhelpers, prompts) |
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

2:1 test-to-code ratio with consistent patterns: table-driven tests, filesystem isolation via `t.TempDir()`, real git repos for integration, lightweight hand-written mocks. The `testhelpers` package (733 LOC) provides reusable primitives (`SetupLizaDir`, `CreateValidState`, `WriteInitialState`, `BuildTaskByStatus`, assertion helpers) used across 55 test files (625 call sites). Integration tests in `internal/integration/` verify complete workflows. All `internal/` packages have tests. *(pass 5: quantified testhelpers usage)*

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

#### ~~Smell: Interactive stdin in library packages~~ *(pass 3, Boundaries lens — resolved: `7a5e79c`)*

**Signal:** Direct `os.Stdin` reads via `bufio.NewReader(os.Stdin)` or `bufio.NewScanner(os.Stdin)` in 8 locations across 5 files (`embedded/embedded.go`, `commands/setup.go`, `commands/init.go`, `commands/delete_task.go`, `commands/delete_agent.go`).

**Fix:** All 8 locations now accept an `io.Reader` parameter, defaulting to `os.Stdin` when nil (CLI behavior unchanged). `cmd/liza/main.go` passes `os.Stdin` at call sites. Tests use `strings.NewReader` for mock input — the `os.Stdin` monkey-patching pattern (`os.Stdin = r` / `defer`) is fully eliminated. `withMockStdin` helper removed.

#### ~~Smell: Duplicated file locking mechanism~~ *(resolved)*

**Signal:** `db/blackboard.go` and `log/logger.go` independently defined identical `DefaultLockTimeout` (10s) and `LockCheckInterval` (100ms) constants, and implemented structurally identical flock polling loops.

**Fix:** Extracted to `internal/filelock` package with the db package's enriched version (stale lock recovery, PID tracking, error classification, metrics) as the basis. Both `db.Blackboard` and `log.Logger` now delegate to `filelock.FileLock`. The log package gained stale lock recovery and error classification it previously lacked. No external consumers of the old `db.LockError` types existed, so no aliases were needed.

#### ~~Smell: Hardcoded configuration — magic number 1800~~ *(mostly resolved — 2 residual sites, pass 6)*

**Signal:** `leaseDuration = 1800` appeared as a fallback default in 3 locations, plus 6 more magic numbers in `getRoleWaitConfig`.

**Fix:** Defined `DefaultLeaseDurationSeconds` and `Default{Coder,Planner,Reviewer}{PollInterval,MaxWait}` constants in `internal/models/state.go` alongside `Config`. ~~All 9 fallback sites reference named constants.~~ `heartbeat.DefaultLeaseDuration` derives from `models.DefaultLeaseDurationSeconds`.

**Residual** *(pass 6, Coupling lens)*: `supervisor.go:127` (`registerAgent(..., 1800)`) and `supervisor.go:221` (`claimReviewerTask(..., 1800, ...)`) still use raw `1800` instead of `models.DefaultLeaseDurationSeconds`. These were missed during the original extraction.

#### ~~Smell: MCP handlers bypass Blackboard locking~~ *(resolved)*

**Signal:** `mcp/handlers.go` read `state.yaml` directly via `os.ReadFile()` (for the `liza://state` resource) instead of going through `db.Blackboard.Read()`.

**Fix:** Added `Blackboard.ReadRaw()` method that reads raw bytes under flock. `Server` struct now holds a `*db.Blackboard` instance. `readStateResource()` uses `s.bb.ReadRaw()` instead of `os.ReadFile()`. `ReadRaw` (rather than `Read` + re-marshal) avoids the YAML round-trip data loss issue. *(Note: the underlying round-trip data loss issue itself is now also resolved — see below.)*

#### ~~Smell: Inconsistent "not found" error types~~ *(resolved)*

**Signal:** `internal/errors` defines `NotFoundError` struct, but **25+ call sites** used `fmt.Errorf("task not found: %s", id)`. The structured type was only used by inspect commands. *(pass 2: quantified — 25+ instances vs "most call sites")*

**Fix:** Added `ID` field to `NotFoundError` (error message format `"task not found: task-42"` matches old ad-hoc format). Migrated all 25+ sites across `ops/` (12 files), `db/blackboard.go`, `agent/` (4 files), and `commands/inspect_*.go` (3 files). `IsNotFound()` updated to use `errors.As` (supports wrapped errors from `bb.Modify`). MCP `classifyError()` now checks `errors.As(&NotFoundError{})` first, with string fallback retained for external errors (git, etc.).

#### ~~Smell: `executeTemplate` panics on error~~ *(resolved)*

**Signal:** `prompts/templates.go:31` and `commands/templates.go:28` both called `panic("template: " + err.Error())` on template execution failure.

**Fix:** Both `executeTemplate` (prompts) and `executeCommandTemplate` (commands) now return `(string, error)`. Error propagated through all callers: `Build{BasePrompt,PlannerContext,CoderContext,ReviewerContext}`, `buildInstructionsForWakeTrigger`, `format{AgentValue,MetricsValue}`, and `agent/prompt.go:buildPrompt`. All callers already returned `(string, error)` — propagation required no architectural changes.

#### Smell: Non-injectable stdio in MCP transport *(partially resolved: `c2fe02b`)*

**Signal:** `NewStdioTransport()` hardwires `os.Stdin`/`os.Stdout`. Cannot inject readers/writers for testing.

**Partial fix:** Bounded request size enforcement (`MaxRequestSize` 10MB) added with `readLineBounded()` using `bufio.Reader.Peek`. Comprehensive tests (214 LOC in `stdio_test.go`) cover size limits, error responses, and normal operation — achieved without `io.Reader`/`io.Writer` injection by testing the bounded read logic directly.

**Remaining impact:** The transport constructor still hardwires `os.Stdin`/`os.Stdout`. Full I/O injection would enable testing `Run()` and the complete server loop.

**Direction:** Accept `io.Reader`/`io.Writer` parameters for full testability.

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

#### Smell: Ops callback boilerplate — FindTask + guard + history *(pass 5, Duplication lens)*

**Signal:** Inside `bb.Modify` callbacks, 10 of 19 ops files repeat identical 3-line task lookup:
```go
task := state.FindTask(taskID)
if task == nil {
    return &errors.NotFoundError{Entity: "task", ID: taskID}
}
```
The `readTaskState()` helper (helpers.go) only works outside callbacks (it calls `bb.Read()`). Inside callbacks, no helper exists.

Additionally, `task.History = append(task.History, models.TaskHistoryEntry{Time: now, Event: "...", Agent: &agentID})` appears in 12 locations with minor variations (some add `Reason`, `Note`, `PreviousAssignee`).

**Impact:** Low-medium. Each occurrence is small (3-5 lines), and the variations in history entries make full extraction non-trivial. The repetition is coincidental similarity rather than copy-paste — each file was independently authored with the same pattern. Risk: if the guard pattern changes (e.g., adding logging on not-found), 10 files need updating.

**Direction:** A `modifyTask(bb, taskID, func(state, task) error) error` helper could encapsulate the Modify+FindTask+guard pattern. History append could get a `task.RecordEvent(time, event, agentID, opts...)` method. However, the current repetition is idiomatic Go — evaluate whether the abstraction adds clarity or obscures intent.

#### ~~Smell: Manual reflection dispatch in `inspect_field.go`~~ *(pass 7, Complexity lens — resolved: `c4bd748`)*

**Signal:** 327 LOC with 9 switch statements forming a hand-written field accessor system.

**Fix:** Replaced direct-field switch dispatch with a reflect-based YAML-tag walker for `getField`. File reduced from 327 to 275 LOC. Exhaustive tests enumerate all config/sprint tagged paths and assert `getField` can resolve every field — adding a model field with a YAML tag automatically makes it discoverable. Computed-field behavior preserved. `sprint.timeline` retains a hardcoded check for backward compatibility (documented in `a35735e`).

#### ~~Smell: `ClaimTask` function complexity~~ *(pass 7, Complexity lens — resolved: `e86abd4`)*

**Signal:** `ops/claim_task.go:ClaimTask` was 265 LOC — the longest function in the codebase. Nesting depth 6, dependency checking duplicated across TOCTOU phases.

**Fix:** Phase 2 worktree handling extracted into status-specific helper functions (`handleReadyClaim`, `handleRejectedSameCoderClaim`, `handleRejectedDifferentCoderClaim`, `handleIntegrationFailedClaim`) with a `phaseResult` struct. Dependency checking extracted to shared `unmetDependencies()` function used in both TOCTOU phases. Format-string wrapper further removed in `0158b64`. File is now 519 LOC total (helpers + main function + tests), with `ClaimTask` itself shorter and lower nesting depth. Direct tests added for dependency resolution behavior. Cleanup errors now surfaced as warnings (`729da05`).

#### Smell: `validateTaskInvariants` monolithic if-chain *(pass 7, Complexity lens)*

**Signal:** 142 LOC, ~15 sequential `if task.Status == X && field == nil` checks with no grouping. Each status's invariants are scattered across the function rather than grouped by status.

**Impact:** Low. The function is simple despite its length — each check is independent and self-documenting. However, the lack of grouping means a developer adding a new status must scan the entire function to ensure all required invariants are covered.

**Direction:** Group checks by status (all IMPLEMENTING invariants together, all REVIEWING together, etc.) — or use a `switch task.Status` with per-status validation functions. Not urgent; the function is correct and readable despite its length.

#### Smell: High nesting depth in `claiming.go` *(pass 7, Complexity lens)*

**Signal:** `handleApprovedMerges` (55 LOC) reaches nesting depth 6: `for range → if status → if approved → if merge_commit → if err → errors.As`. `resumeHandoffTask` (63 LOC) reaches depth 5 inside its `bb.Modify` closure. Both functions are short enough that the nesting doesn't create horizontal scroll, but the depth-to-LOC ratio signals tightly packed control flow.

**Impact:** Low. Both functions are well-commented and the deep nesting follows natural error-handling patterns (check condition → attempt operation → classify error). The `handleApprovedMerges` pattern is particularly common in Go error handling.

**Direction:** `handleApprovedMerges` could extract the inner merge-attempt body into a `tryMergeTask(projectRoot, task, agentID) error` helper, reducing the for-loop body to: filter + call + log. Low priority.

#### Smell: Ops input validation boilerplate *(pass 5, Duplication lens)*

**Signal:** `if taskID == "" { return nil, fmt.Errorf("task ID is required") }` appears in 10 ops files. `if agentID == ""` appears in 7 files. Each function independently validates required string parameters with identical code.

**Impact:** Low. The validation is trivial (1-3 lines per parameter) and self-documenting. The "cost" is mostly visual noise rather than maintenance risk.

**Direction:** A validation helper (`requireNonEmpty("task ID", taskID)`) or struct-based input with a `Validate()` method could reduce noise, but this is borderline — idiomatic Go favors explicit validation at function entry. Not worth abstracting unless the pattern grows beyond simple emptiness checks.

#### Smell: Command test harness repetition *(pass 5, Duplication lens)*

**Signal:** 34 command test files share a structurally identical test loop body (~15 lines): create tmpDir → `SetupLizaDir` → construct `initialState` with common config → apply `setupState` → `WriteInitialState` → call command → check error → validate state. The `initialState` construction (Config fields: IntegrationBranch, LeaseDuration, etc.) is repeated 23 times with near-identical values.

**Impact:** Low. This is standard Go table-driven test boilerplate. The `testhelpers` package already extracts the reusable primitives. Further abstraction would need to handle the variety in command signatures (different parameter sets per command) — a generic harness would trade boilerplate for indirection.

**Direction:** A `testhelpers.RunCommandTest(t, CommandTestCase{...})` helper could encapsulate the common loop body, accepting the command-under-test as a function parameter. Alternatively, a `testhelpers.DefaultState()` function returning a pre-configured `*models.State` (with standard Config values) would eliminate the 23 repeated `initialState` constructions. The latter is lower-risk and higher-value.

#### ~~Smell: Hardcoded `"task/"` branch prefix~~ *(pass 6, Coupling lens — resolved: `59a8e3e`)*

**Signal:** The string `"task/" + taskID` was constructed inline in 7 production files with no constant.

**Fix:** Added `TaskBranchPrefix = "task/"` constant to `internal/paths/paths.go`. Replaced all inline constructions across 8 files: `git/worktree.go` (×2), `ops/claim_task.go` (×5), `ops/wt_merge.go`, `ops/delete_task.go`, `ops/submit_review.go`, `ops/wt_delete.go`. Test added for constant value.

#### Smell: Duplicated identity validation *(pass 6, Coupling lens)*

**Signal:** `agent/registration.go:validateIdentity()` (30 LOC) reimplements the same algorithm as `identity.ValidateFormat()` + `identity.ValidateRole()` without importing the `identity` package. Both: split on last hyphen, validate numeric suffix with `strconv.Atoi`, check role prefix match.

**Impact:** Low-medium. If identity format rules change (e.g., allowing non-numeric suffixes), two implementations need updating independently. The `identity` package is the canonical source but the `agent` package doesn't know it exists.

**Direction:** Replace `validateIdentity()` call with `identity.ValidateFormat()` + `identity.ValidateRole()`. The `identity` package already returns structured errors.

#### ~~Smell: Role naming divergence~~ *(pass 6, Coupling lens — resolved: `a60c72e`)*

**Signal:** Two role naming conventions coexisted: agent config roles (`"code-reviewer"`, hyphen) and task workflow roles (`"code_reviewer"`, underscore) with implicit mapping in ~15 switch/if-else cases.

**Fix:** Created `internal/roles` package with:
- Runtime constants: `RuntimeCoder`, `RuntimeCodeReviewer`, `RuntimePlanner`
- Workflow constants: `WorkflowCoder`, `WorkflowCodeReviewer`
- Explicit mapping functions: `ToWorkflow()`, `ToRuntime()`
- Validation: `IsValidRuntime()`, `IsValidWorkflow()`
- List functions: `AllRuntime()`, `AllWorkflow()`

All agent/, cmd/, and ops/ production code migrated to use role constants. Redundant local constants removed from `ops/recover_agent.go`. Comprehensive tests (253 LOC) cover all mappings and validation. 1 residual: `claiming.go:115` still uses `Role: "coder"` literal.

#### Smell: Inconsistent ops parameter conventions *(pass 6, Coupling lens)*

**Signal:** Most ops functions take `projectRoot string` and internally construct `paths.New(projectRoot)` + `db.For(lp.StatePath())` (e.g., `ClaimTask`, `MergeWorktree`, `DeleteTask`, `Start`, `Stop`). But `AddTask` takes `statePath, logPath string` directly — the caller must construct these paths and pass them in.

**Impact:** Low-medium. Callers must know which convention each function uses. The inconsistency creates a maintenance tax: if path derivation logic changes, functions using `projectRoot` auto-adapt (via `paths.New()`), but `AddTask` callers must update manually. New ops functions must decide which convention to follow with no documented guidance.

**Direction:** Standardize on `projectRoot` parameter convention (majority pattern). Migrate `AddTask` to take `projectRoot` and derive paths internally, consistent with all other ops functions.

#### Smell: Watch thresholds not configurable *(pass 6, Coupling lens)*

**Signal:** `commands/watch.go` defines 10 hardcoded constants:
- `DefaultCheckInterval = 10s`, `LeaseGracePeriod = 120s`, `StallThreshold = 30m`
- `StaleDraftThreshold = 30m`, `CheckpointStaleThreshold = 30m`, `CheckpointStuckThreshold = 2h`
- `CheckpointAbandonedThreshold = 8h`, `PauseStaleThreshold = 30m`, `PauseForgottenThreshold = 2h`
- `OrphanedGracePeriod = 30s`

These are operational tuning parameters with no path to `models.Config`.

**Impact:** Low. The values are reasonable defaults and rarely need changing. However, the `models.Config` struct already exists as the configuration mechanism for other runtime parameters (`LeaseDuration`, `IntegrationBranch`, etc.). Watch thresholds are the only operational parameters that bypass this pattern entirely.

**Direction:** Add watch-related fields to `models.Config` (or a nested `WatchConfig` struct) with the current values as defaults. Not urgent — these are stable values.

#### Smell: Hardcoded `"terminal-1"` in supervisor *(pass 6, Coupling lens)*

**Signal:** `supervisor.go:127` passes `"terminal-1"` as a literal string to `registerAgent()`. The terminal field is recorded in agent state but always set to this fixed value regardless of the agent's actual terminal.

**Impact:** Low. The terminal field is informational (used in status display). However, if multiple agents run in different terminals, they all report the same terminal ID, reducing operational visibility.

**Direction:** Derive from `config.Terminal` or the agent's TTY, or remove the field if it's not providing value.

#### ~~Smell: Divergent GracePeriod values~~ *(pass 6, Coupling lens — resolved: `b9f20ff`)*

**Signal:** `validate.go` defined `GracePeriod = 60` (int) and `watch.go` defined `LeaseGracePeriod = 120 * time.Second` (`time.Duration`) — different values, types, and names for semantically identical concept.

**Fix:** Introduced `models.LeaseExpiryGracePeriod` as a single canonical `time.Duration` constant in `internal/models/lease.go`. Both `validate.go` and `watch.go` now use the shared constant. Tests in `validate_test.go` assert warning behavior at the grace period boundary (verifying `Before()` strict-less-than semantics).

#### Smell: No interface-based seams beyond CLIExecutor *(pass 3, Boundaries lens)*

**Signal:** The entire production codebase has exactly **one interface**: `CLIExecutor` in `agent/supervisor.go`. All other cross-package dependencies use concrete types: `*db.Blackboard`, `*git.Git`, `*log.Logger`, `paths.LizaPaths`. There is one test-only interface (`testingT` in `testhelpers/assertions.go`).

**Impact:** This is a deliberate simplicity choice appropriate for v1 scope. However, it means testing any package that uses `db.Blackboard` requires real file I/O (creating temp directories, writing YAML files). The `testhelpers` package exists specifically to manage this overhead. If the system grows, introducing seams at the `db` and `git` boundaries would enable faster, more isolated tests.

**Direction:** No action for v1 — the current approach works. If test suite time becomes a concern, introduce interfaces at package boundaries (particularly `db` and `git`) to enable in-memory test doubles.

#### ~~Smell: Task-state-machine spec drift (`BLOCKED -> READY`)~~ *(Adversarial pass — resolved in `0f6fe19`)*

**Signal (historical):** `specs/architecture/state-machines.md` documented `BLOCKED -> READY` as valid, but the executable transition graph in `internal/models/state.go` only allows `BLOCKED -> SUPERSEDED|ABANDONED`. `TaskStatus.CanTransition()` tests align with code, not the doc.

**Status:** Resolved in `0f6fe19`. `specs/architecture/state-machines.md` now forbids `BLOCKED -> READY` and aligns with runtime (`BLOCKED -> SUPERSEDED|ABANDONED`).

#### ~~Smell: Worktree recovery doc uses non-canonical branch name~~ *(Adversarial pass — resolved: `ee8a55d`)*

**Signal:** `docs/TROUBLESHOOTING.md` used `git worktree add ... -b task-1` while runtime checks and worktree creation consistently use `task/<id>` (e.g., `"task/" + taskID` in `internal/git/worktree.go` and `internal/ops/submit_review.go`).

**Impact:** Medium. Following documented recovery can produce a local branch layout that later fails submit/merge validation (`expected: task/<id>`), turning recovery into a second failure.

**Fix:** Canonicalized all worktree recovery examples in `docs/TROUBLESHOOTING.md` to use `task/<task-id>` placeholders, matching the runtime `paths.TaskBranchPrefix` convention.

#### ~~Smell: Test guidance drift for short mode~~ *(Adversarial pass — resolved: `84b5a64`)*

**Signal:** `docs/TESTING.md` claimed integration tests are skipped in `go test -short` via `testing.Short()`, but `internal/integration/*.go` currently has no `testing.Short()` guards and tests execute under `-short`.

**Fix:** Added `testing.Short()` guards to all integration test files in `internal/integration/`. Documentation in `docs/TESTING.md` updated to accurately reflect that integration tests now skip when `go test -short` is used.

#### Smell: Pairing initialization doc pointer drift (`docs/USAGE.md`) *(Adversarial pass, entry: specs/)*

**Signal:** `contracts/PAIRING_MODE.md` Session Initialization still requires reading `docs/USAGE.md`, but that file does not exist. Canonical docs are now split (`docs/USAGE_PAIRING.md`, `docs/USAGE_MULTI_AGENTS.md`, indexed in `docs/README.md`).

**Impact:** High. The contract's startup procedure can fail on a missing file before work begins, producing avoidable initialization breaks and inconsistent behavior across agents.

**Direction:** Update Session Initialization to reference canonical docs (`docs/USAGE_PAIRING.md` minimum, or `docs/README.md` as resolver + mode-specific usage doc).

#### ~~Smell: Watch stall detection parses YAML text directly~~ *(Adversarial pass, entry: specs/ — resolved: `61b16d5`)*

**Signal:** `internal/commands/watch.go` `checkStalled()` read `log.yaml` with `os.ReadFile`, then scanned lines for raw `timestamp:` text.

**Fix:** Replaced raw string scanning with typed log parsing. `checkStalled` now uses `log.New(logPath).GetLastTimestamp()` which returns structured `time.Time` from parsed `log.Entry` types. `strings` package import removed from `watch.go`. Tests added for `GetLastTimestamp`. Subsequent `fe8de6b` further improved `GetLastTimestamp` to use bounded tail-window parsing (sub-linear reads as log files grow).

#### Smell: Sprint governance Vision link is broken *(Adversarial pass, entry: specs/)*

**Signal:** `specs/protocols/sprint-governance.md` links to `../vision.md`, while canonical Vision in this repo is `specs/build/0 - Vision.md` (as indexed in `specs/README.md`).

**Impact:** Low. Navigation drift weakens spec coherence and slows onboarding/review.

**Direction:** Update the related-doc link to the canonical Vision path.

#### ~~Smell: CLI contract coverage gap in `cmd/liza/main.go`~~ *(Adversarial pass, entry: tests/ — resolved: `9d95c1c`)*

**Signal:** The command tree defines 31 cobra commands in `cmd/liza/main.go`, but CLI parser/wiring execution is covered only by `cmd/liza/get_test.go` through `rootCmd.SetArgs(...); rootCmd.Execute()`. Most command behavior is validated at `internal/commands/*_test.go` level (direct function calls), which bypasses cobra parsing, flag wiring, and env/flag precedence paths. Helper paths like `requireAgentID()` and `resolveChangedBy()` are currently uncovered in statement-level coverage.

**Fix:** Added `cmd/liza/mutation_wiring_test.go` (215 LOC) with end-to-end cobra command tests for 4 representative mutation commands (`claim-task`, `submit-verdict`, `wt-merge`, `release-claim`) exercising positional args, `--agent-id` flag, `LIZA_AGENT_ID` env fallback, `--changed-by` flag precedence over env, and rejection reason forwarding through the full CLI→ops path. Added `rootcmd_test_helpers_test.go` (122 LOC) with `resetRootCmdForTest(t)` helper that resets identity flags (`--agent-id`, `--changed-by`) and clears `db.For()` singletons between test runs, enabling reliable repeated execution (`-count=2`). `cmd/liza` test LOC grew from 376 to 717 (0.6:1 ratio, up from 0.3:1).

#### Smell: Temporal test coupling and non-parallelizable suite *(Adversarial pass, entry: tests/ — partially resolved: `1914732`, `1ff88d2`)*

**Signal:** ~~Test suite currently has 93 `_test.go` files with 0 uses of `t.Parallel()` and 21 explicit `time.Sleep()` calls (notably in watcher/heartbeat paths).~~ Shared process-level globals exist (`db.instances` singleton map and package-level `rootCmd`), which encourages serial execution in packages that use them.

**Partial fix:** `resetRootCmdForTest(t)` helper isolates `rootCmd` flag state and `db.For()` singletons between tests. `t.Parallel()` introduced in 15 call sites across 4 test files (stateless tests in `roles`, `errors`, `filelock/metrics`, `agent/prompt`). `time.Sleep()` calls reduced from 21 to 5 by replacing brittle sleep-based waits with event-driven synchronization (polling with condition checks) in watcher, heartbeat, and supervisor tests. `internal/testguard/` package (116 LOC) added with ratchet tests enforcing `t.Parallel()` floor (≥10 calls) and `time.Sleep()` ceiling (≤11 calls), preventing regression.

**Remaining:** Tests sharing process-global `rootCmd` (all `cmd/liza` tests) cannot use `t.Parallel()` due to `os.Chdir` and cobra flag state. Further parallelization requires either a `--project-root` flag on rootCmd or test-level process isolation.

**Impact:** Low (downgraded from Medium). The ratchet tests and global-reset helpers address the structural concern; remaining serial tests are constrained by process-global state, not by missing infrastructure.

#### Smell: Configured iteration limits are declarative but unenforced *(Adversarial pass, entry: config/; resolved)*

**Signal:** ~~`config.max_coder_iterations` and `config.max_review_cycles` are defined in `models.Config` and documented as control limits, but runtime task flow does not enforce either limit before continuing loops. `SubmitVerdict` increments review counters without threshold handling, and no non-test production path reads `MaxCoderIterations` or `MaxReviewCycles`. `task.max_iterations` exists in the `Task` model and specs but has no runtime consumer.~~
Resolved: `ops.ClaimTask` now enforces coder iteration ceilings (effective policy: `task.max_iterations` override, else `config.max_coder_iterations`) and transitions exhausted REJECTED tasks to BLOCKED. `ops.SubmitVerdict` now enforces review-cycle ceilings (`config.max_review_cycles`) and iteration ceilings in rejection flow, transitioning exhausted tasks to BLOCKED with explicit escalation metadata.

**Impact:** ~~High. Operators can tune iteration limits in `state.yaml` and receive no behavioral change. This creates false safety assumptions around runaway loops and escalation behavior.~~ Closed.

**Direction:** Closed — enforcement now exists in claim/reclaim and rejection paths, with explicit BLOCKED escalation transitions and test coverage in `internal/ops/claim_task_test.go` and `internal/ops/submit_verdict_test.go`.

#### ~~Smell: `heartbeat_interval` config is modeled/documented but ignored at runtime~~ *(Adversarial pass, entry: config/ — resolved: `9e59acf`, `a16c023`)*

**Signal:** `Config.HeartbeatInterval` was exposed in schema/docs and inspect output, but supervisor heartbeat startup passed a hardcoded `60s` interval.

**Fix:** Heartbeat interval now sourced from `state.Config.HeartbeatInterval` via `NormalizeHeartbeatInterval()` helper in models with bounds validation (rejects ≤0 or >300s). `NewHeartbeat()` reads interval from state when provided. `DefaultHeartbeatInterval` derived from `models.DefaultHeartbeatIntervalSec` (single source of truth, `a16c023`). Tests cover config-sourced intervals and bounds validation.

#### Smell: `LIZA_LOG_LEVEL` is documented but not implemented *(Adversarial pass, entry: config/)*

**Signal:** Docs define `LIZA_LOG_LEVEL`, but no runtime code path reads it; agent logger is initialized at fixed `slog.LevelInfo`.

**Impact:** Low-medium. Documented observability control is a no-op, causing confusion during debugging/operations.

**Direction:** Either implement environment-driven level selection (with validation and defaults) or remove this variable from docs to keep contract-to-runtime alignment.

#### Smell: MCP parse-error response write failure is ignored *(Adversarial pass, entry: error handling)*

**Status: Fixed** (`80297b9`)

**Signal:** `internal/mcp/server.go:252` calls `transport.WriteError(...)` and discards its return value in the parse-error path.

**Impact:** High. If stdout/transport write is broken, the server silently continues after failing to send protocol errors, leaving the MCP session in an unobservable degraded state.

**Direction:** Make this path terminal: if `WriteError` fails, return an error and stop the server loop.

#### Smell: Cleanup failures are suppressed in rebase/worktree recovery paths *(Adversarial pass, entry: error handling — partially resolved: `729da05`)*

**Signal:** Best-effort cleanup drops failures in multiple mutation flows.

**Partial fix:** `729da05` surfaced cleanup errors in `claim_task.go` — worktree/branch cleanup failures during claim failure recovery and worktree recreation recovery are now logged with context instead of silently dropped.

**Remaining:** `internal/ops/submit_review.go:88` ignores `AbortRebase` error. `internal/git/worktree.go` ignores cleanup errors in 3 locations. `internal/ops/wt_delete.go:61` ignores branch-deletion error.

**Direction:** Keep best-effort semantics where appropriate, but surface cleanup outcomes as warning/error in result structs and command output. Escalate to error when cleanup failure can leave the system in a materially inconsistent state.

#### ~~Smell: Stale-lock cleanup errors are discarded during lock recovery~~ *(Adversarial pass, entry: error handling — resolved: `729da05`)*

**Signal:** `internal/filelock/filelock.go:170` called `cleanupStaleLock()` without checking its error before retrying lock acquisition.

**Fix:** `cleanupStaleLock()` failure now propagated as `LockErrorFilesystem` before retrying lock acquisition. Tests added: `TestCleanupStaleLockFailure` verifies behavior, `TestWithLockStaleLockRecovery` verifies stale lock detection and cleanup path.

#### Smell: File existence checks often collapse non-existence with other stat failures *(Adversarial pass, entry: error handling)*

**Signal:** Several paths only branch on `err == nil` or `os.IsNotExist(err)` (for example `internal/commands/setup.go:33`, `internal/commands/init.go:32`, `internal/commands/init.go:38`, `internal/git/worktree.go:162`) without explicit handling for permission/I/O errors.

**Impact:** Low-medium. Permission or transient filesystem errors can be misreported as simple presence/absence outcomes, producing misleading diagnostics and control flow.

**Direction:** Use explicit triage on `os.Stat` calls: exists, not-exist, and other-error (returned with context).

#### Smell: `submit-for-review` commit input is required but not authoritative *(Adversarial pass, entry: data flow)*

**Status: Fixed** (`d4c688e`)

**Signal:** `submit-for-review` interfaces require `commit_sha` (`cmd/liza/main.go:248`, `internal/mcp/server.go:425`, `internal/ops/submit_review.go:29`), but runtime does not validate it against worktree HEAD and always persists post-rebase HEAD as `review_commit` (`internal/ops/submit_review.go:108`, `internal/ops/submit_review.go:137`).

**Impact:** High. API/CLI contracts imply caller-provided commit authority while execution ignores that value after non-empty validation. This creates interface-to-runtime drift and weakens traceability of reviewed content.

**Direction:** Align contract and runtime semantics: either remove `commit_sha` from command/tool surfaces and docs (derive internally), or enforce strict equality between provided `commit_sha` and pre-rebase worktree HEAD.

#### ~~Smell: `DeleteTask` side effects can outpace state commit under race~~ *(Adversarial pass, entry: data flow — resolved: `7dd05ce`)*

**Signal:** `internal/ops/delete_task.go` performed worktree/branch side effects before atomic state mutation, allowing state↔filesystem divergence if `bb.Modify` failed.

**Fix:** Moved worktree/branch cleanup to run only after successful state mutation. Existing warning behavior preserved for cleanup failures. Regression test added that forces commit failure and asserts task, worktree, and branch are preserved.

#### Smell: REJECTED reassignment deletes old worktree before replacement is secured *(Adversarial pass, entry: documented smells)*

**Status: Fixed** (`ccaf9b0`)

**Signal:** In `internal/ops/claim_task.go`, the different-coder REJECTED path removes existing worktree/branch (`internal/ops/claim_task.go:149`, `internal/ops/claim_task.go:150`) before attempting new worktree creation (`internal/ops/claim_task.go:153`). If creation fails, Phase 3 never commits state, but prior artifacts may already be gone.

**Impact:** High. The task can remain REJECTED in state while its previous worktree has been deleted. This directly conflicts with reclaim paths that expect REJECTED worktree presence and creates state↔filesystem drift under failure.

**Direction:** Make reassignment replacement atomic in effect: create/validate replacement first, then retire old artifacts; or add explicit compensating recovery markers when delete succeeded but recreate failed.

#### ~~Smell: Planner max-wait config is modeled but not authoritative~~ *(Adversarial pass, entry: documented smells — resolved: `1d4f4f4`)*

**Signal:** `Config.PlannerMaxWait` was defined and loaded but planner role unconditionally overrode effective max wait to one year (`365 * 24 * time.Hour`).

**Fix:** Removed the hardcoded 365-day override. Planners now respect the configured `planner_max_wait` value (with `DefaultPlannerMaxWait=1800s/30min` as fallback when unset). Tests added including `TestPlannerRespectsMaxWaitConfig`. Documentation comment added to `PlannerMaxWait` field.

#### ~~Smell: Watch stall detection and YAML append logging form O(n) growth paths~~ *(Adversarial pass, entry: documented smells — resolved: `fe8de6b`, `61b16d5`)*

**Signal:** Watcher stall check read/scanned full `log.yaml` every cycle. `log.Logger.Append` read and rewrote full YAML entries on each append.

**Fix:** Two changes: (1) `fe8de6b` switched `Logger.Append` to append-only writes (directly appends YAML sequence items under file lock, no O(n) rewrite). `GetLastTimestamp` uses bounded tail-window parser for sub-linear reads. Regression tests verify existing bytes are preserved on append. (2) `61b16d5` replaced `checkStalled`'s raw `timestamp:` scanning with `log.GetLastTimestamp()` — watch now benefits from the sub-linear tail reads.

#### ~~Smell: MCP stdio transport has no frame-size guard~~ *(Adversarial pass, entry: documented smells — resolved: `c2fe02b`)*

**Signal:** `ReadRequest` used `ReadBytes('\\n')` on stdio with no explicit maximum request size.

**Fix:** Added `MaxRequestSize` constant (10MB) with `readLineBounded()` using `bufio.Reader.Peek` for controlled reading. `discardLine()` helper consumes oversized requests. `ErrRequestTooLarge` error and `RequestTooLarge` JSON-RPC error code (-32005) added. Comprehensive tests (214 LOC in `stdio_test.go`) cover size limit enforcement, error responses, and normal operation.

### 2.4 Patterns

| Pattern | Where Used | Purpose |
|---------|------------|---------|
| Repository (Blackboard) | `internal/db/` | Encapsulates file-based state persistence with locking |
| Strategy (CLIExecutor) | `internal/agent/` | Pluggable agent CLI backend (claude, codex, gemini, vibe) |
| Command | `internal/commands/` | Each CLI command is an independent function with uniform interface |
| Template Method | `internal/prompts/` | Role-specific prompts built from shared templates |
| Observer (Watcher) | `internal/db/watcher.go` | Event-driven state change notification via fsnotify |
| Strategy (claimRelease) | `internal/ops/release_claim.go` | Parameterized coder/reviewer claim release — eliminates duplication between two nearly-identical release flows *(pass 5, Duplication lens: notable counterexample)* |
| Registry | `internal/models/`, `internal/roles/` | Task type → role workflow mapping; unified role constants with runtime↔workflow mapping |
| State Machine | `internal/models/` | Explicit `taskTransitions` map with `CanTransition()`/`Transition()` *(pass 2: added)* |
| Circuit Breaker | `internal/analysis/` | Pattern detection on anomalies triggers system pause |
| Heartbeat/Lease | `internal/agent/heartbeat.go` | Agent liveness detection via periodic lease extension |
| Embed | `internal/embedded/` | Contract/skill files embedded in binary via `go:embed` |
| Adapter | `internal/mcp/` | Translates JSON-RPC wire format into commands calls *(pass 3: identified)* |

### 2.5 Test Coverage

**Overall:** ~15,300 source LOC, ~36,700 test LOC. 2.4:1 ratio. **75.3% statement coverage** (from `go tool cover -func`). *(pass 4: statement-level data added; pass 7: LOC updated)*

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
- `cmd/liza/main.go` (1,275 LOC): 717 LOC tests (0.6:1) — CLI wiring; ~~only `get` exercised through cobra path~~ *(resolved: `9d95c1c` added mutation wiring tests for 4 commands + identity resolution)*
- `internal/models/diagnostics.go` (127 LOC): zero tests, no test file — work detection functions used by supervisor *(pass 4)*
- `internal/mcp/protocol/errors.go` (68 LOC): zero tests, no test file *(pass 4)*
- ~~CLI contract coverage is narrow: only `get` is exercised through cobra `rootCmd.Execute()` path~~ *(resolved: `9d95c1c`)*: `claim-task`, `submit-verdict`, `wt-merge`, `release-claim` now exercised through cobra path with flag/env wiring validation

**Critical 0% coverage paths** *(pass 4, Coverage lens)*:
The 24.7% uncovered code concentrates in two patterns:
1. **Runtime orchestration** — `supervisor.Execute/ExecuteInteractive`, `mcp/server.HandleRequest/Run`, `mcp/server.classifyError` — the integration seams that wire tested components together
2. **I/O-coupled functions** — `embedded.WriteMCPSettings/mergeMCPSettings`, `mcp/protocol/stdio`, `DefaultCLIExecutor` — hardwired to OS-level I/O

**Partially covered functions of concern** *(pass 4; updated with subsequent coverage improvements)*:
| Function | Coverage | Status |
|----------|----------|--------|
| ~~`supervisor.resumeHandoffTask`~~ | ~~11.4%~~ | Resolved (`d8533ab`): success/failure/edge-case tests added |
| ~~`validate.validateAnomalies`~~ | ~~13.3%~~ | Resolved (`d8533ab`): targeted table-driven tests for all anomaly type branches |
| ~~`inspect_field.getSprintMetricsField`~~ | ~~29.4%~~ | Resolved (`c4bd748`): replaced by reflect-based `getField` with exhaustive tag tests |
| `validate.validateHandoff` | 33.3% | Handoff invariant checking — improved by `d8533ab` |
| ~~`inspect_field.getConfigField`~~ | ~~41.2%~~ | Resolved (`c4bd748`): replaced by reflect-based `getField` with exhaustive tag tests |

**I/O coupling as testability barrier** *(pass 4, Coverage lens)*: Functions at 0% coverage strongly correlate with hardwired I/O — this is the Coverage lens perspective on the Boundaries smell (pass 3). The `CLIExecutor` interface demonstrates the solution pattern: abstracting one I/O boundary enabled comprehensive supervisor testing. ~~`DeleteTaskCommand` prompts~~ (resolved: business logic extracted to `ops.CheckDeleteTask` + `ops.DeleteTask`, both fully testable without I/O). ~~`WriteMCPSettings`~~ stdin coupling resolved (`7a5e79c`: `io.Reader` injection). `StdioTransport` bounded read tests achieved without injection (`c2fe02b`), but full `Run()` loop testing still requires I/O injection.

**Integration tests:** 4 files in `internal/integration/` (1,397 LOC) covering concurrent operations, sprint/merge workflows, e2e command sequences, lease expiry. All files guarded by `testing.Short()` — skipped under `go test -short` *(resolved: `84b5a64`)*.

**Test patterns:** Table-driven (dominant), filesystem isolation, hand-written mocks (no frameworks), real git operations. No property-based or fuzz testing. *(pass 3: `os.Stdin` monkey-patching pattern noted as testing boundary smell — 8+ test files)*

**Temporal coupling signal** *(Adversarial pass, entry: tests/ — partially resolved: `1914732`, `1ff88d2`)*: ~~21 explicit `time.Sleep()` calls and 0 `t.Parallel()` uses across 93 test files~~ Now 5 `time.Sleep()` calls and 15 `t.Parallel()` uses across 101 test files. `resetRootCmdForTest(t)` isolates process-global state. `internal/testguard/` ratchet tests enforce `t.Parallel()` floor (≥10) and `time.Sleep()` ceiling (≤11), preventing regression. Remaining serial tests are constrained by process-global state (`rootCmd`, `os.Chdir`), not by missing infrastructure.

**`check-testhelpers` build guard** *(pass 4)*: Makefile target prevents `testhelpers` import in production code — good practice for maintaining test/production boundary.

---

## Phase 3: Recommendations

| Priority | Issue | Rationale | Action |
|----------|-------|-----------|--------|
| **High** | Pairing initialization doc pointer drift (`docs/USAGE.md`) *(Adversarial pass, entry: specs/)* | Session Initialization in `PAIRING_MODE.md` requires a non-existent file; startup protocol can fail before task execution | Point initialization to canonical docs (`USAGE_PAIRING.md` and/or `docs/README.md`) |
| ~~**Medium**~~ | ~~Troubleshooting worktree recovery branch mismatch~~ *(Adversarial pass — resolved: `ee8a55d`)* | Canonicalized recovery examples to `task/<id>` | Moved to Fixed (Traceability) |
| ~~**Medium**~~ | ~~Testing `-short` behavior mismatch~~ *(Adversarial pass — resolved: `84b5a64`)* | Added `testing.Short()` guards; docs updated | Moved to Fixed (Traceability) |
| ~~**Medium**~~ | ~~CLI contract coverage gap in `cmd/liza/main.go`~~ *(Adversarial pass — resolved: `9d95c1c`)* | 4 mutation commands + identity resolution exercised through cobra path | Moved to Fixed (Traceability) |
| **Low** | Temporal test coupling *(Adversarial pass, entry: tests/ — partially resolved: `1914732`, `1ff88d2`)* | 5 `time.Sleep()` calls (down from 21), 15 `t.Parallel()` uses (up from 0), ratchet tests prevent regression; remaining serial tests constrained by process-global state | Continue tightening ratchets; `--project-root` flag would enable full parallelization |
| **Low** | Residual raw `1800` in supervisor.go *(pass 6)* | 2 call sites bypass `models.DefaultLeaseDurationSeconds` constant | Replace with named constant |
| **Low** | Duplicated identity validation *(pass 6)* | `agent/registration.go` reimplements `identity` package logic | Replace with `identity.ValidateFormat()` + `ValidateRole()` |
| **Low** | Inconsistent ops parameter conventions *(pass 6)* | `AddTask` takes `statePath` while 15+ others take `projectRoot` | Standardize on `projectRoot` |
| **Low** | `StdioTransport` not injectable *(partially addressed: `c2fe02b`)* | Bounded read tests achieved without injection; `Run()` still untestable | Accept `io.Reader`/`io.Writer` params for full loop testing |
| **Low** | `PlannerContextConfig` empty struct | Premature abstraction | Remove or document intent |
| **Low** | Duplicated template execution pattern *(pass 3)* | `commands/templates.go` and `prompts/templates.go` near-identical | Extract shared template infrastructure or accept as coincidental similarity |
| **Low** | `derefString` duplicated | In builder.go and templates.go funcMap | Use template func only |
| **Low** | `LIZA_LOG_LEVEL` documentation drift *(Adversarial pass, entry: config/)* | Env var documented but no runtime reader; logger is fixed at INFO | Implement env-driven log level or remove from docs |
| **Low** | `os.Stat` existence checks under-handle non-`IsNotExist` errors *(Adversarial pass, entry: error handling)* | Some presence checks classify only exists/missing and miss permission/I/O distinctions | Standardize tri-state handling (exists / missing / other error) with contextual wrapping |
| **Low** | `validateTaskInvariants` monolithic if-chain *(pass 7)* | 142 LOC, ~15 checks ungrouped by status; hard to verify completeness | Group checks by status or use switch |
| **Low** | High nesting in `claiming.go` helpers *(pass 7, partially resolved: `ac4ce6f5`)* | `handleApprovedMerges` depth 6 remains; ~~`resumeHandoffTask` depth 5~~ extracted to `ops.ResumeHandoff` | Extract inner merge-attempt body |
| **Low** | `cmd/liza/main.go` (1,275 LOC) *(pass 2)* | Organizational god file; behavioral complexity is low | Consider splitting cobra definitions into per-command files if growth continues |
| **Low** | No interface-based seams *(pass 3)* | Deliberate simplicity; acceptable for v1 | Monitor test suite time; introduce seams if needed |
| **Low** | Mutable package-level version variables *(pass 3)* | `mcp.Version = embedded.Version` cross-assignment | Consider constructor parameter or build-time injection |
| **Low** | Regenerate `coverage.out` *(pass 4)* | Report shows 0% for functions with thorough tests; may predate recent commits | Run `make test` to update |
| **Low** | Broken Vision link in sprint governance spec *(Adversarial pass, entry: specs/)* | `../vision.md` target is missing; canonical Vision is under `specs/build/` | Fix link to canonical Vision path |
| **Low** | Ops Modify-callback task guard *(pass 5, Duplication lens)* | 10 files repeat identical FindTask+NotFoundError inside Modify callbacks | Consider `modifyTask(bb, taskID, fn)` helper; evaluate clarity vs indirection |
| **Low** | Command test `initialState` construction *(pass 5, Duplication lens)* | 23 near-identical State constructions with same Config values | Add `testhelpers.DefaultState()` returning pre-configured State |
| **Low** | Watch thresholds not configurable *(pass 6)* | 10 operational constants bypass `models.Config` pattern | Add to Config with current values as defaults |
| **Low** | Hardcoded `"terminal-1"` *(pass 6)* | All agents report same terminal regardless of actual TTY | Derive from config or actual terminal |
| **None** | Ops input validation boilerplate *(pass 5)* | 10 files with `if taskID == ""` — idiomatic Go, low risk | Not worth abstracting |
| **None** | `task.History = append(...)` pattern *(pass 5)* | 12 occurrences with variations — coincidental similarity | Not worth abstracting |
| **None** | `formatKeyValue` bubble sort | Works, small data sets, not perf-sensitive | Not worth changing |
| **None** | Global logger singleton | Acceptable for CLI scope | Not worth changing for v1 |

---

## Fixed (Traceability)

| Issue | Original Priority | Resolved In | Date Moved | Evidence |
|-------|-------------------|-------------|------------|----------|
| Magic number 1800 lease default | Medium | `150c4d0` | 2026-02-21 | `models.DefaultLeaseDurationSeconds` introduced; central defaults now named |
| `executeTemplate` panic behavior | Medium | `ad3288c` | 2026-02-21 | `executeTemplate`/`executeCommandTemplate` now return error instead of panic |
| Inconsistent `NotFoundError` usage | Medium | `e6f7bd2` | 2026-02-21 | Ad-hoc `task not found` strings migrated to structured `NotFoundError` |
| Poll/wait fallback magic numbers | Medium | `150c4d0` | 2026-02-21 | Poll/wait defaults consolidated as named model constants |
| `supervisor.go` god file decomposition | High | `c281430` | 2026-02-21 | Supervisor split into 6 cohesive files (`supervisor`, `registration`, `waitforwork`, `claiming`, `prompt`, `systemctl`) |
| Task lookup duplication (inline loops) | High | `363b440` | 2026-02-21 | Replaced repeated loops with `State.FindTask()` / `State.FindTaskIndex()` |
| Commands-to-ops extraction (agent boundary fix) | High | `c7e98d7`, `bfe179d`, `e7d020d` | 2026-02-21 | Business logic moved to `internal/ops`; agent and MCP mutate via ops, not command handlers |
| Duplicated file locking mechanism | Medium | `a0bd779` | 2026-02-21 | Shared locking extracted into `internal/filelock` and reused by db/log paths |
| MCP state read bypassed Blackboard lock | High | `af911ed` | 2026-02-21 | MCP state resource now reads via lock-safe `Blackboard.ReadRaw()` |
| MCP dispatch/diagnostics critical-path test gaps | Medium | `40ef645` | 2026-02-21 | Added dispatch classification tests and diagnostics coverage |
| Task-state-machine spec drift (`BLOCKED -> READY`) | High | `0f6fe19` | 2026-02-21 | Spec now forbids `BLOCKED -> READY` and matches runtime transition map (`BLOCKED -> SUPERSEDED|ABANDONED`) |
| Iteration-limit config drift (`max_coder_iterations`, `max_review_cycles`, `task.max_iterations`) | High | `5fceaad` | 2026-02-21 | `ClaimTask` + `SubmitVerdict` now enforce effective limits and transition exhausted loops to BLOCKED (clean-code follow-up: `be93dee`) |
| Hardcoded `"task/"` branch prefix in 7 files | Medium | `59a8e3e` | 2026-02-24 | `paths.TaskBranchPrefix` constant; all sites migrated |
| Role naming divergence (`"code-reviewer"` vs `"code_reviewer"`) | Medium | `a60c72e` | 2026-02-24 | `internal/roles` package with `ToWorkflow()`/`ToRuntime()` mapping |
| Divergent GracePeriod values (60s vs 120s) | Medium | `b9f20ff` | 2026-02-24 | Unified `models.LeaseExpiryGracePeriod` constant |
| `ClaimTask` function complexity (265 LOC, depth 6) | Medium | `e86abd4` | 2026-02-24 | Phase helpers extracted; `unmetDependencies()` shared |
| `inspect_field.go` manual reflection (9 switch statements) | Medium | `c4bd748` | 2026-02-24 | Reflect-based YAML-tag walker; exhaustive tests |
| Interactive stdin in library packages | Medium | `7a5e79c` | 2026-02-24 | All 8 locations accept `io.Reader`; monkey-patching eliminated |
| `validate.validateAnomalies` at 13.3% coverage | Medium | `d8533ab` | 2026-02-24 | Targeted table-driven tests for all anomaly types |
| `supervisor.resumeHandoffTask` at 11.4% coverage | Medium | `d8533ab` | 2026-02-24 | Success/failure/edge-case tests added |
| MCP stdio transport no frame-size guard | Medium | `c2fe02b` | 2026-02-24 | `MaxRequestSize` (10MB) with bounded read; `RequestTooLarge` error |
| Watch stall detection parses YAML text directly | Medium | `61b16d5` | 2026-02-24 | Uses `log.GetLastTimestamp()` typed parser |
| Watch/log O(n) growth paths (append rewrite + full scan) | Medium | `fe8de6b` | 2026-02-24 | Append-only writes + bounded tail-window reads |
| `heartbeat_interval` config ignored at runtime | Medium | `9e59acf` | 2026-02-24 | `NormalizeHeartbeatInterval()` with bounds validation |
| Planner max-wait config ignored | Medium | `1d4f4f4` | 2026-02-24 | Planners respect configured value; default 30min fallback |
| Stale-lock cleanup error discarded | Medium | `729da05` | 2026-02-24 | `cleanupStaleLock()` failure propagated as `LockErrorFilesystem` |
| `DeleteTask` side effects outpace state commit | Medium | `7dd05ce` | 2026-02-24 | Git cleanup deferred to after successful state mutation |
| `get config.*` projection drift | Low | `c4bd748` | 2026-02-24 | Reflect-based walker discovers all YAML-tagged fields |
| Worktree recovery doc branch mismatch | Medium | `ee8a55d` | 2026-02-24 | Recovery examples canonicalized to `task/<task-id>` placeholders |
| Test guidance drift for short mode | Medium | `84b5a64` | 2026-02-24 | `testing.Short()` guards added to all integration tests; docs updated |
| CLI contract coverage gap | Medium | `9d95c1c` | 2026-02-24 | `mutation_wiring_test.go` (215 LOC) covers 4 mutation commands + identity resolution through cobra path |
| Temporal test coupling (partial) | Medium→Low | `1914732`, `1ff88d2` | 2026-02-24 | `time.Sleep` reduced 21→5; `t.Parallel()` added (15 uses); ratchet tests in `testguard/` prevent regression |
| Exit code 42 restart loop (no progress detection) | Medium | `f15cd61`, `5f05403` | 2026-02-24 | Per-task restart tracking, exponential backoff (2s→max), circuit breaker to BLOCKED after threshold |
| Planner role invisible in type system | Medium | `e173f71` | 2026-02-24 | `models.RolePlanner`, `roles.WorkflowPlanner`, declarative `plannerWakeTriggerSpecs` |
| Two-Track State Mutation (partial) | Medium | `ac4ce6f` | 2026-02-24 | `ops.ClaimReviewerTask` + `ops.ResumeHandoff` extracted; agent lifecycle still inline |

---

## Summary

Liza's architecture is well-suited to its constraints: a file-based multi-agent coordination system for solo developers. The dependency graph is clean with no cycles. Test coverage is excellent (2.4:1 ratio) with consistent patterns and strong helper infrastructure. The atomic state persistence via flock and fsync+rename is correctly implemented. Health monitoring is comprehensive. The task state machine is now explicit with a complete transition map.

**Pass 2 (Complexity lens)** added two structural findings: (1) Monolithic command functions — `WtMergeCommand` and `ClaimTaskCommand` at 310-319 LOC each are the system's longest single functions, resisting comprehension and targeted testing. ~~(2) Task-lookup duplication — the same 6-line loop appears 55+ times, making it the largest DRY violation in the codebase~~ (resolved: `State.FindTask()` and `FindTaskIndex()` added, all inline lookups migrated).

**Pass 3 (Boundaries lens)** reveals the `commands` package as the system's central boundary concern. It serves two consumers (CLI, MCP) with different I/O expectations but embeds terminal assumptions (40+ stdout writes, ~~5+ stdin reads~~). ~~The supervisor's upward dependency on `commands` compounded this — orchestration logic inherited CLI presentation side effects~~ (resolved: business logic extracted to `internal/ops/` service layer; agent now imports `ops` instead of `commands`). The MCP adapter layer is clean (textbook adapter pattern), and the domain/persistence boundaries are well-drawn. ~~The remaining path forward is separating business logic from presentation in the remaining commands, which would enable clean MCP delegation and eliminate the stdin monkey-patching test pattern~~ (resolved: `7a5e79c` — all stdin reads now accept `io.Reader` parameter; monkey-patching eliminated).

**Pass 4 (Coverage lens)** adds quantitative depth: 75.3% statement coverage overall, with the uncovered 24.7% concentrated in two patterns — runtime orchestration code (supervisor Execute, MCP server dispatch) and I/O-coupled functions. The most actionable finding: `mcp/server.classifyError` and `HandleRequest` are pure logic at 0% that can be tested trivially without any refactoring. Similarly, `models/diagnostics.go` (127 LOC, 4 functions, no test file) is critical work-detection logic that's entirely untested despite being pure functions on `*State`. I/O coupling (already flagged as a Boundaries smell) is now quantitatively confirmed as the primary driver of untested critical paths — functions with hardwired `os.Stdin`/`os.Stdout`/`os/exec` account for the majority of the 0% coverage.

The primary structural concerns in priority order: ~~(1) `supervisor.go` god file~~ (resolved — decomposed into 6 files), ~~(2) commands presentation+logic coupling~~ (resolved — all 15 MCP-exposed mutation commands extracted to `internal/ops/`; MCP handlers call ops directly; protocol corruption risk eliminated), ~~(3) monolithic command functions~~ (resolved — all 4 monolithic commands extracted to ops; `DeleteTaskCommand` was the last, using the two-function pre-check + action pattern from `DeleteAgentCommand`), ~~(4) MCP handler bypassing Blackboard locking~~ (resolved), ~~(5) untested MCP dispatch and diagnostics~~ (resolved), ~~(6) agent→commands upward dependency~~ (resolved — `internal/ops/` service layer). The ops layer now contains 21 operations (~3,200 LOC) serving 3 consumers (agent, commands, mcp). ~~Remaining concerns: interactive stdin in CLI-only commands~~ (fully resolved: `7a5e79c` — `io.Reader` injection).

**Pass 5 (Duplication lens)** examined cross-file repetition patterns. The most significant duplication pattern is within the `ops/` package itself: 10 of 19 ops files repeat an identical 3-line FindTask+NotFoundError guard inside `bb.Modify` callbacks, and 12 files share structurally similar history-append code. The `readTaskState()` helper addresses this for the Read path but has no equivalent for the Modify path. This is idiomatic Go — each function is independently authored with the same pattern — and the impact is low (maintenance burden if the guard pattern changes). In test code, 23 command test files construct near-identical `initialState` objects; a `testhelpers.DefaultState()` helper would be a low-risk improvement. Overall, the codebase's earlier duplication issues (task-lookup loops 55×, file-locking, magic numbers) have been resolved. The remaining repetition is largely structural — Go's explicit style trading conciseness for clarity.

**Pass 6 (Coupling lens)** focused on configuration hardcoding, tight dependencies, and hidden state sharing. ~~The most significant finding is the `"task/"` branch prefix scattered as a string literal across 7 production files with no constant~~ (resolved: `59a8e3e` — `paths.TaskBranchPrefix` constant). ~~The role naming divergence between agent config roles (`"code-reviewer"`, hyphen) and task workflow constants (`"code_reviewer"`, underscore) is a semantic coupling risk~~ (resolved: `a60c72e` — `internal/roles` package with explicit mapping). The magic number 1800 marked as resolved in pass 1 still has 2 residual call sites in `supervisor.go`. Other findings: ~~`GracePeriod` divergence (60s vs 120s for semantically identical concepts)~~ (resolved: `b9f20ff` — unified `models.LeaseExpiryGracePeriod`), duplicated identity validation between `agent` and `identity` packages, inconsistent ops parameter conventions (`projectRoot` vs `statePath`), and watch thresholds with no config path. The coupling patterns are generally low-to-medium impact — the architecture's clean dependency graph prevents coupling from cascading. Remaining open items: identity validation duplication, ops parameter convention split, watch threshold configurability, raw 1800 residuals, and hardcoded `"terminal-1"`.

**Pass 7 (Complexity lens)** revisits complexity with the benefit of 6 prior passes of context. The earlier Complexity lens (pass 2) correctly identified the monolithic commands and task-lookup duplication — both now resolved. This pass reveals the complexity that *replaced* them: ~~`ops/claim_task.go:ClaimTask` at 265 LOC is now the longest function in the codebase~~ (resolved: `e86abd4` extracted phase helpers and `unmetDependencies()` shared function; `0158b64` removed format-string wrapper). `ops/wt_merge.go:MergeWorktree` at 377 LOC (file total) remains a complex function with phased flow. ~~A new finding not in previous passes: `commands/inspect_field.go` (327 LOC, 9 switch statements) is a hand-written reflection system where every model field requires a manual switch case~~ (resolved: `c4bd748` replaced with reflect-based YAML-tag walker; now 275 LOC with exhaustive tag tests). LOC figures updated: production code is ~15,300 LOC (stable), test code grew to ~36,700 LOC (2.4:1 ratio, up from 2:1 at pass 1), and `commands/` is 4,300 LOC (up from 2,800 recorded earlier, likely due to test growth during ops extraction).

**Adversarial pass (entry: docs/)** forced a doc-first path and surfaced contract-level drift missed by prior code-centric passes. All items now resolved: ~~state-machine spec said `BLOCKED -> READY` while runtime disallowed it~~ (fixed in `0f6fe19`), ~~troubleshooting recovery branch naming drift (`task-1` vs `task/<id>`)~~ (resolved: `ee8a55d` — canonicalized to `task/<task-id>`), and ~~testing-doc short-mode drift for integration tests~~ (resolved: `84b5a64` — `testing.Short()` guards added, docs updated).

**Adversarial pass (entry: specs/)** surfaced three additional coherence gaps: (1) Pairing Session Initialization still references `docs/USAGE.md` even though docs were split into `USAGE_PAIRING.md` and `USAGE_MULTI_AGENTS.md`, ~~(2) watcher stall detection parses raw `log.yaml` text for `timestamp:` instead of using typed log parsing~~ (resolved: `61b16d5` — uses `log.GetLastTimestamp()`), and (3) sprint governance links Vision via `../vision.md` while canonical Vision lives in `specs/build/0 - Vision.md`. The initialization-path drift remains high leverage because it can break startup behavior before execution begins.

**Adversarial pass (entry: tests/)** highlighted a distinct quality boundary: ~~the suite strongly validates command/ops internals but under-exercises the binary CLI contract~~ (resolved: `9d95c1c` — `mutation_wiring_test.go` covers 4 mutation commands + identity resolution through cobra path; `cmd/liza` test ratio improved from 0.3:1 to 0.6:1). Temporal coupling signals also addressed (partially resolved: `1914732`, `1ff88d2` — `time.Sleep` calls reduced from 21 to 5, `t.Parallel()` introduced with 15 uses across 4 files, ratchet tests in `internal/testguard/` prevent regression). Remaining serial tests are constrained by process-global state (`rootCmd`, `os.Chdir`), not by missing infrastructure.

**Adversarial pass (entry: config/)** exposed a config-contract gap cluster: key control knobs are modeled and documented but not always executable. One high-priority gap is now closed: ~~iteration limits (`config.max_coder_iterations`, `config.max_review_cycles`, and `task.max_iterations`) are not enforced in runtime task/review flow~~ (resolved in `5fceaad` via `ClaimTask`/`SubmitVerdict` enforcement; clean-code-only refactor follow-up in `be93dee`). Subsequent fixes closed more config gaps: ~~`heartbeat_interval` is still ignored in favor of a hardcoded 60s scheduler~~ (resolved: `9e59acf` — wired to config with bounds validation), ~~`get config.*` still projects only a subset of `models.Config`~~ (resolved: `c4bd748` — reflect-based walker). Remaining open config drift: `LIZA_LOG_LEVEL` remains unimplemented.

**Adversarial pass (entry: error handling)** surfaced a reliability-observability gap cluster: protocol and cleanup error paths are intentionally or accidentally lossy. ~~MCP parse-error response write failures are currently ignored~~ (resolved: `80297b9` — terminal). ~~Lock stale-cleanup errors are dropped before retry~~ (resolved: `729da05` — propagated as `LockErrorFilesystem`). Cleanup failures in claim_task.go partially surfaced (`729da05`). Remaining: rebase/worktree cleanup flows in `submit_review.go`, `git/worktree.go`, and `wt_delete.go` still suppress secondary failures. Some `os.Stat` checks still under-handle non-`IsNotExist` filesystem errors.

**Adversarial pass (entry: data flow)** traced the task lifecycle across CLI/MCP inputs, state transitions, and git side effects. Two integrity gaps emerged: `submit-for-review` treats caller `commit_sha` as required input but runtime authority comes from computed post-rebase HEAD (fixed in `d4c688e`, then regressed), and ~~`DeleteTask` can apply destructive git side effects before final lock-time state commit~~ (resolved: `7dd05ce` — git cleanup deferred to after state mutation).

**Adversarial pass (entry: documented smells)** started from already-documented smell clusters and found adjacent blind spots at boundary seams: ~~(1) REJECTED reassignment teardown happens before replacement creation is secured~~ (resolved: `ccaf9b0`), ~~(2) planner max-wait is modeled but runtime-overridden~~ (resolved: `1d4f4f4` — planners respect configured value), ~~(3) watch/log paths rely on full-file scans and rewrites that scale poorly with log growth~~ (resolved: `fe8de6b`, `61b16d5` — append-only writes + bounded tail reads), and ~~(4) MCP stdio framing has no explicit request-size guard~~ (resolved: `c2fe02b` — 10MB bounded read). All four documented smells from this pass are now resolved.

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
| Role constants | `internal/roles/` |
| Error types | `internal/errors/` |
| Test helpers | `internal/testhelpers/` |
| CLI entry point | `cmd/liza/` |
| MCP entry point | `cmd/liza-mcp/` |
| Integration tests | `internal/integration/` |
