# 30 - Code-Enforced Agent Guardrails

## Context and Problem Statement

ADR-0026 (Role-Specific Prompt Templates) and ADR-0027 (Contract Compression) reduced agent context pressure by removing pairing-specific and role-irrelevant content from prompts and CORE.md. But some of the removed instructions were load-bearing — they enforced behaviors like "reviewers must not call merge tools" or "coders must write tests." Removing enforcement from prompts without replacing it elsewhere would weaken the system's guarantees.

Prompt-based enforcement has two fundamental weaknesses: it consumes context budget, and it's softer than code — agents can ignore or misinterpret instructions, especially under context pressure.

## Considered Options

1. **Keep enforcement in prompts** — accept the context cost as the price of safety.
2. **Move enforcement to Go code** — MCP handlers and ops validate preconditions, reject violations at runtime.

## Decision Outcome

Chose **Option 2**: enforce in Go what was compressed or removed from prompts and contract.

### Architecture

**MCP Role Boundary Enforcement** (`5c44943`):
- `requireRole` validation added to 7 MCP handlers
- Agents calling wrong-role tools (e.g. coder calling `liza_wt_merge` or `liza_submit_verdict`) are rejected with a clear error
- Uses `identity.ExtractRole` + `roles.Runtime*` constants (ADR-0024)
- Agent ID format validated via `identity.ValidateFormat`
- Also normalizes "reviewer-1" → "code-reviewer-1" across the codebase

**Pre-Execution Checkpoint & TDD Enforcement** (`bd38032`):
- `WriteCheckpoint` op: coders must write intent, validation plan, and files-to-modify before submitting for review
- `HasTestFiles` test detection: rejects code task submissions without test files in the diff (supports Go, Python, JS/TS, Shell, Ruby, Java, Kotlin, Rust)
- `tdd_not_required` waiver for legitimate cases (cosmetic changes where existing tests cover behavior)
- `HasCheckpoint` / `GetTDDWaiver` history inspection helpers
- TDD enforcement gate wired into `SubmitForReview`
- MULTI_AGENT_MODE.md checkpoint/iteration sections compressed — Go code is now authoritative

**Prompt/Code Enforcement Alignment** (`3bbe866`):
- `SubmitVerdict`: phased approach — read state, validate `ReviewCommit` vs worktree HEAD (catches post-submission tampering), then atomic update
- Reviewer prompt: TDD instruction reworded to focus on quality (test *presence* already enforced by Go gate), exemptions removed (redundant with `EffectiveType` gate), HEAD check converted from "REJECT" directive to early drift check (Go backend is authoritative)
- Coder prompt: `liza_write_checkpoint` added to tools section, TDD waiver note for cosmetic changes

### Rationale

ADRs 26-27 weakened contract and prompt enforcement to reduce context pressure. Code enforcement compensates: it's stronger (agents cannot bypass Go validation), cheaper (zero context tokens), and enables further prompt compression by making instructions redundant with runtime checks. The Go code implements requirements that were previously only stated in text — making the contract aspirational where the code is authoritative.

This creates a layered enforcement model:
- **Go code**: hard enforcement (role boundaries, TDD gates, checkpoint requirements)
- **Contract/prompts**: intent and judgment guidance (quality standards, collaboration patterns)

### Consequences

**Positive:**
- Role boundary violations are impossible, not just discouraged
- TDD enforcement is structural — no test files means submission rejected, regardless of what the agent "intended"
- Checkpoint requirement catches missing pre-execution planning at submit time
- ReviewCommit validation prevents post-submission tampering
- Enables further prompt compression — instructions redundant with code can be removed

**Limitations accepted:**
- New behavioral requirements must be implemented in Go, not just added to prompts
- Waiver mechanism (`tdd_not_required`) adds judgment calls to what was previously binary enforcement

---
*Reconstructed from commits bd38032, 3bbe866, 5c44943 (2026-02-27 to 2026-02-28)*
