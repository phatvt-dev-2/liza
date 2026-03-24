# 29 - Agent Log Analysis Tools

## Context and Problem Statement

The multi-agent system had zero observability into what agents actually did during a sprint. Some agents consumed disproportionate tokens, but there was no way to understand why — was it a permission issue? An environment problem the agent struggled to work around? Wasted turns on irrelevant tools? Without logs and analysis tooling, post-mortem investigation was impossible and optimizing agent behavior was guesswork.

This also prepares the ground for a future Sprint Analyzer agent role that could automate post-sprint analysis.

## Considered Options

1. **No logging** — rely on blackboard state and commit history for observability.
2. **Add agent output logging and standalone analysis tools** — capture raw session data, build tooling to extract insights.
3. **Integrate analysis into the `liza` CLI** — Go-native analysis commands.

## Decision Outcome

Chose **Option 2**: logging enabled by default with standalone analysis tools as a rapid iteration path.

### Architecture

**Output capture** (enabled by default on `liza agent`, disable with `--no-log`):
- Stdout saved as `{agent-id}-{timestamp}.txt`, stderr as `.err` under `.liza/agent-outputs/`
- Separate stdout/stderr buffers — no concurrent-write race
- Logs saved before error handling, so crashes are captured too
- Automatically disabled in `-i` (interactive) mode

**Analysis tools:**
- **`skills/liza-logs/scripts/analyze-log.py`** — stdlib-only Python CLI for batch analysis. Auto-detects rich (Claude Code NDJSON) and sparse (Codex) log formats.
- **`skills/liza-logs/tools/liza-session-analyzer.html`** — drag-and-drop browser app with charts for visual exploration.

**Diagnostics surfaced:**
- Token usage and per-turn context utilization (including overflow detection)
- Turn timeline: per-action table correlating `tool_use` → `tool_result` by ID
- Tool result breakdown: aggregate sizes by tool (calls, total, avg, max)
- Efficiency insights: error counts, near-duplicate results, low-value chatter
- Struggle sequences: error clusters from one root cause with replay cost (e.g. "jscpd struggle: 14 actions, 8 errors, 13 turns, 628K wasted cache-read")
- System prompt cost multiplier: estimated per-turn replay overhead

### Rationale

Observability is prerequisite to optimization. The logging infrastructure is minimal (on by default, file writes before error handling) while the analysis tools extract actionable insights: which tools waste tokens, where agents struggle, how context budget is consumed. Standalone scripts (Python + HTML) enabled rapid experimentation — the Python analyzer could be integrated into the `liza` CLI later once the analysis patterns stabilize.

### Consequences

**Positive:**
- First observability into agent behavior during sprints
- Root cause analysis possible for token-heavy sessions
- Struggle sequence detection identifies systemic environment issues
- Foundation for future Sprint Analyzer agent role

**Limitations accepted:**
- Analysis tools are Python/HTML, outside the Go binary — different maintenance path
- Integration into `liza` CLI deferred — current tools are successful experiments awaiting refactoring

---
*Reconstructed from commits 81366ec..c59c7f6 (2026-02-27 to 2026-02-28)*
