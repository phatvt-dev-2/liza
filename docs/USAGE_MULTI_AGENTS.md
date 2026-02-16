# Liza - Usage Guide

## Activation of the Contract for Pairing Agents

See [Contract Activation](../contracts/contract-activation.md).

## Liza

See [DEMO](DEMO.md) for a full example.

### Project Structure

```
~/.liza/
├── CORE.md                        → contracts/CORE.md (symlink)
├── contracts/
│   ├── CORE.md                    # Universal rules + mode selection gate
│   ├── PAIRING_MODE.md            # Human-supervised collaboration
│   └── MULTI_AGENT_MODE.md        # Peer-supervised Liza system
└── schemas/
    └── liza-state.yaml            # Blackboard schema

<project>/
├── .liza/
│   ├── state.yaml                 # Current state
│   ├── log.yaml                   # Activity history
│   └── archive/                   # Terminal-state tasks
└── .worktrees/
    └── task-N/                    # Per-task workspace
```

### Quick Start (Target Usage)

**Prerequisites:**
- Claude Code CLI and git installed
- Go >= 1.25.5 installed
- `liza` and `liza-mcp` Go binaries in PATH

**Installing the Liza CLI:**

```bash
# Build
make build

# Copy to a directory in PATH
sudo cp liza liza-mcp /usr/local/bin/

# Verify
liza version
```

**1. Initialize**
```bash
# Create .liza/ directory with blackboard
liza init "[Goal description]" --spec [spec_ref]

# spec_ref: Path to goal specification (default: specs/vision.md)
# Examples:
#   liza init "Implement retry logic"                        # uses specs/vision.md
#   liza init "Add auth" --spec specs/auth-feature.md        # uses custom spec

# Verify
cat .liza/state.yaml
```

`liza init` creates:
- `.liza/state.yaml` — Blackboard state
- `.liza/log.yaml` — Activity log
- `.liza/contracts/` — Embedded agent contracts (CORE.md, PAIRING_MODE.md, MULTI_AGENT_MODE.md, etc.)
- `.liza/skills/` — Embedded skill definitions (code-review, debugging, testing, etc.)
- `.liza/specs/` — Embedded system specifications (architecture, protocols, implementation)
- `.claude/claude-settings.json` — Claude Code permissions (if using Claude Code)
- `.mcp.json` — MCP server configuration (tells Claude Code how to start liza-mcp)
- `.worktrees/` — Task worktrees directory
- `integration` branch — For merging completed work

Embedded files include YAML frontmatter with version metadata (`liza_version`, `liza_git_commit`, `liza_build_date`) to track which version your project is using.

**2. Start Agents (3 terminals)**

Agent identity is provided via the `--agent-id` flag. IDs must follow the pattern `{role}-{number}` (e.g., `coder-1`, `code-reviewer-1`, `planner-1`).

Terminal 1 — Planner:
```bash
liza agent planner --agent-id planner-1
```

Terminal 2 — Coder:
```bash
liza agent coder --agent-id coder-1
```

Terminal 3 — Code Reviewer:
```bash
liza agent code-reviewer --agent-id code-reviewer-1
```

**3. Observe**
```bash
# Run the watcher for alerts
liza watch
```

```bash
# Watch blackboard state
watch -n 2 'yq ".tasks[] | pick([\"id\", \"status\", \"description\"])" .liza/state.yaml'
```

**4. Human Interventions**
```bash
# Pause all agents
liza pause

# Resume
liza resume

# Abort
liza stop

# Checkpoint (halt + generate summary)
liza checkpoint
```

**5. Review Results**
```bash
# Activity log
cat .liza/log.yaml

# Integration branch
git log integration --oneline
```

### CLI Commands

The `liza` binary provides all system operations. Key commands:

| Command | Purpose |
|---------|---------|
| `liza init <goal> --spec <spec_ref>` | Initialize .liza/ directory with blackboard (spec_ref defaults to specs/vision.md) |
| `liza agent <role> --agent-id <id>` | Agent supervisor (start, restart, backoff loop) |
| `liza claim-task <task-id> <agent-id>` | Atomically claim a task for a coder (creates worktree, updates state) |
| `liza validate [state.yaml]` | Validate blackboard state against schema invariants |
| `liza watch` | Monitor blackboard and alert on anomalies |
| `liza release-claim <task-id> [--role R]` | Release claim on a task (crash recovery) |
| `liza checkpoint` | Create a checkpoint (halt + summary) |
| `liza status` | Show system status |
| `liza pause` / `liza resume` | Pause/resume system |
| `liza stop` / `liza start` | Stop/start system |

**Important:** The supervisor claims tasks *before* starting the Claude agent. This avoids interactive permission prompts in `-p` (non-interactive) mode. Agents receive their assigned task in the bootstrap prompt and should NOT call claim commands directly.

See [Architecture Overview](../specs/architecture/overview.md) for detailed component descriptions.

### Configuring Claude Code (MCP)

Liza integrates with Claude Code through the Model Context Protocol (MCP). `liza init` creates the configuration automatically:

**`.mcp.json`** — MCP server configuration:
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

**`claude-settings.json`** — Minimal permissions for Claude Code agents:
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
      "Bash(git add:*)",
      "Bash(git commit:*)",
      "Bash(git status:*)",
      "Bash(git diff:*)",
      "WebFetch"
    ]
  }
}
```

Both CLI commands (e.g., `liza add-task`) and MCP tools (e.g., `liza_add_task`) operate on the same `.liza/state.yaml` file. Claude Code agents use MCP tools for better error handling; the CLI is for manual use.

The root-level `claude-settings.json` and `mcp.json` are templates embedded into the binary. `liza init` writes the active copies to `.claude/claude-settings.json` and `.mcp.json` in the project directory.
