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
  - [Mode Selection Trigger Coupled to Prompt Lexeme](#mode-selection-trigger-coupled-to-prompt-lexeme)
- [Systemic Tensions](#systemic-tensions)
  - [Spec Completeness vs Reality](#spec-completeness-vs-reality)
  - [Code Reviewer Structural Accountability Gap](#code-reviewer-structural-accountability-gap)
  - [Two-Track State Mutation](#two-track-state-mutation)
  - [MCP Cross-Layer Read Dependency](#mcp-cross-layer-read-dependency)
  - [Role-Boundary Severity Drift](#role-boundary-severity-drift)
  - [Reviewer Role Namespace Fragmentation](#reviewer-role-namespace-fragmentation)
  - [Merge Execution Authority Split](#merge-execution-authority-split)
  - [Lease Duration Default Split](#lease-duration-default-split)
  - [Sprint Completion Signal Diverges from Active Scope](#sprint-completion-signal-diverges-from-active-scope)
  - [Task Type Registry Only Supports Coding Workflows](#task-type-registry-only-supports-coding-workflows)
- [Feedback Loops](#feedback-loops)
  - [Hypothesis Exhaustion Without Root Cause](#hypothesis-exhaustion-without-root-cause)
  - [Restart/Lease Churn Under Load](#restartlease-churn-under-load)
  - [Supervisor Wait-Claim-Spawn Loop](#supervisor-wait-claim-spawn-loop)
  - [Contract Complexity vs Context Pressure](#contract-complexity-vs-context-pressure)
  - [Issue Registry Resolution Drift](#issue-registry-resolution-drift)
- [Assumptions](#assumptions)
  - [Human Availability as Bottleneck](#human-availability-as-bottleneck)
  - [Implicit Planner Provenance Default](#implicit-planner-provenance-default)
  - [Spec Maturity Dependency](#spec-maturity-dependency)
  - [Well-Formed Blackboard State](#well-formed-blackboard-state)
  - [Single-Goal Data Model Constrains Applicability](#single-goal-data-model-constrains-applicability)
  - [Planner State Change Verification is Non-Binding](#planner-state-change-verification-is-non-binding)
- [Stress Points](#stress-points)
  - [Supervisor Contention](#supervisor-contention)
  - [Validation Integrity Split by Ingress](#validation-integrity-split-by-ingress)
  - [Filesystem/Git I/O Contention](#filesystemgit-io-contention)
  - [Exit Code 42 Restart Loop Without Progress Detection](#exit-code-42-restart-loop-without-progress-detection)
  - [Cache Coherence Gap in Multi-Process Deployments](#cache-coherence-gap-in-multi-process-deployments)
- [Cascade](#cascade)
  - [Integration Test Script Silent Absence](#integration-test-script-silent-absence)
- [Fragility](#fragility)
  - [Cross-Script State Mutation](#cross-script-state-mutation)
  - [Dual Contract Delivery Paths](#dual-contract-delivery-paths)
  - [Bootstrap Artifact Path Drift](#bootstrap-artifact-path-drift)
  - [File-Based Spec References Without Version Anchors](#file-based-spec-references-without-version-anchors)
  - [MCP Tool Schema Drift](#mcp-tool-schema-drift)
  - [Review Lease Orphaning Without Automatic Reclamation](#review-lease-orphaning-without-automatic-reclamation)
- [Blind Spots](#blind-spots)
  - [Contract Effectiveness Self-Certification](#contract-effectiveness-self-certification)
  - [Initialization Completion Unverifiable](#initialization-completion-unverifiable)
  - [Planner Role Invisible in Type System](#planner-role-invisible-in-type-system)
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

### Code Reviewer Structural Accountability Gap

**Skill:** systemic-thinking
**Category:** TENSION

**Issue:** The Code Reviewer has binding approval/rejection authority but no structural accountability for verdict quality. The contract specifies detection of reviewer dysfunction in two modes: rubber-stamping (>95% approval rate metric, `MULTI_AGENT_MODE.md`) and abandonment (review exhaustion — 2 reviewers exit without verdict). However, these are contract-specified behaviors, not structurally enforced in the supervisor flow — the supervisor does not compute approval rates or detect review exhaustion patterns at runtime. The system cannot detect a third, more damaging mode: incorrect verdicts with plausible reasoning. A reviewer that rejects valid work forces full implement-review cycles before the Planner evaluates (governed by `effectiveCoderIterationLimit` and `effectiveReviewCycleLimit` in `iteration_limits.go`), and the Planner's assessment is itself the unvalidated judgment of the single semantic interpreter. A reviewer that approves flawed work is invisible unless integration tests catch it — but the system doesn't mandate integration tests on the integration branch. The power asymmetry is structural: Coders must address every rejection point-by-point, but there's no mechanism for Coders to challenge a rejection except by re-implementing and re-submitting. Note: with current LLM-based reviewers, over-rejection (spurious rejections with plausible reasoning) is the empirically dominant failure mode, making the iteration limit the most exercised circuit breaker in practice.

**Implication:** Code review quality is the least observable dimension of system health, yet it gates all task completion — the system optimizes for reviewer throughput signals while reviewer accuracy remains unmeasured.

**Future options:**
- Reviewer accuracy metric (compare rejected items against final merged state)
- Coder appeal mechanism (structured objection triggers Planner evaluation before 5 cycles)
- Post-merge validation on integration branch (automated tests catch reviewer misses)

### Two-Track State Mutation

**Skill:** systemic-thinking
**Category:** TENSION

**Issue:** The `ops` extraction that resolved "Commands Presentation+Logic Coupling" was structurally incomplete. All task lifecycle mutations from CLI and MCP consumers route through `ops` — a clean business logic layer with typed inputs, structured results, and three-phase validation for claiming. But the `agent` package — the third and most critical consumer — mutates both task and agent state directly via `bb.Modify` in `claimReviewerTask`, `resumeHandoffTask`, `registerAgent`, `resetAgentAfterExit`, and `setAgentToPlanningStatus`. This creates two mutation tracks: `ops` (validated, structured, reusable across CLI/MCP/agent) and `agent` (inline closures, only callable from the supervisor). The reviewer claiming path is the most consequential — it transitions task status, sets `reviewing_by`, updates agent state, and captures return values via closure variables, all in a single `Modify` closure with no structured result type and no way for MCP or CLI to invoke the same logic.

**Implication:** The ops layer promises to be the single source of truth for state mutations, but reviewer claiming, handoff resumption, and all agent lifecycle management are structurally unreachable from non-supervisor consumers, and changes to claiming semantics must be updated in two different architectural layers with different patterns.

**Current mitigation:** Reviewer claiming is simpler than coder claiming (no worktree creation, no three-phase pattern needed), so the complexity gap hasn't caused bugs yet.

**Future options:**
- Extract `ops.ClaimReviewerTask` and `ops.ResumeHandoff` with structured result types
- Extract `ops.RegisterAgent` / `ops.UnregisterAgent` for agent lifecycle
- Accept the split as intentional: ops owns task lifecycle, agent owns agent lifecycle and reviewer claims (document the boundary)

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

### ~~Reviewer Role Namespace Fragmentation~~

**Skill:** systemic-thinking
**Category:** TENSION
**Status:** RESOLVED (`a60c72e`)

**Issue:** Internal role semantics for the reviewer were encoded as three incompatible namespace forms across internal boundaries: `code_reviewer` in task workflow typing, `code-reviewer` in runtime supervisor/agent role handling, and `reviewer` in claim-release mutation interfaces. These were bridged by implicit string translation rather than a declared canonical mapping.

**Fix:** Created `internal/roles` package with unified constants and explicit mapping:
- Runtime constants: `RuntimeCoder`, `RuntimeCodeReviewer`, `RuntimePlanner`
- Workflow constants: `WorkflowCoder`, `WorkflowCodeReviewer`
- Mapping functions: `ToWorkflow()`, `ToRuntime()`, validation helpers `IsValidRuntime()`, `IsValidWorkflow()`
- All agent/, cmd/, and ops/ production code migrated to use role constants
- Comprehensive tests (253 LOC) cover all mappings, validation, and list functions

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

### Lease Duration Default Split

**Skill:** systemic-thinking
**Category:** TENSION

**Issue:** The blackboard schema document contradicts itself internally: the YAML example declares `config.lease_duration: 1800` (30 minutes), while prose in the same document states "Lease duration: 300 seconds (5 minutes)" and "default: 5 minutes". The runtime code uses 1800 (`DefaultLeaseDurationSeconds` in `models/state.go`, passed through `supervisor.go` and `claimReviewerTask`), so the actual system behavior matches the YAML example, not the prose.

**Implication:** Readers who follow the prose (5-minute model) will have incorrect expectations about lease expiry timing. Operational procedures referencing "5 minutes" describe behavior that doesn't match the code.

**Current mitigation:** None — the schema prose and the code disagree, and the prose is what operators read.

**Future options:**
- Define one canonical lease default source and generate docs from it
- Surface active lease config in runtime status output for operator confirmation
- Add docs consistency checks for duplicated default-value declarations

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

### Implicit Planner Provenance Default

**Skill:** systemic-thinking
**Category:** ASSUMPTION

**Issue:** Task-creation provenance assumes a synthetic planner identity when none is provided. Both MCP `handleAddTask` and `ops.AddTask` default missing agent identity to `planner-1`, so write attribution can be generated without proving who initiated the mutation.

**Implication:** Multi-planner operation collapses to a synthetic single actor in audit trails, reducing accountability and weakening post-incident reconstruction of planning decisions.

**Current mitigation:** CLI and MCP can provide explicit `agent_id`, but omission silently falls back to the default identity.

**Future options:**
- Make planner identity mandatory for task-creation mutations
- Distinguish system-authored vs agent-authored mutations with explicit provenance fields
- Add validation rejecting task-creation events with defaulted identity in multi-planner mode

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

### Planner State Change Verification is Non-Binding

**Skill:** systemic-thinking
**Category:** ASSUMPTION

**Issue:** After planner execution completes, `verifyPlannerStateChanges()` (`agent/systemctl.go`) logs a warning if expected state changes weren't made, but takes no corrective action. The supervisor continues the loop. However, the planner re-invocation depends on `waitForWork` → `DetectPlannerWakeTriggers()`, which checks actual state conditions (unassigned tasks, anomalies, blocked tasks, etc.). If no wake triggers exist, the planner waits indefinitely rather than looping. The infinite loop scenario requires persistent wake triggers that the planner fails to resolve — e.g., a blocked task the planner cannot unblock, or an anomaly it cannot interpret.

**Implication:** The system assumes planners will eventually resolve wake triggers. A planner stuck on an unresolvable trigger (spec ambiguity it cannot bypass, anomaly pattern it cannot interpret) will repeatedly execute without triggering escalation, consuming API tokens and time without progress signals. The failure mode is narrower than "any stuck planner" but still lacks detection.

**Current mitigation:** None explicit. Circuit breaker anomaly patterns may eventually detect the planner loop, but only if anomalies are logged.

**Future options:**
- Escalate to human after N consecutive planner executions without state change
- Require planner to document progress or blocking reason on each wake
- Add planner-specific circuit breaker for no-op execution patterns

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

### Validation Integrity Split by Ingress

**Skill:** systemic-thinking
**Category:** STRESS POINT

**Issue:** Equivalent task-creation mutations do not share equivalent validation pressure by interface. CLI `add-task` executes post-mutation `ValidateCommand`, while MCP `liza_add_task` persists through `ops.AddTask` and returns without that same immediate full-state validation pass. Under scale or automation-heavy MCP usage, validation load shifts from write-time gating to later detection.

**Implication:** State consistency risk concentrates at the highest-throughput ingress, making failures surface later and farther from the originating mutation.

**Current mitigation:** Explicit `liza_validate` calls and watchdog workflows can catch invalid state after the fact.

**Future options:**
- Move mandatory post-write validation into shared ops mutation flow
- Add MCP-side atomic mutate+validate command variants for write operations
- Treat write-without-validation as an explicit mode with telemetry and alerts

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

### Exit Code 42 Restart Loop Without Progress Detection

**Skill:** systemic-thinking
**Category:** STRESS POINT

**Issue:** The supervisor loop (`agent/supervisor.go:293-298`) treats exit code 42 as a graceful restart with a fixed 2-second sleep, with no tracking of restart frequency or progress verification. An agent that repeatedly encounters context pressure and self-aborts will restart indefinitely without triggering circuit breaker patterns or human escalation.

**Implication:** A misconfigured task or environment issue that causes consistent context-exhaustion aborts creates a busy-wait loop consuming compute resources and log volume while making no progress, with no automatic detection or backpressure.

**Current mitigation:** Exit code 42 is intended for context exhaustion where agent believes restart with fresh context will help. No tracking exists to detect when it doesn't.

**Future options:**
- Track restart count per task and escalate after N restarts without progress
- Exponential backoff on repeated exit 42 (2s, 4s, 8s, ... up to max)
- Circuit breaker pattern for exit 42 clusters on same task

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

## Cascade

Failure propagation paths and silent bypass patterns.

### Integration Test Script Silent Absence

**Skill:** systemic-thinking
**Category:** CASCADE

**Issue:** The merge operation (`ops/wt_merge.go:272-308`) checks for `scripts/integration-test.sh` and runs it if present, but silently skips testing if the file doesn't exist. There's no warning, metric, or audit trail that a merge proceeded without validation. The `MergeResult.TestsRan` boolean captures this but it's only visible in the immediate result, not in persistent state or history.

**Implication:** Accidental deletion or renaming of the integration test script will not be detected—merges will appear successful while bypassing quality gates, allowing regressions to reach the integration branch without any systemic signal.

**Current mitigation:** Operators can check `TestsRan` in merge output. The integration test script must be created manually or by project setup — `liza init` does not create it.

**Future options:**
- Require explicit opt-out (flag or config) to merge without tests
- Log warning when merge proceeds without integration tests
- Include `tests_ran` in task history for audit trail

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

### Dual Contract Delivery Paths

**Skill:** systemic-thinking
**Category:** FRAGILITY

**Issue:** Contracts reach agents through two paths: symlinks from repo root (development: `CLAUDE.md → ~/.liza/CORE.md → contracts/CORE.md`) and installed copies (`liza setup` writes to `~/.liza/`). Changes to contracts in the repo don't propagate to installed copies until `liza setup --force` is run. The Go binary embeds contracts at build time (`internal/embedded/`); installed copies are from the last `setup` run; symlinks resolve at read time. A running system can have three contract versions active simultaneously: the embedded version (used by prompt templates), the installed version (in `~/.liza/`), and the repo version (via symlinks). `liza setup` writes version metadata into installed contracts, providing partial version tracking, but there is no compatibility check between binary version and installed contract version, and `state.yaml`'s `version: 1` field is inert. `liza validate` validates state schema, not contract consistency. Note: agent prompts are built from Go templates (`internal/prompts/templates/`), not from embedded contract markdown directly — the embedded copies serve `liza setup`, not runtime prompt construction.

**Implication:** Contract drift between delivery paths is silent — agents may operate under different behavioral rules than the system operator believes are active, with no error signal.

**Future options:**
- Content hash in contract files, verified at agent startup
- `liza validate` checks embedded vs installed contract consistency
- Single delivery path (eliminate duplication, choose symlinks or embedding)

### MCP Tool Schema Drift

**Skill:** systemic-thinking
**Category:** FRAGILITY

**Issue:** Each of the ~20 MCP tools is registered with an `InputSchema` declaring required fields, types, defaults, and enum constraints. These schemas are the agent-facing contract — agents decide which parameters to provide based on schema declarations. The schemas are hand-coded in `server.go` registration calls with no connection to the corresponding `ops.*` function signatures or input types. A schema declaring a field as required while the ops function derives it internally has already occurred in `submit-for-review` and later regressed, demonstrating recurrence risk. There is no compile-time verification, no test that round-trips schema declarations against handler parameter extraction, and no generated schema. Each tool registration is an independent manual synchronization point between three artifacts: the `InputSchema`, the handler's `requireString`/parameter extraction, and the `ops.*` function's actual parameters.

**Implication:** Schema-to-implementation drift is a per-tool risk that scales linearly with tool count, and the system's own history demonstrates this failure mode has already occurred.

**Current mitigation:** Handler functions extract parameters with `requireString` which fails fast on missing fields. MCP dispatch tests cover routing but not schema-to-handler consistency.

**Future options:**
- Generate `InputSchema` from `ops.*Input` struct tags (single source of truth)
- Test that each tool's declared required fields match its handler's `requireString` calls
- Schema validation middleware that rejects calls not matching declared schema before handler invocation

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

**Implication:** Spec drift during task execution is undetectable. A PRD produced by a spec-authoring agent and consumed by the Planner can change between when the Planner decomposes it and when the Coder implements the resulting tasks. The blackboard's `spec_changes` log tracks that changes occurred, not which tasks were affected by which changes.

**Current mitigation:** Code Reviewer validates against "current spec version" and logs `spec_changed` anomaly if material changes detected.

**Future options:**
- Include commit SHA or content hash in `spec_ref`
- Track `spec_version` at task creation and warn on divergence
- Generate spec snapshots when tasks are created

### Review Lease Orphaning Without Automatic Reclamation

**Skill:** systemic-thinking
**Category:** FRAGILITY

**Issue:** Review leases expire based on `review_lease_expires` timestamp. Stale leases are cleared in two situations: (a) reviewer registration (`registration.go`) auto-clears stale claims on agent startup, and (b) the `clear-stale-review-claims` command can be invoked manually. However, there is no periodic in-loop reclamation — if no new reviewer registers and the command isn't invoked, stale REVIEWING tasks remain stuck. The supervisor's `claimReviewerTask` only considers READY_FOR_REVIEW tasks, not stale REVIEWING leases.

**Implication:** Between reviewer agent restarts, tasks can remain stuck in REVIEWING with an expired lease. The gap is not "no mechanism exists" but "no periodic mechanism" — recovery depends on a reviewer agent restarting or manual intervention.

**Current mitigation:** Reviewer registration auto-clears stale claims on startup. `liza clear-stale-review-claims` command available for manual or automated invocation. Signal handling (`SIGINT`/`SIGTERM`) triggers `unregisterAgent()` which atomically releases active review claims on graceful exit — tasks return to READY_FOR_REVIEW immediately rather than waiting for lease expiry. The remaining gap is crash scenarios where the agent cannot execute cleanup.

**Future options:**
- Supervisor periodically runs stale claim clearing before claiming
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

### Planner Role Invisible in Type System

**Skill:** systemic-thinking
**Category:** BLIND SPOT

**Issue:** The planner is identified as the "Single Semantic Interpreter" in this document — the most structurally critical role. Yet it is the only role absent from the type system. The task workflow registry (`taskWorkflows` in `models/state.go`) declares `{coding: [coder, code_reviewer]}`; the planner doesn't appear. Its behavioral rules are distributed implicitly across four files in the `agent` package: infinite wait time override in `waitforwork.go` (`365 * 24 * time.Hour`), wake trigger detection in `workdetection.go` (priority-ordered state inspection), pseudo-task creation in `supervisor.go` (sets `CurrentTask` to string literal `"planning"`), and post-execution state verification in `systemctl.go`. None of these rules reference a declarative definition. `models.RoleCoder` and `models.RoleCodeReviewer` constants exist but there is no `models.RolePlanner`. The agent identity validation in `registration.go` accepts any `{role}-{number}` format — `planner-1` is valid by string convention, not type constraint.

**Implication:** Adding a second coordinator role (architect, integrator) requires discovering and replicating the planner's implicit behavioral conventions by reading Go control flow, rather than extending a declaration — the most critical role is the least formally defined.

**Future options:**
- Add `models.RolePlanner` constant and planner-specific type declarations
- Declare planner wake triggers as data (trigger type → state predicate map) rather than imperative code
- Extract planner behavioral rules from agent package into a declarative configuration consumed by the supervisor

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

**Issue:** The system has a clear mutation layer (`ops`) but no query layer. Complex read operations are implemented wherever first needed: `models` has `FindTask`, `IsClaimable`, `CountClaimableTasks`, `AllPlannedTasksTerminal`; `agent/workdetection.go` has `DetectPlannerWakeTriggers`; `commands` has `InspectCommand` (parametric query with format control), `StatusCommand` (dashboard aggregation), `ValidateCommand` (invariant checking); `db` has `Read`, `ReadCached`, `ReadRaw`, `GetTask`, `GetAgent`. The pattern: each package implements the queries it needs, and cross-package query reuse happens through the wrong seam (`mcp` → `commands`). The `models/diagnostics.go` file represents a partial move toward a query layer (work detection functions extracted from agent), but the extraction stopped there. The three consumers (CLI, MCP, agent supervisor) each need overlapping but different views of state — formatted text, structured JSON, and in-memory assessment — but share no query infrastructure.

**Implication:** As the system's query surface grows (new MCP resources, dashboard enhancements, diagnostic tools), either the `mcp` → `commands` dependency deepens or query logic gets duplicated across consumers.

**Future options:**
- Extract query functions to `ops` or a new `queries` package returning structured data (each presentation layer formats independently)
- Promote `models/diagnostics.go` as the canonical query home and migrate state queries from `commands` and `agent`
- Accept `commands` as the shared query+formatting layer and document or rename to reflect its dual role

---


## Code-Level Architectural Smells

Issues identified through code-level architectural analysis (patterns, structure, duplication).

### ~~Interactive Stdin in Library Packages~~

**Skill:** software-architecture-review
**Category:** Untestable by design
**Status:** RESOLVED (`7a5e79c`)

**Issue:** Direct `os.Stdin` reads via `bufio.NewReader(os.Stdin)` or `bufio.NewScanner(os.Stdin)` in 8 locations across 5 files in 2 packages (`embedded/embedded.go`, `commands/setup.go`, `commands/init.go`, `commands/delete_task.go`, `commands/delete_agent.go`).

**Fix:** All 8 locations now accept an `io.Reader` parameter, defaulting to `os.Stdin` when nil (CLI behavior unchanged). `cmd/liza/main.go` passes `os.Stdin` at call sites. Tests use `strings.NewReader` for mock input — the `os.Stdin` monkey-patching pattern (`os.Stdin = r` / `defer`) is fully eliminated. `withMockStdin` helper removed.


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
- [x] MCP parse-error response write failure ignored — made `WriteError` failure terminal; `Run` returns error instead of silently continuing *(architecture-review)*
- [x] `submit-for-review` `commit_sha` contract drift — aligned contract in `d4c688e`; **REGRESSED**: caller-provided SHA is currently required again in CLI/MCP and ops surfaces *(architecture-review)*
- [x] REJECTED reassignment can orphan worktree on recreate failure — reordered reassignment to secure replacement before teardown with compensating recovery *(architecture-review)*
- [x] Reviewer Role Namespace Fragmentation — `internal/roles` package with unified constants and explicit `ToWorkflow()`/`ToRuntime()` mapping *(systemic-thinking)*
- [x] Interactive Stdin in Library Packages — all 8 locations accept `io.Reader` parameter; `os.Stdin` monkey-patching eliminated *(software-architecture-review)*
- [x] Hardcoded `"task/"` branch prefix — `paths.TaskBranchPrefix` constant; all 7 production files migrated *(software-architecture-review)*
- [x] Role naming divergence — unified via `internal/roles` package *(software-architecture-review)*
- [x] Divergent GracePeriod values (60s vs 120s) — unified `models.LeaseExpiryGracePeriod` *(software-architecture-review)*
- [x] `ClaimTask` function complexity (265 LOC) — phase helpers extracted, `unmetDependencies()` shared *(software-architecture-review)*
- [x] `inspect_field.go` manual reflection — replaced with reflect-based YAML-tag walker *(software-architecture-review)*
- [x] `validate.validateAnomalies` at 13.3% coverage — targeted table-driven tests for all anomaly types *(software-architecture-review)*
- [x] `supervisor.resumeHandoffTask` at 11.4% coverage — success/failure/edge-case tests *(software-architecture-review)*
- [x] MCP stdio transport no frame-size guard — `MaxRequestSize` (10MB) with bounded read *(software-architecture-review)*
- [x] Watch stall detection parses YAML text directly — uses `log.GetLastTimestamp()` typed parser *(software-architecture-review)*
- [x] Watch/log O(n) growth paths — append-only writes + bounded tail-window reads *(software-architecture-review)*
- [x] `heartbeat_interval` config ignored — `NormalizeHeartbeatInterval()` with bounds validation *(software-architecture-review)*
- [x] Planner max-wait config ignored — planners respect configured value *(software-architecture-review)*
- [x] Stale-lock cleanup error discarded — propagated as `LockErrorFilesystem` *(software-architecture-review)*
- [x] `DeleteTask` side effects outpace state commit — git cleanup deferred to after state mutation *(software-architecture-review)*
- [x] `get config.*` projection drift — reflect-based walker discovers all YAML-tagged fields *(software-architecture-review)*

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
| MCP parse-error response write failure ignored | `80297b9` |
| `submit-for-review` `commit_sha` contract drift | `d4c688e` (regressed) |
| REJECTED reassignment can orphan worktree on recreate failure | `ccaf9b0` |
| Reviewer Role Namespace Fragmentation | `a60c72e` |
| Interactive Stdin in Library Packages | `7a5e79c` |
| Hardcoded `"task/"` branch prefix | `59a8e3e` |
| Role naming divergence | `a60c72e` |
| Divergent GracePeriod values | `b9f20ff` |
| `ClaimTask` function complexity | `e86abd4` |
| `inspect_field.go` manual reflection | `c4bd748` |
| `validate.validateAnomalies` low coverage | `d8533ab` |
| `supervisor.resumeHandoffTask` low coverage | `d8533ab` |
| MCP stdio transport no frame-size guard | `c2fe02b` |
| Watch stall detection YAML text parsing | `61b16d5` |
| Watch/log O(n) growth paths | `fe8de6b` |
| `heartbeat_interval` config ignored | `9e59acf` |
| Planner max-wait config ignored | `1d4f4f4` |
| Stale-lock cleanup error discarded | `729da05` |
| `DeleteTask` side effects outpace state commit | `7dd05ce` |
| `get config.*` projection drift | `c4bd748` |

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

### MCP Parse-Error Response Write Failure Ignored

**Skill:** architecture-review
**Category:** Error handling
**Status:** RESOLVED — parse-error write failure is now terminal

**Fix:** `internal/mcp/server.go` parse-error path now checks `WriteError` return value. If the transport write fails, `Run` returns the error immediately instead of silently continuing the server loop.

### `submit-for-review` `commit_sha` Contract Drift

**Skill:** architecture-review
**Category:** Data flow
**Status:** REGRESSED — caller-provided commit SHA is required again in CLI/MCP surfaces and in `ops.SubmitForReview`

**Fix:** `d4c688e` aligned contract and runtime semantics by removing caller-provided `commit_sha` from CLI/MCP surfaces and deriving the authoritative commit from worktree HEAD.

**Current state:** Internal surfaces now require and enforce caller-provided SHA again (`submit-for-review <task-id> <commit-sha>` in CLI, required `commit_sha` in MCP schema/handler, and empty/mismatch rejection in `ops.SubmitForReview`), restoring the same contract drift class.

### REJECTED Reassignment Can Orphan Worktree on Recreate Failure

**Skill:** architecture-review
**Category:** Documented smell
**Status:** RESOLVED — reassignment flow hardened

**Fix:** Reordered the different-coder REJECTED reassignment path in `internal/ops/claim_task.go` to secure the replacement worktree before tearing down the old one. If replacement creation fails, the old worktree is preserved and compensating recovery state is persisted.

---
