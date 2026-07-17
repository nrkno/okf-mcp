---
type: Architecture
title: OKF Standard
description: The Open Knowledge Format v0.1 frontmatter schema, type vocabulary, skip rules, and authoring conventions as implemented by okf-mcp.
tags: [okf, frontmatter, standard, conventions, index, log]
timestamp: 2026-07-16T00:00:00Z
---

# OKF Standard

## What OKF is

OKF (Open Knowledge Format) v0.1 is a lightweight frontmatter convention for markdown documentation files. It was developed for the NRK Plattform ecosystem. The goal is to make documentation machine-queryable: by embedding structured metadata in a predictable YAML block at the top of each file, tools like `okf-mcp` can index, filter, and rank docs without parsing their prose content.

## Frontmatter schema

Every content document must begin with a YAML frontmatter block delimited by `---`:

```yaml
---
type: Architecture
title: My Document Title
description: One sentence describing what this document covers and when to use it.
tags: [tag-one, tag-two, tag-three]
timestamp: 2026-07-15T00:00:00Z
---
```

| Field | Required | Weight in `get_doc` scoring | Purpose |
|-------|----------|----------------------------|---------|
| `type` | **Required** | — | Classifies the document; missing → silently skipped by okf-mcp |
| `title` | Strongly recommended | **3×** | Primary match signal; missing → warning to stderr, still indexed |
| `description` | Strongly recommended | **1×** | Secondary match signal; missing → warning to stderr, still indexed |
| `tags` | Strongly recommended | **2×** | Tag-filtered search; missing → indexed with no tags |
| `timestamp` | NRK convention | — | ISO 8601 timestamp of last significant edit; always include |

`type` is the only hard requirement enforced by okf-mcp. The other fields degrade matching quality rather than preventing indexing.

## Type vocabulary

NRK Plattform uses these six `type` values:

| Type | Use for |
|------|---------|
| `Architecture` | System design, component structure, design decisions, invariants |
| `Playbook` | Step-by-step operational procedures, runbooks, how-to guides |
| `Configuration` | Setup instructions, config schemas, environment variables, client registration |
| `API Reference` | Tool parameters, response shapes, error codes, endpoint contracts |
| `Metrics Reference` | Metric names, labels, alert thresholds, SLO definitions |
| `Log` | Reserved for `log.md` — the documentation change log |

**Note:** `Metrics Reference` is intended for services that expose structured numeric metrics (e.g. Prometheus, OpenTelemetry). okf-mcp does not expose metrics, so no example of this type exists in this repository.

okf-mcp does not enforce or validate the `type` value; any non-empty string passes the indexing gate. The vocabulary above is a convention, not a filter.

## Skip rules enforced by okf-mcp

Files are skipped when any of the following apply (applied in order during scanning):

1. **Hidden directory** — any directory whose name starts with `.` is skipped entirely, along with all its contents. This includes `.git`, `.opencode`, `.github`, etc.
2. **OKF-reserved filename** — files named exactly `index.md` or `log.md` are never indexed, regardless of their contents.
3. **Non-markdown extension** — files whose extension is not `.md` are skipped.
4. **No frontmatter** — files that do not begin with `---\n` are skipped silently.
5. **Missing `type` field** — files with frontmatter but no `type` value are skipped silently.

When `title` or `description` is absent, a warning is written to stderr but the file is still indexed:

```
okf-mcp: WARN: docs/auth.md: missing title
okf-mcp: WARN: docs/auth.md: missing description
```

## Reserved filenames

### `index.md`

- **No frontmatter** — this is a hard OKF spec requirement. A frontmatter block in `index.md` will be rendered as literal text.
- Contains a plain markdown list of links to the other docs in the bundle.
- Never indexed by okf-mcp (invariant I-4).

### `log.md`

- **Has frontmatter** with `type: Log`.
- Contains a chronological record of changes to the docs bundle.
- Never indexed by okf-mcp (invariant I-4) — it is excluded at the scanner level regardless of its frontmatter.

## Tag formatting conventions

- All tags must be **lowercase**.
- Multi-word tags use **hyphens**, not underscores or spaces: `client-setup`, not `client_setup`.
- Tags are specified as a YAML inline list on a single line: `tags: [tag-one, tag-two]`.
- Avoid redundant tags that duplicate the document title. Tags are match signals, not decorations.

## Cross-link conventions

Links between docs in the same bundle use **bundle-relative paths** rooted at the repo root:

```markdown
[Architecture](/docs/architecture.md)
```

Not bare filenames (`architecture.md`) and not absolute filesystem paths.

## Update obligation

When modifying an existing document:

1. Update the `timestamp:` field to the current date in ISO 8601 format.
2. Add an entry to `docs/log.md` under today's date describing what changed.

When creating a new document: add it to `docs/index.md` and add a creation entry to `docs/log.md`.
