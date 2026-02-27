---
title: "CLI and MCP surfaces must stay in sync"
trigger: "When modifying CLI commands, flags, validation, or vocabulary that MCP handlers also expose"
keywords: [CLI, MCP, handlers, server.go, main.go, validation, flags, tool descriptions]
date: 2026-02-27
---

## Context

CLI commands and MCP handlers are parallel surfaces over the same ops layer. Both carry validation logic, role vocabulary, flag defaults, help/description text, and error messages.

## Failure Mode

A change to one surface leaves the other stale. Divergence goes undetected until a cross-cutting feature exposes the inconsistency.

## Solution

When changing any CLI command behavior, grep for the equivalent MCP handler and reconcile. Same in reverse. Check: validation rules, string constants, defaults, descriptions, and test fixtures for both surfaces.

## References

- `cmd/liza/main.go` — CLI definitions
- `internal/mcp/handlers.go` — MCP handlers
- `internal/mcp/server.go` — MCP tool descriptions
