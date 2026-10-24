// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"encoding/json"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"

	"github.com/mark3labs/mcp-go/server"
)

// Annotation classes a tool can be pinned to. read tools are read-only and
// not destructive; write tools are neither; destructive tools are not
// read-only and may destroy data.
const (
	hintRead        = "read"
	hintWrite       = "write"
	hintDestructive = "destructive"
)

// expectedHints pins the annotation class of every tool. A tool added without
// an entry here, or an entry for a tool that is gone, fails
// TestToolAnnotationsPinEveryTool. The class follows the tool's verb: get,
// list, search, check and download are read; delete, remove and cancel are
// destructive; everything else is write.
var expectedHints = map[string]string{
	"add_issue_dependency":                hintWrite,
	"add_issue_labels":                    hintWrite,
	"add_issue_time":                      hintWrite,
	"add_repo_topic":                      hintWrite,
	"add_team_member":                     hintWrite,
	"add_team_repo":                       hintWrite,
	"cancel_issue_stopwatch":              hintDestructive,
	"cancel_workflow_run":                 hintDestructive,
	"change_files":                        hintWrite,
	"check_notifications":                 hintRead,
	"check_org_membership":                hintRead,
	"create_branch":                       hintWrite,
	"create_branch_protection":            hintWrite,
	"create_comment_attachment":           hintWrite,
	"create_file":                         hintWrite,
	"create_issue":                        hintWrite,
	"create_issue_attachment":             hintWrite,
	"create_issue_comment":                hintWrite,
	"create_milestone":                    hintWrite,
	"create_org":                          hintWrite,
	"create_org_label":                    hintWrite,
	"create_org_team":                     hintWrite,
	"create_or_update_org_action_secret":  hintWrite,
	"create_or_update_repo_action_secret": hintWrite,
	"create_pull_request":                 hintWrite,
	"create_pull_review":                  hintWrite,
	"create_release":                      hintWrite,
	"create_release_attachment":           hintWrite,
	"create_repo":                         hintWrite,
	"create_repo_hook":                    hintWrite,
	"create_repo_label":                   hintWrite,
	"create_review_requests":              hintWrite,
	"create_wiki_page":                    hintWrite,
	"delete_branch":                       hintDestructive,
	"delete_branch_protection":            hintDestructive,
	"delete_comment_attachment":           hintDestructive,
	"delete_file":                         hintDestructive,
	"delete_issue_attachment":             hintDestructive,
	"delete_issue_comment":                hintDestructive,
	"delete_issue_time_entry":             hintDestructive,
	"delete_org":                          hintDestructive,
	"delete_org_action_secret":            hintDestructive,
	"delete_org_label":                    hintDestructive,
	"delete_package":                      hintDestructive,
	"delete_pull_review":                  hintDestructive,
	"delete_release":                      hintDestructive,
	"delete_release_attachment":           hintDestructive,
	"delete_release_by_tag":               hintDestructive,
	"delete_repo_action_secret":           hintDestructive,
	"delete_repo_hook":                    hintDestructive,
	"delete_repo_label":                   hintDestructive,
	"delete_repo_topic":                   hintDestructive,
	"delete_review_requests":              hintDestructive,
	"delete_wiki_page":                    hintDestructive,
	"delete_workflow_run":                 hintDestructive,
	"dismiss_pull_review":                 hintWrite,
	"dispatch_workflow":                   hintWrite,
	"download_comment_attachment":         hintRead,
	"download_issue_attachment":           hintRead,
	"download_release_attachment":         hintRead,
	"edit_branch_protection":              hintWrite,
	"edit_comment_attachment":             hintWrite,
	"edit_issue_attachment":               hintWrite,
	"edit_issue_comment":                  hintWrite,
	"edit_milestone":                      hintWrite,
	"edit_org":                            hintWrite,
	"edit_org_label":                      hintWrite,
	"edit_release":                        hintWrite,
	"edit_release_attachment":             hintWrite,
	"edit_repo":                           hintWrite,
	"edit_repo_hook":                      hintWrite,
	"edit_repo_label":                     hintWrite,
	"fork_repo":                           hintWrite,
	"get_action_artifact":                 hintRead,
	"get_action_job_logs":                 hintRead,
	"get_branch_protection":               hintRead,
	"get_comment_attachment":              hintRead,
	"get_commit_statuses":                 hintRead,
	"get_file_content":                    hintRead,
	"get_forgejo_mcp_server_version":      hintRead,
	"get_issue_attachment":                hintRead,
	"get_issue_by_index":                  hintRead,
	"get_issue_comment":                   hintRead,
	"get_latest_release":                  hintRead,
	"get_milestone":                       hintRead,
	"get_my_user_info":                    hintRead,
	"get_notification_thread":             hintRead,
	"get_org":                             hintRead,
	"get_org_label":                       hintRead,
	"get_package":                         hintRead,
	"get_pull_request_by_index":           hintRead,
	"get_pull_request_diff":               hintRead,
	"get_pull_review":                     hintRead,
	"get_release_attachment":              hintRead,
	"get_release_by_id":                   hintRead,
	"get_release_by_tag":                  hintRead,
	"get_repo":                            hintRead,
	"get_repo_hook":                       hintRead,
	"get_repo_label":                      hintRead,
	"get_repo_tree":                       hintRead,
	"get_wiki_page":                       hintRead,
	"get_wiki_revisions":                  hintRead,
	"get_workflow_run":                    hintRead,
	"issue_state_change":                  hintWrite,
	"list_action_run_artifacts":           hintRead,
	"list_action_run_jobs":                hintRead,
	"list_branches":                       hintRead,
	"list_branch_protections":             hintRead,
	"list_comment_attachments":            hintRead,
	"list_issue_attachments":              hintRead,
	"list_issue_comments":                 hintRead,
	"list_issue_dependencies":             hintRead,
	"list_issue_dependents":               hintRead,
	"list_issue_tracked_times":            hintRead,
	"list_my_orgs":                        hintRead,
	"list_my_repos":                       hintRead,
	"list_my_stopwatches":                 hintRead,
	"list_my_tracked_times":               hintRead,
	"list_org_action_secrets":             hintRead,
	"list_org_labels":                     hintRead,
	"list_org_members":                    hintRead,
	"list_org_teams":                      hintRead,
	"list_package_files":                  hintRead,
	"list_packages":                       hintRead,
	"list_pull_request_files":             hintRead,
	"list_pull_review_comments":           hintRead,
	"list_pull_reviews":                   hintRead,
	"list_release_attachments":            hintRead,
	"list_releases":                       hintRead,
	"list_repo_action_secrets":            hintRead,
	"list_repo_commits":                   hintRead,
	"list_repo_contents":                  hintRead,
	"list_repo_hooks":                     hintRead,
	"list_repo_issues":                    hintRead,
	"list_repo_labels":                    hintRead,
	"list_repo_milestones":                hintRead,
	"list_repo_notifications":             hintRead,
	"list_repo_pull_requests":             hintRead,
	"list_repo_topics":                    hintRead,
	"list_repo_tracked_times":             hintRead,
	"list_user_orgs":                      hintRead,
	"list_wiki_pages":                     hintRead,
	"list_workflow_runs":                  hintRead,
	"mark_all_notifications_read":         hintWrite,
	"mark_notification_read":              hintWrite,
	"mark_repo_notifications_read":        hintWrite,
	"merge_pull_request":                  hintWrite,
	"remove_issue_dependency":             hintDestructive,
	"remove_issue_labels":                 hintDestructive,
	"remove_org_member":                   hintDestructive,
	"remove_team_member":                  hintDestructive,
	"remove_team_repo":                    hintDestructive,
	"replace_issue_body_line":             hintWrite,
	"reset_issue_time":                    hintWrite,
	"search_issues":                       hintRead,
	"search_org_teams":                    hintRead,
	"search_repos":                        hintRead,
	"search_users":                        hintRead,
	"set_repo_topics":                     hintWrite,
	"start_issue_stopwatch":               hintWrite,
	"stop_issue_stopwatch":                hintWrite,
	"submit_pull_review":                  hintWrite,
	"test_repo_hook":                      hintWrite,
	"update_file":                         hintWrite,
	"update_issue":                        hintWrite,
	"update_pull_request":                 hintWrite,
	"update_wiki_page":                    hintWrite,
}

// toolHints is the annotation pair a tools/list entry reports.
type toolHints struct {
	ReadOnly    *bool `json:"readOnlyHint"`
	Destructive *bool `json:"destructiveHint"`
}

// listedHints sends tools/list and returns each tool's reported hints.
func listedHints(t *testing.T, s *server.MCPServer) map[string]toolHints {
	t.Helper()
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
	raw, err := json.Marshal(s.HandleMessage(context.Background(), req))
	if err != nil {
		t.Fatalf("marshal tools/list response: %v", err)
	}
	var resp struct {
		Result struct {
			Tools []struct {
				Name        string    `json:"name"`
				Annotations toolHints `json:"annotations"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("decode tools/list response: %v\n%s", err, raw)
	}
	out := make(map[string]toolHints, len(resp.Result.Tools))
	for _, tool := range resp.Result.Tools {
		out[tool.Name] = tool.Annotations
	}
	return out
}

func TestToolAnnotationsPinEveryTool(t *testing.T) {
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, "test", fake.Factory{Forgejo: fake.New()})
	listed := listedHints(t, s)

	for name, class := range expectedHints {
		hints, ok := listed[name]
		if !ok {
			t.Errorf("expected tool %q is not in tools/list", name)
			continue
		}
		if hints.ReadOnly == nil || hints.Destructive == nil {
			t.Errorf("%s: readOnlyHint and destructiveHint must both be set, got %+v", name, hints)
			continue
		}
		wantReadOnly := class == hintRead
		wantDestructive := class == hintDestructive
		if *hints.ReadOnly != wantReadOnly {
			t.Errorf("%s: readOnlyHint = %v, want %v (class %s)", name, *hints.ReadOnly, wantReadOnly, class)
		}
		if *hints.Destructive != wantDestructive {
			t.Errorf("%s: destructiveHint = %v, want %v (class %s)", name, *hints.Destructive, wantDestructive, class)
		}
	}
	for name := range listed {
		if _, ok := expectedHints[name]; !ok {
			t.Errorf("tool %q has no annotation class in expectedHints; add one deliberately", name)
		}
	}
}
