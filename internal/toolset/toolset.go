// SPDX-License-Identifier: GPL-3.0-or-later

// Package toolset groups the MCP tools into named toolsets and resolves the
// operator's selection of them. It has no dependencies, so the config loader
// can validate a selection and print its help, and the operation package can
// register the chosen tools.
//
// To assign a new tool to a toolset, add its name to the Tools list of that
// toolset in catalogue below. The registry test fails the build for a tool
// that is in no toolset or in two.
package toolset

import (
	"fmt"
	"sort"
	"strings"
)

// Toolset is a named group of tools that an operator can load together.
type Toolset struct {
	Name  string
	Tools []string
}

// catalogue lists every toolset in help order. Each registered tool belongs to
// exactly one of them. Resource templates follow the toolset of their domain
// (see operation.RegisterSelectedResources).
var catalogue = []Toolset{
	{
		Name: "user",
		Tools: []string{
			"check_notifications",
			"get_my_user_info",
			"get_notification_thread",
			"list_repo_notifications",
			"mark_all_notifications_read",
			"mark_notification_read",
			"mark_repo_notifications_read",
		},
	},
	{
		Name: "repos",
		Tools: []string{
			"add_repo_topic",
			"change_files",
			"create_branch",
			"create_file",
			"create_repo",
			"delete_branch",
			"delete_file",
			"delete_repo_topic",
			"edit_repo",
			"fork_repo",
			"get_commit_statuses",
			"get_file_content",
			"get_repo",
			"get_repo_tree",
			"list_branches",
			"list_my_repos",
			"list_repo_commits",
			"list_repo_contents",
			"list_repo_topics",
			"set_repo_topics",
			"update_file",
		},
	},
	{
		Name: "issues",
		Tools: []string{
			"create_issue",
			"create_issue_comment",
			"delete_issue_comment",
			"edit_issue_comment",
			"get_issue_by_index",
			"get_issue_comment",
			"issue_state_change",
			"list_issue_comments",
			"list_repo_issues",
			"replace_issue_body_line",
			"search_issues",
			"update_issue",
		},
	},
	{
		Name: "labels",
		Tools: []string{
			"add_issue_labels",
			"create_org_label",
			"create_repo_label",
			"delete_org_label",
			"delete_repo_label",
			"edit_org_label",
			"edit_repo_label",
			"get_org_label",
			"get_repo_label",
			"list_org_labels",
			"list_repo_labels",
			"remove_issue_labels",
		},
	},
	{
		Name: "milestones",
		Tools: []string{
			"create_milestone",
			"edit_milestone",
			"get_milestone",
			"list_repo_milestones",
		},
	},
	{
		Name: "dependencies",
		Tools: []string{
			"add_issue_dependency",
			"list_issue_dependencies",
			"list_issue_dependents",
			"remove_issue_dependency",
		},
	},
	{
		Name: "pulls",
		Tools: []string{
			"create_pull_request",
			"create_pull_review",
			"create_review_requests",
			"delete_pull_review",
			"delete_review_requests",
			"dismiss_pull_review",
			"get_pull_request_by_index",
			"get_pull_request_diff",
			"get_pull_review",
			"list_pull_request_files",
			"list_pull_review_comments",
			"list_pull_reviews",
			"list_repo_pull_requests",
			"merge_pull_request",
			"submit_pull_review",
			"update_pull_request",
		},
	},
	{
		Name: "search",
		Tools: []string{
			"search_org_teams",
			"search_repos",
			"search_users",
		},
	},
	{
		Name: "orgs",
		Tools: []string{
			"add_team_member",
			"add_team_repo",
			"check_org_membership",
			"create_org",
			"create_org_team",
			"delete_org",
			"edit_org",
			"get_org",
			"list_my_orgs",
			"list_org_members",
			"list_org_teams",
			"list_user_orgs",
			"remove_org_member",
			"remove_team_member",
			"remove_team_repo",
		},
	},
	{
		Name: "packages",
		Tools: []string{
			"delete_package",
			"get_package",
			"list_package_files",
			"list_packages",
		},
	},
	{
		Name: "releases",
		Tools: []string{
			"create_release",
			"create_release_attachment",
			"delete_release",
			"delete_release_attachment",
			"delete_release_by_tag",
			"download_release_attachment",
			"edit_release",
			"edit_release_attachment",
			"get_latest_release",
			"get_release_attachment",
			"get_release_by_id",
			"get_release_by_tag",
			"list_release_attachments",
			"list_releases",
		},
	},
	{
		Name: "actions",
		Tools: []string{
			"cancel_workflow_run",
			"create_or_update_org_action_secret",
			"create_or_update_repo_action_secret",
			"delete_org_action_secret",
			"delete_repo_action_secret",
			"delete_workflow_run",
			"dispatch_workflow",
			"get_action_artifact",
			"get_action_job_logs",
			"get_workflow_run",
			"list_action_run_artifacts",
			"list_action_run_jobs",
			"list_org_action_secrets",
			"list_repo_action_secrets",
			"list_workflow_runs",
		},
	},
	{
		Name: "attachments",
		Tools: []string{
			"create_comment_attachment",
			"create_issue_attachment",
			"delete_comment_attachment",
			"delete_issue_attachment",
			"download_comment_attachment",
			"download_issue_attachment",
			"edit_comment_attachment",
			"edit_issue_attachment",
			"get_comment_attachment",
			"get_issue_attachment",
			"list_comment_attachments",
			"list_issue_attachments",
		},
	},
	{
		Name: "tracking",
		Tools: []string{
			"add_issue_time",
			"cancel_issue_stopwatch",
			"delete_issue_time_entry",
			"list_issue_tracked_times",
			"list_my_stopwatches",
			"list_my_tracked_times",
			"list_repo_tracked_times",
			"reset_issue_time",
			"start_issue_stopwatch",
			"stop_issue_stopwatch",
		},
	},
	{
		Name: "branch-protection",
		Tools: []string{
			"create_branch_protection",
			"delete_branch_protection",
			"edit_branch_protection",
			"get_branch_protection",
			"list_branch_protections",
		},
	},
	{
		Name: "webhooks",
		Tools: []string{
			"create_repo_hook",
			"delete_repo_hook",
			"edit_repo_hook",
			"get_repo_hook",
			"list_repo_hooks",
			"test_repo_hook",
		},
	},
	{
		Name: "wiki",
		Tools: []string{
			"create_wiki_page",
			"delete_wiki_page",
			"get_wiki_page",
			"get_wiki_revisions",
			"list_wiki_pages",
			"update_wiki_page",
		},
	},
	{
		Name: "version",
		Tools: []string{
			"get_forgejo_mcp_server_version",
		},
	},
}

// All returns every toolset in help order. The result is a copy.
func All() []Toolset {
	out := make([]Toolset, len(catalogue))
	for i, ts := range catalogue {
		out[i] = Toolset{Name: ts.Name, Tools: append([]string(nil), ts.Tools...)}
	}
	return out
}

// Of returns the toolset that holds the named tool, or "" when none does.
func Of(tool string) string {
	for _, ts := range catalogue {
		for _, t := range ts.Tools {
			if t == tool {
				return ts.Name
			}
		}
	}
	return ""
}

// Selection is the set of tools and resource templates an operator chose.
type Selection struct {
	full     bool
	toolsets map[string]bool
	tools    map[string]bool
}

// Everything is the selection with nothing chosen: every tool and resource.
func Everything() Selection {
	return Selection{full: true}
}

// Parse resolves the operator's toolset and tool names into a Selection. It is
// ParseWithPreset with no preset.
func Parse(toolsets, tools []string) (Selection, error) {
	return ParseWithPreset("", toolsets, tools)
}

// ParseWithPreset resolves the operator's preset, toolset and tool names into a
// Selection. With none given it selects everything. Otherwise it selects the
// union of the preset's tools, the tools of the named toolsets and the named
// tools. Resource templates follow the named toolsets only, so a preset or a
// tools-only selection registers none. An unknown name is an error that lists
// the valid names.
func ParseWithPreset(preset string, toolsets, tools []string) (Selection, error) {
	if preset == "" && len(toolsets) == 0 && len(tools) == 0 {
		return Everything(), nil
	}
	sel := Selection{toolsets: map[string]bool{}, tools: map[string]bool{}}
	if preset != "" {
		presetTools, ok := PresetTools(preset)
		if !ok {
			return Selection{}, fmt.Errorf("unknown preset %q; valid presets: %s", preset, strings.Join(PresetNames(), ", "))
		}
		for _, t := range presetTools {
			sel.tools[t] = true
		}
	}
	byName := make(map[string]Toolset, len(catalogue))
	for _, ts := range catalogue {
		byName[ts.Name] = ts
	}
	for _, name := range toolsets {
		ts, ok := byName[name]
		if !ok {
			return Selection{}, fmt.Errorf("unknown toolset %q; valid toolsets: %s", name, strings.Join(Names(), ", "))
		}
		sel.toolsets[name] = true
		for _, t := range ts.Tools {
			sel.tools[t] = true
		}
	}
	known := make(map[string]bool)
	for _, t := range ToolNames() {
		known[t] = true
	}
	for _, name := range tools {
		if !known[name] {
			return Selection{}, fmt.Errorf("unknown tool %q; valid tools: %s", name, strings.Join(ToolNames(), ", "))
		}
		sel.tools[name] = true
	}
	return sel, nil
}

// Full reports whether the selection is the default: everything registers.
func (s Selection) Full() bool { return s.full }

// HasTool reports whether the selection holds the named tool.
func (s Selection) HasTool(name string) bool { return s.full || s.tools[name] }

// HasToolset reports whether the resource templates of the named toolset
// register. A tools-only selection holds no toolset, so it has none.
func (s Selection) HasToolset(name string) bool { return s.full || s.toolsets[name] }

// Names returns the toolset names in help order.
func Names() []string {
	out := make([]string, len(catalogue))
	for i, ts := range catalogue {
		out[i] = ts.Name
	}
	return out
}

// ToolNames returns every tool name in the catalogue, sorted.
func ToolNames() []string {
	var out []string
	for _, ts := range catalogue {
		out = append(out, ts.Tools...)
	}
	sort.Strings(out)
	return out
}

// FlagHelp is the help text for the toolsets flag: every toolset and the tools
// it holds, one toolset per line.
func FlagHelp() string {
	var b strings.Builder
	b.WriteString("Comma-separated toolsets to load. Default: every tool. Toolsets and the tools each holds:")
	for _, ts := range catalogue {
		fmt.Fprintf(&b, "\n  %s: %s", ts.Name, strings.Join(ts.Tools, ", "))
	}
	return b.String()
}
