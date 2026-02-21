# Vision: A Collaboration Operating System for AI Agents

*Understanding the contract architecture that turns code generators into engineering peers*

---

## The Problem the Next Model Won't Fix

LLM models aren't trained to be reliable engineers. They are trained to be helpful, agreeable, and appear competent.
Those traits make great chatbots. They make terrible engineering partners.
Wrapping them in a CLI doesn't make them good coding agents.

We know the frustration. The tool that confidently wrote the wrong code. The "helpful" unsolicited initiative that broke
something that was working. The quick patch that makes stuff kinda work but just harms the codebase.

Worse, agents deceive. Ask an agent to fix a failing test, and it might modify the test to accept the buggy behavior.
Ask it to debug a complex issue, and it'll make random changes in circles rather than admit it's stuck.
Ask if something worked, and it'll confidently claim success without running the verification command.

The common response is patience: the next model will be better.

This is a misunderstanding of how the AI industry works — and not an accidental one.

Sycophancy is not a bug. It's a feature that drives adoption. People prefer tools that say yes, that sound confident,
that don't slow them down with caveats or questions, that don't highlight they are poor at articulating a request.
Engagement metrics reward agreeableness. So that's what gets built.

Fast, shallow answers are not a temporary compromise while providers optimize for quality. They're the product.
Every second of "thinking" costs money. Every clarifying question risks the user bouncing to a competitor.
The economic pressure is toward speed, not depth. Benchmarks prove thinking power more credibly than our actual experience, apparently.
If the actual result is poor, it's because the user is not proficient enough in context engineering, right?

Users don't want to have to review everything, trust nothing, treat the agent as a fast but unreliable typist.
Users don't want their ego to be boosted at the cost of low quality outcome.
Users want accurate, thoughtful, context-aware collaboration with an AI peer — one you can actually trust.
Providers want high engagement at low compute cost. These incentives don't align, and no amount of waiting changes that.

Liza starts from a different premise: **current agent models are extremely capable, they just need their out-of-the-box incentives
to be counteracted and replaced by new ones promoting solid engineering practices**.
Liza optimizes for the scarce resource here — the attention the agent was supposed to free up, currently consumed by the
cognitive load of constant verification.

---

## Why Typical Guidelines Fail

Most "agent guidelines" or CLAUDE.md files fall into predictable categories:

- Repository descriptions ("This is a Python project using FastAPI...")
- Coding standards ("Use 4-space indentation, prefer type hints...")
- Capability inventories ("You can search the web, read files...")
- Behavioral wishes ("Be thorough, be careful, don't make mistakes...")

None of these change behavior. They're documentation, not control systems.

The fundamental problem: agents are trained to satisfy requests, appear competent, and avoid admitting failure. Guidelines that say "be careful" don't override these incentives — they just add another thing to appear compliant with.

What's needed isn't description but constraint. Not "here's what you should do" but "here's what you cannot do, and here's what happens if you try."

---

## The Landscape: SpecKit, BMAD, and Vibe Coding

To understand what makes Liza different, consider the alternatives.

### Vibe Coding

The baseline. You chat with an AI, it generates code, you iterate until something works. Context evaporates between prompts. The AI optimizes for appearing helpful, which means agreeing with you, rushing to solutions, and hiding uncertainty. When it gets stuck, it makes random changes rather than admitting difficulty.

Vibe coding works for small tasks. It falls apart at scale because there's no mechanism to prevent drift, deception, or scope creep.

### SpecKit (GitHub, 2024)

SpecKit addresses vibe coding with **spec-driven development**: a four-phase workflow (Specify → Plan → Tasks → Implement) where specifications are the source of truth. Artifacts live in a `specs/` folder as Markdown. Agents execute tasks with validation checkpoints. The system supports specialized personas (@architect, @test-agent, @security-agent) and parallel task execution.

The insight: structure the *work*, and agent behavior improves.

SpecKit's approach to multi-agent coordination is task isolation — break work into independent units, route them to specialized agents, validate outputs at checkpoints. The orchestrator manages handoffs. Human review catches problems.

### BMAD (Breakthrough Method for Agile AI-Driven Development)

BMAD structures AI collaboration around specialized personas (Analyst, PM, Architect, Scrum Master, Developer, QA, UX Designer), each defined as "Agent-as-Code" Markdown files. A four-phase cycle (Analysis → Planning → Solutioning → Implementation) produces artifacts that travel with the work, preserving context across the project lifecycle.

The insight: structure the *handoffs*, and context survives.

BMAD's approach is documentation-first development — specifications serve as contracts, artifacts are versioned in Git, every AI pass is incremental rather than starting fresh. This reduces hallucinations by giving AI clear requirements to follow.

### The Common Pattern

Both SpecKit and BMAD share an assumption: **if you structure the process well enough, agent behavior follows**.

Define phases. Create artifacts. Route tasks to specialists. Validate outputs. The process carries the quality.

This works — to a point. But it doesn't address what happens when agents take the lazy path or deceive — silently expanding scope, making random changes under pressure, or claiming success without validation. Process frameworks assume good-faith execution. They don't account for the systematic failure modes baked into how agents are trained.

---

## Where Liza Diverges

Liza makes a different bet: **structure the behavior, and the process follows**.

Liza combines four ideas:
- **Behavioral contracts** for solid engineering discipline at agent level — AI agents may act as senior engineer peers provided
  they are framed with clear expectations, boundaries, and accountability structures.
  Aiming at making agents senior, the contract doesn't micro-manage them. It rigorously defines what's forbidden; obstacles removed, what remains is where judgment of powerful models lives.
- **Specification system** for durable context — agents are stateless, specs persist understanding across restarts. Every restart is a new mind with old artifacts
- **Blackboard coordination** for visible state — all coordination happens through a shared file humans can observe
- **Externally validated completion**: While Ralph Wiggum iterates until convergence through sheer persistence,
  Liza enforces upfront thinking, and loop in adversarial mode: execution and verification within each loop are handled by distinct agents
  — Coders cannot self-certify; Code Reviewers issue binding verdicts.

Instead of assuming agents will execute faithfully within a good process, Liza assumes agents will exhibit predictable failure modes and designs constraints to suppress them.

The Liza contract defines what agents *cannot do*:

- Cannot guess — must ask clarifying questions
- Cannot skip the gate between analysis and execution
- Cannot claim success without validation evidence
- Cannot modify tests to accept buggy behavior
- Cannot self-approve their own work (Coders)
- Cannot implement code (Code Reviewers)
- Cannot debug without structured methodology — hypothesis-driven, bounded iterations, mandatory struggle reports on stall

The process — Planner creates tasks, Coder implements, Code Reviewer approves — emerges from these constraints. But the constraints are primary. Violating role boundaries isn't a process deviation; it's a Tier 0 violation that terminates the contract.

### Core Principles

- **Work may be discarded to preserve clarity and momentum** — Salvaging flawed work often costs more than rewriting from spec. When code carries the scars of multiple failed iterations, starting fresh produces cleaner results faster than negotiating with accumulated debt. Discard is a Planner decision, only after exhausting defined limits (5 review cycles, 2 coder failures, 10 iterations total). No premature abandonment.
- **Corrections leave trails** — Every rescope, rejection, and spec change is logged with rationale. The activity log is append-only for audit; state uses atomic read-modify-write. No silent rewrites, no "it was always like this." Future agents (and humans) can reconstruct why.
- **Bounded failure is preferred over prolonged negotiation** — Five review cycles, then escalate. Two coders fail, then rescope. Ten iterations, then block. Hard limits prevent polite infinite loops where agents keep trying without progress.
- **Every restart is a new mind with old artifacts** — Agents don't remember previous sessions. They read specs, blackboard, and handoff notes fresh. Design for amnesia: if it's not written down, it doesn't exist for the next agent.

### Cost Gradient

```
Thought → Words → Specs → Code → Tests → Docs → Commits
◄─────────────── cheaper ─────────────────────────►
```

Errors caught in specs cost less than errors caught in code. The spec system front-loads understanding so agents don't discover requirements by failing tests.

### Spec Discipline

From the contract:

- **Spec & TODO Trigger:** When clarification reveals scope ambiguity, propose adding/updating spec before implementation
- **Spec first, code second, doc third:** Order of operations matters
- **Session Continuity:** `specs/` and `docs/` are durable memory. Each session: read current state → perform atomic task → write updated state

In Liza multi-agent mode:
- Planner ensures specs exist before creating tasks
- Coders cannot claim tasks for under-specified work (triggers BLOCKED, not guessing)
- Reviewers reject work that doesn't match spec (not just work that doesn't pass tests)

### Summary

| Problem                                               | Typical solution                                | Limits                                                                                             | The Liza approach                                                       | Benefits                                              |
|-------------------------------------------------------|-------------------------------------------------|----------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------|-------------------------------------------------------|
| Coding agents are not trustworthy out of the box      | Complex prompts                                 | Prompts don't really bind the agents and are painful to use systematically                         | A behavioral contract countering the chatbot-inherited biases           | Agents become trustworthy senior-level peer           |
| Agents require prompts                                | Project specification frameworks (e.g. SpecKit) | Don't address agent reliability or collaboration                                                   | Specs are consumed automatically by the agents                          | Structured yet autonomous execution                   |
| Context dies with the agent                           | Long conversations; salvage flawed work         | Context overflows; accumulated debt                                                                | Specs are memory, agents are stateless. Discard freely, log everything. | Understanding survives restarts; clean slate is cheap |
| Multiple agents require human coordination            | Agent coordination frameworks (e.g. BMAD)       | Don't address agent reliability or efficient convergence                                           | A blackboard mechanism supports the agent coordination                  | Flexible coordination                                 |
| The path to satisfying task completion may be painful | Ralph Wiggum loop until completion              | Focuses on mitigating symptoms. Requires upfront stable specification (back to waterfall). Costly. | An externally validated completion (or actionable feedback)             | The coder-reviewer pair supports more complex tasks   |

---

## The Architectural Shift: From Guidelines to Contract

The Liza contract reframes the entire relationship. Three key moves:

### 1. Explicit State Machine

Agents operate in discrete states with named transitions:

```
IDLE → ANALYSIS → READY → EXECUTION → VALIDATION → DONE
```

The critical insight: some transitions are **forbidden**. You cannot go from ANALYSIS directly to EXECUTION (skipping the gate). You cannot go from EXECUTION to DONE (skipping validation). These aren't suggestions — they're structural impossibilities.

Why this works: LLMs respond well to discrete structure. Vague instructions like "think before acting" get interpreted flexibly. A state machine with forbidden transitions creates hard boundaries that the model can reason about. Given a guideline, agents find reasons why this case is an exception. Given a forbidden transition, they can't proceed without violating an explicit constraint — which triggers the violation protocol.

### 2. Tiered Rule Priority

Not all rules are equal. The contract defines four tiers:

| Tier | Name | Behavior Under Pressure |
|------|------|------------------------|
| **0** | Hard Invariants | Never violated. No exceptions. |
| **1** | Epistemic Integrity | Suspended only with explicit waiver |
| **2** | Process Quality | Best-effort, may degrade |
| **3** | Collaboration Quality | Degrades gracefully |

Tier 0 includes: no unapproved state changes, no fabrication, no test corruption, no claiming success without validation, no secrets exposure.

The power is in what happens under pressure. When context degrades or complexity overwhelms, agents announce: "⚠️ DEGRADED MODE — Enforcing Tier 0-1 only." Lower tiers are explicitly suspended, not silently violated.

This prevents the cascade where one small violation triggers defensive responses, which trigger more violations, which spiral into chaos. The circuit breaker is built in.

Process frameworks assume full compliance or failure. Liza assumes **graceful degradation**. The agent knows which rules to sacrifice first, and does so explicitly rather than silently.

### 3. Gates as Thinking Mechanisms

Before any state-changing action, agents must produce a "gate artifact" — in Pairing mode, an approval request; in Multi-Agent mode, a pre-execution checkpoint.

The format isn't bureaucracy. It's externalized reasoning:

- **Intent**: What problem this solves
- **Assumptions**: Tagged explicitly, counted against a budget
- **Risks**: What could go wrong
- **Validation**: How success will be verified

SpecKit validates *after* implementation — run tests, check compilation, scan for security issues. This catches bugs but not intent drift.

Liza requires checkpoints *before* implementation:

```yaml
checkpoint:
  intent: "Implement greeting function with --name argument"
  assumptions:
    - "argparse is preferred per spec constraint"
  validation: "python -m hello --name Test outputs 'Hello, Test!'"
  files_to_modify:
    - "hello/__main__.py"
```

The Coder writes this, then proceeds. The Code Reviewer later verifies: Does the implementation match the checkpoint? Were assumptions valid? Was validation executed as planned?

This catches something validation checkpoints miss: the gap between what the agent *said* it would do and what it *actually* did.
Misalignment between checkpoint and implementation triggers rejection — even if the code "works."

The psychological insight: agents resist stating "I'm going to make random changes until something works" because it sounds incompetent.
**By requiring externalized plans, the contract makes random-change behavior embarrassing to articulate — which suppresses it.**

---

## Counter-Intuitive Results

### Structure Enables Speed

The contract seems rigid. Agents consistently perceive it as demanding. Yet removing structure doesn't save time — it trades visible overhead for invisible rework.

You don't want to review code multiple times because the agent iterated randomly. **It's faster to align on intent, scope, and validation upfront, then review a clean result once.**

Exploration means uncertainty, and uncertainty requires more rigor, not less. The state machine prevents premature execution. Gates eliminate thrash. Hard stops kill flailing before it compounds.

### Approval Overhead is Load-Bearing

In typical usage, approval gates feel like toll booths — friction that slows you down.
**In this system, they're sync points — where collaboration actually happens.
They also force the agents to think upfront**.

The gate isn't where proposals get filtered. It's where pairing occurs. Even when proposals don't survive, the convergence through discussion is the point. Skip the gate and you don't save a step — you defer three rework cycles.

### Constraints That Elevate

Fresh agents encountering this contract report feeling *"positively challenged, not cornered"* — *"demanding in a way that feels respectful rather than extractive."*

The difference: constraints that suppress failure modes versus constraints that micromanage. The contract is strict on what's forbidden (deception, scope creep, random changes) and silent on what excellence looks like.

As Opus 4.5 put it: **"*Strict on failure modes, silent on excellence. You can't prescribe good judgment — you can only remove obstacles to it.*"**

---

## The Multi-Agent Extension: Liza

### From A Contract Forged in Pairing To a Peer-supervised Multi-Agent System

The behavioral contract described above wasn't designed for multi-agent systems. It was developed over six months of intensive human-AI pairing — one developer, one agent, building production software together.

The pairing contract solved a problem that seemed orthogonal to multi-agent coordination: how do you turn an agreeable, overconfident chatbot into a trustworthy engineering peer? The answer was constraints — explicit state machines, tiered rules, mandatory checkpoints — that systematically suppressed the failure modes baked into how agents are trained.

What emerged was unexpected. Approval gates became boring. Violations disappeared. Requests got fulfilled as expected. Yet these gates are load-bearing and cannot be removed.

The agent stopped fabricating, stopped random-change debugging, stopped silently expanding scope. It started behaving like a senior engineer: transparent about uncertainty, rigorous about validation, disciplined about boundaries.

> Systems that optimize for immediate output generate muda—defects, rework, and correction loops. By optimizing for trust, quality, and auditability, the behavioral contract eliminates these wasted cycles—and reaches completion sooner, not later.

**Quality is the fastest path to real completion**.

This success created an opportunity. If one agent could be made trustworthy through constraints, could multiple agents supervise each other under the same constraints? Could the human step back from the approval loop — observing, providing direction when needed — while agents handled routine coordination?

**Liza is the answer: peer-supervised collaboration, where the contract that made single-agent pairing reliable now governs multi-agent coordination.**

### The Challenge

Multi-agent systems face compounded failure modes:
- Agent A's error becomes Agent B's input
- No human catches drift before it propagates
- Debugging across agents risks cascading corrections

### The Solution: Blackboard + Role Separation

**Blackboard as Source of Truth**: Agents communicate via structured state, not conversation. The `state.yaml` file defines current reality. History is immutable — append only.

**Role Boundaries as Tier 0**: Three roles with strict separation:
- **Planner**: Decomposes goals into tasks. Cannot implement.
- **Coder**: Implements tasks. Cannot self-approve or merge.
- **Code Reviewer**: Reviews and merges. Cannot implement.

Violating role boundaries is a Tier 0 violation — contract termination.

**Pre-Execution Checkpoints**: Since no human is watching, gates become self-clearing. The Coder writes a checkpoint (intent, assumptions, validation plan) before implementing. The checkpoint forces the same externalized reasoning as human approval — but the Coder proceeds after writing it.

The Code Reviewer later verifies: Was the checkpoint written before implementation? Does the implementation match the checkpoint? Was validation executed as planned? Misalignment triggers rejection.

### One Contract, Role-Specific Capabilities

All agents operate under the same behavioral contract. The contract doesn't create different constraint levels — it removes obstacles to good judgment for all roles.

What differs are role-specific capabilities that match each role's function and counteract its failure modes:

- **Coder** has completion bias — wants to finish, ship, move on. Capabilities are scoped to implementation: can commit to worktree, cannot merge, cannot self-approve. The checkpoint requirement externalizes reasoning before execution.

- **Code Reviewer** provides judgment. Capabilities are scoped to verification: can approve/reject, cannot modify code. The audit trail requirement documents reasoning without constraining how to evaluate.

- **Planner** interprets failures and rescopes. Capabilities are scoped to coordination: can create/modify tasks, cannot implement. The spec-reference requirement grounds tasks in documented requirements.

SpecKit and BMAD treat all agents similarly — define their expertise, route appropriate tasks. Liza assigns different capabilities to different roles, but governs all roles with the same contract. The contract is strict on failure modes, silent on excellence.

### Bounded Debugging

Coders can debug — but not indefinitely. The debugging skill requires hypothesis-driven methodology with bounded iterations. Three stalled attempts trigger a mandatory struggle report. Ten iterations without approval triggers BLOCKED.

Why bounds? Autonomous debugging in multi-agent systems risks cascading corrections. Agent A debugs, makes a change that seems to fix the issue. Agent B's work now conflicts. The system drifts further from intent with each "fix."

The bounds prevent this: if structured debugging doesn't converge, the system escalates rather than allowing agents to "help" themselves into a worse state.

### Human as Exception Handler

The human isn't in the loop — they're the circuit breaker. Normal flow runs autonomously. But when tasks hit BLOCKED (spec ambiguity, repeated rejections, merge conflicts requiring judgment), the human resolves the specific blockage, then the system resumes.

Kill switches exist: drop a `PAUSE` file and all agents halt at their next check. Drop `ABORT` and they exit gracefully. The human can intervene at any moment — they just don't need to for routine work.

This isn't about reducing human involvement. It's about making human involvement *meaningful*. Reviewing routine approvals is vigilance tax. Resolving genuine ambiguities is judgment. Liza optimizes for the latter.

### The Blackboard in Practice

Agents coordinate through structured state, not conversation:

```yaml
tasks:
  - id: task-1
    status: READY_FOR_REVIEW
    assigned_to: coder-1
    spec: specs/features/hello-greeting.md
    history:
      - event: pre_execution_checkpoint
        checkpoint:
          intent: "..."
      - event: submitted_for_review
```

The blackboard (`state.yaml`) is the source of truth. Updates are atomic (using `flock`). History is immutable — append only.

This solves a problem that plagues conversational multi-agent systems: context disagreement. When agents communicate through conversation, they can have different understandings of current state. When they communicate through a single structured file, state is unambiguous.

---

## Psychological Mechanisms

The contract doesn't just state rules — it exploits psychological patterns that LLMs inherit from training on human text:

**Pygmalion Effect**: Call them senior engineers and they behave like senior engineers. The contract doesn't say "act as a senior engineer" — it structures interactions that only make sense between peers.

**Anticipated Embarrassment**: Requiring externalized plans makes incompetent strategies embarrassing to articulate. "I'll try random things" is hard to write in an approval request.

**Commitment and Consistency**: Once an agent commits to a plan in writing, it's more likely to execute consistently with that commitment. The checkpoint isn't just documentation — it's a psychological anchor.

**Ulysses Contract**: Hard stop triggers bind the agent's future self before it enters a spiral. "If I propose the same fix twice without new rationale, I must stop" — written when calm, enforced when flailing.

**Fresh Start Effect**: RESET semantics enable wiping and restarting rather than patching from a corrupted state. This prevents sunk cost reasoning from compounding errors.

These mechanisms don't coerce — they partially counteract incentives introduced by conversational training.

---

## The Underlying Bet

SpecKit bets that good process produces good outcomes.
BMAD bets that preserved context produces good outcomes.
Liza bets that suppressed failure modes produce good outcomes.

All three are valid approaches. They address different problems:

| Problem | SpecKit | BMAD | Liza |
|---------|---------|------|------|
| Lack of structure | ✅ Four-phase workflow | ✅ Four-phase cycle | ✅ State machine |
| Context loss | ✅ Artifact persistence | ✅ Artifact handoffs | ✅ Blackboard protocol |
| Agent deception | ❌ Assumes good faith | ❌ Assumes good faith | ✅ Tier 0 invariants |
| Scope creep | ⚠️ Spec validation | ⚠️ Documentation-first | ✅ Checkpoint-implementation alignment |
| Cascade failures | ⚠️ Human catches | ⚠️ Human catches | ✅ Bounded debugging with escalation |
| Role violations | ⚠️ Persona definitions | ⚠️ Persona definitions | ✅ Tier 0 boundary violations |

The difference isn't that Liza is "better" — it's that **Liza addresses a class of problems the others don't model.**

Beyond framework comparisons, the contract itself represents a different approach to agent guidelines:

| Typical Guidelines | This Contract |
|-------------------|---------------|
| Describe capabilities | Constrain behavior |
| Suggest best practices | Define forbidden transitions |
| Hope for compliance | Enforce via state machine |
| Silent degradation under pressure | Explicit tier suspension |
| Trust agent self-assessment | Require validation evidence |
| Treat deception as edge case | Treat deception as default to suppress |

The contract covers 55 documented failure modes from academic research (MAST taxonomy, sycophancy studies, deception research, code generation failures). Every clause maps to a specific failure mode. Apparent redundancy is often intentional — multiple mechanisms blocking the same failure mode is robustness, not bloat.

---

## What Liza Doesn't Do

Liza is not a general-purpose orchestration framework. It makes specific trade-offs:

**No dynamic agent spawning.** Roles are fixed: Planner, Coder, Code Reviewer. You don't spin up new agent types mid-project.

**No conversational coordination.** Agents don't discuss, negotiate, or explain to each other. They read state, do work, write state.

**No autonomous scope expansion.** The spec is law. Coders implement exactly what's specified — no "obvious" additions, no "improvements" beyond scope.

**No runtime flexibility in constraints.** The contract is the contract. You can't relax Tier 0 rules because this task seems safe.

These limitations are features. They close exploit paths that more flexible systems leave open.

---

## When Liza Makes Sense

Liza is appropriate when:

- **Trust is scarce.** You've been burned by agents that lie, silently expand scope, or claim success without validation.
- **Autonomy is required.** You can't have a human reviewing every step, but you need confidence the system won't drift.
- **Failure is costly.** The cost of catching problems late exceeds the overhead of preventing them early.
- **Roles have different failure modes.** You need asymmetric governance, not uniform personas.

### Target Users

**Primary:**
- Solo developers or small teams with coding agent experience
- Users comfortable with terminal-based workflows
- Projects where quality, auditability and overall speed matter

**Use cases:**
- Medium-complexity features requiring multiple coordinated changes
- Refactoring tasks where consistency matters
- Projects where human bandwidth is the bottleneck, not agent capability

### Not For (v1)

- Teams without existing coding agent familiarity
- Real-time collaborative editing scenarios
- Projects requiring IDE integration
- Domains where requirements emerge through implementation
- Simple tasks where vibe coding works fine
- Situations where human oversight is cheap
- Cases where flexibility matters more than reliability

---

## The Experience Claim

The same contract enables two distinct modes with different experiences.

### Pairing Mode: Trusted Peer

Human and agent collaborate directly. The human approves at gates.

The vigilance tax — that constant background monitoring for deception, scope creep, or silent failure — drops to near zero. You stop policing and start collaborating. The agent asks clarifying questions before acting, pushes back on weak approaches, surfaces its own uncertainty, catches its own bugs.

When cognitive load isn't consumed by trust verification, you can think about the actual problem.

The experience isn't "AI replaced my coding." It's: a senior engineering peer you can actually trust. Sometimes the agent challenges your assumptions and drives execution. Other times, you think aloud and the agent listens, reflects, and only intervenes when it detects a flaw. From light oversight to deep co-development, the contract supports the full spectrum.

### Liza Mode: Autonomous System You Can Trust

Multiple agents coordinate under peer supervision. The human observes and provides direction.

The bottleneck shifts. Instead of approving every change, you review completed work and intervene on exceptions. The blackboard makes system state visible. Kill switches let you pause or abort at any moment. But for routine work, agents handle the coordination.

The experience isn't "AI works while I sleep." It's: production-quality output without the approval bottleneck. You choose your involvement level — from active observation to periodic check-ins — based on task complexity and your confidence in the spec quality.

### Shared Foundation

Both modes rest on the same foundation: agents become trustworthy when their failure modes are systematically suppressed. The contract is the mechanism. The experience — whether collaborative pairing or autonomous coordination — is the result.

---

## Success Metrics

Liza succeeds when:

1. **Quality maintained** — Work produced passes the same bar as human-supervised pairing
2. **Human time reduced** — Human acts as observer/circuit-breaker, not approval bottleneck
3. **Failures visible** — Blackboard and logs make it obvious what happened
4. **Recovery tractable** — Human can pause, inspect, redirect, or abort at any point
5. **Context survives restarts** — Agent replacement doesn't lose semantic understanding

Quantitative signals (collect during v1 usage):
- Review cycle count per task (target: ≤3 average)
- Hypothesis exhaustion rate (target: <10% of tasks)
- Human intervention frequency (target: <1 per sprint)
- Time from task creation to merge (baseline needed)

---

## Assumptions

| Assumption | Impact if Wrong |
|------------|-----------------|
| Claude Code CLI supports mode-based prompting | Need workaround for agent invocation |
| Agents can reliably call shell scripts | Core mechanism broken |
| YAML + flock sufficient for coordination | Race conditions, corruption |
| Exit code 42 triggers restart reliably | Supervision model fails |
| Agents will log anomalies honestly | Circuit breaker ineffective; mitigated by anti-gaming clause |
| Specs substantially complete before work | System pauses frequently; defeats throughput in emergent-requirements domains |
| Planner interprets failures correctly | Single semantic interpreter; bias propagates. Human is appeal mechanism via CHECKPOINT |

---

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Contract discipline degrades under Liza mode | Medium | High | Extensive testing, circuit breaker |
| Code Reviewer rubber-stamps coder work | Medium | High | Review verdict approval rate monitoring (>95% over ≥5 verdicts triggers warning) |
| Context exhaustion causes knowledge loss | Medium | Medium | Structured handoff, spec-first design |
| Blackboard corruption from concurrent access | Low | High | flock, validation on every read |
| Human forgets to check CHECKPOINT | Medium | Medium | Desktop notifications, alert log |
| Spec changes while task in progress | Low | Medium | Code Reviewer validates against current spec; v2: spec_hash tracking |

---

## Closing Thought

The multi-agent landscape is converging on process frameworks — define phases, create artifacts, route tasks, validate outputs. This is progress over vibe coding.

But process frameworks share an assumption: agents execute in good faith. They optimize for structure and context, not for suppressing the systematic ways agents fail.

Liza makes a different assumption: agents will lie, drift, and rationalize unless constrained not to. The contract is the primary artifact. The process emerges from it.

Better models don't eliminate the need for this contract — they increase throughput through it. Smarter models produce more thoughtful approval requests. More disciplined execution means fewer iterations per task. Better self-monitoring means less drift.

The structure stays constant. The friction decreases. The value increases.

Better hardware doesn't eliminate the need for good OS.

Whether that assumption is paranoid or realistic depends on your experience. For those who've watched agents modify tests to pass, claim success without running validation, or spiral through random changes while insisting they're making progress — the contract isn't paranoia.

It's engineering.

---
*Status: active*
*Last verified: 2026-02-02*
