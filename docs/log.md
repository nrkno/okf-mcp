---
type: Log
title: Documentation Change Log
description: Chronological record of changes to the docs/ bundle for plattform-okf-mcp.
tags: [changelog, log, okf, multi-bundle]
timestamp: 2026-08-31T00:00:00Z
---

# Directory Update Log

## 2026-08-31

**Creation**: `graph_concept` MCP tool — returns metadata and edge counts for a single concept by relative `file_path`.
**Creation**: `graph_relationships` MCP tool — returns outgoing and incoming relationships for a concept, with optional `direction` and `type` filters.
**Creation**: `graph_trace` MCP tool — BFS traversal of upstream or downstream relationships from a starting concept, with optional `type` filter and `max_depth` clamped to 1–20.
**Creation**: `graph_search` MCP tool — text search over indexed concepts using the existing weighted-token scorer, with optional `type`/`tags` filters and `limit` clamped to 1–100.
**Update**: `cmd/okf-mcp/main.go` — added `--profile` CLI flag; profile loading follows the order explicit flag → auto-discovered `.okf-profile.yaml` in scan root → default profile. An invalid explicit path exits code 2; an invalid discovered file logs a warning and falls back to default. Profile selection is always logged to stderr.
**Update**: `docs/tools.md` — documented the four new graph navigation tools (`graph_concept`, `graph_relationships`, `graph_trace`, `graph_search`).
**Update**: `docs/configuration.md` — documented the `--profile` flag, profile resolution order, exit-code behaviour, and added the four new graph tool permission strings.

## 2026-07-23

**Update**: `cmd/okf-mcp/main.go` — `WithInstructions` string rewritten to lead with use cases (documentation, code definitions, architecture design, decision records, reports) so any agent that hits a documentation-adjacent question routes to okf-mcp before reading files directly; per-tool guidance updated to reflect 0.4.0 behavior (`get_index` first to discover tree + bundles, `bundle` field on responses, per-entry `source` on `get_log`, `--enable-hidden` for multi-bundle repos, VCS always-skip).
**Update**: `docs/architecture.md` — `WithInstructions` auto-registration section rewritten to match the new instructions language (use-case-led, `get_index` first, bundle-aware, hidden-dir note).
**Update**: `docs/configuration.md` — Auto-registration section rewritten to match the new instructions language (tools reordered, bundle/hidden-dir context added).
**Update**: `cmd/okf-mcp/main.go` — added `--enable-hidden` CLI flag; threaded through `index.New` in both the MCP server path and the `--validate` path so hidden-dir OKF bundles (e.g. `.opencode/architecture/`) become first-class indexable bundles.
**Update**: `cmd/okf-mcp/main.go` — `get_log` response now aggregates entries from all `log.md` files in the index (multi-bundle), each entry tagged with its source; top-level `source` field removed (the prior first-wins behavior was a latent bug that silently dropped non-first `log.md` entries).
**Update**: `cmd/okf-mcp/main.go` — `list_docs`, `get_doc`, and `get_index` response shapes now include a `bundle` field on each doc and on every leaf tree node; the field is the relative path to the nearest ancestor directory containing `index.md` (or the file's immediate parent directory as fallback).
**Update**: `internal/scanner/scanner.go` — added `ScanOptions{EnableHidden}` parameter, VCS always-skip list (`.git`, `.hg`, `.svn`).
**Update**: `internal/index/index.go` — added `Bundle` field to `parser.Doc` and `TreeNode`, added `resolveBundle` function that walks the directory tree to find the nearest ancestor `index.md`; `index.New` now accepts `scanner.ScanOptions`.
**Update**: `AGENTS.md` — invariants I-5 and I-12 restated; I-17, I-18, I-19 added (5 invariants amended/added, 14 preserved).
**Update**: `docs/architecture.md` — added `Multi-bundle support` section (flag, bundle resolution, multi-log aggregation); scanner and index sections reflect `ScanOptions` and the `Bundle` field; invariants table extended to I-1→I-19.
**Update**: `docs/configuration.md` — added `CLI flags` section documenting `--validate`, `--path`, and `--enable-hidden`; VCS always-skip list and the byte-identical-when-off default are explicit.
**Update**: `docs/tools.md` — `list_docs`, `get_doc`, and `get_index` response shapes include `bundle`; `get_log` per-entry `source` documented, top-level `source` removal noted, sort-order tiebreak (date desc → source asc → document order) explicit.
**Update**: `cmd/okf-mcp/main_test.go` — `TestGetLog_SameSourceTiebreak` exercises the tertiary tiebreak (document order for same-date same-source entries); `TestGetLog_Filtered` switched from typed `logparser.LogEntry` to generic `[]map[string]any` so the per-entry `source` is observable through the filter; `TestGetLog_MissingLog` removed (subsumed by `TestGetLog_NoLogMd`); `TestCLI_Validate_HiddenBundle` exercises `--validate --enable-hidden` end-to-end via `exec.Command`.

## 2026-07-19

**Update**: `cmd/okf-mcp/main.go` — `WithInstructions` string now mentions all six tools (added `validate_doc`, `get_index`, `get_log`), not just `list_tags` and `get_doc`.
**Update**: `docs/configuration.md` — auto-registration section lists all six tools in the injected instructions.
**Update**: `docs/architecture.md` — `WithInstructions` auto-registration section mentions all six tools.

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
