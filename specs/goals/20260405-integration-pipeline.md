# Integration Sub-Pipeline

## Context

Liza has two sub-pipelines today: epic-spec (epic-planner → us-writer) and coding (code-planner → coder). After the coding sub-pipeline completes all tasks for a goal, the branch contains individually-reviewed work that may have cross-task consistency issues — type alignment, serialization tags, error mapping gaps, API surface drift, misplaced code. These are integration issues invisible to task-scoped review because each reviewer only saw their task's diff.

Currently, the human reviews the entire branch interactively (reviewer agent finds issues, doer agent fixes, loop until clean). This is effective but manual. The issues found are overwhelmingly mechanical — they don't require human judgment.

## Design

### New Roles

| YAML Key | Type | Display Name | Description | Skills |
|----------|------|--------------|-------------|--------|
| `integration-analyst` | doer | Integration Analyst | Scans full branch diff, produces mechanical integration findings | code-review, clean-code |
| `integration-reviewer` | reviewer | Integration Reviewer | Validates and enriches findings with systemic analysis | systemic-thinking, software-architecture-review |

The integration analyst focuses on mechanical consistency: type alignment, serialization completeness, error mapping, API surface coherence, test/code agreement, code organization. Pattern-matchable, no judgment calls. Its `output[]` must contain fully-formed fix-task definitions (desc, done_when, scope, spec_ref) — not raw findings. The per-subtask transition consumes task definitions, so the analyst's job is to produce actionable tasks, not a findings list requiring conversion.

The integration reviewer validates that each proposed fix-task is well-formed, correctly scoped, and enriches the list with systemic concerns: interaction effects across fixes, architectural drift, emergent risks invisible at the individual finding level. This is the reviewer counterpart that makes the pair complete.

### New Role-Pair

```yaml
integration-pair:
  doer: integration-analyst
  reviewer: integration-reviewer
  states:
    initial: DRAFT_INTEGRATION_ANALYSIS
    executing: ANALYZING_INTEGRATION
    submitted: INTEGRATION_ANALYSIS_TO_REVIEW
    reviewing: REVIEWING_INTEGRATION_ANALYSIS
    approved: INTEGRATION_ANALYSIS_APPROVED
    rejected: INTEGRATION_ANALYSIS_REJECTED
    # Clean scan (no findings) requires a terminal completion path that
    # bypasses per-subtask. Not representable in today's role-pair schema —
    # see Implementation Cost item 4.
```

### New Sub-Pipeline

```yaml
integration-subpipeline:
  steps:
    - integration-pair
    - coding-pair          # reuse existing pair — coder fixes, code-reviewer validates
  transitions:
    - name: integration-to-fix
      from: integration-pair.approved
      to: coding-pair.initial
      trigger: auto        # no human gate — trust was already served by task-level review
      cardinality: per-subtask  # one fix task per finding in output[]
```

Self-contained: findings flow to fixes within the same sub-pipeline. The coding-pair is reused — same coder + code-reviewer roles, same submit/verdict mechanism. No cross-pipeline transitions. Fix tasks bypass code-planning — these are small, specific fixes where planning overhead is disproportionate. If a fix turns out to require non-trivial work (e.g., type alignment across many files, new error type + propagation), the fixer marks BLOCKED → orchestrator can promote it to a full coding sub-pipeline task with planning.

### Trigger and Lifecycle

The integration pipeline is **not** a standing pipeline step. It is orchestrator-driven:

1. All coding tasks for a goal reach `done` (CODE_APPROVED + merged)
2. Orchestrator spawns an integration-analyst task scoped to the full branch
3. Integration analyst scans branch diff from goal base commit (see Scan Boundary below), produces fix-task definitions as `output[]`
4. Integration reviewer validates/enriches task definitions via submit/verdict
5. **Two outcomes after approval:**
   - **Findings exist** (`output[]` non-empty): per-subtask transition creates fix tasks → coder + code-reviewer submit/verdict → supervisor merges each fix → all fix tasks MERGED → integration phase complete
   - **Clean scan** (`output[]` empty): integration-pair task completes via a dedicated terminal state (INTEGRATION_ANALYSIS_CLEAN) that bypasses per-subtask transition entirely. Current ops layer rejects empty output[] in both SetTaskOutput and per-subtask proceed — a clean scan needs its own completion path, not an edge case of the normal flow.

Fix tasks follow the standard lifecycle: CODE_APPROVED → supervisor merges to integration branch → MERGED. Same as all other coding tasks. If a fix introduces new issues, the fixer or fix-reviewer marks BLOCKED → orchestrator wakes → decides action. No special re-scan mechanism.

### Integration Analyst Context

The integration analyst needs different context than other doers:

- **Branch diff from goal base commit** (see Scan Boundary below) — not a single task worktree
- **All specs** referenced by completed tasks — to verify implementation matches intent across tasks
- **Worktree from integration branch HEAD** — provisioned normally by ClaimTask. The analyst doesn't commit to it but having a real worktree means ClaimTask, base_commit, and submit_for_review all work unchanged. Cheaper than implementing a worktree-less doer path. The review_commit equals the worktree HEAD (no delta — analyst produces findings, not code).
- **Completed task list** with `done_when` criteria — to understand what each task was supposed to do

This is a new context section (e.g. `branch-integration-context`). It should render the git diff command for the agent to run at analysis time, not store the diff output in the blackboard — branch diffs can be massive and would bloat state. The section provides the command (`git diff goal.base_commit..HEAD`) and the completed task list; the agent executes the diff in its worktree.

**Zero-diff submission.** The analyst submits without having committed. submit_for_review doesn't check that files changed — it validates commit SHA, branch, TDD gate, and rebases onto integration (no-op here). However, TDD enforcement will fire: `EffectiveType()` defaults empty type to `coding` (`internal/models/task.go:255`), and the TDD gate checks `doer + coding + base_commit` (`internal/ops/submit_review.go:122`). The analyst needs either a new task type (e.g. `integration`) or an explicit role-level TDD exemption. See Implementation Cost item 1.

### Scan Boundary

The diff base for integration analysis is the **goal base commit**: the integration branch HEAD at the time the orchestrator created the goal's first coding-subpipeline task. This is a concrete, recordable value — the orchestrator snapshots it when spawning the first code-planning task.

Stored as `goal.base_commit` in the blackboard. The analyst diffs `goal.base_commit..HEAD` within its worktree. This avoids hardcoding any branch name and gives a stable reference point that captures exactly the work done for this goal.

### Integration Reviewer Context

- Same branch diff as the analyst
- Prior rejection context for re-reviews (existing `prior-rejection` section)
- systemic-thinking skill for systemic analysis

### Entry Point

```yaml
entry-points:
  general-objective: epic-spec-subpipeline.epic-planning-pair
  detailed-spec: coding-subpipeline.code-planning-pair
  # No entry-point for integration — orchestrator spawns it after coding completes
```

Integration is not an entry point. It is triggered by the orchestrator as a post-coding phase.

## Three Sub-Pipelines

After this change, Liza has three sub-pipelines with uniform structure:

| Sub-Pipeline | Doer | Reviewer | Produces |
|---|---|---|---|
| Epic-Spec | epic-planner → us-writer | epic-plan-reviewer → us-reviewer | Specs (epics, user stories) |
| Coding | code-planner → coder | code-plan-reviewer → code-reviewer | Code (plans, implementations) |
| Integration | integration-analyst → coder | integration-reviewer → code-reviewer | Findings → fix tasks (self-contained) |

All use the same pair + submit/verdict pattern. The coding-pair is reused across coding and integration sub-pipelines.

## Implementation Cost

Despite reusing submit/verdict and coding-pair, the integration analyst introduces genuinely new patterns:

1. **TDD check bypass for analyst.** `EffectiveType()` defaults to `coding`, so TDD enforcement fires for the analyst (doer + coding + base_commit). Options: (a) add a new task type (e.g. `integration`) and thread it through validation/workflows, or (b) add a role-level TDD exemption in submit_for_review. Option (b) is smaller but ad-hoc; option (a) is cleaner but touches more code.
2. **Branch-wide context assembly.** A new context section (`branch-integration-context`) that renders the diff command and completed task list with done_when criteria and referenced specs. The diff itself is not stored — the agent runs the command in its worktree at analysis time.
3. **Orchestrator wake logic for post-coding trigger.** The orchestrator must detect "all coding tasks for this goal are done" and spawn an integration task. This is a new wake condition.
4. **⚠️ Clean-scan completion path (highest-risk item — address first).** Today's pipeline schema (RolePairStates) only supports: initial, executing, submitted, reviewing, approved, rejected, plus optional quorum states. The resolver's transition map is hardcoded to this lifecycle. Clean scan requires extending the schema with a new terminal state type (INTEGRATION_ANALYSIS_CLEAN) and updating the resolver to recognize it as terminal without attempting per-subtask child creation. This is most likely to have second-order effects on existing pipeline behavior — fail fast here. Alternative considered and rejected: reusing the approved state with empty output[] and relaxing SetTaskOutput + proceed constraints — this overloads "approved" to mean two different things (findings approved vs. no findings) and would require every per-subtask consumer to handle the empty case.
5. **Goal base commit tracking.** New `goal.base_commit` field snapshotted by orchestrator when first coding task is created. Used as diff base for integration analysis.

## Open Questions (v2 — out of scope for initial implementation)

1. **Informational findings**: Should the analyst be able to emit non-actionable observations (no fix needed) that get recorded but don't create tasks? If so, where — discoveries section? Separate output field?
2. **Incremental re-scan scope**: If re-scan is added (question 6), should subsequent scans cover the full goal diff or only files touched by fix tasks?
3. **Integration analyst skill set**: `code-review` + `clean-code` is a starting point. May need a dedicated integration-analysis skill as patterns emerge.
4. **Auto-trigger**: The `integration-to-fix` transition is marked `auto`. Should the first integration scan also auto-trigger when all coding tasks complete, or should it require a human checkpoint?
5. **software-architecture-review entrypoint**: The existing skill is oriented toward planning ("should we build it this way?"). The integration reviewer asks a different question: "did the sum of these changes drift architecturally?" May need a dedicated entrypoint or framing for branch-scoped architectural review.
6. **Re-scan after fixes**: Should the orchestrator spawn a new integration scan after all fix tasks complete? This would catch integration issues introduced by fixes but adds orchestrator complexity and a loop-like pattern absent from other sub-pipelines. May be unnecessary if fix-level code review is sufficient for mechanical fixes.
7. **Concurrent goals and scan boundary**: `goal.base_commit..HEAD` includes changes from other goals that merged between this goal's first coding task and the integration scan. This may be intentional (cross-goal interactions cause integration issues) or confusing (analyst produces fix-tasks for issues introduced by goal B's code). Needs a decision: scan this goal's changes only, or scan everything since base?
