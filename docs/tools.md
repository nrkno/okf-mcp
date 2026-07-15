---
type: API Reference
title: MCP Tools Reference
description: Complete reference for the three MCP tools exposed by okf-mcp — list_tags, list_docs, and get_doc — including parameters, response shapes, scoring, and error codes.
tags: [api, tools, list-tags, list-docs, get-doc, mcp, scoring, match]
timestamp: 2026-07-15T00:00:00Z
---

# MCP Tools Reference

`okf-mcp` exposes three tools over the MCP stdio protocol. All tools rebuild the index on every call — freshly created or edited files are always reflected without restarting the server.

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
