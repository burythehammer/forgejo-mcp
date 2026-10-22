// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcptest"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// These tests pin the wire contract of every issue tool: its tool definition
// (schema snapshot) and its output and error text (golden), together with the
// calls it made on the Forgejo port. They run against a stub port that returns
// fixed, fully populated data, so the bytes are deterministic.

var goldenAllTools = []mcp.Tool{
	GetIssueByIndexTool, ListRepoIssuesTool, CreateIssueTool, CreateIssueCommentTool,
	UpdateIssueTool, AddIssueLabelsTools, RemoveIssueLabelsTools, IssueStateChangeTool,
	ListIssueCommentsTool, GetIssueCommentTool, EditIssueCommentTool, DeleteIssueCommentTool,
	ListRepoMilestonesTool, CreateMilestoneTool, EditMilestoneTool, GetMilestoneTool,
	ListRepoLabelsTool, ListOrgLabelsTool, SearchIssuesTool,
	CreateRepoLabelTool, EditRepoLabelTool, DeleteRepoLabelTool, GetRepoLabelTool,
	CreateOrgLabelTool, EditOrgLabelTool, DeleteOrgLabelTool, GetOrgLabelTool,
	ListIssueDependenciesTool, ListIssueDependentsTool, AddIssueDependencyTool, RemoveIssueDependencyTool,
}

func TestSchemaSnapshots(t *testing.T) {
	t.Parallel()
	for _, tool := range goldenAllTools {
		t.Run(tool.Name, func(t *testing.T) { mcptest.SchemaSnapshot(t, tool) })
	}
}

// TestMilestoneArgsMatchSchemas keeps the milestone tools' argument structs in
// step with their schemas.
func TestMilestoneArgsMatchSchemas(t *testing.T) {
	t.Parallel()
	mcptest.ArgsMatchSchema(t, CreateMilestoneTool, createMilestoneArgs{})
	mcptest.ArgsMatchSchema(t, EditMilestoneTool, editMilestoneArgs{})
	mcptest.ArgsMatchSchema(t, GetMilestoneTool, getMilestoneArgs{})
}

// TestCompactWriteArgsMatchSchemas keeps the write tools' argument structs,
// including the optional verbose flag, in step with their schemas.
func TestCompactWriteArgsMatchSchemas(t *testing.T) {
	t.Parallel()
	mcptest.ArgsMatchSchema(t, GetIssueByIndexTool, getIssueArgs{})
	mcptest.ArgsMatchSchema(t, ListRepoIssuesTool, listIssuesArgs{})
	mcptest.ArgsMatchSchema(t, SearchIssuesTool, searchArgs{})
	mcptest.ArgsMatchSchema(t, CreateIssueTool, createIssueArgs{})
	mcptest.ArgsMatchSchema(t, UpdateIssueTool, updateIssueArgs{})
	mcptest.ArgsMatchSchema(t, IssueStateChangeTool, stateArgs{})
	mcptest.ArgsMatchSchema(t, CreateIssueCommentTool, createCommentArgs{})
	mcptest.ArgsMatchSchema(t, EditIssueCommentTool, editCommentArgs{})
	mcptest.ArgsMatchSchema(t, GetIssueCommentTool, getCommentArgs{})
	mcptest.ArgsMatchSchema(t, AddIssueLabelsTools, labelsArgs{})
	mcptest.ArgsMatchSchema(t, RemoveIssueLabelsTools, labelsArgs{})
}

var (
	gT0  = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	gT1  = time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	gUsr = &forgejo_sdk.User{ID: 1, UserName: "octo", FullName: "Octo Cat", Email: "octo@example.com", Created: gT0, LastLogin: gT1, IsActive: true}
)

func goldenIssue(title string) *forgejo_sdk.Issue {
	return &forgejo_sdk.Issue{
		ID: 11, URL: "https://f.example/api/v1/repos/octo/hello/issues/7", HTMLURL: "https://f.example/octo/hello/issues/7",
		Index: 7, Poster: gUsr, OriginalAuthor: "ghost", OriginalAuthorID: 5, Title: title, Body: "body", Ref: "refs/heads/feat",
		Labels:    []*forgejo_sdk.Label{{ID: 3, Name: "bug", Color: "ff0000", Description: "broken", URL: "https://f.example/l/3"}},
		Milestone: &forgejo_sdk.Milestone{ID: 4, Title: "v1", Created: gT0},
		Assignees: []*forgejo_sdk.User{gUsr}, State: forgejo_sdk.StateOpen, IsLocked: true, Comments: 2,
		Created: gT0, Updated: gT1, Closed: &gT1, Deadline: &gT1,
		PullRequest: &forgejo_sdk.PullRequestMeta{HasMerged: true, Merged: &gT1},
		Repository:  &forgejo_sdk.RepositoryMeta{ID: 9, Name: "hello", Owner: "octo", FullName: "octo/hello"},
	}
}

func goldenComment(body string) *forgejo_sdk.Comment {
	return &forgejo_sdk.Comment{
		ID: 31, HTMLURL: "https://f.example/c/31", PRURL: "https://f.example/p/7", IssueURL: "https://f.example/i/7",
		Poster: gUsr, OriginalAuthor: "ghost", OriginalAuthorID: 5, Body: body, Created: gT0, Updated: gT1,
	}
}

// goldenStub is a forge.Forgejo whose issue, label and dependency domains
// return fixed data and record each call. failOn makes the named method
// return the given error; info is what the list calls report about their
// rows. Any other domain would panic on the nil embed.
type goldenStub struct {
	forge.Forgejo
	mu    sync.Mutex
	calls []string
	// failOn maps a port method name to the error it returns.
	failOn map[string]error
	// info is the PageInfo that ListRepoIssues and SearchIssues report.
	info forge.PageInfo
	// ceiling and ceilingKnown are the instance's pagination ceiling.
	ceiling      int
	ceilingKnown bool
}

func newGoldenStub() *goldenStub {
	return &goldenStub{failOn: map[string]error{}, ceiling: 50, ceilingKnown: true}
}

func (s *goldenStub) rec(name string, args ...any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(args)
	s.calls = append(s.calls, name+" "+string(b))
	return s.failOn[name]
}

func goldenStatus(code int) error {
	return &forge.StatusError{StatusCode: code, Err: errors.New("status")}
}

func (s *goldenStub) MaxResponseItems(context.Context) (int, bool) {
	return s.ceiling, s.ceilingKnown
}

func (s *goldenStub) GetIssue(_ context.Context, o, r string, i int64) (*forgejo_sdk.Issue, error) {
	return goldenIssue("fixed"), s.rec("GetIssue", o, r, i)
}

func (s *goldenStub) ListRepoIssues(_ context.Context, o, r string, opt forge.ListIssuesOptions) ([]*forgejo_sdk.Issue, forge.PageInfo, error) {
	return []*forgejo_sdk.Issue{goldenIssue("one"), goldenIssue("two")}, s.info, s.rec("ListRepoIssues", o, r, opt)
}

func (s *goldenStub) SearchIssues(_ context.Context, opt forgejo_sdk.ListIssueOption) ([]*forgejo_sdk.Issue, forge.PageInfo, error) {
	issues := []*forgejo_sdk.Issue{goldenIssue("one"), goldenIssue("two")}
	if opt.Page > 1 {
		issues = nil
	}
	return issues, s.info, s.rec("SearchIssues", opt)
}

func (s *goldenStub) CreateIssue(_ context.Context, o, r string, opt forgejo_sdk.CreateIssueOption) (*forgejo_sdk.Issue, error) {
	return goldenIssue(opt.Title), s.rec("CreateIssue", o, r, opt)
}

func (s *goldenStub) EditIssue(_ context.Context, o, r string, i int64, opt forgejo_sdk.EditIssueOption) (*forgejo_sdk.Issue, error) {
	return goldenIssue("edited"), s.rec("EditIssue", o, r, i, opt)
}

func (s *goldenStub) ReplaceIssueLabels(_ context.Context, o, r string, i int64, ids []int64) error {
	return s.rec("ReplaceIssueLabels", o, r, i, ids)
}

func (s *goldenStub) ClearIssueLabels(_ context.Context, o, r string, i int64) error {
	return s.rec("ClearIssueLabels", o, r, i)
}

func (s *goldenStub) ListIssueComments(_ context.Context, o, r string, i int64, opt forgejo_sdk.ListIssueCommentOptions) ([]*forgejo_sdk.Comment, forge.PageInfo, error) {
	return []*forgejo_sdk.Comment{goldenComment("one")}, forge.PageInfo{}, s.rec("ListIssueComments", o, r, i, opt)
}

func (s *goldenStub) GetIssueComment(_ context.Context, o, r string, id int64) (*forgejo_sdk.Comment, error) {
	return goldenComment("got"), s.rec("GetIssueComment", o, r, id)
}

func (s *goldenStub) CreateIssueComment(_ context.Context, o, r string, i int64, body string) (*forgejo_sdk.Comment, error) {
	return goldenComment(body), s.rec("CreateIssueComment", o, r, i, body)
}

func (s *goldenStub) EditIssueComment(_ context.Context, o, r string, id int64, body string) (*forgejo_sdk.Comment, error) {
	return goldenComment(body), s.rec("EditIssueComment", o, r, id, body)
}

func (s *goldenStub) DeleteIssueComment(_ context.Context, o, r string, id int64) error {
	return s.rec("DeleteIssueComment", o, r, id)
}

func (s *goldenStub) ListRepoMilestones(_ context.Context, o, r string, opt forgejo_sdk.ListMilestoneOption) ([]*forgejo_sdk.Milestone, error) {
	return []*forgejo_sdk.Milestone{{ID: 4, Title: "v1", Created: gT0}}, s.rec("ListRepoMilestones", o, r, opt)
}

// goldenMilestone is milestone 4 as the server holds it. The write methods
// return it with the fields they were given applied, as Forgejo does.
func goldenMilestone() *forgejo_sdk.Milestone {
	return &forgejo_sdk.Milestone{ID: 4, Title: "v1", Description: "first", State: forgejo_sdk.StateOpen, OpenIssues: 2, ClosedIssues: 1, Created: gT0}
}

func (s *goldenStub) GetMilestone(_ context.Context, o, r string, id int64) (*forgejo_sdk.Milestone, error) {
	return goldenMilestone(), s.rec("GetMilestone", o, r, id)
}

func (s *goldenStub) CreateMilestone(_ context.Context, o, r string, opt forgejo_sdk.CreateMilestoneOption) (*forgejo_sdk.Milestone, error) {
	m := goldenMilestone()
	m.Title, m.Description, m.Deadline = opt.Title, opt.Description, opt.Deadline
	return m, s.rec("CreateMilestone", o, r, opt)
}

func (s *goldenStub) EditMilestone(_ context.Context, o, r string, id int64, opt forgejo_sdk.EditMilestoneOption) (*forgejo_sdk.Milestone, error) {
	m := goldenMilestone()
	if opt.Title != "" {
		m.Title = opt.Title
	}
	if opt.Description != nil {
		m.Description = *opt.Description
	}
	if opt.State != nil {
		m.State = *opt.State
	}
	if opt.Deadline != nil {
		m.Deadline = opt.Deadline
	}
	return m, s.rec("EditMilestone", o, r, id, opt)
}

// repoLabels are the labels of every repository: "bug" and "ui".
var goldenRepoLabels = []*forgejo_sdk.Label{
	{ID: 3, Name: "bug", Color: "ff0000", Description: "broken", URL: "https://f.example/l/3"},
	{ID: 4, Name: "ui", Color: "00ff00", Description: "look", URL: "https://f.example/l/4"},
}

// goldenOrgLabels are the labels of the organization "acme". "ui" repeats a
// repository label name, so resolving it is ambiguous. The owner "octo" is a
// user, which has no organization labels, and "denied" answers 403.
var goldenOrgLabels = []*forgejo_sdk.Label{
	{ID: 12, Name: "infra", Color: "0000ff", Description: "org", URL: "https://f.example/o/12"},
	{ID: 13, Name: "ui", Color: "00aaaa", Description: "org ui", URL: "https://f.example/o/13"},
}

func (s *goldenStub) ListRepoLabels(_ context.Context, o, r string, opt forgejo_sdk.ListLabelsOptions) ([]*forgejo_sdk.Label, forge.PageInfo, error) {
	var out []*forgejo_sdk.Label
	if opt.Page <= 1 {
		out = goldenRepoLabels
	}
	return out, forge.PageInfo{}, s.rec("ListRepoLabels", o, r, opt)
}

func (s *goldenStub) GetRepoLabel(_ context.Context, o, r string, id int64) (*forgejo_sdk.Label, error) {
	return goldenRepoLabels[0], s.rec("GetRepoLabel", o, r, id)
}

func (s *goldenStub) CreateRepoLabel(_ context.Context, o, r string, opt forgejo_sdk.CreateLabelOption) (*forgejo_sdk.Label, error) {
	return &forgejo_sdk.Label{ID: 5, Name: opt.Name, Color: opt.Color, Description: opt.Description, URL: "u"}, s.rec("CreateRepoLabel", o, r, opt)
}

func (s *goldenStub) EditRepoLabel(_ context.Context, o, r string, id int64, opt forgejo_sdk.EditLabelOption) (*forgejo_sdk.Label, error) {
	return goldenRepoLabels[0], s.rec("EditRepoLabel", o, r, id, opt)
}

func (s *goldenStub) DeleteRepoLabel(_ context.Context, o, r string, id int64) error {
	return s.rec("DeleteRepoLabel", o, r, id)
}

func (s *goldenStub) ListOrgLabels(_ context.Context, org string, page, limit int) ([]*forgejo_sdk.Label, forge.PageInfo, error) {
	err := s.rec("ListOrgLabels", org, page, limit)
	switch {
	case org == "denied":
		return nil, forge.PageInfo{}, goldenStatus(http.StatusForbidden)
	case org == "octo" || page > 1:
		return nil, forge.PageInfo{}, err
	}
	return goldenOrgLabels, forge.PageInfo{}, err
}

func (s *goldenStub) GetOrgLabel(_ context.Context, org string, id int64) (*forgejo_sdk.Label, error) {
	return goldenOrgLabels[0], s.rec("GetOrgLabel", org, id)
}

func (s *goldenStub) CreateOrgLabel(_ context.Context, org string, opt forgejo_sdk.CreateLabelOption) (*forgejo_sdk.Label, error) {
	return &forgejo_sdk.Label{ID: 14, Name: opt.Name, Color: opt.Color, Description: opt.Description, URL: "u"}, s.rec("CreateOrgLabel", org, opt)
}

func (s *goldenStub) EditOrgLabel(_ context.Context, org string, id int64, opt forgejo_sdk.EditLabelOption) (*forgejo_sdk.Label, error) {
	return goldenOrgLabels[0], s.rec("EditOrgLabel", org, id, opt)
}

func (s *goldenStub) DeleteOrgLabel(_ context.Context, org string, id int64) error {
	return s.rec("DeleteOrgLabel", org, id)
}

func (s *goldenStub) ListOrgRepoNames(_ context.Context, org string, page, limit int) ([]string, error) {
	var out []string
	if page == 1 {
		out = []string{"hello"}
	}
	return out, s.rec("ListOrgRepoNames", org, page, limit)
}

func (s *goldenStub) AddIssueLabels(_ context.Context, o, r string, i int64, ids []int64) error {
	return s.rec("AddIssueLabels", o, r, i, ids)
}

func (s *goldenStub) RemoveIssueLabel(_ context.Context, o, r string, i, id int64) error {
	return s.rec("RemoveIssueLabel", o, r, i, id)
}

func (s *goldenStub) ListIssueDependencies(_ context.Context, o, r string, i int64, page, limit int) ([]*forgejo_sdk.Issue, error) {
	return []*forgejo_sdk.Issue{goldenIssue("dep")}, s.rec("ListIssueDependencies", o, r, i, page, limit)
}

func (s *goldenStub) ListIssueDependents(_ context.Context, o, r string, i int64, page, limit int) ([]*forgejo_sdk.Issue, error) {
	return []*forgejo_sdk.Issue{goldenIssue("dependent")}, s.rec("ListIssueDependents", o, r, i, page, limit)
}

func (s *goldenStub) AddIssueDependency(_ context.Context, o, r string, i int64, dep forge.IssueRef) error {
	return s.rec("AddIssueDependency", o, r, i, dep)
}

func (s *goldenStub) RemoveIssueDependency(_ context.Context, o, r string, i int64, dep forge.IssueRef) error {
	return s.rec("RemoveIssueDependency", o, r, i, dep)
}

// goldenInvoke calls a registered tool the way the server does and renders
// what the client would see: the text, or the error, then the port calls made.
func goldenInvoke(t *testing.T, stub *goldenStub, tool string, args map[string]any) string {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Forgejo: stub})
	st := s.GetTool(tool)
	if st == nil {
		t.Fatalf("tool %s not registered", tool)
	}
	base := map[string]any{"owner": "octo", "repo": "hello"}
	for k, v := range args {
		if v == nil {
			delete(base, k) // a nil value means "leave the argument out"
			continue
		}
		base[k] = v
	}
	res, err := st.Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: tool, Arguments: base}})
	var out string
	switch {
	case err != nil:
		out = "ERROR: " + err.Error()
	case res.IsError:
		out = "RESULT-ERROR: " + res.Content[0].(mcp.TextContent).Text
	default:
		out = res.Content[0].(mcp.TextContent).Text
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return fmt.Sprintf("%s\n-- port calls --\n%s\n", out, strings.Join(stub.calls, "\n"))
}

type goldenCase struct {
	name string
	tool string
	args map[string]any
	stub func(*goldenStub)
}

func failing(method string, err error) func(*goldenStub) {
	return func(s *goldenStub) { s.failOn[method] = err }
}

var errBoom = errors.New("boom")

var goldenCases = []goldenCase{
	// get_issue_by_index
	{name: "get_issue", tool: GetIssueByIndexToolName, args: map[string]any{"index": float64(7)}},
	// verbose=true restores the full entity; this golden is the pre-compact output.
	{name: "get_issue_verbose", tool: GetIssueByIndexToolName, args: map[string]any{"index": float64(7), "verbose": true}},
	{name: "get_issue_string_index", tool: GetIssueByIndexToolName, args: map[string]any{"index": "7"}},
	// Forgiving argument handling of the original handlers, kept: a missing
	// required argument is forwarded as its zero value, and a fractional
	// number is truncated.
	{name: "get_issue_fractional_index", tool: GetIssueByIndexToolName, args: map[string]any{"index": 5.5}},
	{name: "get_issue_missing_required", tool: GetIssueByIndexToolName, args: map[string]any{"owner": nil, "repo": nil}},
	{name: "get_issue_port_error", tool: GetIssueByIndexToolName, args: map[string]any{"index": float64(7)}, stub: failing("GetIssue", errBoom)},
	// The golden stub's index 7 is a pull request, so type=issues refuses it and
	// type=pulls accepts it.
	{name: "get_issue_type_issues_refuses_pull", tool: GetIssueByIndexToolName, args: map[string]any{"index": float64(7), "type": "issues"}},
	{name: "get_issue_type_pulls_accepts_pull", tool: GetIssueByIndexToolName, args: map[string]any{"index": float64(7), "type": "pulls"}},
	{name: "get_issue_type_unknown", tool: GetIssueByIndexToolName, args: map[string]any{"index": float64(7), "type": "issue"}},

	// list_repo_issues
	{name: "list_issues_defaults", tool: ListRepoIssuesToolName},
	// verbose=true restores the full page; this golden is the pre-compact output.
	{name: "list_issues_verbose", tool: ListRepoIssuesToolName, args: map[string]any{"verbose": true}},
	{name: "list_issues_explicit_empty_state", tool: ListRepoIssuesToolName, args: map[string]any{"state": ""}},
	{name: "list_issues_non_string_state", tool: ListRepoIssuesToolName, args: map[string]any{"state": float64(5)}},
	{name: "list_issues_filtered", tool: ListRepoIssuesToolName, args: map[string]any{"state": "closed", "type": "issues", "milestones": "1,2", "labels": "bug,ui", "sort": "latest", "page": float64(2), "limit": float64(5)}},
	{name: "list_issues_fractional_paging", tool: ListRepoIssuesToolName, args: map[string]any{"page": 2.9, "limit": 3.5}},
	{name: "list_issues_string_paging", tool: ListRepoIssuesToolName, args: map[string]any{"page": "2", "limit": "4"}},
	{name: "list_issues_missing_owner", tool: ListRepoIssuesToolName, args: map[string]any{"owner": nil}},
	{name: "list_issues_missing_repo", tool: ListRepoIssuesToolName, args: map[string]any{"repo": nil}},
	{name: "list_issues_bad_sort", tool: ListRepoIssuesToolName, args: map[string]any{"sort": "random"}},
	{name: "list_issues_port_error", tool: ListRepoIssuesToolName, stub: failing("ListRepoIssues", errBoom)},

	// search_issues
	{name: "search_issues_defaults", tool: SearchIssuesToolName},
	// verbose=true restores the full page; this golden is the pre-compact output.
	{name: "search_issues_verbose", tool: SearchIssuesToolName, args: map[string]any{"verbose": true}},
	{name: "search_issues_known_total", tool: SearchIssuesToolName, args: map[string]any{"limit": float64(2)}, stub: func(s *goldenStub) {
		s.info = forge.PageInfo{HasNext: true, TotalCount: 42, TotalKnown: true}
	}},
	{name: "search_issues_over_ceiling", tool: SearchIssuesToolName, args: map[string]any{"limit": float64(60)}},
	{name: "search_issues_unknown_ceiling_probe", tool: SearchIssuesToolName, args: map[string]any{"limit": float64(2)}, stub: func(s *goldenStub) {
		s.ceilingKnown = false
	}},
	{name: "search_issues_unknown_ceiling_last_page", tool: SearchIssuesToolName, args: map[string]any{"page": float64(2)}, stub: func(s *goldenStub) {
		s.ceilingKnown = false
	}},
	{name: "search_issues_missing_owner", tool: SearchIssuesToolName, args: map[string]any{"owner": nil}},
	{name: "search_issues_filters", tool: SearchIssuesToolName, args: map[string]any{"state": "all", "type": "pulls", "labels": "a,b", "milestones": "1", "q": "crash", "created_by": "bob", "assigned_by": "al", "mentioned_by": "cy", "page": "0", "limit": "0"}},
	{name: "search_issues_port_error", tool: SearchIssuesToolName, stub: failing("SearchIssues", errBoom)},

	// create_issue
	{name: "create_issue", tool: CreateIssueToolName, args: map[string]any{"title": "T", "body": "B", "labels": "bug,ui", "assignees": "a, b", "milestone": "4", "ref": "feat"}},
	{name: "create_issue_plain", tool: CreateIssueToolName, args: map[string]any{"title": "T"}},
	{name: "create_issue_verbose", tool: CreateIssueToolName, args: map[string]any{"title": "T", "body": "B", "labels": "bug,ui", "verbose": true}},
	{name: "create_issue_numeric_label_ids", tool: CreateIssueToolName, args: map[string]any{"title": "T", "labels": "3,4"}},
	{name: "create_issue_empty_labels", tool: CreateIssueToolName, args: map[string]any{"title": "T", "labels": ""}},
	{name: "create_issue_unknown_label", tool: CreateIssueToolName, args: map[string]any{"title": "T", "labels": "bug,nope"}},
	{name: "create_issue_ambiguous_label", tool: CreateIssueToolName, args: map[string]any{"owner": "acme", "title": "T", "labels": "ui"}},
	{name: "create_issue_empty_assignees", tool: CreateIssueToolName, args: map[string]any{"title": "T", "assignees": ""}},
	{name: "create_issue_bad_milestone", tool: CreateIssueToolName, args: map[string]any{"title": "T", "milestone": "x"}},
	{name: "create_issue_missing_title", tool: CreateIssueToolName, args: map[string]any{"title": nil}},
	{name: "create_issue_port_error", tool: CreateIssueToolName, args: map[string]any{"title": "T"}, stub: failing("CreateIssue", errBoom)},

	// create_issue_comment, list/get/edit/delete_issue_comment
	{name: "create_issue_comment", tool: CreateIssueCommentToolName, args: map[string]any{"index": float64(7), "body": "hi"}},
	{name: "create_issue_comment_port_error", tool: CreateIssueCommentToolName, args: map[string]any{"index": float64(7), "body": "hi"}, stub: failing("CreateIssueComment", errBoom)},
	{name: "create_issue_comment_verbose", tool: CreateIssueCommentToolName, args: map[string]any{"index": float64(7), "body": "hi", "verbose": true}},
	{name: "add_issue_labels_verbose", tool: AddIssueLabelsToolName, args: map[string]any{"index": float64(7), "labels": "bug", "verbose": true}},
	{name: "remove_issue_labels_verbose", tool: RemoveIssueLabelsToolName, args: map[string]any{"index": float64(7), "labels": "bug", "verbose": true}},
	{name: "list_issue_comments_defaults", tool: ListIssueCommentsToolName, args: map[string]any{"index": float64(7)}},
	{name: "list_issue_comments_paged_since_before", tool: ListIssueCommentsToolName, args: map[string]any{"index": float64(7), "page": float64(2), "limit": float64(5), "since": "2026-01-02T03:04:05Z", "before": "2026-02-03T04:05:06Z"}},
	{name: "list_issue_comments_bad_since", tool: ListIssueCommentsToolName, args: map[string]any{"index": float64(7), "since": "yesterday"}},
	{name: "list_issue_comments_bad_before", tool: ListIssueCommentsToolName, args: map[string]any{"index": float64(7), "before": "x"}},
	{name: "list_issue_comments_port_error", tool: ListIssueCommentsToolName, args: map[string]any{"index": float64(7)}, stub: failing("ListIssueComments", errBoom)},
	{name: "get_issue_comment", tool: GetIssueCommentToolName, args: map[string]any{"comment_id": float64(31)}},
	// verbose=true restores the full comment; this golden is the pre-compact output.
	{name: "get_issue_comment_verbose", tool: GetIssueCommentToolName, args: map[string]any{"comment_id": float64(31), "verbose": true}},
	{name: "get_issue_comment_port_error", tool: GetIssueCommentToolName, args: map[string]any{"comment_id": float64(31)}, stub: failing("GetIssueComment", errBoom)},
	{name: "edit_issue_comment", tool: EditIssueCommentToolName, args: map[string]any{"comment_id": float64(31), "body": "x"}},
	{name: "edit_issue_comment_port_error", tool: EditIssueCommentToolName, args: map[string]any{"comment_id": float64(31), "body": "x"}, stub: failing("EditIssueComment", errBoom)},
	{name: "edit_issue_comment_verbose", tool: EditIssueCommentToolName, args: map[string]any{"comment_id": float64(31), "body": "x", "verbose": true}},
	{name: "delete_issue_comment", tool: DeleteIssueCommentToolName, args: map[string]any{"comment_id": float64(31)}},
	{name: "delete_issue_comment_port_error", tool: DeleteIssueCommentToolName, args: map[string]any{"comment_id": float64(31)}, stub: failing("DeleteIssueComment", errBoom)},

	// update_issue
	{name: "update_issue_fields", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7), "title": "T", "body": "B", "assignee": "bob", "milestone": "4", "due_date": "2026-08-20T00:00:00Z", "ref": "feat"}},
	{name: "update_issue_verbose", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7), "title": "T", "body": "B", "verbose": true}},
	{name: "update_issue_assignees_win", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7), "assignees": "a,b", "assignee": "bob"}},
	{name: "update_issue_clear_assignees", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7), "assignees": ""}},
	{name: "update_issue_clear_ref", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7), "ref": ""}},
	{name: "update_issue_clear_due", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7), "clear_due_date": true}},
	{name: "update_issue_due_conflict", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7), "due_date": "2026-08-20T00:00:00Z", "clear_due_date": true}},
	{name: "update_issue_bad_due", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7), "due_date": "soon"}},
	{name: "update_issue_bad_milestone", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7), "milestone": "x"}},
	{name: "update_issue_set_labels", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7), "title": "T", "set_labels": "bug,ui"}},
	{name: "update_issue_set_labels_only", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7), "set_labels": "bug"}},
	{name: "update_issue_clear_labels", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7), "set_labels": ""}},
	{name: "update_issue_set_labels_unknown", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7), "title": "T", "set_labels": "nope"}},
	{name: "update_issue_nothing", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7)}},
	{name: "update_issue_wrong_types", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7), "title": float64(5), "clear_due_date": "true", "assignees": float64(7)}},
	{name: "update_issue_edit_error", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7), "title": "T"}, stub: failing("EditIssue", errBoom)},
	{name: "update_issue_replace_error", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7), "set_labels": "bug"}, stub: failing("ReplaceIssueLabels", errBoom)},
	{name: "update_issue_clear_error", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7), "set_labels": ""}, stub: failing("ClearIssueLabels", errBoom)},
	{name: "update_issue_reread_error", tool: UpdateIssueToolName, args: map[string]any{"index": float64(7), "set_labels": "bug"}, stub: failing("GetIssue", errBoom)},

	// issue_state_change
	{name: "issue_state_change_closed", tool: IssueStateChangeToolName, args: map[string]any{"index": float64(7), "state": "closed"}},
	{name: "issue_state_change_verbose", tool: IssueStateChangeToolName, args: map[string]any{"index": float64(7), "state": "closed", "verbose": true}},
	{name: "issue_state_change_invalid", tool: IssueStateChangeToolName, args: map[string]any{"index": float64(7), "state": "merged"}},
	{name: "issue_state_change_port_error", tool: IssueStateChangeToolName, args: map[string]any{"index": float64(7), "state": "open"}, stub: failing("EditIssue", errBoom)},

	// list_repo_milestones
	{name: "list_milestones_defaults", tool: ListRepoMilestonesToolName},
	{name: "list_milestones_closed_paged", tool: ListRepoMilestonesToolName, args: map[string]any{"state": "closed", "page": float64(2), "limit": float64(10)}},
	{name: "list_milestones_empty_state", tool: ListRepoMilestonesToolName, args: map[string]any{"state": ""}},
	{name: "list_milestones_port_error", tool: ListRepoMilestonesToolName, stub: failing("ListRepoMilestones", errBoom)},

	// create_milestone, edit_milestone, get_milestone
	{name: "create_milestone", tool: CreateMilestoneToolName, args: map[string]any{"title": "v2", "description": "second", "due_date": "2026-12-01T00:00:00Z"}},
	{name: "create_milestone_no_due", tool: CreateMilestoneToolName, args: map[string]any{"title": "v2"}},
	{name: "create_milestone_blank_title", tool: CreateMilestoneToolName, args: map[string]any{"title": "  "}},
	{name: "create_milestone_bad_due", tool: CreateMilestoneToolName, args: map[string]any{"title": "v2", "due_date": "tomorrow"}},
	{name: "create_milestone_port_error", tool: CreateMilestoneToolName, args: map[string]any{"title": "v2"}, stub: failing("CreateMilestone", errBoom)},
	{name: "edit_milestone_close", tool: EditMilestoneToolName, args: map[string]any{"id": float64(4), "state": "closed"}},
	{name: "edit_milestone_partial", tool: EditMilestoneToolName, args: map[string]any{"id": float64(4), "description": "fixed"}},
	{name: "edit_milestone_rename_and_due", tool: EditMilestoneToolName, args: map[string]any{"id": float64(4), "title": "v1.1", "due_date": "2027-01-02T03:04:05Z"}},
	{name: "edit_milestone_no_fields", tool: EditMilestoneToolName, args: map[string]any{"id": float64(4)}},
	{name: "edit_milestone_bad_state", tool: EditMilestoneToolName, args: map[string]any{"id": float64(4), "state": "archived"}},
	{name: "edit_milestone_bad_id", tool: EditMilestoneToolName, args: map[string]any{"id": float64(2.5), "state": "closed"}},
	{name: "edit_milestone_not_found", tool: EditMilestoneToolName, args: map[string]any{"id": float64(99), "state": "closed"}, stub: failing("EditMilestone", goldenStatus(http.StatusNotFound))},
	{name: "get_milestone", tool: GetMilestoneToolName, args: map[string]any{"id": float64(4)}},
	{name: "get_milestone_zero_id", tool: GetMilestoneToolName, args: map[string]any{"id": float64(0)}},
	{name: "get_milestone_not_found", tool: GetMilestoneToolName, args: map[string]any{"id": float64(99)}, stub: failing("GetMilestone", goldenStatus(http.StatusNotFound))},
	{name: "get_milestone_port_error", tool: GetMilestoneToolName, args: map[string]any{"id": float64(4)}, stub: failing("GetMilestone", errBoom)},

	// list_repo_labels, list_org_labels
	{name: "list_repo_labels_user_owner", tool: ListRepoLabelsToolName},
	{name: "list_repo_labels_merged", tool: ListRepoLabelsToolName, args: map[string]any{"owner": "acme"}},
	{name: "list_repo_labels_no_org", tool: ListRepoLabelsToolName, args: map[string]any{"owner": "acme", "include_org_labels": false}},
	{name: "list_repo_labels_include_string", tool: ListRepoLabelsToolName, args: map[string]any{"owner": "acme", "include_org_labels": "false"}},
	{name: "list_repo_labels_paged", tool: ListRepoLabelsToolName, args: map[string]any{"page": float64(2), "limit": float64(1)}},
	{name: "list_repo_labels_port_error", tool: ListRepoLabelsToolName, stub: failing("ListRepoLabels", errBoom)},
	{name: "list_repo_labels_org_error", tool: ListRepoLabelsToolName, args: map[string]any{"owner": "acme"}, stub: failing("ListOrgLabels", errBoom)},
	{name: "list_org_labels_defaults", tool: ListOrgLabelsToolName, args: map[string]any{"org": "acme"}},
	{name: "list_org_labels_user_is_empty", tool: ListOrgLabelsToolName, args: map[string]any{"org": "octo"}},
	{name: "list_org_labels_forbidden", tool: ListOrgLabelsToolName, args: map[string]any{"org": "denied"}},

	// add_issue_labels, remove_issue_labels
	{name: "add_issue_labels_names", tool: AddIssueLabelsToolName, args: map[string]any{"index": float64(7), "labels": "bug,ui"}},
	{name: "add_issue_labels_id", tool: AddIssueLabelsToolName, args: map[string]any{"index": float64(7), "labels": "4"}},
	{name: "add_issue_labels_unknown", tool: AddIssueLabelsToolName, args: map[string]any{"index": float64(7), "labels": "nope"}},
	{name: "add_issue_labels_empty", tool: AddIssueLabelsToolName, args: map[string]any{"index": float64(7), "labels": ""}},
	{name: "add_issue_labels_port_error", tool: AddIssueLabelsToolName, args: map[string]any{"index": float64(7), "labels": "bug"}, stub: failing("AddIssueLabels", errBoom)},
	{name: "add_issue_labels_reread_error", tool: AddIssueLabelsToolName, args: map[string]any{"index": float64(7), "labels": "bug"}, stub: failing("GetIssue", errBoom)},
	{name: "remove_issue_labels_names", tool: RemoveIssueLabelsToolName, args: map[string]any{"index": float64(7), "labels": "bug,ui"}},
	{name: "remove_issue_labels_unknown", tool: RemoveIssueLabelsToolName, args: map[string]any{"index": float64(7), "labels": "nope"}},
	{name: "remove_issue_labels_port_error", tool: RemoveIssueLabelsToolName, args: map[string]any{"index": float64(7), "labels": "bug"}, stub: failing("RemoveIssueLabel", errBoom)},

	// repo labels
	{name: "create_repo_label", tool: CreateRepoLabelToolName, args: map[string]any{"name": "ui2", "color": "#0088FF", "description": "d"}},
	{name: "create_repo_label_bad_color", tool: CreateRepoLabelToolName, args: map[string]any{"name": "x", "color": "red"}},
	{name: "create_repo_label_port_error", tool: CreateRepoLabelToolName, args: map[string]any{"name": "x", "color": "0088ff"}, stub: failing("CreateRepoLabel", errBoom)},
	{name: "edit_repo_label", tool: EditRepoLabelToolName, args: map[string]any{"id": float64(3), "name": "x", "color": "00ff00"}},
	{name: "edit_repo_label_no_fields", tool: EditRepoLabelToolName, args: map[string]any{"id": float64(3)}},
	{name: "edit_repo_label_empty_name_and_description", tool: EditRepoLabelToolName, args: map[string]any{"id": float64(3), "name": "", "description": ""}},
	{name: "edit_repo_label_bad_color", tool: EditRepoLabelToolName, args: map[string]any{"id": float64(3), "color": "zz"}},
	{name: "edit_repo_label_port_error", tool: EditRepoLabelToolName, args: map[string]any{"id": float64(3), "name": "x"}, stub: failing("EditRepoLabel", errBoom)},
	{name: "delete_repo_label_unused", tool: DeleteRepoLabelToolName, args: map[string]any{"id": float64(3)}},
	{name: "delete_repo_label_in_use", tool: DeleteRepoLabelToolName, args: map[string]any{"id": float64(3)}, stub: func(s *goldenStub) {
		s.info = forge.PageInfo{TotalCount: 2, TotalKnown: true}
	}},
	{name: "delete_repo_label_force", tool: DeleteRepoLabelToolName, args: map[string]any{"id": float64(3), "delete_mode": "force"}},
	{name: "delete_repo_label_not_found", tool: DeleteRepoLabelToolName, args: map[string]any{"id": float64(3), "delete_mode": "force"}, stub: failing("DeleteRepoLabel", goldenStatus(http.StatusNotFound))},
	{name: "delete_repo_label_port_error", tool: DeleteRepoLabelToolName, args: map[string]any{"id": float64(3), "delete_mode": "force"}, stub: failing("DeleteRepoLabel", errBoom)},
	{name: "delete_repo_label_get_error", tool: DeleteRepoLabelToolName, args: map[string]any{"id": float64(3)}, stub: failing("GetRepoLabel", errBoom)},
	{name: "get_repo_label", tool: GetRepoLabelToolName, args: map[string]any{"id": float64(3)}},
	{name: "get_repo_label_port_error", tool: GetRepoLabelToolName, args: map[string]any{"id": float64(3)}, stub: failing("GetRepoLabel", errBoom)},

	// org labels
	{name: "create_org_label", tool: CreateOrgLabelToolName, args: map[string]any{"org": "acme", "name": "n", "color": "0088ff"}},
	{name: "create_org_label_bad_color", tool: CreateOrgLabelToolName, args: map[string]any{"org": "acme", "name": "n", "color": "red"}},
	{name: "create_org_label_port_error", tool: CreateOrgLabelToolName, args: map[string]any{"org": "acme", "name": "n", "color": "0088ff"}, stub: failing("CreateOrgLabel", errBoom)},
	{name: "edit_org_label", tool: EditOrgLabelToolName, args: map[string]any{"org": "acme", "id": float64(12), "name": "n"}},
	{name: "edit_org_label_no_fields", tool: EditOrgLabelToolName, args: map[string]any{"org": "acme", "id": float64(12)}},
	{name: "edit_org_label_port_error", tool: EditOrgLabelToolName, args: map[string]any{"org": "acme", "id": float64(12), "name": "n"}, stub: failing("EditOrgLabel", errBoom)},
	{name: "delete_org_label_unused", tool: DeleteOrgLabelToolName, args: map[string]any{"org": "acme", "id": float64(12)}, stub: func(s *goldenStub) {
		s.info = forge.PageInfo{TotalCount: 0, TotalKnown: true}
	}},
	{name: "delete_org_label_in_use", tool: DeleteOrgLabelToolName, args: map[string]any{"org": "acme", "id": float64(12)}, stub: func(s *goldenStub) {
		s.info = forge.PageInfo{TotalCount: 3, TotalKnown: true}
	}},
	{name: "delete_org_label_force", tool: DeleteOrgLabelToolName, args: map[string]any{"org": "acme", "id": float64(12), "delete_mode": "force"}},
	{name: "delete_org_label_count_error", tool: DeleteOrgLabelToolName, args: map[string]any{"org": "acme", "id": float64(12)}, stub: failing("ListOrgRepoNames", errBoom)},
	{name: "delete_org_label_port_error", tool: DeleteOrgLabelToolName, args: map[string]any{"org": "acme", "id": float64(12), "delete_mode": "force"}, stub: failing("DeleteOrgLabel", errBoom)},
	{name: "get_org_label", tool: GetOrgLabelToolName, args: map[string]any{"org": "acme", "id": float64(12)}},
	{name: "get_org_label_port_error", tool: GetOrgLabelToolName, args: map[string]any{"org": "acme", "id": float64(12)}, stub: failing("GetOrgLabel", errBoom)},

	// issue dependencies
	{name: "list_dependencies_defaults", tool: ListIssueDependenciesToolName, args: map[string]any{"index": float64(7)}},
	{name: "list_dependencies_paged", tool: ListIssueDependenciesToolName, args: map[string]any{"index": float64(7), "page": float64(2), "limit": float64(5)}},
	{name: "list_dependencies_port_error", tool: ListIssueDependenciesToolName, args: map[string]any{"index": float64(7)}, stub: failing("ListIssueDependencies", errBoom)},
	{name: "list_dependents_defaults", tool: ListIssueDependentsToolName, args: map[string]any{"index": float64(7)}},
	{name: "list_dependents_port_error", tool: ListIssueDependentsToolName, args: map[string]any{"index": float64(7)}, stub: failing("ListIssueDependents", errBoom)},
	{name: "add_dependency", tool: AddIssueDependencyToolName, args: map[string]any{"index": float64(7), "depends_on_index": float64(9)}},
	{name: "add_dependency_cross_repo", tool: AddIssueDependencyToolName, args: map[string]any{"index": float64(7), "depends_on_owner": "acme", "depends_on_repo": "other", "depends_on_index": float64(7)}},
	{name: "add_dependency_self", tool: AddIssueDependencyToolName, args: map[string]any{"index": float64(7), "depends_on_index": float64(7)}},
	{name: "add_dependency_self_case_insensitive", tool: AddIssueDependencyToolName, args: map[string]any{"index": float64(7), "depends_on_owner": "OCTO", "depends_on_repo": "HELLO", "depends_on_index": float64(7)}},
	{name: "add_dependency_missing_depends_on", tool: AddIssueDependencyToolName, args: map[string]any{"index": float64(7), "depends_on_index": nil}},
	{name: "add_dependency_bad_index", tool: AddIssueDependencyToolName, args: map[string]any{"index": "abc", "depends_on_index": float64(9)}},
	{name: "add_dependency_wrong_type_owner", tool: AddIssueDependencyToolName, args: map[string]any{"index": float64(7), "depends_on_owner": float64(5), "depends_on_index": float64(9)}},
	{name: "add_dependency_empty_owner_defaults", tool: AddIssueDependencyToolName, args: map[string]any{"index": float64(7), "depends_on_owner": "", "depends_on_index": float64(9)}},
	{name: "add_dependency_port_error", tool: AddIssueDependencyToolName, args: map[string]any{"index": float64(7), "depends_on_index": float64(9)}, stub: failing("AddIssueDependency", errBoom)},
	{name: "remove_dependency", tool: RemoveIssueDependencyToolName, args: map[string]any{"index": float64(7), "dependency_index": float64(9)}},
	{name: "remove_dependency_cross_repo", tool: RemoveIssueDependencyToolName, args: map[string]any{"index": float64(7), "dependency_owner": "acme", "dependency_repo": "other", "dependency_index": float64(9)}},
	{name: "remove_dependency_missing_index", tool: RemoveIssueDependencyToolName, args: map[string]any{"index": float64(7), "dependency_index": nil}},
	{name: "remove_dependency_wrong_type_repo", tool: RemoveIssueDependencyToolName, args: map[string]any{"index": float64(7), "dependency_repo": true, "dependency_index": float64(9)}},
	{name: "remove_dependency_port_error", tool: RemoveIssueDependencyToolName, args: map[string]any{"index": float64(7), "dependency_index": float64(9)}, stub: failing("RemoveIssueDependency", errBoom)},
}

func TestGolden(t *testing.T) {
	t.Parallel()
	for _, c := range goldenCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stub := newGoldenStub()
			if c.stub != nil {
				c.stub(stub)
			}
			mcptest.Golden(t, c.name, []byte(goldenInvoke(t, stub, c.tool, c.args)))
		})
	}
}
