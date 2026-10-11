# AGENTS.md

Code examples and patterns: [DEVELOPER.md](DEVELOPER.md).

Default branch is `main`. `upstream` (b4mad) is a separate project; how its work merges in is not yet decided.

## Quick Reference

```bash
make build          # Build the binary (outputs ./forgejo-mcp)
make vendor         # Tidy and verify Go module dependencies
```

## Architecture Summary

```
main.go → cmd/cmd.go (CLI parsing) → operation/operation.go (tool registration) → operation/{domain}/*.go (MCP adapters) → app/{domain}/ (use cases) → pkg/forge/ (port)
```

Key directories:

- `operation/` - MCP adapters by domain: tool definitions, argument decoding, rendering
- `app/` - Use cases by domain: MCP-free and SDK-free (`scripts/ci/check-app-boundary.sh`)
- `pkg/forge/` - The Forgejo port (interfaces, no SDK client); `pkg/forge/sdk/` is the one real adapter; `pkg/forge/<domain>adapter/` bridges a use case's port to it
- `pkg/to/` - Response formatting helpers
- `pkg/params/` - Shared parameter descriptions

## File Header

Every new file starts with `SPDX-License-Identifier: GPL-3.0-or-later` as its first line, in the file's own comment syntax, before any package declaration or import (`// …` in Go, `# …` in YAML and shell, `<!-- … -->` in Markdown). The identifier line alone is the header.

## Adding a New Tool

1. Put the logic in a use case in `app/{domain}/` (typed input, repo-owned result). `app/**` must not import mcp-go or the Forgejo SDK (`scripts/ci/check-app-boundary.sh`); reach Forgejo through the port in `pkg/forge/`, bridged by `pkg/forge/<domain>adapter/`
2. Put the MCP adapter in `operation/{domain}/`: a `mcp.NewTool()` definition, an argument struct (`json:"name"`, `required:"true"`) decoded with `decode.Lax`, then a call to the use case. Numbers are `float64`
3. Register in the domain's `RegisterTool(s *server.MCPServer, f forge.Factory)` function
4. If new domain, import and call in `operation/operation.go`
5. **Pin it with tests.** `mcptest.SchemaSnapshot` and `mcptest.Golden` per tool, `mcptest.ArgsMatchSchema` per argument struct (`go test ./operation/{domain} -update` writes the files), and use-case tests in `app/{domain}/`
6. **Go through the port.** The adapter's `run` helper calls `f.For(ctx)` per request and hands the port to the bridge; a new
   endpoint is a port method, its adapter in `pkg/forge/sdk/`, its fake and a
   contract case. Nothing outside `pkg/forge/sdk` builds an SDK client or an
   HTTP request to Forgejo. **Escape API paths** in the adapter's raw-HTTP
   methods (`a.rawJSON`, `a.rawDo`, `a.raw.doAPIRaw`, `a.raw.doMultipart`):
   build the path with `forge.APIPath("repos", owner, repo, …)` — it escapes
   every segment. A hand-rolled `fmt.Sprintf("/repos/%s/…")` lets an owner
   containing `/` or `?` retarget the request at another endpoint;
   `scripts/ci/check-api-path-escaping.sh` fails the build on it. Append query
   strings to the `APIPath` result.
7. **Bound the output.** If response size depends on data (not tool
   semantics), the tool MUST satisfy [docs/design/output-bounding.md](docs/design/output-bounding.md):
   client-controlled bound + resumability + documented parameters. Use the
   checklist there in the PR description.
8. **Add the README row.** Every registered tool needs a row in the README
   "Available Tools" table, and every `forgejo://` resource template a row in
   the "Resources" table. `scripts/ci/check-readme-tool-coverage.sh` fails the
   build on a tool or template with no row: it compares the `…ToolName`
   constants and registered URI templates under `operation/` against the
   tables.

See [DEVELOPER.md](DEVELOPER.md) for complete code examples and patterns.

## Resources

Resource templates expose Forgejo entities as `forgejo://` URIs. **Prefer a resource read over the equivalent tool call**: one `forgejo://` read returns the whole entity (a `pr/{index}` read replaces `get_pull_request_by_index` + `list_issue_comments`).

**Expand the template yourself.** `resources/list` is empty by design (only `resources/templates/list` has them), so a client that only lists resources, such as Claude Code's `ListMcpResourcesTool`, reports none. That is not a broken server: substitute every `{…}` and read the concrete URI, for example `forgejo://repo/burythehammer/forgejo-mcp/pr/533`. `{sha}` must be 40 hex characters; percent-encode `/` as `%2F` and spaces as `%20`.

The template table, expansion gotchas and wiki notes are in [docs/agents/resources.md](docs/agents/resources.md). When adding a template, put it under `operation/<domain>/resources*.go` and use the `operation/resource` package (`ParseXxx`, `Bounded`, `MapForgejoError`).

## Blocked Features

Some features are blocked on upstream API/SDK support. See `docs/plans/` for:

- `wiki-support.md` - historical SDK plan, superseded by the direct REST implementation
- `projects-support.md` - Projects/Kanban API (blocked on Gitea 1.26.0)

## Attribution

End every commit with an `Assisted-by: [Model Name] via [Tool Name]` footer, for example `Assisted-by: GLM 4.6 via Claude Code`.

## Releasing

The release process in this repo is inherited from upstream and unverified for this fork. Do not cut a release unless asked; see [docs/legacy/upstream-release.md](docs/legacy/upstream-release.md).

## Agent skills

### Issue tracker

Issues are tracked in Forgejo (`forgejo.burythehammer.com/burythehammer/forgejo-mcp`) via this repo's own forgejo-mcp server (`forgejo` in `.mcp.json`, tools `mcp__forgejo__*`; needs `FORGEJO_URL`, `FORGEJO_TOKEN` and a built `forgejo-mcp` on `PATH`). `fj` stays as a fallback. See `docs/agents/issue-tracker.md`.

### Triage labels

Default triage labels plus `needs-spec`, priority labels, and `spec`/`subtask`: `docs/agents/triage-labels.md`. Pass label names, never IDs, to `add_issue_labels`, `remove_issue_labels`, `create_issue` and `update_issue`.

### Domain docs

Single-context: `GLOSSARY.md` + `docs/adr/` at the repo root, created lazily. See `docs/agents/domain.md`.
