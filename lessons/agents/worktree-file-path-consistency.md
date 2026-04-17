---
title: "Worktree file path consistency"
trigger: "Reading, editing, or creating files in a worktree"
date: 2026-03-11
---

Claude Code's Edit tool tracks prior reads by exact string path. If you Read via one path and Edit via a slightly different string (even if both resolve to the same inode), the edit is rejected.

**Rules:**
1. Use the worktree path variable (`{{.WorktreePath}}`) for ALL file operations — reads, edits, writes, git commands.
2. Never retype, reconstruct, or abbreviate the path. Copy it from the template variable.
3. To create a new file: use Write directly. Do NOT attempt Read first — the file doesn't exist yet, so Read will error and block you.

**Recovery:**
- Edit rejected ("file not read"): re-Read the file using the exact same path string you will pass to Edit, then retry.
- Write rejected for a new file: fall back to Bash with a quoted heredoc:
  `cat > {{.WorktreePath}}/path <<'EOF'` … content … `EOF`
  (Single-quoted `'EOF'` prevents variable expansion.)

**Related lesson:**
[worktree-path-construction.md](worktree-path-construction.md) covers a different failure — prepending the task id inside an already-task-rooted worktree path (`.worktrees/<id>/<id>/`). Same path-variable discipline fixes both.
