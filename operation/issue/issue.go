package issue

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// issueSortValues are the Forgejo API's valid `sort` values for
// GET /repos/{owner}/{repo}/issues (see routers/api/v1/repo/issue.go's
// ListIssues swagger comment). "nearduedate"/"farduedate" are what daikon#93
// needs for EDF selection; the SDK (codeberg.org/mvdkleijn/forgejo-sdk v3.0.0)
// has no Sort field on ListIssueOption, so this list is duplicated here
// rather than sourced from the SDK.
var issueSortValues = []string{
	"relevance", "latest", "oldest", "recentupdate", "leastupdate",
	"mostcomment", "leastcomment", "nearduedate", "farduedate",
}

func isValidIssueSort(s string) bool {
	for _, v := range issueSortValues {
		if s == v {
			return true
		}
	}
	return false
}

// refArgDescription is the shared description of the 'ref' argument on
// create_issue and update_issue. The value goes to Forgejo as given: this
// server neither normalises it nor checks that the branch exists.
const refArgDescription = "The branch this issue is linked to (Forgejo's issue 'ref' field, distinct from a pull request link). Passed to Forgejo as given; Forgejo decides which forms it accepts."

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

func (h handlers) GetIssueByIndexFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetIssueByIndexFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	issue, err := client.GetIssue(ctx, owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get issue err: %w", err))
	}
	return to.TextResult(issue)
}

func (h handlers) ListRepoIssuesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoIssuesFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	if owner == "" {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	if repo == "" {
		return to.ErrorResult(fmt.Errorf("repo is required; for owner-wide listing across repositories use search_issues instead"))
	}
	state, ok := req.GetArguments()["state"].(string)
	if !ok {
		state = "open"
	}
	issueType, _ := req.GetArguments()["type"].(string)
	milestones, _ := req.GetArguments()["milestones"].(string)
	labels, _ := req.GetArguments()["labels"].(string)
	sort, _ := req.GetArguments()["sort"].(string)
	page, _ := to.Float64(req.GetArguments()["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(req.GetArguments()["limit"])
	if limit == 0 {
		limit = 20
	}

	if sort != "" && !isValidIssueSort(sort) {
		return to.ErrorResult(fmt.Errorf("invalid sort %q: must be one of %s", sort, strings.Join(issueSortValues, ", ")))
	}

	// Create ListIssueOption according to the Forgejo API
	opt := forgejo_sdk.ListIssueOption{
		// State is correctly set directly
		State: forgejo_sdk.StateType(state),
		// ListOptions maps directly
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}

	// Set issue type if provided
	if issueType != "" {
		opt.Type = forgejo_sdk.IssueType(issueType)
	}

	// Set milestones if provided
	if milestones != "" {
		opt.Milestones = strings.Split(milestones, ",")
	}

	// Set labels if provided
	if labels != "" {
		opt.Labels = strings.Split(labels, ",")
	}

	// The adapter carries sort to Forgejo (the SDK cannot), see forge.ListIssuesOptions.
	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	issues, _, err := client.ListRepoIssues(ctx, owner, repo, forge.ListIssuesOptions{ListIssueOption: opt, Sort: sort})
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get issues list err: %w", err))
	}
	return to.TextResult(issues)
}

// searchIssuesEnvelope is the response shape for search_issues: the issues
// together with a continuation signal, per docs/design/output-bounding.md.
type searchIssuesEnvelope struct {
	Issues     []*forgejo_sdk.Issue `json:"issues"`
	Page       int                  `json:"page"`
	Limit      int                  `json:"limit"`
	Count      int                  `json:"count"`
	HasNext    bool                 `json:"has_next"`
	TotalCount *int                 `json:"total_count,omitempty"`
}

func (h handlers) SearchIssuesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called SearchIssuesFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	if owner == "" {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}

	state, ok := args["state"].(string)
	if !ok || state == "" {
		state = "open"
	}
	issueType, _ := args["type"].(string)
	labels, _ := args["labels"].(string)
	milestones, _ := args["milestones"].(string)
	keyword, _ := args["q"].(string)
	createdBy, _ := args["created_by"].(string)
	assignedBy, _ := args["assigned_by"].(string)
	mentionedBy, _ := args["mentioned_by"].(string)

	page, _ := to.Float64(args["page"])
	if page < 1 {
		page = 1
	}
	limit, _ := to.Float64(args["limit"])
	if limit < 1 {
		limit = 20
	}
	requestedLimit := int(limit)

	// Instance pagination ceiling (design.md Decision 3): known ceilings are
	// enforced by rejecting an over-ceiling request before the upstream call
	// rather than silently clamping it, so the response never claims a limit
	// larger than what was actually applied. An unknown ceiling (settings
	// endpoint unreachable/403) falls back to the same-limit next-page probe
	// below.
	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	ceiling, ceilingKnown := client.MaxResponseItems(ctx)
	if ceilingKnown && requestedLimit > ceiling {
		return to.ErrorResult(fmt.Errorf(
			"limit %d exceeds this instance's maximum of %d (max_response_items); retry with limit <= %d",
			requestedLimit, ceiling, ceiling,
		))
	}
	// effectiveLimit is the limit actually sent upstream. It feeds both the
	// SDK call and the response envelope from the same variable so the two
	// can never diverge; see the effective-limit reporting requirement in
	// specs/search-issues/spec.md.
	effectiveLimit := requestedLimit

	opt := forgejo_sdk.ListIssueOption{
		Owner: owner,
		State: forgejo_sdk.StateType(state),
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: effectiveLimit,
		},
	}
	if issueType != "" {
		opt.Type = forgejo_sdk.IssueType(issueType)
	}
	if labels != "" {
		opt.Labels = strings.Split(labels, ",")
	}
	if milestones != "" {
		opt.Milestones = strings.Split(milestones, ",")
	}
	if keyword != "" {
		opt.KeyWord = keyword
	}
	if createdBy != "" {
		opt.CreatedBy = createdBy
	}
	if assignedBy != "" {
		opt.AssignedBy = assignedBy
	}
	if mentionedBy != "" {
		opt.MentionedBy = mentionedBy
	}

	issues, info, err := client.SearchIssues(ctx, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("search issues err: %w", err))
	}
	count := len(issues)
	var totalCount *int
	if info.TotalKnown {
		totalCount = &info.TotalCount
	}

	// has_next (design.md Decision 3, revised): when the ceiling is known,
	// the rejection above guarantees effectiveLimit <= ceiling, so the
	// request was honored in full and the simple count == limit rule is
	// sound again — a full page may have more, a short page cannot. That
	// rule was unsound only because nothing clamped limit client-side; now
	// something does. When the ceiling is unknown, fall back to the
	// same-limit next-page probe: never limit+1, since Forgejo derives page
	// offsets from page size and a changed page size would make later pages
	// skip rows.
	var hasNext bool
	if ceilingKnown {
		hasNext = count == effectiveLimit
	} else if count > 0 {
		probeOpt := opt
		probeOpt.Page = int(page) + 1
		nextIssues, _, err := client.SearchIssues(ctx, probeOpt)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("probe next issues page err: %w", err))
		}
		hasNext = len(nextIssues) > 0
	}

	return to.TextResult(searchIssuesEnvelope{
		Issues:     issues,
		Page:       int(page),
		Limit:      effectiveLimit,
		Count:      count,
		HasNext:    hasNext,
		TotalCount: totalCount,
	})
}

func (h handlers) CreateIssueFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateIssueFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	title, _ := req.GetArguments()["title"].(string)
	body, _ := req.GetArguments()["body"].(string)
	labels, labelsProvided := req.GetArguments()["labels"].(string)
	assigneesRaw, assigneesProvided := req.GetArguments()["assignees"].(string)
	milestone, _ := req.GetArguments()["milestone"].(string)

	branchRef, _ := req.GetArguments()["ref"].(string)

	opt := forgejo_sdk.CreateIssueOption{
		Title: title,
		Body:  body,
		Ref:   branchRef,
	}
	// Labels resolve before the POST, not after it: a typo must not leave a
	// real unlabelled issue behind for someone to clean up.
	if labelsProvided {
		labelIDs, rerr := h.resolveLabelIDs(ctx, owner, repo, labels)
		if rerr != nil {
			return to.ErrorResult(rerr)
		}
		opt.Labels = labelIDs
	}
	if assigneesProvided {
		opt.Assignees = splitCSV(assigneesRaw)
	}
	if milestone != "" {
		milestoneID, merr := strconv.ParseInt(milestone, 10, 64)
		if merr != nil {
			return to.ErrorResult(fmt.Errorf("invalid milestone ID: %w", merr))
		}
		opt.Milestone = milestoneID
	}

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	issue, err := client.CreateIssue(ctx, owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create issue err: %w", err))
	}
	return to.TextResult(issue)
}

func (h handlers) CreateIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateIssueCommentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	body, _ := req.GetArguments()["body"].(string)

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	comment, err := client.CreateIssueComment(ctx, owner, repo, int64(index), body)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create issue comment err: %w", err))
	}
	return to.TextResult(comment)
}

func (h handlers) UpdateIssueFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called UpdateIssueFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	title, _ := req.GetArguments()["title"].(string)
	body, _ := req.GetArguments()["body"].(string)
	assignee, _ := req.GetArguments()["assignee"].(string)
	assigneesRaw, assigneesProvided := req.GetArguments()["assignees"].(string)
	milestone, _ := req.GetArguments()["milestone"].(string)
	dueDate, _ := req.GetArguments()["due_date"].(string)
	clearDueDate, _ := req.GetArguments()["clear_due_date"].(bool)
	setLabelsRaw, setLabelsProvided := req.GetArguments()["set_labels"].(string)
	branchRef, branchRefProvided := req.GetArguments()["ref"].(string)

	// The Forgejo edit-issue endpoint has no labels field and ignores one if
	// sent, so set_labels is a separate request (PUT to replace, DELETE to
	// clear) rather than part of this PATCH. patchRequested keeps a labels-only
	// update from sending an empty PATCH, which would move updated_at for
	// nothing; with no set_labels at all the handler behaves exactly as before.
	patchRequested := title != "" || body != "" || assigneesProvided || assignee != "" ||
		milestone != "" || dueDate != "" || clearDueDate || branchRefProvided

	opt := forgejo_sdk.EditIssueOption{}

	// An explicitly provided empty 'ref' clears the branch link (sent as "");
	// an omitted one stays nil and leaves it unchanged.
	if branchRefProvided {
		opt.Ref = &branchRef
	}

	// Only set fields that were provided
	if title != "" {
		opt.Title = title
	}
	if body != "" {
		opt.Body = &body
	}
	// Assignees: 'assignees' (CSV) wins if provided; otherwise fall back to singular 'assignee'.
	// An explicitly provided empty 'assignees' clears all assignees (sent as []).
	switch {
	case assigneesProvided:
		opt.Assignees = splitCSV(assigneesRaw)
	case assignee != "":
		opt.Assignees = []string{assignee}
	}
	if milestone != "" {
		milestoneID, err := strconv.ParseInt(milestone, 10, 64)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("invalid milestone ID: %w", err))
		}
		opt.Milestone = &milestoneID
	}
	// due_date / clear_due_date map to the Forgejo API's due_date / unset_due_date
	// EditIssueOption fields (routers/api/v1/repo/issue.go's EditIssue: setting
	// unset_due_date=true clears the deadline; due_date sets it; both absent
	// leaves it unchanged). Mutually exclusive — ambiguous intent otherwise.
	switch {
	case clearDueDate && dueDate != "":
		return to.ErrorResult(fmt.Errorf("cannot set 'due_date' and 'clear_due_date' at the same time"))
	case clearDueDate:
		remove := true
		opt.RemoveDeadline = &remove
	case dueDate != "":
		parsed, err := time.Parse(time.RFC3339, dueDate)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("invalid due_date format (expected RFC3339): %w", err))
		}
		opt.Deadline = &parsed
	}

	// Resolve before either request so an unknown label name leaves the issue
	// untouched instead of half-updated.
	var setLabelIDs []int64
	clearLabels := false
	if setLabelsProvided {
		if len(splitCSV(setLabelsRaw)) == 0 {
			clearLabels = true
		} else {
			ids, rerr := h.resolveLabelIDs(ctx, owner, repo, setLabelsRaw)
			if rerr != nil {
				return to.ErrorResult(rerr)
			}
			setLabelIDs = ids
		}
	}

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}

	var issue *forgejo_sdk.Issue
	if patchRequested || !setLabelsProvided {
		issue, err = client.EditIssue(ctx, owner, repo, int64(index), opt)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("update issue err: %w", err))
		}
	}

	if setLabelsProvided {
		if clearLabels {
			if cerr := client.ClearIssueLabels(ctx, owner, repo, int64(index)); cerr != nil {
				return to.ErrorResult(fmt.Errorf("clear issue labels err: %w", cerr))
			}
		} else if rerr := client.ReplaceIssueLabels(ctx, owner, repo, int64(index), setLabelIDs); rerr != nil {
			return to.ErrorResult(fmt.Errorf("replace issue labels err: %w", rerr))
		}
		// Re-read: any issue returned by the PATCH above predates the label
		// change, so returning it would report the old label set.
		issue, err = client.GetIssue(ctx, owner, repo, int64(index))
		if err != nil {
			return to.ErrorResult(fmt.Errorf("get updated issue err: %w", err))
		}
	}

	return to.TextResult(issue)
}

func (h handlers) IssueStateChangeFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called IssueStateChangeFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	state, _ := req.GetArguments()["state"].(string)

	if state != "open" && state != "closed" {
		return to.ErrorResult(fmt.Errorf("invalid state: %s, must be 'open' or 'closed'", state))
	}

	// Convert string to StateType and create pointer
	stateType := forgejo_sdk.StateType(state)

	opt := forgejo_sdk.EditIssueOption{
		State: &stateType,
	}

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	issue, err := client.EditIssue(ctx, owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("change issue state err: %w", err))
	}
	return to.TextResult(issue)
}

func (h handlers) ListIssueCommentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListIssueCommentsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	since, _ := req.GetArguments()["since"].(string)
	before, _ := req.GetArguments()["before"].(string)
	page, _ := to.Float64(req.GetArguments()["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(req.GetArguments()["limit"])
	if limit == 0 {
		limit = 20
	}

	opt := forgejo_sdk.ListIssueCommentOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}

	// Set time filters if provided
	if since != "" {
		sinceTime, err := time.Parse(time.RFC3339, since)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("invalid since time format (expected RFC3339): %w", err))
		}
		opt.Since = sinceTime
	}
	if before != "" {
		beforeTime, err := time.Parse(time.RFC3339, before)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("invalid before time format (expected RFC3339): %w", err))
		}
		opt.Before = beforeTime
	}

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	comments, _, err := client.ListIssueComments(ctx, owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list issue comments err: %w", err))
	}
	return to.TextResult(comments)
}

func (h handlers) GetIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetIssueCommentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	commentID, _ := to.Float64(req.GetArguments()["comment_id"])

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	comment, err := client.GetIssueComment(ctx, owner, repo, int64(commentID))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get issue comment err: %w", err))
	}
	return to.TextResult(comment)
}

func (h handlers) EditIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditIssueCommentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	commentID, _ := to.Float64(req.GetArguments()["comment_id"])
	body, _ := req.GetArguments()["body"].(string)

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	comment, err := client.EditIssueComment(ctx, owner, repo, int64(commentID), body)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("edit issue comment err: %w", err))
	}
	return to.TextResult(comment)
}

func (h handlers) DeleteIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteIssueCommentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	commentID, _ := to.Float64(req.GetArguments()["comment_id"])

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	err = client.DeleteIssueComment(ctx, owner, repo, int64(commentID))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("delete issue comment err: %w", err))
	}
	return to.TextResult("Delete comment success")
}

func (h handlers) ListRepoMilestonesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoMilestonesFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	state, ok := req.GetArguments()["state"].(string)
	if !ok || state == "" {
		state = "open"
	}
	page, _ := to.Float64(req.GetArguments()["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(req.GetArguments()["limit"])
	if limit == 0 {
		limit = 100
	}

	opt := forgejo_sdk.ListMilestoneOption{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
		State: forgejo_sdk.StateType(state),
	}

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	milestones, err := client.ListRepoMilestones(ctx, owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list repo milestones err: %w", err))
	}
	return to.TextResult(milestones)
}

// handlers serve the issue tools. They hold the factory rather than a port so
// each request gets the port for its own credential.
type handlers struct {
	forge forge.Factory
	// resolveLabelIDs turns a caller's label names or IDs into label IDs. It
	// defaults to resolveIssueLabelIDs, which reads the catalogue through the
	// port; tests may replace it.
	resolveLabelIDs func(ctx context.Context, owner, repo, csv string) ([]int64, error)
}

func newHandlers(f forge.Factory) handlers {
	h := handlers{forge: f}
	h.resolveLabelIDs = h.resolveIssueLabelIDs
	return h
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
