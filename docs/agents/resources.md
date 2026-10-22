<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# Resources

Reference for the `forgejo://` resource templates. The short rule agents need on every change lives in [AGENTS.md](../../AGENTS.md#resources); everything else is here.

Resource templates expose Forgejo entities as `forgejo://` URIs — instance-portable, additive, coexisting with all existing tools (no tool removed). Clients that support `resources/templates/list` and `resources/read` resolve these URIs directly; others fall back to tools transparently.

When adding a new resource template, place it under `internal/operation/<domain>/resources*.go`. Use the `internal/operation/resource` package for URI parsing (`ParseXxx`), embedded-list bounding (`Bounded`), and error mapping (`MapForgejoError`). Embedded lists MUST use `internal/operation/resource.Bounded` so the truncation sentinel stays consistent across resources.

See `openspec/specs/mcp-resources-core/spec.md` for the full normative spec (added by this slice when the change archives).

## Prefer a resource read over a tool call (agent-facing)

When an entity in the table below covers what you need, **read the resource
instead of calling the equivalent tool**. One `forgejo://` read returns the
whole entity — a `pr/{index}` read gives title, state, author, both head/base
SHAs, `mergeable`, and the bounded comment thread with excerpted bodies, in
place of `get_pull_request_by_index` + `list_issue_comments`. Dual-MIME
resources return the JSON block *and* a rendered markdown block in the same
response. Fall back to tools for anything the table does not cover, for writes,
and when a resource's truncation sentinel names a list tool you need.

**Expand the template yourself — do not expect discovery to find it.** These are
resource *templates*: they are served by `resources/templates/list`, and
`resources/list` is empty by design (there are no static resources). A client
that only calls `resources/list` — Claude Code's `ListMcpResourcesTool` among
them — reports no resources at all. That is not a broken server. Resolution
still works: substitute every `{…}` and read the concrete URI.

```
ReadMcpResourceTool(server: "<your server name>",
                    uri: "forgejo://repo/agentic-forges/forgejo-mcp/pr/533")
```

The table below is therefore the discovery mechanism in such clients. Three
gotchas on expansion, each also noted per row:

- `{sha}` must be the full 40 hex characters; short SHAs are rejected.
- Percent-encode `/` as `%2F` and spaces as `%20` in any segment that can
  contain them (wiki page names, branch-protection rule patterns, label
  filters). Do not double-encode; `+` is not a space.
- `{?state,labels,page,limit}` rows take a real query string, e.g.
  `…/issues?state=open&limit=10`.

## Resource table

| URI template | MIME | What it returns |
| --- | --- | --- |
| `forgejo://owner/{owner}` | application/json | User or org profile |
| `forgejo://repo/{owner}/{repo}` | application/json | Repository overview |
| `forgejo://repo/{owner}/{repo}/commit/{sha}{?page,limit,message_page}` | application/json + markdown | Commit metadata (sha must be 40 hex chars); `files` paged by `page`/`limit` (at most 30 per page), `files_next` names the next page; a message over 16 KiB is read in parts by `message_page`, `message_next` names the next part |
| `forgejo://repo/{owner}/{repo}/commit/{sha}/status` | application/json | Combined CI status |
| `forgejo://repo/{owner}/{repo}/issue/{index}` | application/json + markdown | Issue with bounded comments (cap 30) |
| `forgejo://repo/{owner}/{repo}/issues{?state,labels,page,limit}` | application/json | Bounded issue rows, no bodies (state default open; cap 30, sentinel `list_repo_issues`); in `labels`, encode a literal `/` as `%2F` and spaces as `%20` without double-encoding (`+` does not work), and note that names are AND-ed and an unrecognised one is silently dropped — if none match you get the unfiltered set, not an empty one |
| `forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}` | application/json + markdown | Single comment |
| `forgejo://repo/{owner}/{repo}/{kind}/{index}/comments{?page,limit}` | application/json | Bounded comment thread, full bodies (cap 30, sentinel `list_issue_comments`) |
| `forgejo://repo/{owner}/{repo}/pr/{index}` | application/json + markdown | PR with bounded comments + reviews (cap 30) |
| `forgejo://repo/{owner}/{repo}/branch_protections{?page,limit}` | application/json | Bounded list of branch protection rules (page/limit client-controlled, cap 30, sentinel `list_branch_protections`) |
| `forgejo://repo/{owner}/{repo}/branch_protection/{rule}` | application/json | Single branch protection rule; rule names are branch patterns, so encode a literal `/` as `%2F` and spaces as `%20` without double-encoding (`release%2Fv1`) — a raw `/` does not resolve |
| `forgejo://repo/{owner}/{repo}/label/{id}` | application/json | Single repository label |
| `forgejo://repo/{owner}/{repo}/labels{?page,limit}` | application/json | Bounded repo label list (cap 30, sentinel `list_repo_labels`) |
| `forgejo://org/{org}/labels{?page,limit}` | application/json | Bounded org label list (cap 30, sentinel `list_org_labels`) |
| `forgejo://repo/{owner}/{repo}/hooks{?page,limit}` | application/json | Bounded list of repository webhooks (page/limit client-controlled, cap 30, sentinel `list_repo_hooks`; secret never returned) |
| `forgejo://repo/{owner}/{repo}/hook/{id}` | application/json | Single repository webhook (secret never returned) |
| `forgejo://repo/{owner}/{repo}/wiki/{pageName}` | application/json + markdown | Wiki page with bounded revisions; use the normalized `page_name`, encoding literal `/` as `%2F` and spaces as `%20` without double-encoding |

Wiki tools live in `internal/operation/wiki/`. `list_*` enumerates entities; `get_*` fetches one
entity by its server-normalized name. Always reuse the `page_name` returned by create/list.
Slash-separated titles are only a flat subpage naming convention: Forgejo does not create a
parent page or store a parent-child relationship. Create any desired parent page separately.

