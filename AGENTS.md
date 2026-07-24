# AGENTS.md: OKF MCP Server

## 1. Overview

**plattform-okf-mcp** is a standalone Go MCP server that scans an OKF-conformant repository for markdown files, builds an in-memory index from YAML frontmatter, and exposes six tools (`list_tags`, `list_docs`, `get_doc`, `validate_doc`, `get_index`, `get_log`) so agents can query documentation without traversing files directly. It also provides a `--validate` CLI flag and a pre-commit hook for validating doc conformance.

Single binary — no config file, no database, no HTTP, no CGO. The process's current working directory is the scan root: wherever you launch the binary, that directory tree is what gets indexed.

Entry point: `cmd/okf-mcp/main.go` — wires the six MCP tool handlers as package-level functions and starts a stdio MCP server via `server.ServeStdio`.

---

## 2. Package Structure

| Package            | Role                                                                                                                                       |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------ |
| `internal/scanner` | `ScanAll(dir, opts)` — walks `*.md` recursively, applies skip rules (hidden dirs per `ScanOptions.EnableHidden`, non-`.md` files); returns indexable and reserved file paths   |
| `internal/parser`  | `Parse(path)` — extracts YAML frontmatter into `Doc` struct; `DetectFrontmatter` is the single source of truth (I-15)                    |
| `internal/index`   | `Index.Rebuild()` — calls scanner+parser, stores relative paths, computes the `Bundle` field per doc/reserved file (I-17), mutex-guarded; `Docs()` + `Tags()` + `Reserved()` + `Tree()` for reads  |
| `internal/matcher` | `Score()` + `FindBest()` — weighted token scoring (title 3×, tags 2×, description 1×); AND/OR tag filter                                  |
| `internal/validator` | `ValidateDoc()`, `ValidateReserved()`, `ValidateBundle()` — frontmatter conformance checks (E0–E3, W1–W4, N1)                           |
| `internal/logparser` | `Parse(body)` — parses log.md body into structured `LogEntry` slices (date, action, target, detail)                                     |

Tool handlers (`listTagsHandler`, `listDocsHandler`, `getDocHandler`, `validateDocHandler`, `getIndexHandler`, `getLogHandler`) live in `cmd/okf-mcp/main.go`. Tool definitions (`listTagsTool`, `listDocsTool`, `getDocTool`, `validateDocTool`, `getIndexTool`, `getLogTool`) are package-level variables — tests in `cmd/okf-mcp/` share them directly for schema parity.

---

## 3. Design Invariants

Every change must preserve these. When a change would break one, stop and escalate.

- **I-1**: Every `file_path` in tool responses is relative to cwd (enforced in `Index.Rebuild` via `filepath.Rel`)
- **I-2**: `get_doc` and `get_log` content is live-read from disk (`os.ReadFile` in handlers, not cached in `Rebuild`)
- **I-3**: Files missing the `type` field are silently skipped — never indexed
- **I-4**: `index.md` and `log.md` are never indexed (scanner basename skip-list); surfaced via `Reserved()` (I-8)
- **I-5**: Hidden directories (names starting with `.`) are skipped by default; `--enable-hidden` opts in to traversing them, except VCS internals (`.git`, `.hg`, `.svn`) which are always skipped
- **I-6**: `get_doc` is deterministic — tie-break by alphabetical `file_path` ascending
- **I-7**: Zero-doc startup does not crash — returns empty results, not panic
- **I-8**: `index.md` and `log.md` appear in `Reserved()` but never in `Docs()`
- **I-9**: `validate_doc` returns zero errors for a conformant bundle
- **I-10**: `validate_doc` returns at least one error for a file with frontmatter but no `type` field
- **I-11**: `get_index` returns a tree whose leaves are indexed `.md` files and reserved files
- **I-12**: `get_log` returns entries from all `log.md` files in the index, merged in reverse-chronological order, each tagged with `source` (relative path to its log.md); ties broken by source ascending, then document order
- **I-13**: `--validate` exits 0 on conformant, 1 on errors, 2 on infra failure; does not start MCP server
- **I-14**: Pre-commit hook invokes `okf-mcp --validate` and blocks commit on exit 1
- **I-15**: `parser.DetectFrontmatter` is the single source of truth for frontmatter detection
- **I-16**: `ValidateReserved` applies only E3; `ValidateDoc` applies only E0/E1/E2/W1–W4/N1
- **I-17**: Every document response (`list_docs`, `get_doc`, `get_index` leaf) includes a `bundle` field: the relative path to the nearest ancestor directory containing `index.md`, or the file's immediate parent directory if no ancestor has one
- **I-18**: `--enable-hidden` defaults to off. When off, scanner behavior is byte-identical to pre-flag behavior (all dot-dirs skipped)
- **I-19**: VCS directories (`.git`, `.hg`, `.svn`) are always skipped regardless of `--enable-hidden`

---

## 4. Using gopls in This Repo

**gopls is registered as an MCP server in this opencode environment. Use it for Go symbol work — it is faster and more accurate than grep for this codebase.**

Reach for gopls instead of Read/Grep when:

- **Finding where a function or type is defined** → `gopls_go_search` to locate it, then `gopls_go_file_context` for cross-file dependencies
- **Finding all callers of a function** → `gopls_go_symbol_references` (e.g. all callers of `Index.Rebuild`)
- **Checking what a package exports** → `gopls_go_package_api` with the import path
- **Checking for build errors or type diagnostics** → `gopls_go_diagnostics`
- **Renaming a symbol safely across the repo** → `gopls_go_rename_symbol`

Internal package import paths for `gopls_go_package_api`:

```
github.com/nrkno/plattform-okf-mcp/internal/scanner
github.com/nrkno/plattform-okf-mcp/internal/parser
github.com/nrkno/plattform-okf-mcp/internal/index
github.com/nrkno/plattform-okf-mcp/internal/matcher
github.com/nrkno/plattform-okf-mcp/internal/validator
github.com/nrkno/plattform-okf-mcp/internal/logparser
```

Do **not** reach for gopls to read file content, check test output, or run builds — use `Read` and `Bash` for those.

---

## 5. Build, Test & Verify

```bash
# Build
go build ./cmd/okf-mcp
CGO_ENABLED=0 go build ./cmd/okf-mcp   # static binary (production)

# Test — always with race detector
go test -race ./...
go test -race -shuffle=on -count=3 ./cmd/okf-mcp/...   # integration: shuffle for order-independence

# Vet + lint
go vet ./...
golangci-lint run ./...

# Vulnerability check
govulncheck ./...
```

**Pre-commit:** `go test -race ./...` and `go vet ./...` must both be clean before pushing.

The integration test suite in `cmd/okf-mcp/` drives all assertions through the real MCP JSON-RPC pipe using `mcptest` — do not bypass it with direct handler calls. Tests that reassign the global `idx` must NOT call `t.Parallel()`.

---

## 6. OKF Documentation Standard

The full standard is in `docs/okf-standard.md` (queryable via `get_doc(topic="okf standard")`). Key rules for agents modifying docs in this repo:

- Every `docs/*.md` file (except `index.md`) must have YAML frontmatter with a non-empty `type:` field
- `index.md` has **no frontmatter** — required by OKF spec
- Valid `type` values: `Architecture`, `Playbook`, `Configuration`, `API Reference`, `Metrics Reference`, `Log`
- Tags: lowercase, hyphenated strings
- Cross-links: bundle-relative paths (e.g. `/docs/file.md`)

### Update obligation

Any code change that alters documented behavior triggers this section — you do not have to be told separately. If a change affects tool behavior, configuration, or deployment, update the relevant `docs/*.md` in the SAME commit as the code change. When modifying any `docs/` file: (1) update its `timestamp:` field to the current date; (2) add a dated entry to `docs/log.md` (newest first, bold action prefix: `**Update**`, `**Creation**`, `**Migration**`, `**Deprecation**`).

---

## 7. Conventional Commits & Release Triggers

This repo uses **semantic-release** (`@semantic-release/commit-analyzer` with the `conventionalcommits` preset) to automate releases on push to `main`. The `Conventional Commits` prefix in each merged commit message determines whether a release fires, and at what level. Configuration lives in `.releaserc.json`.

| Prefix | Release level | Version bump (from 0.4.0) |
|--------|---------------|---------------------------|
| `feat!:` or `BREAKING CHANGE:` footer | Major | 0.4.0 → 1.0.0 |
| `feat:` | Minor | 0.4.0 → 0.5.0 |
| `fix:` | Patch | 0.4.0 → 0.4.1 |
| `perf:` | Patch | 0.4.0 → 0.4.1 |
| `chore:`, `docs:`, `style:`, `refactor:`, `test:`, `ci:`, `build:`, `revert:` | **No release** | (no version change) |

> **A `chore:`, `docs:`, `refactor:`, `test:`, or other non-triggering prefix WILL merge cleanly but will NOT produce a release.** This is the gotcha that just bit us.

**Worked example (this session):** PR #13 used `chore(instructions):` and merged without producing a patch release. PR #14 used `fix(instructions):` to land the same content with a patch-release trigger. The content was identical — only the commit prefix differed. `chore:` is invisible to the release pipeline.

**Pre-commit self-check:** Before pushing, read the commit message and ask: *"Does the prefix match the release impact I want?"* Default to `fix:` for any user-facing behavior change, even small ones.

The canonical mapping is in the `conventionalcommits` preset of `@semantic-release/commit-analyzer`. To verify the current rules, check `.releaserc.json` and the workflow in `.github/workflows/create-release.yaml`.

---

## 8. Key Documentation

These docs are served by the `okf-mcp` server itself when running in this repo — agents can call `get_doc(topic="architecture")` directly rather than opening files.

| Document                | Content                                                               |
| ----------------------- | --------------------------------------------------------------------- |
| `docs/architecture.md`  | Package responsibilities, invariants I-1→I-19, scoring model          |
| `docs/tools.md`         | `list_tags`, `list_docs`, `get_doc`, `validate_doc`, `get_index`, `get_log` — params, response shapes, errors |
| `docs/configuration.md` | MCP client setup, permission strings for all six tools, opencode/Claude examples |
| `docs/okf-standard.md`  | OKF frontmatter schema, type vocabulary, conventions                  |
| `docs/deployment.md`    | Build, install, release binaries, `--validate` CLI, pre-commit hook   |
| `docs/log.md`           | Chronological record of changes to docs/ files                        |
