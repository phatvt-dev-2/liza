# Liza Competitive Survey — March 2026

## Landscape Overview

The multi-agent coding space has evolved rapidly since Liza's first release. The field now splits into
four distinct categories, each solving a different problem. Liza sits in a category of one.

**General-purpose agent orchestration frameworks** (CrewAI, LangGraph, AutoGen, Semantic Kernel) provide
building blocks for assembling multi-agent workflows across any domain. They optimize for flexibility and
developer experience. None address behavioral trust in software engineering.

**Software company simulators** (MetaGPT/MGX, ChatDev, AgentMesh) encode SOP-based pipelines that mimic
software teams: Product Manager → Architect → Engineer → QA. They optimize for structured output generation.
Trust is assumed through process compliance.

**Scheduler/runners** (Symphony, Paperclip) sit above coding agents and manage work dispatch, workspace
isolation, and project-level orchestration. They optimize for operational coordination. Trust is delegated
to whatever happens inside each agent session.

**Behavioral enforcement systems** (Liza). One entry. A hybrid hardened architecture: deterministic
Go supervisors enforce state transitions, role boundaries, merge authority, and TDD gates mechanically,
while LLM agents handle judgment under a behavioral contract addressing 55+ failure modes. Nine roles
across two pipeline phases (specification → coding), each organized as adversarial doer/reviewer pairs.
Optimizes for trust through mechanical constraint of agent failure modes.

---

## Direct Competitors

### MetaGPT / MGX

**What it is**: Python framework (64k+ stars) encoding standardized operating procedures into multi-agent
pipelines. Five fixed roles (Product Manager, Architect, Project Manager, Engineer, QA). Commercial
evolution is MGX, a hosted no-code platform launched Feb 2025.

**Current state (March 2026)**: Open-source repo hasn't had a release since v0.8.1 (April 2024). Team
energy shifted entirely to MGX. Product Hunt reviews are polarized — praise for fast prototyping, recurring
complaints about instability, data loss, weak context handling, and costly credit burn. MGX recently
rebranded to "Atoms." The promised MetaGPT v1.0 open-source release hasn't materialized.

**Philosophy**: `Code = SOP(Team)`. Encode how a software company works and agents will follow the process.

**Trust model**: Structured outputs as trust proxy. Each agent produces formatted artifacts (PRDs, UML,
API specs). Executable feedback loop retries up to 3 times on failure. No behavioral enforcement —
the system assumes agents follow the SOP if described clearly enough.

**Where it falls short vs Liza**:
- No code-enforced role boundaries (prompt-level only)
- No failure mode catalog or countermeasures
- No provider compatibility testing
- No anti-gaming mechanisms
- No crash recovery or multi-sprint continuity
- No context pressure management
- Retry-based failure handling (same agent tries again) vs structural prevention

**What Liza took from it**: Structured artifact requirements between roles are valuable. The spec phase
pipeline formalizes this. Academic positioning (ICLR oral) provides legitimacy. The MGX model validates
the market for a hosted enterprise product.

**Market position**: Low direct overlap. MGX targets no-code web app generation, not enterprise
software engineering. The open-source project appears stalled. Brand recognition (64k stars,
ICLR paper) keeps it visible.

---

### CrewAI

**What it is**: General-purpose multi-agent orchestration framework (45k+ stars, 100k+ certified
developers, VC-backed). Two primitives: Crews (autonomous role-playing agents) and Flows (event-driven
deterministic workflows). Python, YAML-based.

**Current state (March 2026)**: Active development — v1.9.0 latest stable, regular releases. Added
Agent-to-Agent (A2A) task execution, MCP server integration, structured output support. Enterprise
product (AMP Suite) offers tracing, observability, control plane. AWS Prescriptive Guidance integration.
Strong ecosystem with DeepLearning.ai courses and enterprise relationships.

**Philosophy**: Agent-centric. Design agents with roles, goals, backstories. Let them collaborate
through natural language. Flexibility and developer experience above all.

**Trust model**: Optimistic with retry. Guardrails are post-hoc validation functions (Python or
LLM-based string checks) that run after agent output. Retry up to N times on failure. Hierarchical
manager mode documented but doesn't work as intended — executes sequentially, not as true delegation
(confirmed by independent Towards Data Science analysis).

**Where it falls short vs Liza**:
- Guardrails are output-oriented, not behavior-oriented
- Cannot detect process violations (test mutation, scope creep during execution)
- No failure mode catalog
- No code-enforced role boundaries
- No provider compliance testing
- "Managerial overhead" measured at ~3x tokens for simple tasks vs LangChain
- No context pressure management or crash recovery

**What Liza took from it**: Flows architecture (deterministic backbone + autonomous agents) is the
right production pattern. YAML-based definition lowers barrier to entry. AMP Suite's
tracing/observability is a good model for enterprise positioning. A2A task delegation is worth watching.

**Market position**: Strong in general-purpose orchestration. Most likely framework enterprise teams
try first for any multi-agent use case. When they attempt serious software engineering and hit
behavioral failures, that's Liza's entry point. The competitive risk is if CrewAI adds domain-specific
guardrails for coding — but their general-purpose philosophy works against this.

---

### OpenAI Symphony

**What it is**: Elixir-based scheduler/runner released March 2026 (engineering preview). Polls Linear
for issues, creates per-issue workspace directories, spawns Codex agents. Built on BEAM/OTP for
fault tolerance and concurrency. Open source (Apache 2.0 initially reported, some sources say MIT).

**Current state (March 2026)**: Just released. Engineering preview — explicitly not production-ready.
Reference implementation in Elixir, but the spec (SPEC.md) is designed for multi-language implementation.
Active buzz on tech media. Positions itself as "scheduler, runner, tracker reader" — deliberately
narrower than a full orchestration framework.

**Philosophy**: "Manage work, not agents." Transform project management into automated execution.
Harness engineering — make codebases legible to agents, then let agents run autonomously per issue.
WORKFLOW.md keeps agent policy versioned with code.

**Trust model**: Minimal. "Proof of Work" concept (CI passes, walkthroughs before merge) but the spec
explicitly says it "does not require a single approval, sandbox, or operator-confirmation policy."
Trust posture is implementation-dependent and must be documented. No behavioral enforcement.
Agent self-certifies — no review loop, no role pairs.

**Where it falls short vs Liza**:
- No code review loop (agent self-certifies)
- No behavioral contract or failure mode awareness
- No role pairs (single-stage agent run per issue)
- In-memory state in some implementations (lost on crash)
- Codex-only in reference implementation
- No multi-sprint continuity
- No adversarial architecture

**Ideas worth adopting** (not yet implemented):
- Per-state concurrency limits (`max_concurrent_agents_by_state`) — cap reviewers vs coders
- Lifecycle hooks (pre_run, post_run, on_error) — maps to planned pre/post hooks
- Stall detection (agent alive but not progressing) — complements lease-based heartbeat
- Dispatch priority enforcement (sort candidates by priority before dispatch)

**Market position**: High in mindshare, low in direct competition. Symphony is OpenAI's answer to
"what do we build on top of Codex?" It will attract massive adoption through OpenAI's distribution.
But it's a scheduler, not an orchestration system — it doesn't solve trust, review, or behavioral
enforcement. The risk is that Symphony becomes the default starting point and teams never discover
they need what Liza provides. Liza positions as what you add when Symphony isn't enough.

---

### Paperclip

**What it is**: Open-source Node.js + React orchestration platform for "zero-human companies."
14k stars in days. Org charts, budgets, governance, goal alignment, agent coordination. Multi-tenant.
Agent-agnostic (Claude Code, Codex, Cursor, etc.).

**Current state (March 2026)**: Just launched, trending fast. Riding the "zero-person company" narrative.
Active community. MIT licensed. Embedded PostgreSQL. Well-structured repo.

**Philosophy**: "Your AI agents need a company, not better prompts." Models organizational structure —
org charts, budgets, approval gates, delegation chains. The corporate apparatus, minus the humans.
Completely unopinionated about agent runtimes.

**Trust model**: Governance-oriented. Budget caps (auto-pause at limit), approval gates (human operates
as "board of directors"), audit trails (append-only, no edits/deletions). But no behavioral enforcement
within agent sessions. Trust is organizational (who can do what, how much they can spend) not
behavioral (how they do the work).

**Where it falls short vs Liza**:
- No behavioral contracts or failure mode awareness
- No code review loop
- No role-specific enforcement
- No TDD gates or mechanical validation
- "Not a code review tool" (their own words)
- Trust is budget/approval-based, not execution-based
- No context pressure management

**What's interesting about it**:
- Cost tracking per agent, per task, per project, per goal — Liza's planned economic instrumentation
  should match or exceed this
- Budget enforcement with auto-pause — worth considering for sprint-level cost caps
- Multi-tenant with data isolation — relevant for enterprise product architecture
- "If it can receive a heartbeat, it's hired" — agent-runtime agnostic, like Liza

**Market position**: Low direct overlap (different domain — business operations, not software
engineering). High as a narrative competitor. The "zero-human company" framing captures
imagination. Liza's counter-narrative: "zero-trust agent sessions" matters more
for enterprise software than "zero-human companies."

---

### Ruflo

**What it is**: Multi-agent coding framework with 60+ specialized agent types (coder, tester, architect,
security-architect, etc.), 215+ MCP tools, Q-learning-based task routing, and HNSW-indexed
persistent memory. Uses Claude hooks for pre/post execution checks. ReasoningBank does
trigram/Jaccard similarity matching to find relevant past patterns and route tasks to agents
with the best track record.

**Philosophy**: Specialize and route. Many narrow agent types, each optimized for a specific task
category, with ML-based routing to select the best agent for each job.

**Trust model**: Track-record based. Q-learning routes tasks to agents with the best historical
performance. ReasoningBank matches patterns from past successes. Trust is empirical (past results)
rather than structural (enforced constraints).

**Where it falls short vs Liza**:
- No behavioral contract or failure mode catalog
- No code-enforced role boundaries (Claude hooks are provider-specific)
- No adversarial doer/reviewer pairs
- 60+ agent types creates context overhead that composable skills avoid
- 215+ MCP tools clog context
- Swarm topologies, Byzantine consensus, Q-learning routers solve coordination problems
  that Liza's blackboard architecture doesn't have

**Ideas worth adopting** (not yet implemented):
- Planner hints (model, skills, docs per task) — planner judgment over Q-table routing
- Mechanical pre-review linting — deterministic checks before reviewer spawns (generalized
  beyond Claude-specific hooks to model-agnostic shell scripts)
- Sprint Analyzer concept — capitalizing on patterns at sprint boundaries via lesson-capture
- Per-task model selection — planner assigns models by complexity, better than static tier routing

**What Liza skipped:** Swarm topologies, Byzantine consensus, Q-learning routers, HNSW indexing,
agent-writes-tracker pattern. Adding infrastructure complexity to a system whose core bet is
behavioral simplicity would be self-defeating.

**Market position**: Same domain (multi-agent coding), opposite architectural bet. Ruflo optimizes
for breadth (many specialized agents, many tools, ML routing). Liza optimizes for depth (fewer
roles, behavioral enforcement, composable skills). The test is which approach produces more
reliable output on real codebases.

---

## Adjacent Frameworks (Not Direct Competitors)

### LangGraph

State-machine based framework from LangChain. Closest to Liza's architectural philosophy (explicit
states, transitions, conditional routing). Production-grade, model-agnostic. But general-purpose —
no software engineering domain expertise, no behavioral contracts. Used as infrastructure by teams
building custom agent systems. No threat as a product competitor; potential as infrastructure
Liza could build on (but Liza's Go CLI approach is deliberately simpler).

### AutoGen (Microsoft)

Multi-agent conversation framework with human-in-the-loop patterns. Supports asynchronous messaging,
event-driven and request/response patterns. Robust framework for custom agent systems, but requires
heavy engineering. Known issues: can get trapped in loops, limited interface, high token costs.
Not a product — an SDK for building products. No domain-specific trust mechanisms.

### Cline / RooCode

Terminal/editor-native coding agents. Single-agent, not multi-agent orchestration. Permissioned
actions, clear plans, model flexibility. Popular with individual developers for day-to-day coding.
Not competing with Liza's multi-agent supervision — competing with Claude Code and Cursor as
the agent runtime that Liza orchestrates.

### AgentMesh

Academic prototype (184 stars). Planner → Coder → Debugger → Reviewer. Python script under 1,000
lines. In-memory state. Honest about limitations. Validates the multi-role architecture thesis but
stops at "this could work." Case study was a CLI to-do app. No ongoing development visible.

### MAS²

Research paper (Sept 2025). Meta-level paradigm — a MAS that generates other MAS. Tri-agent
architecture: Generator → Implementor → Rectifier. Reports impressive benchmark numbers but
operates at architecture-selection level, not behavioral enforcement level. The Rectifier concept
(runtime monitoring) inspired Liza's planned anomaly detection. No implementation available.

---

## Competitive Dimensions Matrix

| Dimension | Liza | CrewAI | Ruflo | Symphony | Paperclip |
|-----------|------|--------|-------|----------|-----------|
| **Domain** | Software engineering (9 roles, 2 phases) | General-purpose | Software engineering (60+ agent types) | Task scheduling | Business operations |
| **Trust approach** | Behavioral contract (55+ failure modes) | Post-hoc output validation | Track-record based (Q-learning) | Implementation-dependent | Budget/approval governance |
| **Role enforcement** | Code-enforced (MCP handler) | Prompt suggestion | Claude hooks (provider-specific) | None (single-agent) | Org chart hierarchy |
| **Review loop** | Adversarial doer/reviewer pairs | Optional manager mode (broken) | None (single-pass) | None | None |
| **Failure handling** | Structural prevention + escalation | Retry on output failure | Pattern matching from past successes | Implementation-dependent | Budget auto-pause |
| **Provider compliance** | Empirical matrix (5 providers) | None published | Claude-only | Codex-only | Agent-agnostic (no testing) |
| **Context management** | Tiered degradation, handoff protocol | Memory (short/long/entity) | HNSW-indexed persistent memory | Per-issue workspace | Persistent sessions |
| **Crash recovery** | recover-agent, recover-task | None | None | BEAM supervision trees | Session persistence |
| **Cost tracking** | Planned (token-level) | None native | None | None | Per-agent/task/project budgets |
| **Multi-sprint** | Yes (numbering, checkpoints, archive) | No | No | Per-issue runs | Heartbeat-based scheduling |
| **Maturity** | Alpha MAS (spec phase shipped), battle-tested pairing | Production (v1.9.0) | Active development | Engineering preview | Just launched |
| **Stars** | Early | 45k | Growing | New | 14k |
| **License** | Apache 2.0 | MIT | MIT | Apache 2.0 | MIT |

---

## Key Trends (March 2026)

**The scheduler/orchestrator layer is commoditizing.** Symphony and Paperclip prove that dispatching
work to agents, managing workspaces, and tracking runs is becoming table stakes. This is not where
the moat is.

**"Zero-person company" is the dominant narrative.** Paperclip's rapid traction (14k stars in days)
shows the market appetite. The narrative is exciting but premature for enterprise — nobody will
trust fully autonomous agents on production codebases without behavioral guarantees.

**A2A (Agent-to-Agent) protocols are emerging.** CrewAI's A2A task execution, Google's A2A protocol
announcement. Standardization of how agents delegate to each other is coming. Liza's blackboard
pattern already solves this for its domain but should watch for interoperability expectations.

**Harness engineering is OpenAI's answer to trust.** Symphony's philosophy is "make codebases legible
to agents" rather than "constrain agent behavior." Liza's README addresses this directly: it "enforces
governance intrinsically — not through external scaffolding as Harness Engineering does." Harness
engineering is necessary but insufficient — a legible codebase doesn't prevent an agent from modifying
tests to make broken code pass. The constraint must be in the system, not in the codebase.

**Enterprise trust remains unsolved by everyone except Liza.** Every framework survey and comparison
article mentions guardrails as a desirable feature. Nobody has what Liza has — 55+ documented failure
modes with mechanical countermeasures, code-enforced role boundaries, adversarial review, provider
compliance testing. This gap remains Liza's core differentiator.

---

## Code Quality Evidence

Claude Code Opus 4.6 produced a [code quality assessment](code_quality_assessment.md) of the Liza codebase
(commit 972d6c26, March 2026). Key findings:

**Overall Rating: A (Excellent)**

**Metrics:**
- 23,424 lines of Go across 138 production files
- 56,430 lines of tests across 120 test files (2.41:1 test-to-production ratio)
- 997+ test cases with table-driven subtests
- 4 direct dependencies (cobra, yaml.v3, flock, fsnotify) — radically minimal
- Zero TODOs in production Go code
- 41 Architecture Decision Records
- 98 specification files (13,824 lines)
- 21 pre-commit hooks, E2E tests in CI, race detection enabled by default

**Subsystem ratings:**
- State Machine & Models: ★★★★★
- Git Operations: ★★★★★
- State Validation (43+ rules): ★★★★★
- Behavioral Contracts & Skills: ★★★★★
- Testing & Quality Infrastructure: ★★★★★
- Documentation & Specifications: ★★★★★
- Operations Layer: ★★★★☆
- MCP Server: ★★★★☆
- Agent Supervision: ★★★★☆
- CLI Commands: ★★★★☆

**Key quote from the assessment**: "Liza is a technically rigorous project that practices what it
preaches. The behavioral contracts that govern LLM agents are themselves enforced by well-tested
Go code with atomic state management, comprehensive validation, and race-free concurrency patterns."

**Primary challenge identified**: Not code quality but cognitive surface area — 31,000+ lines of
specifications, contracts, and skills create an extraordinary knowledge base that also presents
a steep learning curve for maintainers. This is the "easy to maintain, easy to onboard" claim that needs to be
backed by the convention-over-code pattern holding as the system grows.
