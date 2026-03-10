# 39 - MCP Role-Based Access Control

## Context and Problem Statement

An architecture review discovered that seven state-mutating MCP handlers skipped role validation, inconsistent with other handlers. A coder agent could pause all agents via `liza_sprint_checkpoint`, delete agents via `liza_delete_agent`, or clear review claims — operations that should be restricted to the orchestrator role. MCP and CLI should enforce the same access control.

## Considered Options

1. **Add role-based access control to all MCP handlers** — match CLI access control semantics exactly.

No alternatives were considered. The principle is straightforward: MCP handlers are an API surface with the same trust model as the CLI, and must enforce the same constraints.

## Decision Outcome

Chose **Option 1**: add RBAC to all seven unprotected MCP handlers.

### Access Control Matrix

| Handler | Required Role | Rationale |
|---------|--------------|-----------|
| `liza_analyze` | orchestrator | Admin diagnostic |
| `liza_sprint_checkpoint` | orchestrator | Pauses all agents |
| `liza_update_sprint_metrics` | orchestrator | Admin metric recalculation |
| `liza_clear_stale_review_claims` | orchestrator | Affects other agents' claims |
| `liza_delete_agent` | orchestrator | Removes agents from workspace |
| `liza_wt_create` | doer | Worktree creation is a doer operation |
| `liza_wt_delete` | doer or orchestrator | Orchestrator needs cleanup access for superseded/blocked tasks |

Additionally, `liza_delete_agent` was normalized to use `target_agent_id` + `agent_id` (consistent with other mutation tools) instead of a one-off `caller_id` parameter.

Orchestrator prompt templates were updated with `agent_id` parameter payloads so live prompts match the new handler schemas. Builder tests assert the exact parameterized snippets to prevent future schema/prompt drift.

### Rationale

MCP handlers are a machine-protocol API surface. Without RBAC, any agent could perform any mutation — the access control existed only in the CLI layer. This extends the code-enforced guardrails philosophy (ADR-0030): security constraints belong in code, not in prompt instructions that agents might ignore.

### Consequences

**Positive:**
- MCP and CLI now enforce identical access control
- Prevents privilege escalation through the MCP interface
- Builder tests catch future schema/prompt drift

**Limitations accepted:**
- None identified

**Extends:** ADR-0030 (Code-Enforced Agent Guardrails) — same philosophy applied to MCP access control.

---
*Reconstructed from commit e491e07 (2026-03-09)*
