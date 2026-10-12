// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcptest"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// These tests pin the wire contract of the repository-administration tools
// (repositories, edit, topics and commit statuses): each tool's definition
// (schema snapshot) and its output and error text (golden), with the calls the
// tool made on the Forgejo port. They run against a stub port with fixed,
// fully populated data, so the bytes are deterministic.

var adminTools = []mcp.Tool{
	CreateRepoTool, ForkRepoTool, ListMyReposTool, GetRepoTool, EditRepoTool,
	ListRepoTopicsTool, SetRepoTopicsTool, AddRepoTopicTool, DeleteRepoTopicTool,
	GetCommitStatusesTool,
}

func TestAdminSchemaSnapshots(t *testing.T) {
	t.Parallel()
	for _, tool := range adminTools {
		t.Run(tool.Name, func(t *testing.T) { mcptest.SchemaSnapshot(t, tool) })
	}
}

var (
	adminT0    = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	adminT1    = time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	adminSHA   = "0123456789abcdef0123456789abcdef01234567"
	adminUser  = &forgejo_sdk.User{ID: 1, UserName: "octo", FullName: "Octo Cat", Email: "octo@example.com", Created: adminT0, LastLogin: adminT1, IsActive: true}
	adminTopic = []string{"go", "mcp"}
)

func adminRepo(name string) *forgejo_sdk.Repository {
	return &forgejo_sdk.Repository{
		ID: 9, Owner: adminUser, Name: name, FullName: "octo/" + name, Description: "desc",
		Private: true, Size: 42, HTMLURL: "https://f.example/octo/" + name,
		SSHURL:   "git@f.example:octo/" + name + ".git",
		CloneURL: "https://f.example/octo/" + name + ".git",
		Website:  "https://example.com", Stars: 1, Forks: 2, Watchers: 3, OpenIssues: 4,
		OpenPulls: 5, Releases: 6, DefaultBranch: "main", Created: adminT0, Updated: adminT1,
		Permissions:       &forgejo_sdk.Permission{Admin: true, Push: true, Pull: true},
		HasIssues:         true,
		InternalTracker:   &forgejo_sdk.InternalTracker{EnableTimeTracker: true},
		HasWiki:           true,
		HasPullRequests:   true,
		HasProjects:       true,
		HasReleases:       true,
		HasActions:        true,
		AllowMerge:        true,
		AllowRebase:       true,
		AllowRebaseMerge:  true,
		AllowSquash:       true,
		DefaultMergeStyle: "merge",
	}
}

func adminStatuses() forge.CommitStatuses {
	total := 3
	return forge.CommitStatuses{
		Statuses: []*forgejo_sdk.Status{
			{ID: 1, State: forgejo_sdk.StatusSuccess, TargetURL: "https://ci/1", Description: "ok", Context: "build", URL: "u", Creator: adminUser, Created: adminT0, Updated: adminT1},
			nil,
			{ID: 2, State: forgejo_sdk.StatusFailure, Context: "lint", Created: adminT1},
		},
		TotalCount: &total,
	}
}

// stubAdmin is a forge.Forgejo whose repo-administration and commit-status
// methods return fixed data and record the calls they receive. Any other
// domain would panic on the nil embed.
type stubAdmin struct {
	forge.Forgejo
	mu        sync.Mutex
	calls     []string
	fail      error
	topics    []string
	topicsErr error
	statuses  forge.CommitStatuses
}

func (s *stubAdmin) rec(name string, args ...any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(args)
	s.calls = append(s.calls, name+" "+string(b))
	return s.fail
}

func (s *stubAdmin) CreateRepo(_ context.Context, opt forgejo_sdk.CreateRepoOption) (*forgejo_sdk.Repository, error) {
	return adminRepo(opt.Name), s.rec("CreateRepo", opt)
}

func (s *stubAdmin) CreateOrgRepo(_ context.Context, org string, opt forgejo_sdk.CreateRepoOption) (*forgejo_sdk.Repository, error) {
	return adminRepo(opt.Name), s.rec("CreateOrgRepo", org, opt)
}

func (s *stubAdmin) ForkRepo(_ context.Context, o, r string, opt forgejo_sdk.CreateForkOption) (*forgejo_sdk.Repository, error) {
	return adminRepo(r), s.rec("ForkRepo", o, r, opt)
}

func (s *stubAdmin) ListMyRepos(_ context.Context, opt forgejo_sdk.ListReposOptions) ([]*forgejo_sdk.Repository, error) {
	return []*forgejo_sdk.Repository{adminRepo("one"), adminRepo("two")}, s.rec("ListMyRepos", opt)
}

func (s *stubAdmin) GetRepo(_ context.Context, o, r string) (*forgejo_sdk.Repository, error) {
	return adminRepo(r), s.rec("GetRepo", o, r)
}

func (s *stubAdmin) EditRepo(_ context.Context, o, r string, opt forgejo_sdk.EditRepoOption) (*forgejo_sdk.Repository, error) {
	return adminRepo(r), s.rec("EditRepo", o, r, opt)
}

func (s *stubAdmin) ListRepoTopics(_ context.Context, o, r string, opt forgejo_sdk.ListRepoTopicsOptions) ([]string, error) {
	if err := s.rec("ListRepoTopics", o, r, opt); err != nil {
		return nil, err
	}
	return s.topics, s.topicsErr
}

func (s *stubAdmin) SetRepoTopics(_ context.Context, o, r string, topics []string) error {
	return s.rec("SetRepoTopics", o, r, topics)
}

func (s *stubAdmin) AddRepoTopic(_ context.Context, o, r, topic string) error {
	return s.rec("AddRepoTopic", o, r, topic)
}

func (s *stubAdmin) DeleteRepoTopic(_ context.Context, o, r, topic string) error {
	return s.rec("DeleteRepoTopic", o, r, topic)
}

func (s *stubAdmin) ListCommitStatuses(_ context.Context, o, r, sha string, opt forgejo_sdk.ListStatusesOption) (forge.CommitStatuses, error) {
	return s.statuses, s.rec("ListCommitStatuses", o, r, sha, opt)
}

// adminInvoke calls a registered tool the way the server does and renders what
// the client would see: the text, or the error, then the port calls made.
func adminInvoke(t *testing.T, stub *stubAdmin, tool string, args map[string]any) string {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Forgejo: stub})
	st := s.GetTool(tool)
	if st == nil {
		t.Fatalf("tool %s not registered", tool)
	}
	res, err := st.Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: tool, Arguments: args}})
	var out string
	switch {
	case err != nil:
		out = "ERROR: " + err.Error()
	case res.IsError:
		out = "RESULT-ERROR: " + res.Content[0].(mcp.TextContent).Text
	default:
		out = res.Content[0].(mcp.TextContent).Text
	}
	return fmt.Sprintf("%s\n-- port calls --\n%s\n", out, strings.Join(stub.calls, "\n"))
}

type adminCase struct {
	name string
	tool string
	args map[string]any
	stub func(*stubAdmin)
}

func withFail(s *stubAdmin) { s.fail = errors.New("boom") }
func withTopicsNotFound(s *stubAdmin) {
	s.topics = nil
	s.topicsErr = &forge.StatusError{StatusCode: 404, Err: errors.New("not found")}
}

func repoArgs(extra map[string]any) map[string]any {
	m := map[string]any{"owner": "octo", "repo": "hello"}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

var adminCases = []adminCase{
	// create_repo
	{name: "admin_create_repo", tool: CreateRepoToolName, args: map[string]any{"name": "fresh", "description": "d", "private": true, "auto_init": true, "template": true, "issue_labels": "Bug", "gitignores": "Go", "license": "MIT", "readme": "Default", "default_branch": "main"}},
	{name: "admin_create_repo_org", tool: CreateRepoToolName, args: map[string]any{"name": "fresh", "owner": "acme"}},
	{name: "admin_create_repo_empty_owner", tool: CreateRepoToolName, args: map[string]any{"name": "fresh", "owner": ""}},
	{name: "admin_create_repo_missing_name", tool: CreateRepoToolName, args: map[string]any{"description": "d"}},
	{name: "admin_create_repo_numeric_name", tool: CreateRepoToolName, args: map[string]any{"name": float64(5)}},
	{name: "admin_create_repo_port_error", tool: CreateRepoToolName, args: map[string]any{"name": "fresh"}, stub: withFail},

	// fork_repo
	{name: "admin_fork_repo", tool: ForkRepoToolName, args: map[string]any{"user": "octo", "repo": "hello", "organization": "acme", "name": "mine"}},
	{name: "admin_fork_repo_empty_optionals", tool: ForkRepoToolName, args: map[string]any{"user": "octo", "repo": "hello", "organization": "", "name": ""}},
	{name: "admin_fork_repo_missing_user", tool: ForkRepoToolName, args: map[string]any{"repo": "hello"}},
	{name: "admin_fork_repo_missing_repo", tool: ForkRepoToolName, args: map[string]any{"user": "octo"}},
	{name: "admin_fork_repo_port_error", tool: ForkRepoToolName, args: map[string]any{"user": "octo", "repo": "hello"}, stub: withFail},

	// list_my_repos
	{name: "admin_list_my_repos_defaults", tool: ListMyReposToolName},
	{name: "admin_list_my_repos_paged", tool: ListMyReposToolName, args: map[string]any{"page": float64(2), "limit": float64(5)}},
	{name: "admin_list_my_repos_fractional", tool: ListMyReposToolName, args: map[string]any{"page": 2.9, "limit": 3.5}},
	{name: "admin_list_my_repos_string_numbers", tool: ListMyReposToolName, args: map[string]any{"page": "2", "limit": "5"}},
	{name: "admin_list_my_repos_port_error", tool: ListMyReposToolName, stub: withFail},

	// get_repo
	{name: "admin_get_repo", tool: GetRepoToolName, args: repoArgs(nil)},
	{name: "admin_get_repo_missing_owner", tool: GetRepoToolName, args: map[string]any{"repo": "hello"}},
	{name: "admin_get_repo_missing_repo", tool: GetRepoToolName, args: map[string]any{"owner": "octo"}},
	{name: "admin_get_repo_no_topics", tool: GetRepoToolName, args: repoArgs(nil), stub: withTopicsNotFound},
	{name: "admin_get_repo_topics_error", tool: GetRepoToolName, args: repoArgs(nil), stub: func(s *stubAdmin) { s.topicsErr = errors.New("boom") }},
	{name: "admin_get_repo_port_error", tool: GetRepoToolName, args: repoArgs(nil), stub: withFail},

	// edit_repo
	{name: "admin_edit_repo_description_only", tool: EditRepoToolName, args: repoArgs(map[string]any{"description": "only desc"})},
	{name: "admin_edit_repo_all_fields", tool: EditRepoToolName, args: repoArgs(map[string]any{"name": "renamed", "description": "d", "website": "w", "default_branch": "dev", "private": false, "template": true, "archived": true, "has_issues": false, "has_wiki": true, "has_pull_requests": false, "has_projects": true, "has_releases": false, "has_packages": true, "has_actions": false})},
	{name: "admin_edit_repo_empty_strings", tool: EditRepoToolName, args: repoArgs(map[string]any{"name": "", "description": "", "website": "", "default_branch": ""})},
	{name: "admin_edit_repo_only_empty_name", tool: EditRepoToolName, args: repoArgs(map[string]any{"name": ""})},
	{name: "admin_edit_repo_no_fields", tool: EditRepoToolName, args: repoArgs(nil)},
	{name: "admin_edit_repo_string_bool", tool: EditRepoToolName, args: repoArgs(map[string]any{"private": "true"})},
	{name: "admin_edit_repo_missing_repo", tool: EditRepoToolName, args: map[string]any{"owner": "octo", "archived": true}},
	{name: "admin_edit_repo_port_error", tool: EditRepoToolName, args: repoArgs(map[string]any{"archived": true}), stub: withFail},

	// list_repo_topics
	{name: "admin_list_repo_topics_defaults", tool: ListRepoTopicsToolName, args: repoArgs(nil), stub: func(s *stubAdmin) { s.topics = adminTopic }},
	{name: "admin_list_repo_topics_paged", tool: ListRepoTopicsToolName, args: repoArgs(map[string]any{"page": float64(3), "limit": float64(2)}), stub: func(s *stubAdmin) { s.topics = adminTopic }},
	{name: "admin_list_repo_topics_fractional", tool: ListRepoTopicsToolName, args: repoArgs(map[string]any{"page": 1.5, "limit": 2.5}), stub: func(s *stubAdmin) { s.topics = adminTopic }},
	{name: "admin_list_repo_topics_none", tool: ListRepoTopicsToolName, args: repoArgs(nil)},
	{name: "admin_list_repo_topics_port_error", tool: ListRepoTopicsToolName, args: repoArgs(nil), stub: withFail},

	// set_repo_topics
	{name: "admin_set_repo_topics", tool: SetRepoTopicsToolName, args: repoArgs(map[string]any{"topics": "go, MCP, go,, ci"})},
	{name: "admin_set_repo_topics_empty", tool: SetRepoTopicsToolName, args: repoArgs(map[string]any{"topics": ""})},
	{name: "admin_set_repo_topics_missing_key", tool: SetRepoTopicsToolName, args: repoArgs(nil)},
	{name: "admin_set_repo_topics_too_many", tool: SetRepoTopicsToolName, args: repoArgs(map[string]any{"topics": strings.Join(numberedTopics(26), ",")})},
	{name: "admin_set_repo_topics_invalid", tool: SetRepoTopicsToolName, args: repoArgs(map[string]any{"topics": "ok,Bad Name"})},
	{name: "admin_set_repo_topics_too_long", tool: SetRepoTopicsToolName, args: repoArgs(map[string]any{"topics": strings.Repeat("a", 36)})},
	{name: "admin_set_repo_topics_missing_owner", tool: SetRepoTopicsToolName, args: map[string]any{"repo": "hello", "topics": "go"}},
	{name: "admin_set_repo_topics_port_error", tool: SetRepoTopicsToolName, args: repoArgs(map[string]any{"topics": "go"}), stub: withFail},

	// add_repo_topic and delete_repo_topic
	{name: "admin_add_repo_topic", tool: AddRepoTopicToolName, args: repoArgs(map[string]any{"topic": "  CI  "})},
	{name: "admin_add_repo_topic_empty", tool: AddRepoTopicToolName, args: repoArgs(map[string]any{"topic": ""})},
	{name: "admin_add_repo_topic_invalid", tool: AddRepoTopicToolName, args: repoArgs(map[string]any{"topic": "Bad Name"})},
	{name: "admin_add_repo_topic_port_error", tool: AddRepoTopicToolName, args: repoArgs(map[string]any{"topic": "ci"}), stub: withFail},
	{name: "admin_delete_repo_topic", tool: DeleteRepoTopicToolName, args: repoArgs(map[string]any{"topic": "CI"})},
	{name: "admin_delete_repo_topic_invalid", tool: DeleteRepoTopicToolName, args: repoArgs(map[string]any{"topic": "-x"})},
	{name: "admin_delete_repo_topic_port_error", tool: DeleteRepoTopicToolName, args: repoArgs(map[string]any{"topic": "ci"}), stub: withFail},

	// get_commit_statuses
	{name: "admin_statuses", tool: GetCommitStatusesToolName, args: repoArgs(map[string]any{"sha": adminSHA}), stub: func(s *stubAdmin) { s.statuses = adminStatuses() }},
	{name: "admin_statuses_paged", tool: GetCommitStatusesToolName, args: repoArgs(map[string]any{"sha": adminSHA, "page": float64(2), "limit": float64(2)}), stub: func(s *stubAdmin) { s.statuses = adminStatuses() }},
	{name: "admin_statuses_limit_clamped", tool: GetCommitStatusesToolName, args: repoArgs(map[string]any{"sha": adminSHA, "limit": float64(999)}), stub: func(s *stubAdmin) { s.statuses = adminStatuses() }},
	{name: "admin_statuses_zero_paging", tool: GetCommitStatusesToolName, args: repoArgs(map[string]any{"sha": adminSHA, "page": float64(0), "limit": float64(0)}), stub: func(s *stubAdmin) { s.statuses = adminStatuses() }},
	{name: "admin_statuses_fractional_limit", tool: GetCommitStatusesToolName, args: repoArgs(map[string]any{"sha": adminSHA, "limit": 2.5}), stub: func(s *stubAdmin) { s.statuses = adminStatuses() }},
	{name: "admin_statuses_no_total", tool: GetCommitStatusesToolName, args: repoArgs(map[string]any{"sha": adminSHA}), stub: func(s *stubAdmin) {
		s.statuses = adminStatuses()
		s.statuses.TotalCount = nil
	}},
	{name: "admin_statuses_empty", tool: GetCommitStatusesToolName, args: repoArgs(map[string]any{"sha": adminSHA})},
	{name: "admin_statuses_short_sha", tool: GetCommitStatusesToolName, args: repoArgs(map[string]any{"sha": "abc123"})},
	{name: "admin_statuses_bad_char_sha", tool: GetCommitStatusesToolName, args: repoArgs(map[string]any{"sha": strings.Repeat("g", 40)})},
	{name: "admin_statuses_missing_owner", tool: GetCommitStatusesToolName, args: map[string]any{"repo": "hello", "sha": adminSHA}},
	{name: "admin_statuses_port_error", tool: GetCommitStatusesToolName, args: repoArgs(map[string]any{"sha": adminSHA}), stub: withFail},
}

func numberedTopics(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("t%d", i)
	}
	return out
}

func TestAdminGolden(t *testing.T) {
	t.Parallel()
	for _, c := range adminCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubAdmin{}
			if c.stub != nil {
				c.stub(stub)
			}
			mcptest.Golden(t, c.name, []byte(adminInvoke(t, stub, c.tool, c.args)))
		})
	}
}

// MaxResponseItems reports an unknown ceiling: the repository admin goldens pin
// the paging contract without a ceiling, as they always have.
func (s *stubAdmin) MaxResponseItems(context.Context) (int, bool) { return 0, false }
