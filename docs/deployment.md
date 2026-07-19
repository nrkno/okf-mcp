---
type: Playbook
title: Deployment
description: How to build, install, run, test, validate, and release okf-mcp — from local development to production binaries.
tags: [deployment, build, release, go, binary, install, validate, pre-commit]
timestamp: 2026-07-18T00:00:00Z
---

# Deployment

## Prerequisites

- Go 1.26.5 or later (the version in `go.mod`).

## Build from source

```sh
go build ./cmd/okf-mcp
```

This produces an `okf-mcp` binary in the current directory, dynamically linked against the host libc.

## Static binary (recommended for containers)

```sh
CGO_ENABLED=0 go build ./cmd/okf-mcp
```

Disabling CGO produces a fully static binary with no external library dependencies. Recommended for container images and portable distribution.

## Install via `go install`

```sh
go install github.com/nrkno/plattform-okf-mcp/cmd/okf-mcp@latest
```

Installs the binary to `$GOPATH/bin` (or `$HOME/go/bin` if `GOPATH` is not set). Ensure that directory is on your `$PATH`.

## Release binaries

Pre-built binaries are attached to every GitHub release. Platforms and architectures:

| OS | Architectures |
|----|--------------|
| Linux | amd64, arm64 |
| macOS (darwin) | amd64, arm64 |
| Windows | amd64, arm64 |

Archives are named `okf-mcp_<version>_<os>_<arch>.tar.gz` (`.zip` for Windows). A `checksums.txt` file is included in each release for verification.

## Running

`okf-mcp` must be run from the repository root you want to index — the working directory is the scan root.

```sh
cd /path/to/your/repo
okf-mcp
```

On startup it prints to stderr:

```
okf-mcp: serving /path/to/your/repo
```

In practice, the MCP host (opencode, Claude Desktop) starts `okf-mcp` as a subprocess and manages its lifecycle. The working directory is controlled by the host configuration — see [Configuration](/docs/configuration.md) for how to set it correctly.

## Validating docs (`--validate`)

`okf-mcp` can validate OKF-conformant documents without starting the MCP server. Use the `--validate` flag:

```sh
okf-mcp --validate              # validate the current working directory
okf-mcp --validate --path docs/ # validate a specific path
```

The binary reads all `.md` files, checks frontmatter conformance, and prints findings to stderr. It exits before starting the MCP server.

### Exit codes

| Exit code | Meaning |
|-----------|---------|
| 0 | All files conformant (zero errors, may have warnings) |
| 1 | One or more errors found |
| 2 | Infrastructure failure (bad path, scan error) |

### Pre-commit hook

A pre-commit hook is included at `.githooks/pre-commit` that validates the entire bundle before each commit. Install it with:

```sh
git config core.hooksPath .githooks
```

The hook requires `okf-mcp` to be on your `$PATH`. If the binary is not found, the hook prints an installation reminder and exits with code 1. If validation finds errors, the commit is blocked (exit 1). Warnings do not block the commit.

## Development

Run the test suite:

```sh
go test ./...
```

Run tests with the race detector:

```sh
go test -race ./...
```

Vet the code:

```sh
go vet ./...
```

## Release process

Releases are fully automated:

1. Pull requests use [Conventional Commits](https://www.conventionalcommits.org/) format (`feat:`, `fix:`, `chore:`, etc.).
2. On merge to `main`, [semantic-release](https://semantic-release.gitbook.io/) analyses commits and cuts a new semver version.
3. [GoReleaser](https://goreleaser.com/) builds binaries for all six platform/architecture combinations and attaches them to the GitHub release.

To trigger a release, merge a conventional-commit PR to `main`. No manual tagging is required.
