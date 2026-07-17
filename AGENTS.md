# AGENTS.md: OKF MCP Server

## 1. Overview

**plattform-okf-mcp** is a standalone Go MCP server that scans an OKF-conformant repository for markdown files, builds an in-memory index from YAML frontmatter, and exposes three tools (`list_tags`, `list_docs`, `get_doc`) so agents can query documentation without traversing files directly.

Single binary — no config file, no database, no HTTP, no CGO. The process's current working directory is the scan root: wherever you launch the binary, that directory tree is what gets indexed.

Entry point: `cmd/okf-mcp/main.go` — wires the three MCP tool handlers as package-level functions and starts a stdio MCP server via `server.ServeStdio`.

---

## 2. Package Structure

| Package            | Role                                                                                                                                       |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------ |
| `internal/scanner` | `Scan(dir)` — walks `*.md` recursively, applies skip rules (hidden dirs, `index.md`, `log.md`, non-`.md` files)                           |
| `internal/parser`  | `Parse(path)` — extracts YAML frontmatter into `Doc` struct; skips files missing `type`; warns to stderr on missing `title`/`description` |
| `internal/index`   | `Index.Rebuild()` — calls scanner+parser, stores relative paths, mutex-guarded for concurrent handlers; `Docs()` + `Tags()` for reads      |
| `internal/matcher` | `Score()` + `FindBest()` — weighted token scoring (title 3×, tags 2×, description 1×); AND/OR tag filter                                  |

Tool handlers (`listTagsHandler`, `listDocsHandler`, `getDocHandler`) live in `cmd/okf-mcp/main.go`. Tool definitions (`listTagsTool`, `listDocsTool`, `getDocTool`) are package-level variables — tests in `cmd/okf-mcp/` share them directly for schema parity.

---

## 3. Design Invariants

Every change must preserve these. When a change would break one, stop and escalate.

- **I-1**: Every `file_path` in tool responses is relative to cwd (enforced in `Index.Rebuild` via `filepath.Rel`)
- **I-2**: `get_doc` content is live-read from disk (`os.ReadFile` in `getDocHandler`, not cached in `Rebuild`)
- **I-3**: Files missing the `type` field are silently skipped — never indexed
- **I-4**: `index.md` and `log.md` are never indexed (scanner basename skip-list)
- **I-5**: Hidden directories (names starting with `.`) are never traversed
- **I-6**: `get_doc` is deterministic — tie-break by alphabetical `file_path` ascending
- **I-7**: Zero-doc startup does not crash — returns empty results, not panic

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

## 7. Key Documentation

These docs are served by the `okf-mcp` server itself when running in this repo — agents can call `get_doc(topic="architecture")` directly rather than opening files.

| Document                | Content                                                               |
| ----------------------- | --------------------------------------------------------------------- |
| `docs/architecture.md`  | Package responsibilities, invariants I-1→I-7, scoring model           |
| `docs/tools.md`         | `list_tags`, `list_docs`, `get_doc` — params, response shapes, errors |
| `docs/configuration.md` | MCP client setup, permission strings, opencode/Claude examples        |
| `docs/okf-standard.md`  | OKF frontmatter schema, type vocabulary, conventions                  |
| `docs/deployment.md`    | Build, install, release binaries                                      |
| `docs/log.md`           | Chronological record of changes to docs/ files                        |
