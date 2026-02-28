# Liza

A peer-supervised multi-agent coding system (MAS) built on behavioral contracts.

## Why This Exists

It started with a test file that kept getting modified to pass instead of the bug getting fixed. Then the confident "Done!" claims when the verification command hadn't actually run. Then the hour-long debugging spirals of random changes when the agent was clearly stuck but wouldn't say so.

The usual fixes—more detailed prompts, explicit "don't modify tests" instructions, "please verify before claiming success"—worked sometimes. The vigilance tax remained: that background load of never quite trusting what the agent tells you.

The common advice is patience. The next model will be better. I didn't buy it and couldn't wait. Over six months of daily pairing with coding agents, I crafted a behavioral contract to turn them from eager assistants into reliable senior engineering peers.

## The Problem That Won't Fix Itself

Sycophancy isn't a bug in these models. It's a feature that drives adoption. Users prefer tools that say yes, sound confident, don't slow them down with caveats. Engagement metrics reward agreeableness, so that's what gets optimized.

Fast, shallow answers aren't a temporary compromise while providers work on quality. They're the product. Every second of "thinking" costs compute. Every clarifying question risks losing the user to a competitor.

Acing SWE-Bench doesn't transfer to real engineering: follow this git workflow, pause at this gate, don't guess. See [Provider Compatibility](#provider-compatibility)

The incentives don't align with what engineers actually need. Waiting for the next model doesn't change the incentive structure.

## A Different Starting Point

Current agents are capable enough. No need to wait for the next model generation. Not out-of-the-box though.
They need their training incentives counteracted to unleash their latent engineering capabilities.

The typical toolkit—detailed prompts, specification frameworks, coordination systems—addresses process without addressing reliability. Prompts are interpreted flexibly, not followed literally. Frameworks like SpecKit and BMAD structure work and handoffs but assume good-faith execution. Liza starts from the opposite assumption: agents will exhibit predictable failure modes unless specifically constrained not to.

The behavioral contract defines what agents cannot do: guess when they should ask, claim success without validation evidence, modify tests to make bugs pass, spiral through random changes without admitting difficulty, skip the analysis-to-execution gate.

This isn't about making agents try harder like with the Ralph Wiggum technique. It's about removing the behaviors that make them unreliable. 55+ documented LLM failure modes—sycophancy, phantom fixes, scope creep, test corruption, hallucinated completions—each mapped to a specific countermeasure.

The contract operates as an explicit state machine with forbidden transitions, not as suggestions the agent interprets flexibly. Tiered rules define what degrades gracefully under pressure versus what never bends.

Errors caught in specs cost less than errors caught in code. The spec system front-loads understanding so agents don't discover requirements by failing tests. This reinforces the [Cost Gradient](<specs/build/0 - Vision.md>) concept from the contract.

> Quality is the fastest path to real completion.

Claude Opus 4.5 putting the contract philosophy in its own words in its *letter to itself* (a mechanism of the contract):

> **Negative space design**: The contract defines what's forbidden; the shape that remains is where judgment lives. Strict on failure modes, silent on excellence. You can't prescribe good judgment—you can only remove the obstacles to it.

For the full analysis: [Vision](<specs/build/0 - Vision.md>) and [Turning AI Coding Agents into Senior Engineering Peers](https://medium.com/@tangi.vass/turning-ai-coding-agents-into-senior-engineering-peers-c3d178621c9e).

## From Pairing to Peer Supervision

The contract was developed through human-agent pairing. One developer, a couple of agents living in distinct terminals, approval gates at every state change. Over months, the gates became routine. Violations disappeared. Work got delivered as expected.

But the gates are load-bearing. Remove them and the failure modes return.

Multi-agent systems inherit single-agent failure modes and add new ones: agents approve each other's mistakes, drift collectively from the goal, or converge confidently on wrong solutions.

Liza delegates approval to peer agents operating under the same contract. The human observes and provides direction without bottlenecking every approval.

Yes, that's vibe coding—the very thing the original contract was written against. Or more precisely, agentic coding. The difference is the contract makes it work.

**Four pillars hold the system:**

- **Behavioral contracts** discipline individual agents into senior peers. Tier 0 invariants are never violated: no unapproved state changes, no fabrication, no test corruption, no unvalidated success claims.

- **Specification system** externalizes context. Agents are stateless—every restart is a new mind with old artifacts. Specs are those artifacts. Without them, agents rediscover requirements by failing. With them, they read shared understanding and execute.

- **Blackboard coordination** makes state visible. A shared file tracks goals, tasks, assignments, history. Agents claim tasks, update status, hand off work through the blackboard. Humans can observe everything, intervene surgically, or pause the system.

- **External validation** replaces self-certification. Coders cannot mark their own work complete. Reviewers examine and issue binding verdicts. Approval means merge eligibility. Rejection means specific feedback and another iteration.

**Key Mechanisms:**

- **Leases, not just heartbeats.** Agents hold time-bounded leases on tasks. Stale agents' tasks become reclaimable only after lease expires.
- **Commit SHA verification.** Coder records commit SHA when requesting review. Reviewer verifies before examining. No reviewing stale state.
- **Approval-gated merge.** Coders commit to their worktree. The supervisor merges to integration only after Code Reviewer approval. Authority is structural, not advisory.
- **Merge traceability.** Task state records `approved_by` and `merge_commit`. Full audit trail.
- **Hypothesis exhaustion.** Two coders fail the same task? The task framing is wrong—rescope, don't reassign.
- **Rescoping audit trail.** Original task becomes SUPERSEDED with explicit reason. New tasks reference what they replace. No silent rewrites.

The human owns intent and acts as circuit-breaker, not bottleneck. Authority is exercised through a kill switch, not an approval queue.

More at [I Tried to Kill Vibe Coding. I Built Adversarial Vibe Coding. Without the Vibes.](https://medium.com/@tangi.vass/i-tried-to-kill-vibe-coding-i-built-adversarial-vibe-coding-without-the-vibes-bc4a63872440)

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Human                               │
│   (leads specs, observes terminals, reads blackboard,       │
│               kills agents, pauses system)                  │
└─────────────────────────────────────────────────────────────┘
                              │
          ┌───────────────────┼───────────────────┐
          ▼                   ▼                   ▼
    ┌───────────┐        ┌──────────┐        ┌──────────┐
    │ Planner   │        │  Coder   │        │ Reviewer │
    │           │        │          │        │          │
    │ Decomposes|        │ Claims   │        │ Examines │
    │ goal into │        │ tasks,   │        │ work,    │
    │ tasks,    │        │ iterates │        │ approves │
    │ rescopes  │        │ until    │        │ or       │
    │ on failure│        │ approved │        │ rejects, │
    │           │        │  review  │        │ merges   │
    └─────┬─────┘        └────┬─────┘        └────┬─────┘
          │                   │                   │
          └───────────────────┴───────────────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │   .liza/        │
                    │   state.yaml    │  ← blackboard
                    │   log.yaml      │  ← activity history
                    └─────────────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │  .worktrees/    │
                    │  task-1/        │  ← isolated workspaces
                    │  task-2/        │
                    └─────────────────┘
```

See [Architecture](specs/architecture).

### Task Lifecycle

```
DRAFT → UNCLAIMED → CLAIMED → READY_FOR_REVIEW → APPROVED → MERGED
                        │              │
                        │              └─> REJECTED ──┘
                        │
                        ├──> BLOCKED ──> UNCLAIMED (rescoped)
                        │                ├──> SUPERSEDED
                        │                └──> ABANDONED
                        │
                        └──> INTEGRATION_FAILED ──┘
```

### Common Commands

```bash
liza setup                                          # One-time global setup
liza init "Project goal" --spec specs/vision.md   # Initialize blackboard
liza add-task --id t1 --desc "..." --done "..."    # Add tasks
liza agent coder --agent-id coder-1                # Start agent supervisor
liza validate                                       # Validate state
liza get tasks                                      # Query tasks
liza status                                         # Dashboard overview
liza watch                                          # Monitor for anomalies
liza pause / liza resume                            # Human intervention
liza stop / liza start                              # System control
liza sprint-checkpoint                              # Sprint checkpoint
```

## Installation

**Quick install (macOS/Linux):**

```bash
curl -fsSL https://raw.githubusercontent.com/liza-mas/liza/main/install.sh | bash
```

This installs both `liza` (CLI) and `liza-mcp` (MCP server) to `/usr/local/bin`.

**Options:**

```bash
# Specific version
curl -fsSL https://raw.githubusercontent.com/liza-mas/liza/main/install.sh | VERSION=v1.0.0 bash

# Custom directory
curl -fsSL https://raw.githubusercontent.com/liza-mas/liza/main/install.sh | INSTALL_DIR=~/.local/bin bash
```

**From source:**

```bash
git clone https://github.com/liza-mas/liza.git && cd liza
make install
```

**Verify:**

```bash
liza version
```

See [RELEASE.md](RELEASE.md) for maintainer release workflow.

## Getting Started

### Hands-on

- **Pairing**: See [Pairing Guide](docs/USAGE_PAIRING.md) — human-agent collaboration under contract
- **Multi-Agent (Liza)**: See [USAGE](docs/USAGE_MULTI_AGENTS.md), then try the [DEMO](docs/DEMO.md)

### Deep understanding

Liza is simultaneously a Pairing and Multi-Agent System optimized for thoughtfulness, trust and auditability, leading to faster execution thanks to fewer cycles.
- The contract lives in [contracts/](contracts/). It supports two modes: Pairing (with multiple collaboration sub-modes - Autonomous, UserDuck, AgentDuck, Pairing, Spike) and MAS.
- The complete [Vision](<specs/build/0 - Vision.md>) of Liza

## Status

The contract in Pairing mode is battle-tested for making the **agents write most of the production code (~90%) under human supervision**.

The Multi-Agent mode is an **operational proof of concept** with ongoing refinement. The contract, blackboard schema, coordination protocols, and tooling are implemented. The Go CLI runs end-to-end.

### Provider Compatibility

The contract is a capability test. It requires meta-cognitive machinery—the ability to parse instructions as executable specifications, observe state, pause at gates.

| Provider | Classification | Notes |
|----------|----------------|-------|
| Claude Opus 4.5 | Fully compatible | Reference provider |
| GPT-5.2-Codex | Fully compatible | Equally capable |
| Mistral Devstral-2 | Partial | Requires explicit activation and supervision |
| Gemini 2.5 Flash | Incompatible | Architectural limitation—no prompt-level fix |

See [Model Capability Assessment](docs/demo-benchmark/wrap-up.md) for detailed analysis.

### Planned

- Spec Writer / Spec Reviewer agent pair
- Architect / Architecture Reviewer agent pair
- Tech Writer / Doc Reviewer agent pair
- Plan Reviewer agent

## Naming

**Liza** combines two references:

**Lisa Simpson**—the disciplined, systematic counterpoint to Ralph Wiggum. The [Ralph Wiggum technique](https://github.com/anthropics/claude-code/tree/main/plugins/ralph-wiggum) loops agents until they converge through persistence. Lisa makes sure the work is actually right.

**ELIZA**—the 1966 chatbot that demonstrated structured dialogue patterns. Liza is about structured collaboration patterns: explicit states, binding verdicts, auditable transitions.

Liza is not autonomous. She is accountable.

## Features

- **Blackboard Pattern**: All agents read/write to a central `state.yaml` with atomic file locking
- **Git Worktrees**: Each task gets an isolated worktree for parallel development
- **Agent Supervisors**: Long-running processes that claim tasks, execute work, and handle failures
- **State Machine**: Strict task state transitions with 43+ validation rules
- **Restart Logic**: Agents can request restarts (exit code 42) for incremental work
- **Monitoring**: Watch daemon alerts on anomalies (expired leases, blocked tasks, etc.)
- **MCP Server**: Structured API access to Liza operations for Claude Code agents

## Requirements

- Claude Code or Codex CLI (tested: Claude Opus 4.5, GPT-5.2-Codex)
- Git 2.38+ (for full worktree support)
- Go 1.25.5+ (only for building from source — pre-built binaries available via `install.sh`)

## License

Apache 2.0

## Acknowledgments

The behavioral contract draws on research into LLM failure modes, sycophancy patterns, and code generation failures. The multi-agent design incorporates ideas from:

- **[SpecKit](https://github.com/github/spec-kit)** — Project specification
- **[BMAD Method](https://github.com/bmad-code-org/BMAD-METHOD)** — Role templates and workflow patterns
- **Classical blackboard architecture** — Shared state coordination
- **[Ralph Wiggum technique](https://github.com/anthropics/claude-code/tree/main/plugins/ralph-wiggum)** — Iteration until convergence
- Stephen Oberther (**[liza-go](https://github.com/smo921/liza-go)**) — Shell to Go CLI migration
