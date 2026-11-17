<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# 0008. outFile writes a whole paged listing to a local file

Status: accepted

## Context

A list tool returns one page, bounded by the instance ceiling (`max_response_items`). An agent that needs every row of a long list, to process by script, would otherwise page through the model's context. Spec #361 adds an `outFile` argument to `list_repo_issues` and `search_issues` that writes the whole listing to a file instead.

This is not a fan-out (ADR 0007): it reads one paged Forgejo list endpoint, page after page, under the same operation a page read uses (ADR 0005). What it adds is a local side effect, a file, which no list tool had before.

## Decision

`outFile` on a list tool:

- **Reads exhaustively, one endpoint.** It reads every row of the list through the list module (`list.ReadWhole` in `internal/app/list`), at the default limit lowered to a known instance ceiling. It is not a page: the reply says where the file is, not what the rows are.
- **Is bounded by a file ceiling, and fails loudly.** The most rows a file output may hold is `list.FileCeiling` (1,000), shared by every tool that takes `outFile`. A listing of exactly that many is written whole; one more row, or a reported total over it, fails with an error that names the ceiling and writes nothing. It never truncates. The ceiling is a server limit, not a caller-set bound, so it is an exemption from the caller-bounding rule of `docs/design/output-bounding.md`; the caller's bound is the file itself.
- **Writes a local file, on the stdio transport only.** The file goes in `<temp dir>/forgejo-mcp/` (created `0700`, refused if it is a symlink, open to group or other users, or owned by another uid), with a bare file name. A network transport refuses it before any listing, because the file would land on the server, where the caller cannot read it.
- **Is refused with `page` or `limit`.** Presence is read from the raw request, since a zero page or limit reads as absent in the argument struct (see `refuseOutFilePaging`). A null counts as absent.
- **Replies `{path, count, keys}`** in place of a page. `keys` can hold up to 1,000 values, so the reply is not a fixed shape; the ceiling is what bounds it.

## Alternatives considered

- **Page through the list in the skill.** Rejected: the agent pays a round trip per page and every row passes through its context, which is the cost `outFile` exists to avoid.
- **Truncate to the first 1,000 rows and say so.** Rejected: a script that processes a silently partial list is the failure output-bounding forbids (no silent truncation).
- **Raise the ceiling to the instance ceiling.** Rejected for now: the file ceiling is a separate, explicit limit so raising the instance ceiling never changes what a file holds.

## Consequences

- `list_repo_issues` and `search_issues` gain a side effect: they write to the temp directory. Their hint annotation is still `Read()`, which says the tool does not modify its environment. That is no longer true for the `outFile` path; see the open question below.
- A listing over 1,000 rows cannot be exported in one call. The caller narrows the query (state, labels, milestone, dates) until it fits, or pages by hand.
- The stdio-only rule means a remote (sse or http) client cannot use `outFile`. Those clients keep paged reads.

## Open question: hint annotation

`annotate.Read()` declares `readOnlyHint: true` on both tools. The `outFile` path writes a file, so the hint is inaccurate for that path. Changing it to `annotate.Write()` alters the `readOnlyHint` in `internal/operation/issue/testdata/list_repo_issues.schema.json` and `search_issues.schema.json`, and the two rows of `expectedHints` in `internal/operation/annotations_test.go`. Not yet decided; the decision would be a schema change recorded in an amendment to ADR 0003.

## Amendment: with_blockers (#363)

The statement above that outFile "is not a fan-out (ADR 0007)" holds for `outFile` alone. The `with_blockers` argument, valid only with `outFile`, adds a bounded dependency read per listed row: one `ListIssueDependencies` request per row, paged only when a row has more blockers than one page. That read is a fan-out under ADR 0007, and it is the only part of this file output that makes more than one kind of read. It stays on this path: whole listing, the file ceiling (`list.FileCeiling`) on the listing and on each row's blockers, and a failed or over-ceiling read fails the call and writes no file. Each row's `blocked_by` is an array of integers for blockers in the listed repository and `owner/repo#n` strings otherwise.
