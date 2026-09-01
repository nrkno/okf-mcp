---
type: Configuration
title: OKF Profile Format
description: Complete reference for the .okf-profile.yaml relationship profile used by okf-mcp to classify Markdown links into typed graph edges and enforce graph integrity invariants.
tags: [profile, okf, yaml, schema, graph, relationships, invariants]
timestamp: 2026-09-01T00:00:00Z
---

# OKF Profile Format

## 1. What `.okf-profile.yaml` is

`.okf-profile.yaml` is an optional YAML file that lives at the scan root of an OKF bundle. It defines a domain-specific relationship vocabulary for the bundle's Markdown documents: which headings classify links into which edge types, what the inverse of each edge is, which concept types may appear on each side of an edge, and what cardinality invariants should hold. `okf-mcp` loads it automatically on startup (auto-discover) or when given an explicit `--profile` flag. If no profile is found, `profile.Default()` is used: every link is recorded as an `untyped` edge and no integrity invariants are enforced.

## 2. Top-level schema

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | no | Human-readable profile name, logged on load |
| `version` | string | **yes** | MUST be `"1.0"` |
| `concept_types` | list | no | Document categories used by the profile |
| `relationships` | list | no | Heading → edge-type mappings |
| `invariants` | list | no | Cardinality rules over relationships |

## 3. `concept_types[]`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | **yes** | Canonical concept type name |
| `aliases` | string[] | no | Alternative names for the type |

Aliases are matched verbatim; they are **not** normalized like heading aliases. There is no uniqueness check across concept types.

## 4. `relationships[]`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | **yes** | Canonical relationship name (e.g. `"depends_on"`) |
| `heading_aliases` | string[] | **yes** | One or more headings that map to this relationship |
| `inverse` | string | no | Inverse relationship name; defaults to `"referenced_by"` |
| `allowed_source_types` | string[] | no | Allowed source concept types; empty or `["*"]` means any |
| `allowed_target_types` | string[] | no | Allowed target concept types; empty or `["*"]` means any |

### Heading alias normalization

Each heading alias is normalized before it is stored: lowercased, trimmed, and consecutive whitespace collapsed to a single space. For example:

```text
"  DEPENDS   on " → "depends on"
```

### Wildcard semantics

An empty list, a missing field, or the literal string `"*"` means "any type allowed". A named list restricts the edge to only those concept types.

### I-31 uniqueness

No two relationships may share the same normalized heading alias. If two relationships claim `"depends on"`, `profile.Load()` rejects the profile with a descriptive error naming the alias and both relationship definitions. Duplicate aliases **within** a single relationship are deduplicated rather than rejected.

### Inverse fallback

If `inverse` is omitted or empty, the runtime inverse for that relationship is `"referenced_by"`.

## 5. `invariants[]`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | **yes** | Short invariant identifier (e.g. `"P1"`) |
| `description` | string | no | Human-readable explanation |
| `source_type` | string | no | Concept type to check; `"*"` or omitted means all types |
| `relationship` | string | **yes** | Relationship the invariant applies to |
| `direction` | string | **yes** | `"incoming"` or `"outgoing"` |
| `min` | int | no | Minimum required edge count; default `0` |
| `max` | int | no | Maximum allowed edge count; default `0`; `-1` means unlimited; `0` means forbidden |
| `severity` | string | **yes** | `"error"`, `"warning"`, or `"notification"` |

An invariant with `min > max` is rejected as contradictory. A `max` of `0` forbids the relationship in the given direction, while `max: -1` places no upper bound.

## 6. How heading classification works

When `okf-mcp` extracts a Markdown link, it records the heading the link sits under. That heading is normalized using the same rules as `heading_aliases` and matched against the profile. If a match is found, the edge is typed with the corresponding relationship. If no match is found, the edge is recorded as `untyped` (per I-25). With no profile loaded, every edge is `untyped` (per I-26).

## 7. Worked example

The following is the shipped `.okf-profile.yaml` from this repository, annotated with inline comments:

```yaml
# Human-readable name, logged to stderr when the profile is loaded.
name: "plattform-okf-mcp"

# Profile format version. Only "1.0" is supported.
version: "1.0"

# Concept types mirror the OKF type vocabulary used by the docs bundle.
concept_types:
  - name: "Architecture"
    aliases: ["architecture", "system-design"]
  - name: "Playbook"
    aliases: ["playbook", "runbook", "how-to"]
  - name: "Configuration"
    aliases: ["configuration", "setup"]
  - name: "API Reference"
    aliases: ["api-reference", "reference"]
  - name: "Metrics Reference"
    aliases: ["metrics-reference", "metrics"]
  - name: "Log"

# Relationships map Markdown headings to canonical edge types.
relationships:
  # A general "see also" / reference edge. "Running" catches the
  # deployment.md → configuration.md link under the "Running" heading.
  - name: "references"
    heading_aliases: ["See also", "References", "Related", "Related work", "Running"]
    inverse: "referenced_by"
    allowed_source_types: ["*"]
    allowed_target_types: ["*"]

  # Dependency edge: source depends on target.
  - name: "depends_on"
    heading_aliases: ["Depends on", "Dependencies", "Requires"]
    inverse: "depended_on_by"
    allowed_source_types: ["*"]
    allowed_target_types: ["*"]

  # Implementation edge: source implements a target architecture/config/API.
  - name: "implements"
    heading_aliases: ["Implements", "Implementation of", "Realizes"]
    inverse: "implemented_by"
    allowed_source_types: ["*"]
    allowed_target_types: ["Architecture", "Configuration", "API Reference"]

  # Supersession edge: source replaces a target document.
  - name: "supersedes"
    heading_aliases: ["Supersedes", "Replaces", "Obsoletes"]
    inverse: "superseded_by"
    allowed_source_types: ["*"]
    allowed_target_types: ["*"]

# Integrity invariants enforce cardinality rules over the graph.
invariants:
  - id: "P1"
    description: "Supersession is one-to-one: no document should supersede more than one predecessor."
    source_type: "*"
    relationship: "supersedes"
    direction: "outgoing"
    min: 0
    max: 1
    severity: "warning"
```

## 8. Loading order

`okf-mcp` resolves the profile in this order:

1. **Explicit `--profile <path>`** — if given, the file must be valid. An invalid explicit path exits code 2.
2. **Auto-discovered `<scan-root>/.okf-profile.yaml`** — if present, it is loaded. An invalid discovered file logs a warning and falls through to the default profile.
3. **`profile.Default()`** — used when no explicit path is given and no `.okf-profile.yaml` exists, or when the discovered file is invalid. All edges are `untyped`.

Profile selection is always logged to stderr.

## 9. Validation errors

| Error | Cause | Fix |
|-------|-------|-----|
| `unsupported profile version` | `version` is not `"1.0"` | Set `version: "1.0"` |
| `relationship name is empty` | A relationship has no `name` | Add a non-empty `name` |
| `relationship ... has no heading aliases` | `heading_aliases` is empty or missing | Add at least one heading alias |
| `heading alias ... is shared by relationships ...` (I-31) | Two relationships normalize to the same alias | Ensure each normalized alias is unique across all relationships |
| `invariant id is empty` | An invariant has no `id` | Add a non-empty `id` |
| `invariant ... has invalid direction` | `direction` is not `"incoming"` or `"outgoing"` | Use `"incoming"` or `"outgoing"` |
| `invariant ... has invalid severity` | `severity` is not one of the allowed values | Use `"error"`, `"warning"`, or `"notification"` |
| `invariant ... has contradictory cardinality` | `min > max` | Adjust `min` and `max` so `min <= max` (or set `max: -1` for unlimited) |
