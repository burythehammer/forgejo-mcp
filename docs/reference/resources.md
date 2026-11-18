<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# Resource templates


MCP resource templates expose Forgejo entities as URI-addressable resources using the `forgejo://` scheme. The URI scheme is instance-portable — the same URI form works against any Forgejo instance — and does not collide with Forgejo web links. Clients that support `resources/templates/list` and `resources/read` (Claude Code, Claude Desktop, Codex, Cursor) can resolve these URIs directly. Clients without resource-template support continue to use the tools above — no functionality is removed.

Resources do NOT replace any MCP tool — every existing list/get tool stays available; resources are an additive, URI-addressable read surface intended for auto-resolution from LLM context and for content-addressable caching of immutable entities like commits.

Resources that embed a list (issue, pr) cap the embedded array at 30 items. When truncated, the JSON payload includes a sentinel naming the corresponding `list_*` tool the caller should invoke for the full list.

**When to use resources vs tools:** prefer a resource when you have a specific sha or index in hand; prefer a tool when listing or searching.

| URI Template | Entity | Notes |
|---|---|---|
| `forgejo://owner/{owner}` | application/json | User or org profile addressed by login; resolves user first, falls back to org. |
| `forgejo://repo/{owner}/{repo}` | application/json | Repository overview: identity + counts, no embedded lists. |
| `forgejo://repo/{owner}/{repo}/commit/{sha}{?page,limit,message_page}` | Commit metadata | Immutable per sha. Returns JSON + markdown sidecar. sha must be 40 hex chars. `files` is paged: `page`/`limit` select a page of at most 30 files; when files are cut the response carries `files_total`, `files_page`, `files_limit`, `files_truncated` and `files_next` (the next page's URI). A message over 16 KiB is read in parts of at most 16 KiB: `message_page` (default 1) selects a part; a cut message is marked `message_truncated` with `message_page` and `message_pages`, the sidecar names the next part, and `message_next` gives its URI when one follows. A message that fits in one part is returned whole. |
| `forgejo://repo/{owner}/{repo}/commit/{sha}/status` | application/json | Combined CI status for a sha: aggregate state + bounded per-context statuses (cap 30, sentinel names list tool `get_commit_statuses`). |
| `forgejo://repo/{owner}/{repo}/issue/{index}` | application/json (+ text/markdown sidecar) | Issue metadata + rendered body + bounded recent comments (cap 30, sentinel names `list_issue_comments`). |
| `forgejo://repo/{owner}/{repo}/issues{?state,labels,page,limit}` | application/json | Bounded list of issues as rows — index, title, state, author, labels, assignees, milestone, comment count, timestamps, due date — and **no bodies**. `state` ∈ {open, closed, all} (default `open`); `labels` comma-separated; cap 30, sentinel names `list_repo_issues`. Read the single-issue resource for a body. |
| `forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}` | application/json (+ text/markdown sidecar) | Single comment by id; kind ∈ {issue, pr}. |
| `forgejo://repo/{owner}/{repo}/{kind}/{index}/comments{?page,limit}` | application/json | Bounded comment thread with **full bodies** (the single-issue resource excerpts them at 200 chars); kind ∈ {issue, pr}; cap 30, sentinel names `list_issue_comments`. |
| `forgejo://repo/{owner}/{repo}/pr/{index}` | application/json (+ text/markdown sidecar) | PR metadata, head/base refs, mergeability, bounded recent comments (cap 30, sentinel `list_issue_comments`) and reviews (cap 30, sentinel `list_pull_reviews`). |
| `forgejo://repo/{owner}/{repo}/branch_protections{?page,limit}` | application/json | Bounded list of branch protection rules (page/limit client-controlled, cap 30, sentinel names `list_branch_protections`). |
| `forgejo://repo/{owner}/{repo}/branch_protection/{rule}` | application/json | Single branch protection rule. Rule names are branch patterns, so encode a literal `/` as `%2F` and spaces as `%20` (`release%2Fv1`); a raw `/` does not resolve. |
| `forgejo://repo/{owner}/{repo}/hooks{?page,limit}` | application/json | Bounded list of repository webhooks (page/limit client-controlled, cap 30, sentinel names `list_repo_hooks`). The secret is never returned. |
| `forgejo://repo/{owner}/{repo}/hook/{id}` | application/json | Single repository webhook by id. The secret is never returned. |
| `forgejo://repo/{owner}/{repo}/label/{id}` | application/json | Single repository label by numeric id, with its `exclusive` flag. |
| `forgejo://repo/{owner}/{repo}/labels{?page,limit}` | application/json | Bounded list of repository labels, each with its `exclusive` flag (cap 30, sentinel names `list_repo_labels`). |
| `forgejo://org/{org}/labels{?page,limit}` | application/json | Bounded list of organization-level labels, each with its `exclusive` flag (cap 30, sentinel names `list_org_labels`). |
| `forgejo://repo/{owner}/{repo}/wiki/{pageName}` | application/json (+ text/markdown sidecar) | Wiki page with bounded revisions and Markdown capped at 1 MiB. Use the returned normalized `page_name`; encode a literal `/` as `%2F` and spaces as `%20` in the URI (do not double-encode an already normalized name). |

Slash-separated titles such as `Guides/Setup` are useful as a subpage naming convention,
but Forgejo stores the pages in a flat list: it neither creates `Guides` automatically nor
records a parent-child relationship. Create the parent separately when readers need it, and
always address subsequent calls with the normalized `page_name` returned by create or list.
The wiki REST behavior documented here was live-tested against Forgejo
`15.0.4+gitea-1.22.0`; the complete MCP demo was replayed successfully against
`16.0.0+gitea-1.22.0`. This is the tested range, not a guarantee for every intermediate
or differently proxied deployment.

## Client compatibility

| Client | `resources/templates/list` | `resources/read` |
|---|---|---|
| Claude Code | supported | supported |
| Claude Desktop | supported | supported |
| Codex | supported | supported |
| Cursor (current) | supported | supported |
| Older / minimal clients | tools only | tools only |
