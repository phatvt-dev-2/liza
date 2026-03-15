# Declarative Role Definitions — Phase 1 Implementation Plan

Spec: `specs/build/3 - Declarative Role Definitions.md#phase-1-declarative-role-properties`

## Spec Requirements (Phase 1)

| # | Requirement | Task(s) |
|---|-------------|---------|
| R1 | Add `roles` section to pipeline YAML schema | CP1-1 |
| R2 | Load role definitions at pipeline init, derive classification and mappings | CP1-2 |
| R3 | Replace hardcoded constants in `internal/roles/` with YAML-driven maps | CP1-3 |
| R4 | Replace `NewRoleStrategy()` switch with type-based generic selection | CP1-4 |
| R5 | Migrate timeout resolution to use role YAML defaults | CP1-5 |
| R6 | Wire `allowed-operations` into MCP handler authorization | CP1-6 |
| R7 | Persist `provider` metadata from `--cli` into agent blackboard entry | CP1-7 |

## Current State Analysis

### Files affected by Phase 1

| File | Current Role | Phase 1 Change |
|------|-------------|----------------|
| `internal/pipeline/config.go` | Pipeline YAML types + validation | Add `RoleDef`, `TimeoutDef` types; add `Roles` to `Pipeline`; update validation to use `Roles` instead of `AgentRoles` |
| `internal/pipeline/resolver.go` | State resolution queries | Add role classification methods: `RoleType`, `IsDoerRole`, `IsReviewerRole`, `DoerRoleNames`, `ReviewerRoleNames`, `AllowedOperations`, `RoleTimeouts`, `RoleDisplayName`, `MaxInstances` |
| `internal/embedded/pipeline.yaml` | Default pipeline config | Replace `agent-roles` with full `roles` section for all 9 roles |
| `internal/roles/roles.go` | Hardcoded constants + classification | Remove classification functions (`DoerRoles`, `ReviewerRoles`, `IsDoerRole`, `IsReviewerRole`); keep string constants for Phase 4 |
| `internal/agent/strategy.go` | `NewRoleStrategy()` 9-way switch | Replace with 3-way switch on role type + context builder map |
| `internal/agent/strategy_doer.go` | `DefaultTimeout()` returns 2h; `WaitConfig()` reads coder config keys | Read timeouts from role YAML via resolver |
| `internal/agent/strategy_reviewer.go` | `DefaultTimeout()` returns 30m; `WaitConfig()` reads reviewer config keys | Read timeouts from role YAML via resolver |
| `internal/agent/strategy_orchestrator.go` | `DefaultTimeout()` returns 4h; `WaitConfig()` reads orchestrator config keys | Read timeouts from role YAML via resolver |
| `internal/agent/supervisor.go` | Calls `NewRoleStrategy(config.Role)` | Pass resolver to `NewRoleStrategy` |
| `internal/mcp/server.go` | MCP server struct | Cache pipeline resolver for allowed-ops checks |
| `internal/mcp/middleware.go` | `withRole` middleware | Extend to support allowed-operations check |
| `internal/mcp/server_registration.go` | Per-handler `roleChecker` closures | Replace with generic allowed-operations check |
| `internal/mcp/handlers_helpers.go` | `requireDoerRole`, `requireReviewerRole` | Remove (replaced by allowed-operations) |
| `internal/models/agent.go` | `Agent` struct | Add `Provider` field |
| `internal/agent/registration.go` | `registerAgent()` | Accept and persist `CLIName` as provider |
| `internal/prompts/wake.go` | `AgentRoles[rp.Doer]` display name lookup | Use `Resolver.RoleDisplayName()` |
| `internal/ops/proceed.go` | `AgentRoles[rp.Doer]` display name lookup | Use `Resolver.RoleDisplayName()` |

### Key architectural observations

1. **Pipeline config is loaded from `.liza/pipeline.yaml`** via `pipeline.LoadFrozen()`. A `Resolver` wraps it for queries. Both are already used throughout the codebase — adding role methods to `Resolver` is natural.

2. **The MCP server has `projectRoot`** and can load the pipeline config at startup. Caching a resolver on the `Server` struct is straightforward.

3. **The strategy pattern is clean**: 3 strategy types (`doerStrategy`, `reviewerStrategy`, `orchestratorStrategy`) already exist. The 9-way switch maps role names to (strategy type + context builder function). Replacing the switch means: look up role type from YAML (3-way), look up context builder from a map (same map, different lookup).

4. **Timeout resolution hierarchy** (spec): `state.yaml config > role YAML definition > role-type default > hardcoded fallback`. Currently the chain is `state.yaml config > hardcoded default`. Adding role YAML as an intermediate layer requires the strategy to access the resolver.

5. **`agent-roles` is used by 2 production callers** (`prompts/wake.go:156`, `ops/proceed.go:344`) for display name lookup, plus validation in `config.go`. All test fixtures embed `agent-roles` in inline YAML. Migration to `roles` affects ~30 test YAML snippets across 8 test files.

6. **Provider metadata**: `SupervisorConfig.CLIName` holds the CLI name ("claude", "codex", etc.) but it's never persisted to the blackboard. Adding a `Provider` field to `models.Agent` and passing `CLIName` through `registerAgent()` is a clean, isolated change.

## Task Decomposition

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

**done_when**: `TestRoleType` asserts "coder" → "doer", "code-reviewer" → "reviewer", "orchestrator" → "orchestrator", unknown → error. `TestDoerRoleNames` returns exactly {coder, code-planner, epic-planner, us-writer}. `TestReviewerRoleNames` returns exactly {code-reviewer, code-plan-reviewer, epic-plan-reviewer, us-reviewer}. `TestAllowedOperations("coder")` returns the 5 operations from the spec. `TestRoleTimeouts("coder")` returns execution=2h, poll-interval=30s, max-wait=30m.

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
- Keep `requireRole(agentID, expectedRole)` — it's identity verification, not operation authorization
- Keep `authorizeClaimRelease` — it validates claim-type alignment, which is orthogonal to operation authorization
- Remove/deprecate `roles.DoerRoles()`, `roles.ReviewerRoles()`, `roles.IsDoerRole()`, `roles.IsReviewerRole()` (all MCP callers migrated)
- Update remaining non-MCP callers (agent tests) to use resolver or inline the check

**Files**: `internal/mcp/server.go`, `internal/mcp/middleware.go`, `internal/mcp/server_registration.go`, `internal/mcp/handlers_helpers.go`, `internal/mcp/middleware_test.go`, `internal/mcp/handlers_mutation.go`, `internal/roles/roles.go`, `internal/agent/strategy_test.go`, `internal/integration/full_sprint_test.go`

**done_when**: `liza_submit_for_review` called by `coder-1` succeeds (coder's allowed-operations includes `submit-for-review`). `liza_submit_for_review` called by `code-reviewer-1` is rejected with "operation not allowed" error. `liza_add_tasks` called by `orchestrator-1` succeeds. `liza_add_tasks` called by `coder-1` is rejected. `requireDoerRole` and `requireReviewerRole` functions no longer exist. `roles.IsDoerRole` and `roles.IsReviewerRole` no longer exist in `roles.go`. All tests pass.

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

## Dependency Graph

```
CP1-7 (provider)   [independent]

CP1-1 (schema)
  └─► CP1-2 (resolver methods)
        ├─► CP1-3 (absorb agent-roles)
        ├─► CP1-4 (strategy switch)
        │     └─► CP1-5 (timeout resolution)
        └─► CP1-6 (allowed-operations)
```

## Execution Order (recommended)

Parallelizable groups:
1. **CP1-1** + **CP1-7** (independent foundations)
2. **CP1-2** (depends on CP1-1)
3. **CP1-3** + **CP1-4** + **CP1-6** (all depend on CP1-2, independent of each other)
4. **CP1-5** (depends on CP1-4)

## Out of Scope (Phase 2+)

- Composable prompt sections (`context-sections` YAML → template assembly) — Phase 2
- Review quorum (`review-policy`, `PARTIALLY_APPROVED` state) — Phase 3
- Dual name elimination (runtime/workflow unification) — Phase 4
- `mandatory-docs` and `skills` wiring into prompt assembly — Phase 2
- Custom template blocks — Open Question #1
