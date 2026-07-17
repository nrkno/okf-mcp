# Contributing to okf-mcp

Thanks for your interest in contributing. This document covers development setup, testing, commit conventions, documentation standards, and the PR process.

## Prerequisites

- **Go 1.26.5 or later** — the version specified in `go.mod`
- **git**
- **golangci-lint** — required for local linting (install via [golangci-lint.com](https://golangci-lint.run/welcome/install/))

## Development Setup

Clone and build:

```sh
git clone https://github.com/nrkno/plattform-okf-mcp.git
cd plattform-okf-mcp
go build ./cmd/okf-mcp
```

This produces an `okf-mcp` binary in the current directory. For a static binary (useful for containers):

```sh
CGO_ENABLED=0 go build ./cmd/okf-mcp
```

For full build, install, and release details, see [Deployment](/docs/deployment.md).

## Running Tests

The pre-commit gate requires both of these to pass cleanly:

```sh
go test -race ./...
go vet ./...
```

Additional checks useful during development:

```sh
golangci-lint run ./...   # static analysis
go test -race -shuffle=on -count=3 ./cmd/okf-mcp/...   # integration tests with order shuffling
govulncheck ./...         # dependency vulnerability scan
```

All tests run with the race detector by default in CI. The integration test suite in `cmd/okf-mcp/` uses `mcptest` to drive assertions through the real MCP JSON-RPC pipe — do not bypass it with direct handler calls.

## Commit Conventions

This project uses [Conventional Commits](https://www.conventionalcommits.org/). Every commit message **must** follow this format:

```
<type>(<optional scope>): <description>
```

Common types:

| Type | Use for |
|------|---------|
| `feat` | New feature or capability |
| `fix` | Bug fix |
| `chore` | Maintenance, dependency updates, CI changes |
| `docs` | Documentation-only changes |
| `refactor` | Code restructuring without behavior change |
| `test` | Adding or updating tests |
| `perf` | Performance improvement |

Examples:

```
feat(matcher): add OR tag filter to get_doc
fix(scanner): skip hidden directories at every depth
docs: add OKF frontmatter to architecture.md
chore: bump golangci-lint to v2.1
```

**Why this matters:** Commits to `main` are analysed by [semantic-release](https://semantic-release.gitbook.io/) to automatically determine the next version number and generate release notes. Non-conventional commits are ignored by the release tool, so your change may ship silently without a version bump.

## OKF Documentation Guide

When contributing documentation in `docs/`, follow the [OKF Standard](/docs/okf-standard.md). The key rules:

### Frontmatter

Every `docs/*.md` file (except `index.md`) must begin with a YAML frontmatter block containing at minimum a `type` field:

```yaml
---
type: Architecture
title: Your Document Title
description: One sentence describing what this covers.
tags: [relevant, tags]
timestamp: 2026-07-15T00:00:00Z
---
```

Valid `type` values: `Architecture`, `Playbook`, `Configuration`, `API Reference`, `Metrics Reference`, `Log`.

`index.md` is a special case — it must have **no** frontmatter and contain a plain markdown list of links.

### Update obligation

When modifying an existing doc:

1. Update the `timestamp:` field to the current date in ISO 8601 format.
2. Add a dated entry to `docs/log.md` (newest first, with a bold action prefix: `**Update**`, `**Creation**`, `**Migration**`, `**Deprecation**`).

When creating a new doc: add it to `docs/index.md` and add a creation entry to `docs/log.md`.

These updates belong in the **same commit** as the content change — do not defer them.

### Tags and cross-links

- Tags: lowercase, hyphenated (e.g. `client-setup`, not `client_setup`).
- Cross-links: bundle-relative paths rooted at the repo root (e.g. `[Architecture](/docs/architecture.md)`).

## Pull Request Process

1. **Branch from `main`** and keep your PR focused — one logical change per PR.
2. **All CI checks must pass:** tests (`go test -race ./...`), vet (`go vet ./...`), linting (`golangci-lint run ./...`), security scanning, and commit linting.
3. **Commit messages must follow Conventional Commits** — the CI will reject PRs with non-conforming messages.
4. **Include documentation updates** in the same PR when your change affects documented behavior.
5. **Keep PRs reviewable** — if the diff grows beyond a single coherent change, split it into stacked PRs.

### DCO Sign-off

By contributing, you agree to the [Developer Certificate of Origin (DCO)](https://developercertificate.org/). Every commit must include a `Signed-off-by:` line, certifying that you have the right to submit the work under the project's license.

Add the sign-off automatically:

```sh
git commit -s -m "feat: your commit message"
```

This appends a line like:

```
Signed-off-by: Your Name <your.email@example.com>
```

Make sure the name and email match your git identity:

```sh
git config user.name "Your Name"
git config user.email "your.email@example.com"
```

PRs without DCO sign-off will not be merged.
