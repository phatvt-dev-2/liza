# Implementation Plan: Sub-pipelines Phase 1, Steps 5-8

**Spec reference**: `specs/build/2 - Sub-pipelines and spec writing.md` (Phase 1, steps 5-8)
**Goal**: Make the pipeline declarative via YAML config.

---

## Overview

Steps 5-8 transform Liza's hardcoded pipeline into a config-driven system. The work decomposes into 4 coding tasks with a clear dependency chain:

```
Task 1: Pipeline config package (foundation)
   ↓
Task 2: liza init --config (step 5)
   ↓
Task 3: Pipeline-aware state machine + ops (steps 6-7)
   ↓
Task 4: Pipeline-aware proceed + downstream tasks (step 8)
```

### Design Decisions

**Pipeline config access pattern**: Ops functions call `pipeline.LoadFrozen(projectRoot)` which returns `*PipelineConfig` (or `nil` for legacy goals). This matches the existing stateless pattern where ops read `state.yaml` each time. The config is small and frozen, so repeated loading is acceptable. No global state, no signature changes for the nil case.

**Backward compatibility**: Legacy goals (no `.liza/pipeline.yaml`, `pipeline_version` absent or != 2) continue using the existing hardcoded state machine. The codebase already has hardcoded code-planning-pair states from steps 1-4 — these remain as the legacy path. Pipeline-configured goals use the new resolver exclusively.

**Transition structure**: The YAML declares state *names* only, not transitions. The intra-pair transition structure is fixed for all role-pairs (initial→executing→submitted→reviewing→approved|rejected, with rejected→initial loop). Cross-cutting meta-states (BLOCKED, ABANDONED, SUPERSEDED, INTEGRATION_FAILED) remain hardcoded. The resolver generates the full transition map from role-pair state names + fixed structure.

**`role_pair` supersedes `type`**: For pipeline-configured goals, `role_pair` is the authority for claimability and state resolution. The `type` field may remain for human-readable categorization but is no longer used mechanically. For legacy goals, `type` + the hardcoded switch statements continue to work.

---

## Task 1: Pipeline Config Package

**Spec step**: Foundation for steps 5-8

**Description**: Create `internal/pipeline/` package with Go types matching the spec's YAML structure, a parser/validator, and a state resolver.

### Files to create
- `internal/pipeline/config.go` — Types and YAML parsing
- `internal/pipeline/resolver.go` — State resolution and transition map generation
- `internal/pipeline/config_test.go` — Tests for parsing, validation, and resolver
- `internal/pipeline/testdata/valid-coding-subpipeline.yaml` — Phase 1 test fixture

### Types (from spec YAML)

```go
type PipelineConfig struct {
    Pipeline Pipeline `yaml:"pipeline"`
}

type Pipeline struct {
    AgentRoles  map[string]string       `yaml:"agent-roles"`
    RolePairs   map[string]RolePairDef  `yaml:"role-pairs"`
    SubPipelines map[string]SubPipeline `yaml:"sub-pipelines"`
    EntryPoints map[string]string       `yaml:"entry-points"`
}

type RolePairDef struct {
    Doer     string          `yaml:"doer"`
    Reviewer string          `yaml:"reviewer"`
    States   RolePairStates  `yaml:"states"`
}

type RolePairStates struct {
    Initial   string `yaml:"initial"`
    Executing string `yaml:"executing"`
    Submitted string `yaml:"submitted"`
    Reviewing string `yaml:"reviewing"`
    Approved  string `yaml:"approved"`
    Rejected  string `yaml:"rejected"`
}

type SubPipeline struct {
    Steps       []string         `yaml:"steps"`
    Transitions []TransitionDef  `yaml:"transitions"`
}

type TransitionDef struct {
    Name        string `yaml:"name"`
    From        string `yaml:"from"`   // e.g., "code-planning-pair.approved"
    To          string `yaml:"to"`     // e.g., "coding-pair.initial"
    Trigger     string `yaml:"trigger"`     // "manual" or "auto"
    Cardinality string `yaml:"cardinality"` // "per-subtask" or "one-to-one"
}
```

### Resolver API

```go
// LoadFrozen loads the frozen pipeline config from .liza/pipeline.yaml.
// Returns nil, nil when no pipeline config exists (legacy goal).
func LoadFrozen(projectRoot string) (*PipelineConfig, error)

// Load parses and validates a pipeline config from the given path.
func Load(path string) (*PipelineConfig, error)

// Resolver wraps a PipelineConfig for state resolution queries.
type Resolver struct { config *PipelineConfig }

func NewResolver(config *PipelineConfig) *Resolver

// State resolution by role-pair name + phase
func (r *Resolver) InitialStatus(rolePair string) (models.TaskStatus, error)
func (r *Resolver) ExecutingStatus(rolePair string) (models.TaskStatus, error)
func (r *Resolver) SubmittedStatus(rolePair string) (models.TaskStatus, error)
func (r *Resolver) ReviewingStatus(rolePair string) (models.TaskStatus, error)
func (r *Resolver) ApprovedStatus(rolePair string) (models.TaskStatus, error)
func (r *Resolver) RejectedStatus(rolePair string) (models.TaskStatus, error)

// Transition map: generates valid transitions for a role-pair's states.
// Used by the state machine to validate transitions dynamically.
func (r *Resolver) TransitionMap() map[models.TaskStatus][]models.TaskStatus

// All declared states across all role-pairs (for IsValid checks).
func (r *Resolver) AllDeclaredStates() []models.TaskStatus

// Sprint-terminal states: approved state of each role-pair,
// except coding-pair which uses MERGED.
func (r *Resolver) SprintTerminalStates() []models.TaskStatus

// Role-pair lookup: given a task's role_pair, return the RolePairDef.
func (r *Resolver) RolePair(name string) (*RolePairDef, error)

// Doer/reviewer role resolution
func (r *Resolver) DoerRole(rolePair string) (string, error)
func (r *Resolver) ReviewerRole(rolePair string) (string, error)

// Transition lookup: given a transition name, return its definition.
func (r *Resolver) Transition(name string) (*TransitionDef, error)

// Available transitions for a task based on its current status.
func (r *Resolver) AvailableTransitions(status models.TaskStatus, transitionsExecuted map[string]bool) []string

// Target role-pair for a transition (parsed from "to" field)
func (r *Resolver) TransitionTargetRolePair(transitionName string) (string, error)
```

### Validation rules
- All role-pair names must be globally unique
- All `doer`/`reviewer` must reference keys in `agent-roles`
- All 6 state names must be non-empty and unique across the entire config
- Sub-pipeline `steps` must reference existing role-pairs
- Transition `from`/`to` must reference valid `<role-pair>.<phase>` paths
- Transition `trigger` must be "manual" or "auto"
- Transition `cardinality` must be "per-subtask" or "one-to-one"
- Entry-point values must reference valid `<sub-pipeline>.<role-pair>` paths

### Dependencies
None (foundation package)

### Done when
- `pipeline.Load("testdata/valid-coding-subpipeline.yaml")` parses the Phase 1 config from the spec successfully
- Validation rejects: missing state fields, duplicate state names across role-pairs, invalid transition references, unknown cardinality values, unknown trigger values
- `resolver.InitialStatus("coding-pair")` returns `"DRAFT_CODE"`
- `resolver.ExecutingStatus("code-planning-pair")` returns `"CODE_PLANNING"`
- `resolver.TransitionMap()` generates correct transitions for all role-pairs (initial→executing, executing→submitted, etc.)
- `resolver.SprintTerminalStates()` returns `[CODING_PLAN_APPROVED, MERGED]` for the Phase 1 config (MERGED is hardcoded for coding-pair)
- `resolver.AvailableTransitions(CODING_PLAN_APPROVED, nil)` returns `["code-plan-to-coding"]`
- Tests pass: `go test ./internal/pipeline/...`

---

## Task 2: `liza init --config`

**Spec step**: Step 5

**Description**: Add `--config` and `--entry-point` flags to `liza init`. When `--config` is provided, validate the pipeline YAML, freeze it into `.liza/pipeline.yaml`, and record `pipeline_version: 2` and `goal.entry_point` in the blackboard.

### Files to modify
- `cmd/liza/main.go` — Add `--config` and `--entry-point` flags to `initCmd`
- `internal/commands/init.go` — Accept and handle config parameters
- `internal/models/state.go` — Add `PipelineVersion int` to `State`, `EntryPoint string` to `Goal`

### Files to create
- `internal/commands/init_test.go` — Test init with and without `--config`

### Changes detail

**`cmd/liza/main.go`**:
- Add flag: `initCmd.Flags().String("config", "", "path to pipeline config YAML")`
- Add flag: `initCmd.Flags().String("entry-point", "", "entry-point name (bypasses LLM classification)")`
- Pass values to `InitCommand`

**`internal/commands/init.go`**:
- Change signature: `InitCommand(description, specRef, configPath, entryPoint string, stdin io.Reader) error`
- When `configPath != ""`:
  - Validate with `pipeline.Load(configPath)`
  - Copy validated config to `.liza/pipeline.yaml`
  - Set `state.PipelineVersion = 2`
  - If `entryPoint != ""`: validate it exists in config's entry-points, set `state.Goal.EntryPoint = entryPoint`
  - If `entryPoint == ""`: leave `goal.entry_point` empty (Orchestrator will resolve it)
- When `configPath == ""`: existing behavior unchanged

**`internal/models/state.go`**:
- Add to `State`: `PipelineVersion int \`yaml:"pipeline_version,omitempty"\``
- Add to `Goal`: `EntryPoint string \`yaml:"entry_point,omitempty"\``

### Dependencies
- Task 1 (pipeline config package — `pipeline.Load()` must exist)

### Done when
- `liza init --config pipeline.yaml "Goal" --spec spec.md` creates `.liza/pipeline.yaml` identical to input
- `state.yaml` contains `pipeline_version: 2`
- `state.yaml` goal section contains `entry_point: <name>` when `--entry-point` is provided
- `liza init "Goal" --spec spec.md` (without `--config`) works as before, no `pipeline_version` or `pipeline.yaml`
- `liza init --config invalid.yaml "Goal" --spec spec.md` fails with validation error, no `.liza/` created
- `liza init --config valid.yaml --entry-point nonexistent "Goal" --spec spec.md` fails with entry-point error
- Tests pass: `go test ./internal/commands/...`

---

## Task 3: Pipeline-Aware State Machine and Ops

**Spec steps**: Step 6 + Step 7

**Description**: Replace hardcoded state resolution in the ops layer with pipeline config lookups. Make the state machine validation pipeline-aware. Make `role_pair` the primary mechanism for role dispatch in pipeline-configured goals.

### Files to modify
- `internal/models/state.go` — Pipeline-aware `IsValid`, `CanTransition`, `IsSprintTerminal`
- `internal/ops/claim_task.go` — Resolver-based state resolution
- `internal/ops/submit_review.go` — Resolver-based state resolution
- `internal/ops/submit_verdict.go` — Resolver-based state resolution
- `internal/ops/add_task.go` — Resolver-based initial status
- `internal/ops/resume_handoff.go` — Recognize new executing states
- `internal/statevalidate/validate.go` — Validate `role_pair` for pipeline goals

### Files to create
- `internal/ops/pipeline_ops.go` — Shared helper to load pipeline and build resolver for ops functions
- Test files for modified ops

### Changes detail

**`internal/ops/pipeline_ops.go`** (new shared helper):
```go
// loadResolver loads the frozen pipeline config for the given project root.
// Returns nil, nil for legacy goals (no pipeline.yaml).
func loadResolver(projectRoot string) (*pipeline.Resolver, error)
```

**`internal/models/state.go`**:

The existing `TaskStatus.IsValid()`, `CanTransition()`, `IsTerminal()`, `IsSprintTerminal()` are receiver methods on `TaskStatus` — they have no access to pipeline config. Two approaches:

**Approach A (recommended)**: Keep the existing methods for legacy goals. Add state-level methods that accept a resolver:
```go
// IsPipelineValid checks if the status is valid within the given pipeline config.
func (ts TaskStatus) IsPipelineValid(resolver *pipeline.Resolver) bool

// CanPipelineTransition checks if the transition is valid within the pipeline config.
func (ts TaskStatus) CanPipelineTransition(to TaskStatus, resolver *pipeline.Resolver) bool

// IsPipelineSprintTerminal checks sprint-terminal status against pipeline config.
func (ts TaskStatus) IsPipelineSprintTerminal(resolver *pipeline.Resolver) bool
```

Then ops functions choose which method to call based on whether a resolver exists.

**Approach B**: Make `Task.Transition()` resolver-aware:
```go
func (t *Task) Transition(to TaskStatus, resolver *pipeline.Resolver) error
```

Approach A is lower-risk (existing callers unchanged) and is recommended. Approach B is cleaner but requires updating every `task.Transition()` call site.

**NOTE**: The plan recommends Approach A. The implementer should verify there are no other significant callers of `IsValid()`/`CanTransition()` before deciding. A quick `grep` for `\.IsValid\(\)` and `\.CanTransition\(` will reveal the full call surface.

**`internal/ops/claim_task.go`**:

Current hardcoded switch (lines 84-111):
```go
switch task.Status {
case models.TaskStatusReady, models.TaskStatusRejected, models.TaskStatusIntegrationFailed:
    // coder path
case models.TaskStatusDraftCodingPlan, models.TaskStatusCodingPlanRejected:
    // code-planner path
}
```

Replace with:
```go
resolver, err := loadResolver(projectRoot)
if resolver != nil && task.RolePair != "" {
    // Pipeline path: resolve states from config
    initialStatus, _ := resolver.InitialStatus(task.RolePair)
    executingStatus, _ := resolver.ExecutingStatus(task.RolePair)
    rejectedStatus, _ := resolver.RejectedStatus(task.RolePair)

    switch task.Status {
    case initialStatus, rejectedStatus, models.TaskStatusIntegrationFailed:
        targetStatus = executingStatus
        // validate role matches doer role from config
    }
} else {
    // Legacy path: existing hardcoded switch
}
```

**`internal/ops/submit_review.go`**:

Current (lines 47-52):
```go
expectedCurrentStatus := models.TaskStatusImplementing
targetSubmittedStatus := models.TaskStatusReadyForReview
if runtimeRole == roles.RuntimeCodePlanner {
    expectedCurrentStatus = models.TaskStatusCodePlanning
    targetSubmittedStatus = models.TaskStatusCodingPlanToReview
}
```

Replace with resolver lookup when pipeline config exists:
```go
resolver, _ := loadResolver(projectRoot)
if resolver != nil && task.RolePair != "" {
    expectedCurrentStatus, _ = resolver.ExecutingStatus(task.RolePair)
    targetSubmittedStatus, _ = resolver.SubmittedStatus(task.RolePair)
} else {
    // Legacy path
}
```

**`internal/ops/submit_verdict.go`**:

Same pattern — replace lines 59-66 with resolver lookup.

**`internal/ops/add_task.go`**:

Replace `initialTaskStatus()` (lines 163-168):
```go
func initialTaskStatus(rolePair, projectRoot string) models.TaskStatus {
    resolver, _ := loadResolver(projectRoot) // or pass resolver as param
    if resolver != nil && rolePair != "" {
        status, err := resolver.InitialStatus(rolePair)
        if err == nil {
            return status
        }
    }
    // Legacy fallback
    if rolePair == "code-planning-pair" {
        return models.TaskStatusDraftCodingPlan
    }
    return models.TaskStatusReady
}
```

**`internal/ops/resume_handoff.go`**:

Line 66 checks for `TaskStatusImplementing` and `TaskStatusCodePlanning`. For pipeline goals, check if the task's current status matches any role-pair's `executing` state.

**`internal/statevalidate/validate.go`**:

Add validation rules for pipeline goals:
- If `state.PipelineVersion == 2` and `.liza/pipeline.yaml` exists:
  - Every task must have a non-empty `role_pair`
  - Every task's `role_pair` must exist in the pipeline config
  - Every task's `status` must be a valid state for its role-pair (or a cross-cutting state)

**`internal/models/state.go` — `AllPlannedTasksTerminal()`**:

Currently calls `task.Status.IsSprintTerminal()` which is hardcoded. For pipeline goals, this needs to check each task's status against its role-pair's approved state:

```go
func (s *State) AllPlannedTasksTerminalWithResolver(resolver *pipeline.Resolver) bool {
    for _, taskID := range s.Sprint.Scope.Planned {
        task := s.FindTask(taskID)
        if task == nil { return false }
        if resolver != nil && task.RolePair != "" {
            if !task.Status.IsPipelineSprintTerminal(resolver) { return false }
        } else {
            if !task.Status.IsSprintTerminal() { return false }
        }
    }
    return true
}
```

### Dependencies
- Task 1 (pipeline config package)
- Task 2 (init --config, for `.liza/pipeline.yaml` to exist in runtime)

### Done when
- `ClaimTask` for a pipeline-configured task with `role_pair: "coding-pair"` transitions to `IMPLEMENTING_CODE` (not `IMPLEMENTING`)
- `SubmitForReview` for `role_pair: "coding-pair"` transitions to `CODE_READY_FOR_REVIEW`
- `SubmitVerdict(APPROVED)` for `role_pair: "code-planning-pair"` transitions to `CODING_PLAN_APPROVED`
- `initialTaskStatus("coding-pair")` returns `DRAFT_CODE` for pipeline goals
- Legacy goals (no pipeline.yaml) continue using `DRAFT`, `IMPLEMENTING`, `READY_FOR_REVIEW`, etc.
- `IsSprintTerminal` for `CODING_PLAN_APPROVED` returns true for pipeline goals
- `AllPlannedTasksTerminal` works correctly for mixed legacy/pipeline sprints
- `liza validate` rejects pipeline-goal tasks missing `role_pair`
- `liza validate` rejects pipeline-goal tasks with `role_pair` not in pipeline config
- Tests pass: `go test ./internal/ops/... ./internal/models/... ./internal/statevalidate/...`

---

## Task 4: Pipeline-Aware Proceed and Downstream Task Creation

**Spec step**: Step 8

**Description**: Replace hardcoded `knownTransitions` in `proceed.go` with pipeline config lookups. Make `buildChildTask` set `role_pair` on child tasks. Update `AvailableTransitions` to read from config.

### Files to modify
- `internal/ops/proceed.go` — Read transitions from pipeline config
- `internal/commands/status.go` — Pass project root to `AvailableTransitions`

### Changes detail

**`internal/ops/proceed.go`**:

Replace `knownTransitions` lookup (lines 52-55) with pipeline config:

```go
func Proceed(projectRoot, taskID, transitionName string) (*ProceedResult, error) {
    resolver, err := loadResolver(projectRoot)

    var tDef transitionDef
    if resolver != nil {
        // Pipeline path: look up transition from config
        td, err := resolver.Transition(transitionName)
        if err != nil {
            return nil, fmt.Errorf("unknown transition %q: %w", transitionName, err)
        }
        fromPair, fromPhase := parseTransitionRef(td.From)  // e.g., "code-planning-pair", "approved"
        toPair, toPhase := parseTransitionRef(td.To)        // e.g., "coding-pair", "initial"

        fromStatus, _ := resolver.StatusByPhase(fromPair, fromPhase)
        toStatus, _ := resolver.StatusByPhase(toPair, toPhase)

        tDef = transitionDef{
            requiredStatus: fromStatus,
            targetStatus:   toStatus,
            cardinality:    td.Cardinality,
            targetRolePair: toPair,  // NEW FIELD
        }
    } else {
        // Legacy path: use knownTransitions
        td, ok := knownTransitions[transitionName]
        if !ok {
            return nil, fmt.Errorf("unknown transition %q", transitionName)
        }
        tDef = td
    }

    // ... rest of proceed logic unchanged
}
```

Add `targetRolePair` to `transitionDef`:
```go
type transitionDef struct {
    requiredStatus models.TaskStatus
    targetStatus   models.TaskStatus
    cardinality    string
    targetRolePair string  // NEW: set role_pair on child tasks
}
```

**`buildChildTask`** (line 171-185):

Add `role_pair` to child tasks:
```go
func buildChildTask(childID, parentID string, entry models.OutputEntry,
    targetStatus models.TaskStatus, targetRolePair string, now time.Time) models.Task {
    return models.Task{
        // ... existing fields ...
        RolePair:    targetRolePair,  // NEW
        // Type remains TaskTypeCoding for legacy compat
    }
}
```

**`AvailableTransitions`** (lines 206-214):

Current implementation iterates `knownTransitions`. For pipeline goals:
```go
func AvailableTransitions(task *models.Task, projectRoot string) []string {
    resolver, _ := loadResolver(projectRoot)
    if resolver != nil {
        return resolver.AvailableTransitions(task.Status, task.TransitionsExecuted)
    }
    // Legacy path
    var available []string
    for name, tDef := range knownTransitions {
        if task.Status == tDef.requiredStatus && !task.TransitionsExecuted[name] {
            available = append(available, name)
        }
    }
    return available
}
```

**Note**: This changes `AvailableTransitions` signature (adds `projectRoot`). Update all callers:
- `internal/commands/status.go:174` — pass `projectRoot`

**Error messages**: When listing available transitions, use pipeline config to enumerate valid options instead of hardcoded list.

### Dependencies
- Task 1 (pipeline config package)
- Task 3 (pipeline-aware ops, `loadResolver` helper, `transitionDef` changes)

### Done when
- `Proceed("code-plan-to-coding")` on a pipeline-configured task reads transition from `.liza/pipeline.yaml` and creates child tasks with `role_pair: "coding-pair"` and `status: "DRAFT_CODE"`
- `Proceed` on a legacy task still uses `knownTransitions` and creates children with `status: "DRAFT"` (no `role_pair`)
- `AvailableTransitions` for a pipeline task at `CODING_PLAN_APPROVED` returns `["code-plan-to-coding"]` from config
- `liza status` shows available transitions correctly for both legacy and pipeline goals
- Error message for unknown transition lists available transitions from pipeline config
- Child tasks created by Proceed have `parent_task` set to source task ID
- Tests pass: `go test ./internal/ops/... ./internal/commands/...`

---

## Dependency Graph

```
Task 1: Pipeline config package
  │
  ├──→ Task 2: liza init --config (also depends on model changes in Task 2 itself)
  │      │
  │      └──→ Task 3: Pipeline-aware state machine + ops
  │             │
  │             └──→ Task 4: Pipeline-aware proceed + downstream tasks
  │
  └──→ Task 3 (direct dependency on resolver)
        │
        └──→ Task 4 (direct dependency on loadResolver helper and transitionDef)
```

Strict ordering: **Task 1 → Task 2 → Task 3 → Task 4**

Tasks 2 and 3 could theoretically be parallelized (Task 2 modifies init, Task 3 modifies ops), but Task 3 needs the model changes from Task 2 (`PipelineVersion` field) to detect pipeline-configured goals, and Task 3's tests need the init flow from Task 2 to create `.liza/pipeline.yaml`. Sequential ordering is recommended.

---

## Risk Assessment

**RISK: Backward compatibility**. The biggest risk is breaking legacy goals. Mitigation: every ops function has an explicit `if resolver != nil && task.RolePair != ""` guard that falls back to the existing hardcoded path. Tests must cover both paths.

**RISK: State machine dual-path complexity**. Having two code paths (legacy + pipeline) increases maintenance burden. Mitigation: the legacy path is frozen (no new states will be added to it). Once all existing goals complete, the legacy path can be removed. The pipeline path is the only path for new goals.

**RISK: `IsValid()` scope**. `TaskStatus.IsValid()` is a receiver method with no config access. For pipeline goals, status strings like `"DRAFT_CODE"` would fail the hardcoded `IsValid()` check. Mitigation: `IsPipelineValid()` or modify `IsValid()` to accept a resolver parameter. The plan recommends Approach A (separate methods) to avoid changing all existing callers.

**RISK: `AvailableTransitions` signature change**. Adding `projectRoot` parameter changes the function signature, requiring caller updates. Mitigation: the function has only 2 callers (`status.go` and `proceed.go` internally). Small blast radius.

---

## Out of Scope

- `auto` trigger mode implementation (spec explicitly marks as "RESERVED — not yet implemented")
- Bootstrap prompt templates for new roles
- Orchestrator supervisor loop changes (entry-point dispatch, auto transition polling)
- Phase 2 role-pairs (Epic Planner, US Writer, etc.)
- `liza proceed --all` convenience command
- `liza undo-proceed` command
- Making `iteration_cap` configurable per role-pair
