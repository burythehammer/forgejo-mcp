// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"context"
	"fmt"
	"strings"

	apppull "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/pull"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/pulladapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/compact"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"

	"github.com/mark3labs/mcp-go/mcp"
)

// handlers serve the pull tools. They hold the factory rather than a port so
// each call is bound to the credential of its own request. Each handler is
// the MCP adapter of one use case in internal/app/pull: decode the arguments, run the
// use case, render the result.
type handlers struct{ forge forge.Factory }

// Argument structs, one per tool. `json` names the MCP argument and
// `required:"true"` marks the required ones; mcptest.ArgsMatchSchema keeps
// them in agreement with the tool definitions.

type getArgs struct {
	Owner   string  `json:"owner" required:"true"`
	Repo    string  `json:"repo" required:"true"`
	Index   float64 `json:"index" required:"true"`
	Verbose bool    `json:"verbose"`
}

type listArgs struct {
	Owner     string  `json:"owner" required:"true"`
	Repo      string  `json:"repo" required:"true"`
	State     string  `json:"state"`
	Sort      string  `json:"sort"`
	Milestone string  `json:"milestone"`
	Page      float64 `json:"page"`
	Limit     float64 `json:"limit"`
	Verbose   bool    `json:"verbose"`
}

type createArgs struct {
	Owner   string `json:"owner" required:"true"`
	Repo    string `json:"repo" required:"true"`
	Head    string `json:"head" required:"true"`
	Base    string `json:"base" required:"true"`
	Title   string `json:"title" required:"true"`
	Body    string `json:"body"`
	Verbose bool   `json:"verbose"`
}

type updateArgs struct {
	Owner     string  `json:"owner" required:"true"`
	Repo      string  `json:"repo" required:"true"`
	Index     float64 `json:"index" required:"true"`
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	Base      string  `json:"base"`
	Assignee  string  `json:"assignee"`
	Milestone string  `json:"milestone"`
	Verbose   bool    `json:"verbose"`
}

type pagedArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Index float64 `json:"index" required:"true"`
	Page  float64 `json:"page"`
	Limit float64 `json:"limit"`
}

type reviewArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Index float64 `json:"index" required:"true"`
	ID    float64 `json:"id" required:"true"`
}

// reviewCommentsArgs is list_pull_review_comments' argument struct: a review
// reference and the page to read.
type reviewCommentsArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Index float64 `json:"index" required:"true"`
	ID    float64 `json:"id" required:"true"`
	Page  float64 `json:"page"`
	Limit float64 `json:"limit"`
}

// getReviewArgs is get_pull_review's own argument struct, so its verbose flag
// does not reach the review tools that share reviewArgs.
type getReviewArgs struct {
	Owner   string  `json:"owner" required:"true"`
	Repo    string  `json:"repo" required:"true"`
	Index   float64 `json:"index" required:"true"`
	ID      float64 `json:"id" required:"true"`
	Verbose bool    `json:"verbose"`
}

type mergeArgs struct {
	Owner             string  `json:"owner" required:"true"`
	Repo              string  `json:"repo" required:"true"`
	Index             float64 `json:"index" required:"true"`
	Style             string  `json:"style" required:"true"`
	Title             string  `json:"title"`
	Message           string  `json:"message"`
	DeleteBranch      bool    `json:"delete_branch_after_merge"`
	ForceMerge        bool    `json:"force_merge"`
	MergeWhenChecksOK bool    `json:"merge_when_checks_succeed"`
}

type diffArgs struct {
	Owner    string  `json:"owner" required:"true"`
	Repo     string  `json:"repo" required:"true"`
	Index    float64 `json:"index" required:"true"`
	FilePath string  `json:"file_path"`
	Page     float64 `json:"page"`
	Limit    float64 `json:"limit"`
}

type createReviewArgs struct {
	Owner    string  `json:"owner" required:"true"`
	Repo     string  `json:"repo" required:"true"`
	Index    float64 `json:"index" required:"true"`
	Body     string  `json:"body"`
	State    string  `json:"state" required:"true"`
	Comments string  `json:"comments"`
}

type submitReviewArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Index float64 `json:"index" required:"true"`
	ID    float64 `json:"id" required:"true"`
	Body  string  `json:"body"`
	State string  `json:"state" required:"true"`
}

type dismissArgs struct {
	Owner   string  `json:"owner" required:"true"`
	Repo    string  `json:"repo" required:"true"`
	Index   float64 `json:"index" required:"true"`
	ID      float64 `json:"id" required:"true"`
	Message string  `json:"message" required:"true"`
}

type reviewersArgs struct {
	Owner         string  `json:"owner" required:"true"`
	Repo          string  `json:"repo" required:"true"`
	Index         float64 `json:"index" required:"true"`
	Reviewers     string  `json:"reviewers"`
	TeamReviewers string  `json:"team_reviewers"`
}

// run is the shared shape of a tool call: decode the arguments into A, take
// the use cases bound to this request's credential, and hand both to do,
// which runs one use case and renders its result.
func run[A any](ctx context.Context, h handlers, req mcp.CallToolRequest, name string,
	do func(context.Context, *apppull.Service, A) (*mcp.CallToolResult, error),
) (*mcp.CallToolResult, error) {
	log.Debugf("Called %s", name)
	var args A
	if err := decode.Lax(req, &args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	port, err := h.forge.For(ctx)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return do(ctx, apppull.New(pulladapter.New(port)), args)
}

// message sends a use case's outcome as plain text.
func message(text string, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcpresult.Fail(err)
	}
	return mcp.NewToolResultText(text), nil
}

func (h handlers) GetPullRequestByIndexFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "GetPullRequestByIndexFn", func(ctx context.Context, s *apppull.Service, a getArgs) (*mcp.CallToolResult, error) {
		in := apppull.PR{Owner: a.Owner, Repo: a.Repo, Index: int64(a.Index)}
		read, err := s.Read(ctx, in, apppull.ReadOptions{})
		if err != nil {
			return nil, mapErr(pullResourceURI(a.Owner, a.Repo, in.Index), err)
		}
		return compact.RenderPullRequestRead(read.PR, a.Verbose)
	})
}

func (h handlers) ListRepoPullRequestsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListRepoPullRequestsFn", func(ctx context.Context, s *apppull.Service, a listArgs) (*mcp.CallToolResult, error) {
		page, err := s.List(ctx, apppull.ListInput{
			Owner: a.Owner, Repo: a.Repo, State: a.State, Sort: a.Sort, Milestone: a.Milestone, Page: int(a.Page), Limit: int(a.Limit),
		})
		envelope, err := pullListPage(page, err)
		return compact.RenderPullList(envelope, err, a.Verbose)
	})
}

func (h handlers) CreatePullRequestFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "CreatePullRequestFn", func(ctx context.Context, s *apppull.Service, a createArgs) (*mcp.CallToolResult, error) {
		pr, err := s.Create(ctx, apppull.CreateRequest{Owner: a.Owner, Repo: a.Repo, Head: a.Head, Base: a.Base, Title: a.Title, Body: a.Body})
		return compact.RenderPullRequest(pr, err, a.Verbose)
	})
}

func (h handlers) UpdatePullRequestFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "UpdatePullRequestFn", func(ctx context.Context, s *apppull.Service, a updateArgs) (*mcp.CallToolResult, error) {
		pr, err := s.Update(ctx, apppull.UpdateInput{
			PR:    apppull.PR{Owner: a.Owner, Repo: a.Repo, Index: int64(a.Index)},
			Title: a.Title, Body: a.Body, Base: a.Base, Assignee: a.Assignee, Milestone: a.Milestone,
		})
		return compact.RenderPullRequest(pr, err, a.Verbose)
	})
}

func (h handlers) ListPullReviewsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListPullReviewsFn", func(ctx context.Context, s *apppull.Service, a pagedArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(reviewPage(s.Reviews(ctx, apppull.PagedPR{PR: apppull.PR{Owner: a.Owner, Repo: a.Repo, Index: int64(a.Index)}, Page: int(a.Page), Limit: int(a.Limit)})))
	})
}

func (h handlers) GetPullReviewFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "GetPullReviewFn", func(ctx context.Context, s *apppull.Service, a getReviewArgs) (*mcp.CallToolResult, error) {
		review, err := s.Review(ctx, reviewRef(reviewArgs{Owner: a.Owner, Repo: a.Repo, Index: a.Index, ID: a.ID}))
		return compact.RenderReviewRead(review, err, a.Verbose)
	})
}

func (h handlers) ListPullReviewCommentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListPullReviewCommentsFn", func(ctx context.Context, s *apppull.Service, a reviewCommentsArgs) (*mcp.CallToolResult, error) {
		ref := reviewRef(reviewArgs{Owner: a.Owner, Repo: a.Repo, Index: a.Index, ID: a.ID})
		return mcpresult.Render(reviewCommentPage(s.ReviewComments(ctx, ref, apppull.Paging{
			Page:  decode.PageOr(a.Page, 1),
			Limit: decode.IntOr(a.Limit, 0),
		})))
	})
}

func (h handlers) MergePullRequestFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "MergePullRequestFn", func(ctx context.Context, s *apppull.Service, a mergeArgs) (*mcp.CallToolResult, error) {
		res, err := s.Merge(ctx, apppull.MergeRequest{
			PR:    apppull.PR{Owner: a.Owner, Repo: a.Repo, Index: int64(a.Index)},
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
		return mcpresult.Render(filesPage(s.Files(ctx, apppull.PagedPR{PR: apppull.PR{Owner: a.Owner, Repo: a.Repo, Index: int64(a.Index)}, Page: int(a.Page), Limit: int(a.Limit)})))
	})
}

func (h handlers) GetPullRequestDiffFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "GetPullRequestDiffFn", func(ctx context.Context, s *apppull.Service, a diffArgs) (*mcp.CallToolResult, error) {
		page, err := s.Diff(ctx, apppull.DiffInput{
			PR:       apppull.PR{Owner: a.Owner, Repo: a.Repo, Index: int64(a.Index)},
			FilePath: a.FilePath,
			Page:     decode.PageOr(a.Page, 1),
			Limit:    decode.IntOr(a.Limit, 0),
		})
		if err != nil {
			return message("", err)
		}
		return message(diffWithMarker(page), nil)
	})
}

// diffWithMarker is the diff page as text, followed by a truncation marker
// when files follow it or the page is past the last file. A diff that fits in
// one page comes back as the bare diff, as it always has.
func diffWithMarker(p apppull.DiffPage) string {
	switch {
	case p.HasNext:
		first := (p.Page-1)*p.Limit + 1
		marker := fmt.Sprintf("[diff truncated: files %d-%d of %d shown; call get_pull_request_diff with page=%d and limit=%d for the next part, or pass file_path for one file]",
			first, first+p.Count-1, p.Files, p.Page+1, p.Limit)
		return withMarker(p.Text, marker)
	case p.Page > 0 && p.Count == 0 && p.Files > 0:
		return fmt.Sprintf("[no files on page %d: the diff has %d files]", p.Page, p.Files)
	}
	return p.Text
}

func withMarker(text, marker string) string {
	if text == "" {
		return marker
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return text + "\n" + marker
}

func (h handlers) CreatePullReviewFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "CreatePullReviewFn", func(ctx context.Context, s *apppull.Service, a createReviewArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.CreateReview(ctx, apppull.CreateReviewRequest{
			PR:    apppull.PR{Owner: a.Owner, Repo: a.Repo, Index: int64(a.Index)},
			State: a.State, Body: a.Body, Comments: a.Comments,
		}))
	})
}

func (h handlers) SubmitPullReviewFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "SubmitPullReviewFn", func(ctx context.Context, s *apppull.Service, a submitReviewArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.SubmitReview(ctx, apppull.SubmitReviewRequest{
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
	return apppull.ReviewRef{PR: apppull.PR{Owner: a.Owner, Repo: a.Repo, Index: int64(a.Index)}, ID: int64(a.ID)}
}

func reviewers(a reviewersArgs) apppull.ReviewersInput {
	return apppull.ReviewersInput{
		PR:        apppull.PR{Owner: a.Owner, Repo: a.Repo, Index: int64(a.Index)},
		Reviewers: a.Reviewers, TeamReviewers: a.TeamReviewers,
	}
}
