---
name: okf-authoring
description: Discover and consume OKF knowledge for agent tasks; author and maintain OKF metadata, profiles, relationships, indexes and logs, and verify their representation. Not for deciding whether substantive product or technical content is correct.
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

Read the direct [format reference](references/okf-standard.md) for this repository's OKF v0.1 conventions and the direct [profile reference](references/profile-format.md) before editing a profile. These shipped references are synchronized release snapshots, not a second independently maintained schema. The upstream [OKF v0.2 specification](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/main/SPEC.md) defines newer, optional provenance/trust/lifecycle fields. **Do not silently claim that this server or its v0.1 reference implements every v0.2 feature.** Apply target-repository rules and the observed server version; report mismatches rather than inventing support.

## Consume task-relevant knowledge

Use this method for development, review, or delegation **even when no documentation will be authored**. It is a discovery method, not permission to make product decisions.

1. **Establish the corpus.** Confirm the intended scan root and visible bundles with `get_index`; inspect active profile/relationship vocabulary if typed edges matter. When needed, use `list_tags`, `list_docs`, or direct file inventory to find documents that may be missing from the index.
2. **Start narrow.** Use `graph_search` by topic, `type`, or `tags` to select relevant candidates. `get_doc` returns the **single highest-scoring** topic match, not proof that all matching documents were read. Verify its returned path; read a known file directly when exact identity matters.
3. **Follow meaning, not every edge.** Use `graph_context` for a bounded neighborhood and `graph_relationships` for direct parents/dependents. Use `graph_trace` only for a concrete dependency or impact question. Check whether links are typed, untyped, dangling, or affected by a fallback profile; an edge alone is not a guarantee.
4. **Read authoritative documents.** Expand selected source content, not the whole corpus. Distinguish accepted intent, proposed decisions, historical/superseded material, and current implementation claims. Interpret optional `sources`, `generated`, `verified`, `status`, and `stale_after` only when present and supported; absence is not a conformance error, and a recorded verification actor is not independent proof in itself.
5. **Compare with reality.** Inspect relevant code, tests, runtime behavior, and current external contracts. Code proves what exists; it does not silently override accepted requirements. A graph or coverage query cannot prove implementation correctness.
6. **Package a bounded handoff.** Give the working agent or fresh session the source paths and necessary semantic contracts, constraints, gaps, and verification expectations. Do not copy entire documents or invent unresolved meaning to fill a task template.

If the index is empty or incomplete, check scanner visibility, frontmatter and actual files; fall back to source reads and report the OKF limitation. Do not treat missing search results as proof that a requirement does not exist.

## Derive relationships from evidence

For each candidate relationship, record the source document, its actual link/statement, nearest heading, target document, and the meaning already established by the accepted content. Discover existing edges with `graph_search`, `graph_concept`, and `graph_relationships`; use bounded `graph_context` or `graph_trace` when surrounding dependencies matter. Inspect unknown relationship names rather than guessing vocabulary.

The **source contains the link**; the **target is the linked document**. If `requirement.md` links to `anchor.md` under `Derived from`, read `requirement → derived-from → anchor`: `allowed_source_types: [requirement]`, `allowed_target_types: [anchor]`, using those exact frontmatter `type` values. An incoming inverse query on the anchor does not swap these forward constraints. Separately, invariant `source_type` selects the node being checked even for `direction: incoming`; to count incoming derivations at anchors, select `anchor` and name the forward `derived-from` relationship. The direct profile reference provides the complete worked example.

Choose a small vocabulary that expresses genuine relationships, consolidating repeated patterns where useful. Distinguish reference, dependency, derivation, implementation and supersession only when the source material supports that distinction. Do not manufacture edges to satisfy a desired coverage number, infer a dependency merely from co-occurrence, or rewrite substantive meaning to make a graph green. If meaning is unresolved, identify the question and keep the affected classification explicit rather than deciding it in the profile.

## Realize the profile and documents

### Worked corpus-to-profile walkthrough

Suppose discovery finds `docs/goal.md` (`anchor`), `docs/retry.md` (`requirement`), `docs/client.md` (`design`), and `docs/background.md` (`Architecture`). The inventory also contains `docs/draft.md` without metadata: inspect it before deciding whether it is a document to repair or an intentional asset; do not let its absence from `list_docs` decide membership.

Accepted content says retry derives from goal; client implements retry. The accepted vocabulary permits derivation only from requirements to anchors and implementation only from designs to requirements. Retry links to goal under `Derived from`; another requirement uses `Source goal` for the same meaning. Client links to retry under `Implements` and to background under `Further reading`. Client prose says it cannot operate without the protocol in background, but that statement is not yet a link under a dependency heading. The only accepted cardinality obligation is that every requirement cites at least one source goal.

Build a small evidence ledger before choosing fields:

| Source / heading / target | Established meaning | Representation decision |
|---|---|---|
| retry / Derived from / goal | Goal motivates requirement | `derived-from`; requirement → anchor |
| other requirement / Source goal / goal | Same derivation meaning | Add `Source goal` as an alias, not a new relationship |
| client / Implements / retry | Design realizes requirement | `implements`; design → requirement |
| client / Further reading / background | Reading reference | Leave `untyped`, or define unrestricted `references` if useful for navigation |
| client prose / background | Operational prerequisite | Preserve the statement; add an authorized link under `Depends on` if dependency navigation is wanted; do not relabel the reading link |

Choose `derived-from` and `implements` first: they answer the corpus's traceability questions. A separate `depends-on` is useful only if operational dependency queries are needed and the source actually represents that relationship. Keeping ordinary reading links untyped is valid; mapping them to `references` is also reasonable, but neither makes them dependencies. A single generic `related-to` would lose the accepted distinction between motivation and realization.

Use exact observed types for the two typed relationships. Do not restrict optional references to this four-file sample: no accepted closed type domain supports that restriction. Add only the outgoing requirement minimum (`relationship: derived-from`, `source_type: requirement`, `direction: outgoing`, `min: 1`, `max: -1`). Omit a maximum of one, mandatory implementation, anchor incoming coverage, and dependency acyclicity: none was required. Concept declarations describe vocabulary, not obligations.

Iterate against actual links: both derivation headings must produce the same forward type; client reading must not count as implementation or dependency. Inspect a link moved under `### Notes`: the nearest heading changes classification. Removing retry's source should fail the accepted minimum; removing optional reading should not. If that result is surprising, revisit the evidence ledger and aliases before changing substantive content or weakening an obligation. The [setup guide](references/profile-setup.md) supplies server connection steps; the direct [field reference](references/profile-format.md) includes complete linked documents for exercising direction and selectors.

For the minimal choice above (ordinary reading remains untyped), the profile is:

```yaml
name: traceability-example
version: "1.0"
relationships:
  - name: derived-from
    heading_aliases: [Derived from, Source goal]
    inverse: derives
    allowed_source_types: [requirement]
    allowed_target_types: [anchor]
  - name: implements
    heading_aliases: [Implements]
    inverse: implemented-by
    allowed_source_types: [design]
    allowed_target_types: [requirement]
invariants:
  - id: requirement-has-goal
    source_type: requirement
    relationship: derived-from
    direction: outgoing
    min: 1
    max: -1
    severity: error
```

### Representation details

- Preserve the repository's vocabulary when it already fits. Map observed headings to relationships; select allow-lists from actual frontmatter types. Use unique normalized heading aliases and exact case-sensitive types/names. Concept aliases do not rewrite types or expand allow-lists.
- Derive cardinality constraints from accepted obligations, not aesthetic preferences. Include `relationship`, explicit node selector/direction, and intentional bounds. `max: -1` is unlimited; omitted/null maximum is zero. An incoming check names the original forward relationship, not its displayed inverse. Descriptions do not override fields.
- Confirm startup selection with stderr and graph tools. Explicit `--profile` fails on invalid input; invalid automatic discovery can fall back to an untyped default. Restart after profile changes. Do not mistake `profile_loaded: false` or zero findings for proof of intended enforcement.
- Format ordinary documents with valid YAML and non-empty `type`; provide useful title, description, lowercase/hyphenated tags, and current timestamp where applicable. Use repository-appropriate headings and scan-root-relative cross-links. Check that the nearest heading really classifies each intended link. Unmapped headings produce `untyped` edges, not type violations.
- This repository's `index.md` files have **no frontmatter**. Maintain a plain Markdown link inventory of **all applicable documents owned by that bundle**, including its log when applicable; no self-link is needed. Check actual targets, missing documents, renamed paths, and duplicate/stale entries. Do not flatten unrelated nested bundles into the parent inventory. Upstream OKF v0.2 permits a special root-index version declaration; do not assume this server's current reserved-file validator accepts it.
- **In this repository**, `log.md` uses `type: Log` frontmatter; upstream OKF does not universally require that metadata. Preserve newest-first `## YYYY-MM-DD` entries and follow each target repository's log policy. For documentation changes, record affected documents in the applicable current-date log and update metadata required by that repository. A structurally valid but stale log is not current. OKF v0.2 favors `generated.at` for provenance, while legacy v0.1 `timestamp` remains readable; never fabricate producer or verification identity.

## Coordinate parallel documentation updates

Documentation changes may be written alongside independent implementation tasks. Before dispatch, identify the **content owner** and the owner of any shared `index.md`, `log.md`, and `.okf-profile.yaml` edits.

- Prefer agents editing their own authoritative documents and using the same agreed relationship vocabulary. Do not let separate tasks invent conflicting document IDs, targets, or semantics.
- Do not concurrently overwrite shared indexes/logs/profile files. A coordinating or integrating agent may apply these shared updates after branches or task outputs converge, within the same overall change/integration unit.
- When paths change, reconcile all inbound links, index inventories, log entries and supersession references against the **integrated tree**, not just individually valid branches.
- Re-check document conformance, resolved typed edges and applicable graph integrity on the combined result. An individually passing subtask cannot establish integrated consistency.
- If two documents make conflicting semantic claims, report the substantive conflict to its owning authority; do not fix it by relabeling graph edges or manufacturing extra links.

## Compatibility and version limits

OKF is a **format**, not a mandate for a specific server profile or document hierarchy. The published v0.2 format keeps only `type` generally required; optional provenance, verification and lifecycle metadata are consumer signals, not additional rejection gates. `log.md` metadata requirements, index completeness, tag styling and graph cardinalities can be valid **repository rules** but must not be described as universal OKF conformance.

The upstream specification treats leading `/` links as **bundle-relative**, while this server's documented graph resolver treats them as **scan-root-relative**. Prefer verified link forms and inspect actual graph resolution when nested bundles are involved; do not silently translate one convention into the other. This skill does not change server behavior or certify upstream v0.2 support.

## Verify the distinct claims

1. Run `validate_doc` on changed documents/reserved files and the applicable corpus. Single-file `known_types` can describe the local vocabulary; bundle validation derives its vocabulary from indexed types. Neither proves substantive content truth.
2. Check profile selection, typed/inverse edges, and `graph_integrity` for applicable type/cardinality constraints. Inspect findings and relevant dangling references; do not discard them to obtain a clean result. Profile integrity is **not** enforced by `validate_doc` or CLI `--validate`.
3. Independently compare the actual bundle inventory with index links, and the changed-file list/current date with log entries. Implemented validation does not prove index completeness, log currency, tag style, timestamps, or correct bundle membership.
4. Keep validation scopes explicit. A native skill metadata file can be correct for its harness yet fail OKF document validation if wrongly included. Validate document corpora, not arbitrary repository Markdown; do not modify valid native metadata solely to silence the wrong validator.
5. Exercise a representative real relationship and a useful near-miss (such as a missing required link or wrong target type). Report observed results separately from proposed examples, unexecuted checks, and unavailable tools. No MCP access means verification remains unproven, not passed.

Return changed paths, bundle/index/log reconciliation, profile selection, observed conformance and graph results, and any unresolved boundary. Keep substantive correctness claims outside this skill's remit.
