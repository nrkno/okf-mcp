---
type: Architecture
title: Architecture
description: Internal structure of okf-mcp — packages, design invariants, and the weighted scoring model used by get_doc.
tags: [architecture, scanner, parser, index, matcher, validator, logparser, mcp, scoring]
timestamp: 2026-07-19T00:00:00Z
---

# Architecture

## What okf-mcp does

`okf-mcp` is a stdio MCP server that makes OKF-conformant documentation queryable by agents. On every tool call it scans the process working directory recursively, builds an in-memory index from the YAML frontmatter of every conformant markdown file it finds, and serves six tools — `list_tags`, `list_docs`, `get_doc`, `validate_doc`, `get_index`, `get_log` — so agents can look up platform documentation without traversing the file tree themselves. It also provides a `--validate` CLI flag and a pre-commit hook for validating doc conformance.

## Internal packages

The server is structured as six internal packages under `internal/`, wired together in `cmd/okf-mcp/main.go`.

### `internal/scanner`

Walks a directory tree using `filepath.WalkDir` and returns the absolute paths of all candidate `.md` files. Applies skip rules before returning any path:

1. Directories whose name starts with `.` are skipped entirely (the whole subtree is pruned).
2. Files named exactly `index.md` or `log.md` are collected as reserved files, not indexable docs (I-4, I-8).
3. Files whose extension is not `.md` are skipped.

Two entry points: `ScanAll(dir)` returns both indexable and reserved file paths; `Scan(dir)` returns only indexable files (backward-compatible wrapper).

### `internal/parser`

Reads a single `.md` file, extracts the YAML frontmatter block delimited by `---`, unmarshals it, and returns a `Doc` struct. The parser:

- Returns `(Doc{}, false, nil)` — silently skips — if the file has no `---` prefix or is missing the required `type` field.
- Writes a warning to stderr and still returns `(doc, true, nil)` if `title` or `description` is absent.
- Records `BodyOffset`: the byte offset of the first character after the closing `---\n` delimiter, so `get_doc` can strip frontmatter without re-parsing.
- Sets `doc.FilePath` from the path argument, not from frontmatter content.

The `Doc` struct fields: `Title`, `Description`, `Type`, `Tags`, `FilePath`, `BodyOffset`.

The `DetectFrontmatter(content)` function is the single source of truth for frontmatter detection (I-15) — used by both `Parse` and the validator.

### `internal/index`

Owns the in-memory doc slice and exposes operations for the MCP tools:

- `New(dir string) *Index` — creates an empty index rooted at `dir`.
- `Rebuild() error` — calls `scanner.ScanAll`, then parser on each file, relativises paths to the scan root, and atomically replaces the internal doc and reserved slices under a `sync.Mutex`.
- `Docs() []parser.Doc` — returns a defensive copy of the current slice (safe for callers to mutate).
- `Tags() []string` — returns a sorted, deduplicated list of all tags across all docs.
- `Reserved() []ReservedFile` — returns reserved file metadata (index.md, log.md) from the last Rebuild (I-8). Reserved files never appear in `Docs()` (I-4).
- `Tree() TreeNode` — returns the bundle tree built from docs and reserved files (I-11).

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
- Returns entries in document order; the `get_log` handler sorts them reverse-chronologically.
- Unparseable lines are skipped; entries with missing fields get empty strings.

## Design invariants

The sixteen invariants are the correctness contracts the implementation upholds and the integration tests verify:

| ID | Invariant |
|----|-----------|
| I-1 | `file_path` in every tool response is a **relative** path (relative to the scan-root cwd), never an absolute path. |
| I-2 | `get_doc` and `get_log` return content that is **live-read from disk** (`os.ReadFile` in handlers, not cached in `Rebuild`). |
| I-3 | A file missing the `type` frontmatter field is **silently skipped** — never indexed, never surfaced in any tool response. |
| I-4 | The filenames `index.md` and `log.md` are **never indexed** regardless of their contents. They are surfaced via `Reserved()` (I-8). |
| I-5 | Any directory whose name starts with `.` (e.g. `.git`, `.opencode`) is **never traversed** — its contents are invisible to the index. |
| I-6 | When two docs have equal scores, the one with the **lexicographically smaller `FilePath`** wins. Results are therefore deterministic across runs. |
| I-7 | **Zero conformant docs** is not a fatal error. `Rebuild` succeeds; `get_doc` returns an `IsError: true` response with message prefix `"index is empty: ..."`. |
| I-8 | `index.md` and `log.md` are surfaced by the scanner as reserved files — they appear in `Index.Reserved()` but never in `Index.Docs()`. |
| I-9 | `validate_doc` returns zero errors for a conformant bundle. |
| I-10 | `validate_doc` returns at least one error for a file with frontmatter but no `type` field. |
| I-11 | `get_index` returns a tree whose leaves are indexed `.md` files and reserved files. |
| I-12 | `get_log` returns entries in reverse-chronological order with parsed date, action, and target fields. |
| I-13 | `--validate` exits 0 on conformant, 1 on errors, 2 on infrastructure failure; does not start the MCP server. |
| I-14 | Pre-commit hook invokes `okf-mcp --validate` and blocks the commit on exit 1. |
| I-15 | `parser.DetectFrontmatter` is the single source of truth for frontmatter detection — used by both `Parse` and the validator. |
| I-16 | `ValidateReserved` applies only E3; `ValidateDoc` applies only E0/E1/E2/W1–W4/N1. |

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

`main.go` registers the MCP server with a `WithInstructions(...)` option. The mcp-go library includes this string in the `initialize` response as the `instructions` field. MCP hosts that support this field (opencode does) inject it into the agent system prompt automatically on session start. This means an agent connected to `okf-mcp` will know to call `list_tags` first, `get_doc` to retrieve content, `validate_doc` to check conformance, `get_index` to browse the bundle tree, and `get_log` to access change log entries — without any explicit AGENTS.md entry or additional configuration.

## Why scan-on-every-call, not a file watcher

The index is rebuilt on every tool call rather than watching for file changes. This avoids stale state: a file watcher can miss events, have race windows during writes, and adds complexity (background goroutines, signal handling, OS-specific APIs). For a docs corpus of 50–500 files the directory walk completes in milliseconds, making the simplicity cost-free.
