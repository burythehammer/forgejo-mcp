// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"

	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/pipeline"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListRepoCommitsToolName = "list_repo_commits"
)

var ListRepoCommitsTool = mcp.NewTool(
	ListRepoCommitsToolName,
	annotate.Read(),
	mcp.WithDescription("List repo commits. Returns a response envelope {commits, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("path", mcp.Description("File/dir path")),
	mcp.WithString("sha", mcp.Description("SHA/branch to start from")),
	mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
	mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
)

// listRepoCommitsArgs are the arguments of list_repo_commits.
type listRepoCommitsArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Path  string  `json:"path"`
	SHA   string  `json:"sha"`
	Page  float64 `json:"page" required:"true"`
	Limit float64 `json:"limit" required:"true"`
}

// paging is the page and limit of the commit list, refused when either is out of range.
func (a listRepoCommitsArgs) paging() (int, int, error) {
	return decode.PagingArgs(a.Page, a.Limit)
}

func (h handlers) ListRepoCommitsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.gitService, listRepoCommits)(ctx, req)
}

func listRepoCommits(ctx context.Context, svc *apprepo.GitService, a listRepoCommitsArgs) (*mcp.CallToolResult, error) {
	page, limit, err := a.paging()
	if err != nil {
		return mcpresult.Fail(err)
	}
	return gitText(commitListPage(svc.ListCommits(ctx, a.Owner, a.Repo, apprepo.ListCommitsInput{
		Path: a.Path, SHA: a.SHA,
		Page: apprepo.GitPage{Page: page, Limit: limit},
	})))
}

// Validate refuses a commit list that names no repository or whose page or limit is out of range.
func (a listRepoCommitsArgs) Validate() error {
	if err := firstError(requireString("owner", a.Owner), requireString("repo", a.Repo)); err != nil {
		return err
	}
	_, _, err := a.paging()
	return err
}
