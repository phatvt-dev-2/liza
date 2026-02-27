#!/usr/bin/env python3
"""Analyze Liza agent log files for context usage patterns.

Reads NDJSON log files produced by `claude --verbose --output-format stream-json`
and prints a human-readable report of token usage, content breakdown, and cost.

Two log formats are supported:
  - Rich (Format A): first event type is "system". Per-API-call token breakdown.
  - Sparse (Format B): first event type is "thread.started". Aggregate usage only.

Usage:
    python3 scripts/analyze-log.py .liza/agent-outputs/planner-*.txt
    python3 scripts/analyze-log.py .liza/agent-outputs/*.txt
"""

from __future__ import annotations

import json
import sys
from dataclasses import dataclass, field
from pathlib import Path

# ---------------------------------------------------------------------------
# Data classes
# ---------------------------------------------------------------------------


@dataclass
class SessionMeta:
    file: str = ""
    format: str = ""  # "rich" or "sparse"
    model: str = ""
    session_id: str = ""
    duration_ms: int = 0
    num_turns: int = 0
    context_window: int = 0
    max_output_tokens: int = 0


@dataclass
class TurnUsage:
    """Token usage for a single API call (rich format only)."""

    message_id: str = ""
    input_tokens: int = 0
    cache_creation_input_tokens: int = 0
    cache_read_input_tokens: int = 0
    output_tokens: int = 0

    @property
    def total_input(self) -> int:
        return self.input_tokens + self.cache_creation_input_tokens + self.cache_read_input_tokens


@dataclass
class ContentItem:
    """A single content item from the log."""

    item_type: str = ""  # reasoning, agent_message, command_execution, etc.
    item_id: str = ""
    chars: int = 0
    preview: str = ""


@dataclass
class SessionReport:
    meta: SessionMeta = field(default_factory=SessionMeta)
    # Aggregate usage (both formats)
    total_input_tokens: int = 0
    total_cache_creation: int = 0
    total_cache_read: int = 0
    total_output_tokens: int = 0
    total_cost_usd: float = 0.0
    # Per-turn (rich only)
    turns: list[TurnUsage] = field(default_factory=list)
    # Content items (both formats)
    items: list[ContentItem] = field(default_factory=list)
    # Tool call frequency (both formats)
    tool_calls: dict[str, int] = field(default_factory=dict)
    # MCP server status (rich only)
    mcp_servers: list[dict[str, str]] = field(default_factory=list)


# ---------------------------------------------------------------------------
# Format detection
# ---------------------------------------------------------------------------


def detect_format(first_line: str) -> str:
    """Return 'rich' or 'sparse' based on the first JSON event."""
    try:
        obj = json.loads(first_line)
    except json.JSONDecodeError:
        return "unknown"
    event_type = obj.get("type", "")
    if event_type == "system":
        return "rich"
    if event_type == "thread.started":
        return "sparse"
    return "unknown"


# ---------------------------------------------------------------------------
# Rich format parser (Format A)
# ---------------------------------------------------------------------------


def _measure_content_block(block: dict) -> ContentItem:
    """Extract a ContentItem from a rich-format content block."""
    block_type = block.get("type", "unknown")
    text = ""
    if block_type == "thinking":
        text = block.get("thinking", "")
    elif block_type == "text":
        text = block.get("text", "")
    elif block_type == "tool_use":
        text = json.dumps(block.get("input", {}))
    elif block_type == "tool_result":
        content = block.get("content", "")
        if isinstance(content, str):
            text = content
        elif isinstance(content, list):
            parts = []
            for part in content:
                if isinstance(part, dict):
                    parts.append(part.get("text", ""))
                elif isinstance(part, str):
                    parts.append(part)
            text = "\n".join(parts)
    return ContentItem(
        item_type=block_type,
        chars=len(text),
        preview=text[:120].replace("\n", " "),
    )


def parse_rich(lines: list[str]) -> SessionReport:
    """Parse a rich-format (Format A) log file."""
    report = SessionReport()
    report.meta.format = "rich"

    seen_message_ids: dict[str, TurnUsage] = {}

    for line in lines:
        line = line.strip()
        if not line:
            continue
        try:
            obj = json.loads(line)
        except json.JSONDecodeError:
            continue

        event_type = obj.get("type", "")

        if event_type == "system":
            report.meta.session_id = obj.get("session_id", "")
            report.meta.model = obj.get("model", "")
            for srv in obj.get("mcp_servers", []):
                report.mcp_servers.append(
                    {
                        "name": srv.get("name", ""),
                        "status": srv.get("status", ""),
                    }
                )

        elif event_type == "assistant":
            msg = obj.get("message", {})
            msg_id = msg.get("id", "")
            usage = msg.get("usage", {})

            # Dedup: only count usage once per message.id
            if msg_id and msg_id not in seen_message_ids:
                turn = TurnUsage(
                    message_id=msg_id,
                    input_tokens=usage.get("input_tokens", 0),
                    cache_creation_input_tokens=usage.get("cache_creation_input_tokens", 0),
                    cache_read_input_tokens=usage.get("cache_read_input_tokens", 0),
                    output_tokens=usage.get("output_tokens", 0),
                )
                seen_message_ids[msg_id] = turn
                report.turns.append(turn)

            # Extract content items and tool call names
            for block in msg.get("content", []):
                item = _measure_content_block(block)
                if item.chars > 0:
                    report.items.append(item)
                if block.get("type") == "tool_use":
                    name = block.get("name", "unknown")
                    report.tool_calls[name] = report.tool_calls.get(name, 0) + 1

        elif event_type == "user":
            msg = obj.get("message", {})
            for content_part in msg.get("content", []):
                if isinstance(content_part, dict):
                    item = _measure_content_block(content_part)
                    if item.chars > 0:
                        item.item_type = "tool_result"
                        report.items.append(item)
                    # Also check nested content in tool results
                    nested = content_part.get("content", "")
                    if isinstance(nested, str) and nested:
                        report.items.append(
                            ContentItem(
                                item_type="tool_result",
                                chars=len(nested),
                                preview=nested[:120].replace("\n", " "),
                            )
                        )
                    elif isinstance(nested, list):
                        for part in nested:
                            if isinstance(part, dict):
                                text = part.get("text", "")
                                if text:
                                    report.items.append(
                                        ContentItem(
                                            item_type="tool_result",
                                            chars=len(text),
                                            preview=text[:120].replace("\n", " "),
                                        )
                                    )

        elif event_type == "result":
            report.meta.duration_ms = obj.get("duration_ms", 0)
            report.meta.num_turns = obj.get("num_turns", 0)
            report.total_cost_usd = obj.get("total_cost_usd", 0.0)
            usage = obj.get("usage", {})
            model_usage = obj.get("modelUsage", {})
            for model_name, mu in model_usage.items():
                report.meta.model = report.meta.model or model_name
                report.meta.context_window = mu.get("contextWindow", 0)
                report.meta.max_output_tokens = mu.get("maxOutputTokens", 0)

    # Compute totals from deduped turns
    for turn in report.turns:
        report.total_input_tokens += turn.input_tokens
        report.total_cache_creation += turn.cache_creation_input_tokens
        report.total_cache_read += turn.cache_read_input_tokens
        report.total_output_tokens += turn.output_tokens

    return report


# ---------------------------------------------------------------------------
# Sparse format parser (Format B)
# ---------------------------------------------------------------------------


def _measure_sparse_item(item: dict) -> ContentItem:
    """Extract a ContentItem from a sparse-format item."""
    item_type = item.get("type", "unknown")
    item_id = item.get("id", "")
    text = ""

    if item_type == "command_execution":
        text = item.get("aggregated_output", "") or ""
        cmd = item.get("command", "")
        preview = f"[{cmd[:80]}] {text[:40]}"
    elif item_type == "reasoning":
        text = item.get("text", "")
        preview = text[:120]
    elif item_type == "agent_message":
        text = item.get("text", "")
        preview = text[:120]
    elif item_type == "file_change":
        changes = item.get("changes", [])
        text = json.dumps(changes)
        paths = [c.get("path", "") for c in changes]
        preview = ", ".join(paths)[:120]
    elif item_type == "mcp_tool_call":
        result = item.get("result", {})
        args = item.get("arguments", "")
        server = item.get("server", "")
        tool = item.get("tool", "")
        result_text = json.dumps(result) if isinstance(result, dict) else str(result)
        args_text = json.dumps(args) if isinstance(args, dict) else str(args)
        text = args_text + result_text
        preview = f"[{server}/{tool}] {result_text[:80]}"
    else:
        text = json.dumps(item)
        preview = text[:120]

    return ContentItem(
        item_type=item_type,
        item_id=item_id,
        chars=len(text),
        preview=preview.replace("\n", " "),
    )


def _extract_command_name(cmd: str) -> str:
    """Extract a short command name from a shell command string."""
    # Strip shell wrappers like `/usr/bin/zsh -lc "..."`
    for prefix in ("/usr/bin/zsh -lc ", "/bin/bash -lc ", "/bin/sh -c "):
        if cmd.startswith(prefix):
            inner = cmd[len(prefix) :].strip().strip("'\"")
            # Get first token of the inner command
            first = inner.split()[0] if inner.split() else cmd
            # Strip 'set +e;' or similar preambles
            if first in ("set", "echo", "if", "cd"):
                parts = inner.split("&&")
                if len(parts) > 1:
                    first = parts[-1].strip().split()[0]
            return first
    return cmd.split()[0] if cmd.split() else cmd


def parse_sparse(lines: list[str]) -> SessionReport:
    """Parse a sparse-format (Format B) log file."""
    report = SessionReport()
    report.meta.format = "sparse"

    for line in lines:
        line = line.strip()
        if not line:
            continue
        try:
            obj = json.loads(line)
        except json.JSONDecodeError:
            continue

        event_type = obj.get("type", "")

        if event_type == "thread.started":
            report.meta.session_id = obj.get("thread_id", "")

        elif event_type == "item.completed":
            item = obj.get("item", {})
            # Only count completed items (skip in_progress starts)
            if item.get("status") in ("completed", "failed", None):
                ci = _measure_sparse_item(item)
                if ci.chars > 0:
                    report.items.append(ci)
                # Track tool calls
                itype = item.get("type", "")
                if itype == "command_execution":
                    cmd = item.get("command", "")
                    # Normalize: extract the base command name
                    name = _extract_command_name(cmd)
                    report.tool_calls[name] = report.tool_calls.get(name, 0) + 1
                elif itype == "mcp_tool_call":
                    server = item.get("server", "")
                    tool = item.get("tool", "")
                    name = f"{server}/{tool}" if server else tool
                    report.tool_calls[name] = report.tool_calls.get(name, 0) + 1

        elif event_type == "turn.completed":
            usage = obj.get("usage", {})
            total_in = usage.get("input_tokens", 0)
            cached = usage.get("cached_input_tokens", 0)
            # In sparse format, input_tokens is the grand total (fresh + cached).
            # Decompose into fresh vs cached to match the rich-format model.
            report.total_input_tokens = total_in - cached
            report.total_cache_read = cached
            report.total_output_tokens = usage.get("output_tokens", 0)
            report.total_cache_creation = 0  # not available in sparse format

    report.meta.num_turns = 1  # sparse format only has one turn.completed

    return report


# ---------------------------------------------------------------------------
# Rendering
# ---------------------------------------------------------------------------


def _fmt_tokens(n: int) -> str:
    """Format token count with K/M suffix."""
    if n >= 1_000_000:
        return f"{n / 1_000_000:.1f}M"
    if n >= 1_000:
        return f"{n / 1_000:.1f}K"
    return str(n)


def _est_tokens(chars: int) -> int:
    """Rough token estimate: ~4 chars per token."""
    return chars // 4


def render_header(report: SessionReport) -> str:
    lines = [
        "=" * 72,
        "SESSION HEADER",
        "=" * 72,
        f"  File:       {report.meta.file}",
        f"  Format:     {report.meta.format}",
        f"  Model:      {report.meta.model or 'unknown'}",
        f"  Session:    {report.meta.session_id or 'unknown'}",
    ]
    if report.meta.duration_ms:
        secs = report.meta.duration_ms / 1000
        lines.append(f"  Duration:   {secs:.1f}s")
    if report.meta.num_turns:
        lines.append(f"  Turns:      {report.meta.num_turns}")
    if report.meta.context_window:
        lines.append(f"  Ctx Window: {_fmt_tokens(report.meta.context_window)}")
    return "\n".join(lines) + "\n"


def render_token_summary(report: SessionReport) -> str:
    total_input = report.total_input_tokens + report.total_cache_creation + report.total_cache_read
    fresh = report.total_input_tokens
    cache_create = report.total_cache_creation
    cache_read = report.total_cache_read
    output = report.total_output_tokens

    cache_eligible = cache_create + cache_read
    hit_rate = (cache_read / cache_eligible * 100) if cache_eligible > 0 else 0.0
    # For sparse format, cache_creation is unavailable so hit rate is computed
    # as cached / total_input instead (a lower bound).
    if cache_create == 0 and cache_read > 0 and total_input > 0:
        hit_rate = cache_read / total_input * 100

    lines = [
        "",
        "-" * 72,
        "TOKEN SUMMARY",
        "-" * 72,
        (
            f"  Total Input:     {_fmt_tokens(total_input):>10s}"
            f"  (fresh: {_fmt_tokens(fresh)},"
            f" cache_create: {_fmt_tokens(cache_create)},"
            f" cache_read: {_fmt_tokens(cache_read)})"
        ),
        f"  Output:          {_fmt_tokens(output):>10s}",
        f"  Cache Hit Rate:  {hit_rate:>9.1f}%",
    ]
    return "\n".join(lines) + "\n"


def render_content_breakdown(report: SessionReport) -> str:
    # Group by type
    groups: dict[str, list[ContentItem]] = {}
    for item in report.items:
        groups.setdefault(item.item_type, []).append(item)

    total_chars = sum(it.chars for it in report.items)

    lines = [
        "",
        "-" * 72,
        "CONTENT BREAKDOWN",
        "-" * 72,
        f"  {'Type':<22s} {'Count':>6s} {'Chars':>10s} {'Est.Tok':>10s} {'Share':>7s}",
        f"  {'-' * 22} {'-' * 6} {'-' * 10} {'-' * 10} {'-' * 7}",
    ]

    for item_type in sorted(groups, key=lambda t: -sum(i.chars for i in groups[t])):
        items = groups[item_type]
        count = len(items)
        chars = sum(i.chars for i in items)
        est_tok = _est_tokens(chars)
        share = (chars / total_chars * 100) if total_chars > 0 else 0
        lines.append(f"  {item_type:<22s} {count:>6d} {chars:>10,d} {_fmt_tokens(est_tok):>10s} {share:>6.1f}%")

    lines.append(f"  {'-' * 22} {'-' * 6} {'-' * 10} {'-' * 10} {'-' * 7}")
    total_est = _fmt_tokens(_est_tokens(total_chars))
    lines.append(f"  {'TOTAL':<22s} {len(report.items):>6d} {total_chars:>10,d} {total_est:>10s} {'100.0':>6s}%")

    return "\n".join(lines) + "\n"


def render_top_items(report: SessionReport, n: int = 10) -> str:
    sorted_items = sorted(report.items, key=lambda i: -i.chars)[:n]

    lines = [
        "",
        "-" * 72,
        f"TOP {n} ITEMS BY SIZE",
        "-" * 72,
    ]

    for i, item in enumerate(sorted_items, 1):
        est_tok = _est_tokens(item.chars)
        lines.append(f"  {i:>2d}. [{item.item_type:<18s}] {item.chars:>8,d} chars (~{_fmt_tokens(est_tok)} tok)")
        preview = item.preview[:100]
        if preview:
            lines.append(f"      {preview}")

    return "\n".join(lines) + "\n"


def render_per_turn_growth(report: SessionReport) -> str:
    """Rich format only: show per-API-call token progression."""
    if not report.turns:
        return ""

    ctx_window = report.meta.context_window or 200_000  # default if unknown

    lines = [
        "",
        "-" * 72,
        "PER-TURN CONTEXT GROWTH",
        "-" * 72,
        (
            f"  {'#':>3s}  {'Input':>10s}  {'CacheCreate':>12s}"
            f"  {'CacheRead':>10s}  {'Output':>8s}"
            f"  {'TotalIn':>10s}  {'Fill%':>6s}"
        ),
        f"  {'-' * 3}  {'-' * 10}  {'-' * 12}  {'-' * 10}  {'-' * 8}  {'-' * 10}  {'-' * 6}",
    ]

    for i, turn in enumerate(report.turns, 1):
        total_in = turn.total_input
        fill_pct = total_in / ctx_window * 100
        lines.append(
            f"  {i:>3d}  "
            f"{_fmt_tokens(turn.input_tokens):>10s}  "
            f"{_fmt_tokens(turn.cache_creation_input_tokens):>12s}  "
            f"{_fmt_tokens(turn.cache_read_input_tokens):>10s}  "
            f"{_fmt_tokens(turn.output_tokens):>8s}  "
            f"{_fmt_tokens(total_in):>10s}  "
            f"{fill_pct:>5.1f}%"
        )

    return "\n".join(lines) + "\n"


def render_cost(report: SessionReport) -> str:
    """Rich format only: cost breakdown."""
    if report.total_cost_usd == 0:
        return ""

    lines = [
        "",
        "-" * 72,
        "COST",
        "-" * 72,
        f"  Total:            ${report.total_cost_usd:.4f}",
    ]
    if report.turns:
        avg = report.total_cost_usd / len(report.turns)
        lines.append(f"  Per-turn avg:     ${avg:.4f}")
    lines.append(f"  Model:            {report.meta.model}")

    return "\n".join(lines) + "\n"


def render_tool_calls(report: SessionReport) -> str:
    """Tool call frequency breakdown."""
    if not report.tool_calls:
        return ""

    lines = [
        "",
        "-" * 72,
        "TOOL USAGE",
        "-" * 72,
        f"  {'Tool':<40s} {'Calls':>6s}",
        f"  {'-' * 40} {'-' * 6}",
    ]

    for name, count in sorted(report.tool_calls.items(), key=lambda x: -x[1]):
        lines.append(f"  {name:<40s} {count:>6d}")

    lines.append(f"  {'-' * 40} {'-' * 6}")
    total = sum(report.tool_calls.values())
    lines.append(f"  {'TOTAL':<40s} {total:>6d}")
    return "\n".join(lines) + "\n"


def render_mcp_status(report: SessionReport) -> str:
    """MCP server connection status (rich format only)."""
    if not report.mcp_servers:
        return ""

    lines = [
        "",
        "-" * 72,
        "MCP SERVERS",
        "-" * 72,
    ]

    for srv in report.mcp_servers:
        name = srv["name"]
        status = srv["status"]
        icon = "+" if status == "connected" else "x"
        lines.append(f"  [{icon}] {name:<30s} {status}")

    return "\n".join(lines) + "\n"


def render_report(report: SessionReport) -> str:
    """Assemble all report sections."""
    sections = [
        render_header(report),
        render_token_summary(report),
        render_content_breakdown(report),
        render_top_items(report),
        render_tool_calls(report),
    ]

    if report.meta.format == "rich":
        sections.append(render_per_turn_growth(report))
        sections.append(render_cost(report))
        sections.append(render_mcp_status(report))
    else:
        sections.append("")
        sections.append("  Note: Per-turn data unavailable in sparse format (aggregate only).")
        sections.append("")

    return "\n".join(s for s in sections if s)


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


def analyze_file(filepath: str) -> SessionReport | None:
    path = Path(filepath)
    if not path.exists():
        print(f"WARNING: File not found: {filepath}", file=sys.stderr)
        return None

    lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
    if not lines:
        print(f"WARNING: Empty file: {filepath}", file=sys.stderr)
        return None

    fmt = detect_format(lines[0])
    if fmt == "unknown":
        print(f"WARNING: Unknown format in {filepath}, skipping", file=sys.stderr)
        return None

    if fmt == "rich":
        report = parse_rich(lines)
    else:
        report = parse_sparse(lines)

    report.meta.file = filepath
    return report


def main() -> None:
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} <logfile> [logfile ...]", file=sys.stderr)
        sys.exit(1)

    for filepath in sys.argv[1:]:
        report = analyze_file(filepath)
        if report:
            print(render_report(report))
            print()


if __name__ == "__main__":
    main()
