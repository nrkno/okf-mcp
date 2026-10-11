---
type: API Reference
title: MCP Tools Reference
description: Complete reference for the thirteen MCP tools exposed by okf-mcp — list_tags, list_docs, get_doc, validate_doc, get_index, get_log, and the graph tools (graph_concept, graph_relationships, graph_trace, graph_search, graph_integrity, graph_coverage, graph_context) — including parameters, response shapes, scoring, multi-bundle behavior, and error codes.
tags: [api, tools, list-tags, list-docs, get-doc, validate-doc, get-index, get-log, graph-concept, graph-relationships, graph-trace, graph-search, graph-integrity, graph-coverage, graph-context, mcp, scoring, match, multi-bundle, bundle, graph]
timestamp: 2026-08-31T00:00:00Z
---

# MCP Tools Reference

`okf-mcp` exposes navigation and analysis tools over the MCP stdio protocol. All tools rebuild the index on every call — freshly created or edited files are always reflected without restarting the server.

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
| `bundle` | string | OKF bundle the file belongs to (I-17) — relative path to the nearest ancestor directory containing `index.md`, or the file's immediate parent directory if no ancestor has one |

**Example response (single-bundle repo):**

```json
[
  {
    "title": "Architecture",
    "description": "Internal structure of okf-mcp ...",
    "tags": ["architecture", "scanner", "parser"],
    "file_path": "docs/architecture.md",
    "bundle": "docs"
  }
]
```

**Example response (multi-bundle repo, `--enable-hidden` set):**

```json
[
  {
    "title": "Architecture",
    "description": "Internal structure of okf-mcp ...",
    "tags": ["architecture"],
    "file_path": "docs/architecture.md",
    "bundle": "docs"
  },
  {
    "title": "API Design",
    "description": "API patterns",
    "tags": ["api"],
    "file_path": ".opencode/architecture/api-design.md",
    "bundle": ".opencode/architecture"
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
| `bundle` | string | OKF bundle the file belongs to (I-17) — relative path to the nearest ancestor directory containing `index.md`, or the file's immediate parent directory if no ancestor has one |

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
  "description": "Internal structure of okf-mcp ...",
  "bundle": "docs"
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
| `bundle` | string | OKF bundle the file belongs to (I-17) — **leaf-only** field: present on `file` and `reserved` nodes, absent on `directory` nodes |
| `children` | TreeNode[] | Child nodes (only present for directories) |

### Example response

```json
{
  "name": ".",
  "path": "",
  "type": "directory",
  "children": [
    { "name": "index.md", "path": "index.md", "type": "reserved", "bundle": "." },
    { "name": "docs", "path": "docs", "type": "directory", "children": [
      { "name": "architecture.md", "path": "docs/architecture.md", "type": "file", "doc_type": "Architecture", "title": "Architecture", "bundle": "docs" },
      { "name": "tools.md", "path": "docs/tools.md", "type": "file", "doc_type": "API Reference", "title": "MCP Tools Reference", "bundle": "docs" }
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

In a multi-bundle repository, `get_log` aggregates entries from **all** `log.md` files (one per bundle — I-12). Each entry is tagged with the relative path of the `log.md` it came from. Hidden-dir `log.md` files (e.g. `.opencode/architecture/log.md`) are aggregated only when the server was started with `--enable-hidden`; otherwise the visible `log.md` (typically `docs/log.md`) is the only source.

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
| `note` | string | Present when degraded: `"no log.md found"` or `"log.md has malformed entries"` |

The response does **not** carry a top-level `source` field. In a multi-bundle repository a single top-level value would be ambiguous; the per-entry `source` is the source of truth.

Each entry object:

| Field | Type | Description |
|-------|------|-------------|
| `date` | string | Date of the heading (YYYY-MM-DD) |
| `action` | string | Action type (e.g. `"Creation"`, `"Update"`) |
| `target` | string | Target file path |
| `detail` | string | Full description text |
| `source` | string | Relative path of the `log.md` the entry came from (I-12), e.g. `"docs/log.md"` or `".opencode/architecture/log.md"` |

### Sort order

The merged entry list is sorted with a multi-key comparator (I-12):

1. **Primary:** `date` descending (newest first).
2. **Secondary:** `source` ascending (lexicographic — `.opencode/...` sorts before `docs/...`).
3. **Tertiary:** document order. `sort.SliceStable` preserves the order entries appeared in the source `log.md` for entries that share both date and source.

### Fallback behavior

When no `log.md` is found or one is malformed, `get_log` never silently returns nothing:

| Situation | Response |
|-----------|----------|
| No log.md found in any bundle | `{"entries": [], "note": "no log.md found"}` |
| log.md has malformed entries | Parsed entries returned with `"note": "log.md has malformed entries"` |

### Usage pattern

```
1. get_log                                       → all entries, newest first
2. get_log(since="2026-07-01")                   → entries from July 2026 onwards
3. get_log(action="Creation")                    → only creation entries
4. get_log(limit=5)                              → most recent 5 entries
```

---

## `graph_concept`

Returns metadata and edge counts for a single concept (indexed document) identified by its relative `file_path`.

### Parameters

| Param | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `file_path` | string | **yes** | — | Relative path of the concept document |

### Response fields

| Field | Type | Description |
|-------|------|-------------|
| `file_path` | string | Relative path from the scan root |
| `type` | string | Document type from frontmatter |
| `title` | string | Document title from frontmatter |
| `description` | string | Document description from frontmatter |
| `tags` | string[] | Document tags from frontmatter |
| `bundle` | string | OKF bundle the file belongs to (I-17) |
| `outgoing_count` | int | Number of outgoing relationships |
| `incoming_count` | int | Number of incoming relationships |
| `outgoing_types` | string[] | Sorted unique outgoing relationship types |
| `incoming_types` | string[] | Sorted unique incoming relationship types |

### Error responses

| Situation | Error message |
|-----------|---------------|
| `file_path` missing | `file_path is required` |
| Concept not found | `concept not found: "<path>"` |

---

## `graph_relationships`

Return direct one-hop incoming/outgoing edges for a known concept. Use `graph_trace` for transitive traversal.

### Parameters

| Param | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `file_path` | string | **yes** | — | Relative path of the concept document |
| `direction` | string | no | `"both"` | `"outgoing"`, `"incoming"`, or `"both"` |
| `type` | string | no | — | Filter by relationship type |

### Response fields

| Field | Type | Description |
|-------|------|-------------|
| `outgoing` | array | Outgoing edges (see below) |
| `incoming` | array | Incoming edges (see below) |

Each outgoing edge:

| Field | Type | Description |
|-------|------|-------------|
| `target` | string | Relative path of the target concept |
| `type` | string | Relationship type |
| `heading` | string | Section heading where the link appeared |
| `line` | int | 1-based line number in the source document body |

Each incoming edge:

| Field | Type | Description |
|-------|------|-------------|
| `source` | string | Relative path of the source concept |
| `type` | string | Relationship type (inverse of the original edge) |
| `heading` | string | Section heading where the link appeared |
| `line` | int | 1-based line number in the source document body |

### Error responses

| Situation | Error message |
|-----------|---------------|
| `file_path` missing | `file_path is required` |
| Invalid `direction` | `invalid direction "<value>": must be "outgoing", "incoming", or "both"` |

---

## `graph_trace`

Follow relationships transitively upstream/downstream from a known concept.

### Parameters

| Param | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `file_path` | string | **yes** | — | Relative path of the starting concept |
| `direction` | string | **yes** | — | `"upstream"` (incoming) or `"downstream"` (outgoing) |
| `type` | string | no | — | Filter by relationship type |
| `max_depth` | number | no | `5` | Maximum traversal depth, clamped to 1–20 |

### Response fields

| Field | Type | Description |
|-------|------|-------------|
| `start` | string | Starting concept file path |
| `direction` | string | `"upstream"` or `"downstream"` |
| `steps` | array | Reachable concepts in BFS order |
| `total_reachable` | int | Total number of reachable concepts |

Each step:

| Field | Type | Description |
|-------|------|-------------|
| `node` | string | Relative path of the reachable concept |
| `depth` | int | Hop count from the start concept |
| `via_type` | string | Relationship type used to reach this node |
| `path` | string[] | Edge chain as `"source → target"` strings |

### Error responses

| Situation | Error message |
|-----------|---------------|
| `file_path` missing | `file_path is required` |
| Invalid `direction` | `invalid direction "<value>": must be "upstream" or "downstream"` |

---

## `graph_search`

Find graph concepts by topic/type/tags. Use this when you don't yet know the file path.

### Parameters

| Param | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `query` | string | no | `""` | Search query |
| `type` | string | no | — | Exact-match concept type filter |
| `tags` | string[] | no | — | Tag filter (OR semantics) |
| `limit` | number | no | `20` | Maximum results, clamped to 1–100 |

### Response fields

| Field | Type | Description |
|-------|------|-------------|
| `concepts` | array | Matching concepts, sorted by score desc then `file_path` asc |
| `total` | int | Total number of matching concepts before `limit` |

Each concept:

| Field | Type | Description |
|-------|------|-------------|
| `file_path` | string | Relative path from the scan root |
| `type` | string | Document type from frontmatter |
| `title` | string | Document title from frontmatter |
| `description` | string | Document description from frontmatter |
| `tags` | string[] | Document tags from frontmatter |
| `bundle` | string | OKF bundle the file belongs to (I-17) |
| `score` | number | Text relevance score from `matcher.Score` |

### Scoring

Same weighted-token model as `get_doc`: title 3×, tags 2×, description 1×. Tag-filter failures (score `-1`) are excluded. An empty `query` returns all concepts that pass the type/tag filters with score `0`.

---

## `graph_integrity`

Audit graph structure for dangling links, orphans, profile violations and superseded dependencies.

### Parameters

| Param | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `checks` | string[] | no | all | Subset of: `"dangling"`, `"orphans"`, `"profile_violations"`, `"superseded_deps"` |

### Response fields

| Field | Type | Description |
|-------|------|-------------|
| `summary.dangling_refs` | int | Number of dangling-reference findings |
| `summary.orphan_concepts` | int | Number of orphan-concept findings |
| `summary.profile_violations` | int | Number of profile type/cardinality findings |
| `summary.superseded_deps` | int | Number of superseded-dependency findings |
| `summary.total_findings` | int | Sum of all findings |
| `findings` | array | Individual findings (see below) |
| `profile_loaded` | bool | `true` when a non-default profile is loaded |

Each finding object:

| Field | Type | Description |
|-------|------|-------------|
| `check` | string | Check name |
| `severity` | string | `"error"`, `"warning"`, or `"notification"` |
| `source` | string | Source file path (or concept path) |
| `target` | string | Target file path, when applicable |
| `heading` | string | Heading context, when applicable |
| `message` | string | Human-readable description |

### Error responses

| Situation | Error message |
|-----------|---------------|
| Unknown check in `checks` | `unknown check "<value>"` |

---

## `graph_coverage`

Test whether concepts of one type have a relationship path to another type. The BFS traverses the relationship in both directions so that edges authored from the target side (e.g., `Implementation` → `Requirement`) are discovered when querying from the source side (`Requirement` → `Implementation`).

### Parameters

| Param | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `source_type` | string | **yes** | — | Concept type to start from (e.g. `"Requirement"`) |
| `target_type` | string | **yes** | — | Concept type to reach (e.g. `"Implementation"`) |
| `relationship` | string | no | `""` (all types) | Follow only edges of this type |

### Response fields

| Field | Type | Description |
|-------|------|-------------|
| `source_type` | string | Source concept type |
| `target_type` | string | Target concept type |
| `total_sources` | int | Total number of source-type concepts |
| `covered` | int | Number of source concepts that reach at least one target |
| `uncovered` | int | Number of source concepts that reach no targets |
| `coverage_ratio` | number | `covered / total_sources` (0.0–1.0) |
| `uncovered_items` | array | Source concepts with no path to a target |

Each uncovered item:

| Field | Type | Description |
|-------|------|-------------|
| `file_path` | string | Relative path of the uncovered concept |
| `title` | string | Concept title |
| `nearest_targets` | string[] | Always `[]` for uncovered items |

### Error responses

| Situation | Error message |
|-----------|---------------|
| `source_type` missing | `source_type is required` |
| `target_type` missing | `target_type is required` |

---

## `graph_context`

Return a bounded neighborhood intended for reasoning context when loading the entire graph would be excessive. The result includes the center concept, its immediate neighbors, and optionally one more layer, bounded by `max_results`.

### Parameters

| Param | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `file_path` | string | **yes** | — | Center concept |
| `depth` | number | no | `1` | Neighborhood depth (`1` or `2`) |
| `max_results` | number | no | `100` | Maximum total neighbor entries across all directions and depths (`1`–`1000`) |

### Response fields

| Field | Type | Description |
|-------|------|-------------|
| `center` | object | `{file_path, type, title}` of the center concept |
| `neighbors.upstream` | array | Incoming neighbors as `{file_path, type, via}` objects |
| `neighbors.downstream` | array | Outgoing neighbors as `{file_path, type, via}` objects |
| `depth` | int | Requested depth |
| `total_neighbors` | int | Number of neighbors returned |
| `max_results` | int | Requested budget |
| `truncated` | bool | `true` when the budget was exhausted |

Neighbors are collected deterministically: upstream first, then downstream, alphabetical by `file_path` within each direction. The center concept does not count against the `max_results` budget. At `depth=2`, depth-1 neighbors are collected first, then remaining budget is allocated to depth-2 neighbors.

### Error responses

| Situation | Error message |
|-----------|---------------|
| `file_path` missing | `file_path is required` |
| Concept not found | `concept not found: "<path>"` |
