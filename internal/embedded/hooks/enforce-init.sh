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
cwd=$(json_val cwd)
command=$(json_val command)

project_dir="${CLAUDE_PROJECT_DIR:-}"
if [[ -z "$project_dir" ]]; then
  if [[ -n "$cwd" ]]; then
    project_dir=$(git -C "$cwd" rev-parse --show-toplevel 2>/dev/null || printf '%s' "$cwd")
  else
    project_dir=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
  fi
fi

# Fallback: $PPID is the Claude Code process PID (POSIX — the parent that forked this shell).
# Stable for the session lifetime since all hook invocations share the same parent process.
STATE_DIR="/tmp/liza-init-gate-${session_id:-ppid-$PPID}"
if ! mkdir -p "$STATE_DIR" 2>/dev/null; then
  echo "enforce-init: cannot create state dir $STATE_DIR, failing open" >&2
  exit 0
fi

# Fast path: gate already cleared.
if [[ -f "$STATE_DIR/CLEARED" ]]; then
  exit 0
fi

# Initialize: auto-clear GUARDRAILS.md if absent from project root.
if [[ ! -f "$STATE_DIR/GUARDRAILS.done" ]] && \
   [[ ! -f "$project_dir/GUARDRAILS.md" ]]; then
  touch "$STATE_DIR/GUARDRAILS.done"
fi

# Read-only discovery tools: allow before gate clears.
if [[ "$tool_name" == "ToolSearch" || "$tool_name" == "Glob" ]]; then
  exit 0
fi

mark_doc_reads_from_input() {
  if echo "$input" | grep -q 'AGENT_TOOLS\.md'; then
    touch "$STATE_DIR/AGENT_TOOLS.done"
  fi
  if echo "$input" | grep -qE 'PAIRING_MODE\.md|MULTI_AGENT_MODE\.md|SUBAGENT_MODE\.md'; then
    touch "$STATE_DIR/MODE.done"
  fi
  if echo "$input" | grep -q 'GUARDRAILS\.md'; then
    touch "$STATE_DIR/GUARDRAILS.done"
  fi
}

clear_if_ready() {
  if [[ -f "$STATE_DIR/AGENT_TOOLS.done" ]] && \
     [[ -f "$STATE_DIR/MODE.done" ]] && \
     [[ -f "$STATE_DIR/GUARDRAILS.done" ]]; then
    touch "$STATE_DIR/CLEARED"
  fi
}

is_required_doc_path() {
  case "$1" in
    "~/.liza/AGENT_TOOLS.md"|"$HOME"/.liza/AGENT_TOOLS.md) return 0 ;;
    "~/.liza/PAIRING_MODE.md"|"~/.liza/MULTI_AGENT_MODE.md"|"~/.liza/SUBAGENT_MODE.md") return 0 ;;
    "$HOME"/.liza/PAIRING_MODE.md|"$HOME"/.liza/MULTI_AGENT_MODE.md|"$HOME"/.liza/SUBAGENT_MODE.md) return 0 ;;
    "$project_dir"/GUARDRAILS.md|GUARDRAILS.md|./GUARDRAILS.md) return 0 ;;
    *) return 1 ;;
  esac
}

is_safe_bash_doc_read() {
  # The first check rejects shell metacharacters that would make token-level
  # validation misleading. Quoted paths are intentionally unsupported here:
  # Liza's required init docs do not need spaces or shell expansion.
  if echo "$command" | grep -qE '[;&|<>`\\]|[$][(]'; then
    return 1
  fi

  # shellcheck disable=SC2086
  set -- $command
  cmd="$1"
  shift || true

  case "$cmd" in
    cat)
      [[ "$#" -eq 1 ]] && is_required_doc_path "$1"
      ;;
    sed)
      # Allow common print ranges only. Explicitly excludes -i and extra files.
      [[ "$#" -eq 3 ]] || return 1
      [[ "$1" == "-n" ]] || return 1
      range="$2"
      range="${range#\'}"
      range="${range%\'}"
      range="${range#\"}"
      range="${range%\"}"
      [[ "$range" =~ ^[0-9]+(,[0-9]+)?p$ ]] || return 1
      is_required_doc_path "$3"
      ;;
    head|tail)
      if [[ "$#" -eq 1 ]]; then
        is_required_doc_path "$1"
      elif [[ "$#" -eq 3 && "$1" == "-n" && "$2" =~ ^[0-9]+$ ]]; then
        is_required_doc_path "$3"
      else
        return 1
      fi
      ;;
    wc)
      if [[ "$#" -eq 1 ]]; then
        is_required_doc_path "$1"
      elif [[ "$#" -eq 2 && "$1" =~ ^-[clmw]+$ ]]; then
        is_required_doc_path "$2"
      else
        return 1
      fi
      ;;
    *) return 1 ;;
  esac
}

# Codex currently exposes shell reads through the Bash hook surface. Allow a
# narrow set of read-only commands for the mandatory docs so the gate can clear
# even when MCP filesystem read tools are unavailable.
if [[ "$tool_name" == "Bash" ]] && echo "$command" | grep -qE 'AGENT_TOOLS\.md|PAIRING_MODE\.md|MULTI_AGENT_MODE\.md|SUBAGENT_MODE\.md|GUARDRAILS\.md'; then
  if ! is_safe_bash_doc_read; then
    cat <<EOF >&2
BLOCKED — session initialization allows Bash only for simple read-only doc commands.

Use cat, sed, head, tail, wc, or an MCP filesystem read tool on the required docs.
EOF
    exit 2
  fi

  mark_doc_reads_from_input
  clear_if_ready
  exit 0
fi

# Read calls always pass through; track mandatory doc reads. Codex filesystem
# MCP tools use "path"/"paths" while Claude uses "file_path".
if [[ "$tool_name" == "Read" || "$tool_name" =~ ^mcp__filesystem__read ]]; then
  file_path=$(json_val file_path)
  [[ -z "$file_path" ]] && file_path=$(json_val path)
  base=$(basename "$file_path")

  case "$base" in
    AGENT_TOOLS.md)   touch "$STATE_DIR/AGENT_TOOLS.done" ;;
    PAIRING_MODE.md|MULTI_AGENT_MODE.md|SUBAGENT_MODE.md)
                      touch "$STATE_DIR/MODE.done" ;;
    GUARDRAILS.md)    touch "$STATE_DIR/GUARDRAILS.done" ;;
  esac
  mark_doc_reads_from_input
  clear_if_ready

  exit 0
fi

# Non-Read tool call: block if gate not cleared.
# Stderr is shown to the agent as the block reason (exit 2 protocol).
missing=""
[[ ! -f "$STATE_DIR/AGENT_TOOLS.done" ]] && missing="$missing
  - ~/.liza/AGENT_TOOLS.md"
[[ ! -f "$STATE_DIR/MODE.done" ]] && missing="$missing
  - One of: ~/.liza/PAIRING_MODE.md, ~/.liza/MULTI_AGENT_MODE.md, or ~/.liza/SUBAGENT_MODE.md"
[[ ! -f "$STATE_DIR/GUARDRAILS.done" ]] && missing="$missing
  - GUARDRAILS.md (project root, or confirm absent)"

cat <<EOF >&2
BLOCKED — session initialization incomplete.

You must Read these files before using any other tool:
$missing

Use Read, an MCP filesystem read tool, or a simple Bash read command
(cat, sed, head, tail, wc) on each path above, then retry your action.
EOF
exit 2
