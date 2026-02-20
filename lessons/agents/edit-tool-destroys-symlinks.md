---
title: "Edit tool destroys symlinks"
trigger: "When editing files under ~/.liza/ or any path that might be a symlink"
keywords: [symlink, Edit, ~/.liza, CORE.md, SKILL.md, ln -s, regular file]
date: 2026-02-20
---

## Context

The Liza project uses symlinks extensively:
- `~/.liza/CORE.md` → `~/Workspace/liza/contracts/CORE.md` (master)
- `~/Workspace/liza/CLAUDE.md` → `~/.liza/CORE.md`
- `~/Workspace/liza/AGENTS.md` → `~/.liza/CORE.md`
- `~/.liza/skills/<name>/SKILL.md` are installed copies (not symlinks, but not masters either)

Master files live in the repo. Runtime copies under `~/.liza/` are either symlinks or installer-generated copies.

## Failure Mode

The Edit tool replaces a symlink with a regular file containing the edited content. The symlink is silently destroyed — the master file is untouched, and the runtime copy is now a detached orphan. Subsequent installs or syncs won't fix this because the symlink no longer exists.

## Solution

1. **Never edit `~/.liza/` files directly.** Always edit the repo master:
   - CORE.md master: `contracts/CORE.md`
   - Skill masters: `skills/<name>/SKILL.md`
2. If you need to check what a runtime path points to: `readlink -f <path>`
3. If a symlink was accidentally destroyed: `rm <path> && ln -s <target> <path>`

## References

- `contracts/CORE.md` — master contract
- `skills/` — master skill definitions
- `internal/embedded/skills` — build copy (not git-tracked, not master)
