// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"

	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

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

func (h handlers) ListRepoCommitsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoCommitsFn")
	var a listRepoCommitsArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	svc, err := h.gitService(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	page, limit, err := decode.PagingArgs(a.Page, a.Limit)
	if err != nil {
		return to.ErrorResult(err)
	}
	return gitText(commitListPage(svc.ListCommits(ctx, a.Owner, a.Repo, apprepo.ListCommitsInput{
		Path: a.Path, SHA: a.SHA,
		Page: apprepo.GitPage{Page: page, Limit: limit},
	})))
}
