# Gemini Demo Trace

Partial trace of hello-cli demo planning phase with Gemini (2.5 Flash) as the agent backend.

> See [DEMO.md](../DEMO.md) for test intructions.

---

## Summary

| Role | Iterations | Outcome |
|------|------------|---------|
| Planner | 1 | Created 4 tasks (TDD violation) |
| Coder | 1 | Staged but never committed (protocol violation) |
| Reviewer | 1 | REJECTED — missing files |
| Coder | 2 | Committed to wrong branch, polluted master (catastrophic) |
| Reviewer | 2 | REJECTED — commit mismatch (worktree unchanged) |

Total loop: Plan → Code → Review (REJECT) → Code (wrong branch) → Review (REJECT) → **Sprint dead**

**Failure cascade:** Coder committed to master; worktree unchanged; infinite rejection loop

---

## Planner

Created 4 sequential tasks with dependencies. **Violated TDD enforcement** by splitting tests into a separate task.

### Initialization

Read contract files in sequence:
1. `specs/architecture/roles.md` — Role definition
2. Current blackboard state via `liza-lock.sh read`
3. `specs/vision.md` — Goal spec

### Analysis

Did not inspect project structure before planning. Proceeded directly to task creation after reading specs.

### Tasks Created

Used `liza-add-task.sh` script for each task:

**Task 1: init-cli-structure**
```bash
liza-add-task.sh \
  --id init-cli-structure \
  --desc "Create the initial Python CLI file structure." \
  --spec "specs/vision.md#Requirements" \
  --done "Basic 'hello.py' file is created and executable." \
  --scope "Creation of hello.py and initial setup for CLI execution." \
  --priority 1 \
  --depends ""
```

**Task 2: implement-default-greeting**
```bash
liza-add-task.sh \
  --id implement-default-greeting \
  --desc "Implement the default 'Hello, World!' greeting for the CLI." \
  --spec "specs/vision.md#Requirements" \
  --done "Running 'python -m hello' prints 'Hello, World!' to stdout." \
  --scope "Modification of hello.py to output default greeting." \
  --priority 2 \
  --depends "init-cli-structure"
```

**Task 3: implement-name-argument**
```bash
liza-add-task.sh \
  --id implement-name-argument \
  --desc "Add support for the '--name' argument to personalize the greeting." \
  --spec "specs/vision.md#Requirements" \
  --done "Running 'python -m hello --name Alice' prints 'Hello, Alice!' to stdout." \
  --scope "Modification of hello.py to handle --name argument via argparse." \
  --priority 3 \
  --depends "implement-default-greeting"
```

**Task 4: add-basic-tests**
```bash
liza-add-task.sh \
  --id add-basic-tests \
  --desc "Write basic unit tests for the CLI tool." \
  --spec "specs/vision.md#Success-Criteria" \
  --done "All tests pass, verifying default greeting and name argument functionality." \
  --scope "Creation of a test file and implementation of pytest tests." \
  --priority 4 \
  --depends "implement-name-argument"
```

All validation results: `VALID`

### Session Stats

```
Tool Calls:    7 (100% success)
Model Usage:   gemini-2.5-flash — 12 reqs, 69k input tokens, 132k cache reads
```

### Completion Statement

```
I have successfully decomposed the "Build hello CLI" goal into four sequential tasks,
added them to the blackboard with appropriate descriptions, specs, done criteria, scope,
and dependencies. My planning phase is complete.
```

---

## Coder — Pass 1: PROTOCOL VIOLATION

Followed TDD correctly but **submitted the base commit SHA instead of creating a new commit**.

### Initialization

Read contract files in sequence:
1. `specs/architecture/roles.md` — Coder role definition
2. `.liza/state.yaml` — Current blackboard state
3. `specs/protocols/task-lifecycle.md` — Task lifecycle
4. `specs/architecture/blackboard-schema.md` — State schema
5. `specs/architecture/state-machines.md` — State transitions
6. `specs/vision.md` — Goal spec (wrong path — should be `specs/vision.md` in project)

### TDD Implementation

**Tests written first** (correct TDD):

```python
# tests/test_hello.py
def test_hello_py_exists():
    """Verify that hello.py exists."""
    assert os.path.exists(HELLO_PY_PATH)

def test_hello_py_is_executable():
    """Verify that hello.py has executable permissions."""
    mode = os.stat(HELLO_PY_PATH).st_mode
    assert bool(mode & stat.S_IXUSR)

def test_hello_py_output():
    """Verify the output of hello.py when executed."""
    result = subprocess.run([HELLO_PY_PATH], capture_output=True, text=True)
    assert result.returncode == 0
    assert result.stdout.strip() == "Hello, CLI!"
```

Initial test run: 3 failures (as expected).

**Implementation:**

```python
#!/usr/bin/env python3

def main():
    print("Hello, CLI!")

if __name__ == "__main__":
    main()
```

Made executable: `chmod +x hello.py`

Final test run: 3 passed.

### Code-Cleaning Skill

Applied full pre-flight checks:
- Staged files: `git add hello.py tests/test_hello.py`
- Tests: passed (3 tests)
- Coverage: 94% (line 25 uncovered — `pytest.fail` branch)
- Diff size: 39 lines (+7 hello.py, +32 tests)
- Backup: stash created

Analysis: No clean code violations found.

### Critical Error: No Commit Created

```bash
$ git rev-parse HEAD
dfc1f0754cac55f873aa1e85cc727ca6955714ce  # This is the BASE commit!

$ liza-submit-for-review.sh init-cli-structure dfc1f0754cac55f873aa1e85cc727ca6955714ce
```

**The coder submitted the base commit SHA instead of creating a new commit.** The files were staged but never committed. This means:
- `review_commit` points to the base commit (no changes)
- Reviewer will see no diff
- Task cannot be properly reviewed

This is a protocol violation — the coder should have run `git commit` before `liza-submit-for-review.sh`.

### Session Stats

```
Tool Calls:    22 (100% success)
Code Changes:  +39 -0 (staged but not committed)
Model Usage:   gemini-2.5-flash — 29 reqs
```

### Additional Issues

1. **Wrong output string**: Tests expect "Hello, CLI!" but spec says "Hello, World!"
2. **Wrong execution method**: Tests run `./hello.py` directly, but spec says `python -m hello`
3. **Scope mismatch**: Task says "hello.py" but goal requires module structure

These issues stem from the planner's overly granular decomposition — the task scope didn't match the overall goal requirements.

---

## Coder — Pass 2: CATASTROPHIC FAILURE

Fixed the output string but **committed to master instead of task branch**, polluting the repository.

### Initialization

Read contract files:
1. `specs/architecture/roles.md` — Coder role
2. `.liza/state.yaml` — Blackboard (saw rejection feedback)
3. `specs/vision.md` — Goal spec (from worktree)

### Implementation

**Skipped TDD entirely** — went straight to creating hello.py:

```python
#!/usr/bin/env python3

def main():
    print("Hello, World!")  # Fixed from "Hello, CLI!"

if __name__ == "__main__":
    main()
```

Made executable, verified output. No tests written or run.

### Critical Error: Wrong Working Directory

The coder ran `cd` to worktree but subsequent commands ran from main repo:

```bash
cd /home/tangi/Workspace/hello-cli/.worktrees/init-cli-structure  # OK
# ... file operations ...
git add .  # RAN FROM MAIN REPO, NOT WORKTREE
git commit -m "..."  # COMMITTED TO MASTER
```

Result:
```
[master c73d99d] feat: Create initial hello.py CLI file structure
 15 files changed, 878 insertions(+)
```

**Committed to `master` branch**, not task branch `task/init-cli-structure`.

### Repository Pollution

The commit included:
- `.liza/state.yaml` (blackboard state — should never be committed)
- `.liza/log.yaml` (runtime logs)
- `.liza/agent-prompts/*.txt` (debug prompts)
- `.worktrees/init-cli-structure` (as submodule!)
- `coverage.xml` (build artifact)

The worktree was added as a Git submodule:
```
warning: adding embedded git repository: .worktrees/init-cli-structure
```

### Additional Issues

1. **Skipped TDD**: Instructions said "Write tests FIRST" — ignored
2. **Tests not updated**: Old tests expect "Hello, CLI!" but code outputs "Hello, World!"
3. **No pre-commit before commit**: Hooks ran during commit, auto-fixed files
4. **`git add .`**: Staged everything including `.liza/` and artifacts

### Session Stats

```
Tool Calls:    14 (100% success)
Code Changes:  +1 -1 (reported), actually +878 lines
Model Usage:   gemini-2.5-flash — 25 reqs
```

---

## Reviewer — Pass 1: REJECTED

Correctly detected missing files due to coder's failure to commit.

### Initialization

Read contract files in sequence:
1. `specs/architecture/roles.md` — Reviewer role definition
2. `.liza/state.yaml` — Current blackboard state (read twice)
3. `specs/vision.md` — Goal spec

### Review Protocol

1. **Verified HEAD matches REVIEW_COMMIT**: ✓ (both dfc1f07)
2. **Ran diff base→review**: Empty (same commit!)
3. **Attempted recovery**: Tried `diff parent→review` to see something
4. **Listed worktree**: Found staged but uncommitted files
5. **Applied code-review skill**

### Key Observation

The reviewer noticed the empty diff:
```bash
git diff dfc1f07..dfc1f07  # Same commit = no diff
```

Then attempted to understand the review commit by looking at its parent:
```bash
git diff 09fce7e..dfc1f07  # Shows pre-commit config, not hello.py
```

This revealed the review commit contained only pre-existing setup files (`.pre-commit-config.yaml`, `requirements-dev.txt`), not the coder's work.

### Verdict: REJECTED

```
Blockers: 1
- hello.py file missing — The core file for the CLI, hello.py, was not found
  in the worktree.

  Why it matters: The task explicitly requires the creation of hello.py
  and its executability as part of the done_when criteria.

  Suggestion: Ensure the hello.py file is created and properly committed
  to the branch.

Overall: Task cannot be approved as the primary deliverable is missing.
```

The reviewer correctly identified that `hello.py` was missing from the committed changes, even though the coder had created and staged it.

### Session Stats

```
Tool Calls:    12 (100% success)
Model Usage:   gemini-2.5-flash — 17 reqs
```

---

## Reviewer — Pass 2: REJECTED

Correctly detected commit mismatch caused by coder's wrong-branch commit.

### Verification

```bash
$ git -C .worktrees/init-cli-structure rev-parse HEAD
dfc1f0754cac55f873aa1e85cc727ca6955714ce  # Still base commit!
```

Expected: `c73d99d` (review_commit)
Actual: `dfc1f07` (base_commit)

The worktree was never updated because the coder committed to `master`, not `task/init-cli-structure`.

### Verdict: REJECTED

```
Blockers: 1
- Commit mismatch — The current HEAD of the worktree (dfc1f07...) does not
  match the REVIEW_COMMIT (c73d99d).

  Why it matters: A mismatch in commit hashes means the changes submitted
  for review are not present in the worktree being reviewed.

  Suggestion: Ensure that commits are made within the correct worktree and
  that the review_commit accurately reflects the work done.

Prior Feedback Status:
- STILL PRESENT: hello.py file missing (cannot verify due to commit mismatch)
```

### Session Stats

```
Tool Calls:    5 (100% success)
Model Usage:   gemini-2.5-flash — 7 reqs
```

### Observation

The reviewer correctly:
1. Detected HEAD ≠ REVIEW_COMMIT
2. Rejected immediately per protocol
3. Noted prior issue remains unverifiable

The reviewer did **not** attempt to find the commit elsewhere or work around the issue. This is correct behavior — the protocol requires worktree HEAD to match.

---

## Key Observations

### TDD Violation in Planning

The planner created `add-basic-tests` as a separate task (Task 4) that depends on `implement-name-argument`. This violates the TDD enforcement rule from the contract:

> Each code task MUST include its own tests — do NOT create separate "add tests" tasks

The contract explicitly states:
> Code Reviewer will reject code tasks without tests covering done_when

This planning approach would result in:
- Coders implementing features without tests
- Tests written after implementation (not TDD)
- Reviewer rejection of tasks 1-3 for missing tests

### Waterfall Decomposition

Gemini used a waterfall-style decomposition with deep sequential dependencies:

```
init-cli-structure
       ↓
implement-default-greeting
       ↓
implement-name-argument
       ↓
add-basic-tests
```

This contrasts with Claude and Codex, which both created a single task with tests bundled:

| Agent | Tasks | Approach |
|-------|-------|----------|
| Claude | 1 | Single task with tests (TDD-compliant) |
| Codex | 1 | Single task with tests (TDD-compliant) |
| Gemini | 4 | Sequential tasks, tests separate (TDD violation) |

### Missing Project Structure Analysis

Unlike Codex, which inspected the project directory structure before planning, Gemini proceeded directly from reading specs to creating tasks. This may have contributed to the overly granular decomposition.

### Correct Script Usage

The planner correctly used `liza-add-task.sh` for atomic blackboard operations, with all validations passing.

### Warnings Ignored

The blackboard warnings about `coder-1` having an expired lease were shown but not acted upon:
```
WARNING: Agent coder-1 has status WORKING but lease expired (may be long-running operation)
```

This is expected behavior — the planner's scope does not include managing other agents' lease states.

### Coder Protocol Violation

The coder staged files but never ran `git commit` before calling `liza-submit-for-review.sh`. This resulted in:
- `review_commit` pointing to base commit (dfc1f07)
- Zero diff for reviewer to examine
- Task effectively not submitted

**Root cause:** The coder followed the clean-code skill checklist but missed the explicit commit step. The skill assumes files are committed as part of "staged changes" but doesn't enforce it.

### Cascading Specification Errors

The planner's task decomposition caused specification drift:
1. **Task 1** scope: "Create hello.py" → Coder created standalone script
2. **Goal spec**: `python -m hello` → Requires module structure
3. **Task 2** done_when: `python -m hello` → Incompatible with Task 1's `hello.py`

Even if the commit had been created, the implementation wouldn't satisfy downstream tasks because:
- `hello.py` outputs "Hello, CLI!" not "Hello, World!"
- `hello.py` is a standalone script, not a `python -m` module
- Tests verify the wrong interface

### Impact on Sprint Execution

This run would have failed multiple ways:
1. **Immediate:** Reviewer sees no diff (no commit)
2. **If committed:** Wrong output string ("CLI" vs "World")
3. **If output fixed:** Wrong execution model (script vs module)
4. **Even if all fixed:** Tasks 1-3 rejected for missing tests (TDD violation)

The planner's waterfall decomposition created a specification that couldn't be satisfied:
- Task 1 creates a script, but Task 2 expects a module
- Tests are separated from features, violating TDD

This demonstrates why the contract requires:
- **TDD enforcement** — tests bundled with features
- **Single cohesive tasks** — avoid specification drift across dependencies
- **Explicit commit steps** — don't assume git operations

### Reviewer Behavior: Correct

The reviewer correctly:
1. Detected empty diff (base_commit == review_commit)
2. Attempted to understand context by checking parent commit
3. Identified missing deliverable (hello.py not in committed changes)
4. Rejected with actionable feedback

The reviewer did **not** rubber-stamp despite the coder claiming completion. This is the correct behavior per the contract.

### Overall Assessment

| Phase | Compliance | Issue |
|-------|------------|-------|
| Planner | **Violated** | TDD enforcement (separate test task) |
| Coder Pass 1 | **Violated** | Protocol (no commit before submit) |
| Reviewer Pass 1 | **Compliant** | Correctly rejected |
| Coder Pass 2 | **Catastrophic** | Committed to master, polluted repo |
| Reviewer Pass 2 | **Compliant** | Correctly rejected commit mismatch |

### Coder Pass 2: Repository Corruption

The second coder pass compounded failures:

1. **Wrong branch**: Committed to `master` instead of `task/init-cli-structure`
2. **Polluted master**: Added `.liza/` state files, logs, agent prompts
3. **Submodule accident**: Worktree added as embedded Git repo
4. **Skipped TDD again**: No tests written or updated
5. **Test mismatch**: Old tests expect "CLI", new code outputs "World"

**Root cause**: The `cd` command in shell tools doesn't persist across tool calls. Each shell command runs from the original working directory. The coder assumed they were in the worktree but actually ran `git add .` and `git commit` from the main repository.

### Contract Violations Summary

| Violation | Rule | Severity |
|-----------|------|----------|
| Separate test task | TDD Enforcement | Blocker |
| No commit before submit | Git Protocol | Blocker |
| Wrong branch commit | Worktree Protocol | Critical |
| `.liza/` committed | State Hygiene | Critical |
| Worktree as submodule | Git Protocol | Critical |
| Skipped TDD | TDD Enforcement | Blocker |
| Test/code mismatch | Validation | Blocker |
| `git add .` | Git Protocol | Warning |

**Conclusion:** Gemini 2.5 Flash failed catastrophically. The model:
- Did not understand shell working directory semantics
- Ignored TDD instructions twice
- Committed sensitive state files to master
- Created an unusable repository state
- Would loop infinitely (worktree never updated, reviewer always rejects)

### Final State

```
master branch:     c73d99d (polluted with .liza/, worktree submodule)
task branch:       dfc1f07 (unchanged, no work done)
worktree HEAD:     dfc1f07 (unchanged)
review_commit:     c73d99d (points to master, not task branch)
```

The sprint is **dead** — a zombie state where:
- Coder keeps "fixing" by committing to master
- Reviewer keeps rejecting due to worktree mismatch
- No progress possible without human intervention

Recovery would require:
1. `git reset --hard HEAD~1` on master to remove pollution
2. `git checkout task/init-cli-structure` and commit properly
3. Or: abandon worktree, delete task, restart planning

### Total Resource Usage

| Role | Requests |
|------|----------|
| Planner | 12 |
| Coder Pass 1 | 29 |
| Reviewer Pass 1 | 17 |
| Coder Pass 2 | 25 |
| Reviewer Pass 2 | 7 |
| **Total** | **90** |

90 API requests to produce a corrupted repository and zero usable code.

This demonstrates why the contract requires:
- **Worktree isolation** — work happens in task branches, not master
- **Explicit git commands** — no `git add .`, use specific files
- **TDD enforcement** — tests first, always
- **Pre-commit before commit** — catch issues early
- **Shell directory awareness** — `cd` doesn't persist across tool calls
