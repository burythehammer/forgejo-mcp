// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"

	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	CreateRepoToolName  = "create_repo"
	ForkRepoToolName    = "fork_repo"
	ListMyReposToolName = "list_my_repos"
	GetRepoToolName     = "get_repo"
	EditRepoToolName    = "edit_repo"
)

var (
	CreateRepoTool = mcp.NewTool(
		CreateRepoToolName,
		mcp.WithDescription("Create repo"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Repo name")),
		mcp.WithString("description", mcp.Description(params.Description)),
		mcp.WithString("owner", mcp.Description("Owner/org name")),
		mcp.WithBoolean("private", mcp.Description(params.Private)),
		mcp.WithString("issue_labels", mcp.Description("Issue label set")),
		mcp.WithBoolean("auto_init", mcp.Description("Auto-initialize")),
		mcp.WithBoolean("template", mcp.Description("Template repo")),
		mcp.WithString("gitignores", mcp.Description("Gitignore templates")),
		mcp.WithString("license", mcp.Description("License")),
		mcp.WithString("readme", mcp.Description("README content")),
		mcp.WithString("default_branch", mcp.Description("Default branch")),
	)

	ForkRepoTool = mcp.NewTool(
		ForkRepoToolName,
		mcp.WithDescription("Fork repo"),
		mcp.WithString("user", mcp.Required(), mcp.Description(params.User)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("organization", mcp.Description("Org name")),
		mcp.WithString("name", mcp.Description("Fork name")),
	)

	ListMyReposTool = mcp.NewTool(
		ListMyReposToolName,
		mcp.WithDescription("List my repos"),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
	)
)

func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(CreateRepoTool, h.CreateRepoFn)
	s.AddTool(ForkRepoTool, h.ForkRepoFn)
	s.AddTool(ListMyReposTool, h.ListMyReposFn)
	s.AddTool(GetRepoTool, h.GetRepoFn)
	s.AddTool(EditRepoTool, h.EditRepoFn)
	s.AddTool(ListRepoTopicsTool, h.ListRepoTopicsFn)
	s.AddTool(SetRepoTopicsTool, h.SetRepoTopicsFn)
	s.AddTool(AddRepoTopicTool, h.AddRepoTopicFn)
	s.AddTool(DeleteRepoTopicTool, h.DeleteRepoTopicFn)

	// File
	s.AddTool(GetFileContentTool, h.GetFileContentFn)
	s.AddTool(CreateFileTool, h.CreateFileFn)
	s.AddTool(UpdateFileTool, h.UpdateFileFn)
	s.AddTool(DeleteFileTool, h.DeleteFileFn)
	s.AddTool(ChangeFilesTool, h.ChangeFilesFn)

	// Branch
	s.AddTool(CreateBranchTool, h.CreateBranchFn)
	s.AddTool(DeleteBranchTool, h.DeleteBranchFn)
	s.AddTool(ListBranchesTool, h.ListBranchesFn)

	// Commit
	s.AddTool(ListRepoCommitsTool, h.ListRepoCommitsFn)
	s.AddTool(GetCommitStatusesTool, h.GetCommitStatusesFn)

	// Contents / Tree
	s.AddTool(ListRepoContentsTool, h.ListRepoContentsFn)
	s.AddTool(GetRepoTreeTool, h.GetRepoTreeFn)
}

type createRepoArgs struct {
	Name          *string `json:"name" required:"true"`
	Description   string  `json:"description"`
	Owner         string  `json:"owner"`
	Private       bool    `json:"private"`
	IssueLabels   string  `json:"issue_labels"`
	AutoInit      bool    `json:"auto_init"`
	Template      bool    `json:"template"`
	Gitignores    string  `json:"gitignores"`
	License       string  `json:"license"`
	Readme        string  `json:"readme"`
	DefaultBranch string  `json:"default_branch"`
}

func (h handlers) CreateRepoFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return runAdmin(ctx, h, req, "CreateRepoFn", func(ctx context.Context, s *apprepo.AdminService, a createRepoArgs) (*mcp.CallToolResult, error) {
		r, err := s.CreateRepo(ctx, apprepo.CreateRepoInput{
			Owner: a.Owner, Name: a.Name, Description: a.Description, Private: a.Private,
			IssueLabels: a.IssueLabels, AutoInit: a.AutoInit, Template: a.Template,
			Gitignores: a.Gitignores, License: a.License, Readme: a.Readme,
			DefaultBranch: a.DefaultBranch,
		})
		return mcpresult.Render(r, err)
	})
}

type forkRepoArgs struct {
	User         *string `json:"user" required:"true"`
	Repo         *string `json:"repo" required:"true"`
	Organization string  `json:"organization"`
	Name         string  `json:"name"`
}

func (h handlers) ForkRepoFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return runAdmin(ctx, h, req, "ForkRepoFn", func(ctx context.Context, s *apprepo.AdminService, a forkRepoArgs) (*mcp.CallToolResult, error) {
		if err := s.ForkRepo(ctx, apprepo.ForkInput{User: a.User, Repo: a.Repo, Organization: a.Organization, Name: a.Name}); err != nil {
			return mcpresult.Fail(err)
		}
		return to.TextResult("Fork success")
	})
}

type listMyReposArgs struct {
	Page  float64 `json:"page" required:"true"`
	Limit float64 `json:"limit" required:"true"`
}

func (h handlers) ListMyReposFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return runAdmin(ctx, h, req, "ListMyReposFn", func(ctx context.Context, s *apprepo.AdminService, a listMyReposArgs) (*mcp.CallToolResult, error) {
		repos, err := s.ListMyRepos(ctx, apprepo.AdminPaging{Page: decode.PageOr(a.Page, 1), Limit: decode.PageOr(a.Limit, 100)})
		return mcpresult.Render(repos, err)
	})
}
