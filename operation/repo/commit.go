// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"

	apprepo "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/repo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/decode"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListRepoCommitsToolName = "list_repo_commits"
)

var ListRepoCommitsTool = mcp.NewTool(
	ListRepoCommitsToolName,
	mcp.WithDescription("List repo commits"),
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
	return gitText(svc.ListCommits(ctx, a.Owner, a.Repo, apprepo.ListCommitsInput{
		Path: a.Path, SHA: a.SHA,
		Page: apprepo.GitPage{Page: int(orDefault(a.Page, 1)), Limit: int(a.Limit)},
	}))
}
