// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"context"

	apppull "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/pull"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/decode"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/pulladapter"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

// handlers serve the pull tools. They hold the factory rather than a port so
// each call is bound to the credential of its own request. Each handler is
// the MCP adapter of one use case in app/pull: decode the arguments, run the
// use case, render the result.
type handlers struct{ forge forge.Factory }

// Argument structs, one per tool. `json` names the MCP argument and
// `required:"true"` marks the required ones; mcptest.ArgsMatchSchema keeps
// them in agreement with the tool definitions.

type getArgs struct {
	Owner string `json:"owner" required:"true"`
	Repo  string `json:"repo" required:"true"`
	Index int64  `json:"index" required:"true"`
}

type listArgs struct {
	Owner     string `json:"owner" required:"true"`
	Repo      string `json:"repo" required:"true"`
	State     string `json:"state"`
	Sort      string `json:"sort"`
	Milestone string `json:"milestone"`
	Page      int    `json:"page"`
	Limit     int    `json:"limit"`
}

type createArgs struct {
	Owner string `json:"owner" required:"true"`
	Repo  string `json:"repo" required:"true"`
	Head  string `json:"head" required:"true"`
	Base  string `json:"base" required:"true"`
	Title string `json:"title" required:"true"`
	Body  string `json:"body"`
}

type updateArgs struct {
	Owner     string `json:"owner" required:"true"`
	Repo      string `json:"repo" required:"true"`
	Index     int64  `json:"index" required:"true"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Base      string `json:"base"`
	Assignee  string `json:"assignee"`
	Milestone string `json:"milestone"`
}

type pagedArgs struct {
	Owner string `json:"owner" required:"true"`
	Repo  string `json:"repo" required:"true"`
	Index int64  `json:"index" required:"true"`
	Page  int    `json:"page"`
	Limit int    `json:"limit"`
}

type reviewArgs struct {
	Owner string `json:"owner" required:"true"`
	Repo  string `json:"repo" required:"true"`
	Index int64  `json:"index" required:"true"`
	ID    int64  `json:"id" required:"true"`
}

type mergeArgs struct {
	Owner             string `json:"owner" required:"true"`
	Repo              string `json:"repo" required:"true"`
	Index             int64  `json:"index" required:"true"`
	Style             string `json:"style" required:"true"`
	Title             string `json:"title"`
	Message           string `json:"message"`
	DeleteBranch      bool   `json:"delete_branch_after_merge"`
	ForceMerge        bool   `json:"force_merge"`
	MergeWhenChecksOK bool   `json:"merge_when_checks_succeed"`
}

type diffArgs struct {
	Owner    string `json:"owner" required:"true"`
	Repo     string `json:"repo" required:"true"`
	Index    int64  `json:"index" required:"true"`
	FilePath string `json:"file_path"`
}

type createReviewArgs struct {
	Owner    string `json:"owner" required:"true"`
	Repo     string `json:"repo" required:"true"`
	Index    int64  `json:"index" required:"true"`
	Body     string `json:"body"`
	State    string `json:"state" required:"true"`
	Comments string `json:"comments"`
}

type submitReviewArgs struct {
	Owner string `json:"owner" required:"true"`
	Repo  string `json:"repo" required:"true"`
	Index int64  `json:"index" required:"true"`
	ID    int64  `json:"id" required:"true"`
	Body  string `json:"body"`
	State string `json:"state" required:"true"`
}

type dismissArgs struct {
	Owner   string `json:"owner" required:"true"`
	Repo    string `json:"repo" required:"true"`
	Index   int64  `json:"index" required:"true"`
	ID      int64  `json:"id" required:"true"`
	Message string `json:"message" required:"true"`
}

type reviewersArgs struct {
	Owner         string `json:"owner" required:"true"`
	Repo          string `json:"repo" required:"true"`
	Index         int64  `json:"index" required:"true"`
	Reviewers     string `json:"reviewers"`
	TeamReviewers string `json:"team_reviewers"`
}

// run is the shared shape of a tool call: decode the arguments into A, take
// the use cases bound to this request's credential, and hand both to do,
// which runs one use case and renders its result.
func run[A any](ctx context.Context, h handlers, req mcp.CallToolRequest, name string,
	do func(context.Context, *apppull.Service, A) (*mcp.CallToolResult, error),
) (*mcp.CallToolResult, error) {
	log.Debugf("Called %s", name)
	var args A
	if err := decode.Args(req, &args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	port, err := h.forge.For(ctx)
	if err != nil {
		return fail(err)
	}
	return do(ctx, apppull.New(pulladapter.New(port)), args)
}

// fail is the one place use-case errors become MCP errors. The typed kinds in
// app/pull (InvalidInputError, DiffFileNotFoundError, MergeDeclinedError) and
// failures from Forgejo all surface as the tool call's error, with the text
// the use case gave them.
func fail(err error) (*mcp.CallToolResult, error) { return to.ErrorResult(err) }

// render sends a use case's result, or its error, to the client as JSON.
func render(v any, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return fail(err)
	}
	return to.TextResult(v)
}

// message sends a use case's outcome as plain text.
func message(text string, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return fail(err)
	}
	return mcp.NewToolResultText(text), nil
}

func (h handlers) GetPullRequestByIndexFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "GetPullRequestByIndexFn", func(ctx context.Context, s *apppull.Service, a getArgs) (*mcp.CallToolResult, error) {
		return render(s.Get(ctx, apppull.PR{Owner: a.Owner, Repo: a.Repo, Index: a.Index}))
	})
}

func (h handlers) ListRepoPullRequestsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListRepoPullRequestsFn", func(ctx context.Context, s *apppull.Service, a listArgs) (*mcp.CallToolResult, error) {
		return render(s.List(ctx, apppull.ListInput{
			Owner: a.Owner, Repo: a.Repo, State: a.State, Sort: a.Sort, Milestone: a.Milestone, Page: a.Page, Limit: a.Limit,
		}))
	})
}

func (h handlers) CreatePullRequestFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "CreatePullRequestFn", func(ctx context.Context, s *apppull.Service, a createArgs) (*mcp.CallToolResult, error) {
		return render(s.Create(ctx, apppull.CreateRequest{Owner: a.Owner, Repo: a.Repo, Head: a.Head, Base: a.Base, Title: a.Title, Body: a.Body}))
	})
}

func (h handlers) UpdatePullRequestFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "UpdatePullRequestFn", func(ctx context.Context, s *apppull.Service, a updateArgs) (*mcp.CallToolResult, error) {
		return render(s.Update(ctx, apppull.UpdateInput{
			PR:    apppull.PR{Owner: a.Owner, Repo: a.Repo, Index: a.Index},
			Title: a.Title, Body: a.Body, Base: a.Base, Assignee: a.Assignee, Milestone: a.Milestone,
		}))
	})
}

func (h handlers) ListPullReviewsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListPullReviewsFn", func(ctx context.Context, s *apppull.Service, a pagedArgs) (*mcp.CallToolResult, error) {
		return render(s.Reviews(ctx, apppull.PagedPR{PR: apppull.PR{Owner: a.Owner, Repo: a.Repo, Index: a.Index}, Page: a.Page, Limit: a.Limit}))
	})
}

func (h handlers) GetPullReviewFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "GetPullReviewFn", func(ctx context.Context, s *apppull.Service, a reviewArgs) (*mcp.CallToolResult, error) {
		return render(s.Review(ctx, reviewRef(a)))
	})
}

func (h handlers) ListPullReviewCommentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListPullReviewCommentsFn", func(ctx context.Context, s *apppull.Service, a reviewArgs) (*mcp.CallToolResult, error) {
		return render(s.ReviewComments(ctx, reviewRef(a)))
	})
}

func (h handlers) MergePullRequestFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "MergePullRequestFn", func(ctx context.Context, s *apppull.Service, a mergeArgs) (*mcp.CallToolResult, error) {
		res, err := s.Merge(ctx, apppull.MergeRequest{
			PR:    apppull.PR{Owner: a.Owner, Repo: a.Repo, Index: a.Index},
			Style: a.Style, Title: a.Title, Message: a.Message,
			DeleteBranch: a.DeleteBranch, Force: a.ForceMerge, WhenChecksSucceed: a.MergeWhenChecksOK,
		})
		text := "Pull request merged successfully"
		if res.Scheduled {
			text = "Pull request scheduled to merge when all checks succeed"
		}
		return message(text, err)
	})
}

func (h handlers) ListPullRequestFilesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListPullRequestFilesFn", func(ctx context.Context, s *apppull.Service, a pagedArgs) (*mcp.CallToolResult, error) {
		return render(s.Files(ctx, apppull.PagedPR{PR: apppull.PR{Owner: a.Owner, Repo: a.Repo, Index: a.Index}, Page: a.Page, Limit: a.Limit}))
	})
}

func (h handlers) GetPullRequestDiffFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "GetPullRequestDiffFn", func(ctx context.Context, s *apppull.Service, a diffArgs) (*mcp.CallToolResult, error) {
		body, err := s.Diff(ctx, apppull.DiffInput{PR: apppull.PR{Owner: a.Owner, Repo: a.Repo, Index: a.Index}, FilePath: a.FilePath})
		return message(body, err)
	})
}

func (h handlers) CreatePullReviewFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "CreatePullReviewFn", func(ctx context.Context, s *apppull.Service, a createReviewArgs) (*mcp.CallToolResult, error) {
		return render(s.CreateReview(ctx, apppull.CreateReviewRequest{
			PR:    apppull.PR{Owner: a.Owner, Repo: a.Repo, Index: a.Index},
			State: a.State, Body: a.Body, Comments: a.Comments,
		}))
	})
}

func (h handlers) SubmitPullReviewFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "SubmitPullReviewFn", func(ctx context.Context, s *apppull.Service, a submitReviewArgs) (*mcp.CallToolResult, error) {
		return render(s.SubmitReview(ctx, apppull.SubmitReviewRequest{
			ReviewRef: reviewRef(reviewArgs{Owner: a.Owner, Repo: a.Repo, Index: a.Index, ID: a.ID}),
			State:     a.State, Body: a.Body,
		}))
	})
}

func (h handlers) DismissPullReviewFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "DismissPullReviewFn", func(ctx context.Context, s *apppull.Service, a dismissArgs) (*mcp.CallToolResult, error) {
		err := s.DismissReview(ctx, apppull.DismissRequest{
			ReviewRef: reviewRef(reviewArgs{Owner: a.Owner, Repo: a.Repo, Index: a.Index, ID: a.ID}),
			Message:   a.Message,
		})
		return message(`{"Result":"review dismissed successfully"}`, err)
	})
}

func (h handlers) DeletePullReviewFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "DeletePullReviewFn", func(ctx context.Context, s *apppull.Service, a reviewArgs) (*mcp.CallToolResult, error) {
		return message(`{"Result":"review deleted successfully"}`, s.DeleteReview(ctx, reviewRef(a)))
	})
}

func (h handlers) CreateReviewRequestsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "CreateReviewRequestsFn", func(ctx context.Context, s *apppull.Service, a reviewersArgs) (*mcp.CallToolResult, error) {
		return message(`{"Result":"review requests created successfully"}`, s.RequestReviews(ctx, reviewers(a)))
	})
}

func (h handlers) DeleteReviewRequestsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "DeleteReviewRequestsFn", func(ctx context.Context, s *apppull.Service, a reviewersArgs) (*mcp.CallToolResult, error) {
		return message(`{"Result":"review requests deleted successfully"}`, s.CancelReviewRequests(ctx, reviewers(a)))
	})
}

func reviewRef(a reviewArgs) apppull.ReviewRef {
	return apppull.ReviewRef{PR: apppull.PR{Owner: a.Owner, Repo: a.Repo, Index: a.Index}, ID: a.ID}
}

func reviewers(a reviewersArgs) apppull.ReviewersInput {
	return apppull.ReviewersInput{
		PR:        apppull.PR{Owner: a.Owner, Repo: a.Repo, Index: a.Index},
		Reviewers: a.Reviewers, TeamReviewers: a.TeamReviewers,
	}
}
