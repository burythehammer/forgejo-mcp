<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# What runs where

| Where | What | When |
|---|---|---|
| pre-commit (`.pre-commit-config.yaml`) | gitleaks, gofmt rewrite, and the `scripts/ci` guardrails whose files changed | each commit |
| `make check` | build, vet, gofmt check, every `scripts/ci/*.sh` (minus the skips named in the `Makefile`), golangci-lint, `go test -race -shuffle=on` | by hand, before pushing; the done-gate for a ticket |
| Tekton `go-ci` (`.tekton/tasks/go-ci.yaml`, run by `.tekton/on-pull-request.yaml`) | the above, plus `go mod tidy` drift, govulncheck and the licence check | PRs to `main` that touch more than `docs/` or `*.md` |

`make check` mirrors `go-ci`. When you add or change a step in one, change the
other: the Tekton task is the source of truth for what a PR must pass, and the
`Makefile` header lists what `make check` deliberately leaves out (mod-tidy,
govulncheck, licence check; `make vendor` covers the first). Guardrail scripts
are found by globbing `scripts/ci/`, so a new script (for example the layer
rules from #12) runs without editing the target.

## Does a PR trigger the pipeline?

**Not verified.** Pipelines-as-Code triggers on `pull_request` events to `main`
through the CEL expression in `on-pull-request.yaml`, but the commit statuses on
recent `main` merge commits are empty, and the head commit of a recent PR was
not inspected. Run `scripts/ci/check-pr-ci-ran.sh <pr>` against a real PR: a
single pending "op1st Pipelines as Code" status means the author is not in
`OWNERS` and no checks ran. Tracked in #93.

## Decision: Tekton, not Forgejo Actions

Tekton stays the PR gate. Forgejo Actions is not adopted.
