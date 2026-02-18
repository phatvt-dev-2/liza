# 18 - Two-Step Deployment (Setup + Init)

## Context and Problem Statement

`liza init` previously copied contracts and skills into each project's `.liza/` directory. Contract files use cross-references with absolute paths (e.g., `~/.liza/MULTI_AGENT_MODE.md`). When contracts were copied to project-local directories, these cross-references broke because the target paths didn't exist locally.

Additionally, duplicating contracts and skills across every project made maintenance brittle — every contract update required re-initializing all projects, with risk of version skew between them.

## Considered Options

1. **Fix relative paths in contracts** — Rewrite cross-references to use relative paths. Fragile: breaks the single-source property and path resolution depends on reader working directory.
2. **Keep project-local copies with post-processing** — Rewrite paths during copy. Complex, error-prone, and still duplicates content.
3. **Split into global setup + project init** — Single canonical location (`~/.liza/`) for contracts/skills, project-local scaffold with symlinks. Cross-references work because target paths exist.

## Decision Outcome

Chose **Option 3**. Two commands with clear responsibilities:

### Architecture

**`liza setup`** — One-time global installation:
- Writes contracts and skills flat to `~/.liza/`
- Verbose output lists every file written (audit trail for files outside git)
- `--force` flag for updates: lists files to overwrite, asks confirmation
- Protects user-customizable files (e.g., `AGENT_TOOLS.md`) with individual prompts
- Creates `.bak` backups before overwriting

**`liza init`** — Project-local scaffold:
- Creates `.liza/` blackboard (state.json, archive)
- Writes `.claude/settings.json` and `.mcp.json`
- Creates symlinks: `CLAUDE.md`, `AGENTS.md`, `GEMINI.md` → `~/.liza/CORE.md`
- Requires `liza setup` first (clear error if `~/.liza/` missing)
- Preserves existing correct symlinks; prompts before replacing other files

**Two-layer settings:**

| Layer | Location | Managed By | Contains |
|-------|----------|------------|----------|
| Project | `.claude/settings.json` | `liza init` | MCP tools, permissions for liza commands |
| Global | `~/.claude/settings.json` | User (manual) | Personal MCP tools, API keys, paths |

### Rationale

Symlinks and relative paths between contract files don't coexist well. A single canonical location (`~/.liza/`) preserves cross-references and eliminates per-project duplication. Updates propagate instantly to all projects via symlinks — no re-init needed for contract changes.

### Implementation Notes

**User-customizable file protection:** `AGENT_TOOLS.md` is embedded in the binary but intended for user customization. `liza setup --force` handles it specially: bulk confirmation for standard files, individual prompt for customizable ones, `.bak` backup in all cases.

### Consequences

**Positive:**
- Contract cross-references work — `~/.liza/MULTI_AGENT_MODE.md` resolves correctly
- Single update point — change `~/.liza/CORE.md`, all projects see it immediately
- Clear separation: global concerns (setup) vs project concerns (init)
- Backup mechanism prevents accidental loss of customizations

**Limitations accepted:**
- Requires `liza setup` before first `liza init` — extra step for new users (enforced with clear error message)
- Global state outside git — `~/.liza/` is not version-controlled per project
- Symlinks may confuse tools that don't follow them (mitigated: gitignored in liza repo)

---
*Reconstructed from commits 85623ef..62f860a (2026-02-17)*
