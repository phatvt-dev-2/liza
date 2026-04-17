# Pre-Commit Hook Composition in Worktrees

## Problem Statement

`internal/ops/wt_create.go:117` configures every task worktree with `core.hooksPath = <worktree>/.liza-hooks/` and installs `internal/embedded/git-hooks/worktree-pre-commit.sh` as the sole `pre-commit` hook. That script runs `liza check-commit-allowed` and exits — it never invokes project pre-commit hooks.

Consequence: even when `.pre-commit-config.yaml` exists at the repo root, `git commit` in a task worktree does not auto-execute project hooks via the worktree commit hook. (A coder could still invoke `pre-commit run` manually, and `pre-commit install` run elsewhere would write to `.git/hooks/` — which `core.hooksPath` makes inert in worktrees.) `internal/prompts/templates/blocks/commit_workflow.tmpl:3` instructs coders as if project pre-commit fires automatically on commit. Today that automatic path doesn't exist; nothing wires project hooks into the worktree commit path.

This blocks the broader goal of bootstrapping pre-commit for greenfield projects (`20260417-precommit-bootstrap.md`): there is no point creating a config file the commit path won't execute.

## Design

`<worktree>/.liza-hooks/pre-commit` becomes a chain:

1. Run `liza check-commit-allowed <task-id>`. On reject (exit 1) → exit 1, abort commit.
2. If `<worktree>/.pre-commit-config.yaml` exists → run `pre-commit run` (no `--hook-stage` needed: `pre-commit run` defaults to the `pre-commit` stage, matching the git hook name. Project-level `default_stages` is not relied on). Exit with that command's exit code.
3. If config absent → exit 0 (allow commit; bootstrap spec covers introducing the config).

### Implementation surface

- `internal/embedded/git-hooks/worktree-pre-commit.sh` — extend the script to run the chain. The current fail-safe semantics (any unexpected exit code allows the commit) are preserved for the Liza guard step only; the project pre-commit step propagates its real exit code.
- `internal/ops/wt_create.go` — no behavioral change required. The script discovers config presence at commit time; the install path stays as-is.
- Template at `internal/prompts/templates/blocks/commit_workflow.tmpl` — wording stays accurate once chaining is live.

### Discovery semantics

The hook checks `<worktree>/.pre-commit-config.yaml` at commit time (worktree-local). The bootstrap spec's `depends_on` chain ensures non-bootstrap **coding** worktrees are branched from an integration HEAD that already contains the config (phase-gate inheritance, ADR-0048, propagates through coding tasks specifically; `internal/ops/wt_create.go:32`, `specs/protocols/worktree-management.md:33`). No per-commit git lookup or worktree-refresh path is required for coding worktrees.

Code-planning worktrees for non-bootstrap `output[]` entries may be created after the bootstrap *planning* task approves but before the bootstrap *coding* task merges; they commit code-plan markdown into integration without project pre-commit running. This joins the same cold-start bucket as epic-planner, US-writer, and architect worktrees — pre-bootstrap commits of non-source artifacts that the project pre-commit was never going to protect anyway.

**Rollout property:** the chain activates only in worktrees created after the script change ships. In-flight worktrees retain the old single-step hook until next `wt-create`; no forced re-install.

Path resolution: `pre-commit run` discovers the config relative to the worktree's working directory.

`pre-commit install` is **not** required and should not be run. `core.hooksPath = <worktree>/.liza-hooks/` makes `.git/hooks/pre-commit` inert; the chain script *is* the hook.

### `pre-commit` binary availability

If `pre-commit` is not installed when the chain reaches step 2:
- `command -v pre-commit` returns nothing → exit 1 with a clear message ("pre-commit configured but binary missing — install pre-commit or remove .pre-commit-config.yaml").
- Rationale: silent skip would re-introduce the very gap this spec closes.

## Open Questions

1. **Multi-stage hooks**: only the `pre-commit` stage is invoked (the chain runs in the `pre-commit` git hook). Future hooks (commit-msg, pre-push) require parallel chain points — out of scope here.
2. **Performance**: `pre-commit run` cold-start can be slow on first invocation. v1 accepts this; caching strategy is a follow-up if it becomes a complaint.

## Acceptance Criteria

1. A worktree commit with both Liza guard and project pre-commit configured runs the guard first, then project hooks, with correct exit codes for each.
2. A worktree commit with no `.pre-commit-config.yaml` behaves identically to today (Liza guard only, exit 0 if allowed).
3. A worktree commit with config but no `pre-commit` binary fails loudly, not silently.
4. `commit_workflow.tmpl:3-4` instructions are satisfiable end-to-end.
5. Fail-safe asymmetry preserved: Liza guard exit codes outside {0,1} allow commit (current behavior — the guard is a guard, not a lock); project pre-commit non-zero always blocks (project hooks are authoritative on commit content).

## Out of Scope

- Bootstrap of the config file itself — see `20260417-precommit-bootstrap.md`.
- Hook stages other than `pre-commit`.
- Monorepo / subdirectory configs (`.pre-commit-config.yaml` in subdirs of the worktree). v1 checks worktree root only.

## Status

Draft 2026-04-17. Precondition for `20260417-precommit-bootstrap.md`.
