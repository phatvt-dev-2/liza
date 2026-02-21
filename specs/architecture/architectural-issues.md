# Architectural Issues

Persistent record of issues identified by architectural analysis skills.

**Skills that contribute here:**
- `systemic-thinking` — Systemic coherence and risk analysis
- `software-architecture-review` — Code-level architectural patterns and smells

## Update Policy

1. Keep unresolved concerns in their thematic sections (load-bearing, tensions, smells, etc.).
2. When an issue is fixed, record it in `Completed Fixes` and `Fixed (Traceability)` with commit references.
3. Do not delete resolved issues from this document without preserving traceability metadata.
4. If a resolved issue is removed from an active section, add/update its `Fixed (Traceability)` entry in the same change.
5. `Fix Details` keeps the long-form rationale; `Fixed (Traceability)` is the canonical index for historical closure.

## Table of Contents

- [Update Policy](#update-policy)
- [Structural Load-Bearing Elements](#structural-load-bearing-elements)
  - [Planner as Single Semantic Interpreter](#planner-as-single-semantic-interpreter)
  - [Supervisor as Single Correctness Gate](#supervisor-as-single-correctness-gate)
- [Systemic Tensions](#systemic-tensions)
  - [Spec Completeness vs Reality](#spec-completeness-vs-reality)
- [Feedback Loops](#feedback-loops)
  - [Hypothesis Exhaustion Without Root Cause](#hypothesis-exhaustion-without-root-cause)
  - [Restart/Lease Churn Under Load](#restartlease-churn-under-load)
  - [Supervisor Wait-Claim-Spawn Loop](#supervisor-wait-claim-spawn-loop)
- [Assumptions](#assumptions)
  - [Human Availability as Bottleneck](#human-availability-as-bottleneck)
  - [Spec Maturity Dependency](#spec-maturity-dependency)
  - [Well-Formed Blackboard State](#well-formed-blackboard-state)
- [Stress Points](#stress-points)
  - [Supervisor Contention](#supervisor-contention)
  - [Filesystem/Git I/O Contention](#filesystemgit-io-contention)
- [Fragility](#fragility)
  - [Cross-Script State Mutation](#cross-script-state-mutation)
- [Trajectory](#trajectory)
  - [Blackboard Growth Without Pruning](#blackboard-growth-without-pruning)
  - [Anomaly Detail Validation Incomplete](#anomaly-detail-validation-incomplete)
  - [Task Type Registry is Partial Abstraction](#task-type-registry-is-partial-abstraction)
- [Code-Level Architectural Smells](#code-level-architectural-smells)
  - [Interactive Stdin in Library Packages](#interactive-stdin-in-library-packages)
- [Accepted v1 Limitations](#accepted-v1-limitations)
  - [Self-Reported Validation](#self-reported-validation)
  - [Kill Switch Granularity](#kill-switch-granularity)
- [Completed Fixes](#completed-fixes)
- [Fixed (Traceability)](#fixed-traceability)
- [Fix Details](#fix-details)
  - [Documentation/Implementation Desynchronization](#documentationimplementation-desynchronization)
  - [YAML Round-Trip Data Loss](#yaml-round-trip-data-loss)
  - [Merge Conflict Resolution](#merge-conflict-resolution)
  - [Anomaly Log Reader](#anomaly-log-reader)
  - [Human Role Clarification](#human-role-clarification)
  - [Task Dependencies](#task-dependencies)
  - [Supervisor Clarification](#supervisor-clarification)
  - [Review Lease Validation](#review-lease-validation)
  - [Multi-State Claiming](#multi-state-claiming)
  - [Approval Rate Monitoring](#approval-rate-monitoring)
  - [Root Cause Required Before Rescope](#root-cause-required-before-rescope)
  - [Iteration-Limit Config Drift](#iteration-limit-config-drift)
  - [Error Classification Lost at Agent Interface](#error-classification-lost-at-agent-interface)
  - [Implicit State Machine](#implicit-state-machine)
  - [Multi-Instance Blackboard Coherence](#multi-instance-blackboard-coherence)
  - [Magic Number 1800 Scattered](#magic-number-1800-scattered)
  - [executeTemplate Panics on Error](#executetemplate-panics-on-error)
  - [Inconsistent NotFoundError Usage](#inconsistent-notfounderror-usage)
  - [Supervisor God File](#supervisor-god-file)
  - [Duplicated File-Locking Mechanism](#duplicated-file-locking-mechanism)
  - [MCP Handler Bypasses Blackboard Locking](#mcp-handler-bypasses-blackboard-locking)
  - [Commands Presentation+Logic Coupling](#commands-presentationlogic-coupling)
  - [Agent → Commands Upward Dependency](#agent--commands-upward-dependency)
  - [Pervasive Task-Lookup Duplication](#pervasive-task-lookup-duplication)
  - [Untested MCP Server Dispatch Layer](#untested-mcp-server-dispatch-layer)
  - [Untested Work Detection Logic](#untested-work-detection-logic)
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


## Code-Level Architectural Smells

Issues identified through code-level architectural analysis (patterns, structure, duplication).

### Interactive Stdin in Library Packages

**Skill:** software-architecture-review
**Category:** Untestable by design
**Status:** PARTIALLY RESOLVED — MCP-exposed commands no longer read stdin (business logic in ops); remaining stdin reads are in CLI-only commands not exposed to MCP

**Issue:** Direct `os.Stdin` reads via `bufio.NewReader(os.Stdin)` or `bufio.NewScanner(os.Stdin)` in: `embedded/embedded.go` (2 locations: `WriteClaudeSettings`, `WriteMCPSettings`), `commands/setup.go` (2 locations), `commands/init.go` (1), `commands/delete_task.go` (2), `commands/delete_agent.go` (1). Total: 8 locations across 5 files in 2 packages.

**Current state:** The ops extraction resolved the MCP protocol corruption risk — MCP handlers now call `ops.*` functions that have zero I/O. The remaining stdin reads are in CLI-only interactive commands (`setup`, `init`, `delete_task`, `delete_agent`) and `embedded/` settings management, none of which are MCP-exposed. Both `delete_task.go` and `delete_agent.go` retain interactive confirmation at the CLI wrapper level but delegate business logic to `ops.CheckDeleteTask()` + `ops.DeleteTask()` and `ops.DeleteAgent()` respectively — business logic is fully testable without stdin.

**Remaining concern:** Functions with hardwired stdin still cannot be used non-interactively. Tests work around this by replacing `os.Stdin` with pipe readers (observed in 8+ test files) — fragile and not safe for concurrent test execution.

**Direction:** Accept an `io.Reader` parameter or a `Confirmer` callback for interactive prompts. Default to `os.Stdin` at the CLI call site in `cmd/liza/main.go`.


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
- [x] Iteration-limit config drift (`max_coder_iterations`, `max_review_cycles`, `task.max_iterations`) — enforce effective limits in `ClaimTask`/`SubmitVerdict` with explicit BLOCKED escalation *(software-architecture-review)*
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
- [x] Duplicated file-locking mechanism — extracted to `internal/filelock` package, both `db` and `log` use shared implementation *(software-architecture-review)*
- [x] Pervasive task-lookup duplication — `State.FindTask()` and `FindTaskIndex()` replace 35+ inline loops and 3 duplicate helpers *(software-architecture-review)*
- [x] Supervisor god file — decomposed 1,426 LOC into 6 cohesive files within `internal/agent/` by responsibility *(software-architecture-review)*
- [x] Agent → commands upward dependency — extracted business logic to `internal/ops/` package, `agent` no longer imports `commands` *(software-architecture-review)*
- [x] Commands presentation+logic coupling — extracted all 15 MCP-exposed mutation commands to `internal/ops/`; MCP handlers call ops directly; commands are thin presentation wrappers *(software-architecture-review)*
- [x] Monolithic DeleteTaskCommand — extracted business logic to `ops.CheckDeleteTask()` + `ops.DeleteTask()` (220→~75 LOC); interactive confirmation remains at CLI level *(software-architecture-review)*
- [x] Magic number 1800 scattered — defined `Default{LeaseDurationSeconds,*PollInterval,*MaxWait}` constants in `models/state.go`; all 9 fallback sites now reference named constants *(software-architecture-review)*
- [x] executeTemplate panics on error — changed to return `(string, error)` in both `prompts/templates.go` and `commands/templates.go`; propagated through all callers *(software-architecture-review)*
- [x] Multi-instance Blackboard coherence — `db.For()` process-level singleton constructor; all ~30 production `db.New()` calls replaced; tests retain `db.New()` for isolation *(systemic-thinking)*
- [x] Documentation/Implementation Desynchronization — replaced all operational `yq` references across 8 docs/specs files with `liza` CLI equivalents or tool-agnostic instructions *(systemic-thinking)*
- [x] YAML Round-Trip Data Loss — added `Extra map[string]any` with `yaml:",inline"` to all model structs; unknown YAML fields now survive round-trips *(systemic-thinking)*
- [x] Inconsistent NotFoundError Usage — added `ID` field to `NotFoundError`, migrated 25+ ad-hoc string errors to structured type across `ops/`, `db/`, `agent/`, `commands/`; `IsNotFound()` uses `errors.As`; MCP `classifyError()` uses type-based detection with string fallback *(software-architecture-review)*

---


## Fixed (Traceability)

Commit SHA where issue details were first marked as fixed (proxy for actual fix commit).

| Issue | Marked Fixed In |
|-------|-----------------|
| Documentation/Implementation Desynchronization | `e9a932e` |
| YAML Round-Trip Data Loss | `50056d2` |
| Merge Conflict Resolution | `de4bebf` |
| Anomaly Log Reader | `de4bebf` |
| Human Role Clarification | `de4bebf` |
| Task Dependencies | `de4bebf` |
| Supervisor Clarification | `de4bebf` |
| Review Lease Validation | `de4bebf` |
| Multi-State Claiming | `de4bebf` |
| Approval Rate Monitoring | `de4bebf` |
| Root Cause Required Before Rescope | `de4bebf` |
| Error Classification Lost at Agent Interface | `a1e347b` |
| Implicit State Machine | `2b5d236` |
| Multi-Instance Blackboard Coherence | `9d1890c` |
| Magic Number 1800 Scattered | `150c4d0` |
| executeTemplate Panics on Error | `ad3288c` |
| Inconsistent NotFoundError Usage | `e6f7bd2` |
| Supervisor God File | `c281430` |
| Duplicated File-Locking Mechanism | `a0bd779` |
| MCP Handler Bypasses Blackboard Locking | `af911ed` |
| Commands Presentation+Logic Coupling | `bfe179d` |
| Agent → Commands Upward Dependency | `c7e98d7` |
| Pervasive Task-Lookup Duplication | `363b440` |
| Untested MCP Server Dispatch Layer | `40ef645` |
| Untested Work Detection Logic | `40ef645` |
| Iteration-Limit Config Drift (`max_coder_iterations`, `max_review_cycles`, `task.max_iterations`) | `5fceaad` |

---


## Fix Details

### Documentation/Implementation Desynchronization

**Skill:** systemic-thinking
**Category:** TENSION

**Fix:** Complete documentation sweep replacing all operational `yq` commands across 8 files:
- Read-only queries → `liza get`/`liza status` equivalents
- Agent deletion → `liza delete agent`
- Task claim release → `liza release-claim`
- Manual state repairs → tool-agnostic "edit state.yaml" instructions
- Protocol pseudo-code → notes referencing Go implementation

Remaining `yq` references are historical only (ADRs, release notes, benchmark traces) or in independent tooling (spec-backfill scripts).

### YAML Round-Trip Data Loss

**Skill:** systemic-thinking
**Category:** FRAGILITY

**Fix:** Added `Extra map[string]any` with `yaml:",inline"` tag to all model structs in `internal/models/state.go`. The yaml.v3 inline map captures unknown YAML keys during unmarshal and emits them back during marshal. Known struct fields take priority (no duplication). Nil maps produce zero output (no change to existing YAML). Unknown fields at all nesting levels (root, task, agent, config, etc.) now survive `Blackboard.Modify()` and `Read()`+`Write()` round-trips.

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

### Iteration-Limit Config Drift

**Skill:** software-architecture-review
**Category:** Hardcoded configuration / Runtime contract drift

**Original issue:** Iteration controls were declarative only. `config.max_coder_iterations`, `config.max_review_cycles`, and task-level `max_iterations` were modeled/documented but not enforced in runtime flow, so operators could tune limits with no effect.

**Fix:** Added runtime enforcement in `internal/ops`:
- `ClaimTask` now enforces effective coder iteration limit (`task.max_iterations` override, else `config.max_coder_iterations`) before REJECTED reclaim; exhausted tasks transition to `BLOCKED`.
- `SubmitVerdict` now enforces review-cycle and iteration ceilings during rejection flow; exhausted tasks transition to `BLOCKED` with explicit `blocked_reason`/`blocked_questions`.
- State machine updated to allow `REJECTED -> BLOCKED` transitions; tests added in `internal/ops/claim_task_test.go`, `internal/ops/submit_verdict_test.go`, and `internal/models/state_test.go`.
- Follow-up clean-code refactor in `be93dee` extracted escalation helpers (`enforceRejectedIterationLimit`, `classifyLimitEscalation`) with no behavioral change.

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

### Multi-Instance Blackboard Coherence

**Skill:** systemic-thinking
**Category:** ASSUMPTION

**Original issue:** ~31 production `db.New()` call sites each created independent `Blackboard` instances with their own cache state and lock objects. This worked because `Blackboard` was stateless beyond its mtime-based cache, but created an invisible constraint: any future addition of in-process state (metrics, write batching, subscriptions) would fragment silently across instances with no error signal.

**Fix:** Added `db.For(statePath)` — a process-level singleton constructor using `sync.Map`. All production `db.New()` calls replaced with `db.For()`. Callers sharing the same state path within a process now get the same `*Blackboard` instance, ensuring cache coherence and preventing future state fragmentation. `db.New()` retained for tests that need independent instances (natural isolation via unique temp directories). `db/doc.go` documents the instance management pattern.

### Magic Number 1800 Scattered

**Skill:** software-architecture-review
**Category:** Hardcoded configuration

**Fix:** Defined `DefaultLeaseDurationSeconds`, `Default{Coder,Planner,Reviewer}{PollInterval,MaxWait}` constants in `internal/models/state.go` alongside the `Config` struct. All 3 lease-duration fallback sites and 6 poll/wait fallbacks in `getRoleWaitConfig` now reference the named constants. `heartbeat.DefaultLeaseDuration` derives from `models.DefaultLeaseDurationSeconds` (single source of truth).

### executeTemplate Panics on Error

**Skill:** software-architecture-review
**Category:** Leaky abstraction / Non-idempotent operations

**Fix:** Changed `executeTemplate` in `internal/prompts/templates.go` and `executeCommandTemplate` in `internal/commands/templates.go` to return `(string, error)` instead of panicking. Propagated error returns through all callers: `Build{BasePrompt,PlannerContext,CoderContext,ReviewerContext}` in `prompts/builder.go`, `buildInstructionsForWakeTrigger`, `format{AgentValue,MetricsValue}` in `commands/`, and `buildPrompt` in `agent/prompt.go`. All callers already returned `(string, error)` or were internal — propagation was straightforward.

### Inconsistent NotFoundError Usage

**Skill:** software-architecture-review
**Category:** Primitive obsession / Unstable interface

**Fix:** Added `ID` field to `NotFoundError` (producing `"task not found: task-42"` matching the ad-hoc format). Migrated 25+ ad-hoc `fmt.Errorf("...not found...")` sites across `ops/` (12 files), `db/blackboard.go`, `agent/` (4 files), and `commands/inspect_*.go` (3 files) to use `&errors.NotFoundError{Entity: ..., ID: ...}`. Updated `IsNotFound()` to use `errors.As` (supports wrapped errors from `bb.Modify`). MCP `classifyError()` now uses type-based `errors.As` check first, with string fallback retained for external errors (git, etc.).

### Supervisor God File

**Skill:** software-architecture-review
**Category:** God class/module
**Status:** RESOLVED — decomposed into 6 cohesive files within `internal/agent/`

**Fix:** Split `supervisor.go` (1,426 LOC, 31 functions) into 6 files by responsibility:
- `supervisor.go` (~270 LOC) — types, interfaces, main loop (`RunSupervisor`)
- `registration.go` (~175 LOC) — agent identity and lifecycle
- `waitforwork.go` (~300 LOC) — work detection with event-driven + polling
- `claiming.go` (~230 LOC) — task claiming and merge handling
- `prompt.go` (~95 LOC) — prompt assembly
- `systemctl.go` (~160 LOC) — system control, execution, planner verification

Test files split correspondingly. `supervisor_priority_test.go` renamed to `claiming_priority_test.go`. No signature changes, no behavior changes.

### Duplicated File-Locking Mechanism

**Skill:** software-architecture-review
**Category:** DRY violation / Shotgun surgery
**Status:** RESOLVED — extracted to `internal/filelock` package

**Fix:** Created `internal/filelock` package with the complete locking implementation (lock acquisition, PID-based stale lock detection, error classification, metrics). Both `internal/db` and `internal/log` now use `filelock.FileLock` instead of independent implementations. The log package gained stale lock recovery and error classification it previously lacked. Constants (`DefaultLockTimeout`, `LockCheckInterval`) exist in one place. No external consumers of the old `db.LockError` types existed, so no aliases were needed.

### MCP Handler Bypasses Blackboard Locking

**Skill:** software-architecture-review
**Category:** Leaky abstraction / Boundary violation
**Status:** RESOLVED — `readStateResource()` now uses `Blackboard.ReadRaw()` under flock

### Commands Presentation+Logic Coupling

**Skill:** software-architecture-review
**Category:** Leaky abstraction / Inappropriate intimacy
**Status:** RESOLVED — all MCP-exposed mutation commands extracted to `internal/ops/`

**Issue:** The `commands` package serves three consumers with incompatible I/O expectations — CLI (terminal), MCP server (JSON-RPC over stdio), and supervisor (background process) — but embeds terminal assumptions: 40+ `fmt.Print*` calls to stdout/stderr and 5+ direct `os.Stdin` reads in non-test production code. Functions like `ClaimTaskCommand()` print success messages, `SetupCommand()` prompts for confirmation, and `DeleteTaskCommand()` reads interactive input.

**Implication:** MCP server calls commands via `handlers.go` that print to stdout, which is the JSON-RPC transport channel — stdout writes from commands could corrupt the protocol stream. Supervisor calls (`commands.ClaimTaskCommand()`, `commands.WtMergeCommand()`) mix operational output with supervisor logs. Tests must monkey-patch `os.Stdin` (8+ test files use `os.Stdin = r` / `defer func() { os.Stdin = oldStdin }()`) — fragile and not concurrency-safe.

**Direction:** Separate business logic from presentation. Command functions return structured results; callers handle output. The MCP adapter already does this partially — `StatusCommand()` returns a string. Extend pattern to mutation commands. This also resolves the agent→commands coupling (see below).

### Agent → Commands Upward Dependency

**Skill:** software-architecture-review
**Category:** Leaky abstraction
**Status:** RESOLVED — extracted to `internal/ops` package

**Fix:** Created `internal/ops/` package with pure business logic functions: `ClaimTask()` returning `*ClaimResult`, `MergeWorktree()` returning `*MergeResult`, `ClearStaleReviewClaims()` returning `(int, error)`, and `UpdateSprintMetrics()` returning `(SprintMetrics, error)`. `IntegrationFailedError` moved to `ops`. Command files in `internal/commands/` became thin presentation wrappers that call `ops` functions and format output. `internal/agent/` now imports `ops` instead of `commands` — the upward dependency is eliminated. Integration test subprocess output captured to `bytes.Buffer` in the ops layer (included in `MergeResult.TestOutput` and `IntegrationFailedError.TestOutput`) instead of wired to terminal.

### Pervasive Task-Lookup Duplication

**Skill:** software-architecture-review
**Category:** DRY violation / Shotgun surgery
**Status:** RESOLVED — `State.FindTask()` and `FindTaskIndex()` added to `internal/models/state.go`

**Fix:** Added `FindTask(taskID string) *Task` and `FindTaskIndex(taskID string) int` methods to `*models.State`. Migrated all ~35 inline ID-lookup loops in non-test production code across `commands/`, `agent/`, `db/`, and `models/` packages. Removed 3 duplicate private helper functions (`findTaskByID` in `supervisor.go` and `inspect_agents.go`, `findTask` in `validate.go`). `Blackboard.GetTask()` and `UpdateTask()` now delegate to `State.FindTask()` internally. Bug fixes to task-lookup logic now require changing one method instead of 35+ locations.

### Untested MCP Server Dispatch Layer

**Skill:** software-architecture-review
**Category:** Untested critical path
**Status:** RESOLVED — `internal/mcp/server_dispatch_test.go` added

**Fix:** Added `server_dispatch_test.go` with table-driven tests covering: `HandleRequest` routing (all 4 method branches + unknown method), `handleToolCall` (invalid params, missing name, unknown tool, successful handler, nil arguments, handler error with classification), `handleResourceRead` (invalid params), `classifyError` (all 5 classification branches: not found, lock timeout, race condition, validation, internal — 14 test cases), leak prevention (raw error strings never exposed), `handleNotification` (known and unknown). Request ID preservation verified.

### Untested Work Detection Logic

**Skill:** software-architecture-review
**Category:** Untested critical path
**Status:** RESOLVED — `internal/models/diagnostics_test.go` added

**Fix:** Added `diagnostics_test.go` with table-driven tests covering all 4 functions: `CountClaimableTasks` (empty state, role filtering, mixed statuses, dependency blocking/satisfaction), `CountReviewableTasks` (empty state, status filtering, role filtering), `GetCoderWorkDiagnostics` (claimable found, blocked-by-deps, in-progress, combined), `GetReviewerWorkDiagnostics` (unassigned, expired leases, active reviews, nil lease handling).

---
