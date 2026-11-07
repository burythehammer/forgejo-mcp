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
	ListRepoContentsToolName = "list_repo_contents"
	GetRepoTreeToolName      = "get_repo_tree"
)

var (
	ListRepoContentsTool = mcp.NewTool(
		ListRepoContentsToolName,
		annotate.Read(),
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
		annotate.Read(),
		mcp.WithDescription("Get the Git tree of a repository. With recursive=true, returns the complete file tree in a single response (subject to the server's tree-endpoint size cap); use this when you need all paths at once. With recursive=false (default), returns only the top-level entries of the tree. page and limit (default 1000) select one page of those entries, and the response carries has_next to say whether more entries follow."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("ref", mcp.Required(), mcp.Description(params.Ref)),
		mcp.WithBoolean("recursive", mcp.Description("Return the complete file tree in one response (subject to server cap); default false returns top-level entries only")),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(1000), mcp.Min(1)),
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
	Page      float64 `json:"page"`
	Limit     float64 `json:"limit"`
}

// target is the repository a contents list names. A missing owner or repo is
// refused in Validate, before the credential lookup.
func (a listRepoContentsArgs) target() (owner, repo string, err error) {
	if owner, err = need("owner", a.Owner); err != nil {
		return "", "", err
	}
	if repo, err = need("repo", a.Repo); err != nil {
		return "", "", err
	}
	return owner, repo, nil
}

// paging is the page and limit of the list, refused when either is out of range.
func (a listRepoContentsArgs) paging() (int, int, error) {
	return decode.PagingArgs(a.Page, a.Limit)
}

// Validate refuses a contents list that names no repository or whose page or
// limit is out of range.
func (a listRepoContentsArgs) Validate() error {
	if _, _, err := a.target(); err != nil {
		return err
	}
	_, _, err := a.paging()
	return err
}

func (h handlers) ListRepoContentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.gitService, listRepoContents)(ctx, req)
}

func listRepoContents(ctx context.Context, svc *apprepo.GitService, a listRepoContentsArgs) (*mcp.CallToolResult, error) {
	owner, repo, err := a.target()
	if err != nil {
		return mcpresult.Fail(err)
	}
	page, limit, err := a.paging()
	if err != nil {
		return mcpresult.Fail(err)
	}
	return gitText(contentsPage(svc.ListContents(ctx, owner, repo, a.Ref, a.Path, apprepo.GitPage{
		Page:  page,
		Limit: limit,
	})))
}

// target is the repository and ref a tree read names. A missing one is refused
// in Validate, before the credential lookup.
func (a getRepoTreeArgs) target() (owner, repo, ref string, err error) {
	if owner, err = need("owner", a.Owner); err != nil {
		return "", "", "", err
	}
	if repo, err = need("repo", a.Repo); err != nil {
		return "", "", "", err
	}
	if ref, err = need("ref", a.Ref); err != nil {
		return "", "", "", err
	}
	return owner, repo, ref, nil
}

// paging is the page and limit of the tree read, refused when either is out of range.
func (a getRepoTreeArgs) paging() (int, int, error) {
	return decode.PagingArgs(a.Page, a.Limit)
}

// Validate refuses a tree read that names no repository or ref, or whose page
// or limit is out of range.
func (a getRepoTreeArgs) Validate() error {
	if _, _, _, err := a.target(); err != nil {
		return err
	}
	_, _, err := a.paging()
	return err
}

func (h handlers) GetRepoTreeFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.gitService, getRepoTree)(ctx, req)
}

func getRepoTree(ctx context.Context, svc *apprepo.GitService, a getRepoTreeArgs) (*mcp.CallToolResult, error) {
	owner, repo, ref, err := a.target()
	if err != nil {
		return mcpresult.Fail(err)
	}
	page, limit, err := a.paging()
	if err != nil {
		return mcpresult.Fail(err)
	}
	return gitText(treePage(svc.GetTree(ctx, owner, repo, ref, apprepo.TreeInput{
		Recursive: a.Recursive,
		Page:      apprepo.GitPage{Page: page, Limit: limit},
	})))
}
