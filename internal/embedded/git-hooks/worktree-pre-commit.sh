#!/bin/sh
# Liza worktree pre-commit hook.
# Rejects commits when the task is not in a state that permits coder mutations.
# Fail-safe: any error during evaluation allows the commit (see check-commit-allowed).
#
# Installed by `liza wt-create` into each task worktree via:
#   git config extensions.worktreeConfig true
#   git config --worktree core.hooksPath <worktree>/.liza-hooks
#
# The absolute path to the liza binary is baked in at install time. If the
# binary is missing at commit time, the hook falls back to allow (belt-and-
# braces with submit-verdict's HEAD check).

LIZA_BIN="__LIZA_BIN__"
TASK_ID="__TASK_ID__"

if [ ! -x "$LIZA_BIN" ]; then
    LIZA_BIN="$(command -v liza 2>/dev/null)"
fi
if [ -z "$LIZA_BIN" ] || [ ! -x "$LIZA_BIN" ]; then
    exit 0
fi

"$LIZA_BIN" check-commit-allowed "$TASK_ID"
ec=$?
# Only 0 (allow) and 1 (reject) are meaningful policy outcomes. Any other
# exit code (missing subcommand on a stale binary, panic/abort, signal
# death, etc.) must fall through to allow — the hook is a guard, not a
# lock, and a crashing evaluator must never deadlock commits.
case "$ec" in
    0|1) exit "$ec" ;;
    *)   exit 0 ;;
esac
