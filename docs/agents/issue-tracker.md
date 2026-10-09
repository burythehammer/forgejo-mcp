# Issue tracker: Forgejo

Issues and specs live in Forgejo issues at `forgejo.burythehammer.com/burythehammer/forgejo-mcp` (account `burythehammer`). Use the `forgejo` MCP server (`.mcp.json`; tools are `mcp__forgejo__*`, with `owner: "burythehammer"`, `repo: "forgejo-mcp"`); not `gh`. It needs `FORGEJO_URL` and `FORGEJO_TOKEN` in the environment (see "MCP environment" below) and a built binary (`make build`, then install it as `forgejo-mcp` on `PATH`, or point `FORGEJO_MCP_EXEC` at it). Tool schemas are deferred: load them with `ToolSearch` (`select:mcp__forgejo__<name>`) before calling. Confirm names with `list_repo_labels` and the README tool table rather than from memory.

`upstream` (`git.b4mad.industries/agentic-forges/forgejo-mcp`) is the upstream project, not this repo's tracker. Beads (`bd`, see `AGENTS.md`) is local scratch tracking, not the tracker of record.

## MCP environment

The server name (`forgejo`) and variable names are identity-neutral; the values come from the environment, so the same `.mcp.json` shape works for either Forgejo account. Set them per directory with direnv: an `.envrc` in the parent folder of each identity's repos (for example `~/workspace/burythehammer/.envrc`) exports `FORGEJO_URL` and `FORGEJO_TOKEN`, and everything beneath it inherits them. Keep the token in a `chmod 600` file outside the repo and load it with `dotenv <file>` rather than writing it into the `.envrc`. Start `claude` from a shell inside that tree, run `direnv allow` once per `.envrc`, and check `/mcp` shows `forgejo` connected. Unset variables expand to empty and the server fails to authenticate, rather than falling back to another account.

If the server is not connected, `fj` (`fj <command> --help` for flag shapes; it infers the repo from `origin` inside a clone) is the stopgap: `fj issue create/view/comment/close`, `fj issue edit <n> labels --add <label>`, `fj pr create/merge`.

## Conventions

- **Create an issue**: `create_issue` with `title`, `body` and `labels` in one call (a separate labelling call is where a wrong issue number slips in). The body states the reasoning, not just the title.
- **Read an issue or PR**: read the resource `forgejo://repo/burythehammer/forgejo-mcp/issue/{index}` or `pr/{index}` (`ReadMcpResourceTool`, server `forgejo`); it returns the bounded entity with its comments. Fall back to `get_issue_by_index` / `get_pull_request_by_index`. See the resource table in `AGENTS.md`.
- **Search/list issues**: `list_repo_issues`, or `search_issues` across an owner.
- **Comment**: `create_issue_comment`
- **Apply / remove labels**: `add_issue_labels` / `remove_issue_labels`, by name; both also take a PR's index.
- **Assign**: `update_issue` (`assignees`) for issues, `update_pull_request` (`assignee`) for PRs.
- **Close**: a closing comment, then `issue_state_change` with `state: "closed"`.
- **List labels**: `list_repo_labels`
- **Create a label**: `create_repo_label`. Read the list first so it does not duplicate.
- **Native dependency link**: `add_issue_dependency` on the blocked issue.
- **Repo settings**: `edit_repo` (for example `default_branch`).

## Gaps: `executor` / `forgejo_api`

The MCP has no tool for an issue's `ref` (the branch link; see #23). Use the `executor` MCP, path `forgejo_api.user.burythehammer.issue.issueEditIssue`, with `body: {ref: "<branch>"}`. Each write pauses for the user's approval. For any other operation the MCP lacks, find it with `tools.search({namespace: "forgejo_api", query})` and `tools.describe.tool({path})`; copy names from there, not from memory.

## Specs and subtasks

One spec = one worktree = one PR.

- **Spec** (label `spec`): parent issue; owns the branch (`ref` field), the worktree and the single PR.
- **Subtask** (labels `subtask`, `ready-for-agent`): a tracer-bullet slice; lands as commits on the spec's branch (`Refs #N`), never its own PR. Body has `## Parent` and `## Blocked by`, plus native dependency links (`add_issue_dependency`).
- The spec gets a comment (not a body edit) listing its subtasks.
- The PR body lists `Closes #<spec>` and one `Closes #<subtask>` per line (not a comma list). After merge, verify each subtask is closed (read its resource) and close stragglers.

## Pull requests

**PRs as a request surface: no.**

- **Open**: `create_pull_request` (`head` = the branch, `base` = `main`); assign to yourself with `update_pull_request` and apply existing labels with `add_issue_labels`.
- **Retarget**: `update_pull_request` with `base`.
- **Merge**: `merge_pull_request` with `delete_branch_after_merge: true` and the repo's merge `style`, only after explicit user go-ahead.

## When a skill says "publish to the issue tracker"

Create a Forgejo issue with `create_issue`.

## When a skill says "fetch the relevant ticket"

Read the issue's resource (see Conventions).

## Wayfinding operations

Used by `/wayfinder`. The **map** is a single issue with **child** issues as tickets.

- **Map**: an issue labelled `wayfinder:map` (Notes / Decisions-so-far / Fog body). Create the label on first use.
- **Child ticket**: an issue with `Part of #<map>` at the top of its body; labels `wayfinder:<type>` (`research`/`prototype`/`grilling`/`task`). Once claimed, assign to the driving dev.
- **Blocking**: Forgejo native dependencies (`add_issue_dependency`); also a `Blocked by: #<n>` line at the top of the body. A ticket is unblocked when every blocker is closed.
- **Frontier query**: list the map's open children, drop any with an open blocker or an assignee; first in map order wins.
- **Claim**: assign the issue to yourself as the session's first write.
- **Resolve**: comment the answer, close the issue, then append a pointer to the map's Decisions-so-far.
