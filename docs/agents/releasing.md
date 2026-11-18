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

## What the runner can reach

The job container cannot reach the public hostname `forgejo.burythehammer.com`: it resolves to a Tailscale address and the connection times out. It does reach the forge at `http://forgejo:3000`, the host the runner's own checkout uses. So every API call in the job (goreleaser's `gitea_urls.api` in `.goreleaser.yml`, and `FORGEJO_API` in the workflow) uses the internal address, while `gitea_urls.download` stays public because goreleaser only writes it into release-note links. A first step probes `/version` so an unreachable forge fails in seconds. The address is specific to this runner; if a second runner appears, make it configurable then.

A run that dies on `dial tcp 100.x.x.x:443: connect: connection timed out` is this problem: check that no new URL in the workflow or goreleaser config uses the public hostname.

## Not done yet

cosign signing, container image publish, and a changelog commit.
