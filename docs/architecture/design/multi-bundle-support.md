---
type: design
title: Multi-Bundle Support for okf-mcp
description: Design for three coordinated changes to support multi-bundle OKF repositories in a single okf-mcp process --enable-hidden CLI flag, multi-log.md aggregation, and bundle field on document responses.
tags: [multi-bundle, scanner, cli, log, index, mcp]
---

## Problem

The okf-mcp server treats the entire cwd as one implicit bundle. Three limitations follow:

1. **Hidden directories are unconditionally skipped** (`internal/scanner/scanner.go:46-48`). A valid OKF bundle at `.opencode/architecture/` is invisible — the server cannot index it, validate it, or surface it in any tool response.
2. **`get_log` returns only the first `log.md` it finds** (`cmd/okf-mcp/main.go:388-398`). In a multi-bundle repo with `docs/log.md` and `.opencode/architecture/log.md`, entries from the second file are silently dropped.
3. **No bundle provenance on responses.** `list_docs`, `get_doc`, and `get_index` return flat lists with no indication of which OKF bundle a file belongs to. An agent querying a multi-bundle repo cannot distinguish `docs/architecture.md` from `.opencode/architecture/api-design.md` beyond their paths.

## Constraints

- Single CGO_ENABLED=0 Go binary, stdio MCP server, cwd is scan root.
- Six existing tools — no new tools added.
- Existing invariants I-1 through I-16 must be preserved or explicitly amended (not silently broken).
- Single-bundle repos must continue to work identically — the `bundle` field is additive.
- CLI flag only — no config file, no env vars.
- Tags stay global and deduplicated — no per-bundle tag grouping.
- The `--validate` CLI mode must compose with `--enable-hidden`.

## Invariants & Guarantees

### Amended invariants

| ID  | Current statement | New statement | Justification |
| --- | ----------------- | ------------- | ------------- |
| I-5 | Hidden directories (names starting with `.`) are never traversed | Hidden directories are skipped by default. When `--enable-hidden` is set, they are traversed **except** VCS internals (`.git`, `.hg`, `.svn`) which are always skipped. | The original I-5 is the default behavior; the flag is an opt-in relaxation. VCS dirs are never OKF bundles and contain hundreds of thousands of irrelevant files. |
| I-12 | `get_log` returns entries in reverse-chronological order with parsed date, action, and target | `get_log` returns entries from **all** `log.md` files in the index, merged in reverse-chronological order. Each entry carries a `source` field (relative path to its `log.md`). Ties broken by source path ascending, then document order. | The old first-wins behavior was a latent bug, not a documented contract. Multi-bundle requires aggregation. |

### New invariants

| ID  | Statement | Component | Precondition | Falsifiable |
| --- | --------- | --------- | ------------ | ----------- |
| I-17 | Every document response (`list_docs`, `get_doc`, `get_index` leaf) includes a `bundle` field: the relative path to the nearest ancestor directory containing `index.md`, or the file's immediate parent directory if no ancestor has one. | index | File is indexed | Call `list_docs` on a multi-bundle repo; verify each entry's `bundle` matches the walk-up rule |
| I-18 | `--enable-hidden` defaults to off. When off, scanner behavior is byte-identical to pre-flag behavior (all dot-dirs skipped). | scanner | Process starts | Run without flag; verify `.opencode/` files absent from index |
| I-19 | VCS directories (`.git`, `.hg`, `.svn`) are always skipped regardless of `--enable-hidden`. | scanner | Process starts with `--enable-hidden` | Run with flag; verify `.git/` files absent from index |

### Preserved invariants (unchanged)

I-1, I-2, I-3, I-4, I-6, I-7, I-8, I-9, I-10, I-11, I-13, I-14, I-15, I-16 — all preserved without modification.

## Design

### 1. CLI Flag Wiring

**Flag definition** in `cmd/okf-mcp/main.go:main()`:

```go
enableHidden := flag.Bool("enable-hidden", false,
    "Traverse hidden directories (except .git, .hg, .svn)")
```

Parsed alongside existing `--validate` and `--path` flags at `main.go:484-486`.

**Flow into scanner:** A new `scanner.ScanOptions` struct carries the flag:

```go
// internal/scanner/scanner.go
type ScanOptions struct {
    EnableHidden bool
}
```

`ScanAll` signature changes from `ScanAll(dir string)` to `ScanAll(dir string, opts ScanOptions)`. The existing `Scan(dir string)` wrapper passes `ScanOptions{}` (zero value = flag off = current behavior).

**Flow into Index:** The `Index` struct gains a `scanOpts` field:

```go
// internal/index/index.go
type Index struct {
    dir      string
    scanOpts scanner.ScanOptions  // NEW
    mu       sync.Mutex
    docs     []parser.Doc
    reserved []ReservedFile
}
```

`index.New` signature changes from `New(dir string)` to `New(dir string, opts scanner.ScanOptions)`. `Rebuild()` passes `idx.scanOpts` to `scanner.ScanAll`.

**`--validate --enable-hidden` composition:** `runValidate` at `main.go:527` creates a local index. With the flag, it passes `ScanOptions{EnableHidden: true}` to `index.New`. This means `--validate --enable-hidden` validates hidden bundles too — the correct behavior since the user wants to validate `.opencode/architecture/` docs.

**`--validate` without `--enable-hidden`:** unchanged behavior — hidden dirs skipped, only visible bundles validated.

### 2. Bundle Resolution Algorithm

**Location:** `internal/index/` package — a new unexported function `resolveBundle`. No new package needed: the function needs the reserved-file set (already in `Index`) and is called only from `Rebuild()`.

**Computed at `Rebuild()` time**, not on-demand at response time. Justification:
- The reserved-file set is fully known after `ScanAll` returns
- Computing per-response would re-walk the directory hierarchy on every API call
- Bundle is an intrinsic property of a file's position in the tree, not query-dependent

**Storage:** A new `Bundle` field on `parser.Doc`:

```go
// internal/parser/parser.go
type Doc struct {
    Title       string
    Description string
    Type        string
    Tags        []string
    FilePath    string
    BodyOffset  int
    Bundle      string  // NEW: relative path to nearest index.md ancestor dir
}
```

And on `TreeNode`:

```go
// internal/index/index.go
type TreeNode struct {
    Name     string     `json:"name"`
    Path     string     `json:"path"`
    Type     string     `json:"type"`
    DocType  string     `json:"doc_type,omitempty"`
    Title    string     `json:"title,omitempty"`
    Bundle   string     `json:"bundle,omitempty"`  // NEW: only on file/reserved leaves
    Children []TreeNode `json:"children,omitempty"`
}
```

**Algorithm pseudocode:**

```
func resolveBundle(filePath string, reservedSet map[string]bool, root string) string:
    // Build set of directories containing index.md
    // filePath is relative to root (e.g. "docs/sub/deep.md")
    
    dir := filepath.Dir(filePath)  // "docs/sub"
    
    // Walk up from dir toward root
    for d := dir; ; d = filepath.Dir(d):
        candidate := filepath.Join(d, "index.md")
        if d == ".":
            candidate = "index.md"
        if reservedSet[candidate]:
            return d               // e.g. "docs" if docs/index.md exists
        
        if d == "." || d == "":
            break
        parent := filepath.Dir(d)
        if parent == d:            // reached root
            break
    
    // Check root itself
    if reservedSet["index.md"]:
        return "."
    
    // Fallback: immediate parent directory
    return dir
```

**Edge cases resolved:**

| File | Reserved set contains | Bundle result | Reasoning |
| ---- | --------------------- | ------------- | --------- |
| `docs/arch.md` | `docs/index.md` | `"docs"` | Walk up from `docs/`, find `docs/index.md` |
| `docs/sub/deep.md` | `docs/index.md` | `"docs"` | Walk up from `docs/sub/` → `docs/`, find `docs/index.md` |
| `docs/index.md` (reserved) | `docs/index.md` | `"docs"` | Its own dir contains `index.md` |
| `docs/log.md` (reserved) | `docs/index.md` | `"docs"` | Walk up from `docs/`, find `docs/index.md` |
| `guide.md` (root file) | `index.md` | `"."` | Walk up reaches root, find `index.md` |
| `guide.md` (root file) | (no `index.md` anywhere) | `"."` | Fallback: `filepath.Dir("guide.md")` = `"."` |
| `.opencode/architecture/design.md` | `.opencode/architecture/index.md` | `".opencode/architecture"` | Walk up from `.opencode/architecture/`, find its `index.md` |
| `random/notes.md` | (no `index.md` anywhere) | `"random"` | Fallback: immediate parent |

**Root bundle naming:** `"."` — the standard Go relative-path representation of the current directory. Empty string would be ambiguous in JSON (looks like a missing field). `"<unnamed>"` would be a magic value. `"."` is what `filepath.Rel` returns for same-directory and is already used as the tree root node name (`index.go:176`).

### 3. Scanner Changes for Hidden Dirs

**Minimal diff to `internal/scanner/scanner.go`:**

```go
// NEW: always-skip list for VCS internals
var vcsDirs = map[string]bool{
    ".git": true,
    ".hg":  true,
    ".svn": true,
}

// CHANGED: ScanAll accepts options
func ScanAll(dir string, opts ScanOptions) (ScanResult, error) {
    // ...
    // Rule 1 CHANGED:
    if d.IsDir() && strings.HasPrefix(name, ".") {
        if vcsDirs[name] {
            return fs.SkipDir  // VCS always skipped
        }
        if !opts.EnableHidden {
            return fs.SkipDir  // Hidden skipped unless flag set
        }
    }
    // ... rest unchanged
}
```

**`Scan` wrapper** passes zero-value opts — backward compatible:

```go
func Scan(dir string) ([]string, error) {
    r, err := ScanAll(dir, ScanOptions{})
    // ...
}
```

**New test fixtures needed:**
- A temp dir with `.opencode/architecture/design.md` + `guide.md` — test with flag off (skipped) and flag on (indexed)
- A temp dir with `.git/some.md` + `guide.md` — test with flag on (`.git` still skipped)
- A temp dir with two bundles: `docs/index.md` + `docs/arch.md` + `.opencode/architecture/index.md` + `.opencode/architecture/design.md`

### 4. `get_log` Aggregation

**Merge algorithm:**

```
1. Collect all reserved files where filepath.Base == "log.md"
2. For each log.md:
   a. Read file from disk (I-2: live read)
   b. Strip frontmatter
   c. Parse entries via logparser.Parse(body)
   d. Tag each entry with source = log.md's relative path
3. Concatenate all entries into one slice
4. Sort: primary key = date descending, secondary key = source ascending,
   tertiary = document order (stable sort preserves per-file order)
5. Apply filters: since, action, limit (unchanged from current)
6. Return
```

**Sort implementation:** Replace the current insertion sort (`sortLogEntries` at `main.go:475-481`) with `sort.SliceStable` for clarity and correct multi-key sorting:

```go
sort.SliceStable(entries, func(i, j int) bool {
    if entries[i].Date != entries[j].Date {
        return entries[i].Date > entries[j].Date  // newest first
    }
    return entries[i].Source < entries[j].Source   // source alpha asc
})
```

`SliceStable` preserves document order for entries sharing both date and source — the tertiary tiebreak.

**Response type changes:** The `LogEntry` struct in `logparser` stays unchanged (it has no business knowing file provenance). A new response-only type in `main.go`:

```go
type logEntryJSON struct {
    Date   string `json:"date"`
    Action string `json:"action"`
    Target string `json:"target"`
    Detail string `json:"detail,omitempty"`
    Source string `json:"source"`
}

type logResult struct {
    Entries []logEntryJSON `json:"entries"`
    Note    string         `json:"note,omitempty"`
}
```

The top-level `Source` field is **removed** from `logResult` — it no longer makes sense with multiple sources.

**`source` path format:** Relative path without `./` prefix, matching the convention used by `filepath.Rel` throughout the codebase. Examples: `"docs/log.md"`, `".opencode/architecture/log.md"`, `"log.md"`.

**Empty index (no `log.md` files):** Returns `{"entries":[], "note":"no log.md found"}` — same note as current behavior, minus the now-removed top-level `source` field.

**Single `log.md`:** Entries carry `source: "docs/log.md"` (or wherever it lives). The response shape is the same as multi-source — no special case.

### 5. Response Shape Changes

#### `list_docs`

**Before:**
```json
[
  {
    "title": "Architecture",
    "description": "System design",
    "tags": ["design"],
    "file_path": "docs/arch.md"
  }
]
```

**After (single-bundle repo — `docs/` only):**
```json
[
  {
    "title": "Architecture",
    "description": "System design",
    "tags": ["design"],
    "file_path": "docs/arch.md",
    "bundle": "docs"
  }
]
```

**After (multi-bundle repo — `docs/` + `.opencode/architecture/`):**
```json
[
  {
    "title": "Architecture",
    "description": "System design",
    "tags": ["design"],
    "file_path": "docs/arch.md",
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

No other field changes. `title`, `description`, `tags`, `file_path` are untouched.

#### `get_doc`

**Before:**
```json
{
  "content": "# Architecture\n...",
  "file_path": "docs/arch.md",
  "tags": ["design"],
  "title": "Architecture",
  "description": "System design"
}
```

**After:**
```json
{
  "content": "# Architecture\n...",
  "file_path": "docs/arch.md",
  "tags": ["design"],
  "title": "Architecture",
  "description": "System design",
  "bundle": "docs"
}
```

No other field changes.

#### `get_index`

**Before (leaf node):**
```json
{
  "name": "arch.md",
  "path": "docs/arch.md",
  "type": "file",
  "doc_type": "Architecture",
  "title": "Architecture"
}
```

**After (leaf node):**
```json
{
  "name": "arch.md",
  "path": "docs/arch.md",
  "type": "file",
  "doc_type": "Architecture",
  "title": "Architecture",
  "bundle": "docs"
}
```

Directory nodes do NOT carry `bundle` — it is a leaf-only attribute. Reserved file leaves also carry `bundle`:

```json
{
  "name": "log.md",
  "path": "docs/log.md",
  "type": "reserved",
  "doc_type": "Log",
  "bundle": "docs"
}
```

#### `get_log`

**Before:**
```json
{
  "entries": [
    {
      "date": "2026-07-23",
      "action": "Update",
      "target": "docs/arch.md",
      "detail": "Added concurrency section"
    }
  ],
  "source": "docs/log.md"
}
```

**After (single log.md):**
```json
{
  "entries": [
    {
      "date": "2026-07-23",
      "action": "Update",
      "target": "docs/arch.md",
      "detail": "Added concurrency section",
      "source": "docs/log.md"
    }
  ]
}
```

**After (multi log.md):**
```json
{
  "entries": [
    {
      "date": "2026-07-23",
      "action": "Update",
      "target": ".opencode/architecture/api-design.md",
      "detail": "Initial design",
      "source": ".opencode/architecture/log.md"
    },
    {
      "date": "2026-07-23",
      "action": "Update",
      "target": "docs/arch.md",
      "detail": "Added concurrency section",
      "source": "docs/log.md"
    },
    {
      "date": "2026-07-20",
      "action": "Creation",
      "target": "docs/tools.md",
      "detail": "Tool reference",
      "source": "docs/log.md"
    }
  ]
}
```

Note: same-date entries sorted by source path ascending (`.opencode/...` before `docs/...`).

### 6. Invariant Renumbering

**Strategy: replace in-place, not sub-number.** I-5 and I-12 are restated to reflect the new behavior. Sub-numbering (I-5a) creates ambiguity about whether I-5 alone means the old or new rule. A replaced invariant with a clear log entry is the standard practice in this codebase's architecture documents.

**Updated AGENTS.md invariants section:**

```
- **I-5**: Hidden directories (names starting with `.`) are skipped by default;
  `--enable-hidden` opts in to traversing them, except VCS internals
  (`.git`, `.hg`, `.svn`) which are always skipped
- **I-12**: `get_log` returns entries from all `log.md` files in the index,
  merged in reverse-chronological order, each tagged with `source`
  (relative path to its log.md); ties broken by source ascending,
  then document order
- **I-17**: Every document response includes a `bundle` field: nearest ancestor
  dir containing `index.md`, or immediate parent dir if none
- **I-18**: `--enable-hidden` defaults to off; when off, scanner behavior is
  identical to pre-flag behavior
- **I-19**: VCS directories (`.git`, `.hg`, `.svn`) are always skipped
  regardless of `--enable-hidden`
```

### 7. Test Plan

#### Scanner tests (`internal/scanner/scanner_test.go`)

| Test function | What it asserts |
| ------------- | --------------- |
| `TestScan_HiddenDirOpencode_DefaultOff` (rename of existing `TestScan_HiddenDirOpencode`) | `.opencode/` files are NOT returned when `ScanOptions{}` (default). Current behavior preserved. |
| `TestScan_HiddenDirOpencode_FlagOn` | `.opencode/architecture/design.md` IS returned when `ScanOptions{EnableHidden: true}`. |
| `TestScan_HiddenDirGit_AlwaysSkipped` | `.git/some.md` is NOT returned even with `ScanOptions{EnableHidden: true}`. |
| `TestScan_HiddenDirHg_AlwaysSkipped` | `.hg/some.md` is NOT returned even with `ScanOptions{EnableHidden: true}`. |
| `TestScanAll_EnableHidden_ReservedInHiddenDir` | `index.md` and `log.md` inside `.opencode/architecture/` appear in `Reserved` (not `Docs`) when flag is on. |

#### Index tests (`internal/index/index_test.go`)

| Test function | What it asserts |
| ------------- | --------------- |
| `TestBundle_Resolution` | File at `docs/sub/deep.md` with `docs/index.md` in reserved → `Bundle == "docs"` |
| `TestBundle_Fallback` | File at `random/notes.md` with no `index.md` anywhere → `Bundle == "random"` |
| `TestBundle_RootFile` | File at `guide.md` with root `index.md` → `Bundle == "."` |
| `TestBundle_RootFileNoIndex` | File at `guide.md` with no `index.md` anywhere → `Bundle == "."` |
| `TestBundle_ReservedFileBundle` | `docs/log.md` with `docs/index.md` → reserved file's bundle is `"docs"` |
| `TestTree_TwoBundles` | Two bundles (`docs/` + `.opencode/architecture/`) with flag on → tree has both subtrees, leaf nodes carry correct `bundle` field |

#### Integration tests (`cmd/okf-mcp/main_test.go`)

**Test helper `newFixtureServer` and `ScanOptions` (M1 amendment):** The existing helper at `main_test.go:93` calls `index.New(dir)` which will not compile after the signature change to `index.New(dir, opts scanner.ScanOptions)`. **Approach chosen: update `newFixtureServer` to accept `opts scanner.ScanOptions` as a third parameter** — `newFixtureServer(t *testing.T, dir string, opts scanner.ScanOptions)`. Justification: the helper must change regardless (compilation break), and one parameterized helper avoids the drift risk of maintaining two variants. Existing call sites pass `scanner.ScanOptions{}` (zero value = flag off = current behavior). Tests that exercise hidden-directory bundles pass `scanner.ScanOptions{EnableHidden: true}`. The table below marks each test's opts in the **Opts** column.

| Test function | Opts | What it asserts |
| ------------- | ---- | --------------- |
| `TestListDocs_BundleField` | `EnableHidden: true` | Multi-bundle repo (`docs/` + `.opencode/architecture/`) → each entry in `list_docs` response has correct `bundle` field |
| `TestGetDoc_BundleField` | `EnableHidden: true` | `get_doc` response includes `bundle` field matching walk-up rule, tested against a doc in `.opencode/architecture/` |
| `TestGetIndex_BundleOnLeaves` | `EnableHidden: true` | `get_index` leaf nodes have `bundle` (including hidden-dir leaves); directory nodes do not |
| `TestGetLog_MultiSource` | `EnableHidden: true` | Two `log.md` files — one in a visible dir (`docs/log.md`) and one in a hidden dir (`.opencode/architecture/log.md`) → entries merged from both sources, each entry carries `source` (relative path to its `log.md`), sorted reverse-chronologically with source-path ascending as tiebreak. This fixture exercises the primary motivation for the feature: the hidden-dir `log.md` that was silently dropped under the old first-wins behavior. |
| `TestGetLog_SingleSource` | `{}` (default) | One `log.md` in visible dir → entries have `source` field, top-level `source` removed |
| `TestGetLog_NoLogMd` | `{}` (default) | No `log.md` files → `{"entries":[], "note":"no log.md found"}` |
| `TestValidate_HiddenBundle` | `EnableHidden: true` | `--validate --enable-hidden` validates docs in `.opencode/architecture/` |

## Input/Operation Coverage

| Input shape | `list_docs` | `get_doc` | `get_index` | `get_log` | `validate_doc` |
| ----------- | ----------- | --------- | ----------- | --------- | -------------- |
| Single-bundle repo (no flag) | handled — `bundle` = bundle dir | handled — `bundle` added | handled — leaf `bundle` added | handled — entries carry `source` | handled — unchanged |
| Multi-bundle repo (flag on) | handled — each doc tagged | handled — correct bundle | handled — both subtrees | handled — merged entries | handled — validates all bundles |
| Multi-bundle repo (flag off) | handled — hidden bundle invisible | handled — hidden bundle invisible | handled — hidden subtree absent | handled — only visible log.md | handled — only visible bundles |
| Root-level files with `index.md` | handled — `bundle: "."` | handled | handled | handled — `source: "log.md"` | handled |
| Root-level files without `index.md` | handled — `bundle: "."` (fallback) | handled | handled | handled | handled |
| Empty index (no docs) | handled — empty array | handled — error msg | handled — empty tree | handled — note msg | handled — 0 files |
| Reserved file in hidden dir (flag on) | n/a (reserved never in list_docs) | n/a | handled — type `reserved` | handled — aggregated | handled — ValidateReserved |

## Security Threat Model

No security-sensitive surface: the server is a stdio MCP process with no network boundary, no authentication, no secrets, and no PII. The scan root is the process cwd. The `--enable-hidden` flag relaxes a local filesystem traversal that was already unrestricted for non-hidden dirs. The only new surface is reading files from dot-directories the user explicitly opted into — the same trust model as the existing non-hidden scan.

## Anchor Check

**Minimal version reached:**
- `--enable-hidden` CLI flag: designed with scanner parameterization, Index integration, and `--validate` composition.
- Multi-`log.md` aggregation: merge algorithm, sort order, `source` field, response shape all specified.
- `bundle` field: resolution algorithm, storage location, response shapes for all three tools, edge cases all covered.
- I-5 and I-12 updated, I-17/I-18/I-19 added.

**`Not this` respected:**
- No new MCP tool — six tools unchanged.
- No `list_tags` per-bundle grouping — tags stay global.
- No new validation rules beyond hidden-bundle traversal.
- No CLI config file or env vars — flag only.
- No directory-layout migration.
- No compatibility shim for old `get_log` — new shape replaces it.
- No "smart" walk-up — mechanical rule applies uniformly.

**Elements traced to anchor:**
- VCS always-skip list (I-19): traces to the Force "a stranger has to trust it in five seconds" — traversing `.git` would produce thousands of noise files and undermine trust in the tool's output.
- `sort.SliceStable` replacement: traces to I-12's new multi-key sort requirement — the current insertion sort cannot express secondary sort keys cleanly.

## Open Questions

1. **Root bundle name when no `index.md` exists anywhere:** Recommended `"."` (the Go-standard relative path for cwd). Alternative: empty string `""` — but this looks like a missing field in JSON and would require `omitempty` removal to distinguish from "not computed." The `"."` choice is consistent with the tree root node name at `index.go:176`. **Recommendation: `"."`.**

2. **VCS always-skip list scope:** Currently `.git`, `.hg`, `.svn`. Should we add `.jj` (Jujutsu) or other emerging VCS directories? **Recommendation: ship with the three established ones, add others on demand.** The list is a one-line change.

3. **`Scan` function signature:** The thin wrapper `Scan(dir string)` currently passes zero-value opts. Should it also accept `ScanOptions`? **Recommendation: no.** `Scan` is used only in tests and the backward-compatible wrapper is sufficient. If a caller needs opts, they use `ScanAll` directly.

4. **`get_log` top-level `source` removal:** This is a breaking change to the response shape. Any existing consumer that reads `result.source` will get `undefined`. **Recommendation: accept the breakage.** The context brief explicitly says "No compatibility shim for the old single-`log.md` `get_log` behavior — the new shape replaces it." The server is at version 1.0.0 and the old behavior was a latent bug.

## Deferred / Excluded — and the guarantee each one breaks

| Exclusion | Guarantee affected | Acceptable? |
| --------- | ------------------- | ----------- |
| No per-bundle tag grouping in `list_tags` | Tags remain a flat global set — an agent cannot ask "what tags exist in bundle X?" | Yes — explicitly out of scope per anchor. Tags are a cross-cutting concern; per-bundle grouping would require a new tool or parameter. |
| No `bundle` field on `list_tags` response | Tags have no provenance | Yes — tags are deduplicated across bundles by design. |
| No new validation rules for hidden bundles | Hidden bundles are validated with the same rules as visible ones | Yes — the OKF standard applies uniformly. No hidden-specific rules needed. |
| No watch/rebuild-on-change for bundles | Index is rebuilt per-request (existing pattern) | Yes — the existing per-call rebuild pattern handles new files automatically. |

## Implementation Commit Plan

One PR, five reviewable commits in dependency order:

1. **Commit 1: Scanner parameterization.** Add `ScanOptions` struct, change `ScanAll` signature, add VCS always-skip list, update `Scan` wrapper. Update `TestScan_HiddenDirOpencode` → `TestScan_HiddenDirOpencode_DefaultOff`. Add `TestScan_HiddenDirOpencode_FlagOn`, `TestScan_HiddenDirGit_AlwaysSkipped`, `TestScan_HiddenDirHg_AlwaysSkipped`, `TestScanAll_EnableHidden_ReservedInHiddenDir`.

2. **Commit 2: Bundle resolver + Index integration.** Add `Bundle` field to `parser.Doc` and `TreeNode`. Add `resolveBundle` function to `internal/index/`. Change `index.New` to accept `ScanOptions`. Compute bundle at `Rebuild()` for both docs and reserved files. Add bundle resolution tests (`TestBundle_Resolution`, `TestBundle_Fallback`, `TestBundle_RootFile`, `TestBundle_RootFileNoIndex`, `TestBundle_ReservedFileBundle`, `TestTree_TwoBundles`).

3. **Commit 3: Response shape changes.** Add `bundle` to `list_docs`, `get_doc`, `get_index` handler response maps. Add integration tests (`TestListDocs_BundleField`, `TestGetDoc_BundleField`, `TestGetIndex_BundleOnLeaves`).

4. **Commit 4: Log aggregation.** Rewrite `getLogHandler` to collect all `log.md` files, merge entries, add `source` per entry, replace insertion sort with `sort.SliceStable`. Remove top-level `source` from `logResult`. Add integration tests (`TestGetLog_MultiSource`, `TestGetLog_SingleSource`, `TestGetLog_NoLogMd`).

5. **Commit 5: CLI wiring + docs + invariants.** Wire `--enable-hidden` flag in `main()`, pass through to `index.New` in both MCP and `--validate` paths. Add `TestValidate_HiddenBundle`. Update `AGENTS.md` invariants (I-5, I-12, I-17, I-18, I-19). Update `docs/architecture.md`, `docs/configuration.md`, `docs/log.md` per the OKF update obligation.

## Log

- 2026-07-23: Initial design for multi-bundle support — CLI flag, bundle resolution, log aggregation, response shapes, invariant updates
- 2026-07-23: Amendment (M1): Added `newFixtureServer` ScanOptions threading — helper takes `opts scanner.ScanOptions` as third parameter; added Opts column to integration test table marking which tests pass `EnableHidden: true`. (M2): Amended `TestGetLog_MultiSource` to specify one visible-dir `log.md` (`docs/log.md`) + one hidden-dir `log.md` (`.opencode/architecture/log.md`), requiring `ScanOptions{EnableHidden: true}` — exercises the primary motivating scenario.
