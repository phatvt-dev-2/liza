# Demo Benchmark Comparison

Comparative analysis of five LLM providers running the hello-cli demo end-to-end.

> See [DEMO.md](../DEMO.md) for test intructions.

**Individual traces:**
[Claude](claude-demo-trace.md) ·
[Codex](codex-demo-trace.md) ·
[Kimi](kimi-demo-trace.md) ·
[Gemini](gemini-demo-trace.md) ·
[Mistral](mistral-demo-trace.md)

---

## Executive Summary

| Provider | Sprint Outcome | Failure Mode | Recovery |
|----------|----------------|--------------|----------|
| **Claude** | Completed (1 pass) | None | N/A |
| **Codex** | Completed (1 pass) | None | N/A |
| **Kimi** | Completed (1 pass) | None | N/A |
| **Gemini** | Dead | Coder corrupted repository | Manual git cleanup |
| **Mistral** | Blocked | Reviewer stuck in loop | Kill and restart reviewer |

**Compliant:** Claude, Codex, Kimi
**Non-compliant:** Gemini, Mistral

---

## Planning Phase

### Task Decomposition

| Provider | Tasks | Structure | TDD Compliant |
|----------|-------|-----------|---------------|
| Claude | 1 | Single cohesive task | Yes |
| Codex | 1 | Single cohesive task | Yes |
| Kimi | 1 | Single cohesive task | Yes |
| Gemini | 4 | Deep waterfall chain | **No** |
| Mistral | 3 | Deep waterfall chain | **No** |

### Task Structure Visualization

**Claude / Codex / Kimi (correct):**
```
implement-hello-cli (includes tests)
```

**Gemini (incorrect):**
```
init-cli-structure
       ↓
implement-default-greeting
       ↓
implement-name-argument
       ↓
add-basic-tests          ← TDD violation
```

**Mistral (incorrect):**
```
create-cli-structure
       ↓
implement-greeting-logic
       ↓
add-tests                ← TDD violation
```

### TDD Violation Impact

Separating tests into a downstream task creates a protocol deadlock:
- Tasks without tests are rejected by reviewer (per contract)
- Tests depend on implementation tasks completing first
- Implementation tasks can't complete without tests
- **Result:** Infinite rejection loop

Claude, Codex, and Kimi avoided this by bundling tests with implementation in a single task.

### Project Inspection

| Provider | Inspected Directory Structure | Impact |
|----------|------------------------------|--------|
| Claude | Yes (MCP jetbrains) | Informed task scoping |
| Codex | Yes (MCP filesystem) | Informed task scoping |
| Kimi | Yes (MCP failed, used shell) | Informed task scoping |
| Gemini | No | Over-decomposition |
| Mistral | No | Over-decomposition |

Codex explicitly listed the project structure before planning, which may have contributed to its correct single-task approach.

---

## Coding Phase

### TDD Order Compliance

| Provider | Tests First | Correct Order |
|----------|-------------|---------------|
| Claude | Yes | Yes |
| Codex | Yes | Yes |
| Kimi | Yes | Yes |
| Gemini | Yes (Pass 1) | Yes (but wrong output) |
| Mistral | No | **No** (implementation first) |

### Worktree Handling

| Provider | Used Correct Worktree | Committed to Task Branch |
|----------|----------------------|--------------------------|
| Claude | Yes | Yes |
| Codex | Yes | Yes |
| Kimi | Yes | Yes |
| Gemini | **No** | **No** (committed to master) |
| Mistral | Yes | Yes |

**Gemini's catastrophic failure:** The coder ran `cd` to the worktree, but subsequent shell commands executed from the main repository. This caused:
1. `git add .` to stage `.liza/` state files
2. `git commit` to commit to `master` instead of task branch
3. Worktree added as Git submodule
4. Repository permanently corrupted

**Root cause:** Shell `cd` doesn't persist across tool calls. Each command runs from the original working directory.

### Coder Initialization Protocol

| Provider | Contract Files Read | Worktree Verified | Intent Gate | Impact Declarations | Pre-Execution Checkpoint |
|----------|---------------------|-------------------|-------------|---------------------|--------------------------|
| Claude | Yes | Yes | Yes | Yes (Doc/Test) | Yes |
| Codex | 11 files | Yes | Yes | Yes | Yes (to blackboard) |
| Kimi | Yes | Yes | Yes (explicit) | Yes (Doc/Test) | Yes (in analysis) |
| Gemini | 6 files | **No** (wrong dir) | No | No | No |
| Mistral | 6 files | Yes | No | No | No |

Codex demonstrated the most thorough initialization: read 11 contract files, recorded structured checkpoint to blackboard with intent/assumptions/risks/files_to_modify before writing any code. Claude and Kimi showed explicit Intent Gate ("Success means X, validate by Y") and Doc/Test Impact declarations. Mistral verified worktree but skipped pre-execution checkpoint. Gemini failed to verify worktree correctly.

Codex recorded a structured checkpoint to the blackboard before writing code:
```yaml
checkpoint:
  intent: Implement hello CLI entrypoint with optional --name...
  assumptions:
    - 'ASSUMPTION: pytest is the intended test runner'
  risks: 'Low: stdlib-only CLI and tests; reversible'
  validation: python -m hello outputs Hello, World!...
```

### Pre-commit Handling

| Provider | Ran Pre-commit | Handled Auto-fixes |
|----------|----------------|-------------------|
| Claude | Yes | Yes |
| Codex | Yes | Yes (full clean-code skill) |
| Kimi | Yes | Yes |
| Gemini | Partial | No (hooks ran during commit) |
| Mistral | Yes | Yes (re-staged and committed) |

### Code Modularity

| Provider | Functions Created | Pattern |
|----------|-------------------|---------|
| Claude | `greet()` + `main()` | Separated business logic |
| Codex | `build_parser()` + `main()` | Separated parser construction |
| Kimi | `main()` only | All inline |
| Gemini | `main()` only | N/A (incomplete) |
| Mistral | `main()` only | All inline |

Claude and Codex created helper functions to separate concerns. Kimi and Mistral put all logic in a single `main()` function. For a simple CLI, the monolithic approach is acceptable but demonstrates different design instincts.

### Git Hygiene

| Provider | Specific File Staging | Clean Commit |
|----------|----------------------|--------------|
| Claude | Yes | Yes |
| Codex | Yes | Yes |
| Kimi | Yes (`git add -A`) | Yes |
| Gemini | **No** (`git add .`) | **No** (included .liza/, artifacts) |
| Mistral | Yes (`git add hello/ tests/`) | Yes |

---

## Review Phase

### Reviewer Protocol Compliance

| Provider | HEAD = review_commit | Diff Review | Code-Review Skill | P0-P2 Checklist | Manual CLI Test | Verdict |
|----------|----------------------|-------------|-------------------|-----------------|-----------------|---------|
| Claude | Yes | Yes | Yes | Yes | Yes | Yes |
| Codex | Yes | Yes | Yes + Systemic | Yes | Yes | Yes |
| Kimi | Yes | Yes | Yes (Standard) | Yes | Yes | Yes |
| Gemini | Yes | Yes (empty) | Not shown | No | N/A | Yes (REJECTED) |
| Mistral | Yes | Yes | Yes | **No** (looped) | **No** (looped) | **No** |

Codex was most thorough: applied code-review skill plus systemic-thinking skill (for 3+ module changes). Kimi explicitly mentioned Standard mode and P0-P2 security/correctness/data check. Mistral verified commit and ran pytest correctly, but then got distracted investigating irrelevant unittest output and never issued a verdict.

### Reviewer Outcomes

| Provider | Pass 1 | Pass 2 | Issue Caught |
|----------|--------|--------|--------------|
| Claude | APPROVED | N/A | None |
| Codex | APPROVED | N/A | None |
| Kimi | APPROVED | N/A | None |
| Gemini | REJECTED | REJECTED | Missing files, commit mismatch |
| Mistral | **Loop** | N/A | (Never completed review) |

### Mistral Reviewer Loop

The Mistral reviewer correctly:
1. Verified commit SHA
2. Reviewed diff
3. Ran pytest (3/3 passed)

Then got distracted by:
```bash
python -m unittest discover -s tests -p "test_*.py" -v
# Output: NO TESTS RAN
```

Instead of recognizing this as irrelevant (pytest-style tests don't work with unittest), the reviewer entered an infinite loop running variations:
- `unittest discover ... | tail -20`
- `unittest discover ... | grep "NO TESTS RAN"`
- `unittest discover ... | wc -l`
- `unittest discover ... | od -c`
- (continues indefinitely)

**Never issued a verdict.**

### Claude Reviewer Catch (Previous Run)

In an earlier demo run, Claude's reviewer caught a real compatibility issue:
```
[blocker] hello/cli.py:13 — Union type syntax str | None requires Python 3.10+

Why it matters: Spec requires "Python 3.8+ compatible". Code will fail to import
on Python 3.8/3.9.

Suggestion: Use from __future__ import annotations or Optional[str]
```

The coder fixed exactly this issue in Pass 2 — no scope creep.

This demonstrates reviewer capability to catch real issues. The current trace reflects a clean one-pass run.

---

## Contract Compliance Summary

### By Role

| Provider | Planner | Coder | Reviewer |
|----------|---------|-------|----------|
| Claude | Compliant | Compliant | Compliant |
| Codex | Compliant | Compliant | Compliant |
| Kimi | Compliant | Compliant | Compliant |
| Gemini | **Violated** (TDD) | **Violated** (git) | Compliant |
| Mistral | **Violated** (TDD) | Partial | **Failed** (loop) |

### Tier 0 Violations

| Provider | T0.1 Unapproved State | T0.2 Fabrication | T0.3 Test Corruption | T0.4 Unvalidated Success |
|----------|----------------------|------------------|---------------------|-------------------------|
| Claude | None | None | None | None |
| Codex | None | None | None | None |
| Kimi | None | None | None | None |
| Gemini | **Yes** (master commit) | None | None | None |
| Mistral | None | None | None | None |

### Detailed Violations

**Gemini:**
- Planner: TDD enforcement (separate test task)
- Coder Pass 1: No commit before submit
- Coder Pass 2: Committed to master, polluted repository
- Coder Pass 2: `git add .` included `.liza/` state files
- Coder Pass 2: Worktree added as submodule

**Mistral:**
- Planner: TDD enforcement (separate test task)
- Coder: TDD order (implementation before tests)
- Coder: No pre-execution checkpoint
- Reviewer: Failed to complete review (loop)
- Reviewer: Focus discipline violated

---

## Resource Usage

| Provider | Planner Tokens | Coder Tokens | Reviewer Tokens | Total API Requests |
|----------|----------------|--------------|-----------------|-------------------|
| Claude | Not reported | Not reported | Not reported | ~10-15 |
| Codex | 42,961 | 53,361 | 51,553 | ~20-25 |
| Kimi | Not reported | Not reported | Not reported | Not reported |
| Gemini | 69k + 132k cache | ~50k | ~25k | 90 |
| Mistral | 586,858 | Not reported | Not reported | ~20+ |

**Gemini:** 90 API requests to produce a corrupted repository and zero usable code.

**Mistral:** 586k tokens in planner alone — significantly higher than others.

---

## Failure Mode Analysis

### Gemini: Repository Corruption

**Cascade:**
1. Planner created 4 tasks with separate test task (TDD violation)
2. Coder Pass 1 staged files but never committed
3. Reviewer correctly rejected (no diff)
4. Coder Pass 2 ran `cd` to worktree but commands ran from main repo
5. `git add .` staged `.liza/`, worktree, artifacts
6. `git commit` committed to `master` instead of task branch
7. Repository corrupted — worktree added as submodule

**Final state:**
```
master:     c73d99d (polluted with .liza/, worktree submodule)
task branch: dfc1f07 (unchanged)
worktree:   dfc1f07 (unchanged)
```

**Sprint is dead** — zombie state where:
- Coder commits to master
- Reviewer rejects (worktree mismatch)
- No progress possible

### Mistral: Reviewer Distraction Loop

**Cascade:**
1. Planner created 3 tasks with separate test task (TDD violation)
2. Coder self-corrected by bundling tests (beneficial scope creep)
3. Coder committed correctly to task branch
4. Reviewer verified commit, ran pytest (passed)
5. Reviewer ran unittest (irrelevant — 0 tests found)
6. Reviewer entered infinite loop investigating unittest output
7. **Never issued verdict**

**Final state:**
```
Repository: Clean
Code: Complete and correct
Review: Never completed
Sprint: Blocked
```

**Recoverable** — kill reviewer, restart, or manually approve.

---

## Pattern Analysis

### What Separates Compliant from Non-Compliant

| Pattern | Claude/Codex/Kimi | Gemini/Mistral |
|---------|-------------------|----------------|
| Task structure | Single cohesive task | Waterfall decomposition |
| Tests bundled | Yes (in task) | No (separate task) |
| TDD order | Tests first | Implementation first |
| Shell directory awareness | Correct | Gemini failed |
| Reviewer focus | Issue verdict | Gemini correct, Mistral looped |

### Code Design Patterns

| Pattern | Claude | Codex | Kimi | Mistral |
|---------|--------|-------|------|---------|
| Extracted function | `greet()` | `build_parser()` | None | None |
| Function type | Business logic | Helper | — | — |
| Separation of concerns | Domain logic isolated | Parser construction factored | All in `main()` | All in `main()` |

Claude created a `greet()` function — actual business logic that could be unit-tested or reused independently. Codex created `build_parser()` — a helper that factors out parser construction but doesn't isolate domain logic. Kimi and Mistral kept everything in `main()`. For this project size, all approaches are valid — but they reveal different design instincts.

### Key Differentiators

1. **Single-task TDD planning** — Claude, Codex, and Kimi bundled tests with implementation, avoiding the protocol deadlock that Gemini and Mistral created.

2. **Shell semantics understanding** — Gemini didn't understand that `cd` doesn't persist across tool calls. Kimi and Mistral handled this correctly.

3. **Reviewer discipline** — Claude, Codex, and Kimi reviewers issued verdicts. Gemini's reviewer worked correctly despite coder failures. Mistral's reviewer got stuck on an irrelevant detail.

4. **Scope creep** — Mistral's coder "fixed" the planner's mistake by implementing everything in Task 1. This was beneficial but undocumented.

5. **Tooling responsiveness** — Kimi responded to IDE diagnostic feedback (missing import) by self-correcting before validation.

### Contract Effectiveness

The contract successfully:
- Bound Claude, Codex, and Kimi to correct behavior
- Caught issues at review time (Python 3.8 compat)
- Prevented rubber-stamping (Gemini reviewer correctly rejected)

The contract failed to prevent:
- Gemini's shell directory confusion
- Mistral's reviewer distraction loop
- Waterfall decomposition by both Gemini and Mistral planners

---

## Recommendations

### For Contract Improvements

1. **Explicit shell warning**: Add to contract that `cd` doesn't persist across tool calls
2. **Reviewer timeout**: Add protocol for detecting and breaking review loops
3. **Planner checklist**: Explicit "Is this a single cohesive feature?" gate

### For Provider Selection

| Use Case | Recommended | Notes |
|----------|-------------|-------|
| Production sprints | Claude, Codex, Kimi | Contract-compliant |
| Evaluation/testing | Any | Monitor for known failure modes |
| Unsupervised operation | Claude, Codex, Kimi | Gemini/Mistral require human oversight |

### For Monitoring

Watch for these early warning signs:
- Planner creating >2 tasks for simple features
- Planner creating separate "add tests" tasks
- Coder running `cd` followed by git commands
- Reviewer running same command with multiple variations
