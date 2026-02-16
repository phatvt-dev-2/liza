# Troubleshooting Guide

Common issues and solutions when running Liza.

---

## Agent Issues

### COLLISION: agent already registered

**Error:**
```
COLLISION: planner-1 already registered until 2026-01-20T09:53:55Z
ERROR: Failed to register agent planner-1 (collision?)
```

**Cause:** Agent was killed (Ctrl+C, SIGKILL, crash) before it could unregister. The stale entry persists in `state.yaml`.

**Solutions:**

1. **Wait for lease expiry** — The timestamp shows when the lease expires. After that time, re-registration will succeed.

2. **Manual cleanup** — Remove the stale agent entry:
   ```bash
   yq -i 'del(.agents."planner-1")' .liza/state.yaml
   ```

3. **Use a different agent ID:**
   ```bash
   liza agent planner --agent-id planner-2
   ```

**Prevention:** Use `liza pause` before stopping agents — they'll exit gracefully at next check.

### Agent Timeout (Execution Exceeds Time Limit)

**Symptoms:**
- Agent shows WORKING/REVIEWING/PLANNING status for extended period
- Logs show: "Agent execution timeout (CLI may be hung, will retry)"
- Agent status automatically resets to IDLE after timeout

**Timeouts by role:** Reviewer 30min, Coder 2hr, Planner 4hr. See [CONFIGURATION.md](CONFIGURATION.md#agent-execution-timeouts).

**Expected behavior:** Supervisor kills CLI, resets agent to IDLE, retries after 5s delay. This is self-recovering.

**When to investigate:**
- Frequent timeouts indicate underlying issues (bad test, broken tooling)
- Check agent prompt files (`.liza/agent-prompts/`) to see work in progress
- If timeouts recur on same task, consider marking it BLOCKED

### Agent stuck in loop, not claiming tasks

**Diagnosis:**
```bash
tail -f coder-1.log
# "Waiting for claimable tasks..." → No tasks available
# "Task task-1 blocked on dependencies" → Dependencies not met
```

**Common causes and fixes:**
- No claimable tasks → `liza add-task ...`
- Dependencies not met → check with `liza get tasks <dep-id>`
- System paused → `liza get config.mode` then `liza resume`
- Sprint at checkpoint → `liza get sprint.status` then `liza resume`

---

## Lock and Concurrency Issues

Liza uses file-based locking with classified error types for targeted diagnostics.

### Lock Error Classification

| Type | Meaning |
|------|---------|
| `lock error (timeout)` | Failed to acquire lock within timeout |
| `lock error (stale)` | Lock held by dead process |
| `lock error (disk_full)` | No space left on device |
| `lock error (permission)` | Permission denied on lock file |
| `lock error (filesystem)` | I/O error or filesystem issue |

### timeout — failed to acquire lock

**Diagnosis:**
```bash
ps aux | grep liza                     # Running processes
cat .liza/.state.yaml.lock.pid         # PID of lock holder
ps -p $(cat .liza/.state.yaml.lock.pid)  # Is holder alive?
```

**Common causes:** Long-running operation under lock, many agents competing, hung process, slow filesystem (network mount).

**Solutions:**
- If holder is alive and working → wait 30-60s, retry
- If holder is hung → `kill $(cat .liza/.state.yaml.lock.pid)`, lock auto-cleans on next access
- If high contention → reduce parallel agents (1-3 per role)
- If slow filesystem → `df -T .liza/`, move to local SSD

### stale — lock held by dead process

**Automatic cleanup:** Liza detects stale locks via PID tracking — checks if lock holder process exists, removes lock files if dead, then retries.

**Manual cleanup** (if auto-cleanup fails):
```bash
ps aux | grep liza                    # Verify NO liza processes running
rm .liza/.state.yaml.lock .liza/.state.yaml.lock.pid
```

**Common causes:** `kill -9` (SIGKILL), system crash, OOM killer. **Prevention:** Use `kill` or Ctrl+C for graceful shutdown.

### disk_full — no space left on device

```bash
df -h .liza/        # Check disk space
df -i .liza/        # Check inodes
du -sh .worktrees/* # Find large worktrees
```

**Free space:** Clean merged worktrees, archive old log files, `git gc --aggressive --prune=now`.

### permission — permission denied

```bash
ls -la .liza/.state.yaml.lock*   # Check lock file permissions
ls -ld .liza/                     # Check directory permissions
stat .liza/state.yaml             # Check ownership
```

**Common causes:** Lock file owned by different user (ran as root, now as user), NFS mount with restrictive permissions, SELinux/AppArmor blocking.

**Fix:** `sudo chown -R $(whoami):$(id -gn) .liza/`

### filesystem — I/O error

```bash
dmesg | grep -i error | tail -20   # Filesystem errors
touch .liza/test.txt && rm .liza/test.txt  # Test write
```

**Common causes:** Failing drive, filesystem corruption, network FS timeout, readonly mount. If on NFS/SMB, check network connectivity and remount.

### "state modified by another process, retry"

This is normal — the three-phase claim pattern detected a race condition and will retry automatically. If persistent, too many agents may be competing for the same tasks.

---

## State Validation Failures

### Missing required field

**Error:** `INVALID: missing required field 'sprint'`

**Cause:** State file manually edited or created with old init. **Solution:** Add the missing section per `specs/architecture/blackboard-schema.md`.

### Task status invariant violation

**Error:** `INVALID: CLAIMED task without assigned_to: task-1`

**Fix the invariant:**
```bash
# Option 1: Assign the agent
yq -i '(.tasks[] | select(.id == "task-1")).assigned_to = "coder-1"' .liza/state.yaml

# Option 2: Reset to UNCLAIMED
yq -i '(.tasks[] | select(.id == "task-1")).status = "UNCLAIMED"' .liza/state.yaml
```

### Circular dependency detected

**Identify the cycle:**
```bash
yq '.tasks[] | {id, depends_on}' .liza/state.yaml | grep -A5 task-1
# Example cycle: task-1 → task-2 → task-3 → task-1
```

**Break cycle:** Remove one dependency:
```bash
yq -i 'del(.tasks[] | select(.id == "task-3") | .depends_on[] | select(. == "task-1"))' .liza/state.yaml
```

### Spec file not found

```bash
# Option 1: Create the spec file
mkdir -p specs && vi specs/vision.md

# Option 2: Update spec reference in state
yq -i '(.tasks[] | select(.id == "task-1")) |= .spec_ref = "docs/requirements.md"' .liza/state.yaml
```

---

## Worktree Issues

### Worktree directory not found

**Error:** `INVALID: CLAIMED task task-1 has worktree=.worktrees/task-1 but directory does not exist`

**Recreate:** `git worktree add .worktrees/task-1 -b task-1`

**Or reset task** (if work was lost):
```bash
yq -i '(.tasks[] | select(.id == "task-1")).status = "UNCLAIMED"' .liza/state.yaml
yq -i '(.tasks[] | select(.id == "task-1")).assigned_to = null' .liza/state.yaml
yq -i '(.tasks[] | select(.id == "task-1")).worktree = null' .liza/state.yaml
```

### Worktree already exists

```bash
# Option 1: Delete and recreate
liza wt-delete task-1
liza wt-create task-1

# Option 2: Manual cleanup
git worktree remove .worktrees/task-1
rm -rf .worktrees/task-1
git branch -D task/task-1
liza wt-create task-1
```

### Cannot remove worktree: branch is checked out

```bash
git worktree remove .worktrees/task-1 --force
```

### Worktree directory is dirty

```bash
cd .worktrees/task-1 && git status

# Commit, stash, or discard as appropriate
git add . && git commit -m "Save progress"   # save
git stash                                      # stash
git reset --hard HEAD                          # discard
```

### Invalid reference: task/task-1

Task branch doesn't exist:
```bash
git branch -a | grep task/          # List task branches
git branch task/task-1 integration  # Recreate from integration
```

---

## Integration Issues

### Integration branch doesn't exist

```bash
git branch integration main
```

### Task marked INTEGRATION_FAILED

When an APPROVED task's merge to integration fails (conflict or test failure):
- Task status changes APPROVED → INTEGRATION_FAILED
- Merge is aborted, integration branch reverted
- Worktree preserved for conflict resolution

See [RECIPES.md](RECIPES.md#integration-failure-recovery) for the full recovery workflow.

**Quick diagnosis:**
```bash
liza get tasks task-1                    # Check failure details
cd .worktrees/task-1 && git status       # See conflicted files
```

**Merge conflict:** Edit files with conflict markers (`<<<<<<<`/`=======`/`>>>>>>>`), resolve, commit. **Test failure:** Run `go test ./...` to reproduce, fix, commit.

Either way: claim the task, fix in worktree, resubmit for review. The resolution goes through normal review before merge retry.

**Prevention:** Keep task scope small, merge integration branch into task branches frequently.

---

## Watcher Alerts

### False positive alerts on fresh state

**Symptom:** Alerts like `ORPHANED REJECTED` or `IMMEDIATE DISCOVERY` with empty task/agent names.

**Cause:** Empty arrays in `state.yaml` produce empty lines that get processed as valid entries.

**Solution:** Update to latest `liza` binary which includes empty-line guards.

### INVALID STATE alert

**Error:** `🚨 INVALID STATE: Agent coder-1 has status WORKING but lease expired`

**Common causes:** Agent lease expired (task took longer than `lease_duration`), task status invariant violation.

**Fix:** Extend the lease or increase `config.lease_duration`. See [CONFIGURATION.md](CONFIGURATION.md#configuration-matrix) for tuning parameters.

---

## Initialization Issues

### Error: .liza already exists

**Solutions:**
1. **Continue with existing state** — just start the agents.
2. **Reset completely:** `rm -rf .liza .worktrees && liza init "New goal"`

### Error: specs/vision.md required

Create the vision spec first:
```bash
mkdir -p specs
cat > specs/vision.md << 'EOF'
# Vision: Project Name
## Goal
[What you want to build]
## Requirements
[List of requirements]
## Success Criteria
[How to verify completion]
EOF
```

---

## Performance Issues

See [PERFORMANCE.md](PERFORMANCE.md) for tuning parameters, benchmark targets, and lock metric interpretation.

### Agents not responding to changes (30s+ delay)

fsnotify may not work on network filesystems (NFS, SMB). Agents fall back to 30s polling. **Fix:** Use local filesystem, or accept the delay.

Also check: system mode is RUNNING (`liza get config.mode`), sprint status is IN_PROGRESS (`liza get sprint.status`).

### Slow state file reads (>50ms)

```bash
ls -lh .liza/state.yaml   # Check size (target <1MB)
time liza validate          # Time a read
```

**Common causes:** Large state file, slow disk, cache thrashing from external modifications (editors with auto-save, Dropbox syncing `.liza/`).

**Fix:** Archive completed tasks, use SSD, avoid external modifications.

### Slow task claims (5-10s)

Git worktree operations slow on large repos. **Fix:** Use SSD, consider sparse checkout (`git config core.sparseCheckout true`).

### Validation takes too long (>5s)

Large task list or complex dependency graph. **Fix:** Archive old tasks, `liza validate --skip-spec-check` on slow filesystems.

---

## Debugging Techniques

### Verbose output

```bash
liza -v validate              # Timing, detailed errors, internal ops
liza -v claim-task task-1 coder-1
```

### Inspect state

```bash
liza get tasks --format table   # All task statuses
liza get tasks task-1           # Single task detail
liza get agents --format table  # All agents
liza get metrics                # Sprint metrics
liza status                     # Full dashboard
```

### Review logs

```bash
cat .liza/log.yaml              # Activity log
cat .liza/alerts.log            # Alerts
ls .liza/agent-prompts/         # Generated agent prompts
cat .liza/agent-prompts/coder-1-*.txt  # What the agent was told
```

### Monitor locks

```bash
ls -la .liza/.state.yaml.lock*   # Lock file exists?
lsof .liza/.state.yaml.lock      # Who holds it?
cat .liza/.state.yaml.lock.pid   # Lock holder PID
```

### Watch real-time

```bash
watch -n 2 'liza get tasks --format table'
watch -n 2 'liza get agents --format table'
watch -n 5 'liza status'
watch -n 5 'ls -la .worktrees/'
```

### Debug report

```bash
cat > debug-report.txt <<EOF
=== Liza Debug Report ===
Version: $(liza version)
State Validation: $(liza validate 2>&1)
Tasks: $(liza get tasks --format table 2>&1)
Agents: $(liza get agents --format table 2>&1)
Worktrees: $(git worktree list)
Recent Alerts: $(tail -50 .liza/alerts.log 2>/dev/null)
Processes: $(ps aux | grep liza)
Lock Status: $(ls -la .liza/.state.yaml.lock* 2>&1)
EOF
```

---

## Recovery Procedures

### Full state reset (nuclear option)

```bash
cp -r .liza .liza.backup.$(date +%Y%m%d-%H%M%S)
rm -rf .liza
liza init "Goal description"
# Manually migrate in-progress work from backup if needed
```

### Agent stuck in WORKING state

```bash
# Clear agent entry
yq -i 'del(.agents."coder-1")' .liza/state.yaml

# Reset task to UNCLAIMED (loses uncommitted work)
yq -i '(.tasks[] | select(.assigned_to == "coder-1")).status = "UNCLAIMED"' .liza/state.yaml
yq -i '(.tasks[] | select(.assigned_to == "coder-1")).assigned_to = null' .liza/state.yaml

# Or keep CLAIMED and restart with same agent ID (preserves worktree)
```

---

## Getting Help

1. **Check alerts:** `cat .liza/alerts.log`
2. **Check activity:** `cat .liza/log.yaml`
3. **Validate state:** `liza validate`
4. **Watch live:** `liza watch`
5. **Generate debug report** (see above)
