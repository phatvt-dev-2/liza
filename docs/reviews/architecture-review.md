# Architecture Review — Liza

**Date:** 2026-02-21
**Mode:** Adversarial (after pass 12)
**Reviewer:** software-architecture-review skill

---

## Update Policy

1. `Phase 3: Recommendations` tracks open issues only.
2. When an issue is fixed, remove it from Recommendations.

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

**Source size:** ~15,800 LOC production Go, ~41,700 LOC test Go (2.6:1 test-to-code ratio). *(pass 7: updated from ~15,400/~30,800; updated: statevalidate extraction + schema consistency tests)*

### 1.2 Component Walkthrough

#### models (`internal/models/`) — ~680 LOC

**Purpose:** Core domain model. Task lifecycle state machine, agent state, sprint tracking.

**Observations:**
- `State` struct is the central data type — serialized to/from `state.yaml`. `state.go` is 631 LOC with 20 structs — cohesive (all YAML-serialized state types) *(pass 5: updated LOC)*
- `Task` struct has 30+ fields covering full lifecycle
- `TaskType` → role workflow registry (`taskWorkflows` map)
- `IsClaimable()` encodes claiming rules with dependency checking
- 12 task statuses with `IsValid()`, `IsTerminal()` methods
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

#### agent (`internal/agent/`) — 1,716 LOC (6 files)

**Purpose:** Supervisor loop, heartbeat, work detection, logging.

**Observations:**
- 6 cohesive files: `supervisor.go` (~460 LOC), `registration.go` (~175 LOC), `waitforwork.go` (~300 LOC), `claiming.go` (~160 LOC), `prompt.go` (~95 LOC), `systemctl.go` (~160 LOC)
- `RunSupervisor()` (186 LOC, nesting depth 5): checkAbort → waitWhilePaused → handleApprovedMerges → waitForWork → claimTask → buildPrompt → executeAgent → handleExitCode *(pass 7, Complexity lens: nesting depth noted)*
- `CLIExecutor` interface enables mock testing (supports claude, codex, gemini, vibe)
- `waitForWorkEventDriven()` (116 LOC) with fsnotify + polling fallback
- `verifyPlannerStateChanges()` (137 LOC) — 6 switch cases with repetitive before/after counting structure *(pass 2, Complexity lens)*
- `heartbeat.go`: independent Blackboard instance, 60s tick, extends lease
- `workdetection.go` (~170 LOC): 6 planner wake trigger types, now declarative via `plannerWakeTriggerSpecs` (trigger → description → state predicate) replacing imperative branching
- `logging.go`: package-level singleton `slog.Logger`, hardcoded to stdout
- **Core execution paths untested**: `Execute()`, `ExecuteInteractive()`, `handleApprovedMerges()`, `logTaskSubmissionIfCompleted()` at 0% statement coverage; `resumeHandoffTask()` at 11.4%. These are the actual agent loop entry points — tested indirectly via `TestSupervisorBasicLoop` with mock executor but not at statement level *(pass 4, Coverage lens)*
- **`handleApprovedMerges` high nesting-to-LOC ratio**: 55 LOC but nesting depth 6 (for-range → if-status → if-approved → if-merge-commit → if-err → errors.As). The deepest nesting processes `IntegrationFailedError` fields conditionally. ~~Relatedly, `resumeHandoffTask` (63 LOC) reaches nesting depth 5 inside its `bb.Modify` closure~~ *(pass 7, Complexity lens — partially resolved: `ac4ce6f5` extracted `resumeHandoffTask` to `ops.ResumeHandoff`)*
- ~~**Role string literals instead of constants**~~ *(pass 6, Coupling lens — mostly resolved: `a60c72e`)*: `internal/roles` package introduced with `RuntimeCoder`, `RuntimeCodeReviewer`, `RuntimePlanner` constants and `ToWorkflow()`/`ToRuntime()` mapping. All agent/, cmd/, and ops/ files now import role constants. 1 residual: `claiming.go:115` still uses `Role: "coder"` literal
- **Duplicated identity validation**: `registration.go:validateIdentity()` reimplements `identity.ValidateFormat()` + `identity.ValidateRole()` — same algorithm (split on last hyphen, validate numeric suffix, check role prefix) without importing the `identity` package *(pass 6, Coupling lens)*
- **Hardcoded `"terminal-1"` and raw `1800`**: `supervisor.go:127` passes `"terminal-1"` literal and `1800` instead of `models.DefaultLeaseDurationSeconds`; `supervisor.go:221` also uses raw `1800` *(pass 6, Coupling lens)*

#### statevalidate (`internal/statevalidate/`) — 463 LOC

**Purpose:** Shared state validation pipeline, extracted from `commands/validate.go` to allow both CLI and ops to run identical validation without import cycles.

**Observations:**
- `ValidateStateFile()` runs 9 validators: required fields, task states, task invariants, dependencies, agent invariants, handoff, discovered, anomalies, sprint
- Accepts `io.Writer` for non-fatal warnings (nil defaults to `io.Discard`)
- Exported shims `ValidateAgentInvariants()` and `ValidateAnomalies()` expose individual validators for existing `commands/` test callsites
- Used by: `commands/validate.go` (CLI `liza validate`), `ops/add_task.go` (post-write validation)

#### ops (`internal/ops/`) — ~3,750 LOC production, ~6,450 LOC test (25 files)

**Purpose:** Pure business logic layer for all task workflow and system operations. Returns structured results with no terminal I/O side effects.

**Pattern:** Service layer — extracted from `commands` to break the agent→commands upward dependency and eliminate MCP protocol corruption risk.

**Observations:**
- 25 operations covering all mutation commands:
  - Task workflow: `ClaimTask`, `ClaimReviewerTask`, `SubmitForReview`, `SubmitVerdict`, `Handoff`, `ResumeHandoff`, `MarkBlocked`, `ReleaseClaim`, `SupersedeTask`, `AddTask`, `CheckDeleteTask`, `DeleteTask`
  - Agent lifecycle: `DeleteAgent`, `IsAgentProcessRunning`
  - System mode: `Start`, `Stop`, `Pause`, `Resume`
  - Worktree: `CreateWorktree`, `DeleteWorktree`, `MergeWorktree`
  - Sprint: `UpdateSprintMetrics`, `Checkpoint`, `Analyze`
  - Maintenance: `ClearStaleReviewClaims`
- Each function returns a typed result struct (e.g., `*VerdictResult`, `*HandoffResult`, `*ModeChangeResult`)
- Zero `fmt.Print*` or `os.Stdin` calls — verified by grep
- Three consumers: `agent/` (orchestration), `commands/` (CLI presentation), `mcp/` (JSON-RPC adapter)
- Depends on: `db`, `models`, `git`, `log`, `paths`, `analysis`, `statevalidate` — same layer as `commands` minus presentation concerns
- `wt_merge.go` (389 LOC): `MergeWorktree` — linear phased flow (validate → merge → integration tests → update state → cleanup). Now logs WARNING when integration test script is missing and persists `tests_ran` in merge history. Tri-state stat handling for test script presence *(pass 7, Complexity lens; `bce626d`, `52ceac5`)*
- `helpers.go` provides `readTaskState()` for Read-path task lookup, but no equivalent exists for the Modify-callback path *(pass 5, Duplication lens)*
- **Structural repetition within ops** *(pass 5, Duplication lens)*: Most ops functions share an identical skeleton — input validation → `paths.New(projectRoot)` + `db.For(lp.StatePath())` → `bb.Modify(func(state) { FindTask + nil check + status check + mutate + history append })` → wrap error → return result. Quantified: `if taskID == ""` guard in 10/21 files, `FindTask + NotFoundError` inside Modify in 10 files, `task.History = append(...)` in 12 files. See Duplication smell below.
- **Inconsistent parameter conventions** *(pass 6, Coupling lens)*: Some ops functions take `projectRoot` and internally construct `paths.New()` + `db.For()` (ClaimTask, MergeWorktree, DeleteTask, SubmitReview, etc.), while others take `statePath`/`logPath` directly (AddTask). Callers must know which convention each function uses. See Coupling smell below.

#### commands (`internal/commands/`) — ~3,980 LOC *(pass 7: updated from ~2,800; `6fe5bcc`: validation logic extracted to `statevalidate`)*

**Purpose:** CLI presentation wrappers over `ops/` business logic, plus read-only query commands.

**Pattern:** Thin wrapper per command: call `ops.*`, format and print result. Read-only commands (inspect, status, validate) retain their own logic since they already return structured data.

**Observations:**
- 25+ command implementations — mutation commands are thin wrappers (~20-75 LOC each), read-only commands retain logic
- `watch.go` (516 LOC): 11 health checks with alert deduplication, comprehensive monitoring
- `validate.go` (28 LOC): thin wrapper delegating to `internal/statevalidate` package *(pass 2; pass 7: LOC updated; `6fe5bcc`: validation logic extracted to shared package)*
- `format.go` (164 LOC): centralized JSON/YAML/table formatting
- Templates in `commands/templates/`: status_dashboard, agent_value, metrics_value
- **Self-constructing infrastructure** — each command function creates fresh `paths.New()`, `db.New()`, `git.New()` instances internally; no dependency injection *(pass 3, Boundaries lens)*
- **Watch thresholds hardcoded** — 10 constants (`DefaultCheckInterval`, `LeaseGracePeriod`, `StallThreshold`, etc.) with no path to `models.Config`. Operationally tunable parameters hardcoded in source *(pass 6, Coupling lens)*

#### cmd (`cmd/`) — 1,344 LOC

**Purpose:** Binary entry points.

**Observations:**
- `cmd/liza/main.go` (1,275 LOC, 5 functions): 1,100+ lines of inline cobra command `var` blocks + 111-line `init()` for flag registration. Business logic correctly delegates to `commands` package — complexity is organizational, not behavioral. *(pass 2, Complexity lens)*
- `cmd/liza-mcp/main.go` (69 LOC): thin stdio transport launcher. Cross-assigns version info via mutable package globals: `mcp.Version = embedded.Version` *(pass 3, Boundaries lens)*

#### mcp (`internal/mcp/`) — 1,460 LOC

**Purpose:** MCP JSON-RPC server exposing tools and resources to AI agents.

**Observations:**
- `server.go` (757 LOC): tool/resource registration, request dispatch. `registerMutationTools()` is 242 LOC of declarative tool schema definitions — LOC is mostly boilerplate, not algorithmic complexity. `GetTool()`, `GetHandler()`, `ToolNames()` accessors added for test introspection *(pass 2, Complexity lens; `642f94e`)*
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
- `WriteClaudeSettings()` and `WriteMCPSettings()` accept `io.Reader` parameter, defaulting to `os.Stdin` when nil
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

**Two consumers of `commands`**: CLI (`cmd/liza`) and MCP server (`mcp/handlers` — read-only queries only). MCP handlers call `ops` directly for all mutations; `commands` only used by MCP for read-only queries (status, inspect, validate) which already return structured data.

### 1.4 Coverage Checkpoint

**What exists that shouldn't?**
- `PlannerContextConfig` is an empty struct — premature abstraction or placeholder
- `commands/format.go` has bubble-sort for map keys (functional but O(n^2); `sort.Strings` exists)
- `dashboardSection` type with `"table"` format case is a no-op (line 155: just appends empty string)

**What's implicit that should be explicit?**
- The "Blackboard must remain stateless beyond cache" constraint (documented in architectural-issues.md)
- The contract between `commands` and its consumers — commands assume terminal I/O but serve three different transports *(pass 3, Boundaries lens)*

**What's missing from the walkthrough?**
- `db/metrics.go` (113 LOC): lock timing metrics — read and noted
- `commands/status.go` (469 LOC): status dashboard rendering — read via templates

**What requires cross-file comparison?**
- `derefString()` in prompts/builder.go duplicates `deref` template function
- Template execution pattern in `commands/templates.go` vs `prompts/templates.go` — nearly identical: embed.FS + funcMap with `deref` + template.Must + executeTemplate that panics *(pass 3, Boundaries lens)*
- **Ops Modify-callback boilerplate** *(pass 5, Duplication lens)*: `FindTask(taskID) + nil→NotFoundError` inside `bb.Modify` callbacks appears in 10 production files. The existing `readTaskState()` helper only works outside callbacks. The guard is identical in every file — see Smell below.
- **Ops input validation** *(pass 5, Duplication lens)*: `if taskID == "" { return nil, fmt.Errorf("task ID is required") }` in 10/19 ops files; `if agentID == ""` in 7/19. Each function validates its own required parameters independently.
- **Command test harness** *(pass 5, Duplication lens)*: 82 table-driven test loops across 34 command test files, 23 `initialState := &models.State{...}` constructions. The loop body (~15 lines: tmpDir → SetupLizaDir → create state → setup → WriteInitialState → call command → check error → validate state) is structurally identical.
- **Test setup sequence** *(pass 5, Duplication lens)*: 625 occurrences of `testhelpers.{SetupLizaDir|CreateValidState|WriteInitialState}` across 55 test files. The 3-4 line setup is per-test-function, not per-file.
- **Ops parameter convention split** *(pass 6, Coupling lens)*: `ClaimTask`, `MergeWorktree`, `DeleteTask`, `SubmitReview`, `Start`, `Stop`, `Pause`, `Resume`, `CreateWorktree`, `DeleteWorktree` take `projectRoot`; `AddTask` takes `statePath`/`logPath` directly — callers must track which convention each function uses
- **Identity validation duplicated** *(pass 6, Coupling lens)*: `agent/registration.go:validateIdentity()` reimplements `identity.ValidateFormat()` + `identity.ValidateRole()` without importing the package

**Coverage lens statement-level data** *(pass 4)*:

| File/Area | Statement Coverage | Notes |
|-----------|-------------------|-------|
| **Total** | **75.3%** | From `go tool cover -func` |
| `supervisor.Execute/ExecuteInteractive` | 0% | Core agent loop entry points |
| `supervisor.handleApprovedMerges` | 0% | Merge orchestration |
| `supervisor.resumeHandoffTask` | 11.4% | Complex handoff logic |
| `supervisor.RunSupervisor` | 54.4% | Main loop |
| `mcp/server.Run` | 0% | Server main loop |
| `mcp/protocol/*` | 0% | All error constructors + stdio |
| `embedded.WriteMCPSettings` | 0% | Stdin-coupled |
| `validate.validateAnomalies` | 13.3% | Only first branch tested |
| `validate.validateHandoff` | 33.3% | |
| `inspect_field.getSprintMetricsField` | 29.4% | |

**Files without any test file** *(pass 4, Coverage lens)*:
- `cmd/liza/main.go` (1,275 LOC) — CLI wiring
- `cmd/liza-mcp/main.go` (69 LOC) — MCP entry point
- `internal/mcp/protocol/errors.go` (68 LOC) — error constructors
- `internal/prompts/templates.go` (34 LOC) — template execution

**Complexity lens metrics** *(pass 2; pass 7: updated with ops files, nesting depth, function LOC)*:

| File | LOC | Longest Function (LOC) | Max Nesting Depth | Notes |
|------|-----|----------------------|-------------------|-------|
| main.go | 1,275 | init (111) | 2 | Organizational only — flat command registry |
| server.go | 715 | registerMutationTools (242) | 2 | Declarative schema definitions |
| state.go | 631 | — | 2 | 20+ cohesive structs |
| handlers.go | 603 | — | 3 | Thin handlers, 14% branch density |
| watch.go | 516 | — | 3 | 11 health checks, well-decomposed |
| validate.go (commands) | 28 | — | 1 | Thin wrapper over `statevalidate` *(was 448 LOC; `6fe5bcc`)* |
| validate.go (statevalidate) | 463 | validateTaskInvariants (142) | 3 | Sequential if-chain, no early-exit grouping |
| inspect_field.go | 327 | — | 3 | **9 switch statements** — manual reflection *(pass 7)* |
| **ops/claim_task.go** | **299** | **ClaimTask (265)** | **6** | **Highest complexity — 3-phase TOCTOU with duplicated dep check** *(pass 7)* |
| **ops/wt_merge.go** | **285** | **MergeWorktree (189)** | **4** | **Linear but many error-handling paths** *(pass 7)* |
| supervisor.go | 302 | RunSupervisor (186) | 5 | Main event loop *(pass 7: depth noted)* |
| claiming.go | 318 | resumeHandoffTask (63) | 5/6 | handleApprovedMerges: 55 LOC but depth 6 *(pass 7)* |

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
| `statevalidate` | db, models | 0 | 2 (ops, commands) |
| `ops` | 7 packages (db, models, git, log, paths, analysis, statevalidate) | 0 | 3 (agent, commands, mcp) |
| `commands` | **8 packages** (incl. statevalidate) | yaml.v3 | 2 (mcp queries, liza) |
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
| 9 | **Boundaries?** | Domain layer (`models`) clean. Persistence layer (`db`) clean. Transport layers (`mcp`, `cmd`) clean. Service layer (`ops`) clean — pure business logic, no I/O. All MCP-exposed mutations extracted to ops; commands are thin presentation wrappers; agent imports `ops`. |
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

#### ~~Smell: Hardcoded configuration — magic number 1800~~ *(mostly resolved — 2 residual sites, pass 6)*

**Signal:** `leaseDuration = 1800` appeared as a fallback default in 3 locations, plus 6 more magic numbers in `getRoleWaitConfig`.

**Fix:** Defined `DefaultLeaseDurationSeconds` and `Default{Coder,Planner,Reviewer}{PollInterval,MaxWait}` constants in `internal/models/state.go` alongside `Config`. ~~All 9 fallback sites reference named constants.~~ `heartbeat.DefaultLeaseDuration` derives from `models.DefaultLeaseDurationSeconds`.

**Residual** *(pass 6, Coupling lens)*: `supervisor.go:127` (`registerAgent(..., 1800)`) and `supervisor.go:221` (`claimReviewerTask(..., 1800, ...)`) still use raw `1800` instead of `models.DefaultLeaseDurationSeconds`. These were missed during the original extraction.

#### Smell: Non-injectable stdio in MCP transport *(partially resolved: `c2fe02b`)*

**Signal:** `NewStdioTransport()` hardwires `os.Stdin`/`os.Stdout`. Cannot inject readers/writers for testing.

**Partial fix:** Bounded request size enforcement (`MaxRequestSize` 10MB) added with `readLineBounded()` using `bufio.Reader.Peek`. Comprehensive tests (214 LOC in `stdio_test.go`) cover size limits, error responses, and normal operation — achieved without `io.Reader`/`io.Writer` injection by testing the bounded read logic directly.

**Remaining impact:** The transport constructor still hardwires `os.Stdin`/`os.Stdout`. Full I/O injection would enable testing `Run()` and the complete server loop.

**Direction:** Accept `io.Reader`/`io.Writer` parameters for full testability.

#### ~~Smell: Untested critical execution paths~~ *(pass 4, Coverage lens — partially resolved)*

**Signal:** The system's most critical runtime paths have 0% statement coverage:
- `supervisor.Execute()` and `ExecuteInteractive()` — the actual agent execution entry points that build `exec.Cmd`, set stdin/stdout, run the CLI, and handle exit codes
- `supervisor.handleApprovedMerges()` — orchestrates post-approval merge workflow
- `supervisor.logTaskSubmissionIfCompleted()` — completion logging
- `mcp/server.Run()` — the MCP server main loop (read request → dispatch → write response)
- All `mcp/protocol/` functions — error constructors and stdio transport

**Impact:** The tested code (helpers, validators, work detection) is exercised thoroughly, but the code that wires it all together at runtime has no direct tests. This creates a "tested parts, untested whole" pattern. If `Execute` mishandles an exit code, the supervisor loop misbehaves. The remaining untested paths are I/O-coupled functions requiring injection seams.

The root cause is I/O coupling: functions at 0% are precisely those with hardwired `os.Stdin`, `os.Stdout`, or `os/exec.Command`. The `CLIExecutor` interface in supervisor was the right move — but it was the only such seam created.

**Direction:** For `supervisor.Execute`/`ExecuteInteractive`: already abstracted behind `CLIExecutor` interface, which is mocked in `TestSupervisorBasicLoop`, but the `DefaultCLIExecutor` concrete implementation is untested. For `mcp/server.Run` and `protocol/stdio`: require I/O injection (see "Non-injectable stdio" smell).

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

#### Smell: Duplicated identity validation *(pass 6, Coupling lens)*

**Signal:** `agent/registration.go:validateIdentity()` (30 LOC) reimplements the same algorithm as `identity.ValidateFormat()` + `identity.ValidateRole()` without importing the `identity` package. Both: split on last hyphen, validate numeric suffix with `strconv.Atoi`, check role prefix match.

**Impact:** Low-medium. If identity format rules change (e.g., allowing non-numeric suffixes), two implementations need updating independently. The `identity` package is the canonical source but the `agent` package doesn't know it exists.

**Direction:** Replace `validateIdentity()` call with `identity.ValidateFormat()` + `identity.ValidateRole()`. The `identity` package already returns structured errors.

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

#### Smell: No interface-based seams beyond CLIExecutor *(pass 3, Boundaries lens)*

**Signal:** The entire production codebase has exactly **one interface**: `CLIExecutor` in `agent/supervisor.go`. All other cross-package dependencies use concrete types: `*db.Blackboard`, `*git.Git`, `*log.Logger`, `paths.LizaPaths`. There is one test-only interface (`testingT` in `testhelpers/assertions.go`).

**Impact:** This is a deliberate simplicity choice appropriate for v1 scope. However, it means testing any package that uses `db.Blackboard` requires real file I/O (creating temp directories, writing YAML files). The `testhelpers` package exists specifically to manage this overhead. If the system grows, introducing seams at the `db` and `git` boundaries would enable faster, more isolated tests.

**Direction:** No action for v1 — the current approach works. If test suite time becomes a concern, introduce interfaces at package boundaries (particularly `db` and `git`) to enable in-memory test doubles.

#### Smell: Pairing initialization doc pointer drift (`docs/USAGE.md`) *(Adversarial pass, entry: specs/)*

**Signal:** `contracts/PAIRING_MODE.md` Session Initialization still requires reading `docs/USAGE.md`, but that file does not exist. Canonical docs are now split (`docs/USAGE_PAIRING.md`, `docs/USAGE_MULTI_AGENTS.md`, indexed in `docs/README.md`).

**Impact:** High. The contract's startup procedure can fail on a missing file before work begins, producing avoidable initialization breaks and inconsistent behavior across agents.

**Direction:** Update Session Initialization to reference canonical docs (`docs/USAGE_PAIRING.md` minimum, or `docs/README.md` as resolver + mode-specific usage doc).

#### Smell: Sprint governance Vision link is broken *(Adversarial pass, entry: specs/)*

**Signal:** `specs/protocols/sprint-governance.md` links to `../vision.md`, while canonical Vision in this repo is `specs/build/0 - Vision.md` (as indexed in `specs/README.md`).

**Impact:** Low. Navigation drift weakens spec coherence and slows onboarding/review.

**Direction:** Update the related-doc link to the canonical Vision path.

#### Smell: Temporal test coupling and non-parallelizable suite *(Adversarial pass, entry: tests/ — partially resolved: `1914732`, `1ff88d2`)*

**Signal:** ~~Test suite currently has 93 `_test.go` files with 0 uses of `t.Parallel()` and 21 explicit `time.Sleep()` calls (notably in watcher/heartbeat paths).~~ Shared process-level globals exist (`db.instances` singleton map and package-level `rootCmd`), which encourages serial execution in packages that use them.

**Partial fix:** `resetRootCmdForTest(t)` helper isolates `rootCmd` flag state and `db.For()` singletons between tests. `t.Parallel()` introduced in 15 call sites across 4 test files (stateless tests in `roles`, `errors`, `filelock/metrics`, `agent/prompt`). `time.Sleep()` calls reduced from 21 to 5 by replacing brittle sleep-based waits with event-driven synchronization (polling with condition checks) in watcher, heartbeat, and supervisor tests. `internal/testguard/` package (116 LOC) added with ratchet tests enforcing `t.Parallel()` floor (≥10 calls) and `time.Sleep()` ceiling (≤11 calls), preventing regression.

**Remaining:** Tests sharing process-global `rootCmd` (all `cmd/liza` tests) cannot use `t.Parallel()` due to `os.Chdir` and cobra flag state. Further parallelization requires either a `--project-root` flag on rootCmd or test-level process isolation.

**Impact:** Low (downgraded from Medium). The ratchet tests and global-reset helpers address the structural concern; remaining serial tests are constrained by process-global state, not by missing infrastructure.

#### Smell: `LIZA_LOG_LEVEL` is documented but not implemented *(Adversarial pass, entry: config/)*

**Signal:** Docs define `LIZA_LOG_LEVEL`, but no runtime code path reads it; agent logger is initialized at fixed `slog.LevelInfo`.

**Impact:** Low-medium. Documented observability control is a no-op, causing confusion during debugging/operations.

**Direction:** Either implement environment-driven level selection (with validation and defaults) or remove this variable from docs to keep contract-to-runtime alignment.

#### Smell: Cleanup failures are suppressed in rebase/worktree recovery paths *(Adversarial pass, entry: error handling — partially resolved: `729da05`)*

**Signal:** Best-effort cleanup drops failures in multiple mutation flows.

**Partial fix:** `729da05` surfaced cleanup errors in `claim_task.go` — worktree/branch cleanup failures during claim failure recovery and worktree recreation recovery are now logged with context instead of silently dropped.

**Remaining:** `internal/ops/submit_review.go:88` ignores `AbortRebase` error. `internal/git/worktree.go` ignores cleanup errors in 3 locations. `internal/ops/wt_delete.go:61` ignores branch-deletion error.

**Direction:** Keep best-effort semantics where appropriate, but surface cleanup outcomes as warning/error in result structs and command output. Escalate to error when cleanup failure can leave the system in a materially inconsistent state.


#### Smell: File existence checks often collapse non-existence with other stat failures *(Adversarial pass, entry: error handling)*

**Signal:** Several paths only branch on `err == nil` or `os.IsNotExist(err)` (for example `internal/commands/setup.go:33`, `internal/commands/init.go:32`, `internal/commands/init.go:38`, `internal/git/worktree.go:162`) without explicit handling for permission/I/O errors.

**Impact:** Low-medium. Permission or transient filesystem errors can be misreported as simple presence/absence outcomes, producing misleading diagnostics and control flow.

**Direction:** Use explicit triage on `os.Stat` calls: exists, not-exist, and other-error (returned with context).


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

**Overall:** ~15,800 source LOC, ~41,700 test LOC. 2.6:1 ratio. **75.3% statement coverage** (from `go tool cover -func`; may have improved — recommend re-running). *(pass 4: statement-level data added; pass 7: LOC updated; statevalidate + schema tests)*

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
- `cmd/liza/main.go` (1,275 LOC): 717 LOC tests (0.6:1) — CLI wiring
- `internal/models/diagnostics.go` (127 LOC): zero tests, no test file — work detection functions used by supervisor *(pass 4)*
- `internal/mcp/protocol/errors.go` (68 LOC): zero tests, no test file *(pass 4)*

**Critical 0% coverage paths** *(pass 4, Coverage lens)*:
The 24.7% uncovered code concentrates in two patterns:
1. **Runtime orchestration** — `supervisor.Execute/ExecuteInteractive`, `mcp/server.HandleRequest/Run`, `mcp/server.classifyError` — the integration seams that wire tested components together
2. **I/O-coupled functions** — `embedded.WriteMCPSettings/mergeMCPSettings`, `mcp/protocol/stdio`, `DefaultCLIExecutor` — hardwired to OS-level I/O

**Partially covered functions of concern** *(pass 4; updated with subsequent coverage improvements)*:
| Function | Coverage | Status |
|----------|----------|--------|
| `validate.validateHandoff` | 33.3% | Handoff invariant checking — improved by `d8533ab` |

**I/O coupling as testability barrier** *(pass 4, Coverage lens)*: Functions at 0% coverage strongly correlate with hardwired I/O — this is the Coverage lens perspective on the Boundaries smell (pass 3). The `CLIExecutor` interface demonstrates the solution pattern: abstracting one I/O boundary enabled comprehensive supervisor testing. `StdioTransport` bounded read tests achieved without injection (`c2fe02b`), but full `Run()` loop testing still requires I/O injection.

**Integration tests:** 4 files in `internal/integration/` (1,397 LOC) covering concurrent operations, sprint/merge workflows, e2e command sequences, lease expiry. All files guarded by `testing.Short()` — skipped under `go test -short` *(resolved: `84b5a64`)*.

**Test patterns:** Table-driven (dominant), filesystem isolation, hand-written mocks (no frameworks), real git operations. No property-based or fuzz testing. *(pass 3: `os.Stdin` monkey-patching pattern noted as testing boundary smell — 8+ test files)*

**Temporal coupling signal** *(Adversarial pass, entry: tests/ — partially resolved: `1914732`, `1ff88d2`)*: Now 5 `time.Sleep()` calls and 15 `t.Parallel()` uses across 101 test files. `resetRootCmdForTest(t)` isolates process-global state. `internal/testguard/` ratchet tests enforce `t.Parallel()` floor (≥10) and `time.Sleep()` ceiling (≤11), preventing regression. Remaining serial tests are constrained by process-global state (`rootCmd`, `os.Chdir`), not by missing infrastructure.

**`check-testhelpers` build guard** *(pass 4)*: Makefile target prevents `testhelpers` import in production code — good practice for maintaining test/production boundary.

---

## Phase 3: Recommendations

| Priority | Issue | Rationale | Action |
|----------|-------|-----------|--------|
| **High** | Pairing initialization doc pointer drift (`docs/USAGE.md`) *(Adversarial pass, entry: specs/)* | Session Initialization in `PAIRING_MODE.md` requires a non-existent file; startup protocol can fail before task execution | Point initialization to canonical docs (`USAGE_PAIRING.md` and/or `docs/README.md`) |
| **Low** | Temporal test coupling *(Adversarial pass, entry: tests/ — partially resolved: `1914732`, `1ff88d2`)* | 5 `time.Sleep()` calls (down from 21), 15 `t.Parallel()` uses (up from 0), ratchet tests prevent regression; remaining serial tests constrained by process-global state | Continue tightening ratchets; `--project-root` flag would enable full parallelization |
| **Low** | Residual raw `1800` in supervisor.go *(pass 6)* | 2 call sites bypass `models.DefaultLeaseDurationSeconds` constant | Replace with named constant |
| **Low** | Duplicated identity validation *(pass 6)* | `agent/registration.go` reimplements `identity` package logic | Replace with `identity.ValidateFormat()` + `ValidateRole()` |
| **Low** | Inconsistent ops parameter conventions *(pass 6)* | `AddTask` takes `statePath`/`logPath` while 15+ others take `projectRoot` | Standardize on `projectRoot` |
| **Low** | `StdioTransport` not injectable *(partially addressed: `c2fe02b`)* | Bounded read tests achieved without injection; `Run()` still untestable | Accept `io.Reader`/`io.Writer` params for full loop testing |
| **Low** | `PlannerContextConfig` empty struct | Premature abstraction | Remove or document intent |
| **Low** | Duplicated template execution pattern *(pass 3)* | `commands/templates.go` and `prompts/templates.go` near-identical | Extract shared template infrastructure or accept as coincidental similarity |
| **Low** | `derefString` duplicated | In builder.go and templates.go funcMap | Use template func only |
| **Low** | `LIZA_LOG_LEVEL` documentation drift *(Adversarial pass, entry: config/)* | Env var documented but no runtime reader; logger is fixed at INFO | Implement env-driven log level or remove from docs |
| **Low** | `os.Stat` existence checks under-handle non-`IsNotExist` errors *(Adversarial pass, entry: error handling — partially resolved: `52ceac5`)* | Some presence checks classify only exists/missing and miss permission/I/O distinctions. `wt_merge.go` integration-test stat now handles tri-state correctly | Standardize tri-state handling in remaining sites |
| **Low** | `validateTaskInvariants` monolithic if-chain *(pass 7; now in `statevalidate/`)* | 142 LOC, ~15 checks ungrouped by status; hard to verify completeness | Group checks by status or use switch |
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

## Summary

Liza's architecture is well-suited to its constraints: a file-based multi-agent coordination system for solo developers. The dependency graph is clean with no cycles. Test coverage is excellent (2.4:1 ratio) with consistent patterns and strong helper infrastructure. The atomic state persistence via flock and fsync+rename is correctly implemented. Health monitoring is comprehensive. The task state machine is now explicit with a complete transition map.

**Pass 2 (Complexity lens)** identified monolithic command functions — `WtMergeCommand` and `ClaimTaskCommand` at 310-319 LOC each (since resolved via ops extraction). Task-lookup duplication (55+ inline loops) was also resolved via `State.FindTask()`.

**Pass 3 (Boundaries lens)** identified the `commands` package as the system's central boundary concern. Business logic was extracted to `internal/ops/` service layer; agent now imports `ops` instead of `commands`. All stdin reads now accept `io.Reader` parameter. The MCP adapter layer is clean (textbook adapter pattern), and the domain/persistence boundaries are well-drawn.

**Pass 4 (Coverage lens)** adds quantitative depth: 75.3% statement coverage overall, with the uncovered 24.7% concentrated in two patterns — runtime orchestration code (supervisor Execute) and I/O-coupled functions. I/O coupling is the primary driver of untested critical paths — functions with hardwired `os.Stdin`/`os.Stdout`/`os/exec` account for the majority of the 0% coverage. MCP server dispatch, `classifyError`, and `diagnostics.go` have since been resolved.

All six primary structural concerns identified across passes 1-4 have been resolved: supervisor decomposition, commands/ops extraction, monolithic functions, MCP locking, MCP dispatch testing, and agent→commands dependency. The ops layer now contains 25 operations (~3,750 LOC) serving 3 consumers (agent, commands, mcp).

**Pass 5 (Duplication lens)** examined cross-file repetition patterns. The most significant duplication pattern is within the `ops/` package itself: 10 of 19 ops files repeat an identical 3-line FindTask+NotFoundError guard inside `bb.Modify` callbacks, and 12 files share structurally similar history-append code. The `readTaskState()` helper addresses this for the Read path but has no equivalent for the Modify path. This is idiomatic Go — each function is independently authored with the same pattern — and the impact is low (maintenance burden if the guard pattern changes). In test code, 23 command test files construct near-identical `initialState` objects; a `testhelpers.DefaultState()` helper would be a low-risk improvement. Overall, the codebase's earlier duplication issues (task-lookup loops 55×, file-locking, magic numbers) have been resolved. The remaining repetition is largely structural — Go's explicit style trading conciseness for clarity.

**Pass 6 (Coupling lens)** focused on configuration hardcoding, tight dependencies, and hidden state sharing. Major items resolved: `"task/"` branch prefix centralized (`paths.TaskBranchPrefix`), role naming unified (`internal/roles` package), `GracePeriod` divergence unified (`models.LeaseExpiryGracePeriod`). Remaining open items: identity validation duplication, ops parameter convention split, watch threshold configurability, raw 1800 residuals in supervisor.go, and hardcoded `"terminal-1"`.

**Pass 7 (Complexity lens)** revisits complexity with the benefit of 6 prior passes of context. `ClaimTask` complexity and `inspect_field.go` manual reflection have been resolved. `ops/wt_merge.go:MergeWorktree` at 377 LOC (file total) remains a complex function with phased flow. LOC figures updated: production code is ~15,300 LOC (stable), test code grew to ~36,700 LOC (2.4:1 ratio).

**Adversarial pass (entry: docs/)** forced a doc-first path and surfaced contract-level drift missed by prior code-centric passes. All items resolved: state-machine spec drift, troubleshooting branch naming, and testing-doc short-mode drift.

**Adversarial pass (entry: specs/)** surfaced coherence gaps: (1) Pairing Session Initialization still references `docs/USAGE.md` even though docs were split into `USAGE_PAIRING.md` and `USAGE_MULTI_AGENTS.md`, and (2) sprint governance links Vision via `../vision.md` while canonical Vision lives in `specs/build/0 - Vision.md`. Watcher stall detection (resolved: `61b16d5`). The initialization-path drift remains high leverage.

**Adversarial pass (entry: tests/)** CLI contract coverage gap resolved (`9d95c1c` — `mutation_wiring_test.go`). Temporal coupling partially resolved: `time.Sleep` reduced from 21 to 5, `t.Parallel()` introduced (15 uses), ratchet tests prevent regression. Remaining serial tests constrained by process-global state.

**Adversarial pass (entry: config/)** exposed a config-contract gap cluster. Resolved: iteration limit enforcement, heartbeat interval wiring, config field projection. Remaining open config drift: `LIZA_LOG_LEVEL` remains unimplemented.

**Adversarial pass (entry: error handling)** surfaced a reliability-observability gap cluster. MCP parse-error and stale-lock cleanup errors resolved. Remaining: rebase/worktree cleanup flows in `submit_review.go`, `git/worktree.go`, and `wt_delete.go` still suppress secondary failures. Some `os.Stat` checks still under-handle non-`IsNotExist` filesystem errors.

**Adversarial pass (entry: data flow)** traced the task lifecycle. `DeleteTask` side-effect ordering resolved (`7dd05ce`). `submit-for-review` commit_sha semantics fixed (`d4c688e`) then regressed — needs re-verification.

**Adversarial pass (entry: documented smells)** — all four items resolved: REJECTED reassignment atomicity, planner max-wait enforcement, watch/log O(n) growth, and MCP stdio frame-size guard.

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
