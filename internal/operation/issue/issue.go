// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"errors"
	"fmt"

	apppissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/lean"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/compact"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/pipeline"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

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
	GetIssueByIndexToolName      = "get_issue_by_index"
	ListRepoIssuesToolName       = "list_repo_issues"
	CreateIssueToolName          = "create_issue"
	CreateIssueCommentToolName   = "create_issue_comment"
	UpdateIssueToolName          = "update_issue"
	ReplaceIssueBodyLineToolName = "replace_issue_body_line"
	AddIssueLabelsToolName       = "add_issue_labels"
	RemoveIssueLabelsToolName    = "remove_issue_labels"
	IssueStateChangeToolName     = "issue_state_change"
	ListIssueCommentsToolName    = "list_issue_comments"
	GetIssueCommentToolName      = "get_issue_comment"
	EditIssueCommentToolName     = "edit_issue_comment"
	DeleteIssueCommentToolName   = "delete_issue_comment"
	ListRepoMilestonesToolName   = "list_repo_milestones"
	ListRepoLabelsToolName       = "list_repo_labels"
	ListOrgLabelsToolName        = "list_org_labels"
	SearchIssuesToolName         = "search_issues"
)

var (
	GetIssueByIndexTool = mcp.NewTool(
		GetIssueByIndexToolName,
		annotate.Read(),
		mcp.WithDescription("Get issue by index"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithString("type", mcp.Description("Type (issues|pulls). 'issues' refuses a pull request's index and 'pulls' refuses an issue's index, with an error that names the tool for the other kind. Omit to accept either kind."), lean.Keep("issues or pulls; omit to accept either")),
		mcp.WithBoolean("verbose", mcp.Description(compact.VerboseDescription)),
	)

	ListRepoIssuesTool = mcp.NewTool(
		ListRepoIssuesToolName,
		annotate.Read(),
		mcp.WithDescription("List repo issues (page/limit). Returns a response envelope {issues, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("state", mcp.Description("State (open|closed|all)"), mcp.DefaultString("open"), lean.Keep("open, closed or all")),
		mcp.WithString("type", mcp.Description("Type (issues|pulls)"), lean.Keep("issues or pulls")),
		mcp.WithString("milestones", mcp.Description("Milestone names/IDs (comma-separated)"), lean.Keep("comma-separated milestone names or IDs")),
		mcp.WithString("labels", mcp.Description("Labels (comma-separated)"), lean.Keep("comma-separated label names or IDs")),
		mcp.WithString("sort", mcp.Description("Server-side sort order. One of: relevance, latest, oldest, recentupdate, leastupdate, mostcomment, leastcomment, nearduedate, farduedate. Default is the API's own default (latest)."), lean.Keep("one of relevance, latest, oldest, recentupdate, leastupdate, mostcomment, leastcomment, nearduedate, farduedate")),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
		mcp.WithString("outFile", mcp.Description("Bare file name. Writes the whole listing (every page) as a JSON array into the forgejo-mcp temp dir and replies {path,count,keys}; an existing file is overwritten. Refused with page or limit, and on the sse and http transports."), lean.Keep("bare file name; writes the whole listing to a file, refused with page or limit")),
		mcp.WithBoolean("verbose", mcp.Description(compact.VerboseDescription)),
	)

	CreateIssueTool = mcp.NewTool(
		CreateIssueToolName,
		annotate.Write(),
		mcp.WithDescription("Create issue. Labels (by name or ID), assignees and a milestone can be set in the same call."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("title", mcp.Required(), mcp.Description(params.Title)),
		mcp.WithString("body", mcp.Description(params.Body)),
		mcp.WithString("labels", mcp.Description(labelsArgDescription("Labels for the new issue")), lean.Keep("comma-separated label names or IDs")),
		mcp.WithString("assignees", mcp.Description("Assignee usernames (comma-separated)"), lean.Keep("comma-separated usernames")),
		mcp.WithString("milestone", mcp.Description(params.Milestone)),
		mcp.WithString("ref", mcp.Description(refArgDescription), lean.Keep("branch name")),
		mcp.WithBoolean("verbose", mcp.Description(compact.VerboseDescription)),
	)

	CreateIssueCommentTool = mcp.NewTool(
		CreateIssueCommentToolName,
		annotate.Write(),
		mcp.WithDescription("Create issue comment"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
		mcp.WithString("body", mcp.Required(), mcp.Description(params.Body)),
		mcp.WithBoolean("verbose", mcp.Description(compact.VerboseDescription)),
	)

	UpdateIssueTool = mcp.NewTool(
		UpdateIssueToolName,
		annotate.Write(),
		mcp.WithDescription("Update issue. `set_labels` replaces the issue's whole label set; use add_issue_labels or remove_issue_labels to change it incrementally."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithString("title", mcp.Description(params.Title)),
		mcp.WithString("body", mcp.Description(params.Body)),
		mcp.WithString("assignee", mcp.Description("Assignee username (convenience for a single user; equivalent to a one-element 'assignees')")),
		mcp.WithString("assignees", mcp.Description("Assignee usernames (comma-separated). Overrides 'assignee' if both are set. Pass an empty string to clear all assignees."), lean.Keep("comma-separated usernames, empty clears them")),
		mcp.WithString("milestone", mcp.Description(params.Milestone)),
		mcp.WithString("due_date", mcp.Description("Set the issue's due date (RFC3339, e.g. 2026-08-20T00:00:00Z). Mutually exclusive with 'clear_due_date'; setting both is an error."), lean.Keep("RFC3339 time, e.g. 2026-08-20T00:00:00Z")),
		mcp.WithBoolean("clear_due_date", mcp.Description("Clear the issue's due date. Mutually exclusive with 'due_date'."), lean.Keep("true clears the due date, exclusive with due_date")),
		mcp.WithString("set_labels", mcp.Description(labelsArgDescription("Replace the issue's entire label set")+" Pass an empty string to remove every label. To add or drop individual labels without restating the set, use add_issue_labels or remove_issue_labels."), lean.Keep("comma-separated label names or IDs, empty removes all")),
		mcp.WithString("ref", mcp.Description(refArgDescription+" Omit to leave it unchanged; pass an empty string to clear it."), lean.Keep("branch name, empty clears it")),
		mcp.WithBoolean("verbose", mcp.Description(compact.VerboseDescription)),
	)

	ReplaceIssueBodyLineTool = mcp.NewTool(
		ReplaceIssueBodyLineToolName,
		annotate.Write(),
		mcp.WithDescription("Replace one exact piece of an issue's body, without sending the whole body. `old` is matched as a case-sensitive substring (it may span a whole line) and must occur exactly once: zero matches or several refuse with the body left unchanged, and several report the count. `old` must not be empty. Pull request indexes are refused. Limitation: Forgejo has no compare-and-swap for issue edits, so an edit made between this call's read and write can be lost; compare updated_at in the result to detect that."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithString("old", mcp.Required(), mcp.Description("Exact text to find in the body (case-sensitive, non-empty). Must occur exactly once."), lean.Keep("must occur exactly once in the body")),
		mcp.WithString("new", mcp.Required(), mcp.Description("Text that replaces 'old'. May be empty to delete it."), lean.Keep("empty deletes the match")),
	)

	AddIssueLabelsTools = mcp.NewTool(
		AddIssueLabelsToolName,
		annotate.Write(),
		mcp.WithDescription("Add labels to issue, by label name or numeric ID"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithString("labels", mcp.Required(), mcp.Description(labelsArgDescription("Labels to add")), lean.Keep("comma-separated label names or IDs")),
		mcp.WithBoolean("verbose", mcp.Description(compact.VerboseDescription)),
	)

	RemoveIssueLabelsTools = mcp.NewTool(
		RemoveIssueLabelsToolName,
		annotate.Destructive(),
		mcp.WithDescription("Remove labels from issue, by label name or numeric ID"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithString("labels", mcp.Required(), mcp.Description(labelsArgDescription("Labels to remove")), lean.Keep("comma-separated label names or IDs")),
		mcp.WithBoolean("verbose", mcp.Description(compact.VerboseDescription)),
	)

	IssueStateChangeTool = mcp.NewTool(
		IssueStateChangeToolName,
		annotate.Write(),
		mcp.WithDescription("Change issue state"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithString("state", mcp.Required(), mcp.Description("State (open|closed)"), lean.Keep("open or closed")),
		mcp.WithBoolean("verbose", mcp.Description(compact.VerboseDescription)),
	)

	ListIssueCommentsTool = mcp.NewTool(
		ListIssueCommentsToolName,
		annotate.Read(),
		mcp.WithDescription("List issue/PR comments. Returns a response envelope {comments, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
		mcp.WithString("since", mcp.Description(params.Since), lean.Keep("RFC3339 time, e.g. 2026-08-20T00:00:00Z")),
		mcp.WithString("before", mcp.Description(params.Before), lean.Keep("RFC3339 time, e.g. 2026-08-20T00:00:00Z")),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	GetIssueCommentTool = mcp.NewTool(
		GetIssueCommentToolName,
		annotate.Read(),
		mcp.WithDescription("Get comment by ID"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
		mcp.WithBoolean("verbose", mcp.Description(compact.VerboseDescription)),
	)

	EditIssueCommentTool = mcp.NewTool(
		EditIssueCommentToolName,
		annotate.Write(),
		mcp.WithDescription("Edit issue/PR comment"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
		mcp.WithString("body", mcp.Required(), mcp.Description(params.Body)),
		mcp.WithBoolean("verbose", mcp.Description(compact.VerboseDescription)),
	)

	DeleteIssueCommentTool = mcp.NewTool(
		DeleteIssueCommentToolName,
		annotate.Destructive(),
		mcp.WithDescription("Delete issue/PR comment"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
	)

	ListRepoMilestonesTool = mcp.NewTool(
		ListRepoMilestonesToolName,
		annotate.Read(),
		mcp.WithDescription("List repository milestones. Returns a response envelope {milestones, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
		mcp.WithString("state", mcp.Description("Milestone state (open|closed|all)"), mcp.DefaultString("open"), lean.Keep("open, closed or all")),
	)

	ListRepoLabelsTool = mcp.NewTool(
		ListRepoLabelsToolName,
		annotate.Read(),
		mcp.WithDescription("List repository labels. Returns a response envelope {labels, page, limit, count, has_next, total_count}; has_next true means a further page may exist; re-issue the call with page incremented to fetch it. total_count is the server grand total, omitted when unknown. When the owner is an organization and include_org_labels is true (default), org-level labels are merged into the labels list; count includes them, while has_next and total_count describe the repository labels only. Each label carries a scope field of \"repo\" or \"org\"."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
		mcp.WithBoolean("include_org_labels", mcp.Description("Merge org-level labels into the response when the owner is an organization. Default true."), mcp.DefaultBool(true)),
	)

	ListOrgLabelsTool = mcp.NewTool(
		ListOrgLabelsToolName,
		annotate.Read(),
		mcp.WithDescription("List organization-level labels. Returns a response envelope {labels, page, limit, count, has_next, total_count}; has_next true means a further page may exist; re-issue the call with page incremented to fetch it. total_count is the server grand total, omitted when unknown. Each label carries a scope field of \"org\"."),
		mcp.WithString("org", mcp.Required(), mcp.Description("Organization name")),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
	)

	SearchIssuesTool = mcp.NewTool(
		SearchIssuesToolName,
		annotate.Read(),
		mcp.WithDescription("Search issues across one owner's repositories. No repo is named; the owner may be an organization or a user. "+
			"Returns a response envelope {issues, page, limit, count, has_next, total_count} rather than a bare array. "+
			"has_next true means a further page may exist; re-issue the call with page incremented to fetch it. "+
			"total_count, when present, is the total number of matching issues across all pages (from Forgejo's X-Total-Count header); it is omitted rather than 0 when the header is unavailable. "+
			"Each issue identifies its source repository only via its html_url/url field; there is no separate repository field. "+
			"Use list_repo_issues instead when the repo is already known."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("state", mcp.Description("State (open|closed|all)"), mcp.DefaultString("open"), lean.Keep("open, closed or all")),
		mcp.WithString("type", mcp.Description("Type (issues|pulls)"), lean.Keep("issues or pulls")),
		mcp.WithString("labels", mcp.Description("Labels (comma-separated); OR semantics — an issue matching any listed label is returned"), lean.Keep("comma-separated, an issue with any listed label matches")),
		mcp.WithString("milestones", mcp.Description("Milestone names/IDs (comma-separated)"), lean.Keep("comma-separated milestone names or IDs")),
		mcp.WithString("q", mcp.Description(params.Keyword+" (matches issue title, body, and comments)")),
		mcp.WithString("created_by", mcp.Description("Filter by creator username")),
		mcp.WithString("assigned_by", mcp.Description("Filter by assignee username")),
		mcp.WithString("mentioned_by", mcp.Description("Filter by mentioned username")),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
		mcp.WithBoolean("verbose", mcp.Description(compact.VerboseDescription)),
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
	s.AddTool(ReplaceIssueBodyLineTool, h.ReplaceIssueBodyLineFn)
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
	h.registerMilestoneTools(s)
	h.registerDependencyTools(s)
}

// Argument structs, one per tool. `json` names the MCP argument and
// `required:"true"` marks the required ones; mcptest.ArgsMatchSchema keeps
// them in agreement with the tool definitions. A pointer field is an argument
// whose presence matters: nil when it was left out (or given with the wrong
// type), set when it was given, even as "".

type issueRefArgs struct {
	Owner string
	Repo  string
	Index int64 // validated by numarg before the use case sees it
}

type getIssueArgs struct {
	Owner   string `json:"owner" required:"true"`
	Repo    string `json:"repo" required:"true"`
	Index   any    `json:"index" required:"true" schema:"number"`
	idx     int64
	Type    string `json:"type"`
	Verbose bool   `json:"verbose"`
}

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
	// OutFile is a pointer so an explicit empty name is told from no name.
	OutFile *string `json:"outFile"`
	Verbose bool    `json:"verbose"`
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
	Verbose   bool    `json:"verbose"`
}

type createCommentArgs struct {
	Owner   string `json:"owner" required:"true"`
	Repo    string `json:"repo" required:"true"`
	Index   any    `json:"index" required:"true" schema:"number"`
	idx     int64
	Body    string `json:"body" required:"true"`
	Verbose bool   `json:"verbose"`
}

type updateIssueArgs struct {
	Owner        string `json:"owner" required:"true"`
	Repo         string `json:"repo" required:"true"`
	Index        any    `json:"index" required:"true" schema:"number"`
	idx          int64
	Title        string  `json:"title"`
	Body         string  `json:"body"`
	Assignee     string  `json:"assignee"`
	Assignees    *string `json:"assignees"`
	Milestone    string  `json:"milestone"`
	DueDate      string  `json:"due_date"`
	ClearDueDate bool    `json:"clear_due_date"`
	SetLabels    *string `json:"set_labels"`
	Ref          *string `json:"ref"`
	Verbose      bool    `json:"verbose"`
}

type replaceBodyLineArgs struct {
	Owner string `json:"owner" required:"true"`
	Repo  string `json:"repo" required:"true"`
	Index any    `json:"index" required:"true" schema:"number"`
	idx   int64
	Old   string `json:"old" required:"true"`
	New   string `json:"new" required:"true"`
}

// replacedBodyLine is the result of replace_issue_body_line: the compact issue
// write, with the body hash taken from the stored body, plus the text that was
// replaced and its replacement.
type replacedBodyLine struct {
	compact.IssueWrite
	Old string `json:"old"`
	New string `json:"new"`
}

type labelsArgs struct {
	Owner   string `json:"owner" required:"true"`
	Repo    string `json:"repo" required:"true"`
	Index   any    `json:"index" required:"true" schema:"number"`
	idx     int64
	Labels  string `json:"labels" required:"true"`
	Verbose bool   `json:"verbose"`
}

type stateArgs struct {
	Owner   string `json:"owner" required:"true"`
	Repo    string `json:"repo" required:"true"`
	Index   any    `json:"index" required:"true" schema:"number"`
	idx     int64
	State   string `json:"state" required:"true"`
	Verbose bool   `json:"verbose"`
}

type listCommentsArgs struct {
	Owner  string `json:"owner" required:"true"`
	Repo   string `json:"repo" required:"true"`
	Index  any    `json:"index" required:"true" schema:"number"`
	idx    int64
	Since  string  `json:"since"`
	Before string  `json:"before"`
	Page   float64 `json:"page"`
	Limit  float64 `json:"limit"`
}

type commentArgs struct {
	Owner     string `json:"owner" required:"true"`
	Repo      string `json:"repo" required:"true"`
	CommentID any    `json:"comment_id" required:"true" schema:"number"`
	commentID int64
}

type getCommentArgs struct {
	Owner     string `json:"owner" required:"true"`
	Repo      string `json:"repo" required:"true"`
	CommentID any    `json:"comment_id" required:"true" schema:"number"`
	commentID int64
	Verbose   bool `json:"verbose"`
}

type editCommentArgs struct {
	Owner     string `json:"owner" required:"true"`
	Repo      string `json:"repo" required:"true"`
	CommentID any    `json:"comment_id" required:"true" schema:"number"`
	commentID int64
	Body      string `json:"body" required:"true"`
	Verbose   bool   `json:"verbose"`
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
	Verbose     bool    `json:"verbose"`
}

// target is the issue a call names.
func target(a issueRefArgs) apppissue.Target {
	return apppissue.Target{Owner: a.Owner, Repo: a.Repo, Index: a.Index}
}

// kindRefusal names the tool for the other kind, so a caller that asked for
// issues and got a pull request knows where to read it.
func kindRefusal(e *apppissue.KindError) error {
	if e.Want == apppissue.KindPulls {
		return fmt.Errorf("%s; use get_issue_by_index to read it", e.Error())
	}
	return fmt.Errorf("%s; use get_pull_request_by_index to read it", e.Error())
}

func (h handlers) GetIssueByIndexFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a getIssueArgs) (*mcp.CallToolResult, error) {
		kind, err := apppissue.ParseKind(a.Type)
		if err != nil {
			return mcpresult.Fail(err)
		}
		read, err := s.Read(ctx, target(issueRefArgs{Owner: a.Owner, Repo: a.Repo, Index: a.idx}), apppissue.ReadOptions{Kind: kind})
		if err != nil {
			var refused *apppissue.KindError
			if errors.As(err, &refused) {
				return mcpresult.Fail(kindRefusal(refused))
			}
			return nil, mapErr(issueResourceURI(a.Owner, a.Repo, a.idx), err)
		}
		return compact.RenderIssueRead(read.Issue, a.Verbose)
	})(ctx, req)
}

func (h handlers) ListRepoIssuesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a listIssuesArgs) (*mcp.CallToolResult, error) {
		// A missing state is "open"; an explicit "" is passed on as given.
		state := "open"
		if a.State != nil {
			state = *a.State
		}
		in := apppissue.ListInput{
			Owner: a.Owner, Repo: a.Repo, State: state, Type: a.Type, Sort: a.Sort,
			Milestones: a.Milestones, Labels: a.Labels,
		}
		if a.OutFile != nil {
			return listToFile(ctx, s, in, a.Verbose, *a.OutFile)
		}
		pg, lim, perr := decode.PagingArgs(a.Page, a.Limit)
		if perr != nil {
			return to.ErrorResult(perr)
		}
		in.Paging = apppissue.Paging{Page: pg, Limit: lim}
		page, err := s.List(ctx, in)
		envelope, err := issueListPage(page, err)
		return compact.RenderIssueList(envelope, err, a.Verbose)
	})(ctx, req)
}

func (h handlers) SearchIssuesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a searchArgs) (*mcp.CallToolResult, error) {
		pg, lim, perr := decode.PagingArgs(a.Page, a.Limit)
		if perr != nil {
			return to.ErrorResult(perr)
		}
		page, err := s.Search(ctx, apppissue.SearchInput{
			Owner: a.Owner, State: a.State, Type: a.Type, Labels: a.Labels, Milestones: a.Milestones,
			Keyword: a.Q, CreatedBy: a.CreatedBy, AssignedBy: a.AssignedBy, MentionedBy: a.MentionedBy,
			Paging: apppissue.Paging{Page: pg, Limit: lim},
		})
		envelope, err := issueSearchPage(page, err)
		return compact.RenderIssueSearchList(envelope, err, a.Verbose)
	})(ctx, req)
}

func (h handlers) CreateIssueFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a createIssueArgs) (*mcp.CallToolResult, error) {
		iss, err := s.Create(ctx, apppissue.CreateInput{
			Owner: a.Owner, Repo: a.Repo, Title: a.Title, Body: a.Body, Ref: a.Ref,
			Labels: a.Labels, Assignees: a.Assignees, Milestone: a.Milestone,
		})
		return compact.RenderIssueWrite(iss, err, a.Verbose, a.Body != "")
	})(ctx, req)
}

func (h handlers) CreateIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a createCommentArgs) (*mcp.CallToolResult, error) {
		c, err := s.CreateComment(ctx, apppissue.Target{Owner: a.Owner, Repo: a.Repo, Index: a.idx}, a.Body)
		return compact.RenderComment(c, err, a.Verbose)
	})(ctx, req)
}

func (h handlers) UpdateIssueFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a updateIssueArgs) (*mcp.CallToolResult, error) {
		iss, err := s.Update(ctx, apppissue.UpdateInput{
			Target: target(issueRefArgs{Owner: a.Owner, Repo: a.Repo, Index: a.idx}),
			Title:  a.Title, Body: a.Body, Assignee: a.Assignee, Assignees: a.Assignees, Ref: a.Ref,
			Milestone: a.Milestone, DueDate: a.DueDate, ClearDueDate: a.ClearDueDate, SetLabels: a.SetLabels,
		})
		return compact.RenderIssueWrite(iss, err, a.Verbose, a.Body != "")
	})(ctx, req)
}

func (h handlers) ReplaceIssueBodyLineFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a replaceBodyLineArgs) (*mcp.CallToolResult, error) {
		iss, err := s.ReplaceBodyLine(ctx, apppissue.ReplaceBodyLineInput{
			Target: target(issueRefArgs{Owner: a.Owner, Repo: a.Repo, Index: a.idx}),
			Old:    a.Old, New: a.New,
		})
		if err != nil {
			return mcpresult.Fail(err)
		}
		return mcpresult.Render(replacedBodyLine{
			IssueWrite: compact.IssueWrite{Issue: compact.OfIssue(iss), BodySHA256: compact.BodySHA256(iss.Body)},
			Old:        a.Old,
			New:        a.New,
		}, nil)
	})(ctx, req)
}

func (h handlers) AddIssueLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a labelsArgs) (*mcp.CallToolResult, error) {
		iss, err := s.AddLabels(ctx, target(issueRefArgs{Owner: a.Owner, Repo: a.Repo, Index: a.idx}), a.Labels)
		return compact.RenderIssue(iss, err, a.Verbose)
	})(ctx, req)
}

func (h handlers) RemoveIssueLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a labelsArgs) (*mcp.CallToolResult, error) {
		iss, err := s.RemoveLabels(ctx, target(issueRefArgs{Owner: a.Owner, Repo: a.Repo, Index: a.idx}), a.Labels)
		return compact.RenderIssue(iss, err, a.Verbose)
	})(ctx, req)
}

func (h handlers) IssueStateChangeFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a stateArgs) (*mcp.CallToolResult, error) {
		iss, err := s.ChangeState(ctx, apppissue.Target{Owner: a.Owner, Repo: a.Repo, Index: a.idx}, a.State)
		return compact.RenderIssue(iss, err, a.Verbose)
	})(ctx, req)
}

func (h handlers) ListIssueCommentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a listCommentsArgs) (*mcp.CallToolResult, error) {
		pg, lim, perr := decode.PagingArgs(a.Page, a.Limit)
		if perr != nil {
			return to.ErrorResult(perr)
		}
		return mcpresult.Render(commentPage(s.Comments(ctx, apppissue.CommentsInput{
			Target: target(issueRefArgs{Owner: a.Owner, Repo: a.Repo, Index: a.idx}),
			Since:  a.Since, Before: a.Before,
			Paging: apppissue.Paging{Page: pg, Limit: lim},
		})))
	})(ctx, req)
}

func (h handlers) GetIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a getCommentArgs) (*mcp.CallToolResult, error) {
		c, err := s.Comment(ctx, a.Owner, a.Repo, a.commentID)
		return compact.RenderCommentRead(c, err, a.Verbose)
	})(ctx, req)
}

func (h handlers) EditIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a editCommentArgs) (*mcp.CallToolResult, error) {
		c, err := s.EditComment(ctx, a.Owner, a.Repo, a.commentID, a.Body)
		return compact.RenderComment(c, err, a.Verbose)
	})(ctx, req)
}

func (h handlers) DeleteIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a commentArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render("Delete comment success", s.DeleteComment(ctx, a.Owner, a.Repo, a.commentID))
	})(ctx, req)
}

func (h handlers) ListRepoMilestonesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a milestonesArgs) (*mcp.CallToolResult, error) {
		pg, lim, perr := decode.PagingArgs(a.Page, a.Limit)
		if perr != nil {
			return to.ErrorResult(perr)
		}
		return mcpresult.Render(milestonePage(s.Milestones(ctx, a.Owner, a.Repo, apppissue.MilestoneList{
			State:  a.State,
			Paging: apppissue.Paging{Page: pg, Limit: lim},
		})))
	})(ctx, req)
}

func (h handlers) ListRepoLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a repoLabelsArgs) (*mcp.CallToolResult, error) {
		includeOrg := true
		if a.IncludeOrg != nil {
			includeOrg = *a.IncludeOrg
		}
		pg, lim, perr := decode.PagingArgs(a.Page, a.Limit)
		if perr != nil {
			return to.ErrorResult(perr)
		}
		list, err := s.ReadRepoLabels(ctx, apppissue.RepoLabelsInput{
			Owner: a.Owner, Repo: a.Repo, IncludeOrg: includeOrg,
			Paging: apppissue.Paging{Page: pg, Limit: lim},
		})
		if err != nil {
			return nil, mapErr(repoLabelsURI(a.Owner, a.Repo), err)
		}
		return mcpresult.Render(scopedLabelPage(list), nil)
	})(ctx, req)
}

func (h handlers) ListOrgLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.service, func(ctx context.Context, s *apppissue.Service, a orgLabelsArgs) (*mcp.CallToolResult, error) {
		pg, lim, perr := decode.PagingArgs(a.Page, a.Limit)
		if perr != nil {
			return to.ErrorResult(perr)
		}
		list, err := s.ReadOrgLabels(ctx, a.Org, apppissue.Paging{Page: pg, Limit: lim})
		if err != nil {
			return nil, mapErr(orgLabelsURI(a.Org), err)
		}
		return mcpresult.Render(scopedLabelPage(list), nil)
	})(ctx, req)
}
