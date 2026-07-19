---
type: Configuration
title: Configuration
description: How to register okf-mcp in opencode, Claude Desktop, and other MCP hosts, including permission strings and auto-registration behaviour.
tags: [configuration, mcp, opencode, claude, permissions, client-setup]
timestamp: 2026-07-19T00:00:00Z
---

# Configuration

## No config file

`okf-mcp` has no configuration file of its own. The only runtime input is the process working directory, which becomes the scan root. Run the binary from the repository root you want to index.

## Runtime behaviour

`okf-mcp` communicates exclusively over stdio (JSON-RPC). It has no network interface and no authentication. It is designed to run as a subprocess of the MCP host process — the host starts it, pipes stdin/stdout, and terminates it when the session ends.

On startup, `okf-mcp` prints to stderr:

```
okf-mcp: serving /path/to/repo
```

This confirms which directory is being scanned. If the path is wrong, adjust the working directory in the host configuration.

## opencode

Add a server entry to `opencode.json` and include all six tool names in the `permissions.allow` list:

```json
{
  "mcp": {
    "okf-mcp": {
      "type": "local",
      "command": "okf-mcp",
      "args": []
    }
  },
  "permissions": {
    "allow": [
      "mcp__okf-mcp__list_tags",
      "mcp__okf-mcp__list_docs",
      "mcp__okf-mcp__get_doc",
      "mcp__okf-mcp__validate_doc",
      "mcp__okf-mcp__get_index",
      "mcp__okf-mcp__get_log"
    ]
  }
}
```

The six permission strings follow the opencode pattern `mcp__<server-key>__<tool-name>`. If you register the server under a different key than `okf-mcp`, update the permission strings to match.

## Claude Desktop

Register the server in `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or the equivalent on your platform:

```json
{
  "mcpServers": {
    "okf-mcp": {
      "command": "okf-mcp",
      "args": []
    }
  }
}
```

Claude Desktop does not use a separate permission allow-list — registering the server is sufficient.

## Auto-registration via `WithInstructions`

`okf-mcp` uses the MCP `instructions` field (set via `server.WithInstructions(...)` in mcp-go) to inject usage guidance into the agent system prompt on every session start. opencode reads this field from the server's `initialize` response and includes it automatically.

The injected instructions tell the agent to:

1. Call `list_tags` first to discover available topics and tags.
2. Use `get_doc(topic)` to retrieve the relevant document.
3. Use `validate_doc` to check document conformance.
4. Use `get_index` to browse the documentation tree structure.
5. Use `get_log` to access structured change log entries.
6. Prefer these tools over reading files directly when looking for documentation.

No AGENTS.md entry is needed. No additional configuration beyond the server registration and permissions above is required.
