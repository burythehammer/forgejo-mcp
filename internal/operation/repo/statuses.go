// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"

	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/resource"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	GetCommitStatusesToolName = "get_commit_statuses"

	defaultCommitStatusesLimit = resource.EmbeddedListCap
)

var GetCommitStatusesTool = mcp.NewTool(
	GetCommitStatusesToolName,
	mcp.WithDescription("List per-context commit statuses for a full 40-character SHA. Bounded by page (default 1) and limit (default 30, lowered to the instance ceiling when that is lower; a limit above a known ceiling is refused). Returns the envelope {sha, statuses, page, limit, count, has_next, total_count}; has_next true means a further page may exist, so re-issue the call with page incremented. total_count is present only when Forgejo reports X-Total-Count. Combined aggregate stays on forgejo://repo/{owner}/{repo}/commit/{sha}/status. Commit statuses are CI context checks, not Actions workflow runs (list_workflow_runs)."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("sha", mcp.Required(), mcp.Description("Full 40-character hex commit SHA")),
	mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
	mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultCommitStatusesLimit), mcp.Min(1)),
)

type getCommitStatusesArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	SHA   string  `json:"sha" required:"true"`
	Page  float64 `json:"page"`
	Limit float64 `json:"limit"`
}

func (h handlers) GetCommitStatusesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return runAdmin(ctx, h, req, "GetCommitStatusesFn", func(ctx context.Context, s *apprepo.AdminService, a getCommitStatusesArgs) (*mcp.CallToolResult, error) {
		// A zero limit is passed on as zero so the use case applies the default,
		// lowered to the instance ceiling; the list module refuses a limit above it.
		b, err := s.GetCommitStatuses(ctx, a.Owner, a.Repo, a.SHA, apprepo.AdminPaging{Page: decode.PageOr(a.Page, 1), Limit: int(a.Limit)})
		return mcpresult.Render(statusPage(a.SHA, b, err))
	})
}
