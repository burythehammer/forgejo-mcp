# AGENTS.md

This file provides guidance to AI coding assistants (Claude Code, Cursor, etc.) when working with this repository.

For detailed developer documentation, see [DEVELOPER.md](DEVELOPER.md).

Default branch is `main`. `upstream` (b4mad) is a separate project; how its work merges in is not yet decided.

## Quick Reference

```bash
make build          # Build the binary (outputs ./forgejo-mcp)
make vendor         # Tidy and verify Go module dependencies
```

## Architecture Summary

```
main.go → cmd/cmd.go (CLI parsing) → operation/operation.go (tool registration) → operation/{domain}/*.go (tool handlers)
```

Key directories:

- `operation/` - MCP tool definitions and handlers by domain
- `pkg/forgejo/` - Singleton Forgejo SDK client wrapper
- `pkg/to/` - Response formatting helpers
- `pkg/params/` - Shared parameter descriptions

## File Header

Every new source file MUST begin with an SPDX license header as the very first lines, before any package declaration or imports:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
```

For non-Go files (YAML, Markdown, shell, etc.), use the appropriate comment syntax:

```yaml
# SPDX-License-Identifier: GPL-3.0-or-later
```

```bash
# SPDX-License-Identifier: GPL-3.0-or-later
```

Do not add a copyright line — the SPDX identifier line alone is sufficient.

## Adding a New Tool

1. Create or modify a file in `operation/{domain}/`
2. Define tool with `mcp.NewTool()` and implement handler function
3. Register in the domain's `RegisterTool(s *server.MCPServer)` function
4. If new domain, import and call in `operation/operation.go`
5. **Escape API paths.** If the tool uses the raw-HTTP helpers
   (`forgejo.DoJSON`, `DoJSONList`, `DoAPIRaw`, `DoMultipart`) rather than the
   SDK, build the path with `forgejo.APIPath("repos", owner, repo, …)` — it
   escapes every segment. A hand-rolled `fmt.Sprintf("/repos/%s/…")` lets an
   owner containing `/` or `?` retarget the request at another endpoint;
   `scripts/ci/check-api-path-escaping.sh` fails the build on it. Append query
   strings to the `APIPath` result.
6. **Bound the output.** If response size depends on data (not tool
   semantics), the tool MUST satisfy [docs/design/output-bounding.md](docs/design/output-bounding.md):
   client-controlled bound + resumability + documented parameters. Use the
   checklist there in the PR description.
7. **Add the README row.** Every registered tool needs a row in the README
   "Available Tools" table, and every `forgejo://` resource template a row in
   the "Resources" table. `scripts/ci/check-readme-tool-coverage.sh` fails the
   build on a tool or template with no row — it compares the `…ToolName`
   constants and registered URI templates under `operation/` against the
   tables. This was unenforced until #601, by which point 22 of 155 tools and
   4 of 17 templates had drifted out, `delete_org` among them.

See [DEVELOPER.md](DEVELOPER.md) for complete code examples and patterns.

## Resources

Resource templates expose Forgejo entities as `forgejo://` URIs. **Prefer a resource read over the equivalent tool call**: one `forgejo://` read returns the whole entity (a `pr/{index}` read replaces `get_pull_request_by_index` + `list_issue_comments`).

**Expand the template yourself.** `resources/list` is empty by design (only `resources/templates/list` has them), so a client that only lists resources, such as Claude Code's `ListMcpResourcesTool`, reports none. That is not a broken server: substitute every `{…}` and read the concrete URI, for example `forgejo://repo/burythehammer/forgejo-mcp/pr/533`. `{sha}` must be 40 hex characters; percent-encode `/` as `%2F` and spaces as `%20`.

The template table, expansion gotchas and wiki notes are in [docs/agents/resources.md](docs/agents/resources.md). When adding a template, put it under `operation/<domain>/resources*.go` and use the `operation/resource` package (`ParseXxx`, `Bounded`, `MapForgejoError`).

## Blocked Features

Some features are blocked on upstream API/SDK support. See `docs/plans/` for:

- `wiki-support.md` - historical SDK plan, superseded by the direct REST implementation
- `projects-support.md` - Projects/Kanban API (blocked on Gitea 1.26.0)

## Labels

Use the triage vocabulary in [docs/agents/triage-labels.md](docs/agents/triage-labels.md). Pass label names, never IDs, to `add_issue_labels`, `remove_issue_labels`, `create_issue` and `update_issue`.

## Attribution Requirements

AI agents must disclose what tool and model they are using in the "Assisted-by" commit footer:

```text
Assisted-by: [Model Name] via [Tool Name]
```

Example:

```text
Assisted-by: GLM 4.6 via Claude Code
```

<!-- >>> enable-semantic-release v0.0.0-dev sha256:66510a787d1a (managed block — do not edit) >>> -->
## Releasing

The release process in this repo is inherited from upstream and unverified for this fork. Do not cut a release unless asked; see [docs/legacy/upstream-release.md](docs/legacy/upstream-release.md).

## Agent skills

### Issue tracker

Issues are tracked in Forgejo (`forgejo.burythehammer.com/burythehammer/forgejo-mcp`) via this repo's own forgejo-mcp server (`forgejo` in `.mcp.json`, tools `mcp__forgejo__*`; needs `FORGEJO_URL`, `FORGEJO_TOKEN` and a built `forgejo-mcp` on `PATH`). `fj` stays as a fallback. See `docs/agents/issue-tracker.md`.

### Triage labels

Default triage labels (plus `needs-spec`, priority labels, and `spec`/`subtask`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `GLOSSARY.md` + `docs/adr/` at the repo root, created lazily. See `docs/agents/domain.md`.
