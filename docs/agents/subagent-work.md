<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Working as, or briefing, a subagent

Read this when you implement a ticket in a worktree, merge subagent branches into an integration branch, or write a prompt for either.

## Implementer

- **Seams.** Take the test seams from the spec's "Testing Decisions". No one is there to confirm them with, so the `tdd` skill's confirmation step is already answered by the spec.
- **Done means `make check` is green**, and the report quotes its real result. A script that fails on the base branch too is named as such, not hidden.
- **Trailer.** End each commit with the `Co-Authored-By` line the briefing gives. Add no `Assisted-by` or other footer.

## Lead merging subagent work

- Run `make check` on the integration branch after every merge. An implementer's "all green" is a claim; the merged tip is the evidence. A merge of two green branches has failed before (the since-retired `check-app-boundary.sh`).
- Put the trailer text and the `make check` instruction in each implementer's prompt, since subagents cannot see memory.

## Reviewer

- Confirm a "regression from the base" claim with `git show <base>:<file>` before reporting it.
