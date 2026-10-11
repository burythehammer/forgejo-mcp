package search

import (
	"context"
	"fmt"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	SearchUsersToolName    = "search_users"
	SearchOrgTeamsToolName = "search_org_teams"
	SearchReposToolName    = "search_repos"
)

var (
	SearchUsersTool = mcp.NewTool(
		SearchUsersToolName,
		mcp.WithDescription("Search users"),
		mcp.WithString("keyword", mcp.Description(params.Keyword)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
	)

	SearchOrgTeamsTool = mcp.NewTool(
		SearchOrgTeamsToolName,
		mcp.WithDescription("Search org teams"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("keyword", mcp.Description(params.Keyword)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
	)

	SearchReposTool = mcp.NewTool(
		SearchReposToolName,
		mcp.WithDescription("Search repos"),
		mcp.WithString("keyword", mcp.Description(params.Keyword)),
		mcp.WithString("sort", mcp.Description(params.Sort), mcp.DefaultString("updated")),
		mcp.WithString("order", mcp.Description(params.Order), mcp.DefaultString("desc")),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
	)
)

// handlers serve the search tools from the Forgejo port.
type handlers struct{ forge forge.Factory }

func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(SearchUsersTool, h.SearchUserFn)
	s.AddTool(SearchOrgTeamsTool, h.SearchOrgTeamsFn)
	s.AddTool(SearchReposTool, h.SearchReposFn)
}

func (h handlers) SearchUserFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called SearchUserFn")
	keyword, _ := req.GetArguments()["keyword"].(string)
	page, _ := to.Float64(req.GetArguments()["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(req.GetArguments()["limit"])
	if limit == 0 {
		limit = 100
	}

	opt := forgejo_sdk.SearchUsersOption{
		KeyWord: keyword,
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}

	// Use the correct options type for searching
	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	result, err := port.SearchUsers(ctx, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("search user err: %w", err))
	}
	return to.TextResult(result)
}

func (h handlers) SearchOrgTeamsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Create basic search teams options
	log.Debugf("Called SearchOrgTeamsFn")
	org, ok := req.GetArguments()["org"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("org name is required"))
	}

	keyword, _ := req.GetArguments()["keyword"].(string)
	page, _ := to.Float64(req.GetArguments()["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(req.GetArguments()["limit"])
	if limit == 0 {
		limit = 100
	}

	opt := forgejo_sdk.SearchTeamsOptions{
		Query: keyword,
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}

	// Use the proper options type for search
	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	result, err := port.SearchOrgTeams(ctx, org, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("search org teams err: %w", err))
	}
	return to.TextResult(result)
}

func (h handlers) SearchReposFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called SearchReposFn")
	keyword, _ := req.GetArguments()["keyword"].(string)
	sort, _ := req.GetArguments()["sort"].(string)
	order, _ := req.GetArguments()["order"].(string)
	page, _ := to.Float64(req.GetArguments()["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(req.GetArguments()["limit"])
	if limit == 0 {
		limit = 100
	}

	// Create a proper search options structure
	opt := forgejo_sdk.SearchRepoOptions{
		Keyword: keyword,
		Sort:    sort,
		Order:   order,
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}

	// Call search repos with proper options
	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	result, err := port.SearchRepos(ctx, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("search repos err: %w", err))
	}
	return to.TextResult(result)
}
