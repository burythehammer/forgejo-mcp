# Issue tracker: Forgejo

Issues and specs live in Forgejo issues at `forgejo.burythehammer.com/burythehammer/forgejo-mcp` (account `burythehammer`). Use the `fj` CLI; not `gh`. Run `fj <command> --help` to check flag shapes. Infer the repo from `git remote -v` (`origin`); `fj` does this inside a clone.

`upstream` (`git.b4mad.industries/agentic-forges/forgejo-mcp`) is the upstream project, not this repo's tracker. Beads (`bd`, see `AGENTS.md`) is local scratch tracking, not the tracker of record.

## Conventions

- **Create an issue**: `fj issue create "<title>" --body "<why>"`. The body states the reasoning, not just the title.
- **Read an issue**: `fj issue view <number>`
- **Search/list issues**: `fj issue search ...`
- **Comment**: `fj issue comment <number> "..."`
- **Apply / remove labels**: `fj issue edit <number> labels --add <label>` / `--remove <label>`
- **Close**: `fj issue close <number> --with-msg "..."`
- **Create a label**: `fj repo labels create <name> <hex-color>` (note: it does not de-duplicate; check `fj repo labels view` first)

## Fallback: `executor` / `forgejo_api`

`fj` has no subcommand for these, so use the `executor` MCP `forgejo_api`:

- **Native dependency link** (blocked-by): `issue.issueAddIssueDependency`-style call on the blocked issue, naming the blocker.
- **Branch link**: `issue.issueEditIssue` with `body: {ref: "<branch>"}` sets the issue's `ref` field.

## Specs and subtasks

One spec = one worktree = one PR.

- **Spec** (label `spec`): parent issue; owns the branch (`ref` field), the worktree and the single PR.
- **Subtask** (labels `subtask`, `ready-for-agent`): a tracer-bullet slice; lands as commits on the spec's branch (`Refs #N`), never its own PR. Body has `## Parent` and `## Blocked by`, plus native dependency links.
- The spec gets a comment (not a body edit) listing its subtasks.
- The PR body lists `Closes #<spec>` and one `Closes #<subtask>` per line (not a comma list). After merge, verify each subtask is closed with `fj issue view <n>` and close stragglers.

## Pull requests

**PRs as a request surface: no.**

- **Open**: `fj pr create "<title>" --body "<body>" --base main`; assign to yourself and apply existing labels.
- **Merge**: `fj pr merge <pr> -d`, only after explicit user go-ahead.

## When a skill says "publish to the issue tracker"

Create a Forgejo issue with `fj issue create`.

## When a skill says "fetch the relevant ticket"

`fj issue view <number>`.

## Wayfinding operations

Used by `/wayfinder`. The **map** is a single issue with **child** issues as tickets.

- **Map**: an issue labelled `wayfinder:map` (Notes / Decisions-so-far / Fog body). Create the label on first use.
- **Child ticket**: an issue with `Part of #<map>` at the top of its body; labels `wayfinder:<type>` (`research`/`prototype`/`grilling`/`task`). Once claimed, assign to the driving dev.
- **Blocking**: Forgejo native dependencies via the `forgejo_api` fallback; also a `Blocked by: #<n>` line at the top of the body. A ticket is unblocked when every blocker is closed.
- **Frontier query**: list the map's open children, drop any with an open blocker or an assignee; first in map order wins.
- **Claim**: assign the issue to yourself as the session's first write.
- **Resolve**: comment the answer, close the issue, then append a pointer to the map's Decisions-so-far.
