# Declarative Role Definitions — Phase 1 Implementation Plan

Spec: `specs/build/3 - Declarative Role Definitions.md#phase-1-declarative-role-properties`

## Spec Requirements (Phase 1)

| # | Requirement | Task(s) |
|---|-------------|---------|
| R1 | Add `roles` section to pipeline YAML schema | CP1-1 |
| R2 | Load role definitions at pipeline init, derive classification and mappings | CP1-2 |
| R3 | Replace hardcoded constants in `internal/roles/` with YAML-driven maps | CP1-6 (classification functions), CP1-8 (runtime constants + remaining classification callers) |
| R4 | Replace `NewRoleStrategy()` switch with type-based generic selection | CP1-4 |
| R5 | Migrate timeout resolution to use role YAML defaults | CP1-5 |
| R6 | Wire `allowed-operations` into MCP handler authorization | CP1-6 |
| R7 | Persist `provider` metadata from `--cli` into agent blackboard entry | CP1-7 |
| — | Absorb `agent-roles` into `roles` (implicit in R1 "YAML Changes" table) | CP1-3 |
| — | Update architecture docs (spec Context: "must be updated before implementation") | CP1-9 |

## Current State Analysis

### Files affected by Phase 1

| File | Current Role | Phase 1 Change |
|------|-------------|----------------|
| `internal/pipeline/config.go` | Pipeline YAML types + validation | Add `RoleDef`, `TimeoutDef` types; add `Roles` to `Pipeline`; update validation to use `Roles` instead of `AgentRoles` |
| `internal/pipeline/resolver.go` | State resolution queries | Add role classification methods: `RoleType`, `IsDoerRole`, `IsReviewerRole`, `DoerRoleNames`, `ReviewerRoleNames`, `AllRoleNames`, `AllowedOperations`, `RoleTimeouts`, `RoleDisplayName`, `MaxInstances` |
| `internal/embedded/pipeline.yaml` | Default pipeline config | Replace `agent-roles` with full `roles` section for all 9 roles |
| `internal/roles/roles.go` | Hardcoded constants + classification | Phase 1: remove runtime constants (`Runtime*`), classification functions (`DoerRoles`, `ReviewerRoles`, `IsDoerRole`, `IsReviewerRole`), `AllRuntime()`, `IsValidRuntime()`. Keep: `Claim*` selectors, workflow constants, `ToWorkflow`/`ToRuntime`, `IsValidWorkflow`/`AllWorkflow` (Phase 4) |
| `internal/agent/strategy.go` | `NewRoleStrategy()` 9-way switch | Replace with 3-way switch on role type + context builder map |
| `internal/agent/strategy_doer.go` | `DefaultTimeout()` returns 2h; `WaitConfig()` reads coder config keys | Read timeouts from role YAML via resolver |
| `internal/agent/strategy_reviewer.go` | `DefaultTimeout()` returns 30m; `WaitConfig()` reads reviewer config keys | Read timeouts from role YAML via resolver |
| `internal/agent/strategy_orchestrator.go` | `DefaultTimeout()` returns 4h; `WaitConfig()` reads orchestrator config keys | Read timeouts from role YAML via resolver |
| `internal/agent/supervisor.go` | Calls `NewRoleStrategy(config.Role)` | Pass resolver to `NewRoleStrategy`; replace `roles.RuntimeCoder` ref with string literal |
| `internal/agent/registration.go` | `registerAgent()` with role checks | Add `cliName` (CP1-7) and `resolver` (CP1-8) params; replace `roles.Runtime*` with resolver methods |
| `internal/mcp/server.go` | MCP server struct | Cache pipeline resolver for allowed-ops checks |
| `internal/mcp/middleware.go` | `withRole` middleware | Extend to support allowed-operations check |
| `internal/mcp/server_registration.go` | Per-handler `roleChecker` closures | Replace with generic allowed-operations check |
| `internal/mcp/handlers_helpers.go` | `requireDoerRole`, `requireReviewerRole` | Remove role-type checkers (CP1-6); migrate `authorizeClaimRelease` to resolver (CP1-8) |
| `internal/models/agent.go` | `Agent` struct | Add `Provider` field |
| `internal/models/state.go` | `FindOrchestratorID()` | Replace `roles.RuntimeOrchestrator` with string literal |
| `internal/prompts/wake.go` | `AgentRoles[rp.Doer]` display name lookup | Use `Resolver.RoleDisplayName()` |
| `internal/ops/proceed.go` | `AgentRoles[rp.Doer]` display name lookup | Use `Resolver.RoleDisplayName()` |
| `internal/ops/recover_agent.go` | Role-specific recovery logic | Replace `roles.Runtime*` with `resolver.RoleType()` |
| `internal/ops/recover_task.go` | Role-specific recovery | Replace `roles.Runtime*` with string literals |
| `internal/ops/submit_review.go` | TDD enforcement gate | Replace `roles.RuntimeCoder` with string literal |
| `internal/ops/claim_reviewer_task.go` | Reviewer role inference | Replace `roles.RuntimeCodePlanReviewer` with string literal |
| `cmd/liza/cmd_agent.go` | CLI role validation | Replace `roles.AllRuntime()` with resolver-based validation |
| `internal/commands/inspect.go` | Agent ID pattern matching | Replace `roles.AllRuntime()` with resolver-based lookup |
| `specs/architecture/roles.md` | Role architecture docs | Update to reflect declarative roles; mark multiple-orchestrator as superseded |

### Key architectural observations

1. **Pipeline config is loaded from `.liza/pipeline.yaml`** via `pipeline.LoadFrozen()`. A `Resolver` wraps it for queries. Both are already used throughout the codebase — adding role methods to `Resolver` is natural.

2. **The MCP server has `projectRoot`** and can load the pipeline config at startup. Caching a resolver on the `Server` struct is straightforward.

3. **The strategy pattern is clean**: 3 strategy types (`doerStrategy`, `reviewerStrategy`, `orchestratorStrategy`) already exist. The 9-way switch maps role names to (strategy type + context builder function). Replacing the switch means: look up role type from YAML (3-way), look up context builder from a map (same map, different lookup).

4. **Timeout resolution hierarchy** (spec): `state.yaml config > role YAML definition > role-type default > hardcoded fallback`. Currently the chain is `state.yaml config > hardcoded default`. Adding role YAML as an intermediate layer requires the strategy to access the resolver.

5. **`agent-roles` is used by 2 production callers** (`prompts/wake.go:156`, `ops/proceed.go:344`) for display name lookup, plus validation in `config.go`. All test fixtures embed `agent-roles` in inline YAML. Migration to `roles` affects ~30 test YAML snippets across 8 test files.

6. **Provider metadata**: `SupervisorConfig.CLIName` holds the CLI name ("claude", "codex", etc.) but it's never persisted to the blackboard. Adding a `Provider` field to `models.Agent` and passing `CLIName` through `registerAgent()` is a clean, isolated change.

7. **Runtime constants have ~70 call sites** across 15 files. After CP1-4 (strategy) and CP1-6 (MCP authorization) migrate their callers, ~30 sites remain in registration.go, supervisor.go, ops/, models/, cmd/, and commands/. CP1-8 migrates these and removes the constants from `roles.go`.

8. **`releaseTaskClaim` and `authorizeClaimRelease`** use role-name switches for doer/reviewer classification. With the resolver, these become `resolver.RoleType(role)` 3-way switches — naturally extending to cover spec-phase roles (epic-planner, us-writer) which the current code silently ignores.

## Task Decomposition

### CP1-9: Update specs/architecture/roles.md for declarative roles

**Intent**: Architecture docs reflect the declarative role definitions spec. The spec's Context section requires this update before implementation.

**Approach**:
- Update Terminology section: replace references to `internal/roles/roles.go` constants with YAML `roles` section as the source of role definitions
- Update Implementation subsection: document that roles are declared in pipeline YAML with type, display-name, timeouts, allowed-operations, and other properties
- Update Multiple Agents Per Role table: Orchestrator row changes from "Yes" to "No (max-instances: 1, enforced at registration)" per the spec's constraint
- Add reference to `specs/build/3 - Declarative Role Definitions.md` as the governing spec
- Keep existing sections that remain accurate (Shared Capabilities, Shared Constraints, role-specific sections, Agent Identity Protocol)

**Files**: `specs/architecture/roles.md`

**done_when**: `specs/architecture/roles.md` Terminology section references YAML `roles` section instead of `roles.go` constants. Implementation subsection documents declarative YAML role definitions. Orchestrator Multiple Agents row says "No (max-instances: 1)". Document references the Declarative Role Definitions spec.

**Depends on**: none

---

### CP1-1: Add roles section to pipeline YAML schema

**Intent**: Pipeline YAML supports a `roles` section with declarative role definitions including type, display-name, timeouts, allowed-operations, and other properties.

**Approach**:
- Add `RoleDef` struct with fields: `Type`, `DisplayName`, `Description`, `Timeouts`, `AllowedOperations`, `Skills`, `MandatoryDocs`, `MaxInstances`, `ContextSections`
- Add `TimeoutDef` struct with fields: `Execution`, `PollInterval`, `MaxWait` (all `string` for duration parsing)
- Add `Roles map[string]RoleDef` to `Pipeline` struct (alongside existing `AgentRoles` — removal is CP1-3)
- Add validation: `type` must be `doer|reviewer|orchestrator`; `max-instances` only meaningful for orchestrator; if `Roles` is present, role-pair doer/reviewer must reference valid role keys in `Roles`
- Update embedded `pipeline.yaml` with all 9 roles fully defined per the spec schema
- Add new tests for roles parsing and validation; keep existing `agent-roles` tests passing

**Files**: `internal/pipeline/config.go`, `internal/pipeline/config_test.go`, `internal/embedded/pipeline.yaml`

**done_when**: `TestLoad` parses a pipeline YAML with `roles` section into `PipelineConfig.Pipeline.Roles` map with correct field values. `TestValidate` rejects: missing `type` field, invalid type value, role-pair referencing undefined role. Embedded `pipeline.yaml` loads successfully with all 9 roles. Existing tests pass unchanged.

**Depends on**: none

---

### CP1-2: Add role classification methods to Resolver

**Intent**: `pipeline.Resolver` provides methods to query role properties (type, classification, allowed operations, timeouts, display name) from the loaded YAML.

**Approach**:
- Add methods to `pipeline.Resolver`:
  - `RoleType(name string) (string, error)` — returns "doer", "reviewer", or "orchestrator"
  - `IsDoerRole(name string) bool`
  - `IsReviewerRole(name string) bool`
  - `DoerRoleNames() []string`
  - `ReviewerRoleNames() []string`
  - `AllRoleNames() []string`
  - `AllowedOperations(name string) ([]string, error)` — returns YAML allowed-operations list
  - `RoleTimeouts(name string) (*TimeoutDef, error)` — returns per-role timeout config
  - `RoleDisplayName(name string) (string, error)`
  - `MaxInstances(name string) (int, error)` — returns max-instances (0 = unlimited)
- All methods read from `config.Pipeline.Roles`

**Files**: `internal/pipeline/resolver.go`, `internal/pipeline/resolver_test.go`

**done_when**: `TestRoleType` asserts "coder" → "doer", "code-reviewer" → "reviewer", "orchestrator" → "orchestrator", unknown → error. `TestDoerRoleNames` returns exactly {coder, code-planner, epic-planner, us-writer}. `TestReviewerRoleNames` returns exactly {code-reviewer, code-plan-reviewer, epic-plan-reviewer, us-reviewer}. `TestAllRoleNames` returns all 9 role names. `TestAllowedOperations("coder")` returns the 5 operations from the spec. `TestRoleTimeouts("coder")` returns execution=2h, poll-interval=30s, max-wait=30m.

**Depends on**: CP1-1

---

### CP1-3: Absorb agent-roles into roles — remove AgentRoles field

**Intent**: Remove the deprecated `agent-roles` pipeline section. All callers use the `roles` section via Resolver methods.

**Approach**:
- Remove `AgentRoles map[string]string` from `Pipeline` struct
- Update `validate()` in `config.go`: role-pair doer/reviewer lookup uses `Roles` map instead of `AgentRoles`
- Update `prompts/wake.go:156`: replace `cfg.Pipeline.AgentRoles[rp.Doer]` with resolver-based display name lookup
- Update `ops/proceed.go:344`: replace `cfg.Pipeline.AgentRoles[rp.Doer]` with resolver-based display name lookup
- Update all test YAML fixtures across packages: replace `agent-roles:` with minimal `roles:` sections (each test role needs at minimum `type` and `display-name`)
- Remove `agent-roles` from embedded `pipeline.yaml` (already has full `roles` from CP1-1)
- Update KnownFields validation test that checks for `agent_roles` rejection

**Files**: `internal/pipeline/config.go`, `internal/pipeline/config_test.go`, `internal/pipeline/testdata/*.yaml`, `internal/embedded/pipeline.yaml`, `internal/prompts/wake.go`, `internal/ops/proceed.go`, `internal/ops/proceed_test.go`, `internal/ops/add_tasks_test.go`, `internal/agent/waitforwork_test.go`, `internal/prompts/builder_test.go`, `internal/commands/init_test.go`

**done_when**: `AgentRoles` field no longer exists in `Pipeline` struct. `go build ./...` succeeds. All tests pass. `grep -r "agent-roles" internal/` returns only comments/docs, no YAML or Go struct references. `prompts/wake.go` and `ops/proceed.go` use `Resolver.RoleDisplayName()`.

**Depends on**: CP1-2

---

### CP1-4: Replace NewRoleStrategy() switch with type-based selection

**Intent**: Strategy creation is driven by role type (3-way: doer/reviewer/orchestrator) instead of role name (9-way switch), using the pipeline resolver.

**Approach**:
- Change `NewRoleStrategy(role string)` signature to `NewRoleStrategy(role string, resolver *pipeline.Resolver) (RoleStrategy, error)`
- Build a context builder map: `var contextBuilders = map[string]contextBuilderFunc{...}` mapping role names to their context builder functions (same 9 entries, just a map instead of switch arms)
- Look up role type via `resolver.RoleType(role)` — 3-way switch on type creates the appropriate strategy
- Look up context builder via the map — error if role has no registered builder
- Strategy types store the resolver reference (needed for CP1-5 timeout resolution)
- Update `supervisor.go`: pass resolver to `NewRoleStrategy`
- The `workflowRole` field remains on strategies (removed in Phase 4)
- Update all `NewRoleStrategy` call sites in tests to provide a resolver (tests can use a test pipeline config)

**Files**: `internal/agent/strategy.go`, `internal/agent/strategy_doer.go`, `internal/agent/strategy_reviewer.go`, `internal/agent/strategy_orchestrator.go`, `internal/agent/supervisor.go`, `internal/agent/prompt.go`, `internal/agent/strategy_test.go`, `internal/agent/prompt_test.go`, `internal/agent/waitforwork_test.go`

**done_when**: `NewRoleStrategy("coder", resolver)` returns `*doerStrategy`. `NewRoleStrategy("code-reviewer", resolver)` returns `*reviewerStrategy`. `NewRoleStrategy("orchestrator", resolver)` returns `*orchestratorStrategy`. A hypothetical new doer role defined in YAML would get `*doerStrategy` without modifying the switch. `TestNewRoleStrategy` passes for all 9 roles. `TestNewRoleStrategy_UnknownRole` still returns error.

**Depends on**: CP1-2

---

### CP1-5: Migrate timeout resolution to role YAML defaults

**Intent**: Strategy timeout methods (`DefaultTimeout`, `WaitConfig`) resolve from role YAML definitions, following the hierarchy: state.yaml config > role YAML > type default > hardcoded fallback.

**Approach**:
- Strategy types already store a resolver reference (from CP1-4)
- `DefaultTimeout()`: read `resolver.RoleTimeouts(role)` for execution timeout; fall back to type default (2h doer, 30m reviewer, 4h orchestrator) if YAML omits it
- `WaitConfig()`: read role YAML for poll-interval and max-wait defaults; state.yaml config still overrides; type defaults remain as final fallback
- Duration parsing: `TimeoutDef` fields are strings ("2h", "30s", "30m") — parse with `time.ParseDuration`
- The state.yaml config keys (`CoderPollInterval`, `ReviewerPollInterval`, etc.) remain functional as overrides — they are NOT removed in Phase 1

**Files**: `internal/agent/strategy_doer.go`, `internal/agent/strategy_reviewer.go`, `internal/agent/strategy_orchestrator.go`, `internal/agent/strategy_test.go`

**done_when**: `TestDefaultTimeout` passes: coder returns 2h (from YAML), orchestrator returns 4h (from YAML). A test with modified YAML timeout (e.g., coder execution=1h) causes `DefaultTimeout()` to return 1h. `TestWaitConfig` passes: state.yaml config overrides YAML values; YAML values override type defaults; type defaults apply when both are absent.

**Depends on**: CP1-4

---

### CP1-6: Wire allowed-operations into MCP handler authorization

**Intent**: MCP handlers enforce per-role allowed operations from the pipeline YAML `roles` section, replacing hardcoded per-handler role-type checks.

**Approach**:
- Add `resolver *pipeline.Resolver` field to `mcp.Server` struct; load at `NewServer` time via `pipeline.LoadFrozen`
- Add `isOperationAllowed(resolver, role, mcpToolName string) bool`:
  - Extract role from agent ID
  - Convert MCP tool name to operation name: strip `liza_` prefix, replace `_` with `-` (e.g., `liza_submit_for_review` → `submit-for-review`)
  - Look up `resolver.AllowedOperations(role)` and check membership
  - Read-only tools (`liza_get`, `liza_status`, `liza_validate`, `liza_version`) bypass — they have no `roleChecker` today and remain unchanged
- Create a new `RoleChecker` factory: `allowedOpsChecker(resolver, toolName) RoleChecker` that returns a `RoleChecker` calling `isOperationAllowed`
- Replace all per-handler `roleChecker` assignments in `registerMutationTools()` and `registerComplexOperations()` with `allowedOpsChecker(s.resolver, toolName)`
- Remove `requireDoerRole`, `requireDoerOrOrchestratorRole`, `requireReviewerRole` from `handlers_helpers.go` (no longer needed — allowed-operations subsumes them)
- Remove `DoerRoles()`, `ReviewerRoles()`, `IsDoerRole()`, `IsReviewerRole()` from `roles.go` (all callers migrated by this task)
- Keep `requireRole(agentID, expectedRole)` — it's identity verification, not operation authorization
- Keep `authorizeClaimRelease` — it validates claim-type alignment, which is orthogonal to operation authorization (migrated to resolver in CP1-8)
- Update remaining non-MCP callers (agent tests) to use resolver or inline the check

**Files**: `internal/mcp/server.go`, `internal/mcp/middleware.go`, `internal/mcp/server_registration.go`, `internal/mcp/handlers_helpers.go`, `internal/mcp/middleware_test.go`, `internal/mcp/handlers_mutation.go`, `internal/roles/roles.go`, `internal/agent/strategy_test.go`, `internal/integration/full_sprint_test.go`

**done_when**: `liza_submit_for_review` called by `coder-1` succeeds (coder's allowed-operations includes `submit-for-review`). `liza_submit_for_review` called by `code-reviewer-1` is rejected with "operation not allowed" error. `liza_add_tasks` called by `orchestrator-1` succeeds. `liza_add_tasks` called by `coder-1` is rejected. `requireDoerRole` and `requireReviewerRole` functions no longer exist. `roles.IsDoerRole` and `roles.IsReviewerRole` no longer exist in `roles.go`. `roles.DoerRoles` and `roles.ReviewerRoles` no longer exist in `roles.go`. All tests pass.

**Depends on**: CP1-2

---

### CP1-7: Persist provider metadata into agent blackboard entry

**Intent**: Agent registration persists the CLI provider name (from `--cli` flag) in the blackboard agent entry as the `provider` field.

**Approach**:
- Add `Provider string` field to `models.Agent` struct with `yaml:"provider,omitempty"`
- Add `cliName string` parameter to `registerAgent()` function
- Set `agent.Provider = cliName` during registration
- Update `supervisor.go`: pass `config.CLIName` to `registerAgent()`
- Update test callers of `registerAgent()` to pass an empty string or test value

**Files**: `internal/models/agent.go`, `internal/agent/registration.go`, `internal/agent/supervisor.go`, `internal/agent/registration_test.go`, `internal/agent/supervisor_test.go`

**done_when**: After `registerAgent(bb, root, "coder-1", "coder", "terminal-1", 1800, "claude")`, `state.Agents["coder-1"].Provider` equals `"claude"`. `TestRegisterAgent` passes with provider field assertions. `liza_get {"query": "agents/coder-1"}` output includes `provider: claude`.

**Depends on**: none (independent of other tasks)

---

### CP1-8: Remove hardcoded runtime role constants from internal/roles/

**Intent**: The `internal/roles/` package no longer defines hardcoded runtime role name constants or role enumeration functions. Role names, classification, and enumeration come from the pipeline YAML via the resolver. This completes R3.

**Approach**:
- Remove from `roles.go`: 9 `Runtime*` constants, `AllRuntime()`, `IsValidRuntime()`
- Convert `runtimeToWorkflow` map keys from `Runtime*` constants to string literals (map itself stays for Phase 4 along with `ToWorkflow`/`ToRuntime`, workflow constants, `IsValidWorkflow`/`AllWorkflow`)
- Keep `ClaimDoer`/`ClaimReviewer`/`ClaimBoth` (claim-type selectors, not role name constants)
- Migrate remaining callers (not already handled by CP1-4 and CP1-6):
  - `registration.go`: orchestrator singularity check → use `resolver.MaxInstances(role)` instead of `role == "orchestrator"` hardcoded check; reviewer stale claims → use `resolver.IsReviewerRole(role)`; `releaseTaskClaim` switch → use `resolver.RoleType(role)` (3-way: doer/reviewer/orchestrator)
  - `registration.go` signature: add `resolver *pipeline.Resolver` parameter to `registerAgent()`
  - `supervisor.go:107`: replace `roles.RuntimeCoder` with string literal `"coder"` or `resolver.IsDoerRole(role)` depending on semantic intent
  - `ops/recover_agent.go`: replace switch on `roles.RuntimeCoder`/`roles.RuntimeCodeReviewer` with `resolver.RoleType(role)` 3-way switch
  - `ops/recover_task.go`: replace `roles.RuntimeCoder`/`roles.RuntimeCodeReviewer` with string literals (identity check, not classification)
  - `ops/submit_review.go`: replace `roles.RuntimeCoder` with string literal (identity check for TDD enforcement)
  - `ops/claim_reviewer_task.go`: replace `roles.RuntimeCodePlanReviewer` with string literal
  - `models/state.go`: replace `roles.RuntimeOrchestrator` with string literal in `FindOrchestratorID()`
  - `cmd/liza/cmd_agent.go`: replace `roles.AllRuntime()` with resolver-based validation (load pipeline, call `resolver.AllRoleNames()`)
  - `commands/inspect.go`: replace `roles.AllRuntime()` with resolver-based lookup
  - `mcp/handlers_helpers.go`: migrate `authorizeClaimRelease` to use `resolver.RoleType()` instead of switch on specific `Runtime*` constants
  - Update tests for all modified files

**Files**: `internal/roles/roles.go`, `internal/agent/registration.go`, `internal/agent/registration_test.go`, `internal/agent/supervisor.go`, `internal/agent/supervisor_test.go`, `internal/ops/recover_agent.go`, `internal/ops/recover_agent_test.go`, `internal/ops/recover_task.go`, `internal/ops/recover_task_test.go`, `internal/ops/submit_review.go`, `internal/ops/submit_review_test.go`, `internal/ops/claim_reviewer_task.go`, `internal/models/state.go`, `cmd/liza/cmd_agent.go`, `internal/commands/inspect.go`, `internal/mcp/handlers_helpers.go`, `internal/integration/full_sprint_test.go`

**done_when**: `grep -rn "roles\.Runtime" internal/ cmd/` returns zero matches. `grep -n "func AllRuntime\|func IsValidRuntime" internal/roles/roles.go` returns zero matches. `internal/roles/roles.go` retains only: `Claim*` constants, `Workflow*` constants, `ToWorkflow`/`ToRuntime` functions with string-literal keys, `IsValidWorkflow`/`AllWorkflow` (all Phase 4 scope). `registerAgent` accepts a `resolver` parameter and uses `resolver.MaxInstances(role)` for the singularity check. `releaseTaskClaim` uses `resolver.RoleType(role)` for doer/reviewer classification. `go build ./...` succeeds. All tests pass.

**Depends on**: CP1-4, CP1-6, CP1-7

---

## Dependency Graph

```
CP1-9 (docs)       [independent, complete early]
CP1-7 (provider)   [independent]

CP1-1 (schema)
  └─► CP1-2 (resolver methods)
        ├─► CP1-3 (absorb agent-roles)
        ├─► CP1-4 (strategy switch)
        │     └─► CP1-5 (timeout resolution)
        └─► CP1-6 (allowed-operations)

CP1-8 (remove constants) ◄── CP1-4, CP1-6, CP1-7
```

## Execution Order (recommended)

Parallelizable groups:
1. **CP1-1** + **CP1-7** + **CP1-9** (independent foundations)
2. **CP1-2** (depends on CP1-1)
3. **CP1-3** + **CP1-4** + **CP1-6** (all depend on CP1-2, independent of each other)
4. **CP1-5** + **CP1-8** (CP1-5 depends on CP1-4; CP1-8 depends on CP1-4 + CP1-6 + CP1-7)

## Out of Scope (Phase 2+)

- Composable prompt sections (`context-sections` YAML → template assembly) — Phase 2
- Review quorum (`review-policy`, `PARTIALLY_APPROVED` state) — Phase 3
- Workflow constant elimination (`Workflow*` constants, `ToWorkflow()`/`ToRuntime()`, `IsValidWorkflow`/`AllWorkflow`, `runtimeToWorkflow`/`workflowToRuntime` maps, task model migration to single name form) — Phase 4
- `mandatory-docs` and `skills` wiring into prompt assembly — Phase 2
- Custom template blocks — Open Question #1
