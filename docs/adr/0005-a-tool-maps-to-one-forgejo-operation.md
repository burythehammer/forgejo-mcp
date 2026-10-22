<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# 0005. A tool maps to one Forgejo operation; workflow bundling lives in skills

Status: accepted

## Context

Issue #3 proposed compound tools: create a subtask linked to its parent spec, open a PR with assignee and labels, close out a merged spec. Each bundles several Forgejo calls. Two of the three encode one user's spec/subtask convention (the `## Parent` body layout, the subtask-list comment, one spec per PR), and this server is a general Forgejo MCP that others may run.

## Decision

A tool maps to one Forgejo operation. A workflow that strings several operations together belongs in a skill or in `docs/agents/`, not in the server. Where a call count is the real cost, fix it with a generic primitive on the existing tool (for example, `labels` and `assignees` arguments on `create_pull_request`, or `blocked_by` on `create_issue`), never with a tool that names a convention.

## Alternatives considered

- **Compound workflow tools in the server.** Rejected: bakes one user's convention into a shared server, and every extra tool adds to the standing cost that operators pay on each model request (issue #275).

## Consequences

- Skills carry the bundling, so a changed convention is a skill edit, not a release.
- Some workflows take more calls than a compound tool would. Generic arguments on existing tools reduce that without naming a convention.
