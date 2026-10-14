---
type: Playbook
title: Portable OKF Authoring Skill
description: Install and maintain the portable okf-authoring skill for selective agent consumption, document representation, safe parallel updates, and distinct structural checks.
tags: [okf, skill, authoring, consumption, parallel-work, assessment, references]
timestamp: 2026-10-14T18:47:58.306Z
---

# Portable OKF authoring skill

The `skills/okf-authoring/` directory is a portable methodology package: `SKILL.md` teaches bounded **consumption** of indexed documentation for agent tasks, evidence-backed relationships, profiles, formatting, index/log maintenance, **parallel change convergence**, and distinct verification claims. `ASSESSMENT.md` assesses formatting/structure, retrieval method and applicable profile rules, **not substantive content correctness**. No QA companion or additional governance is introduced.

## Use for independent and parallel work

Agents should first verify the scan root, search selectively by topic/type/tags, inspect a bounded graph neighborhood, and read the authoritative source documents. Then they compare intended behavior with current code/tests, and pass relevant source paths, shared contracts, open questions and verification expectations to independent implementers. Neither a graph edge nor a successful format validator proves the implementation satisfies the user.

During parallel work, content authors own their document edits. Agree on relationship names before dependent changes; assign a coordinating/integrating owner for shared `index.md`, `log.md` and `.okf-profile.yaml` edits. Reconcile links, inventories and log entries against the integrated tree before reporting document/graph conformance. This does not impose any task graph, role, approval ceremony or new documentation class.

## Upstream and local format versions

The shipped [local OKF standard](/docs/okf-standard.md) and synchronized skill reference describe this repository's established **v0.1-based** conventions, not the whole upstream specification. The [upstream OKF v0.2 specification](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/main/SPEC.md) adds optional provenance, verification and lifecycle metadata, permits a bundle-root `index.md` version declaration, and does not universally require `log.md` frontmatter.

The current server's reserved-file validator still has stricter local rules (`index.md` without frontmatter and `log.md` with `type: Log`), and its graph resolver documents leading-slash links relative to the **scan root**, rather than the upstream spec's **bundle root**. Check actual server version/profile and link behavior instead of assuming full upstream v0.2 conformance or silently changing the target repository's files. This update changes the **skill guidance and eval fixtures**, not the server parser, validator or resolver.

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

`skills/okf-authoring/evals/cases.json` defines observable positive/negative expectations for profile mismatch, missing index entries, stale logs, nested bundles/native metadata, bounded consumption, parallel updates, v0.2/local compatibility, substantive-review negative space and offline portable use. These are eval inputs, **not claimed ablation results**. The actual invalid-profile incident (missing relationship, conflicting direction and null maximum) provides RED evidence; it is not a fabricated baseline run of this skill.

The corpus envelope is `skill: "okf-authoring"`, `version: "1.0.0"`, `shape: 1`, and a `cases` array. Each case has a unique `id` and `name`, concrete `trap`, inline-response `prompt`, and nonempty string arrays `expectations` and `negative_expectations`. Incident lineage belongs in the trap prose, not an unsupported metadata key. Prompts require no filesystem or shell mutations; actual native skill loading needs separate central integration evidence.

```sh
go test -race ./cmd/okf-mcp -run 'TestOKFAuthoring' -count=1
```

Repository tests check reference synchronization, native metadata, relocated direct references and actual validator blind spots. The local strict checker deliberately enforces this package's selected published shape, not the central Loom loader's complete accepted language: that loader also normalizes legacy inputs. Tests pin this published package to version `1.0.0` separately; nonempty version metadata such as `2.0.0` is not malformed or an unsupported harness schema. No SemVer compatibility rule is inferred. Twelve cases now include mixed-corpus discovery and require actual inline artifacts where requested; explanation alone cannot pass those artifact criteria. Skill retrieval and agent decision quality require separate behavioral evaluation; passing artifact tests does not claim those results.
