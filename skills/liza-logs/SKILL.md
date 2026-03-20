---
name: liza-logs
description: Analyze Liza agents logs
---

SCOPE:
The logs in .liza/agent-outputs (nowhere else unless told otherwise explicitly),
The prompt may filter more specifically, e.g. a specific role and/or time range.

OBJECTIVE:
Finding recurring error patterns and proposing fixes.

PROTOCOL:
1. Start by running the analyzer:
```bash
python3 ~/.liza/skills/liza-logs/scripts/analyze-log.py .liza/agent-outputs/coder-*.txt        # all coder agents
python3 ~/.liza/skills/liza-logs/scripts/analyze-log.py .liza/agent-outputs/coder-1-*.txt # single agent
```
By default, run the analyzer per role.

Report sections: session header, token summary (fresh/cached/output, cache hit rate), content breakdown by type (chars, estimated tokens, share %), top 10 items by size, tool call frequency. Rich format adds per-turn context growth and cost breakdown.

2. Refine the analysis using the raw logs.

3. Propose fixes whenever possible.

NOTE:
The skill contains a web tool for humans to inspect logs: ~/.liza/skills/liza-logs/tools/liza-session-analyzer.html
