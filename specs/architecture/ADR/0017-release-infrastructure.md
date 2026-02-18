# 17 - Release Infrastructure

## Context and Problem Statement

The Go CLI (ADR-0012) needed a distribution mechanism. Users need to install `liza` and `liza-mcp` binaries without requiring a Go toolchain. The project also needed CI for quality gates (lint, test, build) on every push.

## Considered Options

1. **`go install github.com/...`** — Simplest distribution. Requires Go toolchain on user machine, no pre-built binaries.
2. **GoReleaser + GitHub Releases + curl-pipe-sh installer** — Zero-dependency binary distribution. Cross-platform builds triggered by git tags.
3. **Package managers (Homebrew, apt/rpm)** — Wider reach but significant maintenance overhead for formula/spec upkeep.

## Decision Outcome

Chose **Option 2**, ported from the `liza-go` fork which already had a working release setup.

### Architecture

**CI pipeline** (`.github/workflows/ci.yml`):
- Triggers on push and PR to main
- Runs: lint → test → build

**Release pipeline** (`.github/workflows/release.yml`):
- Triggers on version tags (`v*`)
- Runs GoReleaser to build cross-platform binaries and publish GitHub Release

**GoReleaser** (`.goreleaser.yaml`):
- Builds two binaries: `liza` (CLI) and `liza-mcp` (MCP server)
- Cross-compilation: linux/darwin × amd64/arm64
- Produces tar.gz archives with checksums

**Installer** (`install.sh`):
- curl-pipe-sh: `curl -fsSL https://... | sh`
- Detects OS/arch, downloads correct archive, extracts to `/usr/local/bin`

### Rationale

Ported from `../liza-go` which had a working release pipeline. The author is not deeply familiar with Go distribution conventions, so reusing a proven setup was pragmatic.

### Consequences

**Positive:**
- Zero-dependency installation — no Go toolchain required
- Cross-platform: Linux and macOS, both amd64 and arm64
- Tag-triggered releases — push a tag, binaries appear
- CI catches regressions before merge

**Limitations accepted:**
- No Homebrew tap — users can't `brew install liza`
- No apt/rpm packages — no native package manager integration
- curl-pipe-sh requires trust in the download source
- GoReleaser config was ported, not deeply understood — may need revisiting

---
*Reconstructed from commit 8eeabe9 (2026-02-16)*
