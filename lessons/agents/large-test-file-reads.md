---
title: "Large Go test file reads"
trigger: "When reading Go test files (*_test.go)"
keywords: [_test.go, Read, offset, limit, Grep, context window, token budget]
date: 2026-03-24
---

## Context

Go test files in this project often exceed 10K tokens (table-driven tests, helpers, setup).

## Failure Mode

Full reads waste context budget on boilerplate. Multiple such reads trigger context pressure.

## Solution

Grep for the relevant function first, then `Read` with `offset`/`limit` on that region. Only read full files when the task requires understanding overall test structure.
