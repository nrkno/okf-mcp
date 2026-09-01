---
type: design
title: OKF MCP Validator, Bundle Tree, and Structured Log
description: Architecture design for validate_doc MCP tool + CLI --validate, get_index bundle tree, and get_log structured entries on the existing okf-mcp server.
tags: [validator, tree, log, mcp, scanner, cli]
---

## Problem

The okf-mcp server indexes and queries OKF docs but provides no validation, no tree overview, and no structured log access. Agents have no way to verify a doc is conformant before or after writing it, no way to see the bundle's directory structure, and no way to query the documentation change log programmatically. The pre-existing bash validation script demonstrates the need but is not integrated into the MCP workflow.

## Constraints

- Single CGO_ENABLED=0 Go binary. No config file, no database, no HTTP.
- Existing four internal packages (scanner, parser, index, matcher) must not break.
- Existing invariants I-1 through I-7 are non-negotiable.
- The index is rebuilt on every tool call; all new tools follow the same pattern.
- Tests use mcptest.NewServer with a package-level `idx` variable; no `t.Parallel()` when mutating `idx`.
- Files in hidden directories (`.opencode/`, `.git/`) remain invisible to the index.
- The OKF type vocabulary is: `Architecture`, `Playbook`, `Configuration`, `API Reference`, `Metrics Reference`, `Log`.

## Invariants & Guarantees

### Existing invariants (preserved)

| ID  | Invariant                                                       | Status                 |
| --- | --------------------------------------------------------------- | ---------------------- |
| I-1 | `file_path` in every tool response is relative to scan-root cwd | **Preserved**          |
| I-2 | `get_doc` content is live-read from disk                        | **Preserved**          |
| I-3 | Files missing `type` field are silently skipped                 | **Preserved**          |
| I-4 | `index.md` and `log.md` never indexed                           | **Modified** — see I-8 |
| I-5 | Hidden directories never traversed                              | **Preserved**          |
| I-6 | `get_doc` tie-break by alphabetical `file_path` ascending       | **Preserved**          |
| I-7 | Zero-doc startup does not crash                                 | **Preserved**          |

### New invariants

| ID   | Invariant                                                                                                                       | Component      | Precondition                   | Falsifiable on real input                                                         |
| ---- | ------------------------------------------------------------------------------------------------------------------------------- | -------------- | ------------------------------ | --------------------------------------------------------------------------------- |
| I-8  | `index.md` and `log.md` are surfaced by the scanner as reserved files — they appear in `Index.Reserved()` but never in `Docs()` | scanner, index | File exists at scan root       | Call `list_docs` and verify reserved absent; call `Reserved()` and verify present |
| I-9  | `validate_doc` returns zero errors for a conformant bundle                                                                      | validator      | Bundle is conformant           | Create conformant bundle, validate, assert zero errors                            |
| I-10 | `validate_doc` returns at least one error for a file with frontmatter but no `type` field                                       | validator      | File exists on disk            | Create file with `---\ntitle: X\n---\n`, validate, assert error                   |
| I-11 | `get_index` returns a tree whose leaves are indexed .md files and reserved files                                                | get_index      | Index rebuilt                  | Create nested dirs with docs, call get_index, verify structure                    |
| I-12 | `get_log` returns entries in reverse-chronological order with parsed date, action, and target                                   | get_log        | log.md exists                  | Create log.md with dated entries, call get_log, verify order and fields           |
| I-13 | `--validate` exits 0 on conformant, 1 on errors, does not start MCP server                                                      | main.go        | Binary invoked with --validate | Run on conformant dir, assert exit 0; run on non-conformant, assert exit 1        |
| I-14 | Pre-commit hook invokes `okf-mcp --validate` and blocks commit on exit 1                                                        | hook script    | Binary on PATH                 | Introduce non-conformant file, stage, commit, assert hook blocks                  |
| I-15 | `parser.DetectFrontmatter` is the single source of truth for frontmatter detection                                              | parser         | File content available         | Verify `Parse` and `ValidateDoc` both call `DetectFrontmatter`; grep for bare `strings.HasPrefix("---\n")` |
| I-16 | `ValidateReserved` applies only E3; `ValidateDoc` applies only E1/E2/W1-W4/N1-N3                                                | validator      | File path provided             | Create index.md with no frontmatter, validate bundle, assert zero E1 errors      |

## Options considered

### Option A: Separate validator package + scanner reservation changes (Recommended)

- **How it works**: Add `internal/validator` package that accepts an `*index.Index` and file paths, reuses `parser.Parse` for frontmatter extraction, and performs E1/E2/E3 checks plus contextual warnings (unknown type, missing recommended fields). Modify scanner to return reserved files separately via `ScanAll()`. Add `Index.Reserved()` accessor. `validate_doc` MCP tool calls validator after rebuild. `get_index` builds a tree from `Docs()` + `Reserved()`. `get_log` parses `log.md` body. CLI `--validate` flag in `main.go` runs validation and exits before MCP server starts.

- **Pros**: Clean separation; scanner change minimal and backward-compatible; reuses parser.Parse; CLI shares MCP code path; reserved files accessible without breaking I-4
- **Cons**: Scanner API gains new function; Index struct gains two fields

- **Risks**: Scanner change must be backward-compatible — solved by keeping `Scan()` and adding `ScanAll()`

### Option B: Keep scanner unchanged, discover reserved files in the index

- **How it works**: Scanner continues to skip reserved files. The index separately walks for index.md/log.md after the main scan.

- **Pros**: Scanner package unchanged
- **Cons**: Duplicates the walk; reserved files discovered by second `filepath.WalkDir`; scanner's `reserved` map not reusable; two code paths

- **Risks**: Second walk could diverge from scanner behavior

### Option C: Raw markdown for get_log, no parsing

- **How it works**: `get_log` reads and returns the raw body of log.md without parsing.

- **Pros**: Zero parsing logic; trivial
- **Cons**: Agents cannot filter by date or action type; contradicts brainstorm conclusion

- **Risks**: Agent must parse markdown itself, defeating purpose of structured tool

## Recommendation

**Option A** for scanner change and validator. **Option C rejected** for get_log — structured parsing is the point. Scanner change via `ScanAll()` keeps backward compatibility: existing `Scan()` callers untouched; `Rebuild()` uses `ScanAll()` internally.

Key reasons: single code path for validation (MCP + CLI + pre-commit); scanner reservation is clean and testable; tree and log tools build naturally on extended index; new invariants I-8 through I-14 are falsifiable and bounded.

## Scanner changes

### Current behavior

`scanner.Scan(dir)` returns `([]string, error)` — absolute paths of .md files, excluding reserved filenames and hidden dirs.

### Proposed change

Add a `ScanResult` struct and `ScanAll` function. `Scan` remains unchanged (backward compatible):

```go
// ScanResult holds the results of a full directory scan.
type ScanResult struct {
    Docs     []string // absolute paths of indexable .md files (same as Scan)
    Reserved []string // absolute paths of reserved files (index.md, log.md)
}

// ScanAll walks dir and returns both indexable and reserved file paths.
func ScanAll(dir string) (ScanResult, error) {
    var result ScanResult
    err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
        // ... same skip rules as Scan ...
        if reserved[name] {
            result.Reserved = append(result.Reserved, path)
            return nil
        }
        // ... rest of existing logic ...
        result.Docs = append(result.Docs, path)
        return nil
    })
    return result, err
}

// Scan delegates to ScanAll, returning only the Docs slice.
func Scan(dir string) ([]string, error) {
    r, err := ScanAll(dir)
    if err != nil {
        return nil, err
    }
    return r.Docs, nil
}
```

### Index changes

The `Index` struct gains:

```go
type Index struct {
    dir      string
    mu       sync.Mutex
    docs     []parser.Doc
    reserved []ReservedFile  // new
}

type ReservedFile struct {
    FilePath       string // relative path (e.g. "index.md", "docs/log.md")
    HasFrontmatter bool
    Type           string // from frontmatter if present, empty otherwise
}

// Reserved returns reserved files from the last Rebuild.
func (idx *Index) Reserved() []ReservedFile { ... }
```

`Rebuild()` calls `scanner.ScanAll` and populates both `docs` and `reserved`. Reserved files get the same I-1 relativization. For each reserved file, `parser.Parse` extracts frontmatter metadata (index.md has none; log.md has `type: Log`).

**Invariant I-4 preserved**: reserved files never appear in `Docs()`. They are accessible only through `Reserved()`.

## internal/validator package

### Data types

```go
// Severity classifies a validation finding.
type Severity int

const (
    SeverityError       Severity = iota // Blocks conformity — E1, E2, E3
    SeverityWarning                     // Quality issue — missing title, unknown type
    SeverityNotification                // Informational — timestamp missing
)

// Finding is a single validation result.
type Finding struct {
    File     string   // relative path
    Severity Severity
    Code     string   // e.g. "E1", "E2", "E3", "W1", "W2", "N1"
    Message  string   // human-readable description
}

// Result holds all findings for a validation run.
type Result struct {
    Findings []Finding
    Summary  Summary
}

type Summary struct {
    Files         int // total files checked
    Errors        int
    Warnings      int
    Notifications int
}
```

### Shared frontmatter utility — parser package

Extract a shared function in `internal/parser` that both `Parse` and the validator call, eliminating duplicated frontmatter detection:

```go
package parser

// FrontmatterInfo holds the result of frontmatter detection.
type FrontmatterInfo struct {
    HasFrontmatter bool
    YAMLBlock      string // raw YAML between delimiters (empty if !HasFrontmatter)
    BodyOffset     int    // byte offset of markdown body start
}

// DetectFrontmatter checks if content starts with "---\n" and has a closing
// "---" delimiter. Single source of truth for frontmatter detection.
func DetectFrontmatter(content string) FrontmatterInfo {
    if !strings.HasPrefix(content, "---\n") {
        return FrontmatterInfo{}
    }
    rest := content[len("---\n"):]
    end := findClosingDelimiter(rest)
    if end < 0 {
        return FrontmatterInfo{}
    }
    bodyOffset := 4 + end + 4
    if bodyOffset > len(content) {
        bodyOffset = len(content)
    }
    return FrontmatterInfo{
        HasFrontmatter: true,
        YAMLBlock:      rest[:end],
        BodyOffset:     bodyOffset,
    }
}
```

`parser.Parse` refactored to call `DetectFrontmatter` internally. The validator calls `DetectFrontmatter` directly, avoiding the I-3 skip semantics of `Parse` (which returns `false` when `type` is empty — exactly the condition E2 must report).

**Invariant I-15**: `parser.DetectFrontmatter` is the single source of truth for frontmatter detection. `parser.Parse` and `validator.ValidateDoc` must both call it — neither may reimplement the `---\n` prefix check or closing delimiter scan.

### Validation codes

| Code | Severity     | Check                    | Description                                                    | Applies to    |
| ---- | ------------ | ------------------------ | -------------------------------------------------------------- | ------------- |
| E0   | Error        | Read failure             | File could not be read from disk                               | All           |
| E1   | Error        | Frontmatter present      | File has `---` delimiters with YAML content                    | Regular docs  |
| E2   | Error        | Type field non-empty     | Frontmatter contains non-empty `type` field                    | Regular docs  |
| E3   | Error        | Reserved-file structure  | index.md has NO frontmatter; log.md has `type: Log`            | Reserved only |
| W1   | Warning      | Title present            | Frontmatter contains non-empty `title`                         | Regular docs  |
| W2   | Warning      | Description present      | Frontmatter contains non-empty `description`                   | Regular docs  |
| W3   | Warning      | Type in vocabulary       | `type` is one of six known types (contextual, needs index)     | Regular docs  |
| W4   | Warning      | Tags present             | Frontmatter contains at least one tag                          | Regular docs  |
| N1   | Notification | Timestamp present        | Frontmatter contains `timestamp` (NRK convention)              | Regular docs  |
| N2   | Notification | index.md links valid     | Links in index.md point to existing files                      | Reserved only |
| N3   | Notification | log.md entries parseable | Entries follow `## DATE` / `**Action**:` pattern                | Reserved only |

### Package API

```go
package validator

// ValidateDoc validates a regular (non-reserved) document file against OKF rules.
// Reads from disk, detects frontmatter via parser.DetectFrontmatter,
// checks E1/E2 plus contextual warnings (W1-W4, N1-N3).
func ValidateDoc(absPath string, knownTypes []string) ([]Finding, error)

// ValidateReserved validates a reserved file (index.md or log.md).
// Checks only E3 with per-filename logic — no E1/E2.
func ValidateReserved(absPath string, relPath string) ([]Finding, error)

// ValidateBundle validates all files in the index.
// Dispatches to ValidateDoc for Docs() and ValidateReserved for Reserved().
func ValidateBundle(idx *index.Index) Result

// ValidatePaths validates specified absolute file paths.
// Determines reserved status by basename matching.
func ValidatePaths(paths []string, knownTypes []string) Result
```

### How ValidateDoc works

1. Read file from disk.
2. Call `parser.DetectFrontmatter(content)`.
3. Check E1: does `info.HasFrontmatter` hold? If not → Error E1.
4. If `!info.HasFrontmatter`, return findings (just E1) — no further checks.
5. Parse YAML from `info.YAMLBlock` via `yaml.Unmarshal`.
6. Check E2: is `type` non-empty? If not → Error E2.
7. Check W1 (title present), W2 (description present), W3 (type in known vocabulary if `knownTypes` non-nil), W4 (tags present).
8. Check N1 (timestamp present).
9. Return all findings.

### How ValidateReserved works

Receives both `absPath` (for reading) and `relPath` (for reserved-status). Reserved status determined by `filepath.Base(relPath)` matching `"index.md"` or `"log.md"` — same as scanner's reserved map.

```go
func ValidateReserved(absPath string, relPath string) ([]Finding, error) {
    name := filepath.Base(relPath)
    content, err := os.ReadFile(absPath)
    if err != nil { return nil, err }
    info := parser.DetectFrontmatter(string(content))

    switch name {
    case "index.md":
        // E3: must NOT have frontmatter
        if info.HasFrontmatter {
            return []Finding{{
                File: relPath, Severity: SeverityError, Code: "E3",
                Message: "index.md must not have frontmatter (OKF spec)",
            }}, nil
        }
        return nil, nil

    case "log.md":
        // E3: must have frontmatter with type: "Log"
        if !info.HasFrontmatter {
            return []Finding{{
                File: relPath, Severity: SeverityError, Code: "E3",
                Message: "log.md must have frontmatter with type: Log",
            }}, nil
        }
        var fm struct{ Type string `yaml:"type"` }
        if err := yaml.Unmarshal([]byte(info.YAMLBlock), &fm); err != nil {
            return []Finding{{
                File: relPath, Severity: SeverityError, Code: "E3",
                Message: "log.md has invalid YAML frontmatter",
            }}, nil
        }
        if fm.Type != "Log" {
            return []Finding{{
                File: relPath, Severity: SeverityError, Code: "E3",
                Message: fmt.Sprintf("log.md type must be \"Log\", got %q", fm.Type),
            }}, nil
        }
        return nil, nil

    default:
        return nil, nil
    }
}
```

### How ValidateBundle works

```go
func ValidateBundle(idx *index.Index) Result {
    idx.Rebuild()
    knownTypes := uniqueTypes(idx.Docs())
    var findings []Finding

    for _, doc := range idx.Docs() {
        fs, err := ValidateDoc(doc.FilePath, knownTypes)
        if err != nil {
            findings = append(findings, Finding{
                File: doc.FilePath, Severity: SeverityError, Code: "E0",
                Message: fmt.Sprintf("read error: %v", err),
            })
            continue
        }
        findings = append(findings, fs...)
    }

    for _, r := range idx.Reserved() {
        abs := absPathFor(r.FilePath)
        fs, err := ValidateReserved(abs, r.FilePath)
        if err != nil {
            findings = append(findings, Finding{
                File: r.FilePath, Severity: SeverityError, Code: "E0",
                Message: fmt.Sprintf("read error: %v", err),
            })
            continue
        }
        findings = append(findings, fs...)
    }

    return buildResult(findings, len(idx.Docs())+len(idx.Reserved()))
}
```

Key change: `ValidateBundle` dispatches to `ValidateDoc` for regular docs and `ValidateReserved` for reserved files. This prevents `index.md` from triggering a false E1 error.

### Integration with MCP tool

```go
var validateDocTool = mcp.NewTool("validate_doc",
    mcp.WithDescription("Validate OKF-conformant documents and report errors, warnings, and notifications"),
    mcp.WithString("file_path",
        mcp.Description("Optional: relative path of a single file to validate. If omitted, validates entire bundle."),
    ),
)

func validateDocHandler(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    if err := idx.Rebuild(); err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }
    // Extract optional file_path from args
    // If provided: resolve to abs, call ValidateDoc or ValidateReserved
    // If omitted: call ValidateBundle
    // Marshal Result to JSON, return
}
```

## Bundle tree model (get_index)

### Data types

```go
// TreeNode represents a node in the bundle tree.
type TreeNode struct {
    Name     string     `json:"name"`               // filename or directory name
    Path     string     `json:"path"`               // relative path from scan root
    Type     string     `json:"type"`               // "file", "directory", "reserved"
    DocType  string     `json:"doc_type,omitempty"` // from frontmatter (e.g. "Architecture")
    Title    string     `json:"title,omitempty"`    // from frontmatter
    Children []TreeNode `json:"children,omitempty"` // non-nil for directories
}
```

### How the tree is built

Constructed from the flat list of docs + reserved files by splitting paths into segments and building a nested structure:

```
Input: ["docs/architecture.md", "docs/tools.md", "index.md", "docs/log.md"]

Output:
{
  name: ".",
  path: "",
  type: "directory",
  children: [
    { name: "index.md", path: "index.md", type: "reserved" },
    { name: "docs", path: "docs", type: "directory", children: [
      { name: "architecture.md", path: "docs/architecture.md", type: "file", doc_type: "Architecture", title: "Architecture" },
      { name: "log.md", path: "docs/log.md", type: "reserved", doc_type: "Log", title: "Log" },
      { name: "tools.md", path: "docs/tools.md", type: "file", doc_type: "API Reference", title: "MCP Tools Reference" }
    ]}
  ]
}
```

### Tree building function

```go
package index

// Tree returns the bundle tree for the current index state.
// Root node represents the scan root. Children are directories and files.
// Reserved files appear with type "reserved". Content files with type "file".
func (idx *Index) Tree() TreeNode { ... }
```

Built inside the Index package — needs access to both `docs` and `reserved` slices under the mutex.

### get_index MCP tool

```go
var getIndexTool = mcp.NewTool("get_index",
    mcp.WithDescription("Return the bundle tree showing all documents and their directory structure"),
    mcp.WithString("path",
        mcp.Description("Optional: relative path to a subtree root. If omitted, returns full tree."),
    ),
)

func getIndexHandler(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    if err := idx.Rebuild(); err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }
    tree := idx.Tree()
    // If path param provided, walk tree to find subtree root
    // Marshal to JSON, return
}
```

### Subtree support

The `path` parameter enables progressive disclosure: an agent calls `get_index` with `path="docs"` to see only the `docs/` subtree, avoiding token waste. The subtree is found by walking the built tree to the matching node, then returning that node as root.

## Log parsing (get_log)

### Data type

```go
// LogEntry represents a single entry from log.md.
type LogEntry struct {
    Date    string `json:"date"`              // YYYY-MM-DD
    Action  string `json:"action"`            // "Creation", "Update", etc.
    Target  string `json:"target"`            // target file path
    Detail  string `json:"detail,omitempty"`  // full description
}

// LogResult holds parsed log entries and metadata.
type LogResult struct {
    Entries   []LogEntry `json:"entries"`
    Source    string     `json:"source"`        // file_path of log.md (relative)
    Truncated bool       `json:"truncated"`     // true if limit-truncated
}
```

### Parsing logic

The parser scans the body of `log.md` (after frontmatter) for:

1. **Date headings**: lines matching `## YYYY-MM-DD`
2. **Action entries**: lines matching `**Action**: \`target\` — detail`

Regex for entries: `\*\*(\w+)\*\*:\s*\x60([^\x60]+)\x60\s*—\s*(.+)`

Lines that don't match within a date section are detail of the preceding entry (or skipped if before any entry).

### get_log MCP tool

```go
var getLogTool = mcp.NewTool("get_log",
    mcp.WithDescription("Return structured log entries from the documentation change log"),
    mcp.WithString("since",
        mcp.Description("Optional: only return entries on or after this date (YYYY-MM-DD)"),
    ),
    mcp.WithString("action",
        mcp.Description("Optional: filter by action type (e.g. 'Creation', 'Update')"),
    ),
    mcp.WithNumber("limit",
        mcp.Description("Optional: maximum number of entries to return (default: all)"),
    ),
)
```

### Integration

The log parser lives in `internal/logparser`. The `get_log` handler:

1. Calls `idx.Rebuild()` for fresh index.
2. Finds log.md via `idx.Reserved()` (scanning for basename `log.md`).
3. If log.md is absent: returns empty entries with `"note":"no log.md found"`.
4. If log.md exists but has malformed frontmatter: performs best-effort parse of whatever entries exist, returns them with `"note":"log.md has malformed frontmatter; parsed available entries"`.
5. Reads file from disk (I-2 pattern: live read, not cached).
6. Calls `logparser.Parse(body)` → `[]LogEntry`.
7. Applies `since`, `action`, and `limit` filters.
8. Returns `LogResult` as JSON.

Key behavior: `get_log` never silently returns nothing. Degraded cases are observable via the `note` field, not silent.

## CLI --validate flag

### main.go changes

```go
func main() {
    validateFlag := flag.Bool("validate", false, "Validate OKF docs and exit (no MCP server)")
    validatePath := flag.String("path", ".", "Path to validate (relative to cwd)")
    flag.Parse()

    if *validateFlag {
        runValidate(*validatePath)
        return
    }
    // ... existing MCP server setup ...
}

func runValidate(path string) {
    absPath, err := filepath.Abs(path)
    if err != nil {
        fmt.Fprintf(os.Stderr, "okf-mcp: invalid path: %v\n", err)
        os.Exit(2)
    }
    idx := index.New(absPath)
    if err := idx.Rebuild(); err != nil {
        fmt.Fprintf(os.Stderr, "okf-mcp: scan error: %v\n", err)
        os.Exit(2)
    }
    result := validator.ValidateBundle(idx)
    for _, f := range result.Findings {
        fmt.Fprintf(os.Stderr, "%s: [%s] %s: %s\n",
            f.File, f.Code, severityLabel(f.Severity), f.Message)
    }
    fmt.Fprintf(os.Stderr, "\n%d files: %d errors, %d warnings, %d notifications\n",
        result.Summary.Files, result.Summary.Errors,
        result.Summary.Warnings, result.Summary.Notifications)
    if result.Summary.Errors > 0 {
        os.Exit(1)
    }
    os.Exit(0)
}
```

### Exit code convention

| Exit code | Meaning                                               |
| --------- | ----------------------------------------------------- |
| 0         | All files conformant (zero errors, may have warnings) |
| 1         | One or more errors found                              |
| 2         | Infrastructure failure (bad path, scan error)         |

### Usage

```sh
okf-mcp --validate              # validate cwd
okf-mcp --validate --path docs/ # validate specific path
```

## Pre-commit hook

### Script

```sh
#!/bin/sh
# .githooks/pre-commit — validate OKF docs before commit
# Install: git config core.hooksPath .githooks
if ! command -v okf-mcp >/dev/null 2>&1; then
    echo "okf-mcp: pre-commit hook requires okf-mcp on PATH" >&2
    echo "Install: see README.md#installation" >&2
    exit 1
fi
okf-mcp --validate
exit $?
```

### Installation (documented in README)

```sh
git config core.hooksPath .githooks
chmod +x .githooks/pre-commit
```

### Validate changed files only vs whole bundle

The hook validates the **whole bundle**. Rationale: cross-referencing requires the full index (unknown-type warnings need all types; index.md link validation needs all files). The index rebuild is milliseconds for typical bundles. Validating only changed files would miss ripple effects (e.g. removing a file that index.md still links to).

## Input/operation coverage

| Input shape                          | validate_doc     | get_index | get_log  |
| ------------------------------------ | ---------------- | --------- | -------- |
| Single conformant file (all fields)  | handled          | N/A       | N/A      |
| Single file missing frontmatter (E1) | handled          | N/A       | N/A      |
| Single file missing type (E2)        | handled          | N/A       | N/A      |
| index.md with frontmatter (E3)       | handled          | N/A       | N/A      |
| log.md missing type:Log (E3)         | handled          | N/A       | N/A      |
| File with unknown type (W3)          | handled          | N/A       | N/A      |
| Full bundle validation               | handled          | N/A       | N/A      |
| Empty bundle                         | handled (0 errs) | handled   | handled  |
| Flat bundle (no subdirs)             | N/A              | handled   | N/A      |
| Nested bundle (multi-level dirs)     | N/A              | handled   | N/A      |
| Subtree via path param               | N/A              | handled   | N/A      |
| log.md with valid entries            | N/A              | N/A       | handled  |
| log.md with malformed entries        | N/A              | N/A       | handled  |
| log.md missing entirely              | N/A              | N/A       | handled  |
| log.md malformed (no frontmatter)    | handled (E3)     | N/A       | handled (best-effort parse + note) |
| Date filter (since param)            | N/A              | N/A       | handled  |
| Action filter                        | N/A              | N/A       | handled  |
| Limit param                          | N/A              | N/A       | handled  |
| CLI --validate conformant dir        | handled (exit 0) | N/A       | N/A      |
| CLI --validate non-conformant dir    | handled (exit 1) | N/A       | N/A      |
| CLI --validate with --path           | handled          | N/A       | N/A      |

## Security threat model

No security-sensitive surface: the binary operates on local files in the process's working directory, serves over stdio (no network), accepts no authentication credentials, stores no secrets, and handles no PII. The MCP protocol is stdio JSON-RPC between the binary and its host process — no network boundary is crossed. The `--validate` CLI flag reads files from disk but writes only to stderr and exits. No external input is accepted beyond file paths resolved via `filepath.Abs` relative to cwd.

## Anchor check

**Minimal version reached**: Yes.

- validate_doc MCP tool + --validate CLI flag sharing internal/validator with E1/E2/E3 + contextual checks ✓
- get_index returning the bundle tree ✓
- get_log returning structured entries ✓
- Pre-commit hook documented ✓

**Not this boundary respected**: No file watcher, no doc generator, no schema registry, no graph-of-links analysis, no replacement for git log. Tree is structural only. Log parser is read-only.

**Elements traced to Goal/Forces/Invariants**:

- Validator package → Force: "validation must be fast enough for pre-commit hook" + "agent won't call validate unless forced or it's frictionless"
- Scanner reservation → Invariant I-4 preservation + Goal: tree and log tools need reserved files
- get_index tree → Force: "tree must support progressive disclosure per OKF §6"
- get_log structured → Goal: agent can read structured log entries
- CLI --validate → Force: "pre-commit hook must be documented in README"
- Pre-commit hook → Force: "pre-commit hook is the safety net for agent overconfidence"

No speculative elements found.

## Open questions

1. **Log parser robustness** — RESOLVED. Unparseable lines are skipped; entries with missing action/target get empty fields. Handler returns `note` field when degradation occurs.

2. **Reserved files in subdirectories** — RESOLVED. Scanner skips all files named `index.md`/`log.md` regardless of depth. E3 applies to all of them. `ValidateReserved` uses basename matching, consistent with scanner.

3. **get_index empty tree** — RESOLVED. Root node `{name: ".", path: "", type: "directory", children: []}` — empty children array, consistent with `list_docs` returning `[]`.

4. **validate single file via MCP** — RESOLVED. `ValidatePaths` cross-references against the index for unknown-type warnings. Consistent with `ValidateBundle`.

## Deferred / excluded - and the guarantee each one breaks

| Deferred                                            | Guarantee weakened                                                         | Acceptable?                                                |
| --------------------------------------------------- | -------------------------------------------------------------------------- | ---------------------------------------------------------- |
| `--validate --changed` (validate only staged files) | Pre-commit validates whole bundle, not just changes                        | Yes — whole-bundle is correct and fast                     |
| Graph-of-links analysis                             | No link-integrity beyond index.md (N2). Broken body cross-links undetected | Yes — explicitly "Not this"; N2 covers index.md per spec   |
| Schema registry for type vocabulary                 | W3 checks hardcoded list. New types need code change                       | Yes — 6 values, stable, defined by OKF v0.1 spec           |
| Log entry write-back                                | get_log is read-only. Agents cannot add entries via MCP                    | Yes — write-back out of scope; agents edit log.md directly |
| Watcher for live validation                         | No auto re-validation on file change                                       | Yes — watcher in "Not this"; scan-on-every-call sufficient |
| Double-rebuild elimination in ValidateBundle         | `ValidateBundle` calls `idx.Rebuild()` even when caller already did        | Yes — millisecond scan, functionally correct, not worth coupling to caller's state |

## Log

- 2026-07-17: Initial design — scanner reservation (ScanAll), validator package (E1/E2/E3 + contextual), bundle tree (get_index), structured log (get_log), CLI --validate, pre-commit hook, 7 new invariants (I-8 through I-14).
- 2026-07-17: **Amendment** — resolved critic [ESCALATE: QUALITY] findings F1/F2/F3. Split `ValidateFile` into `ValidateDoc` + `ValidateReserved` to prevent false E1 on reserved files (F1). Extracted `parser.DetectFrontmatter` as shared utility to eliminate logic drift (F2). Added explicit reserved-status detection by basename in `ValidateReserved` and `ValidatePaths` (F3). Added `get_log` fallback behavior for malformed/missing log.md. Added pre-commit hook binary-not-found handling. Documented empty bundle behavior. Added invariants I-15, I-16.
