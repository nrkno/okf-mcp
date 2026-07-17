# okf-mcp

An MCP server that makes OKF-conformant documentation queryable by agents.

## Overview

`okf-mcp` runs as a stdio MCP server alongside your existing MCP host. On every tool call it scans the working directory recursively, builds an in-memory index from the YAML frontmatter of every conformant markdown file it finds, and serves three tools so agents can look up docs without traversing the file tree themselves.

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

In opencode, tool permissions follow the pattern `mcp__<server-key>__<tool-name>`, where the server key matches the key you used in the `mcp` block of `opencode.json`. Using the server key `okf-mcp` (as shown in the Usage section above), the three permission strings are:

```
mcp__okf-mcp__list_tags
mcp__okf-mcp__list_docs
mcp__okf-mcp__get_doc
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
      "mcp__okf-mcp__get_doc"
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
- The file is inside a hidden directory (`.git`, `.opencode`, etc. — never traversed)

When `title` or `description` is missing, a warning is written to stderr but the file is still indexed:

```
okf-mcp: WARN: docs/auth.md: missing title
okf-mcp: WARN: docs/auth.md: missing description
```

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
    "file_path": "docs/guide.md"
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
  "description": "End-to-end walkthrough for new users"
}
```

**Error cases:**

| Situation | Error message |
|-----------|---------------|
| No conformant docs found in cwd | `index is empty: no OKF-conformant markdown docs found in cwd` |
| Index has docs but none match topic/tags | `no document matched topic "<topic>" with tags [...]` |
| Invalid `match` value | `invalid match value "<value>": must be "and" or "or"` |

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
