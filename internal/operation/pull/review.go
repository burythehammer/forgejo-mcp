// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	CreatePullReviewToolName     = "create_pull_review"
	SubmitPullReviewToolName     = "submit_pull_review"
	DismissPullReviewToolName    = "dismiss_pull_review"
	DeletePullReviewToolName     = "delete_pull_review"
	CreateReviewRequestsToolName = "create_review_requests"
	DeleteReviewRequestsToolName = "delete_review_requests"
)

var (
	CreatePullReviewTool = mcp.NewTool(
		CreatePullReviewToolName,
		mcp.WithDescription("Create a pull request review with optional inline comments"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithString("body", mcp.Description(params.ReviewBody)),
		mcp.WithString("state", mcp.Required(), mcp.Description(params.ReviewState)),
		mcp.WithString("comments", mcp.Description(params.ReviewComments)),
	)

	SubmitPullReviewTool = mcp.NewTool(
		SubmitPullReviewToolName,
		mcp.WithDescription("Submit a pending pull request review"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description(params.ReviewID)),
		mcp.WithString("body", mcp.Description(params.ReviewBody)),
		mcp.WithString("state", mcp.Required(), mcp.Description(params.ReviewState)),
	)

	DismissPullReviewTool = mcp.NewTool(
		DismissPullReviewToolName,
		mcp.WithDescription("Dismiss a pull request review"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description(params.ReviewID)),
		mcp.WithString("message", mcp.Required(), mcp.Description(params.DismissMessage)),
	)

	DeletePullReviewTool = mcp.NewTool(
		DeletePullReviewToolName,
		mcp.WithDescription("Delete a pending pull request review"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description(params.ReviewID)),
	)

	CreateReviewRequestsTool = mcp.NewTool(
		CreateReviewRequestsToolName,
		mcp.WithDescription("Request reviews from specific users or teams"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithString("reviewers", mcp.Description(params.Reviewers)),
		mcp.WithString("team_reviewers", mcp.Description(params.TeamReviewers)),
	)

	DeleteReviewRequestsTool = mcp.NewTool(
		DeleteReviewRequestsToolName,
		mcp.WithDescription("Cancel pending review requests"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithString("reviewers", mcp.Description(params.Reviewers)),
		mcp.WithString("team_reviewers", mcp.Description(params.TeamReviewers)),
	)
)

// RegisterReviewTools registers the pull-review tools, served from f like RegisterTool.
func RegisterReviewTools(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(CreatePullReviewTool, h.CreatePullReviewFn)
	s.AddTool(SubmitPullReviewTool, h.SubmitPullReviewFn)
	s.AddTool(DismissPullReviewTool, h.DismissPullReviewFn)
	s.AddTool(DeletePullReviewTool, h.DeletePullReviewFn)
	s.AddTool(CreateReviewRequestsTool, h.CreateReviewRequestsFn)
	s.AddTool(DeleteReviewRequestsTool, h.DeleteReviewRequestsFn)
}
