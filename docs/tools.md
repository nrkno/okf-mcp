---
type: API Reference
title: MCP Tools Reference
description: Complete reference for the six MCP tools exposed by okf-mcp — list_tags, list_docs, get_doc, validate_doc, get_index, and get_log — including parameters, response shapes, scoring, and error codes.
tags: [api, tools, list-tags, list-docs, get-doc, validate-doc, get-index, get-log, mcp, scoring, match]
timestamp: 2026-07-18T00:00:00Z
---

# MCP Tools Reference

`okf-mcp` exposes six tools over the MCP stdio protocol. All tools rebuild the index on every call — freshly created or edited files are always reflected without restarting the server.

## `list_tags`

Returns a sorted JSON array of all unique tags across every indexed document.

**Parameters:** none.

**Returns:** a JSON string containing a sorted array of tag strings.

**Example response:**

```json
["api", "architecture", "deployment", "mcp", "okf"]
```

**Use case:** call `list_tags` first at the start of a session to discover the vocabulary available in this repository. Use the returned tags to formulate a `get_doc` query.

---

## `list_docs`

Returns a JSON array of every indexed document with its metadata. File content is not included.

**Parameters:** none.

**Returns:** a JSON array of objects, one per indexed document.

**Response object fields:**

| Field | Type | Description |
|-------|------|-------------|
| `title` | string | Document title from frontmatter |
| `description` | string | Document description from frontmatter |
| `tags` | string[] | Document tags from frontmatter |
| `file_path` | string | Relative path from the scan root |

**Example response:**

```json
[
  {
    "title": "Architecture",
    "description": "Internal structure of okf-mcp ...",
    "tags": ["architecture", "scanner", "parser"],
    "file_path": "docs/architecture.md"
  }
]
```

**Use case:** catalogue — see what documents exist and their metadata without fetching full content.

---

## `get_doc`

Finds the best-matching document for a topic query and returns its full content (frontmatter stripped) plus metadata.

### Parameters

| Param | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `topic` | string | **yes** | — | Topic or title to search for |
| `tags` | string[] | no | — | Tag filter — must be a JSON array, not a plain string |
| `match` | string | no | `"and"` | `"and"` = all tags must match; `"or"` = any tag matches |

### Response fields

| Field | Type | Description |
|-------|------|-------------|
| `content` | string | Markdown body of the document (frontmatter stripped) |
| `file_path` | string | Relative path from the scan root |
| `tags` | string[] | Document tags from frontmatter |
| `title` | string | Document title from frontmatter |
| `description` | string | Document description from frontmatter |

### Scoring

The `topic` string is tokenised (split on non-alphanumeric characters, lowercased, duplicates removed). Each token contributes to a document's score:

| Field matched | Score per token |
|---------------|----------------|
| `title` | **3×** |
| `tags` | **2×** (at most once per token per doc) |
| `description` | **1×** |

The single highest-scoring document is returned. Documents scoring zero or below are not eligible. Ties are broken alphabetically by `file_path` (ascending), making results deterministic.

Tag filtering (when `tags` is provided) is applied before scoring: with `match=and` (default) the document must carry all specified tags; with `match=or` at least one tag must match. Tag comparison is case-insensitive exact match.

### Example response

```json
{
  "content": "# Architecture\n\n## What okf-mcp does\n...",
  "file_path": "docs/architecture.md",
  "tags": ["architecture", "scanner", "parser", "index", "matcher", "mcp", "scoring"],
  "title": "Architecture",
  "description": "Internal structure of okf-mcp ..."
}
```

### Error responses

When `get_doc` cannot satisfy the request, it returns a tool result with `IsError: true`. The error message is a plain string:

| Situation | Error message |
|-----------|---------------|
| No OKF-conformant docs found in cwd | `index is empty: no OKF-conformant markdown docs found in cwd` |
| Docs exist but none matched topic/tags | `no document matched topic "<topic>" with tags [<tags>]` |
| `match` param is not `"and"` or `"or"` | `invalid match value "<value>": must be "and" or "or"` |
| `tags` sent as a scalar string | `"tags" must be an array of strings, not a scalar value` |

### Usage pattern

```
1. list_tags                          → discover vocabulary
2. get_doc(topic="deployment")        → retrieve a specific doc
3. get_doc(topic="build", tags=["go"], match="and")  → filtered retrieval
```

---

## `validate_doc`

Validates OKF-conformant documents against the frontmatter schema and reports errors, warnings, and notifications. Can validate a single file or the entire bundle.

### Parameters

| Param | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `file_path` | string | no | — | Relative path of a single file to validate. If omitted, validates the entire bundle. |
| `known_types` | string[] | no | — | List of known OKF type values for W3 (unknown type) warnings. |

### Response fields

| Field | Type | Description |
|-------|------|-------------|
| `summary.files` | int | Total files checked |
| `summary.errors` | int | Count of error-severity findings |
| `summary.warnings` | int | Count of warning-severity findings |
| `summary.notifications` | int | Count of notification-severity findings |
| `findings` | array | Validation findings (see below) |

Each finding object:

| Field | Type | Description |
|-------|------|-------------|
| `code` | string | Check code (E0–E3, W1–W4, N1–N3) |
| `severity` | string | `"error"`, `"warning"`, or `"notification"` |
| `file` | string | Path of the file with the finding |
| `line` | int | Line number (0 if not line-specific) |
| `message` | string | Human-readable description |

### Validation codes

| Code | Severity | Check | Description |
|------|----------|-------|-------------|
| E0 | Error | Read failure | File could not be read from disk |
| E1 | Error | Frontmatter present | File missing `---` delimiters with YAML content |
| E2 | Error | Type field non-empty | Frontmatter contains empty or missing `type` field |
| E3 | Error | Reserved-file structure | index.md has NO frontmatter; log.md must have `type: Log` |
| W1 | Warning | Title present | Missing `title` in frontmatter |
| W2 | Warning | Description present | Missing `description` in frontmatter |
| W3 | Warning | Type in vocabulary | `type` is not in the known vocabulary list |
| W4 | Warning | Tags present | No tags in frontmatter |
| N1 | Notification | Single-tag collection | Only one tag (collections typically have multiple) |

### Usage pattern

```
1. validate_doc                                   → validate entire bundle
2. validate_doc(file_path="docs/new-doc.md")      → validate a single file
```

---

## `get_index`

Returns the bundle tree showing all documents and their directory structure. Useful for seeing what files exist without fetching their content.

### Parameters

| Param | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `path` | string | no | — | Relative path to a subtree root. If omitted, returns the full tree. |

### Response — `TreeNode`

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Filename or directory name |
| `path` | string | Relative path from scan root |
| `type` | string | `"file"`, `"directory"`, or `"reserved"` |
| `doc_type` | string | From frontmatter (e.g. `"Architecture"`), omitted for directories |
| `title` | string | From frontmatter, omitted for directories |
| `children` | TreeNode[] | Child nodes (only present for directories) |

### Example response

```json
{
  "name": ".",
  "path": "",
  "type": "directory",
  "children": [
    { "name": "index.md", "path": "index.md", "type": "reserved" },
    { "name": "docs", "path": "docs", "type": "directory", "children": [
      { "name": "architecture.md", "path": "docs/architecture.md", "type": "file", "doc_type": "Architecture", "title": "Architecture" },
      { "name": "tools.md", "path": "docs/tools.md", "type": "file", "doc_type": "API Reference", "title": "MCP Tools Reference" }
    ]}
  ]
}
```

### Usage pattern

```
1. get_index                   → full bundle tree
2. get_index(path="docs")     → subtree rooted at docs/
```

---

## `get_log`

Returns structured log entries from the documentation change log (`log.md`). Entries are parsed from the markdown body and returned in reverse-chronological order (newest first).

### Parameters

| Param | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `since` | string | no | — | Only return entries on or after this date (YYYY-MM-DD) |
| `action` | string | no | — | Filter by action type (e.g. `"Creation"`, `"Update"`) |
| `limit` | number | no | all | Maximum number of entries to return |

### Response fields

| Field | Type | Description |
|-------|------|-------------|
| `entries` | array | Parsed log entries |
| `source` | string | `file_path` of log.md (relative path) |
| `note` | string | Present when degraded: `"no log.md found"` or malformed note |

Each entry object:

| Field | Type | Description |
|-------|------|-------------|
| `date` | string | Date of the heading (YYYY-MM-DD) |
| `action` | string | Action type (e.g. `"Creation"`, `"Update"`) |
| `target` | string | Target file path |
| `detail` | string | Full description text |

### Fallback behavior

When log.md is missing or malformed, `get_log` never silently returns nothing:

| Situation | Response |
|-----------|----------|
| No log.md found in bundle | `{"entries": [], "source": "", "note": "no log.md found"}` |
| log.md has malformed entries | Parsed entries returned with `"note": "log.md has malformed entries"` |

### Usage pattern

```
1. get_log                                       → all entries, newest first
2. get_log(since="2026-07-01")                   → entries from July 2026 onwards
3. get_log(action="Creation")                    → only creation entries
4. get_log(limit=5)                              → most recent 5 entries
```
