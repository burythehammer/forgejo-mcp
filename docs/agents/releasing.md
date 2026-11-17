<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Releasing

Releases are cut by `.forgejo/workflows/release.yaml` when a `vX.Y.Z` tag is pushed to `origin`. Upstream's flow (OpenShift Job, Tekton, `castra:release`) is not used here; see [docs/legacy/upstream-release.md](../legacy/upstream-release.md).

Do not cut a release unless asked.

## Cut one

1. `make check` green on an up-to-date `main`.
2. Pick the version from the commits since the last tag (`git log $(git describe --tags --abbrev=0)..HEAD --format=%s`): `feat:` is a minor bump, `fix!:` or a `BREAKING CHANGE` footer is a major one, otherwise patch.
3. `git tag -s vX.Y.Z && git push origin vX.Y.Z`.
4. Watch the `release` workflow run. It builds the archives, checksums and SBOMs, creates the Release with notes from the commit log, then packs and attaches four `.mcpb` bundles.

A complete release carries 13 assets: 4 tarballs, 4 SBOMs, 4 `.mcpb` and the checksums file.

## Setup

The workflow reads the repo Actions secret `RELEASE_TOKEN`, a personal access token with `write:repository`. The owner creates it under the repo's Settings, Actions, Secrets.

## Not done yet

cosign signing, container image publish, and a changelog commit.
