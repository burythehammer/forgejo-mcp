// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"

	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/pipeline"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/resource"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	GetCommitStatusesToolName = "get_commit_statuses"

	defaultCommitStatusesLimit = resource.EmbeddedListCap
)

var GetCommitStatusesTool = mcp.NewTool(
	GetCommitStatusesToolName,
	annotate.Read(),
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

// paging is the page and limit of the call. A zero limit is passed on as zero so
// the use case applies the default, lowered to the instance ceiling; the list
// module refuses a limit above it. Validate runs it before the credential lookup.
func (a getCommitStatusesArgs) paging() (int, int, error) {
	return decode.PagingArgs(a.Page, a.Limit)
}

func (h handlers) GetCommitStatusesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.adminService, getCommitStatuses)(ctx, req)
}

func getCommitStatuses(ctx context.Context, s *apprepo.AdminService, a getCommitStatusesArgs) (*mcp.CallToolResult, error) {
	pg, limit, err := a.paging()
	if err != nil {
		return mcpresult.Render(nil, err)
	}
	b, err := s.GetCommitStatuses(ctx, a.Owner, a.Repo, a.SHA, apprepo.AdminPaging{Page: pg, Limit: limit})
	return mcpresult.Render(statusPage(a.SHA, b, err))
}

// Validate refuses a call that names no repository or SHA, or whose page or limit is out of range.
func (a getCommitStatusesArgs) Validate() error {
	if err := requireOwnerRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	if err := requireString("sha", a.SHA); err != nil {
		return err
	}
	_, _, err := a.paging()
	return err
}
