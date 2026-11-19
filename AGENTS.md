# AGENTS.md

Default branch is `main`. `upstream` (b4mad) is a separate project; how its work merges in is not yet decided.

Run `make check` before reporting done; `make help` lists the other targets.

## Architecture

```
main.go → cmd/cmd.go → internal/operation/operation.go → internal/operation/{domain}/*.go (MCP adapters) → internal/app/{domain}/ (use cases) → internal/forge/ (port)
```

`internal/app/` is MCP-free and SDK-free (depguard in `.golangci.yml`). `internal/forge/` is the Forgejo port; `internal/forge/sdk/` is the real adapter, and nothing outside it builds a Forgejo client or request. Directory map, layers and code examples: [DEVELOPER.md](DEVELOPER.md).

## File header

Every new file starts with `SPDX-License-Identifier: GPL-3.0-or-later` as its first line, in the file's own comment syntax (`// …` in Go, `# …` in YAML and shell, `<!-- … -->` in Markdown), before any package declaration or import.

## Adding or changing a tool

Before adding or changing a tool, read [docs/agents/adding-a-tool.md](docs/agents/adding-a-tool.md): use case, MCP adapter, port, tests, output bound, reference-table row in `docs/reference/` (tools.md, resources.md).

## Numeric arguments

A number argument is read through `internal/numarg` in the MCP adapter, and the use case gets a validated `int` or `int64`. `make check` runs `scripts/ci/check-number-casts.sh`, which enforces two things only: no `float64` anywhere under `internal/app/`, and the count of `float64` mentions per file under `internal/operation/` may not grow past `scripts/ci/number-float64-baseline.txt` (counts may only fall). A bare `int(...)` or `int64(...)` of a `float64` argument is not caught by the script; it is a review rule.

Known leftovers the script does not catch, each pinned by a golden, so they change only with the golden: `int64(*a.ID)` in `internal/operation/user/notification.go` (golden `thread_zero_id`); `to.Float64` in `internal/operation/release/handlers.go` and `internal/operation/issue/dependency.go`; `numarg.Truncate` in `internal/operation/branchprotection`, `internal/operation/org`, `internal/operation/wiki` and `internal/operation/repo/file.go`.

The one exception (a tool whose error text depends on how the number was given is typed `any`) and the helper list are in [DEVELOPER.md](DEVELOPER.md#numeric-arguments).

## Resources

Prefer a `forgejo://` resource read over the equivalent tool call: one read returns the whole entity (`pr/{index}` replaces `get_pull_request_by_index` + `list_issue_comments`).

Expand the template yourself. `resources/list` is empty by design (only `resources/templates/list` has them), so a client that only lists resources, such as Claude Code's `ListMcpResourcesTool`, reports none; that is not a broken server. Substitute every `{…}` and read the concrete URI, for example `forgejo://repo/burythehammer/forgejo-mcp/pr/533`. `{sha}` is 40 hex characters; percent-encode `/` as `%2F` and spaces as `%20`.

Template table, expansion gotchas and how to add a template: [docs/agents/resources.md](docs/agents/resources.md).

## Pointers

- **Blocked features** (wiki, projects): `docs/plans/`.
- **Releasing:** do not cut a release unless asked. Process: [docs/agents/releasing.md](docs/agents/releasing.md) (a tag push runs `.forgejo/workflows/release.yaml`).
- **Subagents** (implementing in a worktree, merging their branches, briefing either): [docs/agents/subagent-work.md](docs/agents/subagent-work.md).
- **Issue tracker** (Forgejo, via this repo's own `forgejo` MCP server, `fj` as fallback): [docs/agents/issue-tracker.md](docs/agents/issue-tracker.md).
- **Triage labels** (pass names, never IDs, to `add_issue_labels`, `remove_issue_labels`, `create_issue` and `update_issue`): [docs/agents/triage-labels.md](docs/agents/triage-labels.md).
- **Domain docs** (`GLOSSARY.md` + `docs/adr/`, created lazily): [docs/agents/domain.md](docs/agents/domain.md).
- **Repo wiki** (architecture, delivery workflow, `log`): the Forgejo wiki of this repo, read with `get_wiki_page` / `list_wiki_pages` / `search_wiki` and logged there. Outline is retired for this repo (#359): do not run `/repo-wiki` against it. A hyphenated page is stored as `<title>.-`; use the `page_name` the tools return.
