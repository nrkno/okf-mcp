# OKF authoring assessment

Assess formatting, structure, field conformance and applicable repository-profile rules. **Do not judge substantive product/technical content correctness, or invent relationships/cardinality obligations.** Accepted meaning is input to this assessment, not a conclusion it supplies.

## Establish the applicable surface

- Inspect repository authoring rules, selected profile/version and actual scan root. Read direct `references/okf-standard.md` and `references/profile-format.md` as shipped reference material; prefer current target rules/server evidence when they differ.
- Build the affected-file inventory from files plus OKF discovery. Check nearest-index bundle ownership and nested boundaries; do not use `list_docs` alone, which omits missing-type documents and reserved files.
- Classify ordinary corpus documents, `index.md`, `log.md`, native skill metadata, companions and fixtures before choosing validation. Do not impose OKF frontmatter on native skill assets. This repository's `index.md` convention forbids frontmatter; upstream v0.2 permits a bundle-root version declaration. Check which convention and validator are actually supported rather than declaring the upstream exception invalid.

## Inspect artifacts, not assurances

| Surface | Required assessment evidence |
|---|---|
| Ordinary documents | Valid YAML/non-empty type, appropriate fields and vocabulary, useful retrieval fields, applicable tag/time conventions. Optional OKF v0.2 provenance/trust/lifecycle fields are not new mandatory conformance gates. Missing type can be an indexing omission rather than a tool error. |
| `index.md` | Conformity with the selected repository rule (no frontmatter here; upstream v0.2 root-index version exception may need different tooling); readable Markdown link inventory. Compare actual bundle files against links and targets; detect omitted, stale, renamed, duplicate and mis-owned entries. Nested bundles are not silently flattened. |
| `log.md` | This repository's `type: Log` metadata when applicable, newest-first date headings and repository action format. Upstream OKF does not universally require `type: Log`. Compare changed-document inventory and current-date entries, not merely the existence of an old valid log. |
| Profile | Loader accepts the intended file; explicit selection or discovery is observed, not inferred. Fields/directions/bounds/aliases match supported format. Check exact type allow-lists and cardinality selectors against already-accepted obligations and actual graph outputs. Do not decide whether the underlying claims are true. |
| Relationship representation | Existing targets, intended nearest-heading classification, supported file-path forms and matching forward/inverse query names. An `untyped` edge or fallback default can hide an intended rule; inspect selection and required-link counts. |
| Native skill/reference assets | Correct asset classification; direct references resolve inside the installed skill directory. Shipped schema snapshots are synchronized with their maintained source, with v0.1 repository rules distinguished from the current upstream v0.2 specification. |

## Assess consumption and concurrency

- **Targeted retrieval:** Given an implementation question, verify the agent identified relevant documents through topic/type/tag search and bounded graph navigation, then read the selected authoritative sources. A high-scoring single `get_doc` result is not a complete inventory.
- **Authority:** Check that the agent distinguished accepted intent, proposals, superseded material, current code, and tests. A valid graph edge or optional `verified` field is not direct proof of product correctness.
- **Missing index items:** Compare OKF discovery with known files and scanner visibility; do not accept an empty search as proof of no documentation.
- **Independent handoffs:** Verify that the selected source paths, shared contracts, constraints, open semantic questions, and verification expectations can be understood without the originating chat.
- **Parallel updates:** Check that agents did not overwrite shared indices/logs/profile, that ownership of their final reconciliation is explicit, and that integrated link/index/log checks were actually performed on the merged result where available.
- **Version boundary:** Apply target repository and server capabilities as observed. Do not enforce optional OKF v0.2 metadata as required or claim older graph/link behavior implements upstream bundle-root resolution.
- Assess the **quality of retrieval and representation**, not whether the actual semantic requirements are wise or fulfilled.

## Interpret verification narrowly

For a formatting-only direction check, trace one already-accepted relationship from the document containing the link to its actual target. Compare their exact frontmatter `type` values with `allowed_source_types` and `allowed_target_types` respectively; flag accidentally swapped lists even if loading succeeds. For `requirement.md → derived-from → anchor.md`, `[anchor]` as source and `[requirement]` as target are reversed. An incoming `derives` query does not excuse that reversal. Check invariant `source_type` separately as the checked-node selector: an incoming rule about anchors selects `anchor` and names forward `derived-from`, not inverse `derives`. Assess representation against accepted meaning, never whether the derivation itself is substantively justified.

`validate_doc`/CLI checks **this implementation's** frontmatter and reserved-file rules. Its index validation checks absence of frontmatter, **not completeness**; its log validation checks this repository's required Log metadata and gives notifications for link/order issues, **not current-date coverage**. These implementation checks do not define the upstream OKF v0.2 minimum. Timestamp/tag-style/bundle-inventory obligations require artifact inspection. Unknown-type findings depend on supplied/derived vocabulary.

`graph_integrity` reports profile type/cardinality constraints separately. Profile-load acceptance is not proof that all names/selectors express the intended rule; graph findings are not automatic commit blockers or CLI conformance failures. Require observed tool output or explicitly mark unavailable proof. Do not accept a successful document validator as a substitute for these distinct claims.

State concrete structural discrepancies by file/rule and distinguish tool-enforced findings from human/skill obligations. If accepted content meaning is missing or conflicting, report that dependency without using this formatting assessment to approve, reject or rewrite substantive meaning. No extra governance, permission or gate is created by this companion.
