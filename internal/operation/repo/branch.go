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
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	CreateBranchToolName = "create_branch"
	DeleteBranchToolName = "delete_branch"
	ListBranchesToolName = "list_branches"
)

var (
	CreateBranchTool = mcp.NewTool(
		CreateBranchToolName,
		annotate.Write(),
		mcp.WithDescription("Create branch"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("branch", mcp.Required(), mcp.Description(params.Branch)),
		mcp.WithString("old_branch", mcp.Required(), mcp.Description(params.OldBranch)),
	)

	DeleteBranchTool = mcp.NewTool(
		DeleteBranchToolName,
		annotate.Destructive(),
		mcp.WithDescription("Delete branch"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("branch", mcp.Required(), mcp.Description(params.Branch)),
	)

	ListBranchesTool = mcp.NewTool(
		ListBranchesToolName,
		annotate.Read(),
		mcp.WithDescription("List branches. Returns a response envelope {branches, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
	)
)

// Argument structs for the branch tools. The json names are the MCP arguments;
// mcptest.ArgsMatchSchema keeps them in agreement with the tool definitions.

type createBranchArgs struct {
	Owner     *string `json:"owner" required:"true"`
	Repo      *string `json:"repo" required:"true"`
	Branch    *string `json:"branch" required:"true"`
	OldBranch string  `json:"old_branch" required:"true"`
}

type deleteBranchArgs struct {
	Owner  string `json:"owner" required:"true"`
	Repo   string `json:"repo" required:"true"`
	Branch string `json:"branch" required:"true"`
}

type listBranchesArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Page  float64 `json:"page" required:"true"`
	Limit float64 `json:"limit" required:"true"`
}

// target is the repository and new branch a create names. A missing one is
// refused in Validate, before the credential lookup.
func (a createBranchArgs) target() (owner, repo, branch string, err error) {
	if owner, err = need("owner", a.Owner); err != nil {
		return "", "", "", err
	}
	if repo, err = need("repo", a.Repo); err != nil {
		return "", "", "", err
	}
	if branch, err = need("branch", a.Branch); err != nil {
		return "", "", "", err
	}
	return owner, repo, branch, nil
}

// Validate refuses a create that names no repository or branch.
func (a createBranchArgs) Validate() error {
	_, _, _, err := a.target()
	return err
}

func (h handlers) CreateBranchFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.gitService, createBranch)(ctx, req)
}

func createBranch(ctx context.Context, svc *apprepo.GitService, a createBranchArgs) (*mcp.CallToolResult, error) {
	owner, repo, branch, err := a.target()
	if err != nil {
		return mcpresult.Fail(err)
	}
	if err := svc.CreateBranch(ctx, owner, repo, apprepo.CreateBranchInput{Branch: branch, OldBranch: a.OldBranch}); err != nil {
		return to.ErrorResult(err)
	}
	return mcp.NewToolResultText("Branch Created"), nil
}

func (h handlers) DeleteBranchFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.gitService, deleteBranch)(ctx, req)
}

func deleteBranch(ctx context.Context, svc *apprepo.GitService, a deleteBranchArgs) (*mcp.CallToolResult, error) {
	if err := svc.DeleteBranch(ctx, a.Owner, a.Repo, a.Branch); err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult("Delete Branch Success")
}

// paging is the page and limit of the branch list, refused when either is out of range.
func (a listBranchesArgs) paging() (int, int, error) {
	return decode.PagingArgs(a.Page, a.Limit)
}

func (h handlers) ListBranchesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.gitService, listBranches)(ctx, req)
}

func listBranches(ctx context.Context, svc *apprepo.GitService, a listBranchesArgs) (*mcp.CallToolResult, error) {
	page, limit, err := a.paging()
	if err != nil {
		return mcpresult.Fail(err)
	}
	return gitText(branchPage(svc.ListBranches(ctx, a.Owner, a.Repo, apprepo.GitPage{Page: page, Limit: limit})))
}

// Validate refuses a branch list that names no repository or whose page or limit is out of range.
func (a listBranchesArgs) Validate() error {
	if err := firstError(requireString("owner", a.Owner), requireString("repo", a.Repo)); err != nil {
		return err
	}
	_, _, err := a.paging()
	return err
}

// Validate refuses a delete that names no repository or branch.
func (a deleteBranchArgs) Validate() error {
	return firstError(requireString("owner", a.Owner), requireString("repo", a.Repo), requireString("branch", a.Branch))
}
