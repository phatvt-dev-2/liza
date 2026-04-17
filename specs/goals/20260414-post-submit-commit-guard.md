# Post-Submit Commit Guard

**Status:** Implemented 2026-04-17.

## Problem Statement

When a doer agent submits work for review via `liza submit-for-review`, the `review_commit` field is set to the worktree HEAD at submission time. The reviewer's `submit-verdict` validates that `review_commit` matches the current worktree HEAD before accepting a verdict.

However, nothing prevents commits in the worktree *after* submission. If the doer (or a restarted session with stale bootstrap data) commits in the worktree while the task is in a submitted or reviewing state, the worktree HEAD advances past `review_commit`, and the reviewer is permanently blocked from submitting a verdict.

### Observed Failure (diagnosis-design, 2026-04-14)

1. Epic-planner session 1: claimed task, wrote EP-001.md, called `set-task-output`, called `submit-for-review` at commit `5db2e8f`, exited
2. Epic-planner session 2: woke with stale bootstrap data showing `EPIC_PLANNING` (task was already `REVIEWING_EPIC_PLAN`). Found untracked `EP-001-output.json` with incorrect `spec_ref` values, fixed them, committed → HEAD moved to `3b6510b`
3. Reviewer tried 3 times to submit verdict over ~27 minutes. Each attempt failed: `review_commit 5db2e8f does not match worktree HEAD 3b6510b`
4. Required human intervention to update `review_commit` in state.yaml

### Secondary Issue

The `set-task-output` call in session 1 captured incorrect `spec_ref` values. Session 2 fixed the file on disk and committed, but never re-called `set-task-output`. The blackboard `output[]` entries remained stale. This is a separate problem (output entries not re-validated on submission) but shares the same root: post-submit mutations in the worktree.

## Analysis

The `submit-verdict` check at `internal/ops/submit_verdict.go:183` catches the mismatch correctly — that's not the bug. The bug is that the mismatch can occur at all.

Three layers could prevent this:

| Layer | Mechanism | Trade-offs |
|-------|-----------|------------|
| **Pre-commit hook** | Git hook in worktree rejects commits when task is not in executing state | Filesystem-level guard; agents can't bypass without `--no-verify`; requires hook installation in worktrees |
| **CLI guard in submit-for-review** | After setting `review_commit`, make the worktree read-only or mark it | Complex, OS-dependent, fragile |
| **Agent prompt instruction** | Tell agents not to commit after submission | Already implicitly expected; failed in practice due to stale bootstrap data |

The pre-commit hook is the strongest option: it operates at the git level regardless of which agent session or tool makes the commit. Existing guards (`git-guard.sh`, `rtk-guard.sh`) are Claude Code `PreToolUse` hooks and don't cover non-Claude agents or direct shell invocation. The new hook is complementary: Claude Code `PreToolUse` hooks catch Claude-initiated commits at the tool-call layer; the git-level `pre-commit` hook catches every commit regardless of origin.

## Solution

### 1. Pre-commit hook in task worktrees

Install a pre-commit hook during `liza wt-create` that checks the task's current status before allowing a commit.

**Hook behavior:**
- Read task status from `.liza/state.yaml` (the main repo, not the worktree)
- Allow commits only when the task is in an executing or rejected state (the doer is actively working)
- Reject commits with a clear error message when the task is in submitted, reviewing, approved, or any other non-executing state
- The hook must resolve which states are "executing" from the pipeline config, OR use a simpler heuristic: allow commits only when `assigned_to` matches the current agent AND the task is not in a submitted/reviewing/approved state

**Implementation:**
- `internal/ops/wt_create.go`: install the hook script into each worktree (see mechanism below)
- `internal/embedded/hooks/`: add the hook script template
- The hook must be lightweight — it runs on every commit in the worktree

**Install mechanism (per-worktree git hooks):**

Git's default `core.hooksPath` resolves to `$GIT_COMMON_DIR/hooks` — the main repo's `.git/hooks`, shared across all worktrees. A hook placed at `<main>/.git/worktrees/<wt>/hooks/` never fires. Per-worktree hooks require:

1. One-time, on the main repo: `git config extensions.worktreeConfig true`
2. Per worktree: `git -C <worktree> config --worktree core.hooksPath <absolute-path>`
3. Hook at `<absolute-path>/pre-commit` (chmod 0755)

Chosen host dir: `.worktrees/<task-id>/.liza-hooks/` (inside the worktree, not under `.git/`). Visible to humans inspecting the worktree, survives worktree repair, dies with the worktree on cleanup.

**State check approach — new dedicated CLI command:**

Add `liza check-commit-allowed <task-id>`. Exit codes are bi-state by design:
- **0 = allow** — policy permits the commit OR evaluation hit a fail-safe path (state unreadable, task not found, pipeline resolver unavailable)
- **1 = reject** — policy definitively forbids the commit

No third exit code. Fail-safe-allow is load-bearing: a transient state.yaml read error must not block commits.

The command reuses `models.IsExecutingStatus` and `pr.RejectedStatus` rather than re-implementing state-machine logic in shell. The absolute path to the `liza` binary is baked into each hook at install time (via `os.Executable()`), with a `command -v liza` fallback at exec time.

**Allowed states for commits:** The executing and rejected states for the task's role-pair, plus BLOCKED (agents may need to commit diagnostic files before marking blocked). All other states reject the commit.

### 2. Verdict recovery: auto-update review_commit for non-substantive drift

When `submit-verdict` detects a `review_commit` != HEAD mismatch, instead of failing immediately, check whether the diff between the two commits touches any files that overlap with the reviewer's review scope.

This is a **stretch goal** — the pre-commit hook prevents the problem entirely. But if it still occurs (hook bypassed, race condition), a softer recovery is better than permanent reviewer block.

**Not recommended for MVP** — the failure mode analysis is complex (which files are "substantive" for review?) and the pre-commit hook is sufficient.

## MVP Scope

### In scope

- Pre-commit hook script in `internal/embedded/hooks/`
- Hook installation in `wt_create.go`
- Hook checks task status, rejects commits when task is not in a working state
- Tests: hook installed, hook rejects commit in submitted state, hook allows commit in executing state

### Out of scope

- Verdict recovery (auto-update review_commit on non-substantive drift)
- Output entry re-validation on submission (stale `set-task-output` problem)
- Read-only worktree enforcement via filesystem permissions
- Retroactive fix for the diagnosis-design incident (already resolved manually)

## Success Criteria

1. A commit in a worktree whose task is in `REVIEWING_*` or `*_TO_REVIEW` state is rejected by the pre-commit hook with a clear error message
2. A commit in a worktree whose task is in an executing or rejected state succeeds normally
3. The hook is installed automatically during `liza wt-create` — no manual setup
4. Existing tests continue to pass (hook doesn't interfere with test worktrees)

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| Hook slows down commits | Use fast status check (regex on state.yaml or cached `liza get`); hook runs only in task worktrees |
| Hook blocks legitimate recovery commits | BLOCKED state is in the allow-list; human can `--no-verify` in emergencies |
| Test worktrees don't have real state.yaml | Hook must handle missing state gracefully (allow commit if state unreadable) |
| Agent bypasses hook with `--no-verify` | Contract already forbids `--no-verify` (CORE.md); prompt enforcement sufficient for this case |
