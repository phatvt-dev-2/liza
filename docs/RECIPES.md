# Operational Recipes

Step-by-step workflows for common Liza operations. See [DEMO.md](DEMO.md) for a full end-to-end tutorial.

## Autonomous Agent System

```bash
# Start monitoring
liza watch > watch.log 2>&1 &

# Start agents (each in background or separate terminal)
liza agent planner --agent-id planner-1 > planner.log 2>&1 &
liza agent coder --agent-id coder-1 > coder-1.log 2>&1 &
liza agent coder --agent-id coder-2 > coder-2.log 2>&1 &
liza agent code-reviewer --agent-id code-reviewer-1 > reviewer.log 2>&1 &

# Monitor progress
watch -n 5 'liza get tasks --format table'    # Task status
watch -n 5 'liza get agents --format table'   # Agent status
watch -n 10 'liza status'                     # Dashboard
tail -f .liza/alerts.log                      # Alerts

# Stop when done
liza stop --reason "work session complete"
wait  # Wait for all agents to exit cleanly
```

## Pause and Resume

Pause agents to make manual state adjustments.

```bash
# 1. Pause (agents block at next check, continue heartbeats)
liza pause --reason "Manual state adjustments"

# 2. Make changes
vi .liza/state.yaml
liza validate  # Always validate after manual edits

# 3. Resume
liza resume
```

**Pause vs Stop**: PAUSED agents stay alive and wait. STOPPED agents exit and must be restarted manually.

## Handling Task Rejection

When a reviewer rejects a task:

```bash
# Task status changes: READY_FOR_REVIEW -> REJECTED
# rejection_reason is set, review_cycles incremented

# With running agents: coder automatically reclaims REJECTED tasks
# The coder prompt includes the rejection feedback

# Manual flow:
liza claim-task task-1 coder-1
cd .worktrees/task-1

# Read rejection reason
yq '.tasks[] | select(.id == "task-1") | .rejection_reason' ../../.liza/state.yaml

# Fix issues, commit, resubmit
git add . && git commit -m "Address review feedback"
cd ../..
COMMIT=$(git -C .worktrees/task-1 rev-parse HEAD)
liza submit-for-review task-1 $COMMIT --agent-id coder-1
```

Watch daemon alerts on high review cycles (>= 5). Check with `liza get tasks task-1`.

## Recovering from Agent Crashes

**Automatic recovery**: Agents detect expired leases on restart and take over.

```bash
# Just restart the crashed agent
liza agent coder --agent-id coder-1
# Output: "Took over expired lease for coder-1"
```

**Manual recovery** (if agent won't restart):

```bash
# Force release the claim
liza release-claim task-1 --role coder --force

# Worktree is preserved -- check for in-progress work
cd .worktrees/task-1 && git status && git log
```

**Prevention**: Use `liza pause` before stopping agents. Avoid `kill -9` (use `kill` or Ctrl+C).

## Sprint Checkpoint

Review progress at end of sprint or major milestone.

```bash
# 1. Create checkpoint (agents pause, summary generated)
liza checkpoint --reason "Sprint 1 complete"

# 2. Review
cat .liza/sprint_summary.md
# Shows: task distribution, sprint metrics, active agents, anomalies

# 3. Analyze progress
liza get tasks --format table
liza get metrics

# 4. Make decisions and resume
liza resume
```

**When to checkpoint**: End of sprint, major milestone, before direction changes, weekly reviews.

## Circuit Breaker Analysis

When multiple tasks seem stuck with similar errors:

```bash
# 1. Run analysis
liza analyze

# If pattern detected:
# - circuit_breaker.status = TRIGGERED
# - config.mode = CIRCUIT_BREAKER_TRIPPED (agents halt)
# - Report written to .liza/circuit_breaker_report.md

# 2. Review report
cat .liza/circuit_breaker_report.md

# 3. Fix root cause (update spec, fix architecture, etc.)
# 4. Resume
liza resume
```

**Pattern types**:

| Pattern | Severity | Indicates |
|---------|----------|-----------|
| retry_cluster | ARCHITECTURE_FLAW | Design issue being repeatedly hit |
| debt_accumulation | SCOPE_FLAW | Too many workarounds |
| assumption_cascade | SPEC_FLAW | Wrong assumption propagating |
| spec_gap_cluster | SPEC_FLAW | Ambiguous spec causing repeated questions |
| workaround_pattern | ARCHITECTURE_FLAW | Root cause patched repeatedly |
| external_service_outage | EXTERNAL_DEPENDENCY | External service unavailable |

## Integration Failure Recovery

When a task merge to the integration branch fails (conflict or test failure):

```bash
# Task status: APPROVED -> INTEGRATION_FAILED
# Worktree preserved for resolution

# 1. Inspect
liza get tasks task-1
cd .worktrees/task-1
git status  # See conflicted files

# 2. Any coder can claim (worktree preserved automatically)
liza claim-task task-1 coder-2

# 3. Resolve (edit conflicts, fix tests)
cd .worktrees/task-1
# ... fix ...
git add . && git commit -m "Resolve integration conflict"

# 4. Resubmit (goes through review again)
cd ../..
COMMIT=$(git -C .worktrees/task-1 rev-parse HEAD)
liza submit-for-review task-1 $COMMIT --agent-id coder-2
```

**Prevention**: Keep task scope small, merge integration branch into task branches frequently.

## Using `liza get`

### Quick Status Checks

```bash
liza get config.mode        # RUNNING / PAUSED / STOPPED
liza get sprint.status      # IN_PROGRESS / CHECKPOINT
liza get sprint.elapsed     # 2d 5h 30m (computed)
```

### Task and Agent Queries

```bash
liza get tasks --format table         # All tasks
liza get tasks task-1                 # Single task detail
liza get agents --format table        # All agents
liza get metrics --format json        # Sprint metrics
liza get anomalies                    # Anomaly history
liza status                           # Full dashboard
```

### Scripting

```bash
MODE=$(liza get config.mode)
READY=$(liza get tasks --format json | jq '[.[] | select(.status=="READY")] | length')
DONE=$(liza get sprint.metrics.tasks_done)
```

Replaces `yq` queries: `liza get config.mode` instead of `yq '.config.mode' .liza/state.yaml`.

## Superseding Tasks

When a task is blocked or needs to be replaced:

```bash
# 1. Create replacement tasks
liza add-task --id task-4 --desc "..." --spec ... --done ... --scope ...
liza add-task --id task-5 --desc "..." --spec ... --done ... --scope ...

# 2. Supersede original
liza supersede-task task-3 task-4,task-5 "Split into smaller tasks due to complexity"

# 3. Clean up old worktree
liza wt-delete task-3
```

Used by the planner agent when tasks are BLOCKED, have failed multiple times, or need decomposition.
