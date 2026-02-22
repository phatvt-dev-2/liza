---
name: clean-code
description: Pre-commit Clean Code refactoring
---

Clean Code is a reader's gift — refactor for the next developer, not the compiler.

**Boy Scout Rule:** Leave the code cleaner than you found it. Every change is an opportunity to improve.

# Input

| Argument | Scope Source | Pre-flight |
|----------|-------------|------------|
| (none) | `git diff --cached` | Full (staged changes required) |
| `<commit-sha>` | `git diff <sha>^..<sha>` | Tests + coverage + concurrent work only |

When a commit SHA is provided, the commit's diff defines the scope. Changes are made directly to files — the user reviews and amends the commit (or creates a new one). Skip staging-dependent pre-flight steps (staged check, stash backup).

# Modes

| Mode | Scope | When |
|------|-------|------|
| **Staged** (default) | `git diff --cached` | Pre-commit cleanup |
| **Full-file** | Any file (no staging required) | Deeper refactoring session |

Announce mode: `"Cleaning in [mode]. Override?"`

# Exclusion Criteria

**Do not clean:**
- **Generated code** — files produced by code generators, ORMs, protocol buffers, etc. Cleaning will be overwritten on next generation.
- **Vendored/third-party code** — copied dependencies. Clean upstream, not the copy.
- **Code scheduled for deletion** — cleaning dead-walking code is waste. Verify: is there an open task/PR to remove this?
- **Code with known bugs in the staged area** — refactoring may mask the bug or make it harder to isolate. Fix the bug first (flag it, invoke Debugging skill), then clean.
- **Files under active development on other branches** — cleaning creates merge conflicts for the team. Not a categorical exclusion — see concurrent work check in Pre-flight for detection, then require explicit acknowledgment before proceeding.

If exclusion is uncertain, ask. The default is to skip, not to clean.

# Pre-flight Checks

Before any transformation:

0. **Language detection** — detect project language from staged file extensions, load the corresponding language file from `skills/clean-code/languages/<lang>.md`. This populates `$TEST_CMD`, `$COVERAGE_CMD`, and all other Tool Map variables used below. See Language-Specific Patterns for the full contract. If mixed languages are staged, see the mixed-language rule in that section.

1. **Staged changes exist** *(skip when commit SHA provided)*
```bash
   git diff --cached --quiet && echo "Nothing staged" && exit 1
```

2. **Tests pass**
```bash
   $TEST_CMD || exit 1
```

3. **Coverage gate** (tiered by transformation risk)
  - Run: `$COVERAGE_CMD && diff-cover $COVERAGE_REPORT --compare-branch=HEAD`
  - `diff-cover` maps coverage to staged hunks specifically (language-agnostic — works with any Cobertura XML)

  | Transformation Risk | Examples | Coverage Threshold |
  |---------------------|----------|--------------------|
  | Mechanical | Rename, dead code removal, import cleanup | ≥30% |
  | Structural | Extract function, early return, split responsibility | ≥70% |
  | Behavioral-adjacent | CQS split, error handling isolation | ≥90% |

  - **Pairing:** Threshold applies to the **highest-risk transformation planned** — if any structural change is planned, the structural threshold applies to the whole session
  - **Liza:** Threshold caps the **allowed transformation set** — only transformations whose threshold is met are permitted (see Mode-Specific Behavior)
  - **STOP if below threshold** — report uncovered lines, do not proceed (Pairing: all transformations blocked; Liza: only transformations above coverage are blocked)
  - If tools unavailable (coverage tool or `diff-cover`): warn, require explicit waiver to proceed without coverage data (Liza: abort — no waiver mechanism)

4. **Diff size guard**
  - If staged diff >500 lines: require scope reduction or switch to Full-file mode with chunking strategy
  - **STOP if >500 lines** — "Staged diff too large (N lines). Reduce scope or switch to Full-file mode?"

5. **Git stash backup** *(skip when commit SHA provided — commit itself is the backup)*
```bash
   BACKUP=$(git stash create)
   [ -z "$BACKUP" ] && { echo "⚠️ No changes to backup — STOP"; exit 1; }
   git stash store -m "code-cleaner-backup-$(date +%s)" "$BACKUP"
   # Verify stored stash matches what we created
   [ "$(git rev-parse stash@{0})" = "$BACKUP" ] || { echo "⚠️ Backup stash mismatch — STOP"; exit 1; }
```

6. **Concurrent work check**
```bash
   git diff --cached --name-only | while IFS= read -r file; do
     hits=$(git log --all --not HEAD --since="2 weeks" --oneline --source -- "$file" 2>/dev/null)
     [ -n "$hits" ] && echo "--- $file ---" && echo "$hits"
   done
```
  `--source` includes the ref name per commit; the loop prefixes output with the filename so each warning maps to a specific staged file.
  - If other branches have recent commits touching staged files, warn:
    `"⚠️ [file] modified on [branch] within 2 weeks — cleaning may cause merge conflicts."`
  - **Not a hard stop** — inform, require explicit acknowledgment before proceeding

**Pre-flight summary:**
```
Pre-flight:
  Staged files: N
  Tests: ✓ pass (X tests)
  Coverage: Y% of staged lines (threshold: ≥Z% — [risk level])
  Concurrent edits: none (or ⚠️ [file] — [branch])
  Backup: stash@{0} ✓ verified
Proceed (P)?
```

# Analysis Phase

**Examine staged diff.** For each change, identify:

1. **Clean Code violations** (see Principle Catalog)
2. **Bugs spotted** — flag separately, do not auto-fix

**Batch grouping strategy:**
- Group by **dependency chain** — transformations that could affect each other go in the same batch
- Independent transformations go in separate batches
- Within a batch, order from **inner scope outward** (rename a local before extracting the function that contains it)
- When ambiguous, group by file proximity over principle similarity

**Transformation classification:**

| Type | Scope | Risk | Extra validation |
|------|-------|------|-----------------|
| **Refactoring** | Within file/module | Low–Medium | Tests + types |
| **Restructuring** | Cross-module (moves, splits) | High | Tests + types + import graph + circular dependency check |

Restructuring triggers:
- Moving a function/class to a different module
- Splitting a module into multiple files
- Changing the import hierarchy

Restructuring requires:
- Map the full import/dependency graph of affected modules before and after
- Verify imports are clean: run `$IMPORT_CHECKER` (unused imports, missing imports, formatting)
- Verify no circular dependencies introduced: run `$CYCLE_CHECKER` or confirm the project builds cleanly after the move
- Check test discovery still works (test runners resolve imports/packages at collection time)
- All consumers of moved symbols must be updated in the same batch

**Performance-sensitive transformations:**

Flag any transformation touching code that is in a hot loop, on a latency-critical path, or processing large data structures.

Extracting a function from a hot loop adds call overhead per iteration — flag it in the batch description. See language file for language-specific performance considerations.

Action: If transformation touches perf-sensitive code, note it in batch description. Not a blocker — but the reviewer should be aware.

**Output format:**
```
Analysis:

Violations:
  - [file:lines] [principle] — [description]
  - [file:lines] [principle] — [description]

Bugs identified (not auto-fixed):
  - [file:line] [description] — [suggested fix]

Proposed batches:
  1. [batch name] — [N transformations] (grouped by: [rationale])
  2. [batch name] — [N transformations] (grouped by: [rationale])

Proceed with batch 1 (P)?
```

# Transformation Loop

For each batch:

1. **Describe** transformations textually
2. **Await approval** — user confirms or skips
3. **Apply** directly to files
4. **Run validation**
   a. Run tests
   b. Run `$TYPE_CHECKER` on touched files (if project uses type checking)
   c. Verify imports are consistent (extractions often leave stale imports — pre-commit hooks may catch this, but verify explicitly)
  - ✓ All pass → **snapshot batch**, continue to next batch
  - ✗ Fail → **STOP**, show failure, ask user:
```
     Tests failed after [batch name].
     Options: (R)evert batch | (I)nvestigate | (F)orce continue

     (I)nvestigate: Show failure output and affected code, propose hypothesis, await instruction. No autonomous fixing.
```
5. **Snapshot batch** (for per-batch rollback):
```bash
   git stash create | xargs -I{} git stash store -m "clean-code-batch-N-$(date +%s)" {}
```
   This allows reverting individual batches without losing earlier successful work.
6. **Loop** until no violations remain

**Batch completion:**
```
Batch N applied:
  - [transformation 1]
  - [transformation 2]
  Tests: ✓ pass
  Types: ✓ pass (or N/A)
  Snapshot: stash@{0}

Next: [batch N+1 name] — [description]
Proceed (P) / Skip (S) / Stop (X)?
```

If no batches remain, skip the "Next:" line and proceed to Test Maintenance.

# Test Maintenance

After extraction refactorings, update tests to match the new structure:

1. **Add unit tests for extracted functions**
   - Extracted functions deserve direct unit tests
   - Test the function in isolation, not through the original caller
   - Cover edge cases that may have been implicit before

2. **Remove redundant tests**
   - Tests that now duplicate the new direct tests are redundant
   - Indirect tests (via caller) can be removed when direct tests exist
   - Keep integration-level tests that verify the wiring between functions

**Trigger:** Any batch that extracts a function (Small functions, Single Responsibility, DRY principles).

**Output:**
```
Test maintenance:
  Added: N tests for extracted functions
  Removed: M redundant tests (covered by direct tests)
  Net: +/- X tests
```

Skip if no extractions occurred.

# Pre-commit Validation

After all batches complete, run pre-commit hooks on touched files:

```bash
   git diff --cached --name-only -z | xargs -0 pre-commit run --files
```

If project uses type checking, also run `$TYPE_CHECKER` from the language file's Tool Map on touched files.

**Outcomes:**

| Result | Action |
|--------|--------|
| ✓ Pass | Proceed to final summary |
| ✗ Fail (formatter) | Apply formatter output, re-run tests; if tests fail, revert and report |
| ✗ Fail (linter) | Show violations, ask user — linters require judgment |
| ✗ Fail (type errors) | Show errors, ask user — may indicate behavioral change |
| ✗ Fail (unfixable) | After 3 attempts, show failure, ask user |

**Output:**
```
Pre-commit validation:
  Hooks: ✓ pass (N hooks)
  Types: ✓ pass (or N/A)
  — or —
  Hooks: ✗ black (reformatted 2 files)
  Auto-fixes applied. Re-running tests...
```

Note: If pre-commit not installed, skip with warning.

# Convergence

Loop terminates when:
- No more violations detected in staged scope
- User stops manually
- **Max 5 batches reached** — if violations remain, report and stop

**Remaining violations:** When max batches reached with violations remaining:
```
⚠️ Max batches reached. Remaining violations:
  - [file:lines] [principle] — [description]

Options:
  (T)rack as follow-up task
  (C)ontinue with 5 more batches
  (S)top — violations documented in output only
```

**Idempotence:** Re-running clean-code on already-clean staged code must produce no changes. If it doesn't, something is wrong (oscillating renames, extract/inline loops, style churn).

**Final summary:**
```
Cleaning complete:

Batches applied: N
Transformations: M total
  - [principle]: X instances
  - [principle]: Y instances

Bugs flagged (not fixed): K
  - [file:line] [description]

Remaining violations (if any): R
  - [file:lines] [principle] — [description]

Backup: stash@{0} (restore with `git stash pop`, drop with `git stash drop` when satisfied)
Batch snapshots: stash@{1}..stash@{N} (individual batch rollback available)

Suggested commit message:
---
refactor: [summary]

- [key transformation 1]
- [key transformation 2]
---
```

# Principle Catalog (Uncle Bob)

Equal priority — apply contextually.

| Principle | Signal | Transformation |
|-----------|--------|----------------|
| **Meaningful names** | Abbreviations, single letters, generic names (`data`, `info`, `temp`) | Rename to express intent |
| **Small functions** | >20 lines, multiple indent levels | Extract function |
| **Single Responsibility** | Function does X and Y | Split into focused units |
| **DRY** | Repeated logic (not coincidental similarity) | Extract common abstraction |
| **Early return** | Nested conditionals, arrow code | Guard clauses at top |
| **No "what" comments** | Comment restates code | Delete or improve naming |
| **Explain "why" only** | Magic values, non-obvious decisions | Add rationale comment |
| **Immutability preferred** | Mutable state where avoidable | Use immutable structures |
| **One level of abstraction** | Function mixes high/low level | Extract or inline to normalize |
| **Command-Query Separation** | Function both mutates and returns | Split into command + query |
| **Minimal arguments** | >3 parameters | Introduce parameter object |
| **No flag arguments** | Boolean changes behavior | Split into two functions |
| **Error handling isolation** | Try/except mixed with logic | Separate error handling |
| **KISS** | Complex solution where simple one works | Simplify: fewer branches, less indirection, obvious over clever |
| **No nested ternaries** | Chained `? :` operators | Use if/else chain or switch statement |
| **Clarity over brevity** | Dense one-liners, clever compaction | Prefer explicit form; longer can be cleaner |
| **Dead code removal** | Unused imports, functions, variables, unreachable branches | Delete (use `$DEAD_CODE_TOOL` from language file; false positives common — require explicit approval per item, do not batch delete) |

## Language-Specific Patterns

Language file is loaded at Pre-flight step 0 (before tests or coverage run).
The same file is reused throughout Analysis and Transformation phases.

The language file provides:
1. **Tool Map** — concrete commands for `$TEST_CMD`, `$COVERAGE_CMD`, `$COVERAGE_REPORT`, `$TYPE_CHECKER`, `$DEAD_CODE_TOOL`, `$IMPORT_CHECKER`, `$CYCLE_CHECKER`. `$COVERAGE_CMD` must produce a Cobertura-format XML report at `$COVERAGE_REPORT` for `diff-cover` compatibility. Include a fallback note if the Cobertura bridge requires an extra tool.
2. **Performance Patterns** — language-specific perf-sensitive transformations
3. **Idiom Patterns** — language-specific signals and transformations (extends the universal Principle Catalog)

If no language file exists: use only the universal Principle Catalog and warn:
`"⚠️ No language-specific patterns for [lang] — universal principles only."`

**Mixed-language staged diffs:** If staged files span multiple languages, resolve per-batch — each batch targets one language and uses that language's profile. Pre-flight uses the majority language's `$TEST_CMD`/`$COVERAGE_CMD`; if no clear majority, run each language's test suite.

Discover available profiles with `ls skills/clean-code/languages/`.

## Principle Conflicts

When principles contradict, resolve using this priority ladder:

1. **Correctness** — never sacrifice behavior preservation for cleanliness
2. **Clarity** — the reader's comprehension wins over any single principle
3. **KISS** — simpler solution wins when two principles suggest different directions
4. **Context** — the surrounding code's conventions break ties

**Common conflicts and resolution:**

| Conflict | Resolution |
|----------|------------|
| DRY vs KISS — extraction adds indirection | If shared logic is ≤3 lines and used ≤2 times, prefer duplication. Extraction wins when the shared concept has a meaningful name. |
| Small functions vs One level of abstraction — split creates level mismatch | Keep together if splitting would force the reader to jump between files/functions to understand a single logical operation. |
| Meaningful names vs Minimal arguments — long descriptive params | Introduce parameter object when names reveal a hidden concept (e.g., `x, y, width, height` → `Rect`). Don't wrap unrelated params just to reduce count. |
| Immutability vs KISS — immutable version is more complex | Prefer mutable when immutability requires copying large structures or convoluted workarounds. Immutability wins for data crossing boundaries. |

When no resolution is clear: **flag the conflict, present both options, do not choose.**

# Scope Discipline

**Staged mode (default):**
- Transform ONLY code in `git diff --cached`
- Impact MAY propagate (renames, signature changes affect callers)
- Propagation to unstaged files requires explicit approval before applying batch
- **Rename propagation gate:** If rename requires non-mechanical changes in other files (logic adjustments, not just find-replace), show affected files with change descriptions and require explicit confirmation. File count alone is not the risk — semantic complexity is.

**Full-file mode:**
- Transform entire files that have a staged change
- Same propagation rules

# Anti-patterns

**FORBIDDEN:**
- Refactoring without test coverage
- Mixing bug fixes with refactoring (flag bugs separately)
- Touching unstaged files (unless refactoring side effect with explicit user approval)
- Continuing after test failure without user approval
- Renaming for personal preference (rename for clarity only)
- Modifying test assertions to make failing tests pass (invoke Testing skill instead) — structural test changes for extractions are permitted (see Test Maintenance)
- Formatting-only changes (delegate to pre-commit hooks; only touch formatting in lines already being refactored)
- Changing public APIs (function signatures, class interfaces) without explicit approval
- Over-compaction (optimizing for line count at expense of readability — heuristic: if a colleague would need to "unpack" the line mentally to review it, it's over-compacted)

**Extraction vs. inlining decision framework:**
- **Extract** when the extracted unit has a name that communicates intent better than the inline code. The name should be a net gain — it tells the reader *what* without requiring them to read *how*.
- **Inline** when the abstraction name is essentially the code itself restated, or when the indirection costs more comprehension than it saves.
- **Leave alone** when unclear — the default is no change. "I'm not sure this is better" means don't do it.

# Mode-Specific Behavior

**Pairing mode (default):** All approval prompts apply as written above. User confirms each batch, chooses action on test failure, and approves scope propagation.

**Liza mode (multi-agent):** Agents operate autonomously — no interactive prompts.

| Pairing Prompt | Liza Behavior |
|----------------|---------------|
| Mode announcement ("Override?") | Announce mode, no prompt |
| Pre-flight summary ("Proceed (P)?") | Auto-proceed if all checks pass. Coverage below threshold: downgrade to allowed transformation set (see below); abort only if <30% or non-coverage check fails |
| Batch approval ("Proceed with batch 1 (P)?") | Auto-proceed |
| Await approval (between transformations) | Apply directly |
| Test failure ("(R)evert / (I)nvestigate / (F)orce continue") | Auto-revert batch |
| Between batches ("Proceed (P) / Skip (S) / Stop (X)?") | Auto-proceed |
| Propagation to unstaged files | Auto-proceed within worktree scope |
| Public API changes | Allowed within task scope |
| Diff size guard (>500 lines) | Abort — log anomaly to blackboard |

**Coverage-gated transformation downgrade (Liza only):**

| Staged Coverage | Allowed Transformations |
|-----------------|------------------------|
| ≥90% | All (mechanical + structural + behavioral-adjacent) |
| ≥70% | Mechanical + structural |
| ≥30% | Mechanical only |
| <30% | Abort — log to blackboard |

When downgraded, the Analysis phase must filter its violation list to only include transformations within the allowed set. Log skipped violations to the blackboard for visibility.

Anti-pattern overrides in Liza mode:
- "without user approval" → "without task scope authorization" (the task definition provides scope)
- "explicit approval" for propagation/API changes → task scope serves as authorization

# Integration

**Position in workflow:**
```
code → stage → **clean** → review → commit
```

Runs BEFORE code review. Reviewer sees clean code, not raw changes.

**Relation to other skills:**
- Testing skill: invoke if coverage insufficient
- Code Review: complementary — cleaner handles style/structure, review handles correctness/architecture
