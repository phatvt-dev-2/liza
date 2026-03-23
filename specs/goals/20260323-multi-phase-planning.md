# Plan: Multi-Phase Planning with Phase-Gate Dependencies

## Context

The planner-reviewer feedback loop has no convergence mechanism for complex goals. A single code-planning task covering an entire complex goal produces plans too large for reliable review convergence (5 iterations, $11+ burned, zero approved plan).

**Intended outcome**: Four changes:
1. Orchestrator creates multiple smaller sequential planning tasks for complex goals (both tiers)
2. Child tasks from later phases auto-depend on child tasks from earlier phases (phase-gate)
3. Planners BLOCKED immediately when their plan can't reconcile with prior phases
4. Replan preserves and retargets dependency chains

---

## Change 1: Multi-phase orchestrator wake prompt

**File**: `internal/prompts/templates/wake_initial_planning.tmpl`

Remove "Create exactly one task" / "Do not decompose" / "❌ Decompose into multiple downstream tasks." Add guidance: when goal is complex (>3 distinct functional areas, >~8 estimated output entries, cross-cutting concerns), create N planning tasks (up to ~5) with sequential `depends_on` chains. Same role-pair, each scoped to a natural boundary in the spec. Simple goals: unchanged (one task, default).

---

## Change 2: Phase-gate dependency propagation

**Files**: `internal/ops/proceed.go`

### 2a. Shared child ID derivation (single source of truth)

Extract ID computation from the inline `fmt.Sprintf` calls in `proceedInner` (line 144) and `recoverCrashedTransition` (line 182) into shared functions:

```go
// perSubtaskChildID returns the deterministic child task ID for a per-subtask transition.
func perSubtaskChildID(parentID, transitionName string, index int) string {
    return fmt.Sprintf("%s-%s-%d", parentID, transitionName, index)
}

// oneToOneChildID returns the deterministic child task ID for a one-to-one transition.
func oneToOneChildID(parentID, transitionName string) string {
    return fmt.Sprintf("%s-%s", parentID, transitionName)
}
```

Used by both child creation AND inherited dep computation — eliminates formula coupling.

### 2b. computeInheritedDeps

Derives phase-gate deps from the SAME transition on upstream dependency tasks. Uses the shared ID functions and verifies existence in state (defense against formula drift):

```go
func computeInheritedDeps(s *models.State, task *models.Task, transitionName string, resolver *pipeline.Resolver) ([]string, error) {
    var inherited []string
    for _, depID := range task.DependsOn {
        depTask := s.FindTask(depID)
        if depTask == nil || !depTask.TransitionsExecuted[transitionName] {
            continue
        }
        td, err := resolver.Transition(transitionName)
        if err != nil {
            continue
        }
        switch td.Cardinality {
        case "per-subtask":
            for i := 0; i < len(depTask.Output); i++ {
                childID := perSubtaskChildID(depID, transitionName, i)
                if s.FindTask(childID) == nil {
                    return nil, fmt.Errorf("upstream task %s has transition %q executed but child %s missing (needs crash recovery)", depID, transitionName, childID)
                }
                inherited = append(inherited, childID)
            }
        case "one-to-one":
            childID := oneToOneChildID(depID, transitionName)
            if s.FindTask(childID) == nil {
                return nil, fmt.Errorf("upstream task %s has transition %q executed but child %s missing (needs crash recovery)", depID, transitionName, childID)
            }
            inherited = append(inherited, childID)
        }
    }
    return inherited, nil
}
```

**Key properties**:
- Transition-specific: only the SAME transition name contributes
- Deterministic: uses shared ID formula, not parent_task scan
- Hard-fail on missing upstream children: returns error instead of silently weakening the barrier. If upstream transition is marked executed but a child is missing, that's crash inconsistency — must be recovered first. In `ExecuteAvailableTransitions` (topo-sorted), upstream recovery runs before downstream, so this error should never fire. In `Proceed` (human-initiated), it surfaces the need for recovery before downstream proceed.

### 2c. Restructured ExecuteAvailableTransitions

**Two problems solved**:
1. **Ordering**: If plan-2 is processed before plan-1 in the same pass, `computeInheritedDeps` finds no upstream children.
2. **Crash recovery gap**: `AvailableTransitions` filters out already-executed transitions (line 272), making crash recovery through EAT unreachable (confirmed in proceed_test.go:1431). A partially-executed upstream transition leaves missing children that block downstream inheritance.

**Fix**: Three-phase approach inside the Modify closure:

**Phase 1a — Collect available transitions** (existing behavior): Scan merged tasks, resolve AvailableTransitions, build `pendingTx{taskIdx, name, def}` list.

**Phase 1b — Collect incomplete transitions** (NEW: crash recovery): Scan merged tasks for `TransitionsExecuted[name] = true` where expected children are missing. Add these to the pending list. `proceedInner` will enter `recoverCrashedTransition` for these (since `TransitionsExecuted[name]` is already set).

```go
// isTransitionIncomplete checks if an executed transition has missing children.
func isTransitionIncomplete(s *models.State, task *models.Task, transName string, resolver *pipeline.Resolver) bool {
    td, err := resolver.Transition(transName)
    if err != nil { return false }
    switch td.Cardinality {
    case "per-subtask":
        for i := 0; i < len(task.Output); i++ {
            if s.FindTask(perSubtaskChildID(task.ID, transName, i)) == nil { return true }
        }
    case "one-to-one":
        if s.FindTask(oneToOneChildID(task.ID, transName)) == nil { return true }
    }
    return false
}
```

Skip entries where `transName == "replanned"` (special marker, not a real transition).

**Phase 2 — Topological sort** (Kahn's algorithm):
- **Stability**: Tie-breaker is original collection index (min-heap, not plain queue). Preserves deterministic behavior.
- **Cycle detection**: Cyclic tasks are NOT executed. Three-part response:

  1. **History event** (durable marker): Add `TaskEventTransitionCycleBlocked` history entry on each cyclic task. Idempotent — check for existing entry before adding (prevents accumulation on repeated EAT runs).

  2. **Predicate exclusion**: A NEW predicate `ops.IsPlanningCompleteEligible` excludes cycle-blocked tasks from PLANNING_COMPLETE detection. `IsUnconsumedPlanningOutput` is NOT changed (see Predicate Scope below).

  3. **Log error** with cycle task IDs. Non-cyclic tasks proceed normally.

  The tasks remain MERGED with unconsumed output — semantically correct (the output IS unconsumed, the transition genuinely couldn't fire). The history event records WHY. The predicate excludes them from auto-detection. The orchestrator can investigate cycle-blocked tasks and resolve (supersede/cancel) at SPRINT_COMPLETE.

  **New model constant**: `TaskEventTransitionCycleBlocked TaskEventName = "transition_cycle_blocked"` in `internal/models/history.go:42`.

  **Exact semantics of `transition_cycle_blocked`**:
  - Does NOT modify Status (task remains MERGED)
  - Does NOT modify TransitionsExecuted (no forgery)
  - Does NOT satisfy dependencies (downstream tasks are not unblocked)
  - ONLY suppresses auto-detection of "unconsumed planning output" for PLANNING_COMPLETE wake purposes
  - Idempotent per (taskID, transitionName, sorted cycle member IDs) — check existing history entries before adding
  - Cycle members stored in `TaskHistoryEntry.Extra["cycle_members"]` (sorted task ID list) for debugging

  **Cycle-Blocked Predicate Scope**: `IsUnconsumedPlanningOutput` is NOT changed — it retains its original semantics and is still used by carry-forward (`advance_sprint.go:228`), replan auto-detect (`replan.go:175`), and checkpoint triggering (`sprint_checkpoint.go:46`). Cycle-blocked tasks remain visible to all these consumers.

  Instead, introduce a NEW exported predicate `ops.IsPlanningCompleteEligible(task, planningPairs) bool` = `IsUnconsumedPlanningOutput(task, planningPairs) && !ops.IsTransitionCycleBlocked(task)`. This is used ONLY by:
  - `countMergedPlanningTasksWithOutput` in `workdetection.go` (wake detection)
  - `collectMergedPlanningTasks` in `wake.go` (prompt rendering)

  `ops.IsTransitionCycleBlocked(task *models.Task) bool` is exported (capital I) — checks for `TaskEventTransitionCycleBlocked` in task history. Usable across packages (ops → prompts).

  **Visibility after exclusion**: Cycle-blocked planning tasks are surfaced in the orchestrator dashboard (sprint state section in `builder.go`) as a separate count: `"Cycle-blocked planning: N"`. This prevents them from becoming operationally invisible after exclusion from auto-detection. The orchestrator handles them via supersede/cancel/replan at SPRINT_COMPLETE.

  **EAT collection contract**:
  - Pending transitions deduped by (taskID, transitionName) before topo sort — same transition may be discovered by both "available" and "incomplete" scans
  - `isTransitionIncomplete` only evaluates real resolver-backed transitions: synthetic markers like `"replanned"` are excluded by checking `resolver.Transition(name)` returns success

**Phase 3 — Execute in sorted order**: Upstream recovery/transitions fire first. By the time downstream runs, all upstream children exist. `computeInheritedDeps` can hard-fail on missing children because the topo sort + crash recovery in phase 1b guarantees completeness.

### 2d. Updated signatures

```go
func proceedInner(s *models.State, taskID, transitionName string, tDef transitionDef,
    inheritedDeps []string, now time.Time, result *ProceedResult) error

func buildChildTask(childID, parentID string, entry models.OutputEntry, targetStatus models.TaskStatus,
    targetRolePair string, siblingIDs, inheritedDeps []string, now time.Time) models.Task

func recoverCrashedTransition(s *models.State, task *models.Task, taskID, transitionName string,
    tDef transitionDef, inheritedDeps []string, now time.Time, result *ProceedResult) error
```

In `buildChildTask`: sibling deps from output entry resolved first, then `inherited` appended.

In `recoverCrashedTransition`: for existing children (created before crash), check if inherited deps are missing and patch. For missing children, create with inherited deps.

### 2e. Proceed (human-initiated) path

`Proceed` also calls `proceedInner`. Currently `resolveTransitionDef` (line 449) returns only `transitionDef` and loads the resolver internally. Refactor: `Proceed` calls `loadResolver` directly, then calls a new `resolveTransitionDefFrom(resolver, transitionName)` variant (avoids double-loading). The resolver is then available for `computeInheritedDeps`. Pass inherited deps to `proceedInner`.

---

## Change 3: Planner BLOCKED on phase inconsistency

### 3a. Richer sibling and dependency metadata

**File**: `internal/prompts/builder.go`

Add `PlanRef` to `SiblingTaskSummary`:
```go
type SiblingTaskSummary struct {
    ID, Description, Status, PlanRef string
}
```

**File**: `internal/agent/prompt.go` (`collectSiblingTasks`)

Populate `PlanRef` from `task.PlanRef`.

**File**: `internal/prompts/role_context.go`

Add `DependsOn []string` to `RoleContextData`.

**File**: `internal/agent/prompt.go` (`buildTaskRoleContextData`)

Populate `DependsOn` from `task.DependsOn`.

### 3b. Phase-consistency blocking rule

**File**: `internal/prompts/templates/blocks/collective_plan_scoping.tmpl`

When task has DependsOn matching sibling IDs, render phase-consistency rule with prior-phase plan artifact paths. Go template logic:

```
{{- $hasPhaseDeps := false -}}
{{- range .DependsOn}}{{$depID := .}}{{range $.SiblingTasks}}{{if eq .ID $depID}}{{$hasPhaseDeps = true}}{{end}}{{end}}{{end}}
{{- if $hasPhaseDeps}}
PHASE CONSISTENCY RULE:
This task depends on prior planning phase(s):
{{range .DependsOn}}{{$depID := .}}{{range $.SiblingTasks}}{{if eq .ID $depID}}- {{.ID}} [{{.Status}}]{{if ne .PlanRef ""}} — plan: {{.PlanRef}}{{end}}
{{end}}{{end}}{{end}}
If your plan CANNOT be made consistent with those prior plans:
- Do NOT iterate with the reviewer — mark BLOCKED immediately via liza_mark_blocked.
- blocked_reason: which prior phase constraint conflicts and why.
- blocked_questions: what needs to change in prior plans to resolve.
{{end}}
```

### 3c. Epic-planner/reviewer role branches + pipeline config

**File**: `internal/prompts/templates/blocks/collective_plan_scoping.tmpl`

Add role-specific branches:
- `epic-planner`: "Do NOT plan capabilities that belong to a sibling phase's scope"
- `epic-plan-reviewer`: "Verify the epic stays within scope — flag scope creep into sibling phase territory"

**Files**: `.liza/pipeline.yaml` + `internal/embedded/pipeline.yaml`

Add `collective-plan-scoping` to BOTH `epic-planner` AND `epic-plan-reviewer` context-sections.

---

## Change 4: Replan dependency chain preservation

**File**: `internal/ops/replan.go`

### 4a. Preserve DependsOn on new task (line ~112)

Add `DependsOn: slices.Clone(task.DependsOn)` to the new task struct. Clone is required — direct assignment aliases the backing array, so in-place mutations (retargeting in 4b) would corrupt the old task's audit trail.

### 4b. Retarget downstream dependencies

After creating the new task, retarget non-terminal downstream tasks. Dedupe DependsOn after retargeting to handle edge cases (repeated replans, partially-retargeted state):

```go
for i := range state.Tasks {
    if state.Tasks[i].Status.IsTerminal() {
        continue
    }
    changed := false
    for j := range state.Tasks[i].DependsOn {
        if state.Tasks[i].DependsOn[j] == task.ID {
            state.Tasks[i].DependsOn[j] = newTaskID
            changed = true
        }
    }
    if changed {
        state.Tasks[i].DependsOn = dedupeStrings(state.Tasks[i].DependsOn) // order-preserving dedupe
    }
}
```

### 4c. Warn about terminal downstream tasks

If any MERGED/terminal task depends on the replanned task, add warning:
```
"task X is MERGED and depends on replanned task Y — consider replanning X too"
```

---

## Spec updates

| Spec | Change |
|------|--------|
| `specs/build/2 - Sub-pipelines and spec writing.md` (lines 122-143) | Document phase-gate inheritance in child creation, topo-sorted execution, patching in recovery |
| `specs/architecture/state-machines.md` | Document phase-gate dependency propagation semantics |
| `specs/protocols/task-lifecycle.md` | Document multi-phase planning (orchestrator creates sequential tasks) |
| `specs/protocols/sprint-governance.md` | Note replan with multi-phase requires explicit task ID; downstream retargeting behavior |
| `specs/architecture/blackboard-schema.md` | Document auto-inherited DependsOn on child tasks; document `transition_cycle_blocked` history event, its `cycle_members` payload, and semantics (suppresses PLANNING_COMPLETE detection without consuming the transition) |

---

## Critical files

| File | Change |
|------|--------|
| `internal/prompts/templates/wake_initial_planning.tmpl` | Multi-phase planning guidance |
| `internal/ops/proceed.go` | `perSubtaskChildID`/`oneToOneChildID`, `computeInheritedDeps`, `isTransitionIncomplete`, `topoSortPending`, restructured `ExecuteAvailableTransitions` (3-phase: collect available + incomplete, topo sort, execute; cycle → history event + predicate exclusion), updated `proceedInner`/`buildChildTask`/`recoverCrashedTransition`, new `resolveTransitionDefFrom` |
| `internal/models/history.go` | New `TaskEventTransitionCycleBlocked` constant |
| `internal/models/history_test.go` | Test for new history event constant |
| `internal/ops/advance_sprint.go` | New `IsPlanningCompleteEligible` + exported `IsTransitionCycleBlocked`; `IsUnconsumedPlanningOutput` unchanged |
| `internal/agent/workdetection.go` | `countMergedPlanningTasksWithOutput`: use `IsPlanningCompleteEligible` instead of `IsUnconsumedPlanningOutput` |
| `internal/prompts/wake.go` | `collectMergedPlanningTasks`: use `IsPlanningCompleteEligible` instead of `IsUnconsumedPlanningOutput` |
| `internal/ops/replan.go` | Preserve DependsOn, retarget downstream, warn terminal |
| `internal/prompts/builder.go` | PlanRef in SiblingTaskSummary; cycle-blocked count in orchestrator dashboard |
| `internal/prompts/role_context.go` | DependsOn in RoleContextData |
| `internal/agent/prompt.go` | Populate DependsOn and PlanRef |
| `internal/prompts/templates/blocks/collective_plan_scoping.tmpl` | Phase-consistency rule, epic-planner/reviewer branches |
| `.liza/pipeline.yaml` | collective-plan-scoping for epic-planner + epic-plan-reviewer |
| `internal/embedded/pipeline.yaml` | Same |
| 5 spec files (see table above) | |

## Known trade-off

Full barrier = zero cross-phase parallelism. Trades parallelism for review convergence.

## Verification

1. **proceed_test.go — Phase-gate propagation**:
   - plan-1 MERGED with `TransitionsExecuted{"code-plan-to-coding": true}` + 3 output entries + 3 children in state
   - plan-2 MERGED depends_on plan-1 with 2 output entries
   - `proceedInner` for plan-2 → children have plan-1's 3 children in DependsOn
   - Edge: dep task hasn't executed same transition → no inherited deps
   - Edge: dep task executed DIFFERENT transition → no inherited deps
   - Both `epic-to-us` and `code-plan-to-coding` tested
   - Error case: upstream transition marked executed but child missing → error returned (not silent drop)

2. **proceed_test.go — Topological ordering**:
   - plan-1 and plan-2 both MERGED in same pass (plan-2 depends_on plan-1)
   - plan-2 appears BEFORE plan-1 in state.Tasks array
   - `ExecuteAvailableTransitions` → plan-1's children created first, plan-2's children have inherited deps
   - Stability: two unrelated tasks (no dep relationship) preserve original array order
   - Cycle detection: circular deps → cyclic tasks get `transition_cycle_blocked` history entry (idempotent), skipped from execution, non-cyclic proceed normally

3. **proceed_test.go — Crash recovery through EAT** (currently unreachable, now fixed):
   - Upstream task with TransitionsExecuted set + missing children
   - Downstream task with depends_on upstream
   - `ExecuteAvailableTransitions` → upstream crash recovery fires first (phase 1b + topo sort), then downstream inherits complete set of upstream children

4. **proceed_test.go — Crash recovery via direct Proceed**:
   - TransitionsExecuted set, some children exist without inherited deps, some missing
   - Recovery creates missing WITH inherited deps, patches existing children

5. **replan_test.go — DependsOn preserved + retargeted + deduped**:
   - Replan phase-1 → new task has DependsOn (cloned, not aliased)
   - Downstream DependsOn retargeted to new ID
   - Repeated replan → no duplicate DependsOn entries

6. **builder_test.go — Template rendering**:
   - collective-plan-scoping with DependsOn + matching sibling → phase-consistency rule renders
   - Without DependsOn → no rule
   - Epic-planner/epic-plan-reviewer branches → correct scope language

7. **Wake template test**: Multi-phase guidance present in rendered output

8. **advance_sprint_test.go / wake_test.go — Predicate/rendering alignment**:
   - `IsPlanningCompleteEligible` excludes cycle-blocked tasks
   - `IsUnconsumedPlanningOutput` still includes cycle-blocked tasks (carry-forward/replan unchanged)
   - `collectMergedPlanningTasks` excludes cycle-blocked tasks from PLANNING_COMPLETE payload
   - Mixed case: one normal merged planning task + one cycle-blocked → only normal task in payload
   - Sprint advance with cycle-blocked planning task → task carried forward (not dropped)

9. **proceed_test.go — History-event idempotency + dedupe**:
   - Repeated EAT runs with persistent cycle → single `transition_cycle_blocked` history entry (not duplicated)
   - One task/transition discovered by both "available" and "incomplete" scans → executed once

10. **Full**: `make test` + `liza validate`
