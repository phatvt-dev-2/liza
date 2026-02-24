---
title: "Run sync-embedded before Go tests in a worktree"
trigger: "When running go test in a fresh worktree and embed patterns fail"
keywords: [go:embed, internal/embedded, make sync-embedded, claude-settings.json, mcp.json, cannot embed irregular file, symlink]
date: 2026-02-23
---

## Context

This repo embeds contracts, skills, and settings from `internal/embedded/`, but those files are generated/synced into the worktree rather than fully tracked in git.

## Failure Mode

Running `go test` directly in a fresh worktree can fail compilation with errors like:
`pattern claude-settings.json: no matching files found`.
The `go:embed` targets in `internal/embedded/embedded.go` are missing until sync runs.
Trying to shortcut with symlinks can fail too: `cannot embed irregular file`.

## Solution

Run `make sync-embedded` in the worktree before running `go test` or `go test ./...`.
Use `make test` when possible, since it includes the sync prerequisite.
If you must patch manually, use regular files/directories (not symlinks) under `internal/embedded/`.

## References

- `Makefile` (sync-embedded target and test prerequisite)
- `REPOSITORY.md` (embedded asset build/test requirement)
- `internal/embedded/embedded.go` (`go:embed` patterns)
