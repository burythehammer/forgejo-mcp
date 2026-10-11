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

Environment variables: `FORGEJO_URL`, `FORGEJO_ACCESS_TOKEN`, `FORGEJO_DEBUG`, `FORGEJO_USER_AGENT`

CLI options: `--url`, `--token`, `--transport`, `--sse-port`, `--http-port`, `--user-agent`,
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
main.go → cmd/cmd.go (CLI parsing) → operation/operation.go (tool registration) → operation/{domain}/*.go (tool handlers)
```

### Directory Structure

| Directory | Purpose |
|-----------|---------|
| `cmd/` | CLI entry point and command parsing |
| `operation/` | MCP tool definitions and handlers, organized by domain |
| `operation/issue/` | Issue-related tools |
| `operation/pull/` | Pull request tools |
| `operation/release/` | Release and release-attachment tools |
| `operation/repo/` | Repository and branch tools |
| `operation/search/` | Search tools (users, repos, teams) |
| `operation/user/` | User info tools |
| `operation/version/` | Server version tool |
| `pkg/forge/` | The Forgejo port: per-domain interfaces and the `Factory` (no SDK client import) |
| `pkg/forge/sdk/` | The one real adapter: `Factory` (credential policy, startup checks) and `Adapter` over the SDK and one raw-HTTP client; the only package that touches the SDK client or the raw transport |
| `pkg/forge/fake/` | Stateful in-memory `forge.Forgejo` for handler tests |
| `pkg/forge/forgetest/` | Contract suite run against the fake and the adapter, and the shared stub server |
| `app/<domain>/` | Use cases (ADR 0003): typed inputs, a domain port, repo-owned results. No mcp-go or SDK import (`scripts/ci/check-app-boundary.sh`) |
| `app/model/` | Repo-owned domain types that use cases return (wire-shaped copies of the SDK types) |
| `app/internal/split/` | The comma-separated list parser the use cases share |
| `operation/decode/`, `operation/mcptest/` | Argument decoding into per-tool structs; golden, schema-snapshot and args-vs-schema test helpers |
| `pkg/forge/<domain>adapter/` | Bridge from a use case's port to `forge.<Domain>`: the one place SDK and repo-owned types are translated |
| `pkg/to/` | Response formatting helpers (`TextResult`, `ErrorResult`) |
| `pkg/params/` | Shared parameter descriptions for tool definitions |
| `pkg/config/` | Typed Config, `Load` and `Validate` |
| `pkg/log/` | Structured logging utilities |

## The Forgejo port and migrating a domain

Every domain reaches Forgejo through a port. The pull domain
(`operation/pull/`, `pkg/forge/pull.go`) is the reference; copy its shape. A
domain has:

1. **Port** (`pkg/forge/<domain>.go`). An interface named for what the tools
   need, SDK types as plain data, `ctx` first. Server failures are
   `*forge.StatusError` (use `forge.IsNotFound`); a decline that Forgejo reports
   as a status rather than an error is a result field (see `forge.MergeResult`).
   Embed the interface in `forge.Forgejo` (`pkg/forge/forge.go`).
2. **Adapter** (`pkg/forge/sdk/<domain>.go`). Methods on `*Adapter`, a
   one-line `var _ forge.<Domain> = (*Adapter)(nil)`, and every SDK call wrapped
   in `statusErr(resp, err)`. Raw-HTTP workarounds go here too, with
   `forge.APIPath`, through the adapter's single raw path (`a.rawJSON` /
   `a.rawDo` / `a.raw.*`), which targets the adapter's own instance with the
   request's own credential.
3. **Fake** (`pkg/forge/fake/<domain>.go`). Methods on `*fake.Forgejo` with its
   own state struct (and mutex) held in one new field on `fake.Forgejo`. It
   must behave like Forgejo: round-trips, 404 for unknown things, pagination via
   `page`. Add seeding helpers for what the port cannot write (the pull fake's
   `AddRepo`, `SetPullRequestDiff`).
4. **Contract cases** (`pkg/forge/forgetest/<domain>.go`, `RunXxx`). Behaviour
   through the port only, never HTTP traffic. Add the seeding methods to
   `Seeder`, then give the stub server the REST routes
   (`forgetest/stub_<domain>.go`, a thin translation onto `Stub.Fake`) and call
   `RunXxx` from `fake/contract_test.go` and `sdk/contract_test.go`. The stub
   answers the SDK startup probe in one place, `forgetest.WriteVersion`.
5. **Handlers take the factory.** `RegisterTool(s, f forge.Factory)` /
   `RegisterResource(s, f)`, a `handlers{forge: f}` struct whose methods call
   `h.forge.For(ctx)` per request (so each request gets its own credential), and
   thread `f` through `operation/operation.go` and `cmd/cli.go`.
6. **Handler tests on the fake** (`operation/<domain>/*_test.go`). Build a fresh
   `fake.New()` per test, call the handler, assert on the tool result, and
   `t.Parallel()`. No `flag.*`, no package-level Forgejo state, no `httptest`
   server.

The composition root (`operation.Run`, `cmd/cli.go`) builds one
`sdk.NewFactory(cfg.Forgejo, version, ...)` from the typed Config and passes it
into every `RegisterTool`/`Register...Resource`. The credential policy (does a
request with no token of its own fall back to the operator's?) is a Factory
option, chosen in `operation.newFactory`; `Factory.VerifyConnection(ctx)` is the
startup gate `Run` calls before serving.

## Splitting a domain into MCP adapters and use cases

ADR 0003 splits every tool into an MCP adapter (`operation/<domain>`) and an
MCP-free use case (`app/<domain>`). `version` is the smallest example, `pull`
(all 16 pull tools) the reference for a domain with a port, and `issue` (the 16
issue tools with their 8 label and 4 dependency tools) the one with the most
moving parts: label-name resolution and the `update_issue` ordering both live
in `app/issue`. Per domain:

1. **Pin first.** Before moving anything, write the characterisation tests
   against the existing code: `mcptest.SchemaSnapshot` for each tool and
   `mcptest.Golden` for each case's output and error text (see
   `operation/pull/golden_test.go`: a stub `forge.Forgejo` returning fixed,
   fully populated SDK data and recording the port calls, rendered together).
   Create the files with `go test ./operation/<domain> -update`. They must pass
   unchanged after the move: tool names, schemas, output bytes and error text
   are frozen.
2. **Use case** (`app/<domain>`): a `Port` interface in repo-owned types
   (`port.go`), a `Service` with one method per tool taking a typed input
   (`service.go`; defaults and parsing live here), and the few typed error
   kinds callers can act on (`errors.go`; text must stay what the tool said,
   Forgejo failures are wrapped `fmt.Errorf("<op> err: %w", err)`). Results come
   from `app/model`; add the wire-shaped types a domain needs there.
3. **Port bridge** (`pkg/forge/<domain>adapter`): `New(forge.<Domain>) app.Port`
   translates inputs to SDK options and converts SDK results to `app/model`
   values through JSON. `shape_test.go` there fails if an SDK upgrade changes a
   shape the model copies. The shared `forge.<Domain>` stays SDK-typed (the
   resources and contract suite use it) until #8 retires it.
4. **MCP adapter** (`operation/<domain>/handlers.go`): one argument struct per
   tool (`json:"name"`, `required:"true"` so `ArgsMatchSchema` checks them against the
   schema). Decode with `decode.Lax`, not `decode.Args`, while the wire contract
   is frozen: the old handlers never rejected a missing or mistyped argument,
   and truncated numbers, so declare numeric fields `float64` and convert with
   `int64(...)` in the handler. Both accept string-encoded numbers. Then each
   handler is a few lines through the generic `run` helper: decode, build
   `apppull.New(pulladapter.New(port))`, call the use case, render with
   `to.TextResult` / plain text. Use-case errors become MCP errors in one
   function (`fail`); invalid or missing arguments are a tool error result
   (`mcp.NewToolResultError`). Keep the exported `XxxTool` definitions and
   `XxxToolName` constants as they were.

   Three refinements came with `issue` (`operation/issue/`, 28 tools):
   - **Defaults that test for zero live in the adapter, before truncation.**
     The old handlers defaulted `page == 0` on the float, so `0.5` was passed
     on as `0`. `pageOr` (in `handlers.go`) keeps that; the use case does not
     default a page or limit it is handed.
   - **Pointer fields where presence matters.** An argument whose "given but
     empty" differs from "not given" (`labels`, `assignees`, `set_labels`,
     `ref`, a label's `name`, `state` on `list_repo_issues`) is a `*string`
     struct field, nil when absent or wrongly typed. `decode.Lax` zeroes a
     field whose value fails to decode, so a wrong type reads as absent.
     `mcptest.ArgsMatchSchema` dereferences pointer fields.
   - **Numbers as a string.** `decode.Lax` reads a numeric string (`"7"`) as
     the number, for a plain or a pointer field. A handler that asserted
     `float64` and ignored a string tags the field `exact:"true"`, so the
     string reads as absent (the wiki paging and line arguments).
   - **Raw checks for a few numbers.** The dependency tools' index arguments
     were always type-checked (a missing one was an error), which a zero value
     cannot express. Those are read with `to.Float64` from the raw arguments in
     the adapter (`numberArg`), with the old error text.
   - **Validation before the credential.** The old handlers checked their
     arguments before calling `h.forge.For`, so a bad request got its input
     error even when no credential resolved. A use case that validates (parses
     a time, checks a duration) takes a `Source` (`func(ctx) (Port, error)`)
     instead of a port, and looks the port up only after its checks. The
     adapter passes `h.source()`. `app/tracking` and `app/user` do this.
   - **Optional numbers.** A pointer to a number reads a JSON number only; a
     numeric string is read only when the field is tagged `numeric:"string"`
     (for the handlers that used `to.Float64`). Plain number fields keep reading
     numeric strings. Pick per field to match the old handler.
5. **Tests.** Use cases: `app/<domain>/service_test.go`, an external test
   package over the in-memory fake through the bridge (no SDK import allowed
   under `app/`, so seed through the fake's SDK-free helpers or the use case
   itself). Adapter: the goldens and snapshots, `mcptest.ArgsMatchSchema` per
   argument struct, a thin table test that tools reach their use case, the
   failure paths, and one `httptest` smoke test through the real SDK adapter
   (`forgetest.NewStub`). New tests call `t.Parallel()`. Retire old handler
   tests only where a use-case or golden test covers the behaviour.
6. **Docs.** README tool tables stay complete
   (`scripts/ci/check-readme-tool-coverage.sh`); update any spec that names a
   moved symbol.

## Adding a New Tool

### Step 1: Create or Modify a Domain File

Tools are organized by domain in `operation/{domain}/`. Create a new file or add to an existing one.

### Step 2: Define the Tool

```go
package mydomain

import (
    "context"
    "fmt"

    "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
    "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/params"
    "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

// Tool definition
var MyTool = mcp.NewTool(
    "my_tool_name",
    mcp.WithDescription("What this tool does"),
    mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
    mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
    mcp.WithNumber("limit", mcp.Description("Page size"), mcp.DefaultNumber(20)),
)

// handlers hold the factory so each call uses its own request's credential.
type handlers struct{ forge forge.Factory }

// Handler function
func (h handlers) MyToolFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    // Extract parameters (numbers come as float64)
    owner, _ := req.Params.Arguments["owner"].(string)
    repo, _ := req.Params.Arguments["repo"].(string)
    limit, _ := req.Params.Arguments["limit"].(float64)

    // Call Forgejo through the port
    f, err := h.forge.For(ctx)
    if err != nil {
        return to.ErrorResult(err)
    }
    result, err := f.SomeMethod(ctx, owner, repo, int(limit))
    if err != nil {
        return to.ErrorResult(fmt.Errorf("operation failed: %v", err))
    }

    // Return formatted result
    return to.TextResult(result)
}
```

`SomeMethod` is a method on the domain's port in `pkg/forge/`, implemented by
the adapter in `pkg/forge/sdk/` and by the fake in `pkg/forge/fake/` (see
"The Forgejo port and migrating a domain" above).

### Step 3: Register the Tool

Add registration in the domain's file:

```go
func RegisterTool(s *server.MCPServer, f forge.Factory) {
    h := handlers{forge: f}
    s.AddTool(MyTool, h.MyToolFn)
}
```

### Step 4: Wire Up New Domains

If you created a new domain, import and register it in `operation/operation.go`:

```go
import "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mydomain"

func RegisterTool(s *server.MCPServer, version string, f forge.Factory) {
    // ... existing registrations
    mydomain.RegisterTool(s, f)
}
```

## Key Patterns

### Parameter Handling

Migrated domains (`version`, `pull`) decode arguments into a per-tool struct
with `decode.Args`; see "Splitting a domain into MCP adapters and use cases".
Not-yet-migrated tools still read the argument map directly:

- String parameters: `value, _ := req.Params.Arguments["param"].(string)`
- Number parameters: `value, _ := req.Params.Arguments["num"].(float64)` (always float64)
- Optional with defaults: Check if value exists before using

### Response Formatting

Use helpers from `pkg/to/`:

```go
// Success response
return to.TextResult(data)

// Error response
return to.ErrorResult(fmt.Errorf("something went wrong: %v", err))
```

### Shared Parameter Descriptions

Reuse descriptions from `pkg/params/` for consistency:

```go
mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner))
mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo))
mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1))
```

## Dependencies

| Package | Purpose |
|---------|---------|
| `codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2` | Forgejo API client |
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
- Reuse parameter descriptions from `pkg/params/` where applicable
