<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# What runs where

| Where | What | When |
|---|---|---|
| pre-commit (`.pre-commit-config.yaml`) | gitleaks, gofmt rewrite, and the `scripts/ci` guardrails whose files changed | each commit |
| `make check` | build, vet, gofmt check, every `scripts/ci/*.sh` (minus the skips named in the `Makefile`), golangci-lint, `go test -race -shuffle=on` | by hand, before pushing; the done-gate for a ticket |
| Forgejo Actions (`.forgejo/workflows/ci.yaml`) | `make check` | PRs to `main` and pushes to `main` |

CI calls `make check`, so the `Makefile` is the one list of steps. Its header
names what it deliberately leaves out (mod-tidy, govulncheck, licence check;
`make vendor` covers the first). Guardrail scripts
are found by globbing `scripts/ci/`, so a new script (for example the layer
rules from #12) runs without editing the target.

## Decision: Forgejo Actions

The PR gate is Forgejo Actions (`.forgejo/workflows/ci.yaml`), which runs
`make check`, so local and CI results agree by construction. The runner is
registered as `self-hosted` with no image, so the job sets its own environment:
`container: golangci/golangci-lint:v2.12.2` (Go, gcc, make, curl, git and the
pinned linter), plus `jq` and `nodejs` installed by apt in the first step. Bump
the image tag to bump the linter. Unverified until a run is seen on a real PR.

The `.tekton/` pipelines are inherited from upstream and are not the gate here.
They are left in place, unused for PRs, until someone decides to remove them.
