# Worktree Management Protocol

## Lifecycle

| Event | Action | Actor |
|-------|--------|-------|
| Task IMPLEMENTING (fresh) | Create worktree via `liza claim-task` | Supervisor |
| Task IMPLEMENTING (reassignment) | Create fresh worktree via `liza claim-task` | Supervisor |
| Task APPROVED | Merge eligible | — |
| Task MERGED | `liza wt-merge task-N` | Supervisor (after Code Reviewer approves) |
| Task BLOCKED | Delete worktree: `liza wt-delete task-N` | Planner |
| Task ABANDONED/SUPERSEDED | Delete worktree: `liza wt-delete task-N` | Planner |
| Task INTEGRATION_FAILED | Worktree retained for conflict resolution | — |

**Note:** Worktree creation is supervisor-only (via `liza claim-task`), not agent-callable. This ensures worktrees exist before agents are spawned.

**Reassignment rule:** When a different coder claims a task (after REJECTED or BLOCKED → READY), the worktree is deleted and recreated fresh. Same coder re-claiming keeps the existing worktree. Rationale: salvaging failed work often costs more than restarting from spec.

---

## Naming

`.worktrees/task-{id}/` — one directory per task.

---

## Branch Strategy

```
main
  └── integration  (all approved work merges here)
        ├── .worktrees/task-1/  (branched from integration)
        ├── .worktrees/task-2/
        └── .worktrees/task-3/
```

Merge to main is human-triggered, not part of Liza flow.

---

## Commit Permissions

| Actor | Can Commit To |
|-------|---------------|
| Coder | Task worktree branch only |
| Code Reviewer | None (read-only; approves for merge) |
| Planner | Neither (no code changes) |
| Supervisor | Integration branch (executes merge after APPROVED) |

**Hard rule:** Coders cannot commit to or merge to integration. Only the supervisor can merge, and only after Code Reviewer approval.

---

## Worktree Rules

1. Coder works only in assigned task's worktree
2. Code Reviewer examines same worktree (read-only)
3. No cross-worktree file access
4. No direct commits to integration branch

---

## Lease Expiration and Worktree State

When a coder's lease expires:

1. **Task becomes reclaimable** — status stays IMPLEMENTING but lease_expires is in the past
2. **Original coder must self-abort** — if they return after expiry, they exit immediately
3. **Worktree handling depends on who supervisor assigns:**
   - Same coder: worktree preserved (agent returning after brief network issue)
   - Different coder: supervisor deletes and recreates worktree fresh

**Design Rationale:**
- Same coder reclaiming: preserve work (crash recovery)
- Different coder reclaiming: fresh start (salvaging failed work costs more than restarting)
- Handoff notes (if written) provide context regardless of worktree state

---

## Staleness Detection

Before starting work, coder checks if worktree base is stale:

```bash
git fetch origin integration
git merge-base --is-ancestor integration HEAD || echo "STALE"
```

If stale, coder decides based on:

| Condition | Risk | Action |
|-----------|------|--------|
| No conflicts after rebase attempt | Low | Auto-rebase, continue |
| Task touches ≤2 files, no shared modules | Low | Auto-rebase, continue |
| Task touches shared code (utils, models, API) | High | BLOCKED, planner decides |
| Merge conflicts detected | High | BLOCKED, planner decides |
| Integration branch has schema/API changes | High | BLOCKED, planner decides |

**Decision rule:** If in doubt, BLOCKED is safer than silent rebase. Planner can always unblock.

---

## Drift Tracking

Tasks record `base_commit` (integration HEAD) at claim time. This enables drift visibility:

**At claim:**
```bash
base_commit=$(git rev-parse integration)
# Stored in task.base_commit
```

**At merge** (implemented by `liza wt-merge`):
```bash
current_integration=$(git rev-parse integration)
base_commit=<task.base_commit from blackboard>
drift_commits=$(git rev-list --count $base_commit..$current_integration)
```

**Sprint summary includes drift metrics:**
- Tasks with `drift_commits > 0` at merge indicate accumulated staleness
- High drift correlates with integration failure risk
- "Last task penalty": sequential tasks accumulate drift; later tasks have higher merge conflict probability

**Retrospective signal:** If drift consistently high, consider:
- Smaller sprints
- More frequent integration checkpoints
- Prioritizing tasks that touch shared code early

---

## Integration Protocol

After APPROVED, **Code Reviewer** executes:

1. Verify `review_commit` matches current HEAD
2. Run `liza wt-merge task-N`
3. Script attempts fast-forward or clean merge
4. If conflict: task → INTEGRATION_FAILED, Code Reviewer reports
5. If integration tests fail: task → INTEGRATION_FAILED
6. On success: task → MERGED, worktree deleted

---

## Integration-Fix Ownership

When task is INTEGRATION_FAILED:

1. Task becomes claimable by any coder
2. Claim scope is explicitly "resolve conflicts / fix integration"
3. Original implementation is not re-reviewed
4. Only the conflict resolution is reviewed
5. Mark in blackboard: `integration_fix: true`

This prevents planner paralysis on merge conflicts.

---

## Clean Sync Invariant

Before setting READY_FOR_REVIEW, coder must ensure:

```bash
# Working tree clean
[ -z "$(git -C $WORKTREE status --porcelain)" ] || abort "Uncommitted changes"

# Record commit for Code Reviewer
COMMIT_SHA=$(git -C $WORKTREE rev-parse HEAD)
```

**Definition of "clean":**
- No staged changes (index matches HEAD)
- No unstaged changes (working tree matches index)
- No untracked files (except .gitignored)
- `git status --porcelain` returns empty string
- Submodule state is not checked (out of scope for v1)

Blackboard records `review_commit: $COMMIT_SHA`. Code Reviewer verifies this SHA before reviewing.

---

## Commit SHA Verification

Code Reviewer must verify before examining work (implemented by supervisor at review claim time):

```
ACTUAL  = git -C $WORKTREE rev-parse HEAD
EXPECTED = task.review_commit from blackboard

if ACTUAL != EXPECTED:
    ERROR: Worktree modified since review requested
```

## Related Documents

- [Task Lifecycle](task-lifecycle.md) — claim, iterate, review
- [Tooling](../implementation/tooling.md) — `liza wt-create`, `liza wt-merge`, `liza wt-delete`
- [Roles](../architecture/roles.md) — commit permissions
