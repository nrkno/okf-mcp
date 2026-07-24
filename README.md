# okf-mcp

An MCP server that makes OKF-conformant documentation queryable by agents.

## Overview

`okf-mcp` runs as a stdio MCP server alongside your existing MCP host. On every tool call it scans the working directory recursively, builds an in-memory index from the YAML frontmatter of every conformant markdown file it finds, and serves six tools (list_tags, list_docs, get_doc, validate_doc, get_index, get_log) so agents can look up docs without traversing the file tree themselves.

The index is rebuilt on each call, so newly added or updated files are always reflected. No config file, no database, no file watcher — just the files in the repo and their frontmatter.

**Frontmatter quality is a functional requirement.** A missing or vague `description` means the wrong document gets returned, or none at all. Treat `title`, `description`, and `tags` as part of the feature, not optional metadata.

## Installation

Build from source:

```sh
go build ./cmd/okf-mcp
```

Or install directly:

```sh
go install github.com/nrkno/plattform-okf-mcp/cmd/okf-mcp@latest
```

## Pre-commit hook

A git pre-commit hook is included in `.githooks/pre-commit`. It validates all OKF docs on every commit, catching frontmatter and structure errors before they land.

Install the hook:

```sh
git config core.hooksPath .githooks
```

The hook requires `okf-mcp` to be on `PATH` (see [Installation](#installation) above). If `okf-mcp` is not found, the commit is blocked with an installation hint.

## Usage

`okf-mcp` must be run from the repo root — the working directory is the scan root. It speaks stdio MCP (JSON-RPC over stdin/stdout).

Add it to your MCP host config. For Claude Desktop (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "okf-mcp": {
      "command": "okf-mcp",
      "args": []
    }
  }
}
```

On startup, `okf-mcp` prints to stderr:

```
okf-mcp: serving /path/to/repo
```

That line confirms which directory is being scanned. If you see the wrong path, adjust the working directory in your host config.

## Permissions

MCP hosts require an explicit allow-list of tool calls before an agent can invoke them. The permission string format depends on the host.

### opencode

In opencode, tool permissions follow the pattern `mcp__<server-key>__<tool-name>`, where the server key matches the key you used in the `mcp` block of `opencode.json`. Using the server key `okf-mcp` (as shown in the Usage section above), the six permission strings are:

```
mcp__okf-mcp__list_tags
mcp__okf-mcp__list_docs
mcp__okf-mcp__get_doc
mcp__okf-mcp__validate_doc
mcp__okf-mcp__get_index
mcp__okf-mcp__get_log
```

A complete `opencode.json` snippet that wires the server registration and the permission allow-list together:

```json
{
  "mcp": {
    "okf-mcp": {
      "type": "local",
      "command": "okf-mcp",
      "args": []
    }
  },
  "permissions": {
    "allow": [
      "mcp__okf-mcp__list_tags",
      "mcp__okf-mcp__list_docs",
      "mcp__okf-mcp__get_doc",
      "mcp__okf-mcp__validate_doc",
      "mcp__okf-mcp__get_index",
      "mcp__okf-mcp__get_log"
    ]
  }
}
```

#### Auto-registration via instructions

`okf-mcp` uses the MCP `instructions` field to inject usage guidance directly into the agent's system prompt on every session start. opencode reads this field from the server's `initialize` response and includes it automatically — no explicit agent instruction or AGENTS.md entry is needed. The agent will know to call `list_tags` first and `get_doc` to retrieve content, without any further configuration beyond the server registration and permissions above.

### Other hosts

Claude Desktop (covered in the Usage section above) does not use a separate allow-list — registering the server is sufficient. Other MCP hosts (Cursor, Zed, VS Code extensions, etc.) each have their own permission model; consult their documentation for the correct format.

## OKF Frontmatter Requirements

Files are indexed only when they have valid YAML frontmatter with a `type` field.

### Required fields

| Field | Required | Notes |
|-------|----------|-------|
| `type` | **yes** | Files without this field are silently skipped |
| `title` | no | Strongly recommended — used for scoring in `get_doc` |
| `description` | no | Strongly recommended — used for scoring in `get_doc` |
| `tags` | no | Strongly recommended — enables tag-filtered search |

### Example frontmatter block

```yaml
---
type: Playbook
title: Authentication Setup
description: How to configure OAuth2 for internal services
tags:
  - auth
  - setup
  - go
---
```

### Skip rules

Files are skipped when any of the following apply:

- The file does not begin with `---` YAML frontmatter
- The frontmatter is missing the `type` field
- The filename is `index.md` or `log.md` (OKF reserved files — always skipped)
- The file is inside a hidden directory (names starting with `.` — skipped by default; pass `--enable-hidden` to traverse, except VCS internals `.git`, `.hg`, `.svn` which are always skipped)

When `title` or `description` is missing, a warning is written to stderr but the file is still indexed:

```
okf-mcp: WARN: docs/auth.md: missing title
okf-mcp: WARN: docs/auth.md: missing description
```

## CLI flags

`okf-mcp` has no config file, no env vars, no remote settings. The full configuration surface is the three flags below. Pass them on the command line.

```
okf-mcp [flags]

Flags:
  -validate         Validate document conformance and exit (no MCP server)
  -path string      Path to validate (relative to cwd) (default ".")
  -enable-hidden    Traverse hidden directories (except .git, .hg, .svn)
```

### `--validate`

Runs OKF conformance validation against the bundle (or against a subdirectory if `--path` is set) and exits. Does not start the MCP server. Exit codes:

- `0` — conformant (no errors).
- `1` — at least one error-severity finding.
- `2` — infrastructure failure (cannot read files, invalid path).

This is the same check the pre-commit hook runs; useful for CI and for validating a single sub-bundle in isolation:

```sh
okf-mcp --validate                # validate the whole tree
okf-mcp --validate --path docs    # validate only the docs/ bundle
```

### `--path string`

Relative path of the directory to validate. Defaults to `.` (the cwd). Composes with `--validate`:

```sh
okf-mcp --validate --path docs
```

When running as an MCP server, the working directory itself is the scan root — `--path` only affects `--validate`.

### `--enable-hidden`

The scanner skips hidden directories by default — any directory whose name starts with `.` (e.g. `.git`, `.opencode`) is invisible to the index. `--enable-hidden` opts in to traversing those directories. The flag composes with both modes:

```sh
# Serve a multi-bundle repo where one bundle lives in .opencode/architecture/
okf-mcp --enable-hidden

# Validate all bundles, including hidden ones
okf-mcp --validate --enable-hidden
```

**VCS internals are always skipped.** The flag is opt-in for general hidden directories, but `.git`, `.hg`, and `.svn` are always skipped regardless of the flag. This is a structural safety guard, not a policy knob — even with `--enable-hidden`, no VCS content is ever indexed. See [Multi-bundle support](#multi-bundle-support) below for the rationale.

**Default is off.** When `--enable-hidden` is not set, the scanner behavior is byte-identical to pre-flag behavior — every dot-dir is skipped, including the canonical `.opencode/architecture/` bundle shipped with this repo's own docs tooling.

## Tools

### `list_tags`

Returns a sorted JSON array of all unique tags across every indexed document.

No parameters.

**Example response:**

```json
["api", "go", "observability", "setup"]
```

---

### `list_docs`

Returns a JSON array of every indexed document with its metadata. File content is not included.

No parameters.

**Example response:**

```json
[
  {
    "title": "User Guide",
    "description": "End-to-end walkthrough for new users",
    "tags": ["api", "setup"],
    "file_path": "docs/guide.md",
    "bundle": "docs"
  }
]
```

---

### `get_doc`

Finds the best-matching document for a topic query and returns its full content and metadata.

**Parameters:**

| Param | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `topic` | string | yes | — | Topic or title to search for |
| `tags` | string[] | no | — | Optional tag filter |
| `match` | string | no | `"and"` | Tag filter mode: `"and"` (all tags must match) or `"or"` (any tag matches) |

**Scoring:** the query is tokenised and matched against each document. Title matches contribute `3×` per token, tag matches `2×`, description matches `1×`. The single highest-scoring document is returned. Ties are broken alphabetically by `file_path`.

**Example response:**

```json
{
  "content": "# User Guide\n...",
  "file_path": "docs/guide.md",
  "tags": ["api", "setup"],
  "title": "User Guide",
  "description": "End-to-end walkthrough for new users",
  "bundle": "docs"
}
```

**Error cases:**

| Situation | Error message |
|-----------|---------------|
| No conformant docs found in cwd | `index is empty: no OKF-conformant markdown docs found in cwd` |
| Index has docs but none match topic/tags | `no document matched topic "<topic>" with tags [...]` |
| Invalid `match` value | `invalid match value "<value>": must be "and" or "or"` |

---

### `validate_doc`

Validates OKF-conformant documents against the frontmatter schema and reports errors, warnings, and notifications. Can validate a single file or the entire bundle.

**Parameters:**

| Param | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `file_path` | string | no | — | Relative path of a single file to validate. If omitted, validates the entire bundle. |
| `known_types` | string[] | no | — | List of known OKF type values (used to flag unknown types as W3). |

**Response fields:**

| Field | Type | Description |
|-------|------|-------------|
| `summary.files` | int | Total files checked |
| `summary.errors` | int | Count of error-severity findings |
| `summary.warnings` | int | Count of warning-severity findings |
| `summary.notifications` | int | Count of notification-severity findings |
| `findings` | array | Validation findings, each with `code`, `severity`, `file`, `line`, `message` |

**Example response:**

```json
{
  "summary": {"files": 6, "errors": 0, "warnings": 1, "notifications": 0},
  "findings": [
    {
      "code": "W1",
      "severity": "warning",
      "file": "docs/architecture.md",
      "line": 0,
      "message": "missing title"
    }
  ]
}
```

A conformant bundle returns `summary.errors == 0`. The same check is available non-interactively as `okf-mcp --validate` (see [CLI flags](#cli-flags)).

---

### `get_index`

Returns the bundle tree showing all documents and their directory structure. Useful for seeing what files exist without fetching their content.

**Parameters:**

| Param | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `path` | string | no | — | Relative path to a subtree root. If omitted, returns the full tree. |

**Tree node fields:**

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Filename or directory name |
| `path` | string | Relative path from scan root |
| `type` | string | `"file"`, `"directory"`, or `"reserved"` |
| `doc_type` | string | From frontmatter (e.g. `"Architecture"`); omitted for directories |
| `title` | string | From frontmatter; omitted for directories |
| `bundle` | string | **Leaf-only** — present on `file` and `reserved` nodes, omitted on `directory` nodes |
| `children` | array | Child nodes (only present for directories) |

**Example response:**

```json
{
  "name": ".",
  "path": "",
  "type": "directory",
  "children": [
    { "name": "index.md", "path": "index.md", "type": "reserved", "bundle": "." },
    { "name": "docs", "path": "docs", "type": "directory", "children": [
      { "name": "architecture.md", "path": "docs/architecture.md", "type": "file", "doc_type": "Architecture", "title": "Architecture", "bundle": "docs" }
    ]}
  ]
}
```

---

### `get_log`

Returns structured log entries from the documentation change log (`log.md`). Entries are parsed from the markdown body and returned in reverse-chronological order (newest first).

In a multi-bundle repository, `get_log` aggregates entries from **all** `log.md` files (one per bundle). Each entry is tagged with the relative path of the `log.md` it came from. Hidden-dir `log.md` files (e.g. `.opencode/architecture/log.md`) are aggregated only when the server was started with `--enable-hidden`; otherwise the visible `log.md` (typically `docs/log.md`) is the only source.

**Parameters:**

| Param | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `since` | string | no | — | Only return entries on or after this date (`YYYY-MM-DD`) |
| `action` | string | no | — | Filter by action type (e.g. `"Creation"`, `"Update"`) |
| `limit` | number | no | all | Maximum number of entries to return |

**Entry fields:**

| Field | Type | Description |
|-------|------|-------------|
| `date` | string | Date of the heading (`YYYY-MM-DD`) |
| `action` | string | Action type (e.g. `"Creation"`, `"Update"`) |
| `target` | string | Target file path |
| `detail` | string | Full description text |
| `source` | string | Relative path of the `log.md` the entry came from (e.g. `"docs/log.md"` or `".opencode/architecture/log.md"`) |

**Sort order:** primary key is `date` descending (newest first); ties are broken by `source` ascending, then by document order in the source `log.md`. The top-level `source` field is intentionally absent — with multiple sources, a single top-level value is ambiguous; the per-entry `source` is the source of truth.

**Example response:**

```json
{
  "entries": [
    {
      "date": "2026-07-23",
      "action": "Update",
      "target": "docs/tools.md",
      "detail": "Document get_log and get_index response shapes.",
      "source": "docs/log.md"
    }
  ]
}
```

When no `log.md` is found in any bundle, the response is `{"entries": [], "note": "no log.md found"}`. A malformed `log.md` is reported via `"note": "log.md has malformed entries"` while the successfully parsed entries are still returned.

## Multi-bundle support

A repository can contain more than one OKF bundle. A **bundle** is a directory that owns a self-contained documentation corpus: it has its own `index.md`, its own `log.md`, and any number of frontmatter-bearing `.md` files. A repo with a canonical `docs/` bundle and a `.opencode/architecture/` bundle ships two independent bundles side-by-side.

`okf-mcp` treats each bundle as a first-class peer. Three coordinated capabilities make this work.

### The `bundle` field

Every document response (`list_docs`, `get_doc`, and leaf nodes of `get_index`) carries a `bundle` field. It is the relative path of the nearest ancestor directory that contains an `index.md`, or the file's immediate parent directory if no ancestor has one. The same field is set on reserved files (`index.md`, `log.md`) so you can tell which bundle they belong to. `"."` is used for the root bundle.

| File | Bundle | Why |
|------|--------|-----|
| `docs/architecture.md` | `docs` | `docs/index.md` exists |
| `docs/sub/deep.md` | `docs` | `docs/index.md` is the closest ancestor |
| `docs/log.md` | `docs` | reserved file; bundle is the owning dir |
| `guide.md` | `.` | `index.md` lives at the root |
| `random/notes.md` | `random` | no ancestor has `index.md`; parent is the fallback |
| `.opencode/architecture/design.md` | `.opencode/architecture` | its own bundle (only visible with `--enable-hidden`) |

### `--enable-hidden` for hidden bundles

Bundles placed inside a hidden directory (one whose name starts with `.`) are invisible to the scanner by default. `.opencode/architecture/` is the canonical example — it is the doc bundle shipped with this repo's own `opencode` tooling, and the scanner will not see it unless you opt in:

```sh
okf-mcp --enable-hidden
```

Once opted in, the hidden bundle's docs, its `index.md`, and its `log.md` are all indexed and aggregated alongside the visible bundle. The VCS internals (`.git`, `.hg`, `.svn`) are always skipped, with or without the flag — see [CLI flags](#cli-flags) for the rationale.

### Multi-`log.md` aggregation

A multi-bundle repo has multiple `log.md` files. `get_log` aggregates all of them in a single response, tagging each entry with the relative path of its `log.md`. Entries are sorted by `date` descending; ties are broken by `source` ascending (so `.opencode/...` sorts before `docs/...`), then by document order within the source. The filters (`since`, `action`, `limit`) apply after merging. See [`get_log`](#get_log) above for the full response shape.

A visible-only setup (no `--enable-hidden`) returns just the entries from the visible `log.md` — the hidden bundle is invisible to the rest of the server, not just to `get_log`.

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/okf-mcp
```

```sh
# Static binary (recommended for containers and portable distribution):
CGO_ENABLED=0 go build ./cmd/okf-mcp
```
