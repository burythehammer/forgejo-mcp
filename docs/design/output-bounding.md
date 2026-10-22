# Output Bounding for MCP Tools

Architectural invariant for `forgejo-mcp` tool design. Any new tool that returns data
proportional to repository or upstream state MUST satisfy the rules below before
landing.

The same invariant applies to data-proportional MCP resource content blocks: cap the block
explicitly and include a marker naming a range- or page-bounded tool that retrieves the remainder.

This paragraph used to open "Because `resources/read` has no caller-controlled range parameter".
That is no longer true and should not be cited as a reason to omit one. `resources/read` takes a
URI, and an RFC 6570 query expansion on the registered template (`…/labels{?page,limit}`) gives
the caller a range parameter through it. A collection resource is therefore held to the same
client-controlled bound as a tool — see the "Collection resource" requirement in
`mcp-resources-core`. The cap remains, as a ceiling rather than as the only bound.

## Why

MCP tool outputs flow into an LLM context window. A single unbounded response
(diff, file content, commit list, log stream) can blow the window or silently
truncate at the transport envelope. The caller then sees partial data with no
signal and no way to fetch the remainder. Issue
[#124](https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/124) surfaced this on
`get_pull_request_diff`, `get_file_content`, `list_pull_request_files`,
`list_pull_reviews`. This document generalizes the fix.

## The Rule

**Every tool output must be bounded by the caller, not the server. If the
output size depends on data rather than tool semantics, the tool MUST expose at
least one client-controlled bound AND a way to fetch the remainder.**

A tool whose output is bounded by its own semantics (e.g. `get_my_user_info`
returns one fixed-shape user object) is exempt. Everything else is in scope.

## Sub-rules

### 1. No silent truncation

A server-side envelope cap (e.g. 16 kB) without a caller-visible knob is a
trap: the caller receives partial data with no signal. Either:

- Expose the cap as a parameter the caller can raise / lower, or
- Replace the cap with proper paging / range params (sub-rule 2), or
- Return an explicit truncation marker (sub-rule 3) when the cap fires.

Never silently drop bytes.

### 2. Bound by domain shape, not bytes

Pick the natural unit for the data type. Byte ranges are a last-resort
fallback because they cut mid-token.

| Data type                 | Preferred bound                         | Parameter shape                              |
|---------------------------|-----------------------------------------|----------------------------------------------|
| Code / text file          | Line range                              | `start_line`, `end_line`                     |
| Diff (multi-file)         | Per-file slice (then optional paging)   | `file_path`; index via `list_*_files`        |
| List of entities          | Page + limit                            | `page`, `limit`                              |
| Log stream                | Tail / head + line or byte cap          | `tail_bytes` (or `tail_lines`) + marker      |
| Single binary blob        | Byte range fallback                     | `offset`, `max_bytes`                        |

Reuse parameter names across tools — agents learn one vocabulary, not many.

### 3. Always resumable

When the caller hits the bound, the response must carry a continuation signal
so a follow-up call can retrieve the rest. Acceptable shapes:

- **Paging**: response includes `page`, `total_count`, or `has_next`.
- **Range**: response includes the range actually returned (e.g. lines 1–500
  of 2300) so caller can issue the next slice.
- **Truncation marker**: a sentinel like `[truncated, N more bytes]` for log
  / byte-range tails when paging is unsuitable.
- **Index tool**: a sibling list tool (e.g. `list_pull_request_files`) so the
  caller can enumerate slices before requesting any.

"Got 4 KB of N" beats "got 4 KB."

**`total_count`:** Forgejo sets an `X-Total-Count` response header on paginated
list/search API endpoints, carrying the total number of matching rows across
every page (distinct from `count`, which is the row count on the current page
alone). Tools whose envelope already carries `page`/`limit`/`has_next` SHOULD
also surface this as a `total_count` field, parsed with the shared
the adapter's X-Total-Count helper (`internal/forge/sdk/pagination.go`) rather than
a bespoke per-tool copy; the port hands the tool a `TotalCount *int`. When the header is absent or unparsable, OMIT the key — never
emit `total_count: 0`, which reads as "confirmed zero rows" rather than
"unknown." Today this covers `search_issues`, `list_repo_hooks`, `get_wiki_revisions`, and `get_commit_statuses`.
`list_wiki_pages` does not: its `X-Total-Count` counts raw wiki tree entries rather than
pages, so the total is not exact and the key is omitted.

A pagination envelope is not on its own a reason to add the field: the
endpoint has to actually send the header. Forgejo's handlers call
`SetTotalCountHeader` per endpoint, and several paginated ones do not —
`/repos/{owner}/{repo}/branch_protections`, `/issues/{index}/dependencies`
and `/issues/{index}/blocks` return no `X-Total-Count` at all. Their tools
therefore do NOT carry `total_count`: a key that is structurally always
absent is a promise the tool cannot keep, and a mock that injects the header
proves only the plumbing, not the availability. Check the upstream handler
(or a live response) before adding the field to a new tool.

Envelope `total_count` always means the same thing: the grand total the server
reports for the whole query, not the size of the payload in hand. It usually
arrives in the `X-Total-Count` header and is then a `*int` omitted when the
server does not report it; where the endpoint puts the same total in the
response body instead (`get_wiki_revisions`), it is a plain always-present
`int`. The per-page row count is `count`, and it is a separate field.

Some pre-existing payloads use the name `total_count` for a LOCAL count of the
rows they carry — `internal/operation/branchprotection/resources_branchprotection.go`,
`internal/operation/repo/resources_status.go` and `internal/operation/actions/workflow_logs.go`.
Those are an existing contract and are deliberately left alone. New envelopes
MUST NOT follow them: reserve `total_count` for the server grand total and call
a local count `count`.

## Where the bound lives

Bounding is code, not a checklist a tool author re-implements. The rules above
are enforced in these places:

- **List use cases** (`internal/app/<domain>/`) hand their request to the list module,
  `internal/app/list`. Every paged list uses `list.Run` and returns the envelope with
  `has_next`; `list.Items` is gone, and a list with no continuation marker is
  still an envelope with `has_next`, derived by `list.Run`. Two other entry
  points remain for the cases `list.Run` does not fit: `list.Window` reads one
  page through a port's ceiling and takes `has_next` from the port's own report
  (the label lists, whose endpoint reports it, make no probe), and `list.Cut`
  serves a list Forgejo does not page. The module applies the
  instance ceiling (`list.Ceiling`, read from the port per call, never cached at
  package level), refuses a limit above a known ceiling with `OverCeilingError`
  rather than clamping it, and probes for a next page only when the ceiling is
  unknown. The page/limit defaults come from `list.Normalize`,
  `list.Given` and `list.DefaultLimit`. A new list tool calls the module and
  adds no defaults or ceiling check of its own.
- **Argument reading** (`internal/operation/<domain>/handlers.go`) uses
  `decode.PageOr` (an absent or zero number takes the default) and
  `decode.IntOr` (the default applies to the truncated number). These are the
  only copies of that rule; do not add a local `pageOr`.
- **Embedded lists in resources** use `internal/operation/resource.Bounded`, which
  applies the `EmbeddedListCap` ceiling and writes the truncation sentinel.

Some per-handler `paging` helpers in `internal/operation/{tracking,org,user,search}` and
in `internal/app/{wiki,pull}` remain. Each still has callers and each differs in how it
treats page 0, which the goldens pin. Do not copy one for a new tool; call the
list module.

## Documentation contract

Every bound parameter MUST appear in:

1. The tool's `mcp.NewTool()` description (per-parameter doc).
2. The README tool table.

An undocumented cap is the same trap as no cap.

## Checklist for new tools

The registry test (`internal/operation/registry_bound_test.go`) holds every list
tool and list resource template to the bound: a default call is a full page, the
payload says whether rows follow, paging reads every row once, and `page` and
`limit` are documented. A new list tool is covered once it has one seed row in
`pageToolsTable`. Code cannot answer the rest, so the PR description does:

- [ ] Is the output one fixed-shape object? If yes, exempt and say so.
- [ ] Which sub-rule 2 row matches the data type?
- [ ] If the tool reports `total_count`, does the upstream endpoint send
      `X-Total-Count`? The fake cannot show that; check the handler or a live
      response.
- [ ] If the change adds an entry to a test exemption map (`skippedListTools`,
      `ceilingExempt`), is its reason true?

If any answer is "none" or "unclear", the tool is not ready to merge.

## Open retrofit work

Two shapes of retrofit remain open. File each as a standalone issue, one tool at a
time, referencing this document. Both change a tool's output shape, so the issue
says so.

1. **Add a missing bound.** A tool whose output is data-proportional and exposes
   no `page`/`limit`, range, or per-file parameter at all. This is the sub-rule 2
   gap and the original motivation for this document.
2. **Shrink an already-bounded payload.** A tool that honours `page`/`limit` but
   returns raw SDK structs, so a bounded page is still enormous. Field projection
   is the fix, and the resource layer is the precedent: `forgejo://…/issues`
   returns projected rows with no bodies while the equivalent list tool does not.
   [#596](https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/596)
   is the worked example: `list_repo_pull_requests` still embeds a complete
   `Repository` twice per row through `PRBranchInfo` (`internal/app/model/model.go`).

Retrofit status as of the #141 close-out. The shape 1 gap is closed for every list
tool that had it: `list_my_stopwatches` takes `page`/`limit` and returns the
`has_next` envelope (#296), `get_repo_tree` takes `page`/`limit` over its entries
(#293), and `get_commit_statuses` returns the bounded envelope under the instance
ceiling (#295). `list_my_repos` was once a bare array that ADR 0003 froze; it now
returns the `has_next` envelope through `list.Run` (issue #294, a dated amendment to
ADR 0003).

The exemption maps as they stand:

- `skippedListTools` (`registry_bound_test.go`) is empty. No list tool is skipped.
- `ceilingExempt` (`pagination_ceiling_test.go`) holds one entry,
  `get_wiki_revisions`, with the reason that the revision history is deliberately not
  held to the instance ceiling (see `wiki.Service.Revisions`, which reads through a
  source that reports no ceiling).
- `unpagedListTools` does not exist in the tree; nothing is exempt on that basis.

**Wiki revisions decision.** The revision history stays a documented entity-read cap
with a truncation marker, not a ceiling-bound list. `get_wiki_revisions` pages with
`page`/`limit` and returns `has_next` and `total_count` (the body's grand total) with
no instance ceiling. The revisions embedded in the `forgejo://repo/{owner}/{repo}/wiki/{pageName}` resource
are cut at `resource.EmbeddedListCap`, and the truncation sentinel names
`get_wiki_revisions` as the remainder tool.
