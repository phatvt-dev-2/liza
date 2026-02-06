---
name: clean-code
description: Pre-commit Clean Code refactoring
---

Clean Code is a reader's gift — refactor for the next developer, not the compiler.

**Boy Scout Rule:** Leave the code cleaner than you found it. Every change is an opportunity to improve.

# Modes

| Mode | Scope | When |
|------|-------|------|
| **Staged** (default) | `git diff --cached` | Pre-commit cleanup |
| **Full-file** | Any file (no staging required) | Deeper refactoring session |

Announce mode: `"Cleaning in [mode]. Override?"`

# Pre-flight Checks

Before any transformation:

1. **Staged changes exist**
```bash
   git diff --cached --quiet && echo "Nothing staged" && exit 1
```

2. **Tests pass**
```bash
   pytest -q || exit 1
```

3. **Coverage gate** (tiered by transformation risk)
  - Run: `pytest --cov --cov-report=xml && diff-cover coverage.xml --compare-branch=HEAD`
  - `diff-cover` maps coverage to staged hunks specifically

  | Transformation Risk | Examples | Coverage Threshold |
  |---------------------|----------|--------------------|
  | Mechanical | Rename, dead code removal, import cleanup | ≥30% |
  | Structural | Extract function, early return, split responsibility | ≥70% |
  | Behavioral-adjacent | CQS split, error handling isolation | ≥90% |

  - Threshold applies to the **highest-risk transformation planned** — if any structural change is planned, the structural threshold applies to the whole session
  - **STOP if below threshold** — report uncovered lines, do not proceed
  - If tools unavailable (`pytest-cov`, `diff-cover`): warn, require explicit waiver to proceed without coverage data

4. **Diff size guard**
  - If staged diff >500 lines: require scope reduction or switch to Full-file mode with chunking strategy
  - **STOP if >500 lines** — "Staged diff too large (N lines). Reduce scope or switch to Full-file mode?"

5. **Git stash backup**
```bash
   BACKUP=$(git stash create)
   git stash store -m "code-cleaner-backup-$(date +%s)" "$BACKUP"
```

**Pre-flight summary:**
```
Pre-flight:
  Staged files: N
  Tests: ✓ pass (X tests)
  Coverage: Y% of staged lines (threshold: ≥Z% — [risk level])
  Backup: stash@{0}
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
   b. Run type checker if project uses one (`mypy`, `pyright`): `mypy <touched files>`
   c. Verify imports are consistent (extractions often leave stale imports — pre-commit hooks like `isort`/`autoflake` may catch this, but verify explicitly)
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

If project uses type checking, also run:
```bash
   git diff --cached --name-only -z | xargs -0 mypy
```

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
| **Dead code removal** | Unused imports, functions, variables, unreachable branches | Delete (use `vulture` for detection; false positives common — require explicit approval per item, do not batch delete) |

## Python-Specific Patterns

| Signal | Transformation |
|--------|----------------|
| Raw `dict` for structured data | Use `dataclass` or `TypedDict` |
| `os.path` manipulation | Use `pathlib.Path` |
| Manual resource cleanup (`open`/`close`) | Use context managers (`with`) |
| List comprehension not materialized | Use generator expression |
| Missing `__slots__` on data-heavy classes | Add `__slots__` for memory efficiency |
| `isinstance` chains | Consider `match`/`case` (Python ≥3.10) or dispatch |
| Mutable default arguments (`def f(x=[])`) | Use `None` sentinel + assignment in body |
| Bare `except:` or `except Exception:` | Catch specific exceptions |
| String formatting with `%` or `.format()` | Use f-strings |
| Manual `__init__` + `__repr__` + `__eq__` boilerplate | Use `@dataclass` |

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
| Pre-flight summary ("Proceed (P)?") | Auto-proceed if all checks pass; abort if any fail |
| Batch approval ("Proceed with batch 1 (P)?") | Auto-proceed |
| Await approval (between transformations) | Apply directly |
| Test failure ("(R)evert / (I)nvestigate / (F)orce continue") | Auto-revert batch |
| Between batches ("Proceed (P) / Skip (S) / Stop (X)?") | Auto-proceed |
| Propagation to unstaged files | Auto-proceed within worktree scope |
| Public API changes | Allowed within task scope |
| Diff size guard (>500 lines) | Abort — log anomaly to blackboard |

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
