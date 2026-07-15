# `get_doc` Response Shape: Inline Content vs. File-Path Only

> Design evaluation — okf-mcp post-ship review, 2026-07-15

## Problem

`get_doc` currently returns `{title, description, tags, file_path, content}` where `content` is the entire raw file verbatim (frontmatter YAML + markdown body). The question is whether that shape is right for the hub-and-spoke agent pattern, or whether returning only `file_path` plus metadata is sufficient and less wasteful.

This is a cost/benefit question: does the current inline-content design impose a token tax that exceeds the value it provides?

## Constraints

- stdio MCP server; no streaming, no partial responses in current mcp-go SDK
- No new dependencies — implementable with existing Go stdlib + mcp-go
- Must not break the `file_path` write-back use case regardless of option chosen
- Invariant I-2 (live disk read for freshness) is satisfied as long as content is read at call time
- Consumer is opencode hub + spokes; agents have Read, Grep, Glob file tools available in session
- Typical doc size: 50–300 lines; reference docs may exceed this

## Invariants & Guarantees (relevant subset)

| # | Invariant | Component | Falsifiable by |
|---|-----------|-----------|----------------|
| I-1 | `file_path` in response is repo-relative, not absolute | `getDocHandler` | Check that `file_path` does not start with `/` |
| I-2 | `content` (when returned) is a live disk read, not cached index copy | `getDocHandler` | Mutate file after indexing, call `get_doc`, verify mutation appears |
| I-7 | Response never contains stale content from prior index snapshot | `idx.Rebuild()` before every call | Same as I-2 |

If `content` is removed from the response, I-2 and I-7 become vacuously true — the agent's own `Read` call provides freshness. No invariant is broken by removing `content`; the live-read machinery simply becomes unnecessary.

## Options Considered

### Option A — Keep full content inline (status quo)

**How it works:** `content` field = raw file bytes as UTF-8 string. Entire file, frontmatter included, in every `get_doc` response.

**Pros:**
- Single tool call gives the agent everything. Zero extra round-trips.
- True abstraction: agent never touches the filesystem for docs.
- Works even if the agent lacks file-read permission (in practice it always does).

**Cons:**
- Token cost scales linearly with doc size x call frequency. Three calls to 200-line docs = 9–15K tokens injected into context before any code is read.
- Frontmatter bytes transmitted twice: once as raw YAML in `content`, once as structured `title`/`description`/`tags` fields.
- Hub's primary use of `get_doc` is building `Prior Findings` for spoke dispatch prompts. Embedding 5K of raw doc in a dispatch prompt is wasteful — the hub must trim in-context, burning tokens.
- Large docs will eventually overflow context in a heavy session.

**Risks:** Discovery patterns (agent calls `get_doc` multiple times to find the right doc) inject all rejected docs into context permanently.

---

### Option B — Return `file_path` only (no content)

**How it works:** Response is `{title, description, tags, file_path}` only. Agent reads the file itself if it needs the body.

**Pros:**
- Zero token cost for content. Agent pays only for what it actually reads.
- Hub can pass `file_path` to a spoke, which reads only the relevant section.
- Natural fit for the discovery pattern: list_docs → get_doc (confirm by metadata) → Read (body, possibly partial).

**Cons:**
- Leaky abstraction: agent is back to file I/O after calling `get_doc`. MCP server's value is reduced to "a search index that returns a path."
- Extra tool call adds latency and one step from the step budget per invocation.
- Agent must know it needs to follow up with `Read`. If MCP instructions don't say this explicitly, agents may act on `description` (a one-liner) and miss the body.
- MCP server instructions currently say "use get_doc to retrieve the relevant document" — "retrieve" implies content. Dropping content requires updating instructions or agents will be confused.

**Risks:** Silent false-negative failure mode — agent acts only on `description` without reading body, produces an answer from incomplete information. Hard to detect because agent still produces an answer.

---

### Option C — Return metadata + content with frontmatter stripped

**How it works:** Same as A, but `content` is the markdown body only (everything after the closing `---` delimiter). The parser already locates this boundary.

**Pros:**
- Eliminates frontmatter duplication (5–15 lines per call, not the primary saving but purely wasteful).
- `content` is the prose the agent actually wants — not the YAML header it already has in structured form.
- Non-breaking change: `content` is still present, just cleaner.
- One-line change to `getDocHandler` once `BodyOffset` is surfaced from the parser.

**Cons:**
- Does not address the fundamental token-cost concern — still sends the full body.
- Minor implementation change: `BodyOffset int` must be added to `parser.Doc` to expose the boundary cleanly, or the handler re-slices the raw bytes (fragile duplication of parser logic).

**Risks:** Low. Conservative change, same semantics for consumers.

---

### Option D — Caller-controlled via `include_content` bool param (default `false`)

**How it works:** `get_doc` gains an optional `include_content` boolean. When absent or `false`, returns metadata only. When `true`, returns metadata + content (as in C).

**Pros:**
- Agent decides per call. Discovery/routing calls are cheap; confirmed-relevant reads are complete.
- Default `false` is conservative on tokens.
- Most principled long-term option.

**Cons:**
- Breaking change if default is `false` (current callers get no content without opting in).
- Agents that don't read the param description call `get_doc` without `include_content=true`, see no `content` field, and silently act on metadata only — same false-negative risk as Option B, but more hidden.
- Adds interface complexity before pain is observed.
- Integration test must cover both code paths.

**Risks:** Default-value choice is irreversible without another breaking change. Wrong default = token waste (if `true`) or silent missing-content failures (if `false`).

---

### Option E — Return metadata + content preview (first N lines)

**How it works:** `content` is truncated at a fixed boundary (e.g. first 50 lines or 2000 chars) with a `truncated: true` flag when cut.

**Pros:** Token budget bounded per call. Agent can confirm relevance before committing to a full read.

**Cons:**
- Arbitrary truncation point with no semantic justification.
- Worst failure mode: agent acts on truncated content as if it were complete. Partial content with false confidence is worse than no content.
- `truncated` flag agents must check; most won't.

**Risks:** High for the partial-confidence failure mode. Excluded.

## Recommendation

**Option C now. Option D only if pain is observed.**

### Immediate: Option C — strip frontmatter from `content`

Do this now. It is a conservative, non-breaking mechanical change that eliminates purely redundant bytes (frontmatter transmitted twice every call) and improves content quality (agent receives clean prose, not prose + redundant YAML header it already has in structured fields).

**Implementation:**
1. Add `BodyOffset int` to `parser.Doc` — the byte offset of the first character after the closing `---\n` delimiter. The boundary is already computed in `findClosingDelimiter`; surface it.
2. In `getDocHandler`, slice `content[doc.BodyOffset:]` before marshalling. The existing `os.ReadFile` call is unchanged.

**Resulting response shape (non-breaking):**
```json
{
  "title": "GitHub Rate Limiting",
  "description": "One-liner from frontmatter.",
  "tags": ["rate-limit", "github"],
  "file_path": "docs/rate-limiting.md",
  "content": "# GitHub Rate Limiting\n\nThe GitHub API enforces..."
}
```

`content` is still present. Current callers are unaffected. The only observable difference is that `content` no longer begins with `---\ntitle: ...\n---\n`.

### Conditional: Option D — add `include_content` param

Add only when ONE of these conditions is observed in practice:

1. Context-window pressure attributed to `get_doc` responses (agent hitting limits or the hub explicitly trimming content before embedding in dispatch prompts).
2. Discovery patterns with 3+ `get_doc` calls per session visible in tool call logs.
3. A reference doc significantly larger than 300 lines enters the corpus and agents are seen struggling with context.

When triggered: add `include_content bool`, default `true` (preserving current behaviour). Flip default to `false` only in a deliberate major version bump with clear migration note.

### Why not Option B now?

Option B solves the token problem but creates a reliability problem. The failure mode — agent acts only on the one-line `description` without reading the body — is **silent and hard to detect**. The agent produces an answer; it is just from incomplete information. The token savings are real but do not outweigh the risk of normalising a pattern where agents miss doc content.

Additionally: the hub injects `get_doc` results into spoke dispatch prompts. If content is absent, the hub must decide whether to `Read` the file and embed it. That is a worse abstraction leak than the current one — the hub ends up doing what `get_doc` was supposed to do.

### Why not Option D now?

No observed pain yet. The server is freshly shipped. Adding a param introduces a test surface, documentation, and a default-value decision that cannot be undone without a breaking change. Reserve for confirmed pain.

### Why not Option E?

Partial content with false confidence is worse than either full content or no content. Excluded unconditionally.

## Input/Operation Coverage

| Input shape | list_tags | list_docs | get_doc current (A) | get_doc post-C |
|-------------|-----------|-----------|----------------------|----------------|
| OKF doc 50–200 lines | handled | handled | handled (token mild) | handled |
| OKF doc 200–500 lines | handled | handled | degraded (token cost notable) | degraded (lower — no frontmatter) |
| OKF doc 500+ lines | handled | handled | degraded (token cost high) | degraded (somewhat lower) |
| Non-OKF file (no frontmatter) | handled | handled | out-of-scope (filtered by index) | out-of-scope |
| Hub embeds result in dispatch prompt | n/a | n/a | degraded (hub must trim) | degraded (hub trims less) |
| Spoke needs only a specific section | n/a | n/a | over-provisioned | over-provisioned |
| Write-back use case (file_path only) | n/a | handled | handled | handled |

**Cells marked "degraded"** are functional but inefficient. None are broken. The Option D trigger watches for these cells to surface as operational pain before adding interface complexity.

## Security Threat Model

The MCP server reads files from disk within the indexed repository root. `content` exposes file bytes to the calling agent.

| Boundary | Asset | STRIDE threat | Mitigation |
|----------|-------|--------------|------------|
| Disk → `content` field | Raw file bytes | T: malicious frontmatter attempts prompt injection via `content` string | `content` is an opaque string to the MCP layer; no eval or interpolation occurs. Residual: consumer LLM agent may interpret embedded instructions. Accepted — consumer is a trusted agent in the same trust domain. |
| Disk → `content` field | Secrets in doc body | I: doc may contain example secrets (tokens, passwords) | OKF docs are documentation by convention; secrets do not belong there. No technical mitigation in this design. Accepted risk — a doc authorship policy concern, not an MCP design concern. |
| Index → response | `file_path` path traversal | T/I: `file_path` could point outside repo root | `file_path` is set from scanner output which walks only under `cwd`. `filepath.Join(idx.Dir(), doc.FilePath)` cannot escape via normal paths. Residual: crafted symlink could escape. Accepted — local dev tool, not exposed to untrusted callers. |

Option C (frontmatter stripped) **marginally reduces** information-disclosure surface: secrets embedded in frontmatter comments would not appear in `content`. Minor benefit, not a primary motivation.

## Anchor Check

- **Minimal version**: Option C is implementable with no new dependencies — existing parser state + stdlib string slicing. Reaches the floor. ✓
- **Not this**: No RAG, no embeddings, no chunking. Option C is a response-shape trim, not a retrieval overhaul. ✓
- Option D (deferred) also has no new dependencies. ✓
- Every element traces to a stated Force in the brief. No speculative machinery added.

## Open Questions

1. **`BodyOffset` field placement**: Should `BodyOffset int` be added to `parser.Doc` (cleanest — keeps boundary knowledge in the parser that already computes it) or computed inline in `getDocHandler` by re-slicing raw bytes (no parser change, but duplicates logic)? Parser-change is preferred.

2. **Observability trigger for Option D**: Who watches for the "pain" condition? Consider logging `len(content)` to stderr per `get_doc` call at DEBUG level — gives an observable signal without metric infrastructure.

3. **Large reference doc corpus growth**: Are docs expected to exceed 300 lines significantly? If yes, Option D becomes more urgent. If corpus stays focused/single-topic, Option C is sufficient indefinitely.

## Deferred / Excluded — and the Guarantee Each Breaks

| Exclusion | Guarantee weakened | Acceptable? |
|-----------|-------------------|-------------|
| Option D deferred | Hub cannot request content-free metadata during discovery — pays token cost it doesn't need. No invariant broken; hub ignores `content` field. | Yes — until pain is observed |
| Option B excluded | Silent false-negative: agents act on `description` only without reading body. No invariant broken, but product promise ("retrieve the document") is hollow if content is routinely ignored. | No — excluded unconditionally |
| Option E excluded | Partial-content false-confidence failure: agent acts on truncated content as if complete. No invariant broken, but reliability promise is degraded. | No — excluded unconditionally |
| Prompt-injection hardening | Content from docs reaches the agent's context window; a maliciously crafted doc could attempt to influence agent behaviour. Not mitigated technically. | Accepted for local dev tool with trusted agent consumer; must be re-evaluated if server is ever exposed to untrusted callers. |
