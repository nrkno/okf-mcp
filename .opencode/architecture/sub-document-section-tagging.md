# Sub-Document Section Tagging: Evaluation

> Design evaluation — okf-mcp, 2026-07-15

## Problem

`get_doc` returns the full markdown body of the best-matching document. For focused,
single-topic files (50–300 lines) this is correct and sufficient. For large multi-topic
reference files (500+ lines covering 10 endpoints) the agent receives everything when
it only needed one section — a token-cost and signal-to-noise problem.

The question is whether adding tagged sections inside a markdown file is the right fix,
and if so, which mechanism (mid-document YAML block vs HTML comment annotation).

## Constraints

- No embeddings, no vector DB, no RAG, no chunking pipeline.
- Frontmatter/structure only — must stay consistent with existing design principles.
- No new external dependencies (go stdlib + mcp-go + gopkg.in/yaml.v3 only).
- Complexity budget is real: the server is simple, tested, and well-understood.
- Frontmatter quality is a functional requirement — bad metadata = bad results.

## Invariants and Guarantees (relevant subset)

| # | Invariant | Current owner | Status under section tagging |
|---|-----------|--------------|------------------------------|
| I-2 | content is a live disk read, not cached | getDocHandler | Preserved |
| I-3 | type field required or file skipped | parser.Parse | Unchanged |
| I-6 | tie-break alphabetical by file_path | matcher.FindBest | Must extend to sections |
| NEW | section content is a contiguous slice of body | parser | Precondition: boundaries detected correctly |
| NEW | returned section is best-scoring unit | matcher | Precondition: sections scored independently |

New failure mode: **false-section boundary** — parser detects a section start but
misidentifies its end (nested `---` in a code fence; multiline comment). Returned
`content` is a wrong slice with no agent-visible indication.

## Options Considered

### Option 1 — Mid-document YAML block (`---` delimiter)

A second `---`-delimited YAML block mid-document acts as a section marker.

```markdown
---
title: Full API Reference
type: reference
tags: [api, reference]
---
...intro...

---
tags: [rate-limiting, api]
---
## Rate Limiting
...section content...
```

**Server changes required:** parser detects non-frontmatter `---` lines as section
boundaries; each section becomes an index sub-unit with synthetic title (next heading)
and section tags; matcher scores sections as independent units; `get_doc` response gains
`section_title` and `section_content` fields or overloads `content`.

**Pros:**
- `---` is syntactically unambiguous; easy to split on.
- YAML is already parsed by the server — no new parser technology.
- Mirrors the frontmatter convention authors already know.

**Cons:**
- **`---` mid-document IS a `<hr>` in CommonMark.** GitHub and every standard renderer
  display a horizontal rule at every section boundary. The document looks visually broken.
- Tags-only sections have weak scoring signal. File-level title (3×) will almost always
  outscore a section's tags (2×) — sections dominate only when the file title is vague.
- Authors face two incompatible meanings for `---`: frontmatter at top, hr mid-doc.
- A `---` inside a code fence (common in docs) is misidentified as a boundary. Fence
  awareness significantly increases parser complexity.

**Verdict: Excluded.** Rendering breakage alone disqualifies a documentation system.

---

### Option 2 — HTML comment annotation

An HTML comment before a heading carries the section's tags.

```markdown
<!-- tags: [rate-limiting, api] -->
## Rate Limiting
...section content...
```

**Server changes required:** parser scans for `<!-- tags: [...] -->` before headings;
each annotated heading becomes a section sub-unit; section content is the slice from
heading to next annotated heading or EOF; matcher scores sections independently.

**Pros:**
- HTML comments are invisible in rendered markdown — document reads cleanly on GitHub.
- Convention is recognisable to MDX/Docusaurus users.
- Does not collide with existing markdown syntax.

**Cons:**
- Parsing is fragile: must be single-line, exact whitespace, not inside code fence or
  blockquote. The parser must handle all these cases or produce false boundaries.
- Tags alone give weak scoring signal — same problem as Option 1. The section scorer
  gets only tags, while the file scorer gets title + tags + description. The file wins.
- Non-standard convention — no tooling support (no linter, no renderer highlight).
- Response shape must change: `section_title` (from heading text) and `section_content`
  (the slice) must be added, OR `content` is overloaded — breaking the current contract.

**Verdict: Not excluded outright, but the scoring-signal problem and parser fragility
make this unattractive unless the corpus demonstrably has large indivisible files.**

---

### Option 3 — Document discipline (split large files, no server change)

No server change. The fix is corpus-side: large multi-topic reference files are split
into focused single-topic files, each with well-authored frontmatter.

```
docs/api-reference.md          (500 lines, 10 endpoints)
→ docs/api-rate-limiting.md    (50 lines, rate-limiting only)
→ docs/api-auth.md             (60 lines, auth only)
```

**Pros:**
- Extends the existing design principle cleanly: "frontmatter quality is a functional
  requirement." Discipline applies at authorship time, not retrieval time.
- Each focused file gets full scoring signal: title (3×), tags (2×), description (1×).
  Matching precision is strictly BETTER than section tags (which have only tags, 2×).
- Zero server complexity added. Zero new failure modes. Zero new test surface.
- `list_docs` becomes more useful — each topic has its own discoverable entry.
- Cross-references between files are explicit links, not implicit section boundaries.

**Cons:**
- Requires corpus authors to restructure existing large files. One-time migration cost.
- Does not help for genuinely indivisible large files (rare for OKF documentation).

**Verdict: Recommended.** See recommendation section.

---

### Option 4 — Deferred: `section` param on `get_doc` (heading-anchored slice)

A `section` string parameter lets the caller request a specific heading anchor after
the file match. No new annotation syntax; uses existing heading structure.

```
get_doc(topic="API reference", section="rate-limiting")
```

**Pros:** No new corpus authoring convention. Backward compatible when `section` absent.

**Cons:** Two-call pattern (discover headings, then retrieve section). Agent must predict
heading text without metadata support. Heading-level parsing adds complexity.

**Verdict: Listed as a future escape hatch. Not recommended now.**

## Recommendation

**Do Option 3 (document discipline) now. Do not build section tagging.**

### Primary reason: the scoring-signal problem makes both annotation options weaker than the file-level design

The current scorer weights title (3×), tags (2×), description (1×). A section annotation
adds only tags (2×) — no title, no description. This means:

- A file-level match on title will almost always outscore a section-level match on tags.
- The section index is dominated by its parent file. The agent gets the file anyway;
  the section tag gives marginal additional signal.
- To make sections score competitively, each needs its own title + description — which
  is exactly what a focused file provides, at zero server complexity cost.

**Authoring a focused file is strictly more expressive than authoring a section tag.**
The document-discipline option delivers the same retrieval precision with better scoring
signal, less server complexity, and no new corpus authoring convention to maintain.

### Secondary reason: Option 1 is disqualified by rendering breakage

`---` mid-document renders as `<hr>` in every CommonMark renderer. A documentation
system that makes docs look broken in GitHub is not viable — full stop.

### Secondary reason: Option 2's parser fragility

HTML comment parsing in arbitrary markdown is fragile. Code fences, blockquotes,
multiline comments, and whitespace variations all create false boundaries without
fence-awareness that would make the parser nearly as complex as a full markdown parser.
That is the opposite of the design principle "simple, tested, well-understood."

### What document discipline looks like in practice

1. Identify files in the OKF corpus that exceed ~200 lines AND cover more than one
   distinct topic.
2. Split each into focused files, one per topic.
3. Give each new file full frontmatter: `type`, `title`, `description`, `tags`.
4. `get_doc` results immediately improve — no server change required.

### When to revisit

Revisit Option 2 or Option 4 ONLY when BOTH of these conditions hold:
- The corpus contains large files that CANNOT be split (generated or externally owned),
  AND agents are visibly returning wrong or over-large content for those files specifically.

This is an observed-pain trigger, identical in spirit to the Option D trigger in
`get-doc-response-shape.md`.

## Input/Operation Coverage

| Input shape | list_tags | list_docs | get_doc now | get_doc after discipline |
|-------------|-----------|-----------|-------------|--------------------------|
| Focused file ≤200 lines | handled | handled | handled | handled |
| Large file 200–500 lines (splittable) | handled | handled | over-provisioned | handled |
| Large file 500+ lines (splittable) | handled | handled | over-provisioned | handled |
| Genuinely indivisible large file | handled | handled | over-provisioned | over-provisioned (deferred) |
| Section-tagged file (Option 2) | n/a | n/a | n/a | out-of-scope |
| Non-OKF file (no frontmatter) | out-of-scope | out-of-scope | out-of-scope | out-of-scope |

The "over-provisioned / deferred" cell (genuinely indivisible large file) is not a
blocking gap — it is an accepted residual. Option 4 can address it if it materialises.

## Security Threat Model

No new security surface. Option 3 is a corpus authoring discipline with no server
changes. The existing threat model from `get-doc-response-shape.md` is unchanged.

If Option 2 were built (not recommended), an additional threat would appear:
T: malicious `<!-- tags: [...] -->` comment could attempt to poison the section index
(inject tags, create false boundaries). Mitigation: strict pattern matching and tag
validation. Not applicable since Option 2 is not recommended.

## Anchor Check

- **Minimal version** reached: Option 3 requires no server changes — the minimal
  possible intervention. ✓
- **Not this**: No RAG, no embeddings, no chunking, no new parser technology. ✓
- **All elements trace to stated Forces:**
  - "Frontmatter quality is a functional requirement" → Option 3 extends this cleanly.
  - "Complexity budget" → Option 3 adds zero server complexity.
  - "What does the agent actually need?" → a precise match; focused files deliver this
    better than section tags (full scoring signal vs tags only).
- No speculative machinery added.

## Open Questions

1. **Does the OKF corpus actually have large multi-topic files today?** If not, this
   is a hypothetical problem and even Option 3 has no immediate work. The recommendation
   holds either way (do not build section tagging), but the urgency of corpus splitting
   depends on the answer.

2. **Are any OKF files genuinely indivisible?** If generated files from external sources
   enter the corpus with OKF frontmatter, Option 4 (section param) becomes relevant.
   Track as an observability signal: log `len(content)` per `get_doc` call at DEBUG
   level (already recommended in `get-doc-response-shape.md`).

3. **Who owns the corpus split?** Splitting a file is a documentation authoring task,
   not a server engineering task. If there is no owner for OKF doc quality, the
   document-discipline option requires assigning one before any corpus work begins.

## Deferred / Excluded — and the Guarantee Each Breaks

| Exclusion | Guarantee weakened | Acceptable? |
|-----------|--------------------|-------------|
| Option 1 excluded | None — section tagging is not a current invariant | Yes — rendering breakage disqualifies |
| Option 2 not built | None — same | Yes — revisit on observed pain with genuinely indivisible large files |
| Option 4 deferred | Agent cannot retrieve sub-section of a large indivisible file | Yes — no such file in corpus yet |
| Section scoring signal | Section-level precision not achievable without full metadata per section | Accepted — focused files deliver equal precision at zero server complexity |
