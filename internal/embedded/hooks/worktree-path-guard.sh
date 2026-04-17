#!/bin/bash
# PreToolUse hook: catch the .worktrees/<id>/<id>/ path-duplication bug.
#
# Agents occasionally concatenate the worktree root (already .worktrees/<id>)
# with a relative path that itself starts with the task id, producing
# .worktrees/<id>/<id>/… and burning turns on ENOENT before self-diagnosing.
# This hook fails fast on file-tool calls carrying that pattern.
#
# Coverage:
#   - Read, Write, Edit — VERIFIED (tool_input.file_path per Claude Code docs)
#   - MultiEdit — BEST-EFFORT, UNVERIFIED. Shipped because the hook is
#     one-sided safe: if MultiEdit does send file_path, we catch the bug;
#     if it sends something else, the json_val extraction returns empty
#     and we exit 0 (silent no-op, no false deny). Future maintainers:
#     do NOT treat this as confirmed protection until a real PreToolUse
#     payload is captured. See TECH_DEBT.md.
#   - NotebookEdit — NOT COVERED. Same unverified status as MultiEdit
#     but less common, so not worth shipping the matcher entry either.
#     See TECH_DEBT.md.
#
# Not gated on LIZA_AGENT_ID: the failure mode hits Pairing sessions too.
# Portable across Linux and macOS: uses bash 3.2+ regex + string compare,
# no GNU-only grep -P. No external dependencies (no jq).

input=$(cat)

# Extract JSON string values without jq. Matches "key": "value" anywhere in
# the blob — file_path is unique enough inside the Read/Write/Edit/MultiEdit
# payload shapes that a cross-key collision isn't a practical concern.
json_val() { echo "$input" | grep -o "\"$1\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" | head -1 | sed 's/.*:[[:space:]]*"//;s/"$//'; }

file_path=$(json_val file_path)

if [[ -z "$file_path" ]]; then
    exit 0
fi

# Detect .worktrees/<segment>/<same-segment>. POSIX ERE has no
# backreferences, so split: capture the two segments after .worktrees/ and
# compare them as strings. Works with bash 3.2+ (macOS default).
if [[ "$file_path" =~ \.worktrees/([^/]+)/([^/]+) ]]; then
    first="${BASH_REMATCH[1]}"
    second="${BASH_REMATCH[2]}"
    if [[ "$first" == "$second" ]]; then
        cat <<EOF
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Worktree path duplicates the task id segment (.worktrees/$first/$second). The worktree root already ends in the task id — strip the duplicate segment from your relative path. Path: $file_path"}}
EOF
    fi
fi
