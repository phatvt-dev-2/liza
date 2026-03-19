---
title: "Update master files, not derived copies"
trigger: "When modifying claude-settings.json, .mcp.json, or any file that has a master/embedded source and derived copies"
keywords: [claude-settings.json, .claude/settings.json, embedded, liza init, permissions, MCP tools]
date: 2026-03-06
---

## Context

Configuration files like `claude-settings.json` exist in three places:

1. **Master**: `claude-settings.json` (repo root) — human-editable reference
2. **Embedded**: `internal/embedded/claude-settings.json` — compiled into the binary, written by `liza init`
3. **Derived**: `.claude/settings.json` — project-active copy, created by `liza init`

## Failure Mode

Editing only the derived copy (`.claude/settings.json`) fixes the immediate problem but:
- Next `liza init` overwrites the fix from the stale embedded template
- Other projects initialized with `liza init` never get the fix
- The master reference diverges from what's actually deployed

## Solution

Always update the master first, then propagate to derived copies:

1. `claude-settings.json` (repo root master)
2. `internal/embedded/claude-settings.json` (embedded template)
3. `.claude/settings.json` (active project copy — or re-run `liza init`)

Verify sync: `diff <(grep 'mcp__liza' claude-settings.json | sort) <(grep 'mcp__liza' internal/embedded/claude-settings.json | sort)`

## References

- `claude-settings.json` — master
- `internal/embedded/claude-settings.json` — embedded template
- `internal/embedded/embedded.go:WriteClaudeSettings()` — merge logic
