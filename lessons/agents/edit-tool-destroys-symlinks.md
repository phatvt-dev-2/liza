---
title: "Never edit ~/.liza/ — always edit repo masters"
trigger: "When editing files under ~/.liza/, installed skill copies, or any path that might be a symlink"
date: 2026-02-20
updated: 2026-03-24
---

Files under `~/.liza/` are **installed copies**, not sources — some symlinks, some regular files. Edits there are lost on next install and diverge from the repo. The Edit tool also silently replaces symlinks with regular files.

**Rule:** Always edit the **repo master**, then `cp` to sync.

- `~/.liza/*.md` → `contracts/*.md`
- `~/.liza/skills/<name>/*` → `skills/<name>/*`

**Check:** `readlink -f <path>` (symlink?), `diff <repo> <installed>` (drift?).
