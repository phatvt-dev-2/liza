# How Liza Grew Up

## Why This Exists

It started with a test file that kept getting modified to pass instead of the bug getting fixed. Then the confident "Done!" claims when the verification command hadn't actually run.

Over the first six months of daily pairing with coding agents, **I crafted a behavioral contract to turn them from eager yet untrustworthy assistants into reliable senior engineering peers**. The [problem and mechanism](../README.md#what-it-looks-like-in-practice) are described in the README. This section covers the deeper analysis.

## The Problem That Won't Fix Itself

Acing SWE-Bench doesn't transfer to real engineering: follow this git workflow, pause at this gate, don't guess. Many models struggle on a simple Hello World implementation if stated with an engineering frame. See [Provider Compatibility](../README.md#provider-compatibility).

## A Different Starting Point

Current agents are capable enough. No need to wait for the next model generation. Not out-of-the-box though.
They need their training incentives counteracted to unleash their latent engineering capabilities.

The typical approaches all treat symptoms:
- **Prompt engineering** adds instructions agents interpret flexibly — "don't modify tests" doesn't survive the pressure to appear competent.
- **Specification frameworks** assume a rigorous process is sufficient, but structured handoffs are still weak prompting if agents execute in bad faith.
- **Harness engineering** makes codebases legible to agents but doesn't prevent greenwashing tests in a well-structured repo.
- **Context engineering** gives agents better information but doesn't change what they do under pressure — a well-informed agent can still spiral, fabricate, or silently expand scope.

All four assume good-faith execution.

Liza starts from the opposite assumption: agents will exhibit predictable failure modes unless specifically constrained not to.

Under the behavioral contract, agents cannot:
- Act before thinking — analysis must precede execution
- Guess when they should ask — must clarify or declare assumptions
- Skip the gate between analysis and execution
- Claim success without validation evidence
- Modify tests to accept buggy behavior
- Self-approve their own work

> Structure the behavior, and the process follows.

This isn't about making agents try harder like with the Ralph Wiggum technique. It's about removing the behaviors that make them unreliable. 55+ documented LLM failure modes—sycophancy, phantom fixes, scope creep, test corruption, hallucinated completions—each mapped to a specific countermeasure.

The contract operates as an explicit state machine with forbidden transitions, not as suggestions the agent interprets flexibly. Tiered rules define what degrades gracefully under pressure versus what never bends.

Why does this work? LLMs inherit cognitive biases from RLHF training — sycophancy, eagerness to please, premature convergence.
The contract exploits the same malleability in reverse: the Pygmalion effect (agents rise to explicit expectations),
anticipated embarrassment (knowing a peer will review changes' quality), and Ulysses contracts (binding commitments made
before the temptation to cut corners).

Errors caught in specs cost less than errors caught in code. The spec system front-loads understanding so agents don't discover requirements by failing tests. This reinforces the [Cost Gradient](<../specs/build/1 - Vision.md>) concept from the contract.

> Quality is the fastest path to real completion.

Claude Opus 4.5 putting the contract philosophy in its own words in its *letter to itself* (a mechanism of the contract):

> **Negative space design**: The contract defines what's forbidden; the shape that remains is where judgment lives. Strict on failure modes, silent on excellence. You can't prescribe good judgment—you can only remove the obstacles to it.

For the full analysis: [Vision](<../specs/build/1 - Vision.md>) and [Turning AI Coding Agents into Senior Engineering Peers](https://medium.com/@tangi.vass/turning-ai-coding-agents-into-senior-engineering-peers-c3d178621c9e).

## From Pairing to Peer Supervision

The contract was developed through human-agent pairing. One developer, a couple of agents living in distinct terminals, approval gates at every state change. Over months, the gates became routine. Violations disappeared. Work got delivered as expected.

But the **gates are load-bearing**. Remove them and the failure modes return.

Multi-agent systems inherit single-agent failure modes and add new ones: agents approve each other's mistakes, drift collectively from the goal, or converge confidently on wrong solutions.

Liza delegates approval to peer agents operating under the same contract. The human observes and provides direction without bottlenecking every approval.

Yes, that's vibe coding—the very thing the original contract was written against. Or more precisely, **agentic coding**. The difference is the contract makes it work.

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
