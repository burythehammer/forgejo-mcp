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

func (h handlers) CreateBranchFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateBranchFn")
	var a createBranchArgs
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
	branch, err := need("branch", a.Branch)
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := h.gitService(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := svc.CreateBranch(ctx, owner, repo, apprepo.CreateBranchInput{Branch: branch, OldBranch: a.OldBranch}); err != nil {
		return to.ErrorResult(err)
	}
	return mcp.NewToolResultText("Branch Created"), nil
}

func (h handlers) DeleteBranchFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteBranchFn")
	var a deleteBranchArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	svc, err := h.gitService(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := svc.DeleteBranch(ctx, a.Owner, a.Repo, a.Branch); err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult("Delete Branch Success")
}

func (h handlers) ListBranchesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListBranchesFn")
	var a listBranchesArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	svc, err := h.gitService(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	return gitText(branchPage(svc.ListBranches(ctx, a.Owner, a.Repo, apprepo.GitPage{Page: int(orDefault(a.Page, 1)), Limit: int(a.Limit)})))
}
