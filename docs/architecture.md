---
type: Architecture
title: Architecture
description: Internal structure of okf-mcp — packages, design invariants, the weighted scoring model used by get_doc, and the multi-bundle support (--enable-hidden, bundle field, multi-log aggregation).
tags: [architecture, scanner, parser, index, matcher, validator, logparser, mcp, scoring, multi-bundle, hidden]
timestamp: 2026-07-23T00:00:00Z
---

# Architecture

## What okf-mcp does

`okf-mcp` is a stdio MCP server that makes OKF-conformant documentation queryable by agents. On every tool call it scans the process working directory recursively, builds an in-memory index from the YAML frontmatter of every conformant markdown file it finds, and serves six tools — `list_tags`, `list_docs`, `get_doc`, `validate_doc`, `get_index`, `get_log` — so agents can look up platform documentation without traversing the file tree themselves. It also provides a `--validate` CLI flag and a pre-commit hook for validating doc conformance.

## Internal packages

The server is structured as six internal packages under `internal/`, wired together in `cmd/okf-mcp/main.go`.

### `internal/scanner`

Walks a directory tree using `filepath.WalkDir` and returns the absolute paths of all candidate `.md` files. Applies skip rules before returning any path:

1. VCS-internal directories (`.git`, `.hg`, `.svn`) are **always** skipped (I-19). Other directories whose name starts with `.` are skipped unless `ScanOptions.EnableHidden` is true (I-5, I-18). When the flag is off, behavior is byte-identical to pre-flag behavior.
2. Files named exactly `index.md` or `log.md` are collected as reserved files, not indexable docs (I-4, I-8).
3. Files whose extension is not `.md` are skipped.

`ScanOptions` is a one-field struct controlling the optional hidden-dir traversal:

```go
type ScanOptions struct {
    EnableHidden bool // traverse hidden directories (except VCS internals)
}
```

The VCS always-skip list is hard-coded in the scanner — it is a structural safety guard, not a policy knob users can override.

Two entry points: `ScanAll(dir, opts)` returns both indexable and reserved file paths; `Scan(dir)` returns only indexable files and passes `ScanOptions{}` (flag off — backward-compatible wrapper).

### `internal/parser`

Reads a single `.md` file, extracts the YAML frontmatter block delimited by `---`, unmarshals it, and returns a `Doc` struct. The parser:

- Returns `(Doc{}, false, nil)` — silently skips — if the file has no `---` prefix or is missing the required `type` field.
- Writes a warning to stderr and still returns `(doc, true, nil)` if `title` or `description` is absent.
- Records `BodyOffset`: the byte offset of the first character after the closing `---\n` delimiter, so `get_doc` can strip frontmatter without re-parsing.
- Sets `doc.FilePath` from the path argument, not from frontmatter content.

The `Doc` struct fields: `Title`, `Description`, `Type`, `Tags`, `FilePath`, `BodyOffset`, `Bundle`. The `Bundle` field is set by the index package at `Rebuild` time, not by the parser — it is the relative path to the nearest ancestor directory containing `index.md`, or the file's immediate parent directory if no ancestor has one (I-17).

The `DetectFrontmatter(content)` function is the single source of truth for frontmatter detection (I-15) — used by both `Parse` and the validator.

### `internal/index`

Owns the in-memory doc slice and exposes operations for the MCP tools:

- `New(dir string, opts scanner.ScanOptions) *Index` — creates an empty index rooted at `dir` with the given scan options.
- `Rebuild() error` — calls `scanner.ScanAll` (with the stored `opts`), then parser on each file, relativises paths to the scan root, computes the bundle field for each doc and reserved file (I-17), and atomically replaces the internal doc and reserved slices under a `sync.Mutex`.
- `Docs() []parser.Doc` — returns a defensive copy of the current slice (safe for callers to mutate).
- `Tags() []string` — returns a sorted, deduplicated list of all tags across all docs.
- `Reserved() []ReservedFile` — returns reserved file metadata (index.md, log.md) from the last Rebuild (I-8). Each entry carries a `Bundle` field. Reserved files never appear in `Docs()` (I-4).
- `Tree() TreeNode` — returns the bundle tree built from docs and reserved files (I-11). Leaf nodes (files and reserved) carry a `Bundle` field; directory nodes do not.

Per-file parse errors are logged to stderr and skipped; they do not abort `Rebuild`. Zero conformant docs is logged as a warning but is not an error (invariant I-7).

### `internal/matcher`

Implements the scoring and selection logic for `get_doc`:

- `Score(query, filterTags, matchMode, doc)` — tokenises `query`, applies tag filter, and returns a `float64` score.
- `FindBest(query, filterTags, matchMode, docs)` — iterates all docs, calls `Score`, returns the highest-scoring doc (tie-break: alphabetical by `FilePath`, ascending).

### `internal/validator`

Checks OKF markdown files for frontmatter conformance. Used by both the `validate_doc` MCP tool and the `--validate` CLI flag:

- `ValidateDoc(absPath, knownTypes)` — validates a regular (non-reserved) document: checks E0/E1/E2 plus contextual warnings W1–W4, N1. Uses `parser.DetectFrontmatter` (I-15).
- `ValidateReserved(absPath, relPath)` — validates reserved files (index.md, log.md): checks only E3 with per-filename logic, plus N2/N3 for log.md.
- `ValidateBundle(idx)` — validates all files in the index (dispatches to ValidateDoc and ValidateReserved).
- `ValidatePaths(paths, knownTypes)` — validates specific absolute file paths.

### `internal/logparser`

Parses the body of `log.md` (after frontmatter) into structured `LogEntry` slices:

- `Parse(body)` — scans for `## YYYY-MM-DD` date headings and `**Action**: \`target\` — detail` entry lines.
- Returns entries in document order; the `get_log` handler sorts them reverse-chronologically after merging across all `log.md` files (see [Multi-bundle support](#multi-bundle-support) below).
- Unparseable lines are skipped; entries with missing fields get empty strings.

## Design invariants

The nineteen invariants are the correctness contracts the implementation upholds and the integration tests verify:

| ID | Invariant |
|----|-----------|
| I-1 | `file_path` in every tool response is a **relative** path (relative to the scan-root cwd), never an absolute path. |
| I-2 | `get_doc` and `get_log` return content that is **live-read from disk** (`os.ReadFile` in handlers, not cached in `Rebuild`). |
| I-3 | A file missing the `type` frontmatter field is **silently skipped** — never indexed, never surfaced in any tool response. |
| I-4 | The filenames `index.md` and `log.md` are **never indexed** regardless of their contents. They are surfaced via `Reserved()` (I-8). |
| I-5 | Hidden directories (names starting with `.`) are **skipped by default**; `--enable-hidden` opts in to traversing them, except VCS internals (`.git`, `.hg`, `.svn`) which are always skipped. |
| I-6 | When two docs have equal scores, the one with the **lexicographically smaller `FilePath`** wins. Results are therefore deterministic across runs. |
| I-7 | **Zero conformant docs** is not a fatal error. `Rebuild` succeeds; `get_doc` returns an `IsError: true` response with message prefix `"index is empty: ..."`. |
| I-8 | `index.md` and `log.md` are surfaced by the scanner as reserved files — they appear in `Index.Reserved()` but never in `Index.Docs()`. |
| I-9 | `validate_doc` returns zero errors for a conformant bundle. |
| I-10 | `validate_doc` returns at least one error for a file with frontmatter but no `type` field. |
| I-11 | `get_index` returns a tree whose leaves are indexed `.md` files and reserved files. |
| I-12 | `get_log` returns entries from **all** `log.md` files in the index, merged in reverse-chronological order, each tagged with a `source` field (relative path to its `log.md`); ties are broken by source ascending, then document order. |
| I-13 | `--validate` exits 0 on conformant, 1 on errors, 2 on infrastructure failure; does not start the MCP server. |
| I-14 | Pre-commit hook invokes `okf-mcp --validate` and blocks the commit on exit 1. |
| I-15 | `parser.DetectFrontmatter` is the single source of truth for frontmatter detection — used by both `Parse` and the validator. |
| I-16 | `ValidateReserved` applies only E3; `ValidateDoc` applies only E0/E1/E2/W1–W4/N1. |
| I-17 | Every document response (`list_docs`, `get_doc`, `get_index` leaf) includes a `bundle` field: the relative path to the nearest ancestor directory containing `index.md`, or the file's immediate parent directory if no ancestor has one. |
| I-18 | `--enable-hidden` defaults to off. When off, the scanner behavior is byte-identical to pre-flag behavior (all dot-dirs skipped). |
| I-19 | VCS directories (`.git`, `.hg`, `.svn`) are always skipped regardless of `--enable-hidden`. |

## Scoring model

`get_doc` uses a weighted token-match algorithm. The query string is tokenised (split on non-alphanumeric characters, lowercased, deduplicated). Each token is then tested against every indexed doc:

| Field | Weight per token |
|-------|-----------------|
| `title` | **3×** — substring match, case-insensitive |
| `tags` | **2×** — substring match against any tag, case-insensitive; at most one contribution per token per doc |
| `description` | **1×** — substring match, case-insensitive |

The doc with the highest total score is returned. Scores of zero or below (after tag filtering) are not eligible. Tie-break is alphabetical by `file_path` (I-6).

Tag filtering is applied before scoring: with `match=and` (the default) the doc must carry all `tags` values; with `match=or` at least one must match. Tag comparison is case-insensitive exact match.

## `WithInstructions` auto-registration

`main.go` registers the MCP server with a `WithInstructions(...)` option. The mcp-go library includes this string in the `initialize` response as the `instructions` field. MCP hosts that support this field (opencode does) inject it into the agent system prompt automatically on session start. The instructions frame okf-mcp as the primary way to find documentation, code definitions, architecture design, decision records, and reports — and direct the agent to use the server before reading files directly. They describe each tool: `get_index` to discover the tree and OKF bundles, `list_docs` (each entry tagged with `bundle`), `list_tags` to discover topics, `get_doc(topic, tags?)` for scored document retrieval, `validate_doc` for conformance checking, and `get_log` for change log entries (each tagged with its source `log.md` path). The instructions also note that the server is launched with `--enable-hidden` to include dot-directory bundles like `.opencode/`, while VCS internals (`.git`, `.hg`, `.svn`) are always skipped.

## Why scan-on-every-call, not a file watcher

The index is rebuilt on every tool call rather than watching for file changes. This avoids stale state: a file watcher can miss events, have race windows during writes, and adds complexity (background goroutines, signal handling, OS-specific APIs). For a docs corpus of 50–500 files the directory walk completes in milliseconds, making the simplicity cost-free.

## Multi-bundle support

A repository may contain more than one OKF bundle: the canonical `docs/` bundle in this repo is one bundle, and `.opencode/architecture/` is another. Each bundle is self-contained — it has its own `index.md`, its own `log.md`, its own doc files. Three coordinated capabilities make multi-bundle repos first-class:

### `--enable-hidden` CLI flag

The scanner skips hidden directories by default (I-5, I-18). A repository that places an OKF bundle inside `.opencode/architecture/` would be invisible to the server. The `--enable-hidden` flag opts in to traversing hidden directories. The flag is the only configuration surface — no env vars, no config file.

```
okf-mcp                              # serve — hidden bundles invisible
okf-mcp --enable-hidden              # serve — hidden bundles included
okf-mcp --validate --enable-hidden   # validate — hidden bundles included
okf-mcp --validate                   # validate — hidden bundles invisible
```

The VCS always-skip list (`.git`, `.hg`, `.svn`) is **not** affected by the flag (I-19). Even with `--enable-hidden`, no `.git/` content is ever indexed. This is a structural safety guard, not a policy knob.

### Bundle resolution

Every indexed file and every reserved file has a `Bundle` field (I-17) computed once at `Rebuild` time. The walk-up rule is mechanical:

1. Start from the file's parent directory.
2. Walk up toward the root, checking each ancestor for `<ancestor>/index.md` in the reserved set.
3. The first ancestor with its own `index.md` is the bundle.
4. Fallback: if no ancestor has `index.md`, the bundle is the file's immediate parent directory.

`"."` is used for the root bundle (Go-standard relative-path representation of the current directory, also used as the tree root node name). The walk uses `filepath` package operations throughout, so Windows path separators are handled uniformly.

| File | Reserved `index.md` files | Bundle |
|------|--------------------------|--------|
| `docs/arch.md` | `docs/index.md` | `docs` |
| `docs/sub/deep.md` | `docs/index.md` | `docs` |
| `docs/log.md` | `docs/index.md` | `docs` |
| `guide.md` | `index.md` | `.` |
| `random/notes.md` | (none) | `random` |
| `.opencode/architecture/design.md` | `.opencode/architecture/index.md` | `.opencode/architecture` |

The `bundle` field is included in the `list_docs` and `get_doc` response maps, and as a leaf-only field on `get_index` tree nodes (directory nodes do not carry `bundle`).

### Multi-`log.md` aggregation

A repository with multiple bundles also has multiple `log.md` files. `get_log` aggregates them:

1. Collect every reserved file where the basename is `log.md` (one per bundle).
2. Live-read each `log.md` from disk (I-2), strip frontmatter, parse via `logparser.Parse`.
3. Tag every entry with the source `log.md`'s relative path.
4. Concatenate all entries.
5. Sort: primary key = date descending (newest first), secondary key = source path ascending. `sort.SliceStable` preserves the document order of entries that share both keys (the tertiary tiebreak).
6. Apply the existing filters (`since`, `action`, `limit`).

The top-level `source` field is removed from the response — with multiple sources, a single top-level value is no longer meaningful. The per-entry `source` is the source of truth.

This is a wire-format breaking change for any consumer that previously read the top-level `result.source`. The change is justified: the prior first-wins behavior was a latent bug that silently dropped entries from any non-first `log.md` in a multi-bundle repo.
