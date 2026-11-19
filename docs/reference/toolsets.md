<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# Toolsets and presets

## Toolsets: choosing which tools load

Every tool a client sees costs context on every request. By default the server offers all of them. To offer fewer, name toolsets (groups of related tools), individual tools, or both:

| Setting | Flag | Environment variable |
|---|---|---|
| Toolsets to load | `--toolsets` | `FORGEJO_MCP_TOOLSETS` |
| Individual tools to load | `--tools` | `FORGEJO_MCP_TOOLS` |
| Named preset to load | `--preset` | `FORGEJO_MCP_PRESET` |

- With none set, every tool and resource template registers (the default, unchanged).
- With toolsets only, the tools of those toolsets register, and their resource templates too.
- With tools only, just the named tools register. No resource templates register.
- With a preset, its tools register. No resource templates register for it.
- With more than one of them, the result is the union. The flag wins over the environment variable.
- An unknown toolset, tool or preset name stops the server at startup, and the error lists the valid names.
- An unselected tool is absent from `tools/list`, not disabled. Selection applies to the server, not to `--cli` mode.

`--help` prints every toolset with the tools it holds. The table below is the same catalogue, with the standing cost of each toolset. Tools are assigned to toolsets in `internal/toolset/toolset.go`.

Example, the issue-tracking selection:

```bash
forgejo-mcp --toolsets issues,labels,milestones,dependencies,user
```

### Presets

A preset is a named selection of individual tools that may cross domains. It is not a toolset: each tool still belongs to exactly one toolset, and a preset registers no resource templates. `--help` prints every preset with the tools it holds.

`tracker-core` is the set an issue-tracking client needs, 24 tools across issues, labels, milestones and dependencies, plus `get_my_user_info`:

```bash
forgejo-mcp --preset tracker-core
```

| Preset | Tools |
|---|---|
| `tracker-core` | `get_my_user_info`; `get_issue_by_index`, `list_repo_issues`, `search_issues`, `create_issue`, `update_issue`, `replace_issue_body_line`, `issue_state_change`; `create_issue_comment`, `list_issue_comments`; `add_issue_labels`, `remove_issue_labels`, `list_repo_labels`, `get_repo_label`, `create_repo_label`, `edit_repo_label`; `list_repo_milestones`, `get_milestone`, `create_milestone`, `edit_milestone`; `add_issue_dependency`, `remove_issue_dependency`, `list_issue_dependencies`, `list_issue_dependents` |

The list was inferred from the tfuk-specflow ticket texts, not from its code, so confirm it before relying on it.

**Standing cost.** Estimated as bytes of the `tools/list` JSON-RPC response divided by 4, measured on this build with the fake forge. It is an estimate, not a model tokenizer count, and it is not the `/context` figure from a fresh session.

| Selection | Tools | Bytes | ~Tokens | Resource templates, ~tokens |
|---|---:|---:|---:|---:|
| Default (no selection) | 169 | 131,663 | 32,916 | 1,896 |
| `issues,labels,milestones,dependencies,user` | 39 | 34,452 | 8,613 | 813 |
| `--preset tracker-core` | 24 | 25,580 | 6,395 | 0 |
| `--preset tracker-core --lean` | 24 | 10,577 | 2,644 | 0 |

The selection above costs about 26% of the default. It misses the ticket target of well under 5,000 tokens. Most of the cost is tool descriptions and input schemas, about 760 to 820 bytes per tool, so shortening them is the next lever. That is a follow-up, not part of toolset selection.

The `tracker-core` rows are measured the same way, on this build with the fake forge. The lean row is the figure the budget test guards (`TestTrackerCoreLeanStaysWithinBudget`, budget 11,000 bytes, headroom 10,600). Against the design target of about 8 KB (about 2k tokens), the lean preset is about 2.6 KB over, roughly 32%. The largest remaining lean tools are `list_repo_issues` (887 bytes, the `outFile` text included), `search_issues` (854, `outFile` included), `update_issue` (817), `create_issue` (516) and `edit_repo_label` (507). Those are the next lever for shrinking the preset.

Per-toolset figures below are the same measure for a selection of that one toolset. They do not sum to the totals, because each response carries its own envelope.

### Toolsets and their tools

| Toolset | Tools | ~Tokens | Tool names |
|---|---:|---:|---|
| `user` | 7 | 857 | `check_notifications`, `get_my_user_info`, `get_notification_thread`, `list_repo_notifications`, `mark_all_notifications_read`, `mark_notification_read`, `mark_repo_notifications_read` |
| `repos` | 21 | 4522 | `add_repo_topic`, `change_files`, `create_branch`, `create_file`, `create_repo`, `delete_branch`, `delete_file`, `delete_repo_topic`, `edit_repo`, `fork_repo`, `get_commit_statuses`, `get_file_content`, `get_repo`, `get_repo_tree`, `list_branches`, `list_my_repos`, `list_repo_commits`, `list_repo_contents`, `list_repo_topics`, `set_repo_topics`, `update_file` |
| `issues` | 12 | 3583 | `create_issue`, `create_issue_comment`, `delete_issue_comment`, `edit_issue_comment`, `get_issue_by_index`, `get_issue_comment`, `issue_state_change`, `list_issue_comments`, `list_repo_issues`, `replace_issue_body_line`, `search_issues`, `update_issue` |
| `labels` | 12 | 2336 | `add_issue_labels`, `create_org_label`, `create_repo_label`, `delete_org_label`, `delete_repo_label`, `edit_org_label`, `edit_repo_label`, `get_org_label`, `get_repo_label`, `list_org_labels`, `list_repo_labels`, `remove_issue_labels` |
| `milestones` | 4 | 742 | `create_milestone`, `edit_milestone`, `get_milestone`, `list_repo_milestones` |
| `dependencies` | 4 | 982 | `add_issue_dependency`, `list_issue_dependencies`, `list_issue_dependents`, `remove_issue_dependency` |
| `pulls` | 16 | 2967 | `create_pull_request`, `create_pull_review`, `create_review_requests`, `delete_pull_review`, `delete_review_requests`, `dismiss_pull_review`, `get_pull_request_by_index`, `get_pull_request_diff`, `get_pull_review`, `list_pull_request_files`, `list_pull_review_comments`, `list_pull_reviews`, `list_repo_pull_requests`, `merge_pull_request`, `submit_pull_review`, `update_pull_request` |
| `search` | 3 | 506 | `search_org_teams`, `search_repos`, `search_users` |
| `orgs` | 15 | 1875 | `add_team_member`, `add_team_repo`, `check_org_membership`, `create_org`, `create_org_team`, `delete_org`, `edit_org`, `get_org`, `list_my_orgs`, `list_org_members`, `list_org_teams`, `list_user_orgs`, `remove_org_member`, `remove_team_member`, `remove_team_repo` |
| `packages` | 4 | 994 | `delete_package`, `get_package`, `list_package_files`, `list_packages` |
| `releases` | 14 | 2491 | `create_release`, `create_release_attachment`, `delete_release`, `delete_release_attachment`, `delete_release_by_tag`, `download_release_attachment`, `edit_release`, `edit_release_attachment`, `get_latest_release`, `get_release_attachment`, `get_release_by_id`, `get_release_by_tag`, `list_release_attachments`, `list_releases` |
| `actions` | 15 | 2813 | `cancel_workflow_run`, `create_or_update_org_action_secret`, `create_or_update_repo_action_secret`, `delete_org_action_secret`, `delete_repo_action_secret`, `delete_workflow_run`, `dispatch_workflow`, `get_action_artifact`, `get_action_job_logs`, `get_workflow_run`, `list_action_run_artifacts`, `list_action_run_jobs`, `list_org_action_secrets`, `list_repo_action_secrets`, `list_workflow_runs` |
| `attachments` | 12 | 2253 | `create_comment_attachment`, `create_issue_attachment`, `delete_comment_attachment`, `delete_issue_attachment`, `download_comment_attachment`, `download_issue_attachment`, `edit_comment_attachment`, `edit_issue_attachment`, `get_comment_attachment`, `get_issue_attachment`, `list_comment_attachments`, `list_issue_attachments` |
| `tracking` | 10 | 1694 | `add_issue_time`, `cancel_issue_stopwatch`, `delete_issue_time_entry`, `list_issue_tracked_times`, `list_my_stopwatches`, `list_my_tracked_times`, `list_repo_tracked_times`, `reset_issue_time`, `start_issue_stopwatch`, `stop_issue_stopwatch` |
| `branch-protection` | 5 | 1383 | `create_branch_protection`, `delete_branch_protection`, `edit_branch_protection`, `get_branch_protection`, `list_branch_protections` |
| `webhooks` | 6 | 1218 | `create_repo_hook`, `delete_repo_hook`, `edit_repo_hook`, `get_repo_hook`, `list_repo_hooks`, `test_repo_hook` |
| `wiki` | 7 | 1058 | `create_wiki_page`, `delete_wiki_page`, `get_wiki_page`, `get_wiki_revisions`, `list_wiki_pages`, `search_wiki`, `update_wiki_page` |
| `version` | 2 | 199 | `get_forgejo_mcp_server_version`, `get_forgejo_version` |


