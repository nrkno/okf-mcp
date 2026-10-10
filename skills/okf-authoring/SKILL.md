---
name: okf-authoring
description: Author and maintain OKF documentation using okf-mcp. Use when creating .okf-profile.yaml, deriving document relationships, formatting frontmatter, maintaining index.md or log.md, or checking documentation conformance. Not for deciding whether substantive product or technical content is correct.
compatibility: Requires repository read access and a configured okf-mcp server for observed tool verification; profile checks require graph-capable tools.
license: MIT
---

# OKF authoring

Turn accepted documentation content and actual cross-document dependencies into a discoverable, correctly structured corpus. This method does not grant write permissions or decide content authority.

## Discover before authoring

1. Call `get_index` to identify the scan root and bundle tree. Use `list_docs`, `list_tags`, and `get_doc` to find existing conventions and relevant documents. Confirm the server indexes the intended repository, including required hidden directories.
2. Read the target repository's authoring rules, current profile, bundle indices and governing change logs. Inventory files as well as indexed documents: missing frontmatter/type can make a real document invisible. An empty tool result is not proof of absence.
3. Determine ownership for every changed file. The nearest ancestor `index.md` normally identifies a bundle; absent an index, the server reports the immediate parent as fallback, not proof that a deliberate bundle exists. Nested bundles keep their own applicable inventory. Follow existing policy for shared versus bundle-local logs, rather than creating a log in every folder.
4. Distinguish content documents, reserved files, and non-document assets. Native `SKILL.md` uses harness metadata such as `name`/`description`; do not force an OKF `type` into it. Companion methodology, fixtures and code are not automatically corpus documents. Explicitly identify any intentionally published reference documents.

Read the direct [format reference](references/okf-standard.md) for document conventions and the direct [profile reference](references/profile-format.md) before editing a profile. These shipped references are synchronized release snapshots, not a second independently maintained schema. Target repository rules and the observed server version determine applicability; report version mismatches rather than inventing support.

## Derive relationships from evidence

For each candidate relationship, record the source document, its actual link/statement, nearest heading, target document, and the meaning already established by the accepted content. Discover existing edges with `graph_search`, `graph_concept`, and `graph_relationships`; use bounded `graph_context` or `graph_trace` when surrounding dependencies matter. Inspect unknown relationship names rather than guessing vocabulary.

The **source contains the link**; the **target is the linked document**. If `requirement.md` links to `anchor.md` under `Derived from`, read `requirement → derived-from → anchor`: `allowed_source_types: [requirement]`, `allowed_target_types: [anchor]`, using those exact frontmatter `type` values. An incoming inverse query on the anchor does not swap these forward constraints. Separately, invariant `source_type` selects the node being checked even for `direction: incoming`; to count incoming derivations at anchors, select `anchor` and name the forward `derived-from` relationship. The direct profile reference provides the complete worked example.

Choose a small vocabulary that expresses genuine relationships, consolidating repeated patterns where useful. Distinguish reference, dependency, derivation, implementation and supersession only when the source material supports that distinction. Do not manufacture edges to satisfy a desired coverage number, infer a dependency merely from co-occurrence, or rewrite substantive meaning to make a graph green. If meaning is unresolved, identify the question and keep the affected classification explicit rather than deciding it in the profile.

## Realize the profile and documents

- Preserve the repository's vocabulary when it already fits. Map observed headings to relationships; select allow-lists from actual frontmatter types. Use unique normalized heading aliases and exact case-sensitive types/names. Concept aliases do not rewrite types or expand allow-lists.
- Derive cardinality constraints from accepted obligations, not aesthetic preferences. Include `relationship`, explicit node selector/direction, and intentional bounds. `max: -1` is unlimited; omitted/null maximum is zero. An incoming check names the original forward relationship, not its displayed inverse. Descriptions do not override fields.
- Confirm startup selection with stderr and graph tools. Explicit `--profile` fails on invalid input; invalid automatic discovery can fall back to an untyped default. Restart after profile changes. Do not mistake `profile_loaded: false` or zero findings for proof of intended enforcement.
- Format ordinary documents with valid YAML and non-empty `type`; provide useful title, description, lowercase/hyphenated tags, and current timestamp where applicable. Use repository-appropriate headings and scan-root-relative cross-links. Check that the nearest heading really classifies each intended link. Unmapped headings produce `untyped` edges, not type violations.
- `index.md` has **no frontmatter**. Maintain a plain Markdown link inventory of **all applicable documents owned by that bundle**, including its log when applicable; no self-link is needed. Check actual targets, missing documents, renamed paths, and duplicate/stale entries. Do not flatten unrelated nested bundles into the parent inventory; provide navigation to their indices when the repository convention calls for it.
- `log.md` has `type: Log` frontmatter and newest-first `## YYYY-MM-DD` sections. For documentation changes, update the applicable document timestamp and add a **current-date** entry naming the changed/created documents with the repository's action format (for example `**Update**` or `**Creation**`). Preserve history. A structurally valid but stale log is not current.

## Verify the distinct claims

1. Run `validate_doc` on changed documents/reserved files and the applicable corpus. Single-file `known_types` can describe the local vocabulary; bundle validation derives its vocabulary from indexed types. Neither proves substantive content truth.
2. Check profile selection, typed/inverse edges, and `graph_integrity` for applicable type/cardinality constraints. Inspect findings and relevant dangling references; do not discard them to obtain a clean result. Profile integrity is **not** enforced by `validate_doc` or CLI `--validate`.
3. Independently compare the actual bundle inventory with index links, and the changed-file list/current date with log entries. Implemented validation does not prove index completeness, log currency, tag style, timestamps, or correct bundle membership.
4. Keep validation scopes explicit. A native skill metadata file can be correct for its harness yet fail OKF document validation if wrongly included. Validate document corpora, not arbitrary repository Markdown; do not modify valid native metadata solely to silence the wrong validator.
5. Exercise a representative real relationship and a useful near-miss (such as a missing required link or wrong target type). Report observed results separately from proposed examples, unexecuted checks, and unavailable tools. No MCP access means verification remains unproven, not passed.

Return changed paths, bundle/index/log reconciliation, profile selection, observed conformance and graph results, and any unresolved boundary. Keep substantive correctness claims outside this skill's remit.
