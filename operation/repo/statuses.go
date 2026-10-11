// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"

	apprepo "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/repo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcpresult"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	GetCommitStatusesToolName = "get_commit_statuses"

	defaultCommitStatusesLimit = resource.EmbeddedListCap
	maxCommitStatusesLimit     = 50
)

var GetCommitStatusesTool = mcp.NewTool(
	GetCommitStatusesToolName,
	mcp.WithDescription("List per-context commit statuses for a full 40-character SHA. Bounded by page (default 1) and limit (default 30, maximum 50); returns {sha, statuses, page, limit, count, total_count}. total_count is present only when Forgejo reports X-Total-Count. Combined aggregate stays on forgejo://repo/{owner}/{repo}/commit/{sha}/status. Commit statuses are CI context checks, not Actions workflow runs (list_workflow_runs)."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("sha", mcp.Required(), mcp.Description("Full 40-character hex commit SHA")),
	mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
	mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultCommitStatusesLimit), mcp.Min(1), mcp.Max(maxCommitStatusesLimit)),
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
		// The paging is truncated before the use case clamps it, which gives
		// the same page and limit as clamping the fraction did.
		res, err := s.GetCommitStatuses(ctx, a.Owner, a.Repo, a.SHA, int(a.Page), int(a.Limit))
		return mcpresult.Render(res, err)
	})
}
