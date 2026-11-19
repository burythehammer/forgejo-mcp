<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# 0009. Releases are cut by a hand-pushed tag, not semantic-release

Status: accepted

## Context

Upstream (b4mad) releases with semantic-release: an OpenShift Job reads conventional commits, computes the version, tags `main` and pushes, and a Tekton pipeline builds and signs the assets. None of that infrastructure reaches this fork's forge (ADR 0001 made the break from upstream), and nothing in it had been shown to work here.

## Decision

A release is a `vX.Y.Z` tag that the maintainer chooses and pushes by hand. A Forgejo Actions workflow (`.forgejo/workflows/release.yaml`) reacts to the tag, builds the archives, checksums, SBOMs and `.mcpb` bundles with goreleaser, and creates the Release. Conventional Commits stay as the convention for choosing the version, but no tool computes it. Upstream's release machinery (Tekton release pipelines, the OpenShift Job, castra) is removed. `.releaserc` follows once #236 has moved its commit-type list into the commit-title check.

## Alternatives considered

- **Keep semantic-release and re-host its Job on this forge.** Rejected: it needs an OpenShift cluster, a release agent and a PaC setup this project does not otherwise run, to automate a step that happens a few times a year.
- **Run semantic-release inside a Forgejo Actions workflow.** Rejected for now: it adds a Node toolchain, a token able to push to `main`, and a version-bump rule set to maintain, with no need that hand-tagging does not already meet. It can be revisited if release frequency makes picking versions by hand a burden.

## Consequences

- The maintainer picks each version from the commit log, so a wrong bump is a human error, not a commit-title error. The commit-title check (#236) still guards the prefixes that guidance relies on.
- Release notes come from goreleaser's generated notes; `CHANGELOG.md` is frozen as history.
- Signing, container image publish and a changelog commit are not part of this flow yet.
