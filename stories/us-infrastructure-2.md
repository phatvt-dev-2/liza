# User Stories: Clean Code — Infrastructure

Status: draft

## Goal

Apply clean-code refactoring to all 8 non-test .go files across 5 infrastructure packages (internal/db, internal/embedded, internal/git, internal/identity, internal/pipeline) so that the code follows Uncle Bob's clean-code principles, improving readability and maintainability for contributors.

## Context

The infrastructure layer provides foundational capabilities to the Liza orchestrator: persistent state management (db), resource embedding (embedded), git operations (git), agent identity parsing (identity), and pipeline configuration and resolution (pipeline). These packages are stable, well-tested, and actively used across the codebase.

Applying the clean-code skill in full-file mode analyzes each file against the Uncle Bob principle catalog and applies behavior-preserving transformations in validated batches. This is a mechanical refactoring task — no new features or behavioral changes.

Total scope: 8 files, ~2,411 lines.

## Personas

- **Liza Contributor**: A Go developer maintaining or extending the Liza multi-agent orchestrator. They navigate the codebase via IDE features (go-to-definition, find-usages) and expect infrastructure code to be self-documenting with clear function boundaries, meaningful names, and consistent error handling. Familiar with Go idioms and the project's architectural patterns (blackboard state, worktrees, pipeline config).

## General information

### References

- spec: /home/tangi/Workspace/liza/clean-code.md — goal definition
- skill: ~/.liza/skills/clean-code/SKILL.md — clean-code protocol (pre-flight, analysis, transformation, validation)
- skill: skills/clean-code/languages/go.md — Go-specific patterns and tool map
- source: internal/db/blackboard.go — thread-safe YAML state access with caching and atomic writes (343 lines)
- source: internal/db/doc.go — package-level documentation (15 lines)
- source: internal/db/watcher.go — fsnotify-based state change watcher with debouncing (147 lines)
- source: internal/embedded/embedded.go — embedded resource management, settings merging, file writing (506 lines)
- source: internal/git/worktree.go — git operations wrapper: branches, worktrees, merge, rebase, plumbing (567 lines)
- source: internal/identity/resolver.go — agent ID resolution, validation, and parsing (141 lines)
- source: internal/pipeline/config.go — pipeline configuration types, YAML parsing, structural validation (349 lines)
- source: internal/pipeline/resolver.go — pipeline state resolution queries: status lookup, transitions, terminal states (343 lines)

### Non-Functional Requirements

- NFR-000-1: All transformations must be behavior-preserving — refactoring only, no functional changes.
- NFR-000-2: All existing tests must pass after each transformation batch and at final validation.
- NFR-000-3: Pre-commit hooks must pass on all modified files at final validation.
- NFR-000-4: The clean-code skill's full protocol must be followed: language profile loading, pre-flight checks, analysis with violation catalog, batched transformations with per-batch test validation, and convergence check.

### Related External Components

- Component C-001 - clean-code skill: The refactoring methodology and principle catalog applied during this work.

### Out of Scope

- Test files (*_test.go) in any of the 5 packages — excluded by task scope.
- Bug fixes discovered during analysis — flag per clean-code protocol, do not auto-fix.
- New features or behavioral changes — this is refactoring only.
- Packages outside the 5 listed (internal/db, internal/embedded, internal/git, internal/identity, internal/pipeline).
- Restructuring operations (cross-module moves, file splits) — if analysis identifies restructuring opportunities, flag them per the skill's restructuring protocol for future consideration. Do not auto-execute.
- Formatting-only changes — delegated to pre-commit hooks per clean-code skill anti-patterns.

### Assumptions

- **ASM-000-1**: doc.go (15 lines, package-level documentation only) will require no code transformations since it contains only a package comment with no executable code. The Coder should still include it in the analysis for completeness. — *Why*: The clean-code principle catalog applies to code constructs (functions, names, control flow), not documentation comments. — ⚠️ Confidence: HIGH
- **ASM-000-2**: The Go language profile at `skills/clean-code/languages/go.md` provides all required tool map variables ($TEST_CMD, $COVERAGE_CMD, etc.) for the pre-flight phase. — *Why*: Liza is a Go project and the profile exists in the repository. — ⚠️ Confidence: HIGH

### Open Questions

(none)

---

## Story ST-001 - Clean code for state storage and identity packages

### References

- source: internal/db/blackboard.go (343 lines) — thread-safe YAML state access
- source: internal/db/doc.go (15 lines) — package documentation
- source: internal/db/watcher.go (147 lines) — fsnotify state change watcher
- source: internal/identity/resolver.go (141 lines) — agent ID resolution and validation

### User Story

**As a** Liza Contributor, **I want** the internal/db and internal/identity packages to follow clean-code principles (meaningful names, small functions, single responsibility, DRY, early returns, clarity over brevity), **so that** I can understand and modify the state storage and agent identity code without needing to decipher abbreviations, trace through deeply nested logic, or wonder about function responsibilities.

### Acceptance Criteria

- AC-001-1: Given the clean-code skill is applied in full-file mode to blackboard.go, doc.go, watcher.go, and identity/resolver.go, when the analysis phase completes, then all identified clean-code violations are resolved through behavior-preserving transformation batches.
- AC-001-1b: Given a file has no violations (e.g., doc.go), when analysis completes, then no transformations are applied to that file and it is reported as clean.
- AC-001-2: Given all transformations are applied, when `go test ./internal/db/...` is run, then all tests pass.
- AC-001-3: Given all transformations are applied, when `go test ./internal/identity/...` is run, then all tests pass.
- AC-001-4: Given all transformations are applied, when pre-commit hooks are run on the modified files, then all hooks pass.

### Depends on:

(none)

### Out of Scope

- internal/db/*_test.go and internal/identity/*_test.go — test files are excluded by task scope.
- Concurrency-related refactoring (sync.Map usage, sync.RWMutex patterns in Blackboard) — these are correctness-critical patterns that should not be refactored for style without deep concurrency analysis.

### Assumptions

(none — general assumptions apply)

### Open Questions

(none)

---

## Story ST-002 - Clean code for embedded resources and git operations

### References

- source: internal/embedded/embedded.go (506 lines) — resource embedding, settings merging, file writing
- source: internal/git/worktree.go (567 lines) — git operations: branches, worktrees, merge, rebase, plumbing

### User Story

**As a** Liza Contributor, **I want** the internal/embedded and internal/git packages to follow clean-code principles (meaningful names, small functions, single responsibility, DRY, early returns, clarity over brevity), **so that** I can understand and extend the resource embedding logic and git operations without navigating large files (500+ lines each) with mixed levels of abstraction.

### Acceptance Criteria

- AC-002-1: Given the clean-code skill is applied in full-file mode to embedded.go and worktree.go, when the analysis phase completes, then all identified clean-code violations are resolved through behavior-preserving transformation batches.
- AC-002-1b: Given a transformation batch fails tests, when the Coder detects the failure, then the batch is reverted and the failure is reported (no silent test regressions).
- AC-002-2: Given all transformations are applied, when `go test ./internal/embedded/...` is run, then all tests pass.
- AC-002-3: Given all transformations are applied, when `go test ./internal/git/...` is run, then all tests pass.
- AC-002-4: Given all transformations are applied, when pre-commit hooks are run on the modified files, then all hooks pass.

### Depends on:

(none)

### Out of Scope

- Changes to `//go:embed` directives or build-time variable declarations in embedded.go — these are build infrastructure, not code quality.
- Changes to error detection patterns in worktree.go that match on git stderr strings — these are fragile but functional, and altering the match strings risks behavioral changes.
- Changes to the exec/execInDir helpers in worktree.go — these are low-level plumbing shared by all git methods and are single-responsibility already.

### Assumptions

(none — general assumptions apply)

### Open Questions

(none)

---

## Story ST-003 - Clean code for pipeline configuration and resolution

### References

- source: internal/pipeline/config.go (349 lines) — pipeline config types, YAML parsing, structural validation
- source: internal/pipeline/resolver.go (343 lines) — pipeline state resolution queries

### User Story

**As a** Liza Contributor, **I want** the internal/pipeline package to follow clean-code principles (meaningful names, small functions, single responsibility, DRY, early returns, clarity over brevity), **so that** I can understand and extend the pipeline configuration validation and state resolution logic without tracing through duplicated patterns or reasoning about principle conflicts (e.g., DRY vs type-safe APIs) that should have been resolved during refactoring.

### Acceptance Criteria

- AC-003-1: Given the clean-code skill is applied in full-file mode to config.go and resolver.go, when the analysis phase completes, then all identified clean-code violations are resolved through behavior-preserving transformation batches.
- AC-003-1b: Given a principle conflict is identified (e.g., DRY vs type-safe API for structurally identical status methods), when the Coder encounters it, then the conflict is flagged with both options per the skill's principle conflict resolution guidance rather than silently choosing one.
- AC-003-2: Given all transformations are applied, when `go test ./internal/pipeline/...` is run, then all tests pass.
- AC-003-3: Given all transformations are applied, when pre-commit hooks are run on the modified files, then all hooks pass.

### Depends on:

(none)

### Out of Scope

- Changes to exported types (PipelineConfig, Pipeline, RolePairDef, RolePairStates, SubPipeline, TransitionDef, Resolver) — these are data structures and interfaces consumed across the codebase.
- Adding thread-safety to Resolver's lazy cache — Resolver is currently used single-threaded, and adding concurrency primitives would be a behavioral change, not a clean-code transformation.

### Assumptions

(none — general assumptions apply)

### Open Questions

(none)
