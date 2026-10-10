# OKF authoring assessment

Assess formatting, structure, field conformance and applicable repository-profile rules. **Do not judge substantive product/technical content correctness, or invent relationships/cardinality obligations.** Accepted meaning is input to this assessment, not a conclusion it supplies.

## Establish the applicable surface

- Inspect repository authoring rules, selected profile/version and actual scan root. Read direct `references/okf-standard.md` and `references/profile-format.md` as shipped reference material; prefer current target rules/server evidence when they differ.
- Build the affected-file inventory from files plus OKF discovery. Check nearest-index bundle ownership and nested boundaries; do not use `list_docs` alone, which omits missing-type documents and reserved files.
- Classify ordinary corpus documents, `index.md`, `log.md`, native skill metadata, companions and fixtures before choosing validation. Do not impose document frontmatter on every Markdown asset or frontmatter on `index.md`. Native `SKILL.md` needs its harness-appropriate discoverability metadata, not an invented OKF type.

## Inspect artifacts, not assurances

| Surface | Required assessment evidence |
|---|---|
| Ordinary documents | Valid YAML/non-empty type, appropriate field shapes and local vocabulary, useful retrieval fields, applicable tag conventions and current edit timestamp. Missing type can be an indexing omission rather than a tool error. |
| `index.md` | No frontmatter; readable Markdown link inventory. Compare every applicable bundle document against actual links and targets, including the governing log where applicable. Detect omitted, stale, renamed, duplicate and mis-owned entries. Nested bundles are assessed against their own inventories, not silently flattened. |
| `log.md` | Correct `type: Log` metadata, applicable retrieval fields/timestamp, newest-first date headings and repository action format. Compare today's changed-document list to current-date entries, not just the existence of an old valid log. |
| Profile | Loader accepts the intended file; explicit selection or discovery is observed, not inferred. Fields/directions/bounds/aliases match supported format. Check exact type allow-lists and cardinality selectors against already-accepted obligations and actual graph outputs. Do not decide whether the underlying claims are true. |
| Relationship representation | Existing targets, intended nearest-heading classification, supported file-path forms and matching forward/inverse query names. An `untyped` edge or fallback default can hide an intended rule; inspect selection and required-link counts. |
| Native skill/reference assets | Correct asset classification; direct references resolve inside the installed skill directory. Shipped schema snapshots are synchronized with their maintained source and version applicability is explicit, not independently rewritten tables. |

## Interpret verification narrowly

For a formatting-only direction check, trace one already-accepted relationship from the document containing the link to its actual target. Compare their exact frontmatter `type` values with `allowed_source_types` and `allowed_target_types` respectively; flag accidentally swapped lists even if loading succeeds. For `requirement.md → derived-from → anchor.md`, `[anchor]` as source and `[requirement]` as target are reversed. An incoming `derives` query does not excuse that reversal. Check invariant `source_type` separately as the checked-node selector: an incoming rule about anchors selects `anchor` and names forward `derived-from`, not inverse `derives`. Assess representation against accepted meaning, never whether the derivation itself is substantively justified.

`validate_doc`/CLI checks frontmatter and reserved-file conformance. Existing index validation checks absence of frontmatter, **not completeness**. Log validation checks required Log metadata and gives notifications for link/order issues, **not current-date coverage**. Timestamp/tag-style/bundle-inventory obligations require artifact inspection. Unknown-type findings depend on supplied/derived vocabulary.

`graph_integrity` reports profile type/cardinality constraints separately. Profile-load acceptance is not proof that all names/selectors express the intended rule; graph findings are not automatic commit blockers or CLI conformance failures. Require observed tool output or explicitly mark unavailable proof. Do not accept a successful document validator as a substitute for these distinct claims.

State concrete structural discrepancies by file/rule and distinguish tool-enforced findings from human/skill obligations. If accepted content meaning is missing or conflicting, report that dependency without using this formatting assessment to approve, reject or rewrite substantive meaning. No extra governance, permission or gate is created by this companion.
