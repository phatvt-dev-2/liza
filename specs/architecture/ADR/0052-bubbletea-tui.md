# 52 - Bubbletea TUI for Live Monitoring and Agent Management

## Context and Problem Statement

Liza reached a maturity level where it could be used by anyone without major risk of a bad experience.
But adoption friction remained: the monitoring surface was `console.sh` invoked via `watch -n 2` — a black-and-white,
read-only terminal dump that spawned 5+ subprocesses per refresh.
Operators needed a second terminal to act on what they saw, and a third for anomaly monitoring (`liza tui` in headless mode).
The CLI-only command surface made the system feel like a tool for its author, not for users.

The TUI is part of a broader adoption push (alongside the interactive onboarding wizard) to remove friction for new users. It is also the third feature proposed by community members (after the Go port and the onboarding wizard).

## Considered Options

1. **Bubbletea TUI replacing console.sh and headless `liza tui`** — single interactive terminal with live dashboard, keyboard commands, and inline anomaly monitoring.

No alternatives were considered. Bubbletea was already an indirect dependency via Huh (used by the init wizard), making it a zero-cost promotion to direct dependency. Claude proposed it; no cons were raised.

## Decision Outcome

Chose **Option 1**: interactive Bubbletea TUI under `liza tui`, with `--headless` flag preserving the previous behavior for CI/cron.

### Architecture

**New package:** `internal/tui/` with Bubbletea's Model-Update-View architecture:

```
internal/tui/
├── model.go      — main Bubbletea model (state, panels, input mode, alert cache)
├── update.go     — message handling (keys, ticks, state refresh, window resize)
├── view.go       — rendering (header, panels, footer, alert banner, help overlay)
├── styles.go     — Lipgloss style definitions (colors, borders, layout)
├── commands.go   — Bubbletea Cmd functions (read state, run checks, exec actions)
└── keymap.go     — key binding definitions
```

**Unified status color palette:** A single color system applies to both agent and task statuses. Task status matching uses pattern hierarchy (exact → suffix → prefix → fallback), so pipeline-configurable statuses render correctly without TUI changes.

**Progressive column disclosure:** Both agent and task panels adapt visible columns to terminal width (40/80/120/160 column thresholds), preserving all information from `console.sh` while working on narrow terminals.

**Data refresh:** fsnotify-driven (reactive, via `Blackboard.WatchForChanges()`) with 10-second poll tick as fallback. All 13 anomaly checks from `watch.go:runChecks()` run on the poll tick.

**Command integration:** State-modifying actions (pause, resume, add task, checkpoint, stop) call `commands.*Command()` directly — same process, no subprocess overhead. Exception: agent spawn uses `exec.Command().Start()` because agents are long-running processes needing their own PTY.

**Two-tier input:** Simple inputs (spawn role, pause reason) use inline prompts with tab-completion. Complex inputs (add task) use Huh form overlays, composing natively into Bubbletea.

### Rationale

- **Zero new dependencies** — Bubbletea, Lipgloss, and Bubbles were already indirect deps via Huh; only promoted to direct
- **Unified monitoring** — replaces three separate surfaces (console.sh, headless tui, manual CLI) with one
- **Pipeline-agnostic** — pattern-based status coloring works with any pipeline configuration without TUI code changes
- **In-process commands** — direct function calls instead of subprocess spawning for instant feedback

### Consequences

**Positive:**
- Single terminal for monitoring and operating the system
- Color-coded status differentiation (vs. B&W text)
- Reactive refresh (vs. 2-second poll with 5+ subprocesses)
- Keyboard-driven operation reduces context switching

**Limitations accepted:**
- Phase 1 scope: no panel navigation, item selection, or mouse support (deferred to Phase 2)
- Provider selection (`--cli`) not available in spawn keybinding — operators must use CLI for non-default providers
- `console.sh` deprecated but not yet removed (migration period)

### Implementation Notes

**Spec-first:** Full specification written before implementation (`specs/goals/20260326-tui.md`), including mockups, column priority tables, and data flow. Implemented by Liza agents via multi-phase planning (5 code plans, 8,500 lines).

---
*Reconstructed from commits 4313f55..775ec63 (2026-03-27)*
