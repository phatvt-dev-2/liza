# Activation of the Contract for Pairing Agents

Check [Genesis](../README.md#genesis) for the features.

**WARNING**: Gemini and Mistral are not able to fully comply with the contract.
It's not possible to make them comply strictly with instructions. They are not recommended models for Liza.
Prefer Claude or Codex.

## Two-Layer Settings Architecture

Claude Code unions permissions from **global** and **project** settings. Both are needed:

| Layer | File | Managed by | Contains |
|-------|------|-----------|----------|
| **Project** | `<project>/.claude/settings.json` | `liza init` (automatic) | Liza MCP tools, skills, git/build commands |
| **Global** | `~/.claude/settings.json` | Manual (one-time, below) | Personal MCP tools, `additionalDirectories`, `Read(~/.liza/**)` |

`liza init` writes the project layer from the master [`claude-settings.json`](../claude-settings.json). **Do not hand-craft a subset** — agents will be blocked on any missing permission.

The global layer (below) adds machine-specific tools and paths that don't belong in project settings.

## Central Config

The recommended way to set up `~/.liza/` is:
```bash
liza setup          # one-time: writes contracts + skills to ~/.liza/
liza setup --force  # overwrite existing
```

**Manual / development fallback** — create symlinks (useful when developing liza itself):
```bash
LIZA_DIR=~/Workspace/liza  # adjust to your liza checkout
mkdir -p ~/.liza
cd ~/.liza
ln -s $LIZA_DIR/contracts/CORE.md
ln -s $LIZA_DIR/contracts/PAIRING_MODE.md
ln -s $LIZA_DIR/contracts/MULTI_AGENT_MODE.md
ln -s $LIZA_DIR/contracts/SUBAGENT_MODE.md
ln -s $LIZA_DIR/contracts/AGENT_TOOLS.md
ln -s $LIZA_DIR/contracts/COLLABORATION_CONTINUITY.md
ln -s $LIZA_DIR/skills
```

## Claude

Create symlinks:
```bash
cd ~/.claude
mkdir -p skills
for i in ~/.liza/skills/* ; do ln -s "$i" skills/$(basename "$i") ; done
```

The contract is followed more strictly if the symlink is created at every repo root.
`liza init` creates these symlinks automatically. For repos not managed by liza:
```bash
cd <REPO_ROOT>
ln -s ~/.liza/CORE.md CLAUDE.md
```

In `~/.claude/settings.json`, configure **global** permissions — things that apply across all projects. Adapt to your own MCP tools and paths:

```json
{
  "additionalDirectories": ["~/.liza"],
  "permissions": {
    "defaultMode": "acceptEdits",
    "allow": [
      "Read(~/.claude/**)",
      "Read(~/.liza/**)",

      "Skill(adr-backfill)",
      "Skill(clean-code)",
      "Skill(code-review)",
      "Skill(debugging)",
      "Skill(feynman)",
      "Skill(generic-subagent)",
      "Skill(software-architecture-review)",
      "Skill(spec-review)",
      "Skill(systemic-thinking)",
      "Skill(testing)",
      "Skill(black-box-red-testing)",

      "WebFetch",
      "WebSearch",
      "LSP",

      "Bash(liza:*)",
      "Bash(curl:*)",
      "Bash(wget:*)",
      "Bash(jq:*)",
      "Bash(yq:*)",
      "Bash(sort:*)",
      "Bash(uniq:*)",
      "Bash(cut:*)",
      "Bash(tr:*)",
      "Bash(diff:*)",
      "Bash(realpath:*)",
      "Bash(dirname:*)",
      "Bash(basename:*)",
      "Bash(cd:*)",
      "Bash(echo:*)",
      "Bash(which:*)",
      "Bash(file:*)",
      "Bash(tree:*)",
      "Bash(env:*)",
      "Bash(printenv:*)",
      "Bash(gh:*)",
      "Bash(git add:*)",
      "Bash(git checkout:*)",
      "Bash(git commit:*)",
      "Bash(git status:*)",
      "Bash(git diff:*)",
      "Bash(git log:*)",
      "Bash(git show:*)",
      "Bash(git branch:*)",
      "Bash(git blame:*)",
      "Bash(git ls-files:*)",
      "Bash(git grep:*)",
      "Bash(git worktree:*)",
      "Bash(git stash:*)",
      "Bash(git rev-parse:*)",
      "Bash(pre-commit:*)",
      "Bash(python:*)",
      "Bash(python3:*)",
      "Bash(pytest:*)",
      "Bash(shellcheck:*)",
      "Bash(bash:*)",
      "Bash(ls:*)",
      "Bash(cat:*)",
      "Bash(head:*)",
      "Bash(tail:*)",
      "Bash(wc:*)",
      "Bash(date:*)",
      "Bash(find:*)",
      "Bash(grep:*)"
    ]
  }
}
```

Then add your personal MCP tools (IDE integration, documentation servers, etc.) to the same `allow` array. These vary by machine — see your `~/.claude.json` for available MCP servers.

**Permission categories:**
- `"defaultMode": "acceptEdits"` — Required for Liza agents to work headless (preferred to `"bypassPermissions"` aka YOLO mode)
- `Read(~/.liza/**)` — Access to contract files
- `Bash(liza:*)` — Execution of Liza CLI commands
- `Skill(...)` — Custom skills from `~/.liza/skills/`
- `WebFetch/WebSearch/LSP` — Built-in Claude tools for web and code navigation
- Other `Bash(...)` — Safe read-only shell commands (no package managers)

If agents get blocked on additional tools, add them to your global settings. Refer to "Debug a stuck agent interactively" in [DEMO.md](../docs/DEMO.md#troubleshooting) to identify blocking commands.

Verification:
- Run `claude`
- Prompt `hello`

## Codex

Create symlinks:
```bash
mkdir -p ~/.codex/skills
cd ~/.codex
for i in ~/.liza/skills/* ; do ln -s "$i" skills/$(basename "$i") ; done
```

The contract is followed more strictly if the symlink is created at every repo root:
```bash
cd <REPO_ROOT>
ln -s ~/.liza/CORE.md AGENTS.md
```

Edit `~/.codex/config.toml` (replace `<USER>` with your username):

```toml
approval_policy = "on-failure"
sandbox_mode = "workspace-write"

[sandbox_workspace_write]
network_access = true
writable_roots = ["/home/<USER>/.codex", "/home/<USER>/.liza", "/home/<USER>/.cache"]

[mcp_servers.filesystem]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-filesystem", "/home/<USER>/.claude", "/home/<USER>/.codex", "/home/<USER>/Workspace", "/home/<USER>/.liza"]
```

## Gemini

Create symlinks:
```bash
mkdir -p ~/.gemini/skills
cd ~/.gemini
for i in ~/.liza/skills/* ; do ln -s "$i" skills/$(basename "$i") ; done
```

The contract is followed more strictly if the symlink is created at every repo root:
```bash
cd <REPO_ROOT>
ln -s ~/.liza/CORE.md GEMINI.md
```

Add to ~/.gemini/settings.json:

```json
{
  "context": {
    "includeDirectories": [
      "~/.liza",
      "~/Workspace/liza"
    ]
  }
}
```

## Mistral

Symlink the contract as instructions and add skills:
```bash
mkdir -p ~/.vibe/prompts ~/.vibe/skills
cd ~/.vibe
ln -s ~/.liza/CORE.md prompts/liza.md
for i in ~/.liza/skills/* ; do ln -s "$i" skills/$(basename "$i") ; done
```

Modify `~/.vibe/config.toml`:
- Add `system_prompt_id = "liza"` (replace `system_prompt_id = "cli"` with)
- Add MCP filesystem server (replace `mcp_servers = []` with):
```toml
[[mcp_servers]]
name = "filesystem"
transport = "stdio"
command = "npx"
args = ["-y", "@modelcontextprotocol/server-filesystem", "/home/<USER>/.vibe", "/home/<USER>/Workspace", "/home/<USER>/.liza"]
```

Replace `<USER>` with your username in the paths above.

Verification:
- Run `vibe`
- Prompt `Hello. You MUST follow the contract.` ("hello" is not enough for Gemini and Mistral)

## Kimi (with Claude CLI)

```bash
export ANTHROPIC_BASE_URL=https://api.kimi.com/coding/
export ANTHROPIC_API_KEY=$KIMI_API_KEY
export ANTHROPIC_MODEL="kimi-k2.5"
```
Then run `claude`

Kimi uses Claude's config automatically.
