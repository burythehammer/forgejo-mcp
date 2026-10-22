// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"

	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListRepoContentsToolName = "list_repo_contents"
	GetRepoTreeToolName      = "get_repo_tree"
)

var (
	ListRepoContentsTool = mcp.NewTool(
		ListRepoContentsToolName,
		mcp.WithDescription("List the files and directories at a given path in a repository. Use path=\"\" to list the repository root. Returns one level of entries at the specified path, one page at a time: page and limit (default 100 entries) select the page, and the response carries has_next to say whether more entries follow. For a full recursive file tree use get_repo_tree with recursive=true."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("ref", mcp.Required(), mcp.Description(params.Ref)),
		mcp.WithString("path", mcp.Required(), mcp.Description("Directory path within the repository (empty string lists the repository root)")),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
	)

	GetRepoTreeTool = mcp.NewTool(
		GetRepoTreeToolName,
		mcp.WithDescription("Get the Git tree of a repository. With recursive=true, returns the complete file tree in a single response (subject to the server's tree-endpoint size cap); use this when you need all paths at once. With recursive=false (default), returns only the top-level entries of the tree. page and limit (default 1000) select one page of those entries."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("ref", mcp.Required(), mcp.Description(params.Ref)),
		mcp.WithBoolean("recursive", mcp.Description("Return the complete file tree in one response (subject to server cap); default false returns top-level entries only")),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(1000), mcp.Min(1)),
	)
)

// listRepoContentsArgs are the arguments of list_repo_contents. Owner, repo and
// ref are checked for presence as the tool always has; ref itself is not.
type listRepoContentsArgs struct {
	Owner *string `json:"owner" required:"true"`
	Repo  *string `json:"repo" required:"true"`
	Ref   string  `json:"ref" required:"true"`
	Path  string  `json:"path" required:"true"`
	Page  float64 `json:"page"`
	Limit float64 `json:"limit"`
}

// getRepoTreeArgs are the arguments of get_repo_tree.
type getRepoTreeArgs struct {
	Owner     *string `json:"owner" required:"true"`
	Repo      *string `json:"repo" required:"true"`
	Ref       *string `json:"ref" required:"true"`
	Recursive bool    `json:"recursive"`
	Page      float64 `json:"page" required:"true"`
	Limit     float64 `json:"limit" required:"true"`
}

func (h handlers) ListRepoContentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoContentsFn")
	var a listRepoContentsArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	owner, err := need("owner", a.Owner)
	if err != nil {
		return to.ErrorResult(err)
	}
	repo, err := need("repo", a.Repo)
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := h.gitService(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	return gitText(contentsPage(svc.ListContents(ctx, owner, repo, a.Ref, a.Path, apprepo.GitPage{
		Page:  decode.PageOr(a.Page, 1),
		Limit: decode.IntOr(a.Limit, 0),
	})))
}

func (h handlers) GetRepoTreeFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetRepoTreeFn")
	var a getRepoTreeArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	owner, err := need("owner", a.Owner)
	if err != nil {
		return to.ErrorResult(err)
	}
	repo, err := need("repo", a.Repo)
	if err != nil {
		return to.ErrorResult(err)
	}
	ref, err := need("ref", a.Ref)
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := h.gitService(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	return gitText(svc.GetTree(ctx, owner, repo, ref, apprepo.TreeInput{
		Recursive: a.Recursive,
		Page:      apprepo.GitPage{Page: int(orDefault(a.Page, 1)), Limit: int(orDefault(a.Limit, 1000))},
	}))
}
