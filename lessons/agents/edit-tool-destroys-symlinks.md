---
title: "Edit tool destroys symlinks"
trigger: "When editing files under ~/.liza/ or any path that might be a symlink"
date: 2026-02-20
---

The Edit tool silently replaces symlinks with regular files. The master is untouched; the runtime copy becomes a detached orphan.

**Rule:** Never edit `~/.liza/` files directly. Edit repo masters:
- CORE.md → `contracts/CORE.md`
- Skills → `skills/<name>/SKILL.md`

Check targets: `readlink -f <path>`. Restore: `rm <path> && ln -s <target> <path>`.
