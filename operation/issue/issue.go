// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"

	apppissue "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/issue"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcpresult"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// refArgDescription is the shared description of the 'ref' argument on
// create_issue and update_issue. The value goes to Forgejo as given: this
// server neither normalises it nor checks that the branch exists.
const refArgDescription = "The branch this issue is linked to (Forgejo's issue 'ref' field, distinct from a pull request link). Passed to Forgejo as given; Forgejo decides which forms it accepts."

// labelsArgDescription keeps the four label arguments telling the same story:
// what they accept, and that a name they cannot resolve stops the call instead
// of quietly applying a subset.
func labelsArgDescription(lead string) string {
	return lead + " (comma-separated). Accepts label names or numeric IDs. A name is " +
		"matched before an ID, so a label named \"123\" wins over the ID 123. An " +
		"unrecognised name is an error and nothing is applied; list_repo_labels shows " +
		"what exists."
}

const (
	GetIssueByIndexToolName    = "get_issue_by_index"
	ListRepoIssuesToolName     = "list_repo_issues"
	CreateIssueToolName        = "create_issue"
	CreateIssueCommentToolName = "create_issue_comment"
	UpdateIssueToolName        = "update_issue"
	AddIssueLabelsToolName     = "add_issue_labels"
	RemoveIssueLabelsToolName  = "remove_issue_labels"
	IssueStateChangeToolName   = "issue_state_change"
	ListIssueCommentsToolName  = "list_issue_comments"
	GetIssueCommentToolName    = "get_issue_comment"
	EditIssueCommentToolName   = "edit_issue_comment"
	DeleteIssueCommentToolName = "delete_issue_comment"
	ListRepoMilestonesToolName = "list_repo_milestones"
	ListRepoLabelsToolName     = "list_repo_labels"
	ListOrgLabelsToolName      = "list_org_labels"
	SearchIssuesToolName       = "search_issues"
)

var (
	GetIssueByIndexTool = mcp.NewTool(
		GetIssueByIndexToolName,
		mcp.WithDescription("Get issue by index"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
	)

	ListRepoIssuesTool = mcp.NewTool(
		ListRepoIssuesToolName,
		mcp.WithDescription("List repo issues"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("state", mcp.Description("State (open|closed|all)"), mcp.DefaultString("open")),
		mcp.WithString("type", mcp.Description("Type (issues|pulls)")),
		mcp.WithString("milestones", mcp.Description("Milestone names/IDs (comma-separated)")),
		mcp.WithString("labels", mcp.Description("Labels (comma-separated)")),
		mcp.WithString("sort", mcp.Description("Server-side sort order. One of: relevance, latest, oldest, recentupdate, leastupdate, mostcomment, leastcomment, nearduedate, farduedate. Default is the API's own default (latest).")),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	CreateIssueTool = mcp.NewTool(
		CreateIssueToolName,
		mcp.WithDescription("Create issue, optionally with labels (by name or ID), assignees and a milestone in the same call"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("title", mcp.Required(), mcp.Description(params.Title)),
		mcp.WithString("body", mcp.Description(params.Body)),
		mcp.WithString("labels", mcp.Description(labelsArgDescription("Labels for the new issue"))),
		mcp.WithString("assignees", mcp.Description("Assignee usernames (comma-separated)")),
		mcp.WithString("milestone", mcp.Description(params.Milestone)),
		mcp.WithString("ref", mcp.Description(refArgDescription)),
	)

	CreateIssueCommentTool = mcp.NewTool(
		CreateIssueCommentToolName,
		mcp.WithDescription("Create issue comment"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
		mcp.WithString("body", mcp.Required(), mcp.Description(params.Body)),
	)

	UpdateIssueTool = mcp.NewTool(
		UpdateIssueToolName,
		mcp.WithDescription("Update issue. 'set_labels' replaces the issue's whole label set; use add_issue_labels or remove_issue_labels to change it incrementally"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithString("title", mcp.Description(params.Title)),
		mcp.WithString("body", mcp.Description(params.Body)),
		mcp.WithString("assignee", mcp.Description("Assignee username (convenience for a single user; equivalent to a one-element 'assignees')")),
		mcp.WithString("assignees", mcp.Description("Assignee usernames (comma-separated). Overrides 'assignee' if both are set. Pass an empty string to clear all assignees.")),
		mcp.WithString("milestone", mcp.Description(params.Milestone)),
		mcp.WithString("due_date", mcp.Description("Set the issue's due date (RFC3339, e.g. 2026-08-20T00:00:00Z). Mutually exclusive with 'clear_due_date'; setting both is an error.")),
		mcp.WithBoolean("clear_due_date", mcp.Description("Clear the issue's due date. Mutually exclusive with 'due_date'.")),
		mcp.WithString("set_labels", mcp.Description(labelsArgDescription("Replace the issue's entire label set")+" Pass an empty string to remove every label. To add or drop individual labels without restating the set, use add_issue_labels or remove_issue_labels.")),
		mcp.WithString("ref", mcp.Description(refArgDescription+" Omit to leave it unchanged; pass an empty string to clear it.")),
	)

	AddIssueLabelsTools = mcp.NewTool(
		AddIssueLabelsToolName,
		mcp.WithDescription("Add labels to issue, by label name or numeric ID"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithString("labels", mcp.Required(), mcp.Description(labelsArgDescription("Labels to add"))),
	)

	RemoveIssueLabelsTools = mcp.NewTool(
		RemoveIssueLabelsToolName,
		mcp.WithDescription("Remove labels from issue, by label name or numeric ID"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithString("labels", mcp.Required(), mcp.Description(labelsArgDescription("Labels to remove"))),
	)

	IssueStateChangeTool = mcp.NewTool(
		IssueStateChangeToolName,
		mcp.WithDescription("Change issue state"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithString("state", mcp.Required(), mcp.Description("State (open|closed)")),
	)

	ListIssueCommentsTool = mcp.NewTool(
		ListIssueCommentsToolName,
		mcp.WithDescription("List issue/PR comments"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
		mcp.WithString("since", mcp.Description(params.Since)),
		mcp.WithString("before", mcp.Description(params.Before)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	GetIssueCommentTool = mcp.NewTool(
		GetIssueCommentToolName,
		mcp.WithDescription("Get comment by ID"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
	)

	EditIssueCommentTool = mcp.NewTool(
		EditIssueCommentToolName,
		mcp.WithDescription("Edit issue/PR comment"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
		mcp.WithString("body", mcp.Required(), mcp.Description(params.Body)),
	)

	DeleteIssueCommentTool = mcp.NewTool(
		DeleteIssueCommentToolName,
		mcp.WithDescription("Delete issue/PR comment"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
	)

	ListRepoMilestonesTool = mcp.NewTool(
		ListRepoMilestonesToolName,
		mcp.WithDescription("List repository milestones"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
		mcp.WithString("state", mcp.Description("Milestone state (open|closed|all)"), mcp.DefaultString("open")),
	)

	ListRepoLabelsTool = mcp.NewTool(
		ListRepoLabelsToolName,
		mcp.WithDescription("List repository labels. When the owner is an organization and include_org_labels is true (default), org-level labels are merged into the response. Each label carries a scope field of \"repo\" or \"org\"."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
		mcp.WithBoolean("include_org_labels", mcp.Description("Merge org-level labels into the response when the owner is an organization. Default true."), mcp.DefaultBool(true)),
	)

	ListOrgLabelsTool = mcp.NewTool(
		ListOrgLabelsToolName,
		mcp.WithDescription("List organization-level labels. Each label carries a scope field of \"org\"."),
		mcp.WithString("org", mcp.Required(), mcp.Description("Organization name")),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
	)

	SearchIssuesTool = mcp.NewTool(
		SearchIssuesToolName,
		mcp.WithDescription("Search issues across every repository belonging to one owner (organization or user), without naming a repo. "+
			"Returns a response envelope {issues, page, limit, count, has_next, total_count} rather than a bare array. "+
			"has_next true means a further page may exist; re-issue the call with page incremented to fetch it. "+
			"total_count, when present, is the total number of matching issues across all pages (from Forgejo's X-Total-Count header); it is omitted rather than 0 when the header is unavailable. "+
			"Each issue identifies its source repository only via its html_url/url field; there is no separate repository field. "+
			"Use list_repo_issues instead when the repo is already known."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("state", mcp.Description("State (open|closed|all)"), mcp.DefaultString("open")),
		mcp.WithString("type", mcp.Description("Type (issues|pulls)")),
		mcp.WithString("labels", mcp.Description("Labels (comma-separated); OR semantics — an issue matching any listed label is returned")),
		mcp.WithString("milestones", mcp.Description("Milestone names/IDs (comma-separated)")),
		mcp.WithString("q", mcp.Description(params.Keyword+" (matches issue title, body, and comments)")),
		mcp.WithString("created_by", mcp.Description("Filter by creator username")),
		mcp.WithString("assigned_by", mcp.Description("Filter by assignee username")),
		mcp.WithString("mentioned_by", mcp.Description("Filter by mentioned username")),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)
)

// RegisterTool registers the issue, label and dependency tools. Each call
// takes its Forgejo port from f, so a request is served with the credential
// that request carries.
func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := newHandlers(f)
	s.AddTool(GetIssueByIndexTool, h.GetIssueByIndexFn)
	s.AddTool(ListRepoIssuesTool, h.ListRepoIssuesFn)
	s.AddTool(CreateIssueTool, h.CreateIssueFn)
	s.AddTool(CreateIssueCommentTool, h.CreateIssueCommentFn)
	s.AddTool(UpdateIssueTool, h.UpdateIssueFn)
	s.AddTool(AddIssueLabelsTools, h.AddIssueLabelsFn)
	s.AddTool(RemoveIssueLabelsTools, h.RemoveIssueLabelsFn)
	s.AddTool(IssueStateChangeTool, h.IssueStateChangeFn)
	s.AddTool(ListIssueCommentsTool, h.ListIssueCommentsFn)
	s.AddTool(GetIssueCommentTool, h.GetIssueCommentFn)
	s.AddTool(EditIssueCommentTool, h.EditIssueCommentFn)
	s.AddTool(DeleteIssueCommentTool, h.DeleteIssueCommentFn)
	s.AddTool(ListRepoMilestonesTool, h.ListRepoMilestonesFn)
	s.AddTool(ListRepoLabelsTool, h.ListRepoLabelsFn)
	s.AddTool(ListOrgLabelsTool, h.ListOrgLabelsFn)
	s.AddTool(SearchIssuesTool, h.SearchIssuesFn)
	h.registerLabelTools(s)
	h.registerDependencyTools(s)
}

// Argument structs, one per tool. `json` names the MCP argument and
// `required:"true"` marks the required ones; mcptest.ArgsMatchSchema keeps
// them in agreement with the tool definitions. A pointer field is an argument
// whose presence matters: nil when it was left out (or given with the wrong
// type), set when it was given, even as "".

type issueRefArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Index float64 `json:"index" required:"true"`
}

type getIssueArgs = issueRefArgs

type listIssuesArgs struct {
	Owner      string  `json:"owner" required:"true"`
	Repo       string  `json:"repo" required:"true"`
	State      *string `json:"state"`
	Type       string  `json:"type"`
	Milestones string  `json:"milestones"`
	Labels     string  `json:"labels"`
	Sort       string  `json:"sort"`
	Page       float64 `json:"page"`
	Limit      float64 `json:"limit"`
}

type createIssueArgs struct {
	Owner     string  `json:"owner" required:"true"`
	Repo      string  `json:"repo" required:"true"`
	Title     string  `json:"title" required:"true"`
	Body      string  `json:"body"`
	Labels    *string `json:"labels"`
	Assignees *string `json:"assignees"`
	Milestone string  `json:"milestone"`
	Ref       string  `json:"ref"`
}

type createCommentArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Index float64 `json:"index" required:"true"`
	Body  string  `json:"body" required:"true"`
}

type updateIssueArgs struct {
	Owner        string  `json:"owner" required:"true"`
	Repo         string  `json:"repo" required:"true"`
	Index        float64 `json:"index" required:"true"`
	Title        string  `json:"title"`
	Body         string  `json:"body"`
	Assignee     string  `json:"assignee"`
	Assignees    *string `json:"assignees"`
	Milestone    string  `json:"milestone"`
	DueDate      string  `json:"due_date"`
	ClearDueDate bool    `json:"clear_due_date"`
	SetLabels    *string `json:"set_labels"`
	Ref          *string `json:"ref"`
}

type labelsArgs struct {
	Owner  string  `json:"owner" required:"true"`
	Repo   string  `json:"repo" required:"true"`
	Index  float64 `json:"index" required:"true"`
	Labels string  `json:"labels" required:"true"`
}

type stateArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Index float64 `json:"index" required:"true"`
	State string  `json:"state" required:"true"`
}

type listCommentsArgs struct {
	Owner  string  `json:"owner" required:"true"`
	Repo   string  `json:"repo" required:"true"`
	Index  float64 `json:"index" required:"true"`
	Since  string  `json:"since"`
	Before string  `json:"before"`
	Page   float64 `json:"page"`
	Limit  float64 `json:"limit"`
}

type commentArgs struct {
	Owner     string  `json:"owner" required:"true"`
	Repo      string  `json:"repo" required:"true"`
	CommentID float64 `json:"comment_id" required:"true"`
}

type editCommentArgs struct {
	Owner     string  `json:"owner" required:"true"`
	Repo      string  `json:"repo" required:"true"`
	CommentID float64 `json:"comment_id" required:"true"`
	Body      string  `json:"body" required:"true"`
}

type milestonesArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Page  float64 `json:"page"`
	Limit float64 `json:"limit"`
	State string  `json:"state"`
}

type repoLabelsArgs struct {
	Owner      string  `json:"owner" required:"true"`
	Repo       string  `json:"repo" required:"true"`
	Page       float64 `json:"page"`
	Limit      float64 `json:"limit"`
	IncludeOrg *bool   `json:"include_org_labels"`
}

type orgLabelsArgs struct {
	Org   string  `json:"org" required:"true"`
	Page  float64 `json:"page"`
	Limit float64 `json:"limit"`
}

type searchArgs struct {
	Owner       string  `json:"owner" required:"true"`
	State       string  `json:"state"`
	Type        string  `json:"type"`
	Labels      string  `json:"labels"`
	Milestones  string  `json:"milestones"`
	Q           string  `json:"q"`
	CreatedBy   string  `json:"created_by"`
	AssignedBy  string  `json:"assigned_by"`
	MentionedBy string  `json:"mentioned_by"`
	Page        float64 `json:"page"`
	Limit       float64 `json:"limit"`
}

// target is the issue a call names.
func target(a issueRefArgs) apppissue.Target {
	return apppissue.Target{Owner: a.Owner, Repo: a.Repo, Index: int64(a.Index)}
}

func (h handlers) GetIssueByIndexFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "GetIssueByIndexFn", func(ctx context.Context, s *apppissue.Service, a getIssueArgs) (*mcp.CallToolResult, error) {
		read, err := s.Read(ctx, target(a), apppissue.ReadOptions{})
		if err != nil {
			return nil, mapErr(issueResourceURI(a.Owner, a.Repo, int64(a.Index)), err)
		}
		return mcpresult.Render(read.Issue, nil)
	})
}

func (h handlers) ListRepoIssuesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListRepoIssuesFn", func(ctx context.Context, s *apppissue.Service, a listIssuesArgs) (*mcp.CallToolResult, error) {
		// A missing state is "open"; an explicit "" is passed on as given.
		state := "open"
		if a.State != nil {
			state = *a.State
		}
		return mcpresult.Render(s.List(ctx, apppissue.ListInput{
			Owner: a.Owner, Repo: a.Repo, State: state, Type: a.Type, Sort: a.Sort,
			Milestones: a.Milestones, Labels: a.Labels,
			Paging: apppissue.Paging{Page: pageOr(a.Page, 1), Limit: pageOr(a.Limit, 20)},
		}))
	})
}

func (h handlers) SearchIssuesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "SearchIssuesFn", func(ctx context.Context, s *apppissue.Service, a searchArgs) (*mcp.CallToolResult, error) {
		// Out-of-range page and limit values are corrected by the use case.
		return mcpresult.Render(s.Search(ctx, apppissue.SearchInput{
			Owner: a.Owner, State: a.State, Type: a.Type, Labels: a.Labels, Milestones: a.Milestones,
			Keyword: a.Q, CreatedBy: a.CreatedBy, AssignedBy: a.AssignedBy, MentionedBy: a.MentionedBy,
			Paging: apppissue.Paging{Page: int(a.Page), Limit: int(a.Limit)},
		}))
	})
}

func (h handlers) CreateIssueFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "CreateIssueFn", func(ctx context.Context, s *apppissue.Service, a createIssueArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Create(ctx, apppissue.CreateInput{
			Owner: a.Owner, Repo: a.Repo, Title: a.Title, Body: a.Body, Ref: a.Ref,
			Labels: a.Labels, Assignees: a.Assignees, Milestone: a.Milestone,
		}))
	})
}

func (h handlers) CreateIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "CreateIssueCommentFn", func(ctx context.Context, s *apppissue.Service, a createCommentArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.CreateComment(ctx, apppissue.Target{Owner: a.Owner, Repo: a.Repo, Index: int64(a.Index)}, a.Body))
	})
}

func (h handlers) UpdateIssueFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "UpdateIssueFn", func(ctx context.Context, s *apppissue.Service, a updateIssueArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Update(ctx, apppissue.UpdateInput{
			Target: target(issueRefArgs{Owner: a.Owner, Repo: a.Repo, Index: a.Index}),
			Title:  a.Title, Body: a.Body, Assignee: a.Assignee, Assignees: a.Assignees, Ref: a.Ref,
			Milestone: a.Milestone, DueDate: a.DueDate, ClearDueDate: a.ClearDueDate, SetLabels: a.SetLabels,
		}))
	})
}

func (h handlers) AddIssueLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "AddIssueLabelsFn", func(ctx context.Context, s *apppissue.Service, a labelsArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.AddLabels(ctx, target(issueRefArgs{Owner: a.Owner, Repo: a.Repo, Index: a.Index}), a.Labels))
	})
}

func (h handlers) RemoveIssueLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "RemoveIssueLabelsFn", func(ctx context.Context, s *apppissue.Service, a labelsArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.RemoveLabels(ctx, target(issueRefArgs{Owner: a.Owner, Repo: a.Repo, Index: a.Index}), a.Labels))
	})
}

func (h handlers) IssueStateChangeFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "IssueStateChangeFn", func(ctx context.Context, s *apppissue.Service, a stateArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.ChangeState(ctx, apppissue.Target{Owner: a.Owner, Repo: a.Repo, Index: int64(a.Index)}, a.State))
	})
}

func (h handlers) ListIssueCommentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListIssueCommentsFn", func(ctx context.Context, s *apppissue.Service, a listCommentsArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Comments(ctx, apppissue.CommentsInput{
			Target: target(issueRefArgs{Owner: a.Owner, Repo: a.Repo, Index: a.Index}),
			Since:  a.Since, Before: a.Before,
			Paging: apppissue.Paging{Page: pageOr(a.Page, 1), Limit: pageOr(a.Limit, 20)},
		}))
	})
}

func (h handlers) GetIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "GetIssueCommentFn", func(ctx context.Context, s *apppissue.Service, a commentArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Comment(ctx, a.Owner, a.Repo, int64(a.CommentID)))
	})
}

func (h handlers) EditIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "EditIssueCommentFn", func(ctx context.Context, s *apppissue.Service, a editCommentArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.EditComment(ctx, a.Owner, a.Repo, int64(a.CommentID), a.Body))
	})
}

func (h handlers) DeleteIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "DeleteIssueCommentFn", func(ctx context.Context, s *apppissue.Service, a commentArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render("Delete comment success", s.DeleteComment(ctx, a.Owner, a.Repo, int64(a.CommentID)))
	})
}

func (h handlers) ListRepoMilestonesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListRepoMilestonesFn", func(ctx context.Context, s *apppissue.Service, a milestonesArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Milestones(ctx, a.Owner, a.Repo, apppissue.MilestoneList{
			State:  a.State,
			Paging: apppissue.Paging{Page: pageOr(a.Page, 1), Limit: pageOr(a.Limit, 100)},
		}))
	})
}

func (h handlers) ListRepoLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListRepoLabelsFn", func(ctx context.Context, s *apppissue.Service, a repoLabelsArgs) (*mcp.CallToolResult, error) {
		includeOrg := true
		if a.IncludeOrg != nil {
			includeOrg = *a.IncludeOrg
		}
		return mcpresult.Render(s.RepoLabels(ctx, apppissue.RepoLabelsInput{
			Owner: a.Owner, Repo: a.Repo, IncludeOrg: includeOrg,
			Paging: apppissue.Paging{Page: pageOr(a.Page, 1), Limit: pageOr(a.Limit, 100)},
		}))
	})
}

func (h handlers) ListOrgLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListOrgLabelsFn", func(ctx context.Context, s *apppissue.Service, a orgLabelsArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.OrgLabels(ctx, a.Org, apppissue.Paging{Page: pageOr(a.Page, 1), Limit: pageOr(a.Limit, 100)}))
	})
}
