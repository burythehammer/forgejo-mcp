<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Running the contract cases against a real Forgejo

The contract cases in `internal/forge/forgetest` run against the fake and against the SDK adapter pointed at a stub. A live driver runs the same cases against a real Forgejo instance, so a disagreement between our assumptions and Forgejo shows up as a failing test.

The first slice covers the hook cases (`forgetest.RunHooks`) and is `TestHooksContractLive` in `internal/forge/sdk/contract_live_test.go`.

## Run it

A human runs this, with a token the human supplies:

```sh
FORGEJO_URL=https://forgejo.example.org \
FORGEJO_ACCESS_TOKEN=<token> \
go test ./internal/forge/sdk -run TestHooksContractLive -count=1 -v
```

- If either variable is unset, the test skips. `go test ./...` never needs a server or a token.
- The token is supplied by the human, typed or exported from their own password manager. An agent never reads a token from a file and never searches for one.
- The token's user needs permission to create and delete repositories. Each case creates `contract-<unix nanoseconds>` under that user and deletes it in cleanup, including when the case fails. Existing repositories are never touched.

## When the live run disagrees

Where a live run disagrees with the fake or the stub, correct the fake or stub to match Forgejo and keep the case. The live run is the source of truth; the offline drivers should get more truthful over time.

## Adding a domain

Add a `TestXxxContractLive` next to `TestHooksContractLive`, call `forgetest.RunXxx` with `liveHarness`, and extend `liveSeed` with the seeding that domain needs. Any Seeder method not implemented by `liveSeed` panics through the nil embedded interface, which marks what is still missing.
