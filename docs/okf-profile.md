---
type: Configuration
title: OKF Profile Template
description: How to edit the checked-in .okf-profile.yaml declaration, with concept and relationship examples, field meanings, and the current implementation's lack of profile loading or enforcement.
tags: [okf-profile, configuration, yaml, concepts, relationships, invariants]
timestamp: 2026-10-10T00:00:00Z
---

# OKF profile template

## Support boundary

The repository contains [`.okf-profile.yaml`](../.okf-profile.yaml), an AOS vocabulary declaration. **The implementation in this checkout does not read or enforce it.** There is no profile CLI flag, environment variable, automatic discovery, or validated profile schema. The filename is `.okf-profile.yaml`, not an extensionless `.okf-profile`.

Editing this file records a vocabulary for people or a separately configured consumer; it does not configure this checkout's `okf-mcp` runtime. The comment in the sample claiming conformance to a generic profile schema is not a schema guarantee provided by this implementation. If another tool consumes the file, consult that tool's documentation before relying on field semantics, defaults, or validation.

This checkout serves six documentation tools, not graph tools. It scans Markdown, ignores the profile's YAML file, and validates document frontmatter independently of the profile. `--enable-hidden` enables traversal of hidden directories; it does not enable profiles. `--path` selects a validation directory, not a profile file. See [Configuration](/docs/configuration.md) for supported runtime settings.

## Editing the declaration

Keep the declaration in the repository root as `.okf-profile.yaml`, matching the checked-in sample's location. This is an authoring convention here, **not a runtime discovery rule**. Start from the sample and edit its `name`, `version`, concept names, relationship declarations, and invariants to describe your intended vocabulary. Keep `version` quoted as a string and use YAML lists as shown below.

The sample declares these concept names: `anchor`, `requirement`, `requirement-index`, `decision`, `design`, `spec`, and `plan`. Its relationship names are `derived-from`, `satisfies`, `implements`, `verified-by`, `depends-on`, and `supersedes`.

### Field guide

These are illustrative meanings conveyed by the declaration, not an implemented API contract. Required fields, accepted values, case normalization, uniqueness rules, and omitted-field defaults are not checked by this checkout.

| Field | Meaning expressed by the sample |
|---|---|
| `name` | Profile identifier (`aos` in the checked-in sample). |
| `version` | Version label for the declaration (`"1.0"`). It is not the binary version. |
| `concept_types` | List of mappings with a `name` identifying each concept type. |
| `relationships` | List of named relationship declarations. |
| `heading_aliases` | Human-readable headings associated with a relationship, such as `Derived from`. |
| `inverse` | Name for the relationship viewed in reverse, such as `derives`. |
| `allowed_source_types` | Concept names intended to be permitted at the relationship's source. |
| `allowed_target_types` | Concept names intended to be permitted at its target. |
| `invariants` | List of declared graph checks; none run in this checkout. |
| `id`, `description` | Identifier and human explanation of an invariant. |
| `source_type` | Concept type named by an invariant. |
| `direction` | Direction label in the declaration; no direction semantics are implemented here. |
| `min`, `max` | Intended count bounds; the sample uses `1` and `null`. The meaning of `null` must be confirmed with any actual consumer. |
| `severity` | Finding label (`error` in the sample); it does not affect this checkout's validation exit code. |

### Small vocabulary example

This YAML is an editable declaration example, not an activation recipe:

```yaml
name: example
version: "1.0"
concept_types:
  - name: anchor
  - name: requirement
relationships:
  - name: derived-from
    heading_aliases:
      - Derived from
    inverse: derives
    allowed_source_types:
      - requirement
    allowed_target_types:
      - anchor
invariants: []
```

A Markdown document can express the corresponding intended relationship:

```markdown
---
type: requirement
title: Example Requirement
description: A requirement derived from the example anchor.
tags: [example, requirement]
---

# Example Requirement

## Derived from

- [Example anchor](anchor.md)
```

Place the linked `anchor.md` alongside the requirement when adapting this example. In this checkout, a non-empty `type` makes the document indexable; neither declaring `requirement` in the profile nor using the heading creates a graph edge. Single-file `validate_doc` uses the standard OKF vocabulary unless `known_types` is supplied, so it warns about `requirement` by default. Bundle validation derives its vocabulary from indexed documents instead. Changing `concept_types` does not configure either validation path.

## Invariant caveat in the checked-in sample

The `requirement-has-source` invariant says every requirement must have an **outgoing `derived-from`** edge, but its fields specify `direction: incoming` and do not name `derived-from`. These are inconsistent or underspecified declarations, not an enforced rule. Do not assume the description overrides the fields, that `incoming` means outgoing, or that a consumer infers the relationship from the ID. Resolve the direction and relationship-selection contract with the consuming implementation before using this invariant. This documentation leaves the sample unchanged.

## Checking your work

For this repository's documentation bundle, run from the repository root:

```sh
go run ./cmd/okf-mcp --validate --path docs
```

This checks Markdown conformance only. A successful result does **not** establish that `.okf-profile.yaml` is valid, loaded, or enforced. YAML syntax checking with another tool likewise does not prove semantic compatibility with a profile consumer.
