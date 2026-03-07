---
name: liza-logs
description: Analyze Liza agents logs
---

Analyze the logs in .liza/agent-outputs (nowhere else unless told otherwise explicitly), find recurring error patterns and propose fixes whenever possible.
The prompt may filter more specifically, e.g. a specific role and/or time range.

Start by running the analyzer:
```bash
python3 skills/liza-logs/scripts/analyze-log.py .liza/agent-outputs/*.txt        # all agents
python3 skills/liza-logs/scripts/analyze-log.py .liza/agent-outputs/coder-1-*.txt # single agent
```

Report sections: session header, token summary (fresh/cached/output, cache hit rate), content breakdown by type (chars, estimated tokens, share %), top 10 items by size, tool call frequency. Rich format adds per-turn context growth and cost breakdown.

Then refine the analysis using the raw logs.

The skill contains a web tool for humans to inspect logs: skills/liza-logs/tools/liza-session-analyzer.html
