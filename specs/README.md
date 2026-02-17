# Liza Specification Index

## Quick Navigation

### Build (Intent)

| Document                        | Purpose |
|---------------------------------|---------|
| [Vision](<build/0 - Vision.md>) | Why Liza exists, target users, success metrics, risks |

### Functional (Current State)

| Document                                        | Purpose |
|-------------------------------------------------|---------|
| [Product Description](<functional/0 - Liza.md>) | What Liza is, domains, key integrations, scope |

### Architecture

| Document                                               | Purpose |
|--------------------------------------------------------|---------|
| [Overview](architecture/overview.md)                   | System components, data flow, directory structure |
| [Roles](architecture/roles.md)                         | Planner, Coder, Code Reviewer responsibilities |
| [State Machines](architecture/state-machines.md)       | Task states, agent states, exit codes |
| [Blackboard Schema](architecture/blackboard-schema.md) | state.yaml structure, locking, operations |
| [ADR/](architecture/ADR/)                              | Architecture Decision Records (created as decisions arise) |

### Protocols

| Document | Purpose |
|----------|---------|
| [Task Lifecycle](protocols/task-lifecycle.md) | Claim, iterate, review, merge flow |
| [Sprint Governance](protocols/sprint-governance.md) | Checkpoints, retrospectives, spec evolution |
| [Circuit Breaker](protocols/circuit-breaker.md) | Systemic failure detection, severity classification |
| [Worktree Management](protocols/worktree-management.md) | Isolated workspaces, merge protocol |

### Implementation

| Document | Purpose |
|----------|---------|
| [Tooling](implementation/tooling.md) | Scripts, agent-blackboard interface, startup sequence |
| [Phases](implementation/phases.md) | Implementation roadmap (13 phases) |
| [Validation Checklist](implementation/validation-checklist.md) | v1 completion criteria |
| [Future](implementation/future.md) | v1.1 roadmap, deferred items, technical debt |

---

## Reading Order

**For understanding the system:**
1. [Vision](<build/0 - Vision.md>) — philosophy and rationale
2. [Product Description](<functional/0 - Liza.md>) — what Liza is today
3. [Architecture Overview](architecture/overview.md) — components and flow
4. [Roles](architecture/roles.md) — who does what
5. [Task Lifecycle](protocols/task-lifecycle.md) — how work flows

**For implementation:**
1. [Blackboard Schema](architecture/blackboard-schema.md) — data structures
2. [Tooling](implementation/tooling.md) — scripts and interfaces
3. [Phases](implementation/phases.md) — build order

**For operations:**
1. [Sprint Governance](protocols/sprint-governance.md) — checkpoints and retrospectives
2. [Circuit Breaker](protocols/circuit-breaker.md) — failure detection
3. [Validation Checklist](implementation/validation-checklist.md) — completeness check

---

## Key Concepts

### Four Pillars

1. **Behavioral contracts** — Tier 0 invariants turn agents into accountable peers
2. **Externally validated completion** — Coders cannot self-certify; Code Reviewers approve
3. **Specification system** — specs persist understanding across agent restarts
4. **Blackboard coordination** — all state visible through shared file

### Design Philosophy

See [Vision](<build/0 - Vision.md>) for the design philosophy and cost gradient.

---

## Related Documents

- [README.md](../README.md) — project overview
- [contracts/](../contracts/) — behavioral contracts (LOADER, CORE, modes)

---

## Document Status

| Category | Documents | Status |
|----------|-----------|--------|
| Build | 1 (0.md) | Complete |
| Functional | 7 (0.md + 1.1–1.6) | Complete |
| Architecture | 4 + ADR/ | Complete |
| Protocols | 4 | Complete |
| Implementation | 4 | Complete |
| Contracts | 4 | Pending extraction |

---

## Maintenance Notes

### Agent Runtime Reference

`agent-runtime-reference.md` is a consolidated reference for agents at runtime. It distills operational content from multiple specs into a single doc agents read during bootstrap. The source lives at [`docs/for-agent-eyes/agent-runtime-reference.md`](../docs/for-agent-eyes/agent-runtime-reference.md); `liza init` copies it to `.liza/agent-runtime-reference.md` in each project.

**Sync requirement:** When updating these specs, check if the agent runtime reference needs corresponding updates:

| Spec | Agent reference sections affected |
|------|-----------------------------------|
| `roles.md` | Role-specific sections (capabilities, constraints, protocols) |
| `blackboard-schema.md` | Field reference tables |
| `state-machines.md` | State transitions tables |
| `task-lifecycle.md` | Iteration protocol, blocking protocol, handoff |

The specs contain rationale and design context; the runtime reference contains only what agents need to act.
