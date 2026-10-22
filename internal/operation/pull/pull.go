// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/compact"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	GetPullRequestByIndexToolName  = "get_pull_request_by_index"
	ListRepoPullRequestsToolName   = "list_repo_pull_requests"
	CreatePullRequestToolName      = "create_pull_request"
	UpdatePullRequestToolName      = "update_pull_request"
	ListPullReviewsToolName        = "list_pull_reviews"
	GetPullReviewToolName          = "get_pull_review"
	ListPullReviewCommentsToolName = "list_pull_review_comments"
	MergePullRequestToolName       = "merge_pull_request"
	ListPullRequestFilesToolName   = "list_pull_request_files"
	GetPullRequestDiffToolName     = "get_pull_request_diff"
)

var (
	GetPullRequestByIndexTool = mcp.NewTool(
		GetPullRequestByIndexToolName,
		mcp.WithDescription("Get pull request by index"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithBoolean("verbose", mcp.Description(compact.VerboseDescription)),
	)

	ListRepoPullRequestsTool = mcp.NewTool(
		ListRepoPullRequestsToolName,
		mcp.WithDescription("List repo pull requests. Returns a response envelope {pull_requests, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("state", mcp.Description("State (open|closed|all)"), mcp.DefaultString("open")),
		mcp.WithString("sort", mcp.Description("Sort (oldest|recentupdate|leastupdate|mostcomment)")),
		mcp.WithString("milestone", mcp.Description(params.Milestone)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
		mcp.WithBoolean("verbose", mcp.Description(compact.VerboseDescription)),
	)

	CreatePullRequestTool = mcp.NewTool(
		CreatePullRequestToolName,
		mcp.WithDescription("Create pull request"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("head", mcp.Required(), mcp.Description(params.Head)),
		mcp.WithString("base", mcp.Required(), mcp.Description(params.Base)),
		mcp.WithString("title", mcp.Required(), mcp.Description(params.Title)),
		mcp.WithString("body", mcp.Description(params.Body)),
		mcp.WithBoolean("verbose", mcp.Description(compact.VerboseDescription)),
	)

	UpdatePullRequestTool = mcp.NewTool(
		UpdatePullRequestToolName,
		mcp.WithDescription("Update pull request"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithString("title", mcp.Description(params.Title)),
		mcp.WithString("body", mcp.Description(params.Body)),
		mcp.WithString("base", mcp.Description(params.Base)),
		mcp.WithString("assignee", mcp.Description("Assignee username")),
		mcp.WithString("milestone", mcp.Description(params.Milestone)),
		mcp.WithBoolean("verbose", mcp.Description(compact.VerboseDescription)),
	)

	ListPullReviewsTool = mcp.NewTool(
		ListPullReviewsToolName,
		mcp.WithDescription("List reviews for a pull request. Returns a response envelope {reviews, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	GetPullReviewTool = mcp.NewTool(
		GetPullReviewToolName,
		mcp.WithDescription("Get a specific pull request review"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Review ID")),
		mcp.WithBoolean("verbose", mcp.Description(compact.VerboseDescription)),
	)

	ListPullReviewCommentsTool = mcp.NewTool(
		ListPullReviewCommentsToolName,
		mcp.WithDescription("List comments on a pull request review"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Review ID")),
	)

	ListPullRequestFilesTool = mcp.NewTool(
		ListPullRequestFilesToolName,
		mcp.WithDescription("List changed files in a pull request. Returns a response envelope {files, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(50)),
	)

	GetPullRequestDiffTool = mcp.NewTool(
		GetPullRequestDiffToolName,
		mcp.WithDescription("Get the unified diff of a pull request, one page of whole files at a time (page, limit; default 50 files, so a pull request of ordinary size comes back whole). When files follow the page, the output ends with a truncation marker naming the next page. Pass an optional file_path to receive only the hunks for that file (match is exact on either the pre- or post-rename path); file_path ignores page and limit. Use list_pull_request_files first to discover the file paths in the PR."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithString("file_path", mcp.Description("Optional. Return only the diff section for this file (matched on the diff --git boundary). Omit for the page of files.")),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description("Files per page (default 50; a larger limit than the instance maximum is refused)."), mcp.DefaultNumber(50)),
	)

	MergePullRequestTool = mcp.NewTool(
		MergePullRequestToolName,
		mcp.WithDescription("Merge a pull request"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithString("style", mcp.Required(), mcp.Description("Merge style (merge, rebase, rebase-merge, squash)")),
		mcp.WithString("title", mcp.Description("Merge commit title")),
		mcp.WithString("message", mcp.Description("Merge commit message")),
		mcp.WithBoolean("delete_branch_after_merge", mcp.Description("Delete head branch after merge")),
		mcp.WithBoolean("force_merge", mcp.Description("Force merge even if checks have not passed")),
		mcp.WithBoolean("merge_when_checks_succeed", mcp.Description("Schedule merge for when all checks succeed")),
	)
)

// RegisterTool registers the pull-request tools. Each call takes its Forgejo port
// from f, so a request is served with the credential that request carries.
func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(GetPullRequestByIndexTool, h.GetPullRequestByIndexFn)
	s.AddTool(ListRepoPullRequestsTool, h.ListRepoPullRequestsFn)
	s.AddTool(CreatePullRequestTool, h.CreatePullRequestFn)
	s.AddTool(UpdatePullRequestTool, h.UpdatePullRequestFn)
	s.AddTool(ListPullReviewsTool, h.ListPullReviewsFn)
	s.AddTool(GetPullReviewTool, h.GetPullReviewFn)
	s.AddTool(ListPullReviewCommentsTool, h.ListPullReviewCommentsFn)
	s.AddTool(MergePullRequestTool, h.MergePullRequestFn)
	s.AddTool(ListPullRequestFilesTool, h.ListPullRequestFilesFn)
	s.AddTool(GetPullRequestDiffTool, h.GetPullRequestDiffFn)
}
