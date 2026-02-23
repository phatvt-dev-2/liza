---
title: "Worktree missing embedded assets breaks Go commands"
trigger: "When `go test` or `go run ./cmd/liza` fails with `pattern claude-settings.json: no matching files found`"
keywords: [go:embed, claude-settings.json, internal/embedded, worktree, go test]
date: 2026-02-23
---

## Context

Task worktrees may be missing embedded resource files under `internal/embedded/` even when the main repo has them.

## Failure Mode

Any build/test path that compiles `internal/embedded/embedded.go` fails because `go:embed` cannot find `claude-settings.json`, `mcp.json`, `contracts/`, or `skills/`.

## Solution

Before running Go commands in the worktree, sync embedded assets from the main repo:

```bash
cp -a /home/tangi/Workspace/liza/internal/embedded/claude-settings.json \
  /home/tangi/Workspace/liza/internal/embedded/mcp.json \
  /home/tangi/Workspace/liza/internal/embedded/contracts \
  /home/tangi/Workspace/liza/internal/embedded/skills \
  /home/tangi/Workspace/liza/.worktrees/<task-id>/internal/embedded/
```

## References

- `/home/tangi/Workspace/liza/internal/embedded/embedded.go`
- `/home/tangi/Workspace/liza/.worktrees/unify-grace-period/internal/embedded/`
