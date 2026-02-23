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

Before running Go commands in the worktree, sync embedded assets from the main repo root:

```bash
cp -a "$PROJECT_ROOT/internal/embedded/claude-settings.json" \
  "$PROJECT_ROOT/internal/embedded/mcp.json" \
  "$PROJECT_ROOT/internal/embedded/contracts" \
  "$PROJECT_ROOT/internal/embedded/skills" \
  "$PROJECT_ROOT/.worktrees/<task-id>/internal/embedded/"
```

Or equivalently, run `make sync-embedded` from the worktree directory.

## References

- `internal/embedded/embedded.go` (the `go:embed` directives)
- `Makefile` (`sync-embedded` target)
