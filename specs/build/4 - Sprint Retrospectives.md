# Sprint Retrospectives

## Context Handoff as Blackboard Event

Handoffs — whether the agent completed the task, got stuck, or ran out of context — write a structured event to the blackboard as a standard task field:

- What was attempted and what worked (positive)
- What was tried and failed, and why (negative)
- Current hypothesis and recommended next step (optional — may not apply to clean completions)
- Files that matter, files that are dead ends (optional)

This replaces the existing handoff fields (`summary`, `next_action`) with a richer schema. Fields are optional where noted so the same schema accommodates both context-exhaustion handoffs and clean task completions.

Events are append-only, immutable, part of the audit trail. A task accumulates an ordered array of handoff events — e.g., two context-exhaustion handoffs from agents A and B, then a completion event from agent C. It's mandatory on every task completion, not just context exhaustion.

The future Sprint Analyzer would get a new data source. Patterns emerge across sprints: "every task touching the payment module requires 2+ handoffs because the agent exhausts context on the dependency chain" → signal to the human at sprint review to restructure the module or hint the Planner to decompose differently. "Negative handoffs consistently mention the ORM as confusing" → lesson or spec improvement.

This feeds cross-sprint causal learning (#10): the negative handoffs are the richest signal about where the system struggles, which is exactly what the human needs to steer and the Planner needs to plan better.

**Implementation**: Define a handoff schema as a blackboard field, replacing the current `summary`/`next_action` fields. Make handoff writes mandatory in the submit/complete flow. Initially capture-only — no consumer reads these events at runtime. Future evolution: agents read prior handoff events when resuming a task (operational use), and the Sprint Analyzer processes them for cross-sprint pattern detection (analytical use).

**Derived from**: MAS²'s runtime state monitoring, but capturing epistemic state (what was learned) rather than just execution state (what happened).

## Sprint Analyzer Role as a Steering Interface

The sprint boundary is where the human validates deliverables (specs or code), corrects them (fix specs, raise defects), and tunes agent behavior for the next sprint (fix struggles, add lessons, adjust model/skill assignments). The Sprint Analyzer is the primary channel for all three.

It should be structured not as a report to read but as **actionable decisions to make**.

The Sprint Analyzer processes all agent logs, blackboard history, review iterations, handoff events, and cost data
at the end of a sprint. It produces:

- **Actionable decisions for the human**:
  - **Specs to revise**: specific gaps identified, ambiguities that caused implementation divergence
  - **Defects to raise**: with severity and suggested task specs for the next sprint
  - **Agent struggles to address**: with recommended lessons to capture, skills to hint, or model reassignments
  - **Cost anomalies**: tasks that burned disproportionate budget, review cycles that indicate spec problems
  - **Trend comparisons**: how this sprint's patterns compare to previous sprints
  - **Handoff patterns**: recurring context exhaustion or negative handoffs pointing to structural issues
- **Lessons for the system**: which decomposition strategies led to clean approvals, which spec structures caused fewer
  rejections, which skills were effective for which task types
- **Causal attribution**: not just "this sprint succeeded" but why — correlating outcomes with specific planner
  decisions, skill invocations, review iteration counts, model choices

The human reviews a dashboard of decisions, not a wall of logs. The quality of this interface directly determines how fast the human-agent feedback loop converges on effective sprints. The system stores lessons in a structured format the Planner reads at planning time.

**[Handoff events](#context-handoff-as-blackboard-event) as data source**: The Sprint Analyzer processes them alongside
log data, surfacing patterns like "every task touching module X requires 2+ handoffs" or "negative handoffs consistently
cite the ORM as confusing."

**Key insight from competitive analysis**: Ruflo's ReasoningBank does trigram/Jaccard similarity matching to find
relevant past patterns. MAS²'s CTO framework propagates credit through decision trees. Both are trying to learn from
history. Liza's approach is simpler: the Sprint Analyzer uses the lesson-capture skill to produce digested, structured
lessons — not indexed noisy conversations but actionable knowledge. The human reviews and approves lessons at the sprint
boundary, maintaining quality control over what feeds back into the system.

**Implementation**: A new agent role that runs at sprint end. Reads agent logs (`.liza/agent-outputs/`), blackboard
state, and previous lessons. Uses the lesson-capture skill to produce structured output. The existing
`skills/liza-logs/scripts/analyze-log.py` and `skills/liza-logs/tools/liza-session-analyzer.html` provide the raw analysis; the Sprint Analyzer synthesizes
this into decisions and lessons. Correlates data from blackboard (task states, review iterations, handoffs), log
analysis (token usage, tool patterns, anomalies), and cost instrumentation. Produces structured markdown with
decision categories.

**Derived from**: The realization that Liza's human-at-sprint-boundary design makes the Sprint Analyzer the central steering mechanism — not a nice-to-have observability feature.
