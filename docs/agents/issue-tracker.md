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
- **List labels**: `fj repo labels view` (no repo argument; run inside the clone)
- **Create a label**: `fj repo labels create <name> <hex-color>`. It does not de-duplicate, so read the list first. In zsh, call it from a function (`c(){ fj repo labels create "$1" "$2"; }`), since an unquoted loop variable is not word-split.
- **Label a PR**: `fj pr edit <n> labels --add <label>`

## Fallback: `executor` / `forgejo_api`

`fj` cannot assign a PR, set an issue's `ref`, link dependencies, retarget a PR or change repo settings. Use the `executor` MCP, path `forgejo_api.user.burythehammer.<tag>.<operation>`, taking `{owner, repo, index, body}`. Each write pauses for the user's approval. Find operations with `tools.search({namespace: "forgejo_api", query})` and `tools.describe.tool({path})`; copy names from there, not from memory.

Confirmed to work here:

- **Branch link**: `issue.issueEditIssue` with `body: {ref: "<branch>"}`.
- **Retarget a PR**: `repository.repoEditPullRequest` with `body: {base: "<branch>"}`.
- **Repo settings**: `repository.repoEdit` with `body: {default_branch: "<branch>"}`.
- **Native dependency link**: `issue.issueCreateIssueDependencies` on the blocked issue. The body is `IssueMeta` naming the blocker; the call is not yet exercised here.

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
