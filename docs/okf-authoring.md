---
type: Playbook
title: Portable OKF Authoring Skill
description: Install and maintain the reusable okf-authoring skill, its formatting-only assessment, synchronized portable references, and artifact checks.
tags: [okf, skill, authoring, installation, assessment, references]
timestamp: 2026-10-10T21:10:00Z
---

# Portable OKF authoring skill

The `skills/okf-authoring/` directory is a portable methodology package: `SKILL.md` teaches discovery, evidence-backed relationships, profiles, document formatting, index coverage, log currency and distinct verification claims. `ASSESSMENT.md` assesses formatting/structure and applicable profile rules, **not substantive content correctness**. No QA companion or additional governance is introduced.

## Install

Copy the **entire** directory, including references and license, into a skill location supported by your harness. For OpenCode V2, a project install is `.opencode/skills/okf-authoring/`; a user install is `~/.config/opencode/skills/okf-authoring/`. Alternatively register the repository's `skills` directory as a local skill source. See the current [OpenCode skills reference](https://opencode.ai/v2/docs/skills).

Confirm the catalog advertises `okf-authoring`, then load that exact ID. Supporting reference contents are not loaded automatically. The practitioner and assessment use direct skill-relative links, not paths to implementation files in this source repository. A live profile-capable `okf-mcp` connection is needed for observed graph verification; copying the skill does not install or connect the server. Use [profile setup](/docs/okf-profile.md) for server configuration.

## Maintained reference ownership

The portable references are mechanically synchronized snapshots, not independent schema authorities:

| Maintained source | Packaged reference | Transformation |
|---|---|---|
| `docs/okf-standard.md` | `references/okf-standard.md` | Document body without corpus frontmatter/leading blank line |
| `docs/okf-profile-format.md` | `references/profile-format.md` | Same body extraction; replace setup link target `/docs/okf-profile.md` with `profile-setup.md` |
| Root `LICENSE` | Skill `LICENSE` | Exact bytes |

Edit the maintained sources first, then update the packaged snapshots with only the listed transformation. The packaged references are skill assets, not duplicate corpus publications. `TestOKFAuthoringPortablePackage` rejects drift and checks a copy installed away from this repository. Do not maintain separate schema tables in the practitioner or assessment. When installed elsewhere, snapshots remain readable without the source checkout; compare target repository rules/server capabilities before applying version-sensitive claims.

## Verification boundaries

`references/profile-setup.md` is a curated host-neutral procedure grounded in `docs/okf-profile.md`, not a schema snapshot or a promise to include OpenCode-specific configuration. Maintain its executable/working-directory, explicit-profile, restart and verification instructions alongside that source. The field reference includes complete linked-document examples. The practitioner contains an evidence ledger, vocabulary alternatives, justified omissions and a complete mixed-corpus profile; package tests exercise that actual profile through MCP, not only its file existence.

Native `SKILL.md` metadata is intentionally not OKF content frontmatter. Single-file document validation reports E2 for its missing `type`, even though it is valid harness metadata. Whole-corpus CLI validation instead silently skips files without an indexed type, including native skills, reference assets and accidentally malformed content documents. This repository's existing root-wide pre-commit hook remains unchanged; its success does not prove native metadata or inventory conformance.

The skill separately requires index completeness and current-date log entries, which CLI success alone does not prove. Nested bundle ownership, reserved-file rules, timestamps and native asset classification must be checked from artifacts and repository conventions. Profile graph checks are separate from document conformance; neither establishes content truth.

## Cases and checks

`skills/okf-authoring/evals/cases.json` defines observable positive/negative expectations for profile mismatch, missing index entries, stale logs, nested bundles/native metadata, substantive-review negative space and offline portable use. These are eval inputs, **not claimed ablation results**. The actual invalid-profile incident (missing relationship, conflicting direction and null maximum) provides RED evidence; it is not a fabricated baseline run of this skill.

The corpus envelope is `skill: "okf-authoring"`, `version: "1.0.0"`, `shape: 1`, and a `cases` array. Each case has a unique `id` and `name`, concrete `trap`, inline-response `prompt`, and nonempty string arrays `expectations` and `negative_expectations`. Incident lineage belongs in the trap prose, not an unsupported metadata key. Prompts require no filesystem or shell mutations; actual native skill loading needs separate central integration evidence.

```sh
go test -race ./cmd/okf-mcp -run 'TestOKFAuthoring' -count=1
```

Repository tests check reference synchronization, native metadata, relocated direct references and actual validator blind spots. The local strict checker deliberately enforces this package's selected published shape, not the central Loom loader's complete accepted language: that loader also normalizes legacy inputs. Tests pin this published package to version `1.0.0` separately; nonempty version metadata such as `2.0.0` is not malformed or an unsupported harness schema. No SemVer compatibility rule is inferred. Nine cases now include mixed-corpus discovery and require actual inline artifacts where requested; explanation alone cannot pass those artifact criteria. Skill retrieval and agent decision quality require separate behavioral evaluation; passing artifact tests does not claim those results.
