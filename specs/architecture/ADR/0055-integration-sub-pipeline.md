# 55 - Integration Sub-Pipeline

## Context and Problem Statement

After the coding sub-pipeline completes all tasks for a goal, the integration branch contains individually-reviewed work that may have cross-task consistency issues — type alignment, serialization tags, error mapping gaps, API surface drift, misplaced code. These are integration issues invisible to task-scoped review because each reviewer only saw their task's diff.

The human was reviewing the entire branch interactively (reviewer agent finds issues, doer agent fixes, loop until clean). This was effective but manual. The issues found were overwhelmingly mechanical — they didn't require human judgment. Automating this was part of the initial vision.

## Considered Options

1. **A third sub-pipeline** with integration-analyst/integration-reviewer roles, auto-triggered after coding completion, producing fix tasks that reuse the existing coding-pair.

No alternatives were considered — this was part of the initial pipeline vision. The integration sub-pipeline fills the gap between task-scoped review and branch-wide consistency.

## Decision Outcome

Chose **Option 1**: a self-contained integration sub-pipeline that scans the full branch diff, produces actionable fix-task definitions, and feeds them back through the coding-pair.

### Architecture

**New roles:**

| YAML Key | Type | Description | Skills |
|----------|------|-------------|--------|
| `integration-analyst` | doer | Scans full branch diff, produces mechanical integration findings as `output[]` task definitions | code-review, clean-code |
| `integration-reviewer` | reviewer | Validates fix-task definitions, enriches with systemic concerns | systemic-thinking, software-architecture-review |

**Pipeline structure:**

```yaml
integration-subpipeline:
  steps:
    - integration-pair
    - coding-pair          # reuse — coder fixes, code-reviewer validates
  transitions:
    - name: integration-to-fix
      from: integration-pair.approved
      to: coding-pair.initial
      trigger: auto         # no human gate — trust already served by task-level review
      cardinality: per-subtask
```

**New pipeline mechanics:**

| Mechanism | Purpose |
|-----------|---------|
| Clean terminal state (`INTEGRATION_ANALYSIS_CLEAN`) | Completion path when `output[]` is empty — no findings, no fix tasks. Bypasses per-subtask transition. |
| Auto-transitions (`trigger: auto`) | `integration-to-fix` fires without human gate. New `AvailableAutoTransitions` resolver API alongside existing manual transitions. |
| `CodingComplete` wake trigger | Orchestrator detects all coding tasks for a goal reached done (merged). Fires once per goal. |
| `goal.BaseCommit` snapshot | Records integration branch HEAD when first coding task is created. Used as diff base: `git diff goal.base_commit..HEAD`. |
| `TaskTypeIntegration` | New task type — integration analyst submits without code changes, needs TDD exemption. |
| Clean task worktree cleanup | Reviewer `PreWork` cleans worktree for non-code-producing tasks. |
| Merge-equivalent finalization | Clean-terminal tasks skip the merge gate but still finalize through the supervisor. |

**Trigger and lifecycle:**

1. All coding tasks for a goal reach MERGED
2. `CodingComplete` wake trigger fires → orchestrator spawns integration-analyst task
3. Analyst scans `git diff goal.base_commit..HEAD`, produces fix-task definitions as `output[]`
4. Integration reviewer validates/enriches via submit/verdict
5. Two outcomes:
   - **Findings exist**: per-subtask auto-transition creates fix tasks → standard coder/code-reviewer lifecycle
   - **Clean scan**: task completes via `INTEGRATION_ANALYSIS_CLEAN` terminal state — no children created

**Integration analyst context:** New `branch-integration-context` section provides the diff command and completed task list with `done_when` criteria. The diff itself is not stored in the blackboard — the agent runs the command in its worktree at analysis time.

**Task type registry:** Replaced task-type if-chain with registry lookup (`1ee4ceb`), making task-type-specific behavior (TDD gate, worktree handling) extensible without modifying switch statements.

### Rationale

Three sub-pipelines now follow a uniform pair + submit/verdict pattern:

| Sub-Pipeline | Doer → Reviewer | Produces |
|---|---|---|
| Epic-Spec | epic-planner, us-writer | Specs |
| Coding | code-planner, coder | Code |
| Integration | integration-analyst, coder (reused) | Findings → fixes |

The coding-pair reuse is deliberate — fix tasks are small, specific changes where code-planning overhead is disproportionate. If a fix requires non-trivial work, the fixer marks BLOCKED and the orchestrator can promote it.

### Consequences

**Positive:**
- Automates mechanical integration review that was previously human-driven
- Self-contained — findings flow to fixes within the same sub-pipeline
- Reuses coding-pair rather than introducing new fix-specific roles
- Clean terminal state prevents empty-output edge cases in the per-subtask transition

**Limitations accepted:**
- No re-scan after fixes (v2 — if fix-level code review is sufficient, re-scan is unnecessary)
- Integration analyst context is command-based (agent runs diff), not pre-rendered (branch diffs can be massive)
- `goal.base_commit..HEAD` includes changes from concurrent goals (by design — cross-goal interactions cause integration issues)

**Extends:** ADR-0035 (Declarative Sub-Pipelines) — third sub-pipeline. ADR-0038 (Phase 2 Roles) — two new roles. ADR-0036 (Structured Task Output) — `output[]` drives fix-task creation.

---
*Reconstructed from commits 17c0ae7..633ea5b (2026-04-05)*
