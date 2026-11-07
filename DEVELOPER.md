# Developer Guide

This guide covers building, developing, and contributing to the Forgejo MCP Server.

## Prerequisites

- Go 1.24 or later
- make (optional, for convenience commands)

## Building

### Using Make

```bash
make build          # Build the binary (outputs ./forgejo-mcp)
make vendor         # Tidy and verify Go module dependencies
```

### Using Go Directly

```bash
go build -v         # Build the binary
go mod tidy         # Tidy dependencies
```

## Running Locally

```bash
# stdio mode (for MCP client integration)
./forgejo-mcp --transport stdio --url https://forgejo.example.org --token <token>

# SSE mode (for HTTP-based clients)
./forgejo-mcp --transport sse --url https://forgejo.example.org --token <token> --sse-port 8080

# With debug logging
./forgejo-mcp --transport sse --url <url> --token <token> --debug
```

Environment variables: `FORGEJO_URL`, `FORGEJO_ACCESS_TOKEN`, `FORGEJO_DEBUG`, `FORGEJO_USER_AGENT`, `FORGEJO_TLS_CA`, `FORGEJO_TLS_SKIP_VERIFY`, `FORGEJO_MCP_AUTHORIZATION_SERVER_TLS_CA` (see the README's "Private or self-signed Forgejo certificates")

CLI options: `--url`, `--token`, `--transport`, `--sse-port`, `--http-port`, `--user-agent`, `--tls-ca`, `--tls-skip-verify`,
`--host`, `--allowed-hosts`, `--allowed-origins`, `--allow-operator-token-fallback`

The last four govern the `sse` and `http` transports. `--host` defaults to `127.0.0.1`
and binds both loopback families; a non-loopback value requires `--allowed-hosts` or the
server refuses to start. On these transports every request must carry its own
`Authorization` header unless `--allow-operator-token-fallback` is set. See the
"Exposing the server to a network" section of README.md.

## Architecture

This is an MCP (Model Context Protocol) server that exposes Forgejo API operations as tools for AI assistants.

### Core Flow

```
main.go → cmd/cmd.go (CLI parsing) → internal/operation/operation.go (tool registration) → internal/operation/{domain}/*.go (MCP adapters)
```

### Directory Structure

| Directory | Purpose |
|-----------|---------|
| `cmd/` | CLI entry point and command parsing |
| `internal/operation/` | MCP adapters (tool definitions, argument decoding, rendering), organized by domain; tool registration and `Run`, which hands the server to `transport` |
| `internal/transport/` | Serve loop for stdio, SSE and streamable HTTP; listener hardening (Host and Origin policy, bind validation, refusal limiter, request guard); the authentication port (passthrough and resource-server modes). Built from `config.Config` only |
| `internal/operation/issue/` | Issue-related tools |
| `internal/operation/pull/` | Pull request tools |
| `internal/operation/release/` | Release and release-attachment tools |
| `internal/operation/repo/` | Repository and branch tools |
| `internal/operation/search/` | Search tools (users, repos, teams) |
| `internal/operation/user/` | User info tools |
| `internal/operation/version/` | Server version tool |
| `internal/forge/` | The Forgejo port: per-domain interfaces and the `Factory` (no SDK client import) |
| `internal/forge/sdk/` | The Forgejo adapter (the real one): `Factory` (credential policy, startup checks) and `Adapter` over the SDK and one raw-HTTP client; the only package that constructs the SDK client or the raw transport. SDK types may appear in `internal/forge/*` and on the MCP adapter side (`internal/operation/*`, `internal/forge/*adapter/`), never under `internal/app/**` |
| `internal/forge/fake/` | The fake Forgejo adapter: a stateful in-memory `forge.Forgejo` for tests |
| `internal/forge/forgetest/` | Contract suite run against the fake and the SDK Forgejo adapter, and the shared stub server |
| `internal/app/<domain>/` | Use cases (ADR 0003): typed inputs, a domain port, repo-owned results. No mcp-go or SDK import (depguard in `.golangci.yml`) |
| `internal/app/model/` | Repo-owned domain types that use cases return (wire-shaped copies of the SDK types) |
| `internal/app/split/` | The comma-separated list parser the use cases share |
| `internal/operation/decode/`, `internal/operation/mcptest/` | Argument decoding into per-tool structs; golden, schema-snapshot and args-vs-schema test helpers |
| `internal/forge/<domain>adapter/` | Port bridge from a use case's port to `forge.<Domain>`: the one place SDK and repo-owned types are translated |
| `internal/to/` | Response formatting helpers (`TextResult`, `ErrorResult`) |
| `internal/operation/params/` | Shared parameter descriptions for tool definitions |
| `internal/config/` | Typed Config, `Load` and `Validate` |
| `internal/log/` | Structured logging utilities |

### Dependency rule

Imports point one way. The depguard rules in `.golangci.yml` enforce it in the lint step (`make check` and CI):

- `internal/app/` imports only `internal/app`, the port package `internal/forge` (its root, not its subpackages), `internal/ptr` and `internal/diff`. It never imports mcp-go or the Forgejo SDK.
- `internal/operation/` does not import `internal/forge/sdk`, `internal/forge/fake` or `internal/transport`.
- `internal/transport/` does not import `internal/operation` or `internal/app`.
- The port package `internal/forge` (root files) does not import `internal/operation` or `internal/transport`.
- Port bridges (`internal/forge/<domain>adapter/`) may import use cases and are outside the port rule.

Exemptions: test files in `internal/app` are held only to the mcp-go and SDK bans, and test files in `internal/operation` are outside the operation rule, because they wire the fake and SDK adapter to drive the code under test. `internal/operation/operation.go` is the composition root (it builds the SDK factory and calls `transport.Serve`) and is outside the operation rule too.

## The Forgejo port and migrating a domain

Every domain reaches Forgejo through a port. The pull domain
(`internal/operation/pull/`, `internal/forge/pull.go`) is the reference; copy its shape. A
domain has:

1. **Port** (`internal/forge/<domain>.go`). An interface named for what the tools
   need, SDK types as plain data, `ctx` first. Server failures are
   `*forge.StatusError` (use `forge.IsNotFound`); a decline that Forgejo reports
   as a status rather than an error is a result field (see `forge.MergeResult`).
   Embed the interface in `forge.Forgejo` (`internal/forge/forge.go`).
2. **Forgejo adapter** (`internal/forge/sdk/<domain>.go`). Methods on `*Adapter`, a
   one-line `var _ forge.<Domain> = (*Adapter)(nil)`, and every SDK call wrapped
   in `statusErr(resp, err)`. Raw-HTTP workarounds go here too, with
   `forge.APIPath`, through the adapter's single raw path (`a.rawJSON` /
   `a.rawDo` / `a.raw.*`), which targets the adapter's own instance with the
   request's own credential.
3. **Fake** (`internal/forge/fake/<domain>.go`). Methods on `*fake.Forgejo` with its
   own state struct (and mutex) held in one new field on `fake.Forgejo`. It
   must behave like Forgejo: round-trips, 404 for unknown things, pagination via
   `page`. Add seeding helpers for what the port cannot write (the pull fake's
   `AddRepo`, `SetPullRequestDiff`).
4. **Contract cases** (`internal/forge/forgetest/<domain>.go`, `RunXxx`). Behaviour
   through the port only, never HTTP traffic. Add the seeding methods to
   `Seeder`, then give the stub server the REST routes
   (`forgetest/stub_<domain>.go`, a thin translation onto `Stub.Fake`) and call
   `RunXxx` from `fake/contract_test.go` and `sdk/contract_test.go`. The stub
   answers the SDK startup probe in one place, `forgetest.WriteVersion`.
   To run the same cases against a real Forgejo (opt-in, needs a token the
   human supplies), see [docs/agents/live-contract.md](docs/agents/live-contract.md).
5. **MCP adapters take the factory.** `RegisterTool(s, f forge.Factory)` /
   `RegisterResource(s, f)`, a `handlers{forge: f}` struct (in each `internal/operation/<domain>/handlers.go`) whose methods call
   `h.forge.For(ctx)` per request (so each request gets its own credential), and
   thread `f` through `internal/operation/operation.go` and `cmd/cli.go`.
6. **MCP adapter tests on the fake** (`internal/operation/<domain>/*_test.go`). Build a fresh
   `fake.New()` per test, call the MCP adapter, assert on the tool result, and
   `t.Parallel()`. No `flag.*`, no package-level Forgejo state, no `httptest`
   server.

The composition root (`operation.Run`, `cmd/cli.go`) builds one
`sdk.NewFactory(cfg.Forgejo, version)` from the typed Config and passes it into
every `RegisterTool`/`Register...Resource`. The factory holds no credential
policy: it serves a request's own token, or the shared operator client for a
context marked by `forge.WithOperatorCredential`, and refuses anything else.
Whether a request with no token of its own may use the operator's credential
(the `-allow-operator-token-fallback` decision) is the authentication port's, in
`internal/transport/authport.go`, which sets that mark. `Factory.VerifyConnection(ctx)` is
the startup gate `transport.Serve` calls before serving.

## Splitting a domain into MCP adapters and use cases

ADR 0003 splits every tool into an MCP adapter (`internal/operation/<domain>`) and an
MCP-free use case (`internal/app/<domain>`). `version` is the smallest example, `pull`
(all 16 pull tools) the reference for a domain with a port, and `issue` (the 16
issue tools with their 8 label and 4 dependency tools) the one with the most
moving parts: label-name resolution and the `update_issue` ordering both live
in `internal/app/issue`. Per domain:

1. **Pin first.** Before moving anything, write the characterisation tests
   against the existing code: `mcptest.SchemaSnapshot` for each tool and
   `mcptest.Golden` for each case's output and error text (see
   `internal/operation/pull/golden_test.go`: a stub `forge.Forgejo` returning fixed,
   fully populated SDK data and recording the port calls, rendered together).
   Create the files with `go test ./internal/operation/<domain> -update`. They must pass
   unchanged after the move: tool names, schemas, output bytes and error text
   are frozen.
2. **Use case** (`internal/app/<domain>`): a `Port` interface in repo-owned types
   (`port.go`), a `Service` with one method per tool taking a typed input
   (`service.go`; defaults and parsing live here). Failures a caller can act on
   are the shared `forge` kinds (`forge.ErrNotFound`, matched with
   `forge.IsNotFound` or `errors.Is`); a domain error type in `errors.go` is
   only for an outcome those kinds cannot carry (ADR 0006). Text must stay what
   the tool said, and Forgejo failures are wrapped `fmt.Errorf("<op> err: %w", err)`.
   Results come from `internal/app/model`; add the wire-shaped types a domain
   needs there.
3. **Port bridge** (`internal/forge/<domain>adapter`): `New(forge.<Domain>) app.Port`
   translates inputs to SDK options and converts SDK results to `internal/app/model`
   values through JSON. `shape_test.go` there fails if an SDK upgrade changes a
   shape the model copies. The shared `forge.<Domain>` stays SDK-typed (the
   resources and contract suite use it) until #8 retires it.
4. **MCP adapter** (`internal/operation/<domain>/handlers.go`): one argument struct per
   tool (`json:"name"`, `required:"true"` so `ArgsMatchSchema` checks them against the
   schema). Register each tool with `pipeline.Handle` (`internal/operation/pipeline`),
   which decodes the struct, runs its `Validate` method, and only then calls the
   Binder that looks up the credential (rule in
   [docs/agents/adding-a-tool.md](docs/agents/adding-a-tool.md)). The default decoding
   is lax, as the pre-split tools were: a missing or mistyped argument is not
   rejected unless the tool asks for `pipeline.Strict()`. Declare numeric fields
   `float64` and read them with the numeric-argument module in the MCP adapter (see
   **Numeric arguments** below). Both accept string-encoded numbers. Each handler is
   a few lines: a Binder that builds `apppull.New(pulladapter.New(port))` from
   `forge.For(ctx)`, and a run function that calls the use case and renders with
   `mcpresult.Render` / plain text. Use-case errors become MCP errors in one
   function (`mcpresult.Fail`); invalid or missing arguments are a tool error result
   (`mcp.NewToolResultError`). Keep the exported `XxxTool` definitions and
   `XxxToolName` constants as they were.

   Three refinements came with `issue` (`internal/operation/issue/`, 28 tools):
   - **Defaults that test for zero live in the adapter, before truncation.**
     The pre-split tools defaulted `page == 0` on the float, so `0.5` was passed
     on as `0`. `decode.PagingArgs` reads both through `numarg.Paging` (a fraction
     truncates, zero or below is 0, and the caller's default applies); the
     use case does not default a page or limit it is handed. A list use case
     then bounds the page through the list module (`internal/app/list`).
   - **Pointer fields where presence matters.** An argument whose "given but
     empty" differs from "not given" (`labels`, `assignees`, `set_labels`,
     `ref`, a label's `name`, `state` on `list_repo_issues`) is a `*string`
     struct field, nil when absent or wrongly typed. The lax decoding the pipeline
     uses by default zeroes a field whose value fails to decode, so a wrong type
     reads as absent. `mcptest.ArgsMatchSchema` dereferences pointer fields.
   - **Numbers as a string.** The lax decoding reads a numeric string (`"7"`) as
     the number, for a plain or a pointer field. A pre-split tool that asserted
     `float64` and ignored a string tags the field `exact:"true"`, so the
     string reads as absent (the wiki paging and line arguments).
   - **Raw checks for a few numbers.** The dependency tools' index arguments
     were always type-checked (a missing one was an error), which a zero value
     cannot express. Those are read with `to.Float64` from the raw arguments in
     the adapter (`numberArg`), with the old error text.
   - **Validation before the credential.** The pre-split tools checked their
     arguments before calling `h.forge.For`, so a bad request got its input
     error even when no credential resolved. A use case that validates (parses
     a time, checks a duration) takes a `Source` (`func(ctx) (Port, error)`)
     instead of a port, and looks the port up only after its checks. The
     adapter passes `h.source()`. `internal/app/tracking` and `internal/app/user` do this.
   - **Optional numbers.** A pointer to a number reads a JSON number only; a
     numeric string is read only when the field is tagged `numeric:"string"`
     (for the tools that used `to.Float64`). Plain number fields keep reading
     numeric strings. Pick per field to match the pre-split tool.
5. **Tests.** Use cases: `internal/app/<domain>/service_test.go`, an external test
   package over the in-memory fake through the bridge (no SDK import allowed
   under `internal/app/`, so seed through the fake's SDK-free helpers or the use case
   itself). Adapter: the goldens and snapshots, `mcptest.ArgsMatchSchema` per
   argument struct, a thin table test that tools reach their use case, the
   failure paths, and one `httptest` smoke test through the real SDK adapter
   (`forgetest.NewStub`). New tests call `t.Parallel()`. Retire old pre-split
   tests only where a use-case or golden test covers the behaviour.
6. **Docs.** README tool tables stay complete
   (`scripts/ci/check-readme-tool-coverage.sh`); update any spec that names a
   moved symbol.

## Adding a New Tool

A tool is two parts and a bridge (ADR 0003): a use case that knows nothing
about MCP, and an MCP adapter that decodes the request, calls the use case and
renders the result. The pull domain (`internal/app/pull`, `internal/operation/pull`,
`internal/forge/pulladapter`) is the reference; copy its shape.

| Part | Where | Knows about |
|------|-------|-------------|
| Use case | `internal/app/<domain>/` (`service.go`, `port.go`, `errors.go`) | Typed input, its own `Port`, repo-owned results. No mcp-go, no Forgejo SDK |
| Port bridge | `internal/forge/<domain>adapter/` | Converts the use case's `Port` to `forge.<Domain>` (SDK types) and the SDK results back to `internal/app/model` |
| MCP adapter | `internal/operation/<domain>/` | The `mcp.NewTool` definition, the argument struct, decoding, rendering |

`internal/app/**` must not import mcp-go or the Forgejo SDK. The depguard rules in `.golangci.yml` fail the lint on such an import (see "Dependency rule").
fails the build on such an import.

### Step 1: Write the use case

Put the logic in `internal/app/<domain>/service.go`. Defaults and validation live here,
so the CLI and the tests get the same behaviour as MCP:

```go
// internal/app/mydomain/service.go
type ListInput struct {
	Owner, Repo string
	Limit       int
}

func (s *Service) List(ctx context.Context, in ListInput) ([]model.Thing, error) {
	// defaults, validation, then s.port calls
}
```

Results are types from `internal/app/model`; add wire-shaped types for a new domain in
`internal/app/model/<domain>.go`. Failures the caller can act on are the shared
`forge` kinds (ADR 0006: a missing parent is `forge.ErrNotFound`, an empty list is
an empty result, never an error), not a per-domain not-found type. A new Forgejo call needs a method on the use case's `Port`, a
method in `internal/forge/<domain>adapter`, and (if the port itself is new) the
steps in "The Forgejo port and migrating a domain" above.

### Step 2: Write the MCP adapter

```go
// internal/operation/mydomain/handlers.go
type listArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Limit float64 `json:"limit"`
}

var ListThingsTool = mcp.NewTool(
	"list_things",
	mcp.WithDescription("List the things in a repository"),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithNumber("limit", mcp.Description("Page size"), mcp.DefaultNumber(20)),
)

// Validate holds the checks that run before the credential lookup, in the
// order the tool always made them. It is optional: the pipeline calls it when
// the struct has it.
func (a listArgs) Validate() error {
	return nil
}

// bind is the pipeline Binder: it runs only after the arguments are valid.
func (h handlers) bind(ctx context.Context) (*appmydomain.Service, error) {
	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, err
	}
	return appmydomain.New(mydomainadapter.New(port)), nil
}

func (h handlers) listThings() server.ToolHandlerFunc {
	return pipeline.Handle(h.bind, func(ctx context.Context, s *appmydomain.Service, a listArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.List(ctx, appmydomain.ListInput{Owner: a.Owner, Repo: a.Repo, Limit: int(a.Limit)}))
	})
}
```

`pipeline.Handle` (in `internal/operation/pipeline`, shared by every domain) is the one
path a call takes: it decodes the arguments, runs `Validate`, and refuses a bad request
as a tool error before the Binder runs. The Binder calls `h.forge.For(ctx)` for this
request's credential and builds the use case over the bridge. Options: `pipeline.Strict()`
to refuse a missing or mistyped argument, `pipeline.Lax()` to decode forgivingly whatever
the package default, and `pipeline.WithRaw()` with `pipeline.Raw(ctx)` for the few tools
that must tell an absent argument from a present null. A domain does not call
`decode.Lax` or `decode.Args` itself; `scripts/ci/check-decode-boundary.sh` (run by
`make check`) fails on such a call. `mcpresult.Render` (in `internal/operation/mcpresult`)
sends the result with `to.TextResult` and any error through `mcpresult.Fail`, which uses
`to.ErrorResult`.

Argument struct rules:

- `json:"name"` is the MCP argument name. The tag, not the field name, is what
  the decoder reads.
- Numbers are `float64` in the argument struct (the decode boundary). Never
  convert them with a bare `int(...)` or `int64(...)`: read them through
  `internal/numarg` (see **Numeric arguments** below).
- `required:"true"` marks a required argument. The default (lax) decoding never
  rejects a request: a missing or wrongly typed argument leaves its field at the
  zero value, which is what the hand-written tools saw. `mcptest.ArgsMatchSchema`
  checks the tags against the tool's schema.
- Use a pointer field (`*string`, `*float64`) only where "given but empty"
  differs from "not given". A wrongly typed value reads as absent.
- A numeric string (`"7"`) is read for a plain number field and for a pointer
  field tagged `numeric:"string"`. Tag a number `exact:"true"` to refuse it, to
  match a pre-split tool that asserted `float64` strictly. Copy the pre-split tool's rule.
- A new tool is registered with `pipeline.Handle`; the rule, and the number rule
  with its two exceptions, is stated once in
  [docs/agents/adding-a-tool.md](docs/agents/adding-a-tool.md). Strict decoding
  (`pipeline.Strict()`, a missing or mistyped argument refused in one error) is
  chosen per tool; the argument-less version tool is its one caller today.

### Numeric arguments

One rule, one module: `internal/numarg` turns a raw JSON number into a validated
whole number. It is MCP-free and SDK-free, so a use case may import it, though
the adapter does the conversion and the use case receives `int` or `int64`.

- An ID, index or count that must name something: `numarg.RequiredPositiveInt`
  (wording "<name> is required" / "<name> must be a positive integer"). Use it
  for new code. `numarg.PositiveID` asks the same question (a required value from
  1 to the largest int64) but its range error reads "<name> must be an integer
  between 1 and 9223372036854775807"; the pull and issue adapters keep it because
  their goldens pin that wording. Do not use it where the optional form
  is needed: `numarg.PositiveInt` (absent is not an error) is that form.
- A bounded value (a page size, a byte count): `numarg.IntDefault(name, raw, def,
  numarg.Bounds{Min, Max})`, wording "<name> must be an integer between <min> and <max>".
- A non-negative value where zero means "none": `numarg.NonNegativeInt`.
- A page or limit of a list tool: `decode.PagingArgs` (wraps `numarg.Paging`,
  which clamps and truncates, so a fraction is passed on as its truncation).
- A seconds value that is truncated but must stay in range: `numarg.Whole`.
- A value whose pinned wire behaviour truncates a fraction without a range
  check (`required_approvals` 2.7 is 2): `numarg.Truncate`. Use it only where a
  golden pins that behaviour; new code uses `Int` or a helper above.
- A tool whose error text depends on how a number was given keeps the argument
  typed `any` and passes it to the module. This is the one documented exception
  to `float64` in an argument struct; the module offers that path instead of a
  second one.
- The `id` of `edit_milestone` and `get_milestone` is a `decode.Arg`, the other
  documented exception: its goldens pin "id must be an integer between 1 and
  9223372036854775807" (`numarg.PositiveID` wording) and the missing/null/wrong-type
  split, which `RequiredPositiveInt` would reword. Migrate it only with those goldens.

`internal/app/` takes no `float64`. `scripts/ci/check-number-casts.sh` (run by
`make check`) fails on any `float64` under `internal/app/`, and it ratchets the
count of `float64` mentions per file under `internal/operation/` against
`scripts/ci/number-float64-baseline.txt`, which may only fall.

### Step 3: Register the tool

```go
func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(ListThingsTool, h.listThings())
}
```

If the domain is new, call its `RegisterTool` from `internal/operation/operation.go`.

### Step 4: Pin it with tests

- `mcptest.SchemaSnapshot(t, ListThingsTool)` pins the name, description and
  input schema in `testdata/`.
- `mcptest.Golden(t, name, output)` pins the rendered output and error text for
  each case, through a stub port (see `internal/operation/pull/golden_test.go`).
- `mcptest.ArgsMatchSchema(t, ListThingsTool, listArgs{})` checks the argument
  struct against the schema.
- `go test ./internal/operation/mydomain -update` writes the golden and snapshot files.
  Review the diff, then run the tests without `-update`.
- Use-case tests live in `internal/app/<domain>/service_test.go`, in an external test
  package over the in-memory fake through the bridge.
- Keep one `httptest` smoke test per domain through the real SDK adapter
  (`forgetest.NewStub`).
- New tests call `t.Parallel()`. Retire an old pre-split test only where a new
  use-case or golden test covers the same behaviour.

Before opening a PR, run `make build`, `go vet ./...`, `go test ./...` and every
`scripts/ci/*.sh` check.

## Key Patterns

### Parameter Handling

Every tool decodes its arguments into a per-tool struct through `pipeline.Handle`
(see "Step 2" above). No domain reads `req.Params.Arguments` directly. The few tools
that need the raw request (an upload's source, where presence matters) read it with
`pipeline.Raw(ctx)` after the credential lookup, and a value they must tell apart from
an absent one is a `decode.Arg` field, not a map lookup.

### Response Formatting

Use helpers from `internal/to/`:

```go
// Success response
return to.TextResult(data)

// Error response
return to.ErrorResult(fmt.Errorf("something went wrong: %v", err))
```

### Shared Parameter Descriptions

Reuse descriptions from `internal/operation/params/` for consistency:

```go
mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner))
mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo))
mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1))
```

## Dependencies

| Package | Purpose |
|---------|---------|
| `codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3` | Forgejo API client |
| `github.com/mark3labs/mcp-go` | MCP protocol implementation |
| `github.com/spf13/cobra` | CLI framework |

## Testing

Run with debug mode to troubleshoot issues:

```bash
FORGEJO_DEBUG=true ./forgejo-mcp --transport stdio --url <url> --token <token>
```

## OpenSpec conventions

Change deltas live in `openspec/changes/<change>/specs/<capability>/spec.md` and
are gated by `openspec validate --all --strict` in CI
(`.tekton/tasks/openspec-validate.yaml`).

**Requirement statements go on one unwrapped line.** `openspec` only inspects the
first physical line of a requirement paragraph when it looks for `SHALL`/`MUST`,
so a hard-wrapped statement is rejected with
`ADDED "<name>" must contain SHALL or MUST` even when the rest of the paragraph
is full of `SHALL`s.

```markdown
### Requirement: Attachment upload source

The `create_issue_attachment` and `create_release_attachment` tools SHALL accept optional string arguments `content` and `file_path` and SHALL require exactly one argument key to be present.
```

`.editorconfig` disables `max_line_length` under `openspec/` so editors do not
re-wrap these paragraphs. Run the gate locally before pushing:

```bash
scripts/ci/openspec-validate.sh
```

It also runs as a pre-commit hook (`openspec-validate`) and skips with a warning
if `openspec` is not installed.

## Reviewing a pull request: confirm CI actually ran

Pipelines-as-Code will not start a PipelineRun for an author who is not listed
in `OWNERS`. When it declines, it leaves exactly one commit status on the head
SHA:

```
op1st Pipelines as Code   pending   Pending approval, waiting for an /ok-to-test
```

There are no red checks — because there are no checks. **"Not red" is not
"green."** A maintainer must comment `/ok-to-test` on the PR before any
pipeline runs.

Do not check this by eye. Run the gate:

```bash
scripts/ci/check-pr-ci-ran.sh <pr-number>
scripts/ci/check-pr-ci-ran.sh --sha <commit-sha>
```

It reads the forge's combined-status API for the PR head and exits non-zero
when CI never ran, is still awaiting `/ok-to-test`, or a pipeline reported a
non-success context. It skips with a warning if `curl` or `jq` is missing. Set
`FORGEJO_URL` for another instance and `FORGEJO_TOKEN` for a private repo.

This is a reviewer-side gate, not a pre-commit hook: no local file change
implies it, so there is nothing for `pre-commit` to key on.

### A permanently-red PR is not necessarily a failing one

Pipelines-as-Code reports under two kinds of context:

```
op1st Pipelines as Code                          its own gating slot
op1st Pipelines as Code / forgejo-mcp-code-scans one actual PipelineRun
```

When PaC fails *before* creating any PipelineRun — a `.tekton` manifest that
will not parse is the usual cause — it writes a failure into the gating slot
and stops. A later successful retest creates the named run contexts, but
nothing owns the gating slot, so that failure is never overwritten. The forge
keeps reporting the commit as `aggregate=failure` even though every pipeline
that exists passed, and **only a new head SHA clears it** — retesting cannot,
however many times you try.

Observed on PR #491 at `16ec840`:

```
failure  op1st Pipelines as Code                                2026-08-13T23:19:10Z
success  op1st Pipelines as Code / forgejo-mcp-code-scans       2026-08-13T23:27:53Z
success  op1st Pipelines as Code / forgejo-mcp-on-pull-request  2026-08-13T23:29:43Z
```

This is upstream PaC behaviour, not something this repo can fix. The guard
therefore decides pass/fail from the named run contexts alone and reports the
gating slot without counting it: a red gating slot alongside green runs prints
a warning and still exits 0, and the warning says whether the failure predates
the newest run (almost certainly stale) or not (treat as live). A gating
failure with *no* run contexts at all still exits 1 — there, the empty gating
slot is the only evidence there is, and it says nothing ran.

The warning is worth reading rather than dismissing. A manifest that failed to
parse has no run context to be red, so green runs are not proof that every
intended pipeline ran — only that the ones which started passed.

## Blocked Features

Some planned features are blocked on upstream API or SDK support:

| Feature | Status | Details |
|---------|--------|---------|
| Wiki support | Blocked | Waiting for forgejo-sdk wiki API |
| Projects/Kanban | Blocked | Requires Gitea 1.26.0 API |

See `docs/plans/` for detailed status:
- `wiki-support.md` - Wiki API implementation plan
- `projects-support.md` - Projects/Kanban implementation plan

## Contributing

1. Fork the repository on Codeberg
2. Create a feature branch
3. Make your changes following the patterns above
4. Test locally with both stdio and SSE modes
5. Submit a pull request

### Code Style

- Follow standard Go conventions
- Use meaningful variable names
- Add tool descriptions that clearly explain what each tool does
- Reuse parameter descriptions from `internal/operation/params/` where applicable
