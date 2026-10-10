package pull

import (
	"context"
	"fmt"
	"strconv"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/diff"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/ptr"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
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
	)

	ListRepoPullRequestsTool = mcp.NewTool(
		ListRepoPullRequestsToolName,
		mcp.WithDescription("List repo pull requests"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("state", mcp.Description("State (open|closed|all)"), mcp.DefaultString("open")),
		mcp.WithString("sort", mcp.Description("Sort (oldest|recentupdate|leastupdate|mostcomment)")),
		mcp.WithString("milestone", mcp.Description(params.Milestone)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
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
	)

	ListPullReviewsTool = mcp.NewTool(
		ListPullReviewsToolName,
		mcp.WithDescription("List reviews for a pull request"),
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
		mcp.WithDescription("List changed files in a pull request"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(50)),
	)

	GetPullRequestDiffTool = mcp.NewTool(
		GetPullRequestDiffToolName,
		mcp.WithDescription("Get the unified diff of a pull request. Pass an optional file_path to receive only the hunks for that file (match is exact on either the pre- or post-rename path). Use list_pull_request_files first to discover the file paths in the PR."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithString("file_path", mcp.Description("Optional. Return only the diff section for this file (matched on the diff --git boundary). Omit for the full diff.")),
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

func (h handlers) GetPullRequestByIndexFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetPullRequestByIndexFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	pr, err := port.GetPullRequest(ctx, owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get pull request err: %w", err))
	}
	return to.TextResult(pr)
}

func (h handlers) ListRepoPullRequestsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoPullRequestsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	state, ok := req.GetArguments()["state"].(string)
	if !ok {
		state = "open"
	}
	sort, _ := req.GetArguments()["sort"].(string)
	milestone, _ := req.GetArguments()["milestone"].(string)
	page, _ := to.Float64(req.GetArguments()["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(req.GetArguments()["limit"])
	if limit == 0 {
		limit = 20
	}

	opt := forgejo_sdk.ListPullRequestsOptions{
		State: forgejo_sdk.StateType(state),
		Sort:  sort,
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}

	if milestone != "" {
		milestoneID, err := strconv.ParseInt(milestone, 10, 64)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("invalid milestone ID: %w", err))
		}
		opt.Milestone = milestoneID
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	prs, err := port.ListPullRequests(ctx, owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get pull request list err: %w", err))
	}
	return to.TextResult(prs)
}

func (h handlers) CreatePullRequestFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreatePullRequestFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	head, _ := req.GetArguments()["head"].(string)
	base, _ := req.GetArguments()["base"].(string)
	title, _ := req.GetArguments()["title"].(string)
	body, _ := req.GetArguments()["body"].(string)

	opt := forgejo_sdk.CreatePullRequestOption{
		Head:  head,
		Base:  base,
		Title: title,
		Body:  body,
	}
	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	pr, err := port.CreatePullRequest(ctx, owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create pull request err: %w", err))
	}
	return to.TextResult(pr)
}

func (h handlers) UpdatePullRequestFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called UpdatePullRequestFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	title, _ := req.GetArguments()["title"].(string)
	body, _ := req.GetArguments()["body"].(string)
	base, _ := req.GetArguments()["base"].(string)
	assignee, _ := req.GetArguments()["assignee"].(string)
	milestone, _ := req.GetArguments()["milestone"].(string)

	opt := forgejo_sdk.EditPullRequestOption{}

	if title != "" {
		opt.Title = title
	}
	if body != "" {
		opt.Body = ptr.To(body)
	}
	if base != "" {
		opt.Base = base
	}
	if assignee != "" {
		opt.Assignee = assignee
	}
	if milestone != "" {
		milestoneID, err := strconv.ParseInt(milestone, 10, 64)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("invalid milestone ID: %w", err))
		}
		opt.Milestone = milestoneID
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	pr, err := port.EditPullRequest(ctx, owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("update pull request err: %w", err))
	}
	return to.TextResult(pr)
}

func (h handlers) ListPullReviewsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListPullReviewsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	page, _ := to.Float64(req.GetArguments()["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(req.GetArguments()["limit"])
	if limit == 0 {
		limit = 20
	}

	opt := forgejo_sdk.ListPullReviewsOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	reviews, err := port.ListPullReviews(ctx, owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list pull reviews err: %w", err))
	}
	return to.TextResult(reviews)
}

func (h handlers) GetPullReviewFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetPullReviewFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	id, _ := to.Float64(req.GetArguments()["id"])

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	review, err := port.GetPullReview(ctx, owner, repo, int64(index), int64(id))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get pull review err: %w", err))
	}
	return to.TextResult(review)
}

func (h handlers) MergePullRequestFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called MergePullRequestFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	style, _ := req.GetArguments()["style"].(string)
	title, _ := req.GetArguments()["title"].(string)
	message, _ := req.GetArguments()["message"].(string)
	deleteBranch, _ := req.GetArguments()["delete_branch_after_merge"].(bool)
	forceMerge, _ := req.GetArguments()["force_merge"].(bool)
	mergeWhenChecks, _ := req.GetArguments()["merge_when_checks_succeed"].(bool)

	opt := forgejo_sdk.MergePullRequestOption{
		Style:                  forgejo_sdk.MergeStyle(style),
		DeleteBranchAfterMerge: deleteBranch,
		ForceMerge:             forceMerge,
		MergeWhenChecksSucceed: mergeWhenChecks,
	}

	if title != "" {
		opt.Title = title
	}
	if message != "" {
		opt.Message = message
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	merged, err := port.MergePullRequest(ctx, owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("merge pull request err: %w", err))
	}
	if !merged.Merged && !mergeWhenChecks {
		return to.ErrorResult(fmt.Errorf("merge pull request: server returned HTTP %d (expected 200)", merged.StatusCode))
	}

	result := "Pull request merged successfully"
	if mergeWhenChecks {
		result = "Pull request scheduled to merge when all checks succeed"
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(result)},
	}, nil
}

func (h handlers) ListPullRequestFilesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListPullRequestFilesFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	page, _ := to.Float64(req.GetArguments()["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(req.GetArguments()["limit"])
	if limit == 0 {
		limit = 50
	}

	opt := forgejo_sdk.ListPullRequestFilesOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	files, err := port.ListPullRequestFiles(ctx, owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list pull request files err: %w", err))
	}
	return to.TextResult(files)
}

func (h handlers) GetPullRequestDiffFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetPullRequestDiffFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	index, _ := to.Float64(args["index"])
	filePath, _ := args["file_path"].(string)

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	diffBytes, err := port.GetPullRequestDiff(ctx, owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get pull request diff err: %w", err))
	}

	body := string(diffBytes)
	if filePath != "" {
		slice, found := diff.FileSlice(body, filePath)
		if !found {
			return to.ErrorResult(fmt.Errorf("file_path %q not found in pull request diff", filePath))
		}
		body = slice
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(body)},
	}, nil
}

func (h handlers) ListPullReviewCommentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListPullReviewCommentsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	id, _ := to.Float64(req.GetArguments()["id"])

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	comments, err := port.ListPullReviewComments(ctx, owner, repo, int64(index), int64(id))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list pull review comments err: %w", err))
	}
	return to.TextResult(comments)
}

// handlers serve the pull tools. They hold the factory rather than a port so
// each call is bound to the credential of its own request.
type handlers struct{ forge forge.Factory }
