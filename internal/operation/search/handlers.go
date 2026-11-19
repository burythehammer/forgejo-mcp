// SPDX-License-Identifier: GPL-3.0-or-later

package search

import (
	"context"
	"errors"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
	appsearch "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/search"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/searchadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/mcpresult"

	"github.com/mark3labs/mcp-go/mcp"
)

// handlers serve the search tools. They hold the factory rather than a port so
// each call is bound to the credential of its own request. Each handler is the
// MCP adapter of one use case in internal/app/search: decode the arguments, run the use
// case, render the result.
type handlers struct{ forge forge.Factory }

// Argument structs, one per tool. `json` names the MCP argument and
// `required:"true"` marks the required ones; mcptest.ArgsMatchSchema keeps
// them in agreement with the tool definitions. Numbers are float64 so the
// forgiving decoder truncates them as the original handlers did, and org is a
// pointer so a missing or non-string org reads as absent.

type usersArgs struct {
	Keyword string  `json:"keyword"`
	Page    float64 `json:"page"`
	Limit   float64 `json:"limit"`
}

type orgTeamsArgs struct {
	Org     *string `json:"org" required:"true"`
	Keyword string  `json:"keyword"`
	Page    float64 `json:"page"`
	Limit   float64 `json:"limit"`
}

type reposArgs struct {
	Keyword string  `json:"keyword"`
	Sort    string  `json:"sort"`
	Order   string  `json:"order"`
	Page    float64 `json:"page"`
	Limit   float64 `json:"limit"`
}

// paging reads the page and limit arguments through the numeric-argument
// module: a zero page is page 1, an absent limit is 0 for the use case's
// default, and a value that is not a whole number in range is refused.
func paging(page, limit float64) (appsearch.Paging, error) {
	p, l, err := decode.PagingArgs(page, limit)
	return appsearch.Paging{Page: p, Limit: l}, err
}

// service is the Binder for every search tool: it looks up the request's
// credential and binds the search use case to it.
func (h handlers) service(ctx context.Context) (*appsearch.Service, error) {
	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, err
	}
	return appsearch.New(searchadapter.New(port)), nil
}

// Validate checks the paging arguments before the credential lookup.
func (a usersArgs) Validate() error {
	_, err := paging(a.Page, a.Limit)
	return err
}

// Validate checks the org before the credential lookup, as the original handler
// did, so a request without an org is refused the same way whatever the
// credential policy says. Then it checks the paging arguments.
func (a orgTeamsArgs) Validate() error {
	if a.Org == nil {
		return errors.New("org name is required")
	}
	_, err := paging(a.Page, a.Limit)
	return err
}

// Validate checks the paging arguments before the credential lookup.
func (a reposArgs) Validate() error {
	_, err := paging(a.Page, a.Limit)
	return err
}

func searchUsers(ctx context.Context, s *appsearch.Service, a usersArgs) (*mcp.CallToolResult, error) {
	pg, err := paging(a.Page, a.Limit)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return userSearchPage(s.Users(ctx, appsearch.UserQuery{Keyword: a.Keyword, Paging: pg}))
}

func searchOrgTeams(ctx context.Context, s *appsearch.Service, a orgTeamsArgs) (*mcp.CallToolResult, error) {
	pg, err := paging(a.Page, a.Limit)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return teamSearchPage(s.OrgTeams(ctx, *a.Org, appsearch.TeamQuery{Keyword: a.Keyword, Paging: pg}))
}

func searchRepos(ctx context.Context, s *appsearch.Service, a reposArgs) (*mcp.CallToolResult, error) {
	pg, err := paging(a.Page, a.Limit)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return repositoryPage(s.Repos(ctx, appsearch.RepoQuery{
		Keyword: a.Keyword, Sort: a.Sort, Order: a.Order, Paging: pg,
	}))
}

// userSearchPage renders one page of matching users in its wire form.
func userSearchPage(b list.Bounded[*model.User], err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcpresult.Fail(err)
	}
	return mcpresult.Render(model.UserSearchPage{Users: b.Items, PageMeta: model.PageMetaOf(b)}, nil)
}

// teamSearchPage renders one page of matching teams in its wire form.
func teamSearchPage(b list.Bounded[*model.Team], err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcpresult.Fail(err)
	}
	return mcpresult.Render(model.TeamSearchPage{Teams: b.Items, PageMeta: model.PageMetaOf(b)}, nil)
}

// repositoryPage renders one page of matching repositories in its wire form.
func repositoryPage(b list.Bounded[*model.Repository], err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcpresult.Fail(err)
	}
	return mcpresult.Render(model.RepositoryPage{Repositories: b.Items, PageMeta: model.PageMetaOf(b)}, nil)
}
