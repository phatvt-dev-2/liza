# 57 - MCP Server Removal and CLI-Native Access Control

## Context and Problem Statement

Liza exposed agent operations through two parallel surfaces: a CLI (`liza`) and an MCP server (`liza-mcp`). The MCP server was introduced partly for role-based access control (RBAC) — preventing agents from calling operations outside their role's allowed-operations list — and partly for structured responses.

In practice, maintaining both surfaces was painful for little added value. MCP RBAC provided no real security boundary: the agent passes its own `agent_id` as a parameter (self-declared identity with no authentication), and when MCP was unavailable (e.g. OOM) agents fell back to CLI commands that had no RBAC checks. The security guarantee was only as strong as the weakest access path — prompt compliance — which both surfaces depended on equally.

The breaking point came when MCP reliability failures in one project caused completed work to be unsubmittable. Agents burned 15-24 turns guessing CLI syntax when the MCP server went down under memory pressure. Fixing this required adding CLI equivalents for every MCP-only tool — six new commands. Once every MCP tool had a CLI fallback, the MCP server was a redundant parallel surface with concrete, recurring costs:

- **Token overhead**: ~30 tool schemas loaded into every agent's context before any work begins
- **Reliability**: MCP servers become unresponsive under memory pressure, stalling agents
- **External dependency risk**: Provider changes (e.g. Codex breaking custom MCP support) required workarounds in every tool definition
- **Two-surface maintenance**: CLI commands and MCP handlers were parallel surfaces over the same ops layer, with changes reconciled across both

MCP brought nothing valuable that couldn't be added to the CLI.

## Considered Options

1. **Remove MCP server, move RBAC and structured output to CLI** — add `--agent-id` role validation on state-mutating commands, `--json` flag for structured output, delete `internal/mcp/` and `cmd/liza-mcp/` entirely.

No alternatives were considered. The decision to keep only the CLI had been forming for some time as the maintenance burden accumulated. The CLI fallback work made MCP's redundancy undeniable.

## Decision Outcome

Chose **Option 1**: CLI-native access control with full MCP server removal.

### Architecture

**CLI RBAC via `--agent-id` validation.** Role validation added to state-mutating commands before calling the ops layer, using the identity sources that already exist:

| Identity Pattern | Source | Commands |
|---|---|---|
| Positional `<agent-id>` | `cobra.ExactArgs(2)` | `claim-task` |
| `--agent-id` flag | `requireAgentID(cmd)` | `submit-for-review`, `handoff`, `submit-verdict`, `await-verdict`, `await-resubmission`, `mark-blocked`, `write-checkpoint`, `set-task-output`, `wt-merge` |
| Auto-resolved orchestrator | `resolveOrchestratorID(cmd)` | `add-task`, `add-tasks`, `supersede-task`, `cancel-task`, `assess-blocked`, `assess-hypothesis-exhausted` |
| `--changed-by` (audit-only) | Defaults to `"human"` | `pause`, `stop`, `start`, `resume`, `replan`, `release-claim` — **no RBAC** |
| No identity | — | `get`, `status`, `validate`, `version`, `init`, `tui` — read-only, **no RBAC** |

**Structured output via `--json`.** Added to finite, non-interactive commands. The ops layer already returns typed results and errors; `--json` serializes them directly instead of formatting for humans.

**Removed:**
- `cmd/liza-mcp/` binary and `internal/mcp/` package (server, handlers, middleware, protocol, registration, tests)
- MCP tool schemas from agent context
- MCP fallback instructions from agent prompts
- `.mcp.json` Liza server entry generation from `liza init`
- `.mcp.json` copying into worktrees
- Codex MCP server launch arguments
- `mcp__liza__*` permission entries from embedded settings
- MCP resources (`liza://state`, `liza://tasks`, `liza://agents`) — replaced by `liza get --json` and `liza status --json`

### Rationale

The ops service layer (ADR-0021) made this possible — all mutation logic was already centralized in `internal/ops`, used by both CLI and MCP handlers. Removing MCP meant deleting one caller without touching business logic. The trust model is unchanged: same self-declared identity, same prompt-compliance enforcement. The difference is one surface to maintain instead of two.

### Consequences

**Positive:**
- Single access surface eliminates two-surface maintenance and sync burden
- Agent context freed from ~30 MCP tool schemas
- No more MCP reliability failures under memory pressure
- Simpler agent prompts — exact CLI commands instead of MCP tool names that vary by provider
- Removal of external dependency on provider MCP support

**Limitations accepted:**
- Same trust model as before (self-declared identity, prompt compliance) — authenticated identity deferred
- No MCP interface for future tooling that might benefit from it — acceptable since CLI + `--json` covers all use cases

**Supersedes:** ADR-0039 (MCP Role-Based Access Control) and ADR-0043 (MCP Middleware and Declarative Registration). The RBAC and middleware patterns are gone with the MCP server; CLI RBAC replaces them.

**Extends:** ADR-0021 (Ops Service Layer) — the ops layer's surface-agnostic design is what enabled removing one surface cleanly. ADR-0008 (Multi-LLM Provider Support) — removing provider-specific MCP tool naming reduces per-provider maintenance.

---
*Reconstructed from commits 6d281803..90c132d5 (2026-04-06 to 2026-04-13)*
