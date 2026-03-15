# Implementation Plan: Declarative Roles Fixes

Spec: /home/tangi/Workspace/liza/todo-mas.md

---

## Phase 1 -- Hardcoded role names and resolver-based classification

### CP1: Replace hardcoded role lists in authorizeClaimRelease with resolver-based classification

**Intent:** authorizeClaimRelease currently enumerates role names by hand. A custom YAML-defined doer role hits the default branch and is rejected. Replace the switch with resolver.RoleType() classification.

**Changes:**
- `internal/mcp/handlers_helpers.go`: Change authorizeClaimRelease signature to accept a *pipeline.Resolver. Replace the role-name switch with resolver.RoleType(agentRole) calls: orchestrator allows all, doer allows only doer claims, reviewer allows only reviewer claims, unknown errors. When resolver is nil, fail closed (reject).
- `internal/mcp/handlers_mutation.go`: Update handleReleaseClaim to pass s.resolver to authorizeClaimRelease.
- `internal/mcp/handlers_helpers_test.go`: Update existing authorizeClaimRelease tests to pass a resolver, and add a test case for a custom doer role (e.g., data-engineer) that verifies it is accepted.

**Scope:** `internal/mcp/handlers_helpers.go`, `internal/mcp/handlers_mutation.go`, `internal/mcp/handlers_helpers_test.go`

**Done when:** authorizeClaimRelease no longer contains any hardcoded role name strings. A test with a custom YAML-defined doer role (e.g., data-engineer-1) passes the authorization check for doer claim release. A test with a custom reviewer role passes for reviewer claim release. Nil resolver rejects all.

**Spec ref:** todo-mas.md -- Phase 1, [concern] internal/mcp/handlers_helpers.go:142-155

---

### CP2: Replace hardcoded role=="coder" in recover_agent.go worktree cleanup with resolver-based doer check

**Intent:** RecoverAgent only removes worktrees when role == "coder". Custom doer roles with worktrees skip cleanup on crash. Replace the literal string check with resolver-based type classification.

**Changes:**
- `internal/ops/recover_agent.go`: At the worktree removal check (currently if role == "coder"), use the resolver (already loaded later in the function -- move the resolver load earlier) to check resolver.RoleType(role) == "doer" instead. Also add the nil-resolver warning log (from CP8) since the resolver load is being moved earlier.
- `internal/ops/recover_agent_test.go`: Add a test case verifying that a custom doer role worktree is cleaned up during recovery.

**Scope:** `internal/ops/recover_agent.go`, `internal/ops/recover_agent_test.go`

**Done when:** The string "coder" no longer appears in the worktree-removal condition in recover_agent.go. A test with a custom doer role (e.g., data-engineer) verifies worktree removal occurs during recovery.

**Spec ref:** todo-mas.md -- Phase 1, [concern] internal/ops/recover_agent.go:72

---

### CP3: Replace hardcoded runtimeRole=="coder" in TDD enforcement with resolver-based doer type check

**Intent:** submit_review.go TDD enforcement only applies to literal "coder" role. Custom doer roles skip TDD checks. Replace with resolver-based classification.

**Changes:**
- `internal/ops/submit_review.go`: The resolver is already loaded above the check. Replace runtimeRole == "coder" with a check against the resolver: roleType, _ := resolver.RoleType(runtimeRole); roleType == "doer". This ensures all doer roles (including custom ones) are subject to TDD enforcement for coding tasks.
- `internal/ops/submit_review_test.go`: Add a test verifying TDD enforcement triggers for a custom doer role submitting a coding task.

**Scope:** `internal/ops/submit_review.go`, `internal/ops/submit_review_test.go`

**Done when:** The string "coder" no longer appears in the TDD enforcement condition in submit_review.go. A test with a custom doer role verifies TDD enforcement applies.

**Spec ref:** todo-mas.md -- Phase 1, [concern] submit_review.go:123

---

### CP4: Replace hardcoded role=="code-plan-reviewer" in claim_reviewer_task.go workflow inference with resolver-based lookup

**Intent:** When workflowRole is empty, ClaimReviewerTask infers it from the agent ID by checking for "code-plan-reviewer". Custom reviewer roles fall through to the default "code-reviewer". Replace with resolver-based role-pair lookup.

**Changes:**
- `internal/ops/claim_reviewer_task.go`: When workflowRole is empty, use the resolver to determine the workflow role. The resolver is already loaded as pb.pr below -- move it up. Use the resolver to look up the reviewer role from its role-pair configuration. If resolver is unavailable, fall back to existing behavior.
- `internal/ops/claim_reviewer_task_test.go`: Add a test verifying that a custom reviewer role (e.g., security-reviewer) with a correctly configured role-pair can claim review tasks without explicitly passing workflowRole.

**Scope:** `internal/ops/claim_reviewer_task.go`, `internal/ops/claim_reviewer_task_test.go`

**Done when:** The literal string "code-plan-reviewer" no longer appears in the workflow role inference logic. A test with a custom reviewer role verifies correct workflow role inference.

**Spec ref:** todo-mas.md -- Phase 1, [concern] claim_reviewer_task.go:45

---

### CP5: Enforce orchestrator singularity by resolved type, not role key

**Intent:** registration.go counts live agents by exact role name match (agent.Role == role). Two different orchestrator role keys can register concurrently (one instance each), violating the spec type-based singularity requirement.

**Changes:**
- `internal/agent/registration.go`: In the singularity check within registerAgent, when the registering role resolves to type: orchestrator, count all live agents whose resolved type is "orchestrator" (not just agent.Role == role). Keep the existing per-role-key max-instances check for non-orchestrator roles.
- `internal/agent/registration_test.go`: Add a test that attempts to register two agents with different orchestrator role keys (e.g., orchestrator-1 and lead-orchestrator-1) and verifies the second registration is rejected.

**Scope:** `internal/agent/registration.go`, `internal/agent/registration_test.go`

**Done when:** Registering a second agent with a different role key but type: orchestrator is rejected. Existing per-role-key max-instances enforcement for non-orchestrator roles is unaffected.

**Spec ref:** todo-mas.md -- Phase 1, [blocker] registration.go:69

---

### CP6: Resolve orchestrator from state by type, not literal role name

**Intent:** FindOrchestratorID() in state.go matches agent.Role == "orchestrator" literally. A custom orchestrator role key (e.g., lead-orchestrator) is not found, breaking auto-resolution for liza_add_tasks / liza_supersede_task.

**Changes:**
- `internal/ops/resolve_orchestrator.go`: Update ResolveOrchestratorFromState to accept an optional *pipeline.Resolver. When provided, iterate agents checking resolver.RoleType(agent.Role) == "orchestrator". When nil, fall back to state.FindOrchestratorID() (existing literal match).
- `internal/mcp/handlers_mutation.go`: Update resolveOrchestratorID to pass s.resolver to ResolveOrchestratorFromState.
- `cmd/liza/main.go`: Update the call site to pass the resolver if available, or nil.
- `internal/ops/resolve_orchestrator_test.go`: Add a test with a custom orchestrator role key that verifies type-based resolution succeeds.

**Scope:** `internal/ops/resolve_orchestrator.go`, `internal/ops/resolve_orchestrator_test.go`, `internal/mcp/handlers_mutation.go`, `cmd/liza/main.go`

**Done when:** An agent registered with a custom role key whose resolved type is "orchestrator" is found by ResolveOrchestratorFromState. The literal string "orchestrator" is no longer the sole matching criterion. Existing tests still pass for the standard "orchestrator" role key.

**Spec ref:** todo-mas.md -- Phase 1, [blocker] state.go:57 and handlers_mutation.go:14

---

### CP7: Surface pipeline load error in nil-resolver error path

**Intent:** When pipeline config fails to load, the MCP server stores a nil resolver. All operationChecker-guarded tools fail with a generic message that does not include why. The spec suggests surfacing the original load error.

**Changes:**
- `internal/mcp/server.go`: Store the pipeline load error on the Server struct (e.g., resolverLoadErr error). In isOperationAllowed (or wherever the nil-resolver error is surfaced), include the stored error in the message.

**Scope:** `internal/mcp/server.go`

**Done when:** When the pipeline config fails to load, MCP tool error messages include the original load error text (not just "pipeline resolver not loaded"). Verified by a test that creates a server with an invalid pipeline config and checks the error message from an operation-checked tool.

**Spec ref:** todo-mas.md -- Phase 1, [suggestion] internal/mcp/server.go:26-33

---

### CP8: Add log line for nil-resolver roleType fallback in recover_agent.go

**Intent:** When the resolver is nil during recovery, roleType silently falls through to an empty string and no claim release happens. Add a log line for debuggability.

**Changes:**
- `internal/ops/recover_agent.go`: In the bb.Modify closure, when resolver is nil, add a slog.Warn indicating claim release was skipped due to missing resolver.

**Scope:** `internal/ops/recover_agent.go`

**Done when:** When resolver is nil during agent recovery, a warning log line is emitted indicating the claim release was skipped due to missing resolver.

**Spec ref:** todo-mas.md -- Phase 1, [suggestion] internal/ops/recover_agent.go:102-106

---

## Phase 2 -- Template naming, IntegrationFix propagation, test-YAML parity

### CP9: Rename template defines from underscores to hyphens to match YAML section names

**Intent:** Templates define themselves as mandatory_docs and skills_affinity, but the YAML references mandatory-docs and skills-affinity. BuildRoleContext passes YAML section names to ExecuteTemplate with no normalization, causing runtime failures.

**Changes:**
- `internal/prompts/templates/blocks/mandatory_docs.tmpl`: Change define "mandatory_docs" to define "mandatory-docs".
- `internal/prompts/templates/blocks/skills_affinity.tmpl`: Change define "skills_affinity" to define "skills-affinity".
- `internal/prompts/builder_test.go`: Update block-level tests (TestBlockMandatoryDocs_*, TestBlockSkillsAffinity_*) to use hyphen names when executing templates.

**Scope:** `internal/prompts/templates/blocks/mandatory_docs.tmpl`, `internal/prompts/templates/blocks/skills_affinity.tmpl`, `internal/prompts/builder_test.go`

**Done when:** Template define names match the YAML section keys exactly (mandatory-docs, skills-affinity). Block-level tests pass with hyphen names. BuildRoleContext with these section names does not error.

**Spec ref:** todo-mas.md -- Phase 2, [blocker] builder.go:190 / templates

---

### CP10: Propagate task.IntegrationFix into RoleContextData in buildTaskRoleContextData

**Intent:** RoleContextData.IntegrationFix exists and the integration-fix template block depends on it, but buildTaskRoleContextData() never copies task.IntegrationFix into the data object. Coder prompts silently lose integration-fix workflow instructions.

**Changes:**
- `internal/agent/prompt.go`: In the doer-specific block (currently gated by roleType == "doer" && config.Role == "coder"), add data.IntegrationFix = task.IntegrationFix. Also relax the gate: IntegrationBranch and IntegrationFix should be set for all doer roles, not just literal "coder". Change config.Role == "coder" to just roleType == "doer".
- `internal/agent/prompt_test.go` (or equivalent): Add a test verifying that when a task has IntegrationFix: true, the resulting RoleContextData.IntegrationFix is true.

**Scope:** `internal/agent/prompt.go`, `internal/agent/prompt_test.go`

**Done when:** buildTaskRoleContextData sets data.IntegrationFix = task.IntegrationFix for all doer roles. A test with task.IntegrationFix = true verifies the field is propagated. The config.Role == "coder" gate is replaced by roleType == "doer".

**Spec ref:** todo-mas.md -- Phase 2, [blocker] prompt.go:149 / role_context.go:38

---

### CP11: Drive TestBuildRoleContext_AllRoles section lists from production pipeline YAML

**Intent:** Test section lists are hardcoded subsets of the YAML context-sections, masking drift. The mandatory-docs and skills-affinity blockers went undetected because tests skip them. Drive tests from resolver.ContextSections(role) or the embedded pipeline.

**Changes:**
- `internal/prompts/builder_test.go`: In TestBuildRoleContext_AllRoles, replace hardcoded section lists with sections loaded from the production pipeline YAML (via embedded.PipelineConfig() and pipeline.LoadFromBytes() and resolver.ContextSections(role)). This ensures tests exercise the exact sections defined in the YAML.
- `internal/agent/strategy_test.go`: Update testPipelineYAML to include mandatory-docs and skills-affinity in all roles context-sections, matching the production pipeline.

**Scope:** `internal/prompts/builder_test.go`, `internal/agent/strategy_test.go`

**Done when:** TestBuildRoleContext_AllRoles loads section names from the embedded pipeline YAML rather than hardcoding them. strategy_test.go test fixture includes mandatory-docs and skills-affinity for all roles. Adding or removing a section in the YAML automatically affects test coverage.

**Spec ref:** todo-mas.md -- Phase 2, [concern] builder_test.go:989-995 and [concern] strategy_test.go:27

---

## Dependency Graph

```
CP1  (authorizeClaimRelease)     -- no dependencies
CP2  (recover_agent worktree)    -- no dependencies
CP3  (TDD enforcement)           -- no dependencies
CP4  (claim_reviewer workflow)   -- no dependencies
CP5  (orchestrator singularity)  -- no dependencies
CP6  (orchestrator resolution)   -- no dependencies
CP7  (pipeline error surface)    -- no dependencies
CP8  (nil-resolver log)          -- no dependencies
CP9  (template naming)           -- no dependencies
CP10 (IntegrationFix propagation) -- no dependencies
CP11 (test-YAML parity)          -- depends on CP9
```

All Phase 1 tasks (CP1-CP8) are independent.
All Phase 2 tasks (CP9-CP11) are independent except CP11 depends on CP9 (template names must match before tests can load real sections).

## Spec Coverage Mapping

| Spec Item | Task |
|-----------|------|
| Phase 1: [concern] handlers_helpers.go:142-155 -- hardcoded role names in authorizeClaimRelease | CP1 |
| Phase 1: [concern] recover_agent.go:72 -- role == "coder" for worktree removal | CP2 |
| Phase 1: [concern] submit_review.go:123 -- runtimeRole == "coder" for TDD enforcement | CP3 |
| Phase 1: [concern] claim_reviewer_task.go:45 -- role == "code-plan-reviewer" for workflow inference | CP4 |
| Phase 1: [blocker] registration.go:69 -- singularity per role key, not type | CP5 |
| Phase 1: [blocker] state.go:57 / handlers_mutation.go:14 -- hardcoded orchestrator literal | CP6 |
| Phase 1: [suggestion] server.go:26-33 -- pipeline load error surfacing | CP7 |
| Phase 1: [suggestion] recover_agent.go:102-106 -- nil-resolver fallback log | CP8 |
| Phase 1: [concern] handlers_helpers.go:137 / recover_agent.go:71 -- custom doer/reviewer claim release | CP1 + CP2 (covered) |
| Phase 2: [blocker] builder.go:190 / pipeline.yaml / templates -- template define vs YAML name mismatch | CP9 |
| Phase 2: [blocker] prompt.go:149 / role_context.go:38 -- IntegrationFix not propagated | CP10 |
| Phase 2: [concern] builder_test.go:989-995 -- hardcoded test section lists | CP11 |
| Phase 2: [concern] strategy_test.go:27 -- test fixture omits sections | CP11 |
| Phase 2: [blocker] templates mandatory_docs/skills_affinity -- underscore vs hyphen | CP9 (same fix) |
