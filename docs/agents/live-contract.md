<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Running the contract cases against a real Forgejo

The contract cases in `internal/forge/forgetest` run against the fake and against the SDK adapter pointed at a stub. A live run of the Forgejo adapter runs the same cases against a real Forgejo instance, so a disagreement between our assumptions and Forgejo shows up as a failing test.

The first slice covers the hook cases (`forgetest.RunHooks`) and is `TestHooksContractLive` in `internal/forge/sdk/contract_live_test.go`. The wiki cases (`forgetest.RunWiki`) are `TestWikiContractLive` in the same file.

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

- **Versions.** A matrix of `16.0.3` (the oldest supported: the 16.0 floor the README and `resource-server` mode enforce) and `16.0.6` (the newest 16.x release). v17.0.0 exists, but its release notes advise against upgrading, so it is not in the matrix yet; add it when that changes.
- **Token and repo.** `test/live/bootstrap.sh` finishes the install form inside the job (a generated admin password that is never written anywhere), mints an API token for that admin, creates the `e2e-attachments` repo and writes the coordinates to `$GITHUB_ENV`. No credentials are stored in the repo or in CI secrets. `bash test/live/bootstrap_test.sh` checks the script offline with a fake `curl`.
- **Steps.** The job then runs `TestHooksContractLive` and `test/e2e/attachments.sh` (with `OWNER=ci-bot REPO=e2e-attachments`).
- **Failure.** A failed run turns the workflow red, and also comments the run link on tracking issue #263 with the built-in `GITHUB_TOKEN`.

Not yet run against a real server. The first real run checks the install-form field names, the `scopes: ["all"]` token request and the service-container networking on both versions.

## When the live run disagrees

Where a live run disagrees with the fake or the stub, correct the fake or stub to match Forgejo and keep the case. The live run is the source of truth; the offline runs should get more truthful over time.

## Adding a domain

Add a `TestXxxContractLive` next to `TestHooksContractLive`, call `forgetest.RunXxx` with `liveHarness`, and extend `liveSeed` with the seeding that domain needs. Any Seeder method not implemented by `liveSeed` panics through the nil embedded interface, which marks what is still missing.
