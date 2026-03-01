# 26 - Role-Specific Prompt Templates

## Context and Problem Statement

The `shared_reference.tmpl` template dumped the full task state machine (12 states), all 25 blackboard fields, and all 15 anomaly types into every agent prompt regardless of role. Combined with role-specific tools listed in `base_prompt` for all roles and submit/verdict instructions repeated 3-4 times, each agent received ~350-400 lines of prompt where ~60% was irrelevant. With agents already loading CORE.md, MULTI_AGENT_MODE.md, AGENT_TOOLS.md, base prompt, and role-specific templates, context window pressure was significant — and most of the shared reference content was unused because agents interact with the blackboard through MCP tools, not raw YAML knowledge.

## Considered Options

1. **Keep shared template** — accept the context cost.
2. **Eliminate shared template** — distribute only relevant content to each role template.

## Decision Outcome

Chose **Option 2**: delete `shared_reference.tmpl` and give each role only what it needs.

### Architecture

- **Deleted** `shared_reference.tmpl`
- **`base_prompt.tmpl`**: universal only — identity, autonomy, query tools, exit codes, timestamps. No role-specific MCP tools.
- **Each role template** gains:
  - Its own state transitions (2-4 states vs 12)
  - Its own MCP tools
  - Its anomaly types with required fields
- Submit/verdict instructions consolidated from 3-4 repetitions to 1
- Verbose 18-line expected behavior examples replaced with compact 2-line failure-mode pattern recognition

**Per-role prompt size reduction:**

| Role | Before | After | Reduction |
|------|--------|-------|-----------|
| Coder | 348 lines | 143 lines | -59% |
| Reviewer | 399 lines | 168 lines | -58% |
| Planner | 277 lines | 115 lines | -58% |

### Rationale

The shared template existed for DRY but the content it shared was mostly irrelevant to each consumer. Agents don't need to know all 12 state transitions when they only trigger 2-4 of them. The minor duplication across role templates (e.g. a few shared field descriptions) is preferable to conditional rendering that would add maintenance complexity to an already deep template stack.

### Consequences

**Positive:**
- ~58% reduction in per-role prompt size, freeing context window for actual work
- Each role's template is self-contained and readable
- No irrelevant state machine knowledge cluttering agent context
- Simpler template stack (one fewer file)

**Limitations accepted:**
- Some content is lightly duplicated across role templates
- Adding a new cross-role concept requires updating each role template individually

---
*Reconstructed from commit 26e4c8b (2026-02-24)*
