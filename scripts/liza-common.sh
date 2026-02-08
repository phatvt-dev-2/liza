#!/bin/bash
# Common functions for Liza scripts
# Source this file: source "$(dirname "$0")/liza-common.sh"

# Get the main project root, even when called from inside a worktree
# Usage: PROJECT_ROOT=$(get_project_root)
get_project_root() {
    local toplevel git_common_dir
    toplevel=$(git rev-parse --show-toplevel 2>/dev/null)
    git_common_dir=$(realpath "$(git rev-parse --git-common-dir 2>/dev/null)")

    # In a worktree, .git is a file; git-common-dir points to main repo's .git
    # Main repo: git-common-dir == toplevel/.git
    # Worktree:  git-common-dir == <main>/.git (parent of toplevel)
    if [[ "$git_common_dir" != "$toplevel/.git" ]]; then
        # We're in a worktree - common dir is <main>/.git
        dirname "$git_common_dir"
    else
        echo "$toplevel"
    fi
}

# Get standard Liza paths
# Sets: PROJECT_ROOT, LIZA_DIR, STATE, LOG, LOCK
setup_liza_paths() {
    PROJECT_ROOT=$(get_project_root)
    readonly PROJECT_ROOT
    readonly LIZA_DIR="$PROJECT_ROOT/.liza"
    readonly STATE="$LIZA_DIR/state.yaml"
    readonly LOG="$LIZA_DIR/log.yaml"
    readonly LOCK="$STATE.lock"
}

# ISO timestamp in UTC
iso_timestamp() {
    date -u +%Y-%m-%dT%H:%M:%SZ
}

# ISO timestamp with offset
# Usage: iso_timestamp_offset "+60 seconds"
iso_timestamp_offset() {
    date -u -d "$1" +%Y-%m-%dT%H:%M:%SZ
}

# Verify task exists in blackboard, exit 1 if not
# Usage: require_task_exists "$TASK_ID" "$STATE"
require_task_exists() {
    local task_id="$1"
    local state_file="$2"
    if ! yq -e ".tasks[] | select(.id == \"$task_id\")" "$state_file" > /dev/null 2>&1; then
        echo "ERROR: Task '$task_id' not found in blackboard" >&2
        exit 1
    fi
}

# Normalize a commit reference (short/full SHA, branch, tag) to full 40-char SHA
# Usage: full_sha=$(normalize_sha "/path/to/repo" "abc123")
# Exits with error if commit not found
normalize_sha() {
    local repo_dir="$1"
    local commit_ref="$2"
    local full_sha
    if ! full_sha=$(git -C "$repo_dir" rev-parse "$commit_ref" 2>/dev/null); then
        echo "ERROR: commit '$commit_ref' not found in $repo_dir" >&2
        return 1
    fi
    echo "$full_sha"
}

# Fatal error with optional exit code
# Usage: die "message" or die 3 "message"
die() {
    local code=1
    if [[ "$1" =~ ^[0-9]+$ ]]; then
        code="$1"
        shift
    fi
    echo "ERROR: $*" >&2
    exit "$code"
}

# Current epoch seconds
epoch_now() {
    date +%s
}

# Convert ISO timestamp to epoch seconds
to_epoch() {
    date -d "$1" +%s 2>/dev/null || echo 0
}

# Count tasks matching a yq filter (requires $STATE)
count_tasks() {
    local filter="$1"
    yq "[.tasks[] | select($filter)] | length" "$STATE" 2>/dev/null || echo 0
}
