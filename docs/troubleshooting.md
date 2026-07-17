---
type: Playbook
title: Troubleshooting
description: Common issues and solutions for okf-mcp — empty index, document not found, frontmatter warnings, permission errors, wrong directory, and missing binary.
tags: [troubleshooting, faq, debugging, errors, setup]
timestamp: 2026-07-16T00:00:00Z
---

# Troubleshooting

This guide covers the most common issues new users hit with `okf-mcp`, with exact error messages and step-by-step fixes.

---

## 1. Index is empty — no conformant docs found

**Symptom:**

You call `list_tags` or `get_doc` and get one of these responses:

```
index is empty: no OKF-conformant markdown docs found in cwd
```

Or on stderr:

```
okf-mcp: WARN: no conformant OKF docs found in /path/to/dir
```

**Cause:**

`okf-mcp` scans the process working directory for `.md` files with valid OKF frontmatter. Either:

- You are in the **wrong directory** — the scan root does not contain any OKF-conformant docs.
- Your markdown files **lack frontmatter** entirely, or their frontmatter is missing the required `type:` field.
- Your docs live in a subdirectory (e.g. `docs/`) but you launched `okf-mcp` from the parent — this is fine, `okf-mcp` recurses into subdirectories, so the docs *should* be found. Check that the files actually have valid frontmatter.

**Fix:**

1. Verify the binary is scanning the right directory. On startup it prints to stderr:

   ```
   okf-mcp: serving /path/to/repo
   ```

   If the path is wrong, adjust the working directory in your [MCP host configuration](/docs/configuration.md).

2. Check that your markdown files have valid OKF frontmatter. Every indexed file must:
   - Begin with `---\n` (a line containing exactly three dashes followed by a newline)
   - Contain a closing `---` delimiter on its own line
   - Include a non-empty `type:` field in the frontmatter

3. Files named `index.md` and `log.md` are [never indexed](/docs/okf-standard.md) by design — this is expected.

4. Hidden directories (names starting with `.`) are skipped entirely, including `.git`, `.opencode`, etc.

---

## 2. Document not found for topic X

**Symptom:**

```
no document matched topic "deployment" with tags []
```

**Cause:**

`get_doc` uses weighted token scoring to find the best match: title tokens score 3×, tags score 2×, and description tokens score 1×. The error means no document scored above zero for your query — or you are filtering by tags that no document carries.

Common reasons:

- The topic tokens do not appear in any document's title, tags, or description.
- You passed a tag filter (`tags` parameter) and no documents carry all the specified tags (with the default `match=and` mode).
- The document exists but was skipped during indexing (missing `type:` field, reserved filename, etc.).

**Fix:**

1. Call `list_tags` first to see the available tag vocabulary. Use those exact tag strings in your `get_doc` call.
2. Call `list_docs` to see every indexed document with its metadata — confirm your target doc is there.
3. Try a simpler topic query. The scoring tokenises on non-alphanumeric characters and lowercases, so `get_doc(topic="deploy")` works as well as `get_doc(topic="deployment")`.
4. If using tag filtering, try `match="or"` instead of the default `match="and"` to broaden results.
5. If the doc is not in `list_docs` output at all, see [Scenario 1: Index is empty](#1-index-is-empty--no-conformant-docs-found).

---

## 3. Frontmatter warnings on stderr

**Symptom:**

When `okf-mcp` starts or a tool runs, you see warnings on stderr like:

```
okf-mcp: WARN: docs/auth.md: missing title
okf-mcp: WARN: docs/auth.md: missing description
```

**Cause:**

The file has valid frontmatter with a `type:` field (so it *is* indexed), but `title` or `description` is missing. The file is still indexed and searchable, but scoring degrades: a missing title costs 3× match weight, and a missing description costs 1×. This makes the document harder to find via `get_doc`.

**Fix:**

Add the missing fields to the frontmatter. A complete frontmatter block looks like:

```yaml
---
type: Playbook
title: My Document
description: A short sentence explaining what this doc covers.
tags: [relevant-tag]
timestamp: 2026-07-16T00:00:00Z
---
```

See the [OKF Standard](/docs/okf-standard.md) for the full frontmatter schema.

---

## 4. Tool permission denied

**Symptom:**

Your MCP host (opencode, Claude Desktop, etc.) does not offer the `list_tags`, `list_docs`, or `get_doc` tools, or calling them fails with a permission error.

**Cause:**

In hosts that use an explicit allow-list (like opencode), the permission strings for the three tools have not been configured. Each tool must be individually allowed.

**Fix:**

Add the three permission strings to your MCP host configuration. For [opencode](/docs/configuration.md), the permission strings follow the pattern `mcp__<server-key>__<tool-name>`:

```json
{
  "permissions": {
    "allow": [
      "mcp__okf-mcp__list_tags",
      "mcp__okf-mcp__list_docs",
      "mcp__okf-mcp__get_doc"
    ]
  }
}
```

If you registered the server under a different key than `okf-mcp`, update the strings to match. See [Configuration](/docs/configuration.md) for full setup details for opencode and Claude Desktop.

---

## 5. Wrong directory being scanned

**Symptom:**

`list_tags` returns an empty array `[]`, or `list_docs` returns `[]`, even though you know the repository has docs. On startup, stderr shows a path that is not where you expected:

```
okf-mcp: serving /some/wrong/path
```

**Cause:**

`okf-mcp` scans whatever directory the process is started in. The MCP host controls this via its working directory configuration — if the host launches `okf-mcp` from the wrong directory, it scans the wrong tree.

**Fix:**

1. Check the startup line on stderr to confirm the scan root. The binary prints `okf-mcp: serving /path/to/repo` on launch.
2. In [opencode](/docs/configuration.md), ensure the server entry does not set an incorrect working directory. The server starts in the project root by default.
3. In [Claude Desktop](/docs/configuration.md), check the `workingDirectory` field (if set) in the `mcpServers` entry.
4. If you are running `okf-mcp` manually (not via a host), make sure you are in the repository root before launching it:

   ```sh
   cd /path/to/your/repo
   okf-mcp
   ```

---

## 6. Binary not found

**Symptom:**

Your MCP host reports that `okf-mcp` could not be started, or running `okf-mcp` directly gives:

```
command not found: okf-mcp
```

**Cause:**

The binary is not installed or not on your `$PATH`.

**Fix:**

1. **Build from source** if you have a Go toolchain:

   ```sh
   go build ./cmd/okf-mcp
   ```

   This produces an `okf-mcp` binary in the current directory. Move it to somewhere on your `$PATH` (e.g. `/usr/local/bin`).

2. **Install via `go install`:**

   ```sh
   go install github.com/nrkno/plattform-okf-mcp/cmd/okf-mcp@latest
   ```

   This places the binary in `$GOPATH/bin` (default: `$HOME/go/bin`). Ensure that directory is on your `$PATH`.

3. **Download a pre-built release binary** from the [GitHub releases](https://github.com/nrkno/plattform-okf-mcp/releases) page — archives are available for Linux, macOS, and Windows on amd64 and arm64.

4. After installing, verify it is reachable:

   ```sh
   which okf-mcp
   okf-mcp --help
   ```

   See [Deployment](/docs/deployment.md) for the full build, install, and release workflow.
