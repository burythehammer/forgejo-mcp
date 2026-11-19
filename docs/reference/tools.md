<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# Tool reference

Every tool the server registers, one row each. The tool table is the only place a tool row lives: `scripts/ci/check-readme-tool-coverage.sh` reads it, and fails the build on a registered tool with no row here. Toolsets and presets are in [toolsets.md](toolsets.md); resource templates in [resources.md](resources.md); CLI invocation in [cli.md](cli.md).

## Available Tools

Paged list tools return an envelope with `page`, `limit`, `count`, `has_next` and, when the server reports one, `total_count`. When the applied `limit` differs from the one asked for or defaulted to (for example a default lowered to the instance maximum), the envelope also carries `limit_requested`.

| Tool | Description |
|------|-------------|
| **User** | |
| `get_my_user_info` | Get information about the authenticated user |
| `check_notifications` | Check and list user notifications (page/limit); returns `{notifications,page,limit,count,has_next}` |
| `get_notification_thread` | Get detailed info on a single notification thread |
| `mark_notification_read` | Mark a single notification thread as read |
| `mark_all_notifications_read` | Acknowledge all notifications |
| `list_repo_notifications` | Filter notifications scoped to a single repository (page/limit); returns `{notifications,page,limit,count,has_next}` |
| `mark_repo_notifications_read` | Mark all notifications in a specific repo as read |
| `search_users` | Search for users (page/limit); returns `{users,page,limit,count,has_next}` |
| **Repositories** | |
| `list_my_repos` | List repositories you own, one page at a time (`page`/`limit`, default 100, capped at the instance maximum). Returns `{repositories,page,limit,count,has_next}`; `has_next` true means a further page exists. |
| `get_repo` | Get a single repository by owner and name |
| `create_repo` | Create a new repository |
| `fork_repo` | Fork a repository |
| `edit_repo` | Edit repository settings. Only fields you pass change; omitted fields are left unchanged. Providing no fields is an error. |
| `search_repos` | Search for repositories (page/limit); returns `{repositories,page,limit,count,has_next}` |
| **Topics** | |
| `list_repo_topics` | List a repository's topics. Bounded by `page` (default 1) + `limit` (default 100); returns `{topics,page,limit,count,has_next}`. |
| `set_repo_topics` | Replace all topics. `topics` is a required comma-separated string; an empty string clears. Invalid names are rejected before the request. |
| `add_repo_topic` | Add one topic |
| `delete_repo_topic` | Delete one topic |
| **Branches** | |
| `list_branches` | List all branches in a repository (page/limit); returns `{branches,page,limit,count,has_next}` |
| `create_branch` | Create a new branch |
| `delete_branch` | Delete a branch |
| **Branch Protection** | |
| `list_branch_protections` | List a repository's branch protection rules. Bounded by `page` (1-based) + `limit` (page size); returns `{branch_protections,page,limit,count,has_next}`, so callers can fetch the next page. |
| `get_branch_protection` | Get a single rule by `rule` name |
| `create_branch_protection` | Create a rule. Requires `branch_name`; `status_check_contexts` is a comma-separated list of required checks (e.g. `"ci/build,ci/test"`). |
| `edit_branch_protection` | Edit a rule by `rule` name. Only fields you pass change; omitted fields are left unchanged. |
| `delete_branch_protection` | Delete a rule by `rule` name |
| **Webhooks** | |
| `list_repo_hooks` | List repository webhooks. Bounded by `page` (default 1) + `limit` (default 30; a limit above the instance's max_response_items is refused); returns `{hooks,page,limit,count,total_count,has_next}`, where `total_count` appears only when Forgejo reports `X-Total-Count`. |
| `get_repo_hook` | Get a single repository webhook by ID |
| `create_repo_hook` | Create a repository webhook. The secret is accepted but never echoed in the response. |
| `edit_repo_hook` | Edit a repository webhook. Only fields you pass change; omitted fields are left unchanged. |
| `delete_repo_hook` | Delete a repository webhook by ID |
| `test_repo_hook` | Trigger a test delivery for a repository webhook — WARNING: triggers a live HTTP delivery |
| **Files** | |
| `get_file_content` | Get the content of a file. Optional `start_line`/`end_line` request a 1-indexed inclusive line range (clamps to file extent; with `with_metadata=true` the range replaces the content and keeps the metadata). A whole file or range over 1 MiB of text is refused with a message asking for a narrower range. |
| `list_repo_contents` | List files and directories at a path. `path=""` lists the repository root. Returns one level, one page at a time (`page`/`limit`, default 100 entries) with `has_next`; for a full tree use `get_repo_tree` with `recursive=true`. |
| `get_repo_tree` | Get the Git tree. `recursive=true` returns the complete file tree in one response (subject to the server's tree-endpoint size cap); `recursive=false` (default) returns one level. `page`/`limit` (default 1000) select one page of the entries, under `tree`, with `has_next` saying whether more follow. |
| `create_file` | Create a new file. Supply exactly one of plain-text `content` or binary-safe `content_base64` (strict standard Base64). |
| `update_file` | Update an existing file. Supply exactly one of plain-text `content` or binary-safe `content_base64` (strict standard Base64). |
| `delete_file` | Delete a file |
| `change_files` | Atomically create, update, and delete multiple files in one native Forgejo commit; create/update operations accept `content` or `content_base64`. |
| **Commits** | |
| `list_repo_commits` | List commits in a repository (page/limit); returns `{commits,page,limit,count,has_next}` |
| `get_commit_statuses` | List per-context commit statuses for a full 40-character SHA. Bounded by `page` (default 1) + `limit` (default 30, lowered to the instance ceiling; a limit above a known ceiling is refused); returns `{sha, statuses, page, limit, count, has_next, total_count}` — `has_next` is true when a further page may exist, and `total_count` is present only when Forgejo reports `X-Total-Count`. Combined aggregate stays on the commit status resource. Not Actions runs (`list_workflow_runs`). |
| **Issues** | |
| `list_repo_issues` | List issues in a repository (page/limit); returns `{issues,page,limit,count,has_next}`. Each issue is a compact result by default (number, url, title, state, label names, assignee logins, milestone, updated_at, is_pull_request); `verbose=true` returns the full issues. Optional `sort` orders server-side: `relevance`, `latest`, `oldest`, `recentupdate`, `leastupdate`, `mostcomment`, `leastcomment`, `nearduedate`, `farduedate` (the last two are the due-date directions). Optional `outFile` (a bare file name, no path) writes the whole listing, every page, as a JSON array into `<temp dir>/forgejo-mcp/` and replies `{path,count,keys}` instead of a page; an existing file is overwritten. The listing is held to a ceiling of 1,000 issues (see [File output and its ceiling](#file-output-and-its-ceiling)): a listing over it fails with an error naming the ceiling and writes no file, and is never truncated. The file is written to a temp file and renamed into place, so a failed run keeps an earlier file of the same name. The directory is created `0700` and refused if it is a symlink, open to group or other users, or owned by another uid. `outFile` is refused with `page` or `limit`, and on the `sse` and `http` transports. Optional `with_blockers` (requires `outFile`) adds `blocked_by` to each file row: an array of the blocking issues, each an integer issue number for a blocker in the listed repository or an `owner/repo#n` string for one in another repository. It makes one extra dependency request per listed issue (see [File output and its ceiling](#file-output-and-its-ceiling)). |
| `search_issues` | Search issues across every repository of one owner (page/limit); returns `{issues,page,limit,count,has_next,total_count}` — `total_count` is present only when Forgejo reports `X-Total-Count`. Each issue is a compact result by default (number, url, title, state, label names, assignee logins, milestone, updated_at, is_pull_request); `verbose=true` returns the full issues. Optional `outFile` (a bare file name, no path) writes the whole result, every page, as a JSON array into `<temp dir>/forgejo-mcp/` and replies `{path,count,keys}` instead of a page; an existing file is overwritten. `outFile` is refused with `page` or `limit`, and on the `sse` and `http` transports. Optional `with_blockers` (requires `outFile`) adds `blocked_by` to each file row, as on `list_repo_issues`: integers for blockers in the searched repository, `owner/repo#n` strings for others. |
| `create_issue` | Create a new issue. Optional `labels` (comma-separated names or IDs; two exclusive labels from one scope are refused and no issue is created), `assignees` (comma-separated usernames) and `milestone` (numeric ID) are applied by the same request that creates the issue. Optional `ref` links the issue to a branch (Forgejo's issue `ref` field, passed through as given). Returns a compact result by default (number, url, title, state, label names, assignee logins, milestone, updated_at, and `body_sha256` when a body was sent: the hash of the body the server stored); `verbose=true` returns the full issue. |
| `get_issue_by_index` | Get a specific issue. Returns a compact result by default (number, url, title, state, label names, assignee logins, milestone, updated_at, is_pull_request, body, closed_at, ref, comments); `verbose=true` returns the full issue. Optional `type=issues` refuses a pull request's index and `type=pulls` refuses an issue's, each with an error naming the other tool; omit `type` to accept either |
| `add_issue_labels` | Add labels to an issue. `labels` takes comma-separated label names or numeric IDs; an unrecognised name is an error, and two labels from one exclusive scope are refused; nothing is applied in either case. Returns a compact result by default (number, url, title, state, label names, assignee logins, updated_at); `verbose=true` returns the full issue. |
| `remove_issue_labels` | Remove labels from an issue. `labels` takes comma-separated label names or numeric IDs. Returns a compact result by default (number, url, title, state, label names, assignee logins, updated_at); `verbose=true` returns the full issue. |
| `update_issue` | Update an existing issue (requires numeric milestone ID). `due_date` sets the deadline (RFC3339); `clear_due_date=true` removes it. The two are mutually exclusive — setting both is an error, and omitting both leaves the deadline unchanged. `set_labels` replaces the issue's entire label set (names or IDs); an empty string clears every label; two exclusive labels from one scope are refused and nothing is applied. `ref` sets the branch the issue is linked to (passed through as given); omitting it leaves it unchanged and an empty string clears it. Returns a compact result by default (number, url, title, state, label names, assignee logins, milestone, updated_at; the submitted body is not echoed back, but `body_sha256` is the hash of the stored body when one was sent); `verbose=true` returns the full issue. |
| `replace_issue_body_line` | Replace one exact piece of an issue's body on the server, without sending the whole body. `old` is a case-sensitive substring (it may span a whole line), must not be empty, and must occur exactly once: zero matches or several refuse with the body unchanged (several report the count). `new` may be empty to delete. Pull request indexes are refused. Returns a compact result (number, url, title, state, label names, assignee logins, updated_at), `body_sha256` of the stored body, and the `old` and `new` text. Limitation: Forgejo has no compare-and-swap for issue edits, so an edit made between the read and the write can be lost; compare `updated_at` to detect it. |
| `issue_state_change` | Open or close an issue. Returns a compact result by default; `verbose=true` returns the full issue |
| `list_issue_dependencies` | List issues the given issue depends on (page/limit); returns `{issues,page,limit,count,has_next}` |
| `list_issue_dependents` | List issues that depend on the given issue (page/limit); returns `{issues,page,limit,count,has_next}` |
| `add_issue_dependency` | Make one issue depend on another. The dependency may live in a different repository: optional `depends_on_owner`/`depends_on_repo` default to `owner`/`repo`. |
| `remove_issue_dependency` | Remove a dependency from an issue. For a cross-repo dependency, optional `dependency_owner`/`dependency_repo` default to `owner`/`repo`. |
| `list_repo_milestones` | List milestones with their IDs (use with `update_issue`) (page/limit); returns `{milestones,page,limit,count,has_next}` |
| `create_milestone` | Create an open milestone (`title` required; optional `description` and `due_date`, RFC3339). Returns the milestone as stored: `id`, `title`, `description`, `state`, `due_date`, `open_issues`, `closed_issues`. |
| `edit_milestone` | Edit a milestone by numeric `id`. Only the fields given change: `title`, `description`, `state` (`open` or `closed`, which closes or reopens it) and `due_date` (RFC3339). A due date cannot be cleared. Returns the milestone as stored. |
| `get_milestone` | Get one milestone by numeric `id`, in the same shape as the write tools. A missing id is an error. |
| `list_repo_labels` | List labels with their IDs (page/limit); returns `{labels,page,limit,count,has_next,total_count}`. Merges org-level labels for org-owned repos (set `include_org_labels=false` to opt out). Each entry carries a `scope` field (`"repo"` or `"org"`) and an `exclusive` flag. |
| `list_org_labels` | List organization-level labels with their IDs (page/limit); returns `{labels,page,limit,count,has_next,total_count}`, each with an `exclusive` flag. The assignment tools accept label names directly, so this is for discovery and for the rare name that exists in both the repo and the org scope. |
| `create_repo_label` | Create a repository label (`name`, `color` as 6-digit hex, optional `description`, optional `exclusive`). Returns the numeric `id`; `add_issue_labels` also takes the name. `exclusive=true` marks the label's scope (the name before the last `/`, e.g. `status` in `status/ready`) as exclusive: one write may not name two of its labels. A label the issue already carries is not counted. The result reports the stored `exclusive`; if Forgejo does not store it, the call fails. |
| `edit_repo_label` | Edit a repository label (PATCH — only supplied fields change: `name`, `color`, `description`, `exclusive`). Omitting `exclusive` leaves the stored value unchanged. |
| `delete_repo_label` | Delete a repository label. Refuses by default when the label is in use (reports count); set `delete_mode=force` to override. |
| `get_repo_label` | Get a single repository label by numeric `id`. |
| `create_org_label` | Create an organization-level label. Same fields as `create_repo_label`, including `exclusive`. |
| `edit_org_label` | Edit an organization-level label (PATCH semantics; `exclusive` is optional and omitting it leaves it unchanged). |
| `delete_org_label` | Delete an organization-level label. In-use guard counts across visible org repos (best-effort); `delete_mode=force` overrides. |
| `get_org_label` | Get a single organization-level label by numeric `id`. |
| **Comments** | |
| `list_issue_comments` | List comments on an issue or PR (page/limit); returns `{comments,page,limit,count,has_next}` |
| `get_issue_comment` | Get a specific comment. Returns a compact result by default (id, url, author login, body, created_at, updated_at); `verbose=true` returns the full comment |
| `create_issue_comment` | Add a comment to an issue or PR. Returns a compact result by default (id, url, updated_at; the body is not echoed back); `verbose=true` returns the full comment |
| `edit_issue_comment` | Edit a comment. Returns a compact result by default (id, url, updated_at; the body is not echoed back); `verbose=true` returns the full comment |
| `delete_issue_comment` | Delete a comment |
| **Pull Requests** | |
| `list_repo_pull_requests` | List pull requests in a repository (page/limit); returns `{pull_requests,page,limit,count,has_next}`. Each pull request is a compact result by default (number, url, title, state, label names, assignee logins, updated_at, head and base branch names); `verbose=true` returns the full pull requests |
| `get_pull_request_by_index` | Get a specific pull request. Returns a compact result by default (number, url, title, state, label names, assignee logins, updated_at, head and base branch names, body, merged, merged_at, closed_at, mergeable, merge_commit_sha, head_sha, comments); `verbose=true` returns the full pull request |
| `create_pull_request` | Create a new pull request, optionally with `labels` (names or IDs) and `assignees` (usernames) in the same call; an unknown label fails the call and creates nothing. Returns a compact result by default (number, url, title, state, label names, assignee logins, updated_at, head, base); the submitted body is not echoed back. `verbose=true` returns the full pull request |
| `update_pull_request` | Update an existing pull request. Returns a compact result by default (the submitted body is not echoed back); `verbose=true` returns the full pull request |
| `list_pull_reviews` | List reviews for a pull request (page/limit); returns `{reviews,page,limit,count,has_next}` |
| `get_pull_review` | Get a specific pull request review. Returns a compact result by default (id, url, author login, state, body, commit_id, stale, dismissed, official, comments_count, submitted_at); `verbose=true` returns the full review |
| `list_pull_review_comments` | List comments on a pull request review, one page at a time (`page`/`limit`, default 20) with `has_next` |
| `list_pull_request_files` | List changed files in a pull request (page/limit); returns `{files,page,limit,count,has_next}`. Use the returned filenames as the `file_path` argument to `get_pull_request_diff`. |
| `get_pull_request_diff` | Get the unified diff of a pull request, one page of whole files at a time: `page`/`limit` (default 50 files). A page with files after it ends with a truncation marker naming the next page. Optional `file_path` returns only that file's hunks and ignores paging (matches on either pre- or post-rename path). |
| `merge_pull_request` | Merge a pull request (style: merge/rebase/rebase-merge/squash; optional title/message/delete-branch/force-merge/wait-for-checks). |
| `create_pull_review` | Create a review on a pull request (state: APPROVED/REQUEST_CHANGES/COMMENT) with optional inline comments. |
| `submit_pull_review` | Submit a pending pull request review |
| `dismiss_pull_review` | Dismiss a pull request review |
| `delete_pull_review` | Delete a pending pull request review |
| `create_review_requests` | Request reviews from specific users or teams |
| `delete_review_requests` | Cancel pending review requests |
| **Packages** | |
| `list_packages` | List package versions of a user or org (one row per version). Optional `type` and `q`. Server-paged via `page`/`limit` (default 30, max 50). Envelope `{packages, page, limit, count, has_next, total_count?}`. A missing owner is an error, not an empty list |
| `get_package` | Get one package version. Does not embed owner/creator users |
| `delete_package` | Delete one package version (not every version of the name). No preflight. 4xx/5xx stay errors |
| `list_package_files` | List files of one package version. Client-paged via `page`/`limit` (default 30, max 50); envelope `{files, page, limit, count, has_next, total_count}` (`total_count` is the grand total: the file count of the version) |
| **Actions** | |
| `dispatch_workflow` | Trigger a workflow run via `workflow_dispatch` event |
| `list_workflow_runs` | List workflow runs with optional filtering by status, event, or SHA. Paged by `page`/`limit`; returns `{workflow_runs, page, limit, count, total_count?, has_next}` |
| `get_workflow_run` | Get details of a specific workflow run by ID |
| `list_action_run_jobs` | List jobs for a Forgejo v16+ workflow run with client-side `page` and `limit` bounds. Returns `{run_id, jobs, page, limit, count, total_count, has_next, next_page?}`; `total_count` is always present here because Forgejo does not page jobs, so the whole list is read and counted. |
| `get_action_job_logs` | Read a Forgejo v16+ job log with resumable `offset` and `max_bytes` bounds; defaults to the tail |
| `cancel_workflow_run` | Cancel a pending or running workflow run. Already-finished runs also return success (HTTP 204); the run is left unchanged |
| `delete_workflow_run` | Delete a completed workflow run. A live run is an API error. Removes the run and its job logs; Forgejo marks that run's artifacts deleted |
| `list_action_run_artifacts` | List artifacts of a workflow run. Server-paged via `page`/`limit` (default 30, max 50); optional `name` filter. Envelope `{run_id, artifacts, page, limit, count, total_count?, has_next}` |
| `get_action_artifact` | Get metadata for one Actions artifact. Does not download the zip |
| `list_repo_action_secrets` | List a repository's Actions secrets (names only; Forgejo never returns values). Client-paged via `page`/`limit` (default 30, max 50); returns `{owner, repo, secrets, page, limit, count, has_next}` |
| `create_or_update_repo_action_secret` | Create or update a repository Actions secret. Overwrites any existing secret with the same name; the value is never echoed back |
| `delete_repo_action_secret` | Delete a repository Actions secret |
| `list_org_action_secrets` | List an organization's Actions secrets (names only; Forgejo never returns values). Client-paged via `page`/`limit` (default 30, max 50); returns `{org, secrets, page, limit, count, has_next}` |
| `create_or_update_org_action_secret` | Create or update an organization Actions secret. Overwrites any existing secret with the same name; the value is never echoed back |
| `delete_org_action_secret` | Delete an organization Actions secret |
| **Organizations** | |
| `list_my_orgs` | List my organizations (page/limit); returns `{organizations,page,limit,count,has_next}` |
| `list_user_orgs` | List a user's organizations (page/limit); returns `{organizations,page,limit,count,has_next}` |
| `get_org` | Get organization details |
| `create_org` | Create an organization |
| `edit_org` | Edit organization settings |
| `delete_org` | Delete an organization — **destructive and irreversible**: all repos, teams and data are permanently removed |
| `list_org_members` | List members of an organization (page/limit); returns `{members,page,limit,count,has_next}` |
| `check_org_membership` | Check if a user is a member of an organization |
| `remove_org_member` | Remove a member from an organization |
| `list_org_teams` | List teams in an organization (page/limit); returns `{teams,page,limit,count,has_next}` |
| `search_org_teams` | Search for teams in an organization (page/limit); returns `{teams,page,limit,count,has_next}` |
| `create_org_team` | Create a team in an organization |
| `add_team_member` | Add a user to a team |
| `remove_team_member` | Remove a user from a team |
| `add_team_repo` | Add a repository to a team |
| `remove_team_repo` | Remove a repository from a team |
| **Time Tracking** | |
| `list_issue_tracked_times` | List tracked time entries on an issue or PR (page/limit); returns `{times, page, limit, count, has_next}` |
| `list_repo_tracked_times` | List tracked time entries across a repository (page/limit); returns `{times, page, limit, count, has_next}` |
| `list_my_tracked_times` | List your own tracked time entries, one page at a time (`page`/`limit`, default 20) with `has_next` |
| `add_issue_time` | Log time against an issue or PR (accepts seconds or duration like `15m`) |
| `reset_issue_time` | Delete ALL tracked time entries on an issue or PR (destructive) |
| `delete_issue_time_entry` | Delete a single tracked time entry by ID |
| `start_issue_stopwatch` | Start a stopwatch on an issue or PR |
| `stop_issue_stopwatch` | Stop a running stopwatch and record the elapsed time |
| `cancel_issue_stopwatch` | Cancel a running stopwatch without recording |
| `list_my_stopwatches` | List your currently running stopwatches, one page at a time (`page`/`limit`, default 20) with `has_next` |
| **Attachments** | |
| `list_issue_attachments` | List attachments on an issue or PR, one page at a time (`page`/`limit`, default 20) with `has_next` |
| `get_issue_attachment` | Get metadata for a single issue/PR attachment |
| `download_issue_attachment` | Download an issue/PR attachment (inline if < 1 MiB; metadata + URL otherwise) |
| `create_issue_attachment` | Upload a new attachment to an issue or PR (base64 content or a file path on the MCP host) |
| `edit_issue_attachment` | Rename an issue/PR attachment |
| `delete_issue_attachment` | Delete an issue/PR attachment |
| `list_comment_attachments` | List attachments on an issue/PR comment, one page at a time (`page`/`limit`, default 20) with `has_next` |
| `get_comment_attachment` | Get metadata for a single comment attachment |
| `download_comment_attachment` | Download a comment attachment (inline if < 1 MiB; metadata + URL otherwise) |
| `create_comment_attachment` | Upload a new attachment to an issue/PR comment (base64 content or a file path on the MCP host) |
| `edit_comment_attachment` | Rename a comment attachment |
| `delete_comment_attachment` | Delete a comment attachment |
| **Releases** | |
| `list_releases` | List releases for a repository (page/limit + client-side `state` filter: all/draft/prerelease/published). Envelope `{releases, page, limit, count, has_next}` |
| `get_release_by_id` | Get a release by numeric ID |
| `get_release_by_tag` | Get a release by tag name |
| `get_latest_release` | Get the latest non-draft, non-prerelease release |
| `create_release` | Create a new release (pass `target_commitish` to also create the tag) |
| `edit_release` | Update fields of an existing release (only supplied fields are sent) |
| `delete_release` | Delete a release by numeric ID — destructive |
| `delete_release_by_tag` | Delete a release by tag name — destructive, verify tag |
| `list_release_attachments` | List attachments on a release (response fetched in full, sliced client-side). Envelope `{attachments, page, limit, count, has_next}` |
| `get_release_attachment` | Get metadata for a single release attachment |
| `download_release_attachment` | Download a release attachment (inline if < 1 MiB; metadata + URL otherwise) |
| `create_release_attachment` | Upload a new attachment to a release (base64 content or a file path on the MCP host) |
| `edit_release_attachment` | Rename a release attachment |
| `delete_release_attachment` | Delete a release attachment — destructive |
| **Wiki** | |
| `list_wiki_pages` | List pages using `page`/`limit`. Envelope `{pages, page, limit, count, has_next}`. No `total_count`: Forgejo's `X-Total-Count` counts raw wiki tree entries, not pages, so it is not an exact grand total and is omitted. |
| `search_wiki` | Plain-text, case-insensitive search of a repo's wiki pages by title and body line. Returns `{hits, pages_scanned, pages_skipped, truncated}`; a hit is `{page_name, line, snippet}`, and a title match has no line. Bounded: at most 100 pages scanned and 20 hits returned, snippets capped at 120 bytes, and a page over 1 MiB is skipped and counted. `truncated: true` means a cap stopped the search. |
| `get_wiki_page` | Read decoded Markdown; optional `start_line`/`end_line`, always returns `total_lines`. |
| `get_wiki_revisions` | List revision history using `page`/`limit`; returns `has_next` and `total_count`, the page's total revision count as reported in the response body. |
| `create_wiki_page` | Create a page and return its server-normalized `page_name`; slash-separated titles are a flat subpage naming convention (no hierarchy and no automatic parent page), and an existing title is refused (use `update_wiki_page`); Forgejo itself also answers 400 for an existing page. A percent-encoded title (such as `a%2Fb`, copied from a `page_name`) is refused without a request, because Forgejo stores it as a different page. A title with a hyphen is stored as `<title>.-` and read back only by the `page_name` the create answered. The existence check and the create are two requests, so two simultaneous creates of one new title can both succeed. |
| `update_wiki_page` | Update content/title by normalized `page_name`; last writer wins. A `page_name` that does not exist is refused (use `create_wiki_page`): Forgejo's own edit would create the page, so the tool reads it first. |
| `delete_wiki_page` | Delete a page by normalized `page_name`. |
| **Server** | |
| `get_forgejo_mcp_server_version` | Get the MCP server version |

### File output and its ceiling

`outFile` on `list_repo_issues` writes a whole listing to a file, so a client can process every issue by script without the list passing through the model. The file goes in `<temp dir>/forgejo-mcp/`, which is created `0700` and refused if it is a symlink, open to other users, or owned by another uid. The reply is `{path,count,keys}`.

The **ceiling** is the most rows one file-output listing may hold: 1,000 issues (`list.FileCeiling` in `internal/app/list`, shared by every tool that takes `outFile`). A listing of exactly 1,000 is written whole. One more fails with an error that names the ceiling and writes no file. A failed run leaves an earlier file of the same name untouched. The ceiling is separate from the instance ceiling (`max_response_items`) and does not change the per-page bound on normal list output.

`with_blockers` (valid only with `outFile`, on `list_repo_issues` and `search_issues`) adds `blocked_by` to each file row: the issues that block it, `[]` when none. A blocker in the listed repository is an integer issue number; a blocker in another repository is an `owner/repo#n` string. It makes one extra Forgejo dependency request per listed issue (more only when an issue has more blockers than one page), the fan-out ADR 0007 allows because Forgejo has no bulk endpoint for blockers. Each request is bounded by the same ceiling, and a failed read fails the whole call with no file written. Without `outFile`, `with_blockers` is refused.

### Lean mode: terse tool text

Each tool definition sends a description and a property description for every argument, and mcp-go adds annotations to every tool. `--lean` (or `FORGEJO_MCP_LEAN=1`; the flag wins over the variable) serves the same tools with terse text:

- each tool description is cut to its first sentence;
- a property description is kept only where the argument has a format, an allowed-value list, or a rule its name and type do not show (comma-separated lists, RFC3339 times, `state`, `sort`, and similar); the rest are removed. `verbose` keeps its argument and loses its description;
- annotations are sent as an empty object.

Tool names, argument names and required lists are unchanged, so a prompt or script written for the full text still works. Lean mode is a rewrite at registration (`internal/lean/`), not a second set of descriptions.

Measured by running the server over stdio on this build, for the whole catalogue of 167 tools:

| Text | Bytes of the `tools/list` response | ~Tokens |
|---|---:|---:|
| Full (default) | 127,243 | 31,811 |
| `--lean` | 59,033 | 14,758 |

The estimate is bytes divided by 4, as in the standing-cost table below.

