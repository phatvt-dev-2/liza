# Liza v0.5.1

Adoption-focused release: interactive onboarding wizard, Bubbletea TUI, and
configurable integration branch — removing the three biggest friction points
for new users.

15 commits since v0.5.0 (2026-03-24).

## Contributors

- **Livio Gamassia** — interactive onboarding wizard (#3), TUI mockup design (#4)
- **Stephen Oberther** — enforce-init hook race condition fix (#2)

---

## Highlights

- **Interactive Onboarding Wizard** — `liza init` in a TTY launches a guided Huh-based wizard that detects existing contract files, offers backup/symlink options, and prints a post-init summary. Brownfield-safe: numbered `.bak` files prevent overwrite of previous backups.
- **Bubbletea TUI** — `liza tui` replaces `console.sh` and headless monitoring with a single interactive terminal. Color-coded status palette, progressive column disclosure (40/80/120/160 cols), fsnotify-driven refresh, inline anomaly alerts, and keyboard commands for spawn/pause/resume/add-task/checkpoint/stop. Third community-proposed feature.
- **Configurable Integration Branch** — `liza init --branch <name>` lets projects use an existing branch (e.g. `develop`) instead of always creating `integration`. Validated via `git check-ref-format`.

---

## Features

- Interactive onboarding wizard for `liza init` with contract conflict detection, backup management, and clean Ctrl+C handling (#3)
- Bubbletea TUI with full-width agent/task panels, activity feed, alert banner, and two-tier input (inline prompts + Huh form overlays) (#6)
- `--branch` flag on `liza init` to choose integration branch name
- `SUPPORT.md` written to `.liza/` during init for troubleshooting context
- `install.sh` supports building from a git branch via `BRANCH` env var
- MCP usage section added to liza-logs analyzer

---

## Fixes

| Fix | Impact |
|-----|--------|
| Prevent concurrent claim from deleting winner's worktree (ops layer) | Race condition where losing claimer's cleanup deleted the winner's worktree |
| Guard worktree deletion in claim path (second fix) | Additional guard at claim-task level for the same race window |
| Prefer `~/.local/bin` when on PATH, avoid unnecessary sudo | install.sh no longer prompts for sudo when user bin dir is available |

---

## Documentation

- Console TUI specification (`specs/goals/20260326-tui.md`)
- Guide for producing goal documents (`docs/how-to-produce-a-goal.md`)
- ADR-0052: Bubbletea TUI backfilled
- README rewritten with hardening focus and demo link
- Narrative extracted into dedicated doc
- OpenSpec added to competitive survey

---

## ADRs Added

| ADR | Title |
|-----|-------|
| 0052 | Bubbletea TUI for Live Monitoring and Agent Management |

---

## Installation

**Quick install (macOS/Linux):**
```bash
curl -fsSL https://raw.githubusercontent.com/liza-mas/liza/main/install.sh | bash
```

**From source:**
```bash
go install ./cmd/liza/ ./cmd/liza-mcp/
```

**From a branch:**
```bash
BRANCH=feature-branch curl -fsSL https://raw.githubusercontent.com/liza-mas/liza/main/install.sh | bash
```

---

## Known Limitations

- TUI Phase 1: no panel navigation, item selection, or mouse support
- Spawn keybinding does not support `--cli` provider selection — use CLI for non-default providers
- `console.sh` deprecated but not yet removed (migration period)

---

## What's Next

- **TUI Phase 2** — panel navigation, item selection, agent management, log filtering
- **Architecture role pair** — define architecture from specs for coder consumption
- **Integration sub-pipeline** — validate commit batches before merge to main
- **Sprint Analyzer** role — analyze agent logs at end of sprint via lesson-capture
