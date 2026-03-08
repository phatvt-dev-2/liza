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
6. Keep the **Open Issues Summary** table in sync when adding, resolving, or re-prioritizing issues.

## Table of Contents

- [Update Policy](#update-policy)
- [Open Issues Summary](#open-issues-summary)
- [Structural Load-Bearing Elements](#structural-load-bearing-elements)
  - [Mode Selection Trigger Coupled to Prompt Lexeme](#mode-selection-trigger-coupled-to-prompt-lexeme)
- [Systemic Tensions](#systemic-tensions)
  - [Two-Track State Mutation](#two-track-state-mutation)
  - [MCP Cross-Layer Read Dependency](#mcp-cross-layer-read-dependency)
  - [Role-Boundary Severity Drift](#role-boundary-severity-drift)
  - [Merge Execution Authority Split](#merge-execution-authority-split)
  - [Sprint Completion Signal Diverges from Active Scope](#sprint-completion-signal-diverges-from-active-scope)
  - [Task Type Registry Only Supports Coding Workflows](#task-type-registry-only-supports-coding-workflows)
- [Feedback Loops](#feedback-loops)
  - [Supervisor Wait-Claim-Spawn Loop](#supervisor-wait-claim-spawn-loop)
  - [Contract Complexity vs Context Pressure](#contract-complexity-vs-context-pressure)
  - [Issue Registry Resolution Drift](#issue-registry-resolution-drift)
- [Assumptions](#assumptions)
  - [Implicit Orchestrator Provenance Default](#implicit-orchestrator-provenance-default)
  - [Spec Maturity Dependency](#spec-maturity-dependency)
  - [Well-Formed Blackboard State](#well-formed-blackboard-state)
  - [Single-Goal Data Model Constrains Applicability](#single-goal-data-model-constrains-applicability)
  - [Orchestrator State Change Verification is Non-Binding](#orchestrator-state-change-verification-is-non-binding)
- [Stress Points](#stress-points)
  - [Filesystem/Git I/O Contention](#filesystemgit-io-contention)
  - [Cache Coherence Gap in Multi-Process Deployments](#cache-coherence-gap-in-multi-process-deployments)
- [Fragility](#fragility)
  - [Cross-Script State Mutation](#cross-script-state-mutation)
  - [Bootstrap Artifact Path Drift](#bootstrap-artifact-path-drift)
  - [File-Based Spec References Without Version Anchors](#file-based-spec-references-without-version-anchors)
  - [Review Lease Orphaning Without Automatic Reclamation](#review-lease-orphaning-without-automatic-reclamation)
- [Blind Spots](#blind-spots)
  - [Contract Effectiveness Self-Certification](#contract-effectiveness-self-certification)
  - [Initialization Completion Unverifiable](#initialization-completion-unverifiable)
  - [No Source Type for Pre-Implementation Spec Findings](#no-source-type-for-pre-implementation-spec-findings)
  - [Prompt-Build-to-Execution State Drift](#prompt-build-to-execution-state-drift)
- [Trajectory](#trajectory)
  - [Blackboard Growth Without Pruning](#blackboard-growth-without-pruning)
  - [Role Addition Accelerates Contract Complexity Pressure](#role-addition-accelerates-contract-complexity-pressure)
  - [Anomaly Detail Validation Incomplete](#anomaly-detail-validation-incomplete)
  - [Task Type Registry is Partial Abstraction](#task-type-registry-is-partial-abstraction)
  - [Spec Corpus Lacks Lifecycle Management](#spec-corpus-lacks-lifecycle-management)
  - [Metrics Collection Without Query Interface](#metrics-collection-without-query-interface)
  - [No Query Layer](#no-query-layer)
- [Accepted v1 Limitations](#accepted-v1-limitations)
  - [Orchestrator as Single Semantic Interpreter](#orchestrator-as-single-semantic-interpreter)
  - [Supervisor as Single Correctness Gate](#supervisor-as-single-correctness-gate)
  - [Spec Completeness vs Reality](#spec-completeness-vs-reality)
  - [Code Reviewer Structural Accountability Gap](#code-reviewer-structural-accountability-gap)
  - [Hypothesis Exhaustion Without Root Cause](#hypothesis-exhaustion-without-root-cause)
  - [Restart/Lease Churn Under Load](#restartlease-churn-under-load)
  - [Human Availability as Bottleneck](#human-availability-as-bottleneck)
  - [Supervisor Contention](#supervisor-contention)
  - [Dual Contract Delivery Paths](#dual-contract-delivery-paths)
  - [Self-Reported Validation](#self-reported-validation)
  - [Kill Switch Granularity](#kill-switch-granularity)
- [Completed Fixes](#completed-fixes)
- [Fixed (Traceability)](#fixed-traceability)
- [Fix Details](#fix-details)

## Open Issues Summary

| Priority | Category | Issue |
|----------|----------|-------|
| **high** | LOAD-BEARING | [Mode Selection Trigger Coupled to Prompt Lexeme](#mode-selection-trigger-coupled-to-prompt-lexeme) |
| **high** | LOAD-BEARING | [Orchestrator as Single Semantic Interpreter](#orchestrator-as-single-semantic-interpreter) |
| **high** | LOAD-BEARING | [Supervisor as Single Correctness Gate](#supervisor-as-single-correctness-gate) |
| **high** | TENSION | [Role-Boundary Severity Drift](#role-boundary-severity-drift) |
| **high** | TENSION | [Code Reviewer Structural Accountability Gap](#code-reviewer-structural-accountability-gap) |
| **high** | FEEDBACK | [Contract Complexity vs Context Pressure](#contract-complexity-vs-context-pressure) |
| **high** | STRESS POINT | [Filesystem/Git I/O Contention](#filesystemgit-io-contention) |
| **high** | STRESS POINT | [Cache Coherence Gap in Multi-Process Deployments](#cache-coherence-gap-in-multi-process-deployments) |
| **high** | FRAGILITY | [Dual Contract Delivery Paths](#dual-contract-delivery-paths) |
| **medium** | TENSION | [Two-Track State Mutation](#two-track-state-mutation) (partially resolved) |
| **medium** | TENSION | [MCP Cross-Layer Read Dependency](#mcp-cross-layer-read-dependency) |
| **medium** | TENSION | [Merge Execution Authority Split](#merge-execution-authority-split) |
| **medium** | TENSION | [Sprint Completion Signal Diverges from Active Scope](#sprint-completion-signal-diverges-from-active-scope) |
| **medium** | TENSION | [Task Type Registry Only Supports Coding Workflows](#task-type-registry-only-supports-coding-workflows) |
| **medium** | TENSION | [Spec Completeness vs Reality](#spec-completeness-vs-reality) |
| **medium** | FEEDBACK | [Supervisor Wait-Claim-Spawn Loop](#supervisor-wait-claim-spawn-loop) |
| **medium** | FEEDBACK | [Issue Registry Resolution Drift](#issue-registry-resolution-drift) |
| **medium** | FEEDBACK | [Hypothesis Exhaustion Without Root Cause](#hypothesis-exhaustion-without-root-cause) |
| **medium** | FEEDBACK | [Restart/Lease Churn Under Load](#restartlease-churn-under-load) |
| **medium** | ASSUMPTION | [Implicit Orchestrator Provenance Default](#implicit-orchestrator-provenance-default) |
| **medium** | ASSUMPTION | [Well-Formed Blackboard State](#well-formed-blackboard-state) |
| **medium** | ASSUMPTION | [Orchestrator State Change Verification is Non-Binding](#orchestrator-state-change-verification-is-non-binding) |
| **medium** | ASSUMPTION | [Human Availability as Bottleneck](#human-availability-as-bottleneck) |
| **medium** | STRESS POINT | [Supervisor Contention](#supervisor-contention) |
| **medium** | FRAGILITY | [Cross-Script State Mutation](#cross-script-state-mutation) |
| **medium** | FRAGILITY | [Bootstrap Artifact Path Drift](#bootstrap-artifact-path-drift) |
| **medium** | FRAGILITY | [File-Based Spec References Without Version Anchors](#file-based-spec-references-without-version-anchors) |
| **medium** | FRAGILITY | [Review Lease Orphaning Without Automatic Reclamation](#review-lease-orphaning-without-automatic-reclamation) |
| **medium** | BLIND SPOT | [Contract Effectiveness Self-Certification](#contract-effectiveness-self-certification) |
| **medium** | BLIND SPOT | [Initialization Completion Unverifiable](#initialization-completion-unverifiable) |
| **medium** | ACCEPTED v1 | [Self-Reported Validation](#self-reported-validation) |
| **low** | ASSUMPTION | [Spec Maturity Dependency](#spec-maturity-dependency) |
| **low** | ASSUMPTION | [Single-Goal Data Model Constrains Applicability](#single-goal-data-model-constrains-applicability) |
| **low** | BLIND SPOT | [No Source Type for Pre-Implementation Spec Findings](#no-source-type-for-pre-implementation-spec-findings) |
| **low** | BLIND SPOT | [Prompt-Build-to-Execution State Drift](#prompt-build-to-execution-state-drift) |
| **low** | TRAJECTORY | [Blackboard Growth Without Pruning](#blackboard-growth-without-pruning) |
| **low** | TRAJECTORY | [Role Addition Accelerates Contract Complexity Pressure](#role-addition-accelerates-contract-complexity-pressure) |
| **low** | TRAJECTORY | [Anomaly Detail Validation Incomplete](#anomaly-detail-validation-incomplete) |
| **low** | TRAJECTORY | [Task Type Registry is Partial Abstraction](#task-type-registry-is-partial-abstraction) |
| **low** | TRAJECTORY | [Spec Corpus Lacks Lifecycle Management](#spec-corpus-lacks-lifecycle-management) |
| **low** | TRAJECTORY | [Metrics Collection Without Query Interface](#metrics-collection-without-query-interface) |
| **low** | TRAJECTORY | [No Query Layer](#no-query-layer) |
| **low** | ACCEPTED v1 | [Kill Switch Granularity](#kill-switch-granularity) |

**Counts:** 9 high, 22 medium, 12 low — 43 open issues total.

---

## Structural Load-Bearing Elements

Single points of failure with no redundancy or validation mechanism.

### Mode Selection Trigger Coupled to Prompt Lexeme

**Skill:** systemic-thinking
**Category:** LOAD-BEARING

**Issue:** Mode selection in CORE depends on detecting specific bootstrap wording (`"You are a Liza ... agent"` for Liza mode, `MODE: SUBAGENT` for subagent mode). The prompt template (`base_prompt.tmpl`) currently generates matching wording (`"You are a Liza {{.Role}} agent"`), so there is no active mismatch. However, because gate semantics and approval behavior branch entirely on this lexical detection, the coupling is load-bearing despite being outside the blackboard/state machine. A template edit, prompt builder refactor, or alternative CLI integration that changes the wording would silently change mode without any structural guard.

**Implication:** Mode detection correctness depends on convention alignment between two independently maintained artifacts (CORE.md detection table and prompt templates). No compile-time or runtime check validates this alignment.

**Current mitigation:** Prompt template output currently matches CORE.md detection patterns. Builder tests (`builder_test.go`) assert the expected prefix, providing regression coverage.

**Future options:**
- Add explicit mode declaration outside free-text prompt (e.g., structured field/environment variable)
- Add startup self-check that fails fast when expected mode and detected mode diverge
- Record detected mode in blackboard state for runtime observability

---

## Systemic Tensions

Design contradictions that create structural friction.

### Two-Track State Mutation

**Skill:** systemic-thinking
**Category:** TENSION
**Status:** PARTIALLY RESOLVED (`ac4ce6f5`)

**Issue:** The `ops` extraction that resolved "Commands Presentation+Logic Coupling" was structurally incomplete. All task lifecycle mutations from CLI and MCP consumers route through `ops` — a clean business logic layer with typed inputs, structured results, and three-phase validation for claiming. But the `agent` package — the third and most critical consumer — ~~mutates both task and agent state directly via `bb.Modify` in `claimReviewerTask`, `resumeHandoffTask`,~~ `registerAgent`, `resetAgentAfterExit`, and `setAgentToPlanningStatus`. ~~This creates two mutation tracks: `ops` (validated, structured, reusable across CLI/MCP/agent) and `agent` (inline closures, only callable from the supervisor). The reviewer claiming path is the most consequential — it transitions task status, sets `reviewing_by`, updates agent state, and captures return values via closure variables, all in a single `Modify` closure with no structured result type and no way for MCP or CLI to invoke the same logic.~~ *(partially resolved: `ac4ce6f5` — reviewer claiming and handoff resumption extracted to `ops.ClaimReviewerTask` and `ops.ResumeHandoff` with structured input/result types and comprehensive test coverage)*

**Remaining gap:** Agent lifecycle management (`registerAgent`, `resetAgentAfterExit`, `setAgentToPlanningStatus`) still mutates state via inline `bb.Modify` closures in the `agent` package. These are agent-identity operations (not task-lifecycle), so the boundary may be intentional.

**Implication:** The ops layer is now the source of truth for all task lifecycle mutations including reviewer claiming. Agent lifecycle management remains a second mutation track, but with narrower scope (agent state only, not task transitions).

**Future options:**
- Extract `ops.RegisterAgent` / `ops.UnregisterAgent` for agent lifecycle
- Accept the split as intentional: ops owns task lifecycle, agent owns agent lifecycle (document the boundary)

### MCP Cross-Layer Read Dependency

**Skill:** systemic-thinking
**Category:** TENSION
**Coupled with:** [No Query Layer](#no-query-layer)

**Issue:** The MCP server's read-only handlers (`handleGet`, `handleStatus`, `handleValidate`) import and call `commands.InspectCommand`, `commands.StatusCommand`, and `commands.ValidateCommand` — CLI presentation functions that happen to return strings. This creates a cross-layer dependency: `mcp` (protocol presentation) depends on `commands` (CLI presentation), bypassing `ops` entirely for the read path. The mutation path is clean (`mcp` → `ops` → `db`); the read path is `mcp` → `commands` → `db`, bridging two presentation layers. This dependency exists because there is no query layer between `db` (raw state access) and the presentation layers.

**Implication:** Every new read operation will either be implemented in `commands` (wrong layer for MCP) or duplicated between consumers — the system will accumulate presentation-layer coupling as the query surface grows.

**Current mitigation:** The read commands (`InspectCommand`, `StatusCommand`) return plain strings and don't perform terminal I/O, so the coupling is dormant.

**Future options:**
- Extract query functions to `ops` or a new `queries` package (return structured data, let each presentation layer format)
- Accept the coupling and document `commands` as the shared query+formatting layer (rename or annotate to clarify dual role)

### Role-Boundary Severity Drift

**Skill:** systemic-thinking
**Category:** TENSION

**Issue:** Vision-level contract text classifies role-boundary violations as Tier 0 (contract termination), while the active Multi-Agent mode contract classifies the same class of violations as Tier 1. The same behavioral breach therefore has two incompatible severities across authoritative artifacts.

**Implication:** Violation handling semantics become document-dependent, so recovery behavior can vary by which artifact an agent or operator treats as canonical.

**Current mitigation:** None structural; conflict is resolved ad hoc by whichever document is consulted first.

**Future options:**
- Align role-boundary severity to a single tier across all mode and vision artifacts
- Add consistency checks in contract maintenance workflow for severity-classified rules
- Publish one canonical severity table referenced by all contracts

### Merge Execution Authority Split

**Skill:** systemic-thinking
**Category:** TENSION

**Issue:** Task/worktree protocols state that Code Reviewer executes merge on approval, while role/supervision architecture defines Code Reviewer as read-only and supervisor as the merge executor. Merge authority and operational responsibility are split across documents.

**Implication:** Integration ownership is ambiguous, weakening accountability and making post-incident reconstruction of merge authority less reliable.

**Current mitigation:** Runtime flow appears supervisor-driven in architecture documents, so practical execution tends to converge despite specification drift.

**Future options:**
- Normalize all protocol docs to one merge authority model
- Record merge executor identity explicitly in task history for auditability
- Add validation/docs linting to flag authority contradictions across artifacts

### Task Type Registry Only Supports Coding Workflows

**Skill:** systemic-thinking
**Category:** TENSION
**Related:** [Task Type Registry is Partial Abstraction](#task-type-registry-is-partial-abstraction)

**Issue:** The task workflow registry (`taskWorkflows` in `internal/models/state.go`) contains exactly one entry: `coding` maps to `[coder, code_reviewer]`. The claimability logic in `IsClaimable()` derives role eligibility from this registry. Adding any upstream role that produces specifications (e.g., turning vision docs into PRDs) requires either: (a) creating a new task type with its own workflow, or (b) operating outside the task system entirely.

**Implication:** Pre-implementation specification work cannot be tracked, assigned, or validated through the same mechanisms as code tasks. The blackboard has no structural concept of "spec production" as work that progresses through states with agents assigned to it.

**Current mitigation:** Specification work is assumed to be complete before `liza init` runs, produced by humans or external processes.

**Future options:**
- Extend task type registry with `specification` or `prd` type and corresponding workflow
- Create explicit `spec_writer` role constant and claimability rules
- Document that spec production operates outside Liza workflow (human-only phase)

### Sprint Completion Signal Diverges from Active Scope

**Skill:** systemic-thinking
**Category:** TENSION

**Issue:** Sprint governance allows sprint completion when only the original planned task list is terminal, while replacement tasks created by rescoping may still be active. The completion signal is therefore cadence-based rather than work-closure-based.

**Implication:** A sprint can report completion while unresolved implementation risk remains in flight under replacement tasks.

**Current mitigation:** Sprint governance protocol (`sprint-governance.md`) explicitly documents this as expected behavior — humans must manually update `scope.planned[]` to include replacement tasks, or wait for all active tasks. The gap is that the `AllPlannedTasksTerminal()` function doesn't account for replacements, while the governance protocol assumes humans will maintain scope accuracy.

**Future options:**
- Promote replacement tasks into sprint planned scope automatically
- Add an alternate completion criterion based on all active (planned + replacement) tasks
- Separate cadence checkpoint status from true work-closure status

---

## Feedback Loops

Self-reinforcing patterns that can amplify failures.

### Supervisor Wait-Claim-Spawn Loop

**Skill:** systemic-thinking
**Category:** FEEDBACK

**Issue:** Supervisor's "wait → claim → spawn → restart" loop is tightly coupled with lease timing and work availability. Under slow tasks or transient failures, the loop can become self-reinforcing, cycling agents without progressing state.

**Implication:** System can be active but not advancing, with increasing log noise and human overhead.

**Future options:**
- Supervisor state machine with explicit "stalled" detection
- Alert on N cycles without state change
- Automatic pause after repeated no-progress cycles

### Contract Complexity vs Context Pressure

**Skill:** systemic-thinking
**Category:** FEEDBACK

**Issue:** The contract is the mechanism that suppresses agent failure modes. It competes for the same resource agents need to do work: context tokens. CORE.md is 800 lines. Add the mode contract (~200 lines), AGENT_TOOLS.md (94 lines), initialization reads (REPOSITORY.md, specs, lessons, collaboration continuity), skill files when loaded (100-300 lines each), the blackboard state, and the task's spec — a fresh session starts with 1500+ lines of governance before any work begins. The tier architecture and kernel appendix address degradation after it happens, but the fundamental dynamic is self-defeating: each new clause added to prevent a failure mode consumes context that makes other clauses harder to follow. The system's safety margin shrinks as its safety mechanisms grow.

**Implication:** The contract will hit a ceiling where adding another clause to prevent failure mode N+1 degrades compliance with clauses 1 through N, and no tier architecture can compensate because the contract must be loaded before tiers can be evaluated.

**Future options:**
- Contract compression (semantic deduplication, remove examples that models don't need)
- Conditional loading (only load clauses relevant to current role/task type)
- Structural enforcement replacing behavioral rules (move more rules into Go code, reducing contract size)
- Measure contract-to-work ratio across sessions to detect the ceiling empirically

### Issue Registry Resolution Drift

**Skill:** systemic-thinking
**Category:** FEEDBACK

**Issue:** The architectural issues registry is treated as the durable source of resolved-vs-open architectural risk, but its own resolution claims can diverge from live internal behavior. The `submit-for-review` `commit_sha` item is currently marked resolved in this file while the internal CLI/MCP/ops surfaces still require and enforce caller-provided SHA. That creates a reinforcing loop where planning and review work trusts the registry, then inherits stale assumptions, then perpetuates stale status.

**Implication:** Architectural debt tracking becomes self-invalidating: "resolved" no longer means the risk is absent in current runtime surfaces.

**Current mitigation:** Manual source verification during reviews can detect the mismatch, but only when a reviewer re-audits internals.

**Future options:**
- Add automated checks that verify each "resolved" entry against current code contracts
- Require a validation artifact (test/doc/assertion) link for every resolved architectural issue
- Add a `REGRESSED` status class to avoid binary resolved/unresolved drift

---

## Assumptions

Implicit dependencies that constrain system behavior.

### Implicit Orchestrator Provenance Default

**Skill:** systemic-thinking
**Category:** ASSUMPTION

**Issue:** Task-creation provenance assumes a synthetic orchestrator identity when none is provided. Both MCP `handleAddTask` and `ops.AddTask` default missing agent identity to `orchestrator-1`, so write attribution can be generated without proving who initiated the mutation.

**Implication:** Multi-orchestrator operation collapses to a synthetic single actor in audit trails, reducing accountability and weakening post-incident reconstruction of planning decisions.

**Current mitigation:** CLI and MCP can provide explicit `agent_id`, but omission silently falls back to the default identity.

**Future options:**
- Make orchestrator identity mandatory for task-creation mutations
- Distinguish system-authored vs agent-authored mutations with explicit provenance fields
- Add validation rejecting task-creation events with defaulted identity in multi-orchestrator mode

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

### Single-Goal Data Model Constrains Applicability

**Skill:** systemic-thinking
**Category:** ASSUMPTION

**Issue:** The blackboard schema has exactly one `goal` section, one `sprint` section, and a flat `tasks` array. This single-goal, single-sprint data model is documented as v1 scope (`specs/functional/1 - Liza.md`), but the structural implications are broader than the scoping language suggests. It prevents: concurrent goals (feature work alongside tech debt), multi-sprint planning (seeing the backlog beyond current sprint), hierarchical task relationships (epics containing stories), and project-level metrics (cross-sprint trends). The "every restart is a new mind with old artifacts" philosophy compounds this — there is no memory of previous sprints beyond what's manually archived, and no mechanism to learn from past sprint metrics because each sprint overwrites the metrics section.

**Implication:** Liza is structurally a single-feature-at-a-time system, and this constraint is embedded in the data model rather than documented as a design choice — teams discovering this limit will face a schema migration, not a configuration change.

**Future options:**
- Document as explicit v1 limitation in vision and deployment docs
- Sprint history array (append completed sprints rather than overwriting)
- Goal array with per-goal task filtering
- Backlog section separate from active sprint scope

### Orchestrator State Change Verification is Non-Binding

**Skill:** systemic-thinking
**Category:** ASSUMPTION

**Issue:** After orchestrator execution completes, `verifyOrchestratorStateChanges()` (`agent/systemctl.go`) logs a warning if expected state changes weren't made, but takes no corrective action. The supervisor continues the loop. However, the orchestrator re-invocation depends on `waitForWork` → `DetectOrchestratorWakeTriggers()`, which checks actual state conditions (unassigned tasks, anomalies, blocked tasks, etc.). If no wake triggers exist, the orchestrator waits indefinitely rather than looping. The infinite loop scenario requires persistent wake triggers that the orchestrator fails to resolve — e.g., a blocked task the orchestrator cannot unblock, or an anomaly it cannot interpret.

**Implication:** The system assumes orchestrators will eventually resolve wake triggers. An orchestrator stuck on an unresolvable trigger (spec ambiguity it cannot bypass, anomaly pattern it cannot interpret) will repeatedly execute without triggering escalation, consuming API tokens and time without progress signals. The failure mode is narrower than "any stuck orchestrator" but still lacks detection.

**Current mitigation:** None explicit. Circuit breaker anomaly patterns may eventually detect the orchestrator loop, but only if anomalies are logged.

**Future options:**
- Escalate to human after N consecutive orchestrator executions without state change
- Require orchestrator to document progress or blocking reason on each wake
- Add orchestrator-specific circuit breaker for no-op execution patterns

---

## Stress Points

Bottlenecks that emerge under load.

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

### Cache Coherence Gap in Multi-Process Deployments

**Skill:** systemic-thinking
**Category:** STRESS POINT

**Issue:** The `Blackboard` provides `ReadCached()` for performance, using mtime comparison to detect file changes. However, the cache is process-local and keyed by mtime alone. In a multi-process deployment (multiple `liza agent` instances, CLI commands, and MCP server), one process's cache invalidation doesn't propagate to others. Two processes can hold different cached versions of state simultaneously because there's no cross-process cache coherence mechanism—only file locking for writes.

**Implication:** Under concurrent load or multi-terminal operation, agents may make decisions based on stale state without any error signal, potentially causing claim races or missed work detection that the file locking was designed to prevent.

**Current mitigation:** Cache invalidation happens on write, so single-process deployments are consistent. File locking ensures write serialization.

**Future options:**
- Remove caching in favor of always reading under lock (simplest, performance cost)
- Add cache versioning or generation counter in state.yaml
- Document that `ReadCached()` is unsafe for multi-process use

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

### Bootstrap Artifact Path Drift

**Skill:** systemic-thinking
**Category:** FRAGILITY

**Issue:** Initialization and navigation artifacts reference paths that no longer align: Pairing initialization requires `docs/USAGE.md` even though only split usage docs exist, and multiple docs still point to `specs/build/0 - Vision.md` while the current spec index canonizes `specs/build/1 - Vision.md`. The bootstrap/read path is therefore partially identity-drifted.

**Implication:** Session bootstrap and orientation become dependent on ad hoc path discovery, creating non-deterministic context loading across agents and sessions.

**Current mitigation:** Repository-level discovery (`REPOSITORY.md`, `specs/README.md`) allows humans/agents to recover missing paths manually.

**Future options:**
- Introduce stable alias files for canonical bootstrap paths
- Add link/path integrity checks in CI for contract and spec references
- Generate initialization read lists from a single manifest rather than hardcoded paths

### File-Based Spec References Without Version Anchors

**Skill:** systemic-thinking
**Category:** FRAGILITY

**Issue:** The `spec_ref` field in tasks and goal uses file paths (e.g., `specs/retry-logic.md`, optionally with `#section` anchors). The anchors refer to headings within the file, not to versions of the file. Git tracks file history, but `spec_ref` contains no commit SHA, no version identifier, and no content hash. When a task cites `specs/api.md#pagination`, it references whatever content currently exists at that heading.

**Implication:** Spec drift during task execution is undetectable. A PRD produced by a spec-authoring agent and consumed by the Orchestrator can change between when the Orchestrator decomposes it and when the Coder implements the resulting tasks. The blackboard's `spec_changes` log tracks that changes occurred, not which tasks were affected by which changes.

**Current mitigation:** Code Reviewer validates against "current spec version" and logs `spec_changed` anomaly if material changes detected.

**Future options:**
- Include commit SHA or content hash in `spec_ref`
- Track `spec_version` at task creation and warn on divergence
- Generate spec snapshots when tasks are created

### Review Lease Orphaning Without Automatic Reclamation

**Skill:** systemic-thinking
**Category:** FRAGILITY

**Issue:** Review leases expire based on `review_lease_expires` timestamp. Stale leases need periodic clearing to prevent tasks from getting stuck in REVIEWING with expired leases, particularly after crash scenarios where the agent cannot execute graceful cleanup.

**Implication:** After a crash (no graceful shutdown), tasks can remain stuck in REVIEWING until the next reviewer wait-loop iteration clears stale claims.

**Current mitigation:** Three clearing mechanisms now exist: (a) reviewer registration (`registration.go`) auto-clears stale claims on agent startup, (b) `liza clear-stale-review-claims` command for manual invocation, and (c) reviewer wait-loop (`waitforwork.go`) calls `ops.ClearStaleReviewClaims` before each poll iteration for all reviewer roles (code-reviewer, code-plan-reviewer, epic-plan-reviewer, us-reviewer). Signal handling (`SIGINT`/`SIGTERM`) triggers `unregisterAgent()` which atomically releases active review claims on graceful exit. The remaining gap is narrow: crash scenarios where no reviewer agent is running to execute the wait-loop clearing.

**Future options:**
- Add watcher-based automatic lease expiration (transition to READY_FOR_REVIEW on expiry)
- Include stale lease check in work detection diagnostics

---

## Blind Spots

Unacknowledged forces or gaps the system doesn't model.

### Contract Effectiveness Self-Certification

**Skill:** systemic-thinking
**Category:** BLIND SPOT

**Issue:** The contract's failure mode coverage is self-certified. The failure mode map (`CONTRACT_FAILURE_MODE_MAP.md`) claims 55/55 "Strong" coverage with 0 Partial and 0 Gap. This assessment is produced by the same process that writes the contract — there is no independent validation that clauses actually suppress the failure modes they claim to cover. The map references line numbers from a prior contract version ("Last updated: Contract v3 (882 lines)") while the current contract is 800 lines — every line reference is stale. The maintenance protocol ("check which failure modes the affected clause covers") is a process rule enforced by the same behavioral compliance the contract is designed to compensate for. There is no test suite, no simulation, no adversarial probing of whether the 55 coverage claims hold under context pressure, novel model versions, or multi-agent interaction.

**Implication:** The 55/55 coverage claim provides confidence without evidence — the map may be accurate, or it may be a snapshot of aspirational intent that has drifted from reality as the contract evolved.

**Future options:**
- Adversarial testing: deliberately trigger each failure mode and verify the contract suppresses it
- Automated line-number maintenance (extract clause IDs instead of line numbers)
- Periodic red-team exercises using the failure mode map as a checklist

### Initialization Completion Unverifiable

**Skill:** systemic-thinking
**Category:** BLIND SPOT

**Issue:** The contract requires a complex initialization sequence: mode detection → read mode contract → read project files → build 6 mental models → role-specific initialization. Completion of this sequence is entirely self-reported. There is no structural verification that an agent actually read what it was supposed to read, built the models it was supposed to build, or internalized the constraints. In multi-agent mode, the supervisor verifies agent registration (identity, lease) but not contract compliance. An agent that skips initialization or partially completes it enters the same state machine as a fully initialized agent. The "compaction checkpoint" and "working set" mechanisms handle mid-session degradation but assume initialization was complete — if it wasn't, the agent starts in a degraded state without any detection signal.

**Implication:** Contract compliance depends on a bootstrap sequence that cannot be verified from outside the agent — a model that partially follows initialization instructions produces no observable difference from one that follows them completely, until a violation occurs.

**Future options:**
- Initialization checklist emitted as structured output (supervisor verifies before accepting agent as ready)
- Canary questions: supervisor tests agent's knowledge of key contract clauses before allowing work
- Reduce initialization surface by embedding more rules in supervisor-enforced structural mechanisms

### No Source Type for Pre-Implementation Spec Findings

**Skill:** systemic-thinking
**Category:** BLIND SPOT

**Issue:** The `discovered` section in `state.yaml` tracks findings logged by agents during work. The `source` field documents who/what produced the finding: `null` (implementation discovery by Coder) or `systemic-thinking` (analytical finding by Code Reviewer). There is no source value for findings produced during specification authoring—ambiguous requirements, conflicting constraints in vision docs, or SMART criteria violations identified before implementation begins. Note: the source taxonomy is documented but not enforced at runtime — `models/state.go` and `validate.go` do not reject arbitrary source values, so a spec-authoring agent could technically use any string. The gap is in the documented taxonomy and semantic clarity, not structural enforcement.

**Implication:** Specification-quality issues discovered by a spec-authoring agent would need to use `null` (misleading—implies Coder found it) or `systemic-thinking` (misleading—implies analytical review of existing code), or use an undocumented ad hoc value that other agents won't recognize. The discovery taxonomy cannot represent "this finding blocks the PRD, not the implementation."

**Current mitigation:** Spec-quality issues are assumed to be resolved by humans before the blackboard is initialized.

**Future options:**
- Add `spec-authoring` or `prd-validation` as valid `source` values
- Add `urgency: blocks_spec` to distinguish spec-blockers from implementation-blockers
- Track spec-production work separately from implementation tasks

### Prompt-Build-to-Execution State Drift

**Skill:** systemic-thinking
**Category:** BLIND SPOT

**Issue:** The supervisor builds and saves the prompt file (`agent/supervisor.go:250-259`) before executing the agent. The prompt is constructed from state read at claim time, but the agent execution happens in a separate subprocess that may read different state. There's no mechanism to ensure the prompt content remains consistent with the state the agent actually operates on.

**Implication:** When debugging failures, the saved prompt may not represent the actual state the agent operated on, making post-hoc analysis less reliable. However, since agents read live state via MCP tools during execution (not from the prompt file), the prompt is an initial context artifact, not the runtime truth — the actual impact on agent behavior is low.

**Current mitigation:** Agents read current state via MCP/tools during execution, not from the prompt file. The prompt provides initial context and orientation but is not authoritative for state-dependent decisions. Prompts are timestamped and saved for debugging.

**Future options:**
- Include state version/checksum in prompt header for comparison
- Snapshot state.yaml at prompt build time alongside the prompt
- Add prompt-state consistency verification to post-execution diagnostics

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

### Role Addition Accelerates Contract Complexity Pressure

**Skill:** systemic-thinking
**Category:** TRAJECTORY
**Related:** [Contract Complexity vs Context Pressure](#contract-complexity-vs-context-pressure)

**Issue:** The "Contract Complexity vs Context Pressure" feedback loop (documented in this file) notes that CORE.md is 800 lines and each new clause consumes context agents need for work. Adding a fourth role requires: role definition in `roles.md`, SKILL.md for the role, task type workflow extension, and initialization sequence updates. The contract is the mechanism that suppresses agent failure modes, but the safety margin shrinks with each role added.

**Implication:** Each new role added to the system brings the contract closer to the ceiling where "adding another clause to prevent failure mode N+1 degrades compliance with clauses 1 through N." The tier architecture and kernel appendix handle mid-session degradation, but initialization grows monotonically.

**Current mitigation:** None structural; the existing feedback loop documents the concern but offers no relief mechanism.

**Future options:**
- Conditional contract loading (only load role-relevant sections)
- Structural enforcement replacing behavioral rules (more logic in Go, less in contract)
- Measure contract-to-work ratio empirically before adding roles

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

### Spec Corpus Lacks Lifecycle Management

**Skill:** systemic-thinking
**Category:** TRAJECTORY

**Issue:** The spec-first design requires specifications before implementation, blocks work on missing specs, and logs spec changes to the blackboard. But the specification corpus has no lifecycle management. Specs are created, updated, and appended to — never deprecated, archived, or retired. `spec-mapping.yaml` is already 59KB for a single project (Liza itself). Over multiple sprints and goals, the spec corpus grows monotonically. Agents must read relevant specs on session initialization; as specs accumulate, "relevant" becomes harder to determine and the read cost grows. There is no mechanism to mark a spec as superseded by implementation, no staleness detection for specs whose referent code has changed, and no pruning trigger in the sprint governance protocol.

**Implication:** For long-lived projects, the spec corpus becomes a maintenance burden that consumes human bandwidth proportional to project age — the opposite of the "reduce human workload" goal.

**Future options:**
- Spec status field (active, superseded, archived) with archival workflow
- Staleness detection: flag specs not referenced by any task in N sprints
- Spec pruning as part of sprint retrospective checklist
- Hierarchical spec organization with summary documents to reduce agent read cost

### Metrics Collection Without Query Interface

**Skill:** systemic-thinking
**Category:** TRAJECTORY

**Issue:** The system collects rich metrics—file lock timing (`filelock/metrics.go`), sprint metrics (`models/state.go:406-420`), diagnostic data (`models/diagnostics.go`)—but there's no unified query layer to access them. The MCP server exposes individual tools for specific queries, but operators cannot ask "show me agent performance over time" or "what's the current lock contention rate?" without writing custom code.

**Implication:** Operational visibility requires ad hoc tooling or direct state.yaml inspection. The investment in metrics instrumentation doesn't translate to operational insight because the data is fragmented and inaccessible through standard interfaces.

**Current mitigation:** Individual metrics are accessible via specific commands (`liza inspect metrics`, MCP tools). Sprint metrics visible in `liza status`.

**Future options:**
- Unified query interface aggregating all metric sources
- Time-series storage for historical metric analysis
- Dashboard generation from collected metrics

### No Query Layer

**Skill:** systemic-thinking
**Category:** TRAJECTORY
**Coupled with:** [MCP Cross-Layer Read Dependency](#mcp-cross-layer-read-dependency)

**Issue:** The system has a clear mutation layer (`ops`) but no query layer. Complex read operations are implemented wherever first needed: `models` has `FindTask`, `IsClaimable`, `CountClaimableTasks`, `AllPlannedTasksTerminal`; `agent/workdetection.go` has `DetectOrchestratorWakeTriggers`; `commands` has `InspectCommand` (parametric query with format control), `StatusCommand` (dashboard aggregation), `ValidateCommand` (invariant checking); `db` has `Read`, `ReadCached`, `ReadRaw`, `GetTask`, `GetAgent`. The pattern: each package implements the queries it needs, and cross-package query reuse happens through the wrong seam (`mcp` → `commands`). The `models/diagnostics.go` file represents a partial move toward a query layer (work detection functions extracted from agent), but the extraction stopped there. The three consumers (CLI, MCP, agent supervisor) each need overlapping but different views of state — formatted text, structured JSON, and in-memory assessment — but share no query infrastructure.

**Implication:** As the system's query surface grows (new MCP resources, dashboard enhancements, diagnostic tools), either the `mcp` → `commands` dependency deepens or query logic gets duplicated across consumers.

**Future options:**
- Extract query functions to `ops` or a new `queries` package returning structured data (each presentation layer formats independently)
- Promote `models/diagnostics.go` as the canonical query home and migrate state queries from `commands` and `agent`
- Accept `commands` as the shared query+formatting layer and document or rename to reflect its dual role

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

### Orchestrator as Single Semantic Interpreter

**Skill:** systemic-thinking
**Category:** LOAD-BEARING

**Issue:** Orchestrator carries the entire semantic burden. It decomposes goals, interprets failure signals, resolves blocked reviews, converts discoveries to tasks, and maintains goal alignment. All other roles execute mechanical functions (implement spec, validate against spec) while Orchestrator alone interprets intent. No second opinion, no validation mechanism, no structural redundancy.

**Implication:** Orchestrator drift compounds silently across all tasks until human checkpoint reveals accumulated misalignment. Correction costs scale with drift duration.

**Current mitigation:** Human checkpoints provide periodic correction opportunities.

**Future options:**
- Orchestrator self-review before finalizing task decomposition
- Second Orchestrator instance for cross-validation on critical decisions
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

### Spec Completeness vs Reality

**Skill:** systemic-thinking
**Category:** TENSION

**Issue:** The vision positions specs as the mechanism for context survival ("if it's not written down, it doesn't exist") while stating "Liza v1 assumes specs substantially complete before work" and excluding "domains where requirements emerge through implementation."

Incomplete specs—normal in real projects—trigger a reinforcing loop: coders block on spec gaps, Orchestrator logs spec_gap anomalies, human must update specs, system pauses. The spec-first design shifts work from agents to humans while promising to reduce human workload.

**Implication:** System selects for a narrow project profile (complete specs, solo developers) rather than adapting to common project conditions.

**Current mitigation:** BLOCKED resolution via `human_notes`. Orchestrator reads human_notes on wake.

**Future options:**
- Spike mode for spec discovery
- Orchestrator-assisted spec drafting from coder discoveries
- Graceful degradation when specs incomplete (proceed with explicit assumptions)

### Code Reviewer Structural Accountability Gap

**Skill:** systemic-thinking
**Category:** TENSION

**Issue:** The Code Reviewer has binding approval/rejection authority but no structural accountability for verdict quality. The contract specifies detection of reviewer dysfunction in two modes: rubber-stamping (>95% approval rate metric, `MULTI_AGENT_MODE.md`) and abandonment (review exhaustion — 2 reviewers exit without verdict). However, these are contract-specified behaviors, not structurally enforced in the supervisor flow — the supervisor does not compute approval rates or detect review exhaustion patterns at runtime. The system cannot detect a third, more damaging mode: incorrect verdicts with plausible reasoning. A reviewer that rejects valid work forces full implement-review cycles before the Orchestrator evaluates (governed by `effectiveCoderIterationLimit` and `effectiveReviewCycleLimit` in `iteration_limits.go`), and the Orchestrator's assessment is itself the unvalidated judgment of the single semantic interpreter. A reviewer that approves flawed work is invisible unless integration tests catch it — but the system doesn't mandate integration tests on the integration branch. The power asymmetry is structural: Coders must address every rejection point-by-point, but there's no mechanism for Coders to challenge a rejection except by re-implementing and re-submitting. Note: with current LLM-based reviewers, over-rejection (spurious rejections with plausible reasoning) is the empirically dominant failure mode, making the iteration limit the most exercised circuit breaker in practice.

**Implication:** Code review quality is the least observable dimension of system health, yet it gates all task completion — the system optimizes for reviewer throughput signals while reviewer accuracy remains unmeasured.

**Future options:**
- Reviewer accuracy metric (compare rejected items against final merged state)
- Coder appeal mechanism (structured objection triggers Orchestrator evaluation before 5 cycles)
- Post-merge validation on integration branch (automated tests catch reviewer misses)

**Decision:** Integration Reviewer role. Would also catch incompatible changes made within the various merged worktrees.

### Hypothesis Exhaustion Without Root Cause

**Skill:** systemic-thinking
**Category:** FEEDBACK

**Issue:** Hypothesis exhaustion rule (two coders fail = must rescope) forces Orchestrator intervention but doesn't require root cause identification. Orchestrator may split task-3 into task-3a/task-3b without diagnosing why two coders failed. If underlying cause is spec ambiguity or architecture flaw, new tasks encounter same obstacle.

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

### Supervisor Contention

**Skill:** systemic-thinking
**Category:** STRESS POINT

**Issue:** Supervisor-only worktree creation and claim handling centralize concurrency control and state transitions. All contention and race resolution concentrated in single process. Coders and Reviewers fully dependent on its throughput and correctness.

**Implication:** Supervisor contention becomes primary bottleneck when scaling beyond small task counts.

**Future options:**
- Partition by task ID for parallel claim handling
- Optimistic claiming with conflict resolution
- Dedicated claim coordinator separate from agent supervisor

### Dual Contract Delivery Paths

**Skill:** systemic-thinking
**Category:** FRAGILITY

**Issue:** Contracts reach agents through two paths: symlinks from repo root (development: `CLAUDE.md → ~/.liza/CORE.md → contracts/CORE.md`) and installed copies (`liza setup` writes to `~/.liza/`). Changes to contracts in the repo don't propagate to installed copies until `liza setup --force` is run. The Go binary embeds contracts at build time (`internal/embedded/`); installed copies are from the last `setup` run; symlinks resolve at read time. A running system can have three contract versions active simultaneously: the embedded version (used by prompt templates), the installed version (in `~/.liza/`), and the repo version (via symlinks). `liza setup` writes version metadata into installed contracts, providing partial version tracking, but there is no compatibility check between binary version and installed contract version, and `state.yaml`'s `version: 1` field is inert. `liza validate` validates state schema, not contract consistency. Note: agent prompts are built from Go templates (`internal/prompts/templates/`), not from embedded contract markdown directly — the embedded copies serve `liza setup`, not runtime prompt construction.

**Implication:** Contract drift between delivery paths is silent — agents may operate under different behavioral rules than the system operator believes are active, with no error signal.

**Future options:**
- Content hash in contract files, verified at agent startup
- `liza validate` checks embedded vs installed contract consistency
- Single delivery path (eliminate duplication, choose symlinks or embedding)

---

## Completed Fixes

---

## Fixed (Traceability)

---

## Fix Details
