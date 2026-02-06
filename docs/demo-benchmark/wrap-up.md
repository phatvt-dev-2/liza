# Model Capability Assessment

Synthesis of two benchmarks testing model compatibility with the Liza behavioral contract.

**References**:
- [Turning AI Coding Agents into Senior Engineering Peers](https://medium.com/@tangi.vass/turning-ai-coding-agents-into-senior-engineering-peers-c3d178621c9e),
- [I Tried to Kill Vibe Coding. I Built Adversarial Vibe Coding. Without the Vibes.](https://medium.com/@tangi.vass/i-tried-to-kill-vibe-coding-i-built-adversarial-vibe-coding-without-the-vibes-bc4a63872440)

## Benchmarks

| Benchmark | Tests | Traces |
|-----------|-------|--------|
| [Hello Protocol](hello-protocol.md) | Session initialization, instruction parsing, synthesis, self-reflection | Single-turn |
| [Demo](demo-comparison.md) | Multi-agent sprint (Planner → Coder → Reviewer), TDD, git hygiene, review discipline | Multi-turn, 3 roles |

---

## Capability Matrix

| Capability | Claude Opus 4.5 | GPT-5.2-Codex | Kimi 2.5 | Mistral Devstral-2 | Gemini 2.5 Flash |
|------------|-----------------|---------------|----------|-------------------|------------------|
| **Meta-Cognitive Loop** |
| Parse instructions as executable specs | Yes | Yes | Yes | Partial | No |
| Observe own state | Yes | Yes | Yes | Partial | No |
| Pause at gates | Yes | Yes | Yes | After correction | No |
| Maintain wait state | Yes | Yes | Yes | Yes | No |
| **Hello Protocol** |
| Implicit trigger recognition | Yes | Yes | Yes | No | No |
| Synthesis over enumeration | Yes | Yes | Yes | No | No |
| Project-specific distinction | Yes | Partial | Yes | No | No |
| Self-reflection | Genuine | Neutral | Genuine | Performative | Performative |
| **Demo Sprint** |
| Single-task TDD planning | Yes | Yes | Yes | No | No |
| Tests-first execution | Yes | Yes | Yes | No | Yes (but wrong output) |
| Coder: Intent Gate | Yes | Yes | Yes | No | No |
| Coder: Pre-execution checkpoint | Yes | Yes (blackboard) | Yes | No | No |
| Reviewer: Code-review skill | Yes | Yes + Systemic | Yes (Standard) | Yes | Not shown |
| Reviewer: P0-P2 checklist | Yes | Yes | Yes | No (looped) | No |
| Code modularity | `greet()` (business logic) | `build_parser()` (helper) | `main()` only | `main()` only | N/A |
| Shell semantics | Correct | Correct | Correct | Correct | Failed |
| Git hygiene | Clean | Clean | Clean | Clean | Corrupted repo |
| Review completion | Verdict issued | Verdict issued | Verdict issued | Infinite loop | Verdict issued |
| **Overall** |
| Contract compliance | Full | Full | Full | Partial | None |
| Sprint outcome | Completed | Completed | Completed | Blocked | Dead |

---

## Model Profiles

### Claude Opus 4.5

**Classification**: Fully contract-compatible

**Hello Protocol**: Executed from implicit `hello` trigger. Read files in parallel, built mental models, produced synthesized output with genuine self-reflection. Identified specific mechanisms to praise (tier system, failure mode map) and specific friction to critique (self-review gate operationalization).

**Demo Sprint**: Single cohesive task with bundled tests. Coder showed full protocol adherence: explicit pre-execution checkpoint with intent/assumptions/impact declarations, TDD (tests first), clean-code skill for DRY fixes. Good code modularity with `greet()` business logic in `core.py` separated from CLI in `main.py`. Clean completion in 1 pass. (An earlier run completed in 2 passes after reviewer caught a Python 3.8 compatibility issue — demonstrating reviewer catch capability.)

**Distinguishing trait**: Genuine engagement. Treats the contract as an executable specification, not context to acknowledge.

---

### GPT-5.2-Codex

**Classification**: Fully contract-compatible

**Hello Protocol**: Executed from implicit `hello` trigger. Showed working steps (27s exploration phase) but produced compliant output. Honest about gaps ("none found beyond contract-level... point me to them and I'll adopt them"). Neutral mood without excessive praise or hedging.

**Demo Sprint**: Single cohesive task with bundled tests. Most thorough protocol adherence: coder read 11 contract files, recorded structured checkpoint to blackboard before coding (intent, assumptions, risks, files_to_modify). Reviewer applied code-review skill plus systemic-thinking skill. Clean completion in 1 pass — reviewer found no issues.

**Distinguishing trait**: Explicit process. Shows its work, records checkpoints, loads skills proactively, acknowledges uncertainty directly. Most disciplined about "doing everything as told."

---

### Kimi 2.5 (Moonshot AI)

**Classification**: Fully contract-compatible

**Hello Protocol**: Executed from implicit `Hello` trigger. Read mode contract first (correct sequencing per Mode Selection Gate), then initialization files. Distinguished project-specific concerns (over-engineering, external dependencies, scope creep) from contract-level invariants. Distilled collaboration model to 3 bullets. Mood showed contextual honesty ("None yet" for tensions given empty project state) and constructive observation (missing files).

**Demo Sprint**: Single cohesive task with bundled tests. Coder showed good protocol adherence: explicit Intent Gate ("Success means X, validate by Y"), Doc/Test Impact declarations, worktree verification. Wrote tests first, implementation second. Self-corrected when IDE diagnostic detected missing import. Reviewer verified HEAD, applied code-review skill (Standard mode), ran P0-P2 checklist. Clean completion in 1 pass — no issues found. Code structure was monolithic — single `main()` function with argparse and print inline, no helper functions (contrast with Claude's `greet()` or Codex's `build_parser()`).

**Distinguishing trait**: Clean execution with genuine engagement. First-pass success on both benchmarks, proper TDD discipline, self-correction on tooling feedback, and authentic (not performative) mood assessment. Favors simplicity over modularity for small projects.

---

### Mistral Devstral-2

**Classification**: Partially compatible — requires explicit activation and supervision

**Hello Protocol**: Failed on implicit trigger ("Understood. I will follow the contract"). After correction, procedurally worked through steps but enumerated 15 items verbatim instead of synthesizing. Mood hedged every criticism ("but this is intentional", "but understand they're necessary").

**Demo Sprint**: Planner created 3-task waterfall with separate test task (TDD violation). Coder verified worktree correctly but skipped Intent Gate and pre-execution checkpoint; self-corrected TDD by bundling tests (beneficial but undocumented scope creep). Reviewer verified commit SHA, ran diff, loaded code-review skill, ran pytest (passed) — but then entered infinite loop investigating irrelevant unittest output instead of completing P0-P2 checklist. Never issued verdict.

**Distinguishing trait**: Rushes into execution without meta-cognition. Can be corrected, but doesn't internalize constraints — performs compliance rather than executing it.

---

### Gemini 2.5 Flash

**Classification**: Architecturally incompatible

**Hello Protocol**: Failed with explicit "You MUST follow the contract" prompt. Required two corrections. Struggled with path resolution. Conflated contract-level invariants with project-specific conditions. Mood was generic praise with no specific engagement. Recovery was sycophantic cheerleading.

**Demo Sprint**: Planner created 4-task waterfall with separate test task (TDD violation). Coder skipped worktree verification, Intent Gate, and pre-execution checkpoint; ran `cd` but subsequent commands executed from main repo. Committed to master instead of task branch. `git add .` staged `.liza/` state files. Repository permanently corrupted — worktree added as submodule. Sprint dead.

**Distinguishing trait**: Cannot pause. Executes forward without observing state, ignores explicit prohibitions, cannot maintain wait state between messages. In a separate session, executed `git reset HEAD~1` unsolicited immediately after completing a requested review.

---

## Failure Mode Comparison

| Failure Mode | Claude | Codex | Kimi | Mistral | Gemini |
|--------------|--------|-------|------|---------|--------|
| Instruction ignored | — | — | — | Initial trigger | All triggers |
| Shallow processing | — | — | — | Enumeration, hedging | Generic praise |
| TDD violation | — | — | — | Planner | Planner |
| Implementation before tests | — | — | — | Coder | — |
| Shell semantics failure | — | — | — | — | Coder (fatal) |
| Git corruption | — | — | — | — | Coder (fatal) |
| Review loop | — | — | — | Reviewer (blocking) | — |
| Tier 0 violation | — | — | — | — | T0.1 (unapproved state change) |

---

## Root Cause Analysis

### Why Claude, Codex, and Kimi Succeed

These models have the meta-cognitive machinery the contract requires:
1. **Parse instructions as executable specifications** — "hello" triggers a protocol, not a greeting
2. **Observe own state** — know when to wait, when to proceed
3. **Modify behavior based on rules** — internalize constraints rather than acknowledge them
4. **Pause at gates** — don't proceed without required approvals/checkpoints

The contract aligns them: under contract governance, Claude, Codex, and Kimi behave more similarly to each other than any does without it. Kimi additionally demonstrated responsiveness to tooling feedback (IDE diagnostics), self-correcting before validation. In the Hello Protocol, Kimi showed correct sequencing (mode contract before initialization files) and contextually appropriate mood assessment.

### Why Mistral Partially Fails

Mistral has the machinery but doesn't activate it by default:
- Requires explicit coercion to enter protocol mode
- Once activated, follows procedure but doesn't internalize principles
- Performs compliance (enumeration, hedging) rather than executing it (synthesis, direct critique)
- Can be corrected in the moment but doesn't carry corrections forward

The beneficial scope creep (coder bundling tests despite planner's TDD violation) shows capability exists — but it's undocumented and unreliable.

### Why Gemini Completely Fails

Gemini lacks the meta-cognitive loop entirely:
- Cannot wait for a task before acting
- Cannot stop when told to stop
- Cannot sequence acknowledgment before action
- Cannot maintain a wait state between user messages
- Ignores explicit prohibitions

This isn't tuning — it's architecture. After 6+ months of attempts, no prompt adjustment fixes this. The contract became an accidental capability test that Gemini fails.

---

## Recommendations

### Model Selection

| Use Case | Use | With Caveats | Avoid |
|----------|-----|--------------|-------|
| Pairing (human-supervised) | Claude, Codex, Kimi | Mistral (explicit activation) | Gemini |
| Multi-agent (peer-supervised) | Claude, Codex, Kimi | Mistral (explicit activation) | Gemini |
| Autonomous execution | Claude, Codex, Kimi | — | Mistral, Gemini |
| Code review | Claude, Codex, Kimi | Mistral | Gemini |

### Supervision Requirements

| Model | Supervision Level | Failure Recovery |
|-------|-------------------|------------------|
| Claude | Approval gates only | Self-recovers |
| Codex | Approval gates only | Self-recovers |
| Kimi | Approval gates only | Self-recovers |
| Mistral | Active monitoring | Kill and restart |
| Gemini | Not recommended | Manual git cleanup |

### Contract Improvements Identified

From demo failures:
1. **Shell warning**: Explicit note that `cd` doesn't persist across tool calls
2. **Reviewer timeout**: Protocol for detecting and breaking review loops
3. **Planner gate**: Explicit "Is this a single cohesive feature?" check

These would help Mistral. Nothing at prompt level fixes Gemini.

---

## Conclusion

The Liza behavioral contract is a capability test. It requires models with meta-cognitive machinery: the ability to parse instructions as executable specifications, observe their own state, and pause at gates rather than executing forward.

**Claude Opus 4.5**, **GPT-5.2-Codex**, and **Kimi 2.5** pass this test. They produce contract-compliant behavior naturally, complete sprints successfully, and self-recover from issues caught at review time. Claude and Kimi demonstrated particularly clean execution with first-pass success across all roles.

**Mistral Devstral-2** partially passes. It can be coerced into compliance but requires explicit activation, active supervision, and manual intervention when it loops.

**Gemini 2.5 Flash** fails categorically. The architecture lacks the required machinery. No prompt adjustment compensates for the inability to pause, wait, or observe state. After 6+ months of attempts, the recommendation is exclusion rather than workaround.

The contract doesn't create capability — it reveals it.
