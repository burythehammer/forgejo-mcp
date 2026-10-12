// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"

	apprepo "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/repo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcpresult"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"

	"github.com/mark3labs/mcp-go/mcp"
)

var (
	GetRepoTool = mcp.NewTool(
		GetRepoToolName,
		mcp.WithDescription("Get a single repository by owner and name."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	)

	EditRepoTool = mcp.NewTool(
		EditRepoToolName,
		mcp.WithDescription("Edit repository settings (PATCH — only supplied fields change). Providing no fields is an error. name renames the repository; subsequent calls must use the new name. Does not change topics (those use a separate API)."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("name", mcp.Description("New repository name")),
		mcp.WithString("description", mcp.Description(params.Description)),
		mcp.WithString("website", mcp.Description(params.Website)),
		mcp.WithString("default_branch", mcp.Description("Default branch")),
		mcp.WithBoolean("private", mcp.Description(params.Private)),
		mcp.WithBoolean("template", mcp.Description("Template repo")),
		mcp.WithBoolean("archived", mcp.Description("Archive the repository")),
		mcp.WithBoolean("has_issues", mcp.Description("Enable the issues unit")),
		mcp.WithBoolean("has_wiki", mcp.Description("Enable the wiki unit")),
		mcp.WithBoolean("has_pull_requests", mcp.Description("Enable pull requests")),
		mcp.WithBoolean("has_projects", mcp.Description("Enable the projects unit")),
		mcp.WithBoolean("has_releases", mcp.Description("Enable releases")),
		mcp.WithBoolean("has_packages", mcp.Description("Enable packages")),
		mcp.WithBoolean("has_actions", mcp.Description("Enable Actions")),
	)
)

type getRepoArgs struct {
	Owner string `json:"owner" required:"true"`
	Repo  string `json:"repo" required:"true"`
}

func (h handlers) GetRepoFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return runAdmin(ctx, h, req, "GetRepoFn", func(ctx context.Context, s *apprepo.AdminService, a getRepoArgs) (*mcp.CallToolResult, error) {
		r, err := s.GetRepo(ctx, a.Owner, a.Repo)
		return mcpresult.Render(r, err)
	})
}

// editRepoArgs leaves a setting nil when it was not given (or was given with
// the wrong type), so the use case can tell "not given" from "given as false"
// or "given as empty".
type editRepoArgs struct {
	Owner           string  `json:"owner" required:"true"`
	Repo            string  `json:"repo" required:"true"`
	Name            *string `json:"name"`
	Description     *string `json:"description"`
	Website         *string `json:"website"`
	DefaultBranch   *string `json:"default_branch"`
	Private         *bool   `json:"private"`
	Template        *bool   `json:"template"`
	Archived        *bool   `json:"archived"`
	HasIssues       *bool   `json:"has_issues"`
	HasWiki         *bool   `json:"has_wiki"`
	HasPullRequests *bool   `json:"has_pull_requests"`
	HasProjects     *bool   `json:"has_projects"`
	HasReleases     *bool   `json:"has_releases"`
	HasPackages     *bool   `json:"has_packages"`
	HasActions      *bool   `json:"has_actions"`
}

func (h handlers) EditRepoFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return runAdmin(ctx, h, req, "EditRepoFn", func(ctx context.Context, s *apprepo.AdminService, a editRepoArgs) (*mcp.CallToolResult, error) {
		r, err := s.EditRepo(ctx, apprepo.EditRepoInput{Owner: a.Owner, Repo: a.Repo, Edit: apprepo.RepoEdit{
			Name: a.Name, Description: a.Description, Website: a.Website, DefaultBranch: a.DefaultBranch,
			Private: a.Private, Template: a.Template, Archived: a.Archived,
			HasIssues: a.HasIssues, HasWiki: a.HasWiki, HasPullRequests: a.HasPullRequests,
			HasProjects: a.HasProjects, HasReleases: a.HasReleases, HasPackages: a.HasPackages,
			HasActions: a.HasActions,
		}})
		return mcpresult.Render(r, err)
	})
}
