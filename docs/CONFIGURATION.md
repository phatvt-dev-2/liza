# Configuration Reference

System configuration, tuning parameters, and environment variables.

## MCP Server Setup

Liza provides an MCP server (`liza-mcp`) for Claude Code integration. `liza init` creates both configuration files automatically. If they already exist, `liza init` prompts to merge Liza-specific configuration.

### What Gets Created

**`.mcp.json`** — tells Claude Code how to start the Liza MCP server:

```json
{
  "mcpServers": {
    "liza": {
      "command": "liza-mcp",
      "args": ["--project-root", "."]
    }
  }
}
```

**`.claude/settings.json`** — project-level permissions for MCP tools, file access, and git operations:

```json
{
  "additionalDirectories": [ "~/.liza" ],
  "permissions": {
    "defaultMode": "acceptEdits",
    "allow": [
      "Read(~/.claude/**)",
      "Read(~/.liza/**)",
      "mcp__liza__liza_get",
      "mcp__liza__liza_status",
      "mcp__liza__liza_add_task",
      "mcp__liza__liza_submit_for_review",
      "mcp__liza__liza_submit_verdict",
      "mcp__liza__liza_handoff",
      "Bash(git add:*)",
      "Bash(git commit:*)",
      "Bash(git status:*)",
      "Bash(git diff:*)",
      "WebFetch"
    ]
  }
}
```

**Key elements:**
- **`mcp__liza__*`** — grants permission to invoke specific MCP tools (format: `mcp__<server>__<tool>`)
- **`additionalDirectories`** — allows reading Liza's global config directory (`~/.liza`)
- **`defaultMode: acceptEdits`** — required for headless agent operation

The repo-root [`claude-settings.json`](../claude-settings.json) and [`mcp.json`](../mcp.json) are reference templates. `liza init` writes the runtime files to `.claude/settings.json` and `.mcp.json` in the project root.

For Pairing mode setup (symlinks, provider-specific config), see [Contract Activation](../contracts/contract-activation.md).

### Troubleshooting MCP

**Server won't start:**
- Verify `liza-mcp` is in PATH: `which liza-mcp`
- Check `.mcp.json` exists in project root
- Ensure `.liza/` directory exists

**Permission denied:**
- Verify `settings.json` includes MCP tool permissions (e.g., `"mcp__liza__liza_add_task"`)
- Ensure `enableAllProjectMcpServers: true` is set
- Ensure `enabledMcpjsonServers: ["liza"]` is set

**State file errors:**
- Verify project initialized: `liza validate`
- Check: `ls -la .liza/state.yaml`

### CLI vs MCP

Both interfaces operate on the same `state.yaml` with proper locking.

| | CLI (`liza add-task`) | MCP (`liza_add_task`) |
|---|---|---|
| Use case | Manual / interactive | Programmatic / agent |
| Output | Human-readable text | Structured JSON |
| Error handling | Exit codes + stderr | JSON error responses |

## Configuration Matrix

All configuration lives in `.liza/state.yaml` under the `config` section.

| Parameter | Default | Min | Max | Unit | Purpose |
|-----------|---------|-----|-----|------|---------|
| `max_coder_iterations` | 10 | 1 | 100 | count | Max iterations per coder per task |
| `max_review_cycles` | 5 | 1 | 20 | count | Max review rejection cycles |
| `heartbeat_interval` | 60 | 10 | 300 | seconds | Heartbeat frequency |
| `lease_duration` | 1800 | 300 | 7200 | seconds | Task lease duration |
| `coder_poll_interval` | 30 | 5 | 120 | seconds | Check interval (legacy, now event-driven) |
| `coder_max_wait` | 1800 | 300 | 7200 | seconds | Max idle before agent exits |

### Agent Execution Timeouts

| Role | Timeout | Rationale |
|------|---------|-----------|
| Code Reviewer | 30 min | Reviews should complete quickly |
| Coder | 2 hours | Implementation takes longer |
| Planner | 4 hours | Complex planning needs time |

When exceeded, supervisor kills CLI, resets agent to IDLE, retries after 5s delay.

## Tuning Guidelines

### Short Tasks (<10 min)
```yaml
config:
  heartbeat_interval: 30
  lease_duration: 900       # 15 min
  coder_max_wait: 600       # 10 min
  max_coder_iterations: 5   # Escalate fast
```

### Long Tasks (30min-2hr)
```yaml
config:
  heartbeat_interval: 60
  lease_duration: 3600      # 1 hour
  coder_max_wait: 7200      # 2 hours
  max_coder_iterations: 15  # More iterations
```

### Network Filesystems (NFS, SMB)
```yaml
config:
  heartbeat_interval: 90    # Less frequent writes
  lease_duration: 2700      # 45 min
  # fsnotify may not work -- agents fall back to polling
```

### Fast Feedback
```yaml
config:
  max_coder_iterations: 5   # Escalate faster
  max_review_cycles: 3      # Fewer rejection cycles
  heartbeat_interval: 30    # Faster crash detection
```

## System Modes

| Mode | Agents | Heartbeats | Set by |
|------|--------|------------|--------|
| `RUNNING` | Work normally | Yes | `liza resume` / `liza start` |
| `PAUSED` | Block, don't claim | Yes | `liza pause` |
| `STOPPED` | Exit cleanly | Stop | `liza stop` |
| `CIRCUIT_BREAKER_TRIPPED` | Halt | Yes | `liza analyze` (auto) |

**PAUSED**: Agents stay alive, resume instantly. Use for manual edits.
**STOPPED**: Agents exit. Must restart manually. Use for end of session.

```
RUNNING <-> PAUSED (liza pause / liza resume)
RUNNING -> STOPPED (liza stop)
STOPPED -> RUNNING (liza start, then restart agents)
CIRCUIT_BREAKER_TRIPPED -> RUNNING (liza resume, after fixing root cause)
```

## Task Lifecycle States

| Status | Claimable | Reviewable | Terminal |
|--------|-----------|------------|----------|
| DRAFT | No | No | No |
| UNCLAIMED | Yes | No | No |
| CLAIMED | No | No | No |
| READY_FOR_REVIEW | No | Yes | No |
| REJECTED | Yes | No | No |
| APPROVED | No | No | No |
| MERGED | No | No | **Yes** |
| BLOCKED | No | No | No |
| ABANDONED | No | No | **Yes** |
| SUPERSEDED | No | No | **Yes** |
| INTEGRATION_FAILED | Yes | No | No |

## Agent Identity

Agent identity can be provided in two ways:

1. **CLI flag** (recommended): `liza agent coder --agent-id coder-1`
2. **Environment variable**: `export LIZA_AGENT_ID=coder-1`

The `--agent-id` flag takes precedence over `LIZA_AGENT_ID`.

**Agent ID format**: `{role}-{number}` — e.g. `coder-1`, `code-reviewer-1`, `planner-1`.

**System commands** (`pause`, `stop`, `start`, `resume`, `release-claim`) use `--changed-by` for audit trail (defaults to `human`).

## Environment Variables

| Variable | Required | Default | Purpose |
|----------|----------|---------|---------|
| `LIZA_AGENT_ID` | For agent commands | -- | Agent identifier (format: `{role}-{number}`) |
| `LIZA_SPECS` | No | `specs/` | Path to specs directory (relative to project root) |
| `LIZA_LOG_LEVEL` | No | `INFO` | Logging verbosity: DEBUG, INFO, WARN, ERROR |

## Making Configuration Changes

1. `liza pause --reason "config update"`
2. Edit `state.yaml` (or use commands)
3. `liza validate`
4. `liza resume`

**Never edit state.yaml while agents are running** without pausing first.
