---
title: "Sync embedded assets before Go builds in Liza worktrees"
trigger: "When running `go build` or `go test` in a Liza worktree"
keywords: [go build, go test, make sync-embedded, internal/embedded, go:embed, worktree]
date: 2026-04-12
---

## Context

Liza embeds contracts and skills from `internal/embedded/` at build time. In a worktree, those embedded copies can lag behind the repo masters after branch switches or edits.

## Failure Mode

Bare `go build` can fail in `internal/embedded` when embedded assets are stale. Agents then debug the compile error instead of refreshing the generated copies.

For tests, the stronger repo rule still applies: prefer `make test` over bare `go test ./...`. The stale-embedded failure mode is the reason that rule exists.

## Solution

For test runs, prefer:

`make test`

If a worktree build or test failure points to stale embedded assets, sync them from the worktree root:

`make sync-embedded`

If the shell is not at the worktree root, run:

`make -C <worktree-root> sync-embedded`

## References

- [REPOSITORY.md](../../REPOSITORY.md)
- [internal/embedded/consistency_test.go](../../internal/embedded/consistency_test.go)
- [specs/architecture/ADR/0031-configurable-post-worktree-command.md](../../specs/architecture/ADR/0031-configurable-post-worktree-command.md)
