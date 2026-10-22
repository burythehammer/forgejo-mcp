// SPDX-License-Identifier: GPL-3.0-or-later

package search

import (
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"

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
		annotate.Read(),
		mcp.WithDescription("Search users. Returns a response envelope {users, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("keyword", mcp.Description(params.Keyword)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
	)

	SearchOrgTeamsTool = mcp.NewTool(
		SearchOrgTeamsToolName,
		annotate.Read(),
		mcp.WithDescription("Search org teams. Returns a response envelope {teams, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("keyword", mcp.Description(params.Keyword)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
	)

	SearchReposTool = mcp.NewTool(
		SearchReposToolName,
		annotate.Read(),
		mcp.WithDescription("Search repos. Returns a response envelope {repositories, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("keyword", mcp.Description(params.Keyword)),
		mcp.WithString("sort", mcp.Description(params.Sort), mcp.DefaultString("updated")),
		mcp.WithString("order", mcp.Description(params.Order), mcp.DefaultString("desc")),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
	)
)

// RegisterTool registers the search tools. Each call takes its Forgejo port
// from f, so a request is served with the credential that request carries.
func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(SearchUsersTool, h.SearchUserFn)
	s.AddTool(SearchOrgTeamsTool, h.SearchOrgTeamsFn)
	s.AddTool(SearchReposTool, h.SearchReposFn)
}
