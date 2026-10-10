---
type: Log
title: Documentation Change Log
description: Chronological record of changes to the docs/ bundle for plattform-okf-mcp.
tags: [changelog, log, okf, multi-bundle]
timestamp: 2026-10-10T00:00:00Z
---

# Directory Update Log

## 2026-10-10

**Update**: `.okf-profile.yaml` — Preserved the main-branch AOS vocabulary; repaired its requirement-source invariant with explicit `relationship: derived-from`, outgoing direction, and unlimited `max: -1`.
**Update**: `docs/okf-profile.md`, `docs/okf-profile-format.md`, `docs/configuration.md`, `docs/index.md`, `README.md` — Reconciled recovered profile support with repository setup guidance; removed inert-template claims, corrected schema defaults/alias semantics, and separated document conformance from graph validation.
**Update**: `go.mod` — Raised the Go minimum from 1.26.6 to 1.26.9 to include the standard-library fixes reported by PR20 security CI, which selects its toolchain from this file.
**Update**: `docs/architecture.md`, `AGENTS.md` — Removed stale no-profile/no-config descriptions from adjacent current-reality guidance.
**Creation**: `cmd/okf-mcp/profile_docs_test.go` — Exercised published YAML/Markdown and the root AOS profile through the real MCP pipe, including inverse queries, missing sources, wrong target types, nearest-heading classification, and separate document validation.

## 2026-09-01

**Creation**: `docs/okf-profile-format.md` — complete schema reference for `.okf-profile.yaml`, including concept types, relationships, invariants, heading classification, worked example, loading order, and validation errors.
**Update**: `README.md` — tool count bumped to thirteen, `--profile` flag documented, permissions updated to all thirteen tools, and new Graph tools + Profile sections added.
**Update**: `docs/index.md` — added link to the OKF Profile Format reference.

## 2026-08-31

**Update**: `cmd/okf-mcp/main.go` — `WithInstructions` graph sentence rewritten as a workflow guide; updated `graph_relationships`, `graph_trace`, `graph_search`, `graph_integrity`, `graph_coverage`, and `graph_context` tool descriptions to emphasize when to use each tool and how to discover relationship types from the active profile.
**Update**: `docs/tools.md`, `docs/configuration.md`, `docs/architecture.md` — synced graph tool descriptions and `WithInstructions` guidance.
**Update**: `internal/profile/profile.go` — empty/nil `allowed_source_types` or `allowed_target_types` on a relationship is now treated as "any type allowed" (wildcard), matching the intuitive semantics for minimal profiles.
**Update**: `internal/graph/graph.go` — the `superseded_deps` integrity check identifies supersession relationships from profile invariants whose `relationship` name contains `"supersede"` (case-insensitive) and whose `direction` is `"outgoing"`. This works with the design's canonical P3 shape (`max: 1`) as well as `max: 0` and any other cardinality, and avoids misclassifying non-supersession relationships that happen to have a bounded outgoing cardinality. If no such invariant is present, the check is skipped to avoid false negatives.
**Update**: `internal/graph/graph_test.go` — added `TestBuildSupersededRels` covering canonical `max: 1`, `max: 0`, case-insensitive match, supersede-variant names, and non-supersession exclusions; updated renamed-relationship supersession test to use a relationship name containing `"supersede"`; added `TestIntegrity_SupersededMaxOne` for the canonical P3 shape.
**Note**: `docs/architecture/design/graph-projection.md` still describes the old hard-coded `"supersedes"` superseded-dependency behavior. Edits to `docs/architecture/**` are not permitted in this session; update when the directory is writable.
**Update**: `AGENTS.md` — recorded invariants I-20 through I-31; added `internal/graph`, `internal/linkextract`, and `internal/profile` to the package table; updated overview and key-documentation summary to reflect thirteen tools.
**Update**: `docs/architecture.md` — documented `internal/graph`, `internal/linkextract`, and `internal/profile`; added the graph-projection data model and profile-loading sections; extended the invariants table to I-1→I-31.
**Update**: `docs/tools.md` — documented `graph_integrity`, `graph_coverage`, and `graph_context`; updated frontmatter and intro to reference all thirteen tools and seven graph tools.
**Update**: `docs/configuration.md` — added permission strings for `graph_integrity`, `graph_coverage`, and `graph_context`; updated auto-registration tool list to all thirteen tools.
**Update**: `docs/index.md` — updated the MCP Tools Reference link to note all thirteen tools and the seven graph tools.
**Note**: `docs/architecture/design/graph-projection.md` states that the `internal/index.New()` signature change affects 9 call sites, but the actual count is 29. Flagged here per scope; the design doc was not edited.
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
