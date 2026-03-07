# 37 - Rebase Conflict Detection at Submission

## Context and Problem Statement

When multiple agents work in parallel, rebase conflicts are inevitable at submission time. Agents were observed wasting turns attempting manual conflict resolution — a task that is unreliable for LLMs, especially under context pressure. The core principle: deterministic operations that can be done reliably, faster, and cheaper should not be delegated to agents.

## Considered Options

1. **Let agents resolve conflicts** — instruct them to handle `git rebase` conflicts. Unreliable; agents struggle with the interactive resolution workflow.
2. **Detect and short-circuit** — catch conflicts at submission, abort cleanly, transition to a recovery state, release the agent.

## Decision Outcome

Chose **Option 2**: detect rebase conflicts programmatically and handle them as a state machine transition.

### Architecture

**Typed error** (`internal/git/worktree.go`):
```go
type RebaseConflictError struct {
    Output string  // raw git output with conflict details
}
```

`RebaseOnto()` classifies git output: if it contains `"CONFLICT"` or `"could not apply"`, returns `*RebaseConflictError`; otherwise returns a generic error.

**Flow in `SubmitForReview`:**
1. Rebase onto integration branch
2. On error → always abort rebase first (clean worktree state)
3. `errors.As(err, &rebaseConflict)`:
   - **Not a conflict** → return generic error (agent can retry)
   - **Conflict** → transition task to `INTEGRATION_FAILED`, release agent
4. Return `&IntegrationFailedError{Reason: IntegrationReasonMergeConflict}`

**State machine transitions added:**
```
IMPLEMENTING       → INTEGRATION_FAILED
CODE_PLANNING      → INTEGRATION_FAILED
INTEGRATION_FAILED → IMPLEMENTING  (re-queue for another agent)
INTEGRATION_FAILED → ABANDONED
```

`INTEGRATION_FAILED` is not terminal — it routes back to the executing state for re-claiming by a different agent (or the same agent after the conflicting work is merged).

### Rationale

Conflict resolution requires understanding both sides of a merge — context that a coder agent typically lacks (they only see their own changes). Programmatic detection is instant, deterministic, and leaves the worktree in a clean state. The re-queue mechanism lets another agent pick up the task after the conflicting changes have been integrated.

### Consequences

**Positive:**
- Eliminates wasted agent turns on unreliable conflict resolution
- Worktree always left in clean state (rebase aborted before state transition)
- Deterministic — no agent judgment involved
- Non-terminal — task automatically becomes available for re-claiming

**Limitations accepted:**
- The re-claiming agent must redo the work (no partial merge preservation)
- Human must eventually merge the conflicting work to unblock the queue

---
*Reconstructed from commits d451e14, a89a650 (2026-03-05 to 2026-03-06)*
