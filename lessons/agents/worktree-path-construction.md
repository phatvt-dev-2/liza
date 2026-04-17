---
title: "Worktree path construction — never prepend the task id"
trigger: "Reading, editing, or creating files in a worktree"
date: 2026-04-17
---

The worktree root already ends in the task id: `.worktrees/<task-id>`. Relative paths inside the worktree are rooted there — you do NOT prepend the task id again.

**Anti-pattern observed (burned 72 turns before self-diagnosing):**
```
/path/to/project/.worktrees/my-task-id/my-task-id/specs/plans/...
                            ^^^^^^^^^ ^^^^^^^^^
                            worktree  (duplicate, always wrong)
```

This happens when the agent holds two fragments in memory — the absolute worktree root AND a task-named relative path — and concatenates them without noticing the overlap. Every access returns ENOENT for a file that exists one level up, and the agent often fails to recognize the pattern.

**Rules:**
1. Every path variable (`{{.WorktreePath}}`, `{{.Worktree}}`) already points at `.worktrees/<task-id>`. Join only with the path *below* that root.
2. If you catch yourself typing `.worktrees/<id>/<id>/`, strip one segment.
3. If you see repeated ENOENT on paths that "should exist," immediately run `ls` on the parent dir to confirm the layout before constructing more paths.

**Runtime guard:**
A `PreToolUse` hook (`worktree-path-guard.sh`) denies Read/Write/Edit calls whose `file_path` matches `.worktrees/<id>/<id>/`. The deny reason names the duplicate segment so the fix is obvious.

**Related lesson:**
[worktree-file-path-consistency.md](worktree-file-path-consistency.md) covers a different quirk — Claude Code's Edit tool tracking reads by exact string match. Same overall prescription (use the path variable, don't reconstruct), different root cause.
