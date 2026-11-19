<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Running the contract cases against a real Forgejo

The contract cases in `internal/forge/forgetest` run against the fake and against the SDK adapter pointed at a stub. A live run of the Forgejo adapter runs the same cases against a real Forgejo instance, so a disagreement between our assumptions and Forgejo shows up as a failing test.

The slices are the hook cases (`forgetest.RunHooks`, `TestHooksContractLive`) and the wiki cases (`forgetest.RunWiki`, `TestWikiContractLive`), both in `internal/forge/sdk/contract_live_test.go`. The wiki cases were first run against Forgejo 16.0.4; they corrected the fake in four places (create of an existing page is a 400, an edit of an unknown page creates it, revision `limit` is ignored, and hyphenated titles are stored as `<title>.-`). The wiki cases (`forgetest.RunWiki`) are `TestWikiContractLive` in the same file.

### Forgejo 17.0.2 result

Run against Forgejo `17.0.2` (a local container, sqlite, install lock on, a token for an admin account, as the nightly bootstrap does). Both slices pass after the fake was corrected:

- `TestWikiContractLive`: all 13 cases pass, logs `wiki contract run against Forgejo 17.0.2+gitea-1.22.0`. No difference from the fake.
- `TestHooksContractLive`: 15 cases (the test-delivery case is split in two). Two differences from the fake, both fixed in the fake (`internal/forge/fake/hook.go`) with the contract changed to match (`internal/forge/forgetest/hooks.go`):
  - A subscription to `pull_request` is stored and served as the whole pull_request group (ten events: `pull_request` and its nine `pull_request_*` sub-events), on create and on edit. The fake kept the single name.
  - A test delivery in a repository with no commits is refused with 404 `repository is empty`. The fake accepted it. The success case now gives the repository a file first, through the live seeder's `SetFile`.

Both differences also show on Forgejo `16.0.6`, so they are not 17-only; the corrected hook slice passes on `16.0.6` as well. Mixed event lists (for example `push` with `pull_request`) come back in Forgejo's own order, which the fake does not model; no case exercises that.

## Run it

A human runs this, with a token the human supplies:

```sh
FORGEJO_URL=https://forgejo.example.org \
FORGEJO_ACCESS_TOKEN=<token> \
go test ./internal/forge/sdk -run TestHooksContractLive -count=1 -v
```

For the wiki cases, use `-run TestWikiContractLive` in place of `TestHooksContractLive`. The run logs `wiki contract run against Forgejo <version>`; record that version with the result, because the wiki REST behaviour is checked per Forgejo version (the README records 15.0.4 and 16.0.0 as live-tested).

- If either variable is unset, the test skips. `go test ./...` never needs a server or a token.
- The token is supplied by the human, typed or exported from their own password manager. An agent never reads a token from a file and never searches for one.
- The token's user needs permission to create and delete repositories. Each case creates `contract-<unix nanoseconds>` under that user and deletes it in cleanup, including when the case fails. Existing repositories are never touched.

## Nightly job

`.forgejo/workflows/nightly-live.yaml` runs the same suite nightly (03:17 UTC) and on manual dispatch, against a Forgejo service container. It is separate from the PR gate (`ci.yaml`) and never runs on pull requests.

- **Versions.** A matrix of `16.0.3` (the oldest supported: the 16.0 floor that docs/configuration.md states for `resource-server` mode), `16.0.6` (the newest 16.x release) and `17.0.2` (the latest stable 17.x). The matrix covers both sides of the 16/17 line.
- **Token and repo.** `test/live/bootstrap.sh` finishes the install form inside the job (a generated admin password that is never written anywhere), mints an API token for that admin, creates the `e2e-attachments` repo and writes the coordinates to `$GITHUB_ENV`. No credentials are stored in the repo or in CI secrets. `bash test/live/bootstrap_test.sh` checks the script offline with a fake `curl`.
- **Steps.** The job then runs `TestHooksContractLive` and `test/e2e/attachments.sh` (with `OWNER=ci-bot REPO=e2e-attachments`).
- **Failure.** A failed run turns the workflow red, and also comments the run link on tracking issue #263 with the built-in `GITHUB_TOKEN`.

The nightly job has not run yet. The local runs above used a container and the bootstrap's own admin and token steps by hand, so the first nightly run still checks the install-form field names, the `scopes: ["all"]` token request and the service-container networking on all three versions.

## When the live run disagrees

Where a live run disagrees with the fake or the stub, correct the fake or stub to match Forgejo and keep the case. The live run is the source of truth; the offline runs should get more truthful over time.

## Adding a domain

Add a `TestXxxContractLive` next to `TestHooksContractLive`, call `forgetest.RunXxx` with `liveHarness`, and extend `liveSeed` with the seeding that domain needs. Any Seeder method not implemented by `liveSeed` panics through the nil embedded interface, which marks what is still missing.
