<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# 0006. A missing parent is not found; an empty list is empty

Status: accepted

## Context

A list under a parent (the comments of an issue, the members of an org, the labels of an org) has two outcomes that look alike to an agent: the parent does not exist, and the parent exists with nothing in the list. Forgejo answers the first with a 404 on the list, and the second with a 200 and no rows. Before this change each domain made its own call: some use cases returned a per-domain not-found error, some returned an empty list for a 404, and the owner read, the label reads and the attachment reads each had a different error type for the same outcome.

Two other failures sit near the 404. A 403 or a 5xx on a parent's list says nothing about whether the parent exists, and a Forgejo message that merely contains the words "not found" under any other status is not evidence of a miss either.

## Decision

A Forgejo 404 on a parent means the parent is missing, and the use case returns the shared kind `forge.ErrNotFound`. An empty list means the parent exists and holds nothing, and the use case returns an empty list with no error.

The decision is made once, on the typed 404 only: `forge.IsNotFound`, which matches a `*forge.StatusError` carrying status 404. A 403 or 401 is `forge.ErrUnauthorized`, a 5xx is `forge.ErrUpstream`, and neither is ever read as not found, whatever its text says. A message that contains "not found" is never a miss. Use cases and adapters match `forge.ErrNotFound` or call `forge.IsNotFound` directly; they do not define a per-domain not-found error.

Two outcomes stay outside the shared kind, because each is pinned by something the caller can see:

- `owner.ErrOwnerNotFound` is the owner read's own outcome: a login that is neither a user nor an organization. Its wire message is pinned, and the resource layer maps it to the owner-specific text.
- `pull.DiffFileNotFoundError` is a file path that is not in the pull request's diff. It names the path, which the shared kind cannot.

## Alternatives considered

- **A not-found type per domain** (`issue.NotFoundError`, `repo.NotFoundError`, `owner.ErrOrgNotFound`, `attachment.ErrNotFound`). Rejected: each one was the same outcome under a different name, a caller had to know which domain it came from, and the contract suite could not check them with one assertion.
- **An empty list for every 404.** Rejected: an agent cannot tell a typo in the issue number from an issue with no comments, which is the failure this decision exists to prevent.

## Consequences

- One contract suite (`internal/forge/forgetest/parents.go`, `errors.go`) pins the decision for the fake and the Forgejo adapter alike.
- A new list under a parent follows the same rule with no new type: a 404 on the parent is `forge.ErrNotFound`, an empty page is an empty list.
- The two kept outcomes are the only domain-level not-found errors left. Adding another needs a reason that the shared kind cannot carry.
