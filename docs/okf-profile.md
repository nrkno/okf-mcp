---
type: Configuration
title: Configuring an OKF Profile
description: Create .okf-profile.yaml in your repository, configure okf-mcp in OpenCode, and verify typed Markdown relationships and graph constraints with complete examples.
tags: [okf-profile, profile, configuration, yaml, relationships, invariants, opencode]
timestamp: 2026-10-10T00:00:00Z
---

# Configuring an OKF profile

`.okf-profile.yaml` configures your repository's relationship vocabulary and graph checks. It maps Markdown headings to relationship names, restricts document types on each side of a link, and defines relationship-count constraints. Put it in the repository being indexed, not in an agent prompt or OpenCode's configuration directory. For all fields and defaults, see [OKF Profile Format](/docs/okf-profile-format.md).

## 1. Configure the harness

Use a profile-capable `okf-mcp` binary exposing `--profile` and graph tools. Check the executable your harness actually launches with `--help`; an older six-tool binary is not sufficient. Building the restored source in this repository produces a profile-capable server.

For OpenCode V2, merge this entry into your repository's `opencode.json`, preserving other settings. Replace the executable path:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "servers": {
      "okf-mcp": {
        "type": "local",
        "command": ["/absolute/path/to/okf-mcp", "--profile", ".okf-profile.yaml"],
        "cwd": "."
      }
    }
  }
}
```

OpenCode resolves `cwd` from the workspace; this becomes the server's scan root. For a subdirectory workspace, use the intended repository's absolute path as `cwd`. The profile path is relative to that process directory. Other harnesses need the same executable, arguments, and directory in their own configuration format. See OpenCode's [V2 MCP reference](https://opencode.ai/v2/docs/mcp-servers); older host examples elsewhere in this repository use an earlier format.

Explicit `--profile` is useful during setup: an unreadable/invalid profile fails startup with exit code **2**, instead of falling back. To use automatic discovery, remove `"--profile", ".okf-profile.yaml"` from `command`. The server discovers `.okf-profile.yaml` at the scan root. An invalid discovered profile warns on stderr and falls back to the default; missing profiles also use the default, where links are `untyped` and no profile constraints run.

Check stderr for `loaded profile`, `loaded discovered profile`, or `using default profile`. One startup-selected profile applies to the entire corpus, including nested bundles. Restart/reconnect after editing it; document graph rebuilds do not reload the profile. Add `--enable-hidden` for documents under directories such as `.opencode/`; it controls traversal, not profile activation. VCS internals remain skipped.

## 2. Create `.okf-profile.yaml`

Save this complete profile at your repository's scan root:

```yaml
name: my-repository
version: "1.0"
concept_types:
  - name: anchor
  - name: requirement
relationships:
  - name: derived-from
    heading_aliases: [Derived from]
    inverse: derives
    allowed_source_types: [requirement]
    allowed_target_types: [anchor]
invariants:
  - id: requirement-has-source
    description: Every requirement must link to at least one anchor.
    source_type: requirement
    relationship: derived-from
    direction: outgoing
    min: 1
    max: -1
    severity: error
```

Use exact, case-sensitive frontmatter types in allow-lists and selectors. Heading aliases are lowercased, trimmed, and whitespace-normalized; `Derived from` and ` DERIVED   FROM ` match alike. Relationship names are your vocabulary: hyphens and underscores are not interchangeable in queries. Concept aliases do not rewrite frontmatter or expand allow-lists.

For a minimum of one with no maximum, use `min: 1`, `max: -1` — not `max: null` or an omitted maximum, which default to zero. Include the invariant's `relationship`; descriptions do not override fields. Use `source_type: "*"` explicitly for all types; omission does not mean all. An incoming invariant uses the original forward relationship name, not the inverse shown in incoming query results. See the format reference for validation errors and additional constraints.

## 3. Create linked documents

Create `docs/anchor.md`:

```markdown
---
type: anchor
title: Example Anchor
description: The goal that motivates the example requirement.
tags: [example, goal]
---

# Example Anchor
```

Create `docs/requirement.md` alongside it:

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

The requirement has an outgoing `derived-from` edge to the anchor; the anchor's incoming view shows `derives`. Both files must be indexed: valid YAML frontmatter with non-empty `type`, and visible to the scanner. A concept declaration alone creates no document.

Classification uses the nearest preceding heading, not a matching ancestor. A `### Notes` before the link changes it to `untyped` unless that heading is mapped. Unrecognized headings and links before any heading are `untyped`; type allow-list checks skip them. A minimum-count invariant catches missing or misclassified required links.

Use plain file links. `anchor.md` resolves relative to the source document; `/docs/anchor.md` resolves from the scan root. Escaping paths are dropped. HTTP(S), mailto, and fragment-only links are skipped, as are links inside code blocks/spans. Targets must resolve to indexed documents or are reported as dangling. File fragments such as `anchor.md#section` are not stripped in this extractor, so avoid them for graph links. Counts are extracted links, not unique targets.

## 4. Verify through the harness

Connect the server (`opencode mcp list`, or `/mcps`) and inspect its profile-selection log. Ask your agent to call these tools; harnesses may prefix/group tool names differently:

| Tool | Arguments | Expected for this example |
|---|---|---|
| `get_index` | `{}` | Both documents are present. |
| `graph_relationships` | `{"file_path":"docs/requirement.md","direction":"outgoing","type":"derived-from"}` | One edge to `docs/anchor.md`. |
| `graph_relationships` | `{"file_path":"docs/anchor.md","direction":"incoming","type":"derives"}` | One edge from `docs/requirement.md`. |
| `graph_integrity` | `{"checks":["profile_violations"]}` | `profile_loaded: true`, no type/cardinality findings for the example. Other repository documents may produce findings. |

Removing the link should produce an `error` cardinality finding for the requirement. Changing the anchor's type should produce a target-type warning. Type violations do not remove edges; cardinality findings use the invariant's severity. The `profile_violations` filter includes both kinds of checks.

`profile_loaded: false` can mean no profile, invalid-discovery fallback, or a loaded profile with neither relationships nor invariants. Check stderr and the actual executable/root before treating zero findings as success.

**Document conformance and graph validation are separate.** `validate_doc` checks frontmatter/reserved-file conformance; `graph_integrity` checks relationships and cardinality. Single-file `validate_doc` accepts `known_types` for custom types such as `anchor` and `requirement`; `concept_types` does not configure that vocabulary. CLI `--validate` also checks document conformance, not graph enforcement: its validation index uses the default profile even when startup loads `--profile`. Graph findings do not automatically block commits or fail that CLI command. Have your agent/CI consume both checks and apply your chosen policy.
