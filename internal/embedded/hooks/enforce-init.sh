#!/bin/bash
# PreToolUse hook: block non-Read tool calls until mandatory docs are read.
#
# Required reads:
#   - AGENT_TOOLS.md
#   - GUARDRAILS.md (or verified absent)
#   - One of: PAIRING_MODE.md, MULTI_AGENT_MODE.md, SUBAGENT_MODE.md
#
# No external dependencies (no jq, no sed -i). Portable across Linux and macOS.

input=$(cat)

# Extract JSON string values without jq — matches "key": "value" patterns.
json_val() { echo "$input" | grep -o "\"$1\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" | head -1 | sed 's/.*:[[:space:]]*"//;s/"$//'; }

tool_name=$(json_val tool_name)
session_id=$(json_val session_id)

# Fallback: $PPID is the Claude Code process PID (POSIX — the parent that forked this shell).
# Stable for the session lifetime since all hook invocations share the same parent process.
STATE_FILE="/tmp/liza-init-gate-${session_id:-ppid-$PPID}"

# Fast path: gate already cleared.
if [[ -f "$STATE_FILE" ]] && grep -q "CLEARED" "$STATE_FILE"; then
  exit 0
fi

# Initialize state file on first call.
if [[ ! -f "$STATE_FILE" ]]; then
  guardrails=0
  # Auto-clear GUARDRAILS.md if absent from project root.
  if [[ ! -f "${CLAUDE_PROJECT_DIR:-.}/GUARDRAILS.md" ]]; then
    guardrails=1
  fi
  echo "AGENT_TOOLS=0 MODE=0 GUARDRAILS=$guardrails" > "$STATE_FILE"
fi

# Portable in-place update (no sed -i, which differs between GNU and BSD).
update_state() {
  local old="$1" new="$2" tmp
  tmp="${STATE_FILE}.tmp"
  sed "s/$old/$new/" "$STATE_FILE" > "$tmp" && mv "$tmp" "$STATE_FILE"
}

# Read calls always pass through; track mandatory doc reads.
if [[ "$tool_name" == "Read" ]]; then
  file_path=$(json_val file_path)
  base=$(basename "$file_path")

  case "$base" in
    AGENT_TOOLS.md)   update_state 'AGENT_TOOLS=0' 'AGENT_TOOLS=1' ;;
    PAIRING_MODE.md|MULTI_AGENT_MODE.md|SUBAGENT_MODE.md)
                      update_state 'MODE=0' 'MODE=1' ;;
    GUARDRAILS.md)    update_state 'GUARDRAILS=0' 'GUARDRAILS=1' ;;
  esac

  # Check if all requirements met.
  if grep -q "AGENT_TOOLS=1" "$STATE_FILE" && \
     grep -q "MODE=1" "$STATE_FILE" && \
     grep -q "GUARDRAILS=1" "$STATE_FILE"; then
    echo "CLEARED" >> "$STATE_FILE"
  fi

  exit 0
fi

# Non-Read tool call: block if gate not cleared.
missing=""
grep -q "AGENT_TOOLS=0" "$STATE_FILE" && missing="$missing AGENT_TOOLS.md"
grep -q "MODE=0" "$STATE_FILE" && missing="$missing (one mode contract)"
grep -q "GUARDRAILS=0" "$STATE_FILE" && missing="$missing GUARDRAILS.md"

echo "Blocked: must read mandatory docs before any other action. Missing:$missing" >&2
exit 2
