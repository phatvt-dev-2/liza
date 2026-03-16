# 16 - Embedded Template Architecture for Agent Prompts

## Context and Problem Statement

The Go CLI rewrite (ADR-0012) initially built agent prompts using Go string formatting (`fmt.Sprintf`, `WriteString`, `Fprintf`) in `builder.go`. As the prompt system grew to cover three agent roles (planner, coder, reviewer) and multiple wake triggers, maintaining multi-line prompt content interleaved with Go code became painful.

Additionally, operational context for agents was split across three potentially inconsistent sources:
1. **Contracts** (CORE.md, MULTI_AGENT_MODE.md) — behavioral rules
2. **agent-runtime-reference.md** — operational tables (state machine, fields, anomalies) deployed as a file agents were told to read at bootstrap
3. **Prompts** — role-specific instructions built in Go

This three-source split created drift risk: the runtime reference could go stale relative to the prompt, and agents could skip the file read (wasting a tool call or silently proceeding without context).

## Considered Options

1. **Keep string formatting in Go** — Simple, but content and logic remain entangled. Prompt changes require Go knowledge and recompilation.
2. **External template files loaded at runtime** — Flexible, but adds deployment complexity (files must be co-located with binary).
3. **Embedded Go templates** (`text/template` + `embed.FS`) — Content separated from logic, compiled into the binary. Single-binary deployment preserved.

## Decision Outcome

Chose **Option 3**, implemented in two steps:

1. Extract all prompt content from Go code into `.tmpl` files using `text/template`, embedded via `embed.FS` (commit `896be0a`).
2. Eliminate `agent-runtime-reference.md` entirely by inlining shared operational content into `shared_reference.tmpl`, included in every agent prompt at build time. Trim `MULTI_AGENT_MODE.md` to behavioral-only (commit `ff6bf69`).

### Architecture

**Single source of truth per concern:**

| Concern | Owner | Previously |
|---------|-------|------------|
| Shared operational content (state machine, fields, anomalies, leases, exit codes) | `shared_reference.tmpl` | `agent-runtime-reference.md` |
| Role-specific operational content (logging, blocking, review scope) | `{role}_context.tmpl` | Mixed in Go code + runtime ref |
| Behavioral rules (checkpoints, boundaries, iteration, scope) | `MULTI_AGENT_MODE.md` | Mixed with operational tables |
| Prompt structure (bootstrap, MCP tools, first actions) | `base_prompt.tmpl` | Go code pointing at external file |

**Template inclusion hierarchy:**
```
base_prompt.tmpl
├── shared_reference.tmpl     (operational context for all roles)
├── planner_context.tmpl      (or coder_, reviewer_)
└── wake_*.tmpl               (trigger-specific context)

blocks/                        (modular context sections)
├── worktree_rules.tmpl        (worktree grounding — rendered early)
├── integration_fix.tmpl       (conditional, for merge conflicts)
├── implementation_phase.tmpl  (role-specific implementation steps)
└── ...                        (other context sections)
```

### Rationale

- Maintaining prompts within Go code was painful — content changes required navigating string concatenation logic
- Three potentially inconsistent sources of truth for agents (contract, runtime reference, prompt) created a structural drift risk
- Inlining shared reference content at build time means agents always get current operational context without needing to read an external file
- `embed.FS` preserves the single-binary deployment model from ADR-0012

### Consequences

**Positive:**
- Content fully separated from logic — prompt authors don't need Go knowledge
- Single source of truth per concern — no drift between runtime ref and prompts
- `builder.go` reduced from ~700 lines to ~100 lines of template orchestration
- Template reuse via `{{template "name" .}}` eliminates duplication across roles

**Limitations accepted:**
- All prompt changes require rebuilding the binary — the runtime reference was the last edit-without-rebuild path, now eliminated
- Go `text/template` syntax is less expressive than Go code (no complex conditionals, limited string manipulation)
- Template errors surface at runtime (first use) rather than compile time — mitigated by test coverage
- Data adapter structs required to pre-compute derived values for templates

---
*Reconstructed from commits 896be0a..ff6bf69 (2026-02-16 to 2026-02-17)*
