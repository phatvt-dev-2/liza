# Circuit Breaker

## Rationale

Task-level alarms catch local failures. The circuit breaker catches **systemic failures** — patterns that indicate the problem is upstream of task execution.

The circuit breaker is an **observer, not a participant**. It reads signals, detects patterns, and escalates. It never proposes solutions or modifies artifacts.

---

## Identity and Constraints

```yaml
circuit_breaker:
  identity: observer
  permissions:
    read: [.liza/state.yaml (anomalies), .liza/log.yaml, sprint.metrics]
    write: [.liza/circuit_breaker_report.md, sprint.status → CHECKPOINT]
    execute: NOTHING
  prohibitions:
    - NEVER propose solutions
    - NEVER modify specs, code, or tasks
    - NEVER continue execution after triggering
```

---

## Input: Anomalies Section

The circuit breaker reads the **anomalies section** of the blackboard, populated by Code Reviewers and Coders:

```yaml
anomalies:
  - timestamp: 2025-01-18T14:32:00Z
    task: task-3
    reporter: code-reviewer-1
    type: retry_loop
    details:
      count: 3
      error_pattern: "serialization failure on nested entity"
      root_cause_hypothesis: "data model doesn't support nesting"

  - timestamp: 2025-01-18T15:10:00Z
    task: task-3
    reporter: coder-1
    type: trade_off
    details:
      what: "flatten entity instead of fixing serializer"
      why: "unblock task within iteration limit"
      debt_created: true

  - timestamp: 2025-01-18T16:45:00Z
    task: task-5
    reporter: coder-2
    type: assumption_violated
    details:
      assumption: "API supports pagination"
      reality: "API returns max 100, no cursor"
      spec_ref: "specs/requirements.md#FR-012"
```

---

## Anomaly Types

For the authoritative list of anomaly types, see [Blackboard Schema — Anomaly Types](../architecture/blackboard-schema.md#anomaly-types).

Summary of types relevant to circuit breaker patterns:

| Type | Logged By | Pattern Relevance |
|------|-----------|-------------------|
| `retry_loop` | Coder, Code Reviewer | retry_cluster |
| `trade_off` | Coder | debt_accumulation |
| `assumption_violated` | Coder, Code Reviewer | assumption_cascade |
| `spec_ambiguity` | Coder | spec_gap_cluster |
| `scope_deviation` | Code Reviewer | workaround_pattern |
| `workaround` | Code Reviewer | workaround_pattern |
| `debt_created` | Code Reviewer | debt_accumulation |
| `external_blocker` | Coder | external_service_outage (aggregated by `blocker_service`) |
| `hypothesis_exhaustion` | Planner | (triggers rescope, not CB) |
| `review_deadlock` | Planner | (logged for audit; triggers Planner intervention, not CB) |
| `spec_gap` | Planner | spec_gap_cluster |

---

## Pattern Detection Rules

```yaml
circuit_breaker_rules:
  window: current_sprint
  patterns:
    - name: retry_cluster
      description: "Same error type recurring across tasks"
      condition: count(type=retry_loop, similar(error_pattern)) >= 3
      severity: ARCHITECTURE_FLAW

    - name: debt_accumulation
      description: "Multiple trade-offs creating debt"
      condition: count(type=trade_off, debt_created=true) >= 3
      severity: SCOPE_FLAW

    - name: assumption_cascade
      description: "Same assumption failing across tasks"
      condition: count(type=assumption_violated, same(assumption)) >= 2
      severity: SPEC_FLAW

    - name: workaround_pattern
      description: "Multiple workarounds for similar issues"
      condition: count(type IN [workaround, trade_off], similar(root_cause)) >= 2
      severity: ARCHITECTURE_FLAW

    - name: spec_gap_cluster
      description: "Multiple tasks hitting spec ambiguity"
      condition: count(type=spec_ambiguity, same(spec_ref)) >= 2
      severity: SPEC_FLAW

    - name: external_service_outage
      description: "Same external service blocking multiple tasks"
      condition: count(type=external_blocker, same(blocker_service)) >= 2
      severity: EXTERNAL_DEPENDENCY
      action: CHECKPOINT  # Halt sprint — external issue, not agent problem
```

### Pattern Matching Functions

The pattern conditions use pseudo-functions for matching:

| Function | v1 Implementation | v2 (Future) |
|----------|-------------------|-------------|
| `similar(field)` | **Exact match only** — `liza analyze` uses `group_by(.)` | String similarity threshold (Levenshtein ≥ 0.7) |
| `same(field)` | **Exact match** — string equality comparison | Exact match after normalization |
| `count(...)` | Go implementation counts matching entries | Same |

**v1 Limitations:**
- ``liza analyze`` uses **exact string matching** for pattern detection
- Anomalies with `error_pattern: "timeout"` and `error_pattern: "connection timeout"` are counted separately
- Human must review script output and apply judgment for similar-but-not-identical patterns
- Thresholds may need adjustment: exact matching misses related errors, so lower counts may indicate real patterns

**v1 Workflow:**
1. Run ``liza analyze`` — outputs exact-match counts per pattern
2. Human reviews output and anomalies section
3. Human applies judgment: "these 2 exact + 1 similar = pattern"
4. Human triggers circuit breaker if pattern warrants

**v2 Implementation:** Requires defining similarity thresholds and normalization rules. Defer until v1 proves which patterns are valuable.

---

## Severity Classification

| Severity | Layer Affected | Remediation Scope |
|----------|---------------|-------------------|
| `VISION_FLAW` | Why we're building | Stop, revisit goal and brief |
| `SCOPE_FLAW` | What we're building (MVP) | Pause, revise PRD/requirements |
| `SPEC_FLAW` | Requirements detail | Pause, update specs |
| `ARCHITECTURE_FLAW` | How we're building | Pause, new ADR, possible refactor |
| `TECH_STACK_FLAW` | Tools/frameworks | Spike needed, possible tech pivot |
| `EXTERNAL_DEPENDENCY` | External services | Halt sprint — external issue, not agent problem; wait or escalate |

**Note:** `TECH_STACK_FLAW` is reserved for future patterns (e.g., library version conflicts). No current pattern triggers it.

---

## Circuit Breaker Activation

```
1. TRIGGER — Pattern rule matches, classify severity, log to blackboard
2. HALT — Set sprint.status to CHECKPOINT, agents stop, supervisors wait
3. GENERATE REPORT — Write .liza/circuit_breaker_report.md
4. WAIT FOR HUMAN — Human reviews, decides, documents, releases (`liza resume` or `liza stop`)
```

---

## Circuit Breaker Report Format

```markdown
# Circuit Breaker Report

**Triggered:** 2025-01-18T17:30:00Z
**Pattern:** retry_cluster
**Severity:** ARCHITECTURE_FLAW

## Trigger Evidence
| Task | Anomaly | Error Pattern |
|-------|---------|---------------|
| task-3 | retry_loop | serialization failure on nested entity |
| task-5 | retry_loop | serialization failure on nested entity |

**Common factor:** Nested entity serialization not handled

## Affected Artifacts
| Artifact | Issue |
|----------|-------|
| `specs/data-model.md` | Silent on nesting |
| `docs/architecture.md` | No serialization ADR |

## Recommended Actions
1. HALT current sprint (done)
2. CREATE ADR for nested entity serialization
3. UPDATE specs/data-model.md
4. REASSESS affected tasks
5. RESUME after artifacts updated

## Human Decision Required
- [ ] Acknowledge report
- [ ] Confirm severity assessment
- [ ] Assign ADR creation
- [ ] Release checkpoint with decision logged
```

---

## Implementation: v1 vs v2

**v1: Human-triggered analysis**
- Human runs ``liza analyze`` during checkpoint
- Script parses anomalies, applies rules, generates report
- No background daemon

**v2: Continuous monitoring**
- ``liza watch`` extended with pattern detection
- Auto-triggers checkpoint if pattern matches

**Recommendation:** Start with v1. Promote to v2 if manual analysis becomes bottleneck.

---

## Blackboard Circuit Breaker Section

```yaml
circuit_breaker:
  last_check: 2025-01-18T17:30:00Z
  status: TRIGGERED  # OK, TRIGGERED
  current_trigger:
    timestamp: 2025-01-18T17:30:00Z
    pattern: retry_cluster
    severity: ARCHITECTURE_FLAW
    report_file: .liza/circuit_breaker_report.md
  history:
    - timestamp: 2025-01-17T12:00:00Z
      pattern: null
      result: OK
    - timestamp: 2025-01-18T17:30:00Z
      pattern: retry_cluster
      severity: ARCHITECTURE_FLAW
      result: TRIGGERED
      resolution: "ADR-003 created, specs updated"  # Added by human after resolution
      resolved_at: 2025-01-18T19:00:00Z             # Added by human after resolution
```

### History Entry Fields

| Field | Set By | When | Notes |
|-------|--------|------|-------|
| `timestamp` | Script | On check | ISO 8601 timestamp of check |
| `pattern` | Script | On check | Pattern name (null if OK) |
| `severity` | Script | On TRIGGERED | Severity classification |
| `result` | Script | On check | `OK` or `TRIGGERED` |
| `resolution` | Human | After fix | Free text describing corrective action |
| `resolved_at` | Human | After fix | Timestamp of resolution |

### Resolution Workflow

When circuit breaker is triggered:

1. **Automatic:** ``liza analyze`` sets `status: TRIGGERED`, populates `current_trigger`
2. **Human review:** Human analyzes report, takes corrective action (ADR, spec update, etc.)
3. **Human resolves:** Edit `.liza/state.yaml` directly:
   ```yaml
   # 1. Copy current_trigger into history[] with resolution fields added:
   circuit_breaker:
     history:
       - ...existing entries...
       - timestamp: "2025-01-18T17:30:00Z"  # from current_trigger
         pattern: retry_cluster
         severity: ARCHITECTURE_FLAW
         result: TRIGGERED
         resolution: "ADR-003 created"       # added by human
         resolved_at: "2025-01-18T19:00:00Z" # added by human

   # 2. Clear current trigger and reset status:
     current_trigger: null
     status: OK
   ```
4. **Human resumes:** `liza resume`
5. **Agents resume**

**Resolution field:** Human-written summary of corrective action taken (free text, should reference ADRs/specs if applicable).

## Related Documents

- [Sprint Governance](sprint-governance.md) — checkpoints, retrospectives
- [Roles](../architecture/roles.md) — logging duties
- [Blackboard Schema](../architecture/blackboard-schema.md) — anomalies section
