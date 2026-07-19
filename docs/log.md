---
type: Log
title: Documentation Change Log
description: Chronological record of changes to the docs/ bundle for plattform-okf-mcp.
tags: [changelog, log, okf]
timestamp: 2026-07-18T00:00:00Z
---

# Directory Update Log

## 2026-07-18

**Creation**: `validate_doc` MCP tool — validates OKF-conformant documents with error/warning/notification findings (E0–E3, W1–W4, N1).
**Creation**: `get_index` MCP tool — returns the bundle tree showing all documents and their directory structure.
**Creation**: `get_log` MCP tool — returns structured log entries from the documentation change log with date/action/target filters.
**Creation**: `--validate` CLI flag — validates OKF docs and exits with code 0/1/2 without starting the MCP server.
**Creation**: pre-commit hook — `.githooks/pre-commit` validates the entire bundle before each commit.

## 2026-07-16

**Update**: `docs/okf-standard.md` — added clarifying note after type vocabulary table that `Metrics Reference` is for services exposing numeric metrics and no example exists in this repo.

## 2026-07-16

**Creation**: `docs/troubleshooting.md` — initial creation: common issues and solutions covering empty index, document not found, frontmatter warnings, permission errors, wrong directory, and missing binary.

## 2026-07-15

**Creation**: `docs/architecture.md` — initial creation: internal package structure, design invariants, scoring model.
**Creation**: `docs/configuration.md` — initial creation: MCP host registration, opencode and Claude Desktop examples, permission strings.
**Creation**: `docs/okf-standard.md` — initial creation: OKF frontmatter schema, type vocabulary, skip rules, authoring conventions.
**Creation**: `docs/deployment.md` — initial creation: build, install, run, test, and release procedures.
**Creation**: `docs/tools.md` — initial creation: complete reference for list_tags, list_docs, and get_doc.
