<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# 0007. A tool may fan out bounded reads where Forgejo has no single endpoint

Status: accepted

## Context

ADR 0005 holds that a tool maps to one Forgejo operation. Wiki search (#356) cannot: Forgejo offers no wiki search endpoint, only a page list and a single-page read. Finding a page by content means listing the pages and reading each one.

## Decision

A tool may fan out several Forgejo reads when all three hold:

- Forgejo has no single endpoint for the operation.
- The fan-out is read-only.
- The tool bounds it (pages scanned, hits returned, bytes read) and reports truncation, per `docs/design/output-bounding.md`.

A write workflow, or a sequence that encodes one user's convention, stays with ADR 0005: it belongs in a skill.

## Alternatives considered

- **A skill carries the list-then-get loop.** Rejected: the agent pays a round trip per page and cannot match on content without reading every page into its context.
- **A `query` argument on `list_wiki_pages`.** Rejected: it overloads the list result shape (titles versus hits with snippets) and hides the fan-out behind a list tool.

## Consequences

- `search_wiki` is a new tool, adding to the standing cost operators pay per model request (#275).
- Clients can come to depend on its hit shape, so it is hard to change later.
- Cloning the wiki repository and searching locally is the way out if wikis outgrow the page cap; it is not built now.

## Amendment: create_wiki_page precondition (#357)

`create_wiki_page` refuses an existing title, which takes one read before its one write. That read is a precondition of a single operation, not a workflow, so it stays within ADR 0005. The check and the create are two requests, so two simultaneous creates of one new title can both pass the check; the docs/reference/tools.md row for `create_wiki_page` documents the race.

## Amendment: with_blockers on file output (#363)

`list_repo_issues` and `search_issues`, called with `outFile` and `with_blockers`, read the dependencies of each listed issue: one `ListIssueDependencies` read per listed issue, more pages only when an issue has more blockers than one page. Forgejo has no endpoint that returns the blockers of many issues at once, so this is a fan-out under the three conditions above. It is read-only. It is bounded by the file-output ceiling (`list.FileCeiling`, 1,000) on the listing and on each issue's blockers. The third condition, that the tool bounds the fan-out and reports truncation, is met here by failing loudly past the ceiling instead: nothing is truncated, so there is no truncation to report. A read over the ceiling, or any failed read, fails the call and writes no file. The reads run on the file-output path of ADR 0008, not a page.
