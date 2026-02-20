# Architectural Issues

Persistent record of issues identified by architectural analysis skills.

**Skills that contribute here:**
- `systemic-thinking` — Systemic coherence and risk analysis
- `software-architecture-review` — Code-level architectural patterns and smells

## Table of Contents

- [Structural Load-Bearing Elements](#structural-load-bearing-elements)
  - [Planner as Single Semantic Interpreter](#planner-as-single-semantic-interpreter)
  - [Supervisor as Single Correctness Gate](#supervisor-as-single-correctness-gate)
- [Systemic Tensions](#systemic-tensions)
  - [Spec Completeness vs Reality](#spec-completeness-vs-reality)
  - [Documentation/Implementation Desynchronization](#documentationimplementation-desynchronization)
- [Feedback Loops](#feedback-loops)
  - [Hypothesis Exhaustion Without Root Cause](#hypothesis-exhaustion-without-root-cause)
  - [Restart/Lease Churn Under Load](#restartlease-churn-under-load)
  - [Supervisor Wait-Claim-Spawn Loop](#supervisor-wait-claim-spawn-loop)
- [Assumptions](#assumptions)
  - [Human Availability as Bottleneck](#human-availability-as-bottleneck)
  - [Spec Maturity Dependency](#spec-maturity-dependency)
  - [Well-Formed Blackboard State](#well-formed-blackboard-state)
  - [Multi-Instance Blackboard Coherence](#multi-instance-blackboard-coherence)
- [Stress Points](#stress-points)
  - [Supervisor Contention](#supervisor-contention)
  - [Filesystem/Git I/O Contention](#filesystemgit-io-contention)
- [Fragility](#fragility)
  - [Cross-Script State Mutation](#cross-script-state-mutation)
  - [YAML Round-Trip Data Loss](#yaml-round-trip-data-loss)
- [Trajectory](#trajectory)
  - [Blackboard Growth Without Pruning](#blackboard-growth-without-pruning)
  - [Anomaly Detail Validation Incomplete](#anomaly-detail-validation-incomplete)
  - [Task Type Registry is Partial Abstraction](#task-type-registry-is-partial-abstraction)
- [Accepted v1 Limitations](#accepted-v1-limitations)
  - [Self-Reported Validation](#self-reported-validation)
  - [Kill Switch Granularity](#kill-switch-granularity)
- [Completed Fixes](#completed-fixes)
- [Fix Details](#fix-details)
  - [Error Classification Lost at Agent Interface](#error-classification-lost-at-agent-interface)
  - [Implicit State Machine](#implicit-state-machine)
- [Code-Level Architectural Smells](#code-level-architectural-smells)
  - [Supervisor God File](#supervisor-god-file)
  - [Duplicated File-Locking Mechanism](#duplicated-file-locking-mechanism)
  - [MCP Handler Bypasses Blackboard Locking](#mcp-handler-bypasses-blackboard-locking)
  - [Magic Number 1800 Scattered](#magic-number-1800-scattered)
  - [executeTemplate Panics on Error](#executetemplate-panics-on-error)
  - [Inconsistent NotFoundError Usage](#inconsistent-notfounderror-usage)
  - [Commands Presentation+Logic Coupling](#commands-presentationlogic-coupling)
  - [Agent → Commands Upward Dependency](#agent--commands-upward-dependency)
  - [Interactive Stdin in Library Packages](#interactive-stdin-in-library-packages)
  - [~~Untested MCP Server Dispatch Layer~~](#untested-mcp-server-dispatch-layer)
  - [~~Untested Work Detection Logic~~](#untested-work-detection-logic)
---

## Structural Load-Bearing Elements

Single points of failure with no redundancy or validation mechanism.

### Planner as Single Semantic Interpreter

**Skill:** systemic-thinking
**Category:** LOAD-BEARING

**Issue:** Planner carries the entire semantic burden. It decomposes goals, interprets failure signals, resolves blocked reviews, converts discoveries to tasks, and maintains goal alignment. All other roles execute mechanical functions (implement spec, validate against spec) while Planner alone interprets intent. No second opinion, no validation mechanism, no structural redundancy.

**Implication:** Planner drift compounds silently across all tasks until human checkpoint reveals accumulated misalignment. Correction costs scale with drift duration.

**Current mitigation:** Human checkpoints provide periodic correction opportunities.

**Future options:**
- Planner self-review before finalizing task decomposition
- Second Planner instance for cross-validation on critical decisions
- Automated coherence checks against vision.md

### Supervisor as Single Correctness Gate

**Skill:** systemic-thinking
**Category:** LOAD-BEARING

**Issue:** System depends on supervisors (`liza agent`) performing correct pre-claim/assignment for all roles. This single gate defines whether tasks can proceed and whether agents stay within protocol. Correctness is concentrated in a single control loop that is neither redundant nor independently validated.

**Implication:** A supervisor bug, crash loop, or misconfiguration stalls the entire system and blocks recovery without manual intervention.

**Current mitigation:** Supervisor is implemented in the `liza` Go binary with type-safe error handling. `liza validate` catches invalid states.

**Future options:**
- Supervisor health check endpoint
- Redundant supervisor with leader election
- Agent self-validation of claim state on startup

---

## Systemic Tensions

Design contradictions that create structural friction.

### Spec Completeness vs Reality

**Skill:** systemic-thinking
**Category:** TENSION

**Issue:** The vision positions specs as the mechanism for context survival ("if it's not written down, it doesn't exist") while stating "Liza v1 assumes specs substantially complete before work" and excluding "domains where requirements emerge through implementation."

Incomplete specs—normal in real projects—trigger a reinforcing loop: coders block on spec gaps, Planner logs spec_gap anomalies, human must update specs, system pauses. The spec-first design shifts work from agents to humans while promising to reduce human workload.

**Implication:** System selects for a narrow project profile (complete specs, solo developers) rather than adapting to common project conditions.

**Current mitigation:** BLOCKED resolution via `human_notes`. Planner reads human_notes on wake.

**Future options:**
- Spike mode for spec discovery
- Planner-assisted spec drafting from coder discoveries
- Graceful degradation when specs incomplete (proceed with explicit assumptions)

### Documentation/Implementation Desynchronization

**Skill:** systemic-thinking
**Category:** TENSION

**Issue:** The Go CLI migration (ADR-0012) replaced the entire operational layer (18 bash scripts → Go CLI) but only partially updated the documents that agents and humans read as operational truth. Remaining drift:
- `DEMO.md` contains 9 `yq` commands as monitoring instructions — the Go CLI eliminated the `yq` dependency and provides `liza get`/`liza status` equivalents (documented in `RECIPES.md`)
- Additional `yq` references persist in `TROUBLESHOOTING.md`, `USAGE_MULTI_AGENTS.md`, `sprint-governance.md`, `circuit-breaker.md`, `worktree-management.md`, `roles.md`

Previously fixed:
- ~~`liza handoff` missing~~ — CLI command + MCP tool implemented
- ~~`code_reviewer` underscore~~ — fixed to `code-reviewer`
- ~~Signal file references~~ — replaced with state field mutations

**Implication:** For a system whose core value proposition is agents reading specs as source of truth, stale `yq` references cause confusion — users and agents encounter commands that require an eliminated dependency.

**Current mitigation:** `RECIPES.md` documents all `liza get` equivalents for former `yq` queries.

**Future options:**
- Complete documentation sweep replacing `yq` commands with `liza get`/`liza status` equivalents
- Automated doc-code consistency check (grep for removed patterns)

---

## Feedback Loops

Self-reinforcing patterns that can amplify failures.

### Hypothesis Exhaustion Without Root Cause

**Skill:** systemic-thinking
**Category:** FEEDBACK

**Issue:** Hypothesis exhaustion rule (two coders fail = must rescope) forces Planner intervention but doesn't require root cause identification. Planner may split task-3 into task-3a/task-3b without diagnosing why two coders failed. If underlying cause is spec ambiguity or architecture flaw, new tasks encounter same obstacle.

Circuit breaker theoretically catches this via spec_gap_cluster, but pattern detection uses exact string matching—different coders may describe same issue differently.

**Implication:** System may cycle through rescope iterations without converging, consuming time and compute on task churn rather than progress.

**Future options:**
- Similarity matching for anomaly clustering (semantic, not exact)
- Escalate to human after N rescopes of same original task

### Restart/Lease Churn Under Load

**Skill:** systemic-thinking
**Category:** FEEDBACK

**Issue:** Protocol restarts agents on exit 42 and uses leases/heartbeats for coordination. Under load or long-running operations, lease pressure and restart frequency can amplify each other. The restart loop is assumed stabilizing but can become self-sustaining when work exceeds lease windows.

**Implication:** Under stress, system enters churn state—progress stops but resource usage and log noise increase.

**Current mitigation:** Grace periods on lease checks. Context self-diagnosis triggers graceful abort.

**Future options:**
- Adaptive lease duration based on task complexity
- Supervisor watchdog with timeout detection
- Exponential backoff on repeated restarts

### Supervisor Wait-Claim-Spawn Loop

**Skill:** systemic-thinking
**Category:** FEEDBACK

**Issue:** Supervisor's "wait → claim → spawn → restart" loop is tightly coupled with lease timing and work availability. Under slow tasks or transient failures, the loop can become self-reinforcing, cycling agents without progressing state.

**Implication:** System can be active but not advancing, with increasing log noise and human overhead.

**Future options:**
- Supervisor state machine with explicit "stalled" detection
- Alert on N cycles without state change
- Automatic pause after repeated no-progress cycles

---

## Assumptions

Implicit dependencies that constrain system behavior.

### Human Availability as Bottleneck

**Skill:** systemic-thinking
**Category:** ASSUMPTION

**Issue:** Human is circuit breaker, escalation point, spec author, checkpoint reviewer, and resolution authority for deadlocks. Sprint governance states agents pause indefinitely awaiting human action. The "solo developers, small teams" deployment context is load-bearing, not merely scope-limiting.

If human attention becomes bottleneck (competing priorities, vacation, scaling), system has no degradation path. All escalation paths terminate at same person with no delegation.

**Implication:** Human availability constrains throughput more than agent capacity, inverting goal of reducing human bandwidth as bottleneck.

**Future options:**
- Timeout with automatic abort after N hours without human response
- Delegation mechanism for escalation routing
- Async human review queue with SLA tracking

### Spec Maturity Dependency

**Skill:** systemic-thinking
**Category:** ASSUMPTION

**Issue:** "Specs substantially complete before work" ties throughput to spec maturity and creates dependency on continuous human availability for spec evolution.

**Implication:** When specs incomplete or human constrained, throughput collapses rather than degrading gracefully.

**Addressed by:** BLOCKED resolution via `human_notes`.

### Well-Formed Blackboard State

**Skill:** systemic-thinking
**Category:** ASSUMPTION

**Issue:** CLI commands assume blackboard fields (current_task, review_lease_expires, integration_branch) are present and well-formed. Limited defensive handling for corrupted or partial state.

**Implication:** Single malformed entry can cascade into systemic stop conditions across all roles.

**Current mitigation:** `liza validate` checks invariants.

**Future options:**
- Schema validation on every state read
- Auto-repair for common corruption patterns
- Quarantine malformed entries rather than fail-stop

### Multi-Instance Blackboard Coherence

**Skill:** systemic-thinking
**Category:** ASSUMPTION

**Issue:** The supervisor (`internal/agent/supervisor.go`) creates multiple independent `Blackboard` instances during its lifecycle: one persistent instance for the main loop, plus fresh instances in `checkAbort()` and `waitWhilePaused()` on every call. Additionally, every `commands.*Command()` function creates its own `Blackboard` internally. Each instance has independent cache state (mtime-based) and independent lock timeout. This works today because the filesystem is the sole source of truth and mtime changes propagate correctly. But it creates an invisible constraint: `Blackboard` must remain stateless beyond its cache. If any future change adds in-process state (metrics aggregation, write batching, connection pooling, subscriptions), that state fragments silently across instances with no error signal.

**Implication:** The proliferation of `Blackboard` instances is an invisible architectural constraint that will be discovered through subtle bugs rather than compilation errors if the `Blackboard`'s responsibilities grow.

**Future options:**
- Singleton `Blackboard` per process, passed via dependency injection
- Document the "stateless beyond cache" constraint in the `db` package
- Add a linter check for multiple `db.New()` calls in the same package

---

## Stress Points

Bottlenecks that emerge under load.

### Supervisor Contention

**Skill:** systemic-thinking
**Category:** STRESS POINT

**Issue:** Supervisor-only worktree creation and claim handling centralize concurrency control and state transitions. All contention and race resolution concentrated in single process. Coders and Reviewers fully dependent on its throughput and correctness.

**Implication:** Supervisor contention becomes primary bottleneck when scaling beyond small task counts.

**Future options:**
- Partition by task ID for parallel claim handling
- Optimistic claiming with conflict resolution
- Dedicated claim coordinator separate from agent supervisor

### Filesystem/Git I/O Contention

**Skill:** systemic-thinking
**Category:** STRESS POINT

**Issue:** Worktree creation, review assignment, and merge operations funnel through filesystem and git in same repo. Primary shared resource for all roles. The flock-based locking protects `state.yaml` consistency but does NOT serialize git operations — two agents can concurrently run `git worktree add` and `git merge` against the same repo. In practice this works because worktree operations are scoped to different directories and merge operations are single-threaded per branch, but no formal exclusion mechanism prevents concurrent git state corruption.

**Implication:** I/O contention or git state anomalies become first systemic bottleneck as task volume increases. Concurrent merges to the integration branch are the highest-risk scenario.

**Current mitigation:** Supervisor serializes merge operations per-agent (only one merge per supervisor loop iteration). Different agents operate in separate worktrees.

**Future options:**
- Worktree pool pre-creation
- Git operations queuing (serialization mutex for integration branch merges)
- Separate integration repo for merges

---

## Fragility

Partial failure modes with unclear recovery.

### Cross-Script State Mutation

**Skill:** systemic-thinking
**Category:** FRAGILITY

**Issue:** State mutation distributed across CLI commands (`liza claim-task`, `liza wt-merge`, `liza clear-stale-review-claims`) with shared transactional boundary via the Go binary's internal locking. Cross-command assumptions about state shape are type-checked at compile time.

**Implication:** Partial failure in any command can leave blackboard logically consistent but operationally stuck.

**Future options:**
- State machine validation after each operation
- Transaction log for rollback capability
- Centralized state mutation through single entry point

### YAML Round-Trip Data Loss

**Skill:** systemic-thinking
**Category:** FRAGILITY

**Issue:** The Go struct for `state.yaml` (`internal/models/state.go`) uses standard `yaml.Unmarshal`/`yaml.Marshal` with no unknown-field preservation. Any field present in `state.yaml` but absent from the Go struct is silently dropped on the next write. In the bash era, `yq` mutations preserved all fields. The Go CLI destroys them. This means:
- Manual human additions to `state.yaml` survive one read but are erased on next write
- Schema evolution requires struct changes before any new field can survive
- There is no runtime warning when data is discarded

**Implication:** The blackboard becomes a "closed world" — only what the binary knows about survives. Any out-of-band state enrichment (human notes added via editor, future tool extensions, experimental fields) is silently erased on next CLI write.

**Future options:**
- Use a YAML library that preserves unknown fields (e.g., round-trip aware parser)
- Store auxiliary/extension fields in a separate section with pass-through semantics
- Warn on fields present in file but absent from struct

---

## Trajectory

Long-term concerns about system evolution.

### Blackboard Growth Without Pruning

**Skill:** systemic-thinking
**Category:** TRAJECTORY

**Issue:** System optimizes for accountability via append-only logs, explicit states, and anomaly logging. No clear pruning or partition strategy in v1.

**Implication:** As task volume grows, coordination cost and cognitive load rise nonlinearly. System becomes harder to operate without additional tooling.

**Future options:**
- Archive completed sprints to separate files
- Prune history older than N days
- Split blackboard by concern (tasks, agents, anomalies)

### Anomaly Detail Validation Incomplete

**Skill:** code-review
**Category:** FRAGILITY

**Issue:** `liza validate` enforces required detail fields for 5 of 15 anomaly types (`retry_loop`, `trade_off`, `external_blocker`, `assumption_violated`, `system_ambiguity`). The remaining 10 types — including `reviewer_loop` (requires `count`, `command_pattern`) and `review_exhaustion` (requires `reviewers_failed`, `common_blocker`) — pass validation with empty details. The spec (`blackboard-schema.md:770`) and prompt templates (`shared_reference.tmpl`) both declare required fields for all types.

**Implication:** Agents can write structurally valid but informationally empty anomalies. Circuit breaker pattern detection and retrospective analysis degrade when detail fields are missing.

**Future options:**
- Add cases for all 10 missing types in `validateAnomalies()` (`internal/commands/validate.go:360`)
- Generate validation from a single type→fields declaration (eliminate spec/code/template as three separate lists)

### Task Type Registry is Partial Abstraction

**Skill:** code-review
**Category:** TRAJECTORY

**Issue:** The task type workflow registry (`taskWorkflows` in `internal/models/state.go`) maps `TaskType` → ordered role sequence, but the mapping of role → claimable statuses is hardcoded in `IsClaimable`'s switch statement. The registry captures *which* roles participate but not *how* they participate (i.e., which statuses each role can claim from). Adding a new role requires modifying the switch, which undermines the "new types add rows to the registry" premise.

**Implication:** When a second task type arrives (e.g., `specification` with a `spec_reviewer` role), the claimable-status mapping will need resolving — either by extending the registry to include status rules, or by accepting the switch as the canonical location for claiming semantics.

**Current mitigation:** TODO comment on the switch in `IsClaimable`. Only one task type exists today, so there's no split in practice.

**Future options:**
- Extend registry to map `(TaskType, role)` → `[]TaskStatus` (claimable statuses)
- Keep the switch but validate it against registry entries at init time
- Accept the split as intentional separation of concerns (registry = participation, switch = claiming rules)

---

## Accepted v1 Limitations

### Self-Reported Validation

**Skill:** systemic-thinking

**Issue:** Coder runs validation and reports result. Code Reviewer trusts claim without re-execution.

**Why accept:** Re-execution requires Code Reviewer to run in different worktree, understand commands, handle environment differences.

**Mitigation:** Code Reviewer can request re-run if suspicious.

### Kill Switch Granularity

**Skill:** systemic-thinking

**Issue:** Kill switches (PAUSE/ABORT) affect all agents. Can't surgically stop one misbehaving agent.

**Why accept:** Per-task kills add complexity. Rare failure mode.

**Future option:** Per-task pause via `liza pause --task task-{id}`.

---

## Completed Fixes

- [x] Merge conflict resolution *(systemic-thinking)*
- [x] Anomaly log reader *(systemic-thinking)*
- [x] Human role clarification *(systemic-thinking)*
- [x] Task dependencies *(systemic-thinking)*
- [x] Supervisor clarification *(systemic-thinking)*
- [x] Review lease validation *(systemic-thinking)*
- [x] Multi-state claiming *(systemic-thinking)*
- [x] Approval rate monitoring *(systemic-thinking)*
- [x] Root cause required before rescope *(systemic-thinking)*
- [x] flock inode race — stop deleting lock/PID files after unlock *(code-review)*
- [x] ReadCached shared mutable pointer — cache raw bytes, return fresh structs *(code-review)*
- [x] Watcher AfterFunc panic — check closed flag under mutex before channel send *(code-review)*
- [x] wt_merge TOCTOU — re-validate task status under lock in all 4 Modify callbacks *(code-review)*
- [x] Merge retry cap — 3 retries with linear backoff, then proceed to waitForWork *(code-review)*
- [x] Reviewer tight loop — 5s sleep on claim failure *(code-review)*
- [x] Branch cleanup gating — only delete branch/worktree when created in this invocation *(code-review)*
- [x] Worktree prune — `git worktree prune` after manual removal fallback *(code-review)*
- [x] Path traversal via taskID — `ValidateTaskID()` rejects `/`, `\`, `..`, leading `.` *(code-review)*
- [x] os.Getwd() → paths.GetProjectRoot() — worktree-aware project root via `git rev-parse` *(code-review)*
- [x] wt_merge ordering — commit state to MERGED before worktree deletion *(code-review)*
- [x] Agent status staleness — update agent state in submit-review, submit-verdict, delete-task *(code-review)*
- [x] classifyError stub — pattern-based mapping to JSON-RPC error codes *(code-review)*
- [x] Error classification lost at agent interface — `classifyError()` implemented with 5 error categories *(systemic-thinking)*
- [x] JSON-RPC notifications — detect `id: null` requests, handle without reply *(code-review)*
- [x] Hypothesis exhaustion false positive — exclude terminal tasks from FailedBy check *(code-review)*
- [x] Concurrent git contention — documented limitation in architectural-issues *(code-review)*
- [x] Embedded assets clean-checkout — documented `make sync-embedded` requirement in Makefile + REPOSITORY.md *(code-review)*
- [x] `code_reviewer` → `code-reviewer` — fixed in agent-runtime-reference.md *(code-review)*
- [x] `git init -b main` — fixed bare `git init` in get_test.go to avoid `master` default *(code-review)*
- [x] cleanupStaleLock inode race — truncate lock file instead of deleting it *(code-review)*
- [x] classifyError "invalid" overbroad — narrowed to `invalid task ID`, sanitized all error messages *(code-review)*
- [x] mergeCommit[:7] unguarded in rollback path — added length check *(code-review)*
- [x] Implicit state machine — declared `taskTransitions` map + `Transition()` method, migrated all 14 transition sites *(systemic-thinking)*
- [x] Untested MCP server dispatch layer — `server_dispatch_test.go` covers `HandleRequest` routing, `classifyError` all 5 branches, `handleToolCall`, `handleResourceRead`, `handleNotification` *(software-architecture-review)*
- [x] Untested work detection logic — `diagnostics_test.go` covers all 4 functions (`CountClaimableTasks`, `CountReviewableTasks`, `GetCoderWorkDiagnostics`, `GetReviewerWorkDiagnostics`) *(software-architecture-review)*
- [x] MCP handler bypasses Blackboard locking — `readStateResource()` now uses `Blackboard.ReadRaw()` under flock instead of direct `os.ReadFile` *(software-architecture-review)*

---

## Fix Details

### Merge Conflict Resolution

**Skill:** systemic-thinking

**Original issue:** No guidance on how Code Reviewer should handle merge conflicts. Unclear whether to resolve, escalate, or fail the review.

**Fix:** Code Reviewer MAY resolve trivial conflicts (whitespace, import order, non-overlapping additions). Logic conflicts requiring judgment MUST be escalated to human.

### Anomaly Log Reader

**Skill:** systemic-thinking

**Original issue:** Circuit breaker patterns (retry_cluster, spec_gap_cluster, hypothesis_exhaustion) were logged but Planner had no mechanism to read them, making escalation triggers invisible.

**Fix:** Planner reads `.liza/anomalies.log` on wake to detect patterns and take corrective action.

### Human Role Clarification

**Skill:** systemic-thinking

**Original issue:** Human role was ambiguous—sometimes described as observer, sometimes as decision-maker. Unclear who resolves deadlocks.

**Fix:** Human is escalation point with decision authority, not passive observer. All deadlocks and ambiguities route to human for resolution.

### Task Dependencies

**Skill:** systemic-thinking

**Original issue:** No mechanism to express or enforce task ordering. Coders could claim tasks whose prerequisites weren't complete.

**Fix:** Added `depends_on` field to task schema. `liza claim-task` validates all dependencies are MERGED before allowing claim. Planner instructions updated to specify dependencies when decomposing tasks.

### Supervisor Clarification

**Skill:** systemic-thinking

**Original issue:** "Supervisor" was ambiguous—could be interpreted as singleton process managing all agents, leading to incorrect architectural assumptions.

**Fix:** Clarified that "supervisor" refers to the enclosing loop in each `liza agent` instance, not a singleton process. Each role runs in its own terminal with its own supervisor loop.

### Review Lease Validation

**Skill:** systemic-thinking

**Original issue:** `find_reviewable_task()` treated missing `review_lease_expires` as expired, allowing tasks with `reviewing_by` set but no lease timestamp to be claimed by another reviewer.

**Fix:** Now requires BOTH `reviewing_by` AND `review_lease_expires` to be set before treating a lease as stale. Missing `review_lease_expires` with `reviewing_by` set is treated as actively claimed (not reviewable).

### Multi-State Claiming

**Skill:** systemic-thinking

**Original issue:** `liza claim-task` only handled READY tasks. REJECTED and INTEGRATION_FAILED tasks couldn't be re-claimed, and worktree handling for reassignment was undefined.

**Fix:** Supports claiming from READY, REJECTED, and INTEGRATION_FAILED states:
- READY: creates fresh worktree
- REJECTED (same coder): preserves worktree and base_commit for drift accuracy
- REJECTED (different coder): deletes worktree, creates fresh, resets review_cycles_current
- INTEGRATION_FAILED: preserves worktree for conflict resolution, sets integration_fix flag

### Approval Rate Monitoring

**Skill:** systemic-thinking
**Category:** BLIND SPOT

**Original issue:** Vision identifies "Code Reviewer rubber-stamps coder work" as medium-likelihood, high-impact risk with mitigation "rejection quota monitoring, anomaly patterns." However, circuit breaker patterns detect failure signals (retry_loop, spec_gap) but not success signals that should trigger suspicion.

A Code Reviewer approving everything generates zero anomalies—no retry_cluster, no hypothesis_exhaustion, no review_deadlock. All metrics appear healthy. The system cannot distinguish validation from rubber-stamping.

**Implication:** Core promise of external validation becomes invisible when violated. System health metrics are undefined in presence of colluding or lazy Code Reviewer.

**Fix:** `liza update-sprint-metrics` computes two metrics from task history:
- `review_verdict_approval_rate_percent` = approvals / (approvals + rejections) * 100
- `task_outcome_approval_rate_percent` = approvals / submitted_for_review * 100

Warns if review_verdict_approval_rate >95% over ≥5 review verdicts. Metrics stored in `sprint.metrics`.

**Future options:**
- Random re-review by second Code Reviewer
- Human spot-checks of merged PRs
- Require rejection quota per sprint

### Root Cause Required Before Rescope

**Skill:** systemic-thinking

**Original issue:** Hypothesis exhaustion forced rescope without diagnosing cause, leading to task churn.

**Fix:** Planner must document root cause before rescoping and include it in `rescope_reason` and the rescope log entry (task lifecycle + roles).

### Error Classification Lost at Agent Interface

**Skill:** systemic-thinking
**Category:** BLIND SPOT

**Original issue:** The `db` package introduced a well-designed error taxonomy (`LockError` with 5 classified categories: Timeout, Permission, DiskFull, Filesystem, Stale), but the MCP server's `classifyError()` was a TODO stub returning generic internal error for everything. Agents couldn't distinguish retryable errors from fatal ones.

**Fix:** `classifyError()` in `internal/mcp/server.go` implements pattern-based mapping to distinct JSON-RPC error codes: not found, lock timeout, race condition, validation error, and internal error. Follow-up fix narrowed overbroad "invalid" matching to `invalid task ID` specifically.

### Implicit State Machine

**Skill:** systemic-thinking
**Category:** LOAD-BEARING

**Original issue:** Task state transitions were not enforced by a declared state machine. Each command independently checked its own preconditions, making the valid transition graph emergent from scattered conditional checks across 7 command files and `supervisor.go`. Adding or modifying a command could silently create invalid transition paths.

**Fix:** Declared the complete transition graph as `taskTransitions` map in `internal/models/state.go` with `CanTransition()` and `Transition()` methods. All 14 production transition sites migrated from direct `task.Status = X` to `task.Transition(X)`, which validates against the declared graph and returns a descriptive error on invalid transitions. `IsClaimable()` rewritten to derive claimable statuses from `CanTransition()` instead of a hardcoded switch. Existing precondition checks in commands preserved as defense-in-depth.

---

## Code-Level Architectural Smells

Issues identified through code-level architectural analysis (patterns, structure, duplication).

### Supervisor God File

**Skill:** software-architecture-review
**Category:** God class/module

**Issue:** `internal/agent/supervisor.go` is 1,426 LOC with 5+ responsibilities: loop orchestration, prompt building, lease management, checkpoint handling, and agent spawning. It is the most frequently changed file and the hardest to test in isolation.

**Implication:** High change frequency combined with broad responsibility means most agent-layer changes risk unrelated regressions. Testing requires setting up the full supervisor context even for narrow behaviors.

**Direction:** Extract cohesive pieces — lease management, prompt assembly, and agent spawning are candidates for standalone packages or files within `internal/agent/`.

### Duplicated File-Locking Mechanism

**Skill:** software-architecture-review
**Category:** DRY violation / Shotgun surgery

**Issue:** `internal/db/blackboard.go` and `internal/log/logger.go` independently implement identical file-locking with `gofrs/flock`: same constants (`DefaultLockTimeout=10s`, `LockCheckInterval=100ms`), same polling loop pattern (`withLock` / `withLockOperation`). The db version has additional stale-lock recovery and metrics; the log version does not.

**Implication:** If the polling interval, timeout, or recovery logic is patched in one package, the other silently diverges. The log package also lacks the stale-lock recovery that db has, creating inconsistent failure modes under the same stress conditions.

**Direction:** Extract to a shared `internal/filelock` utility using the db package's enriched version as the basis.

### ~~MCP Handler Bypasses Blackboard Locking~~

**Skill:** software-architecture-review
**Category:** Leaky abstraction / Boundary violation
**Status:** RESOLVED — `readStateResource()` now uses `Blackboard.ReadRaw()` under flock

### Magic Number 1800 Scattered

**Skill:** software-architecture-review
**Category:** Hardcoded configuration

**Issue:** The default lease duration of 1800 seconds appears as an unnamed magic number in `internal/agent/supervisor.go` (lines 469, 860) and `internal/commands/claim_task.go` (line 104). All three use the same fallback pattern: `if leaseDuration <= 0 { leaseDuration = 1800 }`. Additionally, `supervisor.go` has 6 more magic numbers in `getRoleWaitConfig` for poll/wait defaults (30, 60, 1800).

**Implication:** No single place to change the default lease duration. `internal/agent/heartbeat.go` already defines `DefaultLeaseDuration = 30 * time.Minute` as a named constant, but the three fallback sites use raw `1800` instead.

**Direction:** Replace magic numbers with the existing `DefaultLeaseDuration` constant (converting to seconds where needed), or define `DefaultLeaseDurationSeconds` alongside it.

### executeTemplate Panics on Error

**Skill:** software-architecture-review
**Category:** Leaky abstraction / Non-idempotent operations

**Issue:** `internal/prompts/templates.go` calls `panic("template: " + err.Error())` when template execution fails, instead of returning an error. In a long-running supervisor process, this crashes the entire binary on malformed template data.

**Implication:** Template execution errors (missing fields, type mismatches) are unrecoverable. The supervisor cannot gracefully handle or retry — the process dies and must be restarted externally.

**Direction:** Change `executeTemplate` to return `(string, error)`. Callers already handle errors from prompt building; propagation is straightforward.

### Inconsistent NotFoundError Usage

**Skill:** software-architecture-review
**Category:** Primitive obsession / Unstable interface

**Issue:** `internal/errors` defines a structured `NotFoundError` with `IsNotFound()` check, but most "not found" scenarios use ad-hoc `fmt.Errorf("task not found: %s", taskID)` strings. The structured type is only used by inspect commands (`inspect_tasks.go`, `inspect_agents.go`, `inspect_field.go`); other commands and the db package use string-based errors for the same semantic meaning.

**Implication:** Callers cannot reliably distinguish "not found" from other errors programmatically. The MCP `classifyError()` already does pattern matching on error strings — fragile and will break if wording changes.

**Direction:** Adopt `NotFoundError` consistently across `db/blackboard.go` and command files. The MCP `classifyError()` can then use `errors.As` instead of string matching.

### Commands Presentation+Logic Coupling

**Skill:** software-architecture-review
**Category:** Leaky abstraction / Inappropriate intimacy

**Issue:** The `commands` package serves three consumers with incompatible I/O expectations — CLI (terminal), MCP server (JSON-RPC over stdio), and supervisor (background process) — but embeds terminal assumptions: 40+ `fmt.Print*` calls to stdout/stderr and 5+ direct `os.Stdin` reads in non-test production code. Functions like `ClaimTaskCommand()` print success messages, `SetupCommand()` prompts for confirmation, and `DeleteTaskCommand()` reads interactive input.

**Implication:** MCP server calls commands via `handlers.go` that print to stdout, which is the JSON-RPC transport channel — stdout writes from commands could corrupt the protocol stream. Supervisor calls (`commands.ClaimTaskCommand()`, `commands.WtMergeCommand()`) mix operational output with supervisor logs. Tests must monkey-patch `os.Stdin` (8+ test files use `os.Stdin = r` / `defer func() { os.Stdin = oldStdin }()`) — fragile and not concurrency-safe.

**Direction:** Separate business logic from presentation. Command functions return structured results; callers handle output. The MCP adapter already does this partially — `StatusCommand()` returns a string. Extend pattern to mutation commands. This also resolves the agent→commands coupling (see below).

### Agent → Commands Upward Dependency

**Skill:** software-architecture-review
**Category:** Leaky abstraction

**Issue:** The supervisor (`internal/agent/supervisor.go`) directly calls three `commands` package functions: `ClaimTaskCommand()` (line 926), `WtMergeCommand()` (line 1250), `ClearStaleReviewClaimsCommand()` (line 383). It also imports `commands.IntegrationFailedError` for error type checking. The `commands` package doc.go states "Each command corresponds to a subcommand in the liza CLI" — the supervisor is an unintended consumer of CLI-specific handlers.

**Implication:** The orchestration layer (agent) depends on the CLI handler layer (commands), creating a conceptual inversion. The supervisor inherits presentation side effects — when it calls `ClaimTaskCommand()`, it gets terminal output it doesn't want. This coupling means changes to command presentation (e.g., adding progress indicators) leak into the supervisor.

**Direction:** Extract business logic from `ClaimTaskCommand`, `WtMergeCommand`, and `ClearStaleReviewClaimsCommand` into service functions that return structured results (no I/O side effects). Both `commands` and `agent` call these functions. Synergistic with monolithic command decomposition — the "phases" extracted from commands become the shared service layer.

### Interactive Stdin in Library Packages

**Skill:** software-architecture-review
**Category:** Untestable by design

**Issue:** Direct `os.Stdin` reads via `bufio.NewReader(os.Stdin)` or `bufio.NewScanner(os.Stdin)` in: `embedded/embedded.go` (2 locations: `WriteClaudeSettings`, `WriteMCPSettings`), `commands/setup.go` (2 locations), `commands/init.go` (1), `commands/delete_task.go` (2), `commands/delete_agent.go` (1). Total: 8 locations across 5 files in 2 packages.

**Implication:** Functions with hardwired stdin cannot be used non-interactively (e.g., from MCP server or automated scripts). Tests work around this by replacing `os.Stdin` with pipe readers (observed in 8+ test files) — this pattern is fragile and not safe for concurrent test execution.

**Direction:** Accept an `io.Reader` parameter or a `Confirmer` callback for interactive prompts. Default to `os.Stdin` at the CLI call site in `cmd/liza/main.go`. This makes the same functions usable from non-interactive contexts (MCP, automation, tests).

### ~~Untested MCP Server Dispatch Layer~~

**Skill:** software-architecture-review
**Category:** Untested critical path
**Status:** RESOLVED — `internal/mcp/server_dispatch_test.go` added

**Fix:** Added `server_dispatch_test.go` with table-driven tests covering: `HandleRequest` routing (all 4 method branches + unknown method), `handleToolCall` (invalid params, missing name, unknown tool, successful handler, nil arguments, handler error with classification), `handleResourceRead` (invalid params), `classifyError` (all 5 classification branches: not found, lock timeout, race condition, validation, internal — 14 test cases), leak prevention (raw error strings never exposed), `handleNotification` (known and unknown). Request ID preservation verified.

### ~~Untested Work Detection Logic~~

**Skill:** software-architecture-review
**Category:** Untested critical path
**Status:** RESOLVED — `internal/models/diagnostics_test.go` added

**Fix:** Added `diagnostics_test.go` with table-driven tests covering all 4 functions: `CountClaimableTasks` (empty state, role filtering, mixed statuses, dependency blocking/satisfaction), `CountReviewableTasks` (empty state, status filtering, role filtering), `GetCoderWorkDiagnostics` (claimable found, blocked-by-deps, in-progress, combined), `GetReviewerWorkDiagnostics` (unassigned, expired leases, active reviews, nil lease handling).
