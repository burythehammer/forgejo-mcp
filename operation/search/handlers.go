// SPDX-License-Identifier: GPL-3.0-or-later

package search

import (
	"context"
	"errors"

	appsearch "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/search"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/searchadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/log"

	"github.com/mark3labs/mcp-go/mcp"
)

// handlers serve the search tools. They hold the factory rather than a port so
// each call is bound to the credential of its own request. Each handler is the
// MCP adapter of one use case in app/search: decode the arguments, run the use
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

// paging applies the tools' defaults to the page and limit arguments: zero
// means the default, and a fractional number is truncated.
func paging(page, limit float64) appsearch.Paging {
	if page == 0 {
		page = 1
	}
	// An absent limit is passed on as 0: the use case applies the default,
	// lowered to the instance ceiling.
	return appsearch.Paging{Page: int(page), Limit: int(limit)}
}

// run is the shared shape of a tool call: decode the arguments into A, take
// the use cases bound to this request's credential, and hand both to do.
func run[A any](ctx context.Context, h handlers, req mcp.CallToolRequest, name string,
	do func(context.Context, *appsearch.Service, A) (*mcp.CallToolResult, error),
) (*mcp.CallToolResult, error) {
	log.Debugf("Called %s", name)
	var args A
	if err := decode.Lax(req, &args); err != nil {
		return mcpresult.Fail(err)
	}
	port, err := h.forge.For(ctx)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return do(ctx, appsearch.New(searchadapter.New(port)), args)
}

func (h handlers) SearchUserFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "SearchUserFn", func(ctx context.Context, s *appsearch.Service, a usersArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Users(ctx, appsearch.UserQuery{Keyword: a.Keyword, Paging: paging(a.Page, a.Limit)}))
	})
}

// SearchOrgTeamsFn checks the org argument before the credential is looked up,
// as the original handler did, so a request without an org is refused the same
// way whatever the credential policy says.
func (h handlers) SearchOrgTeamsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called SearchOrgTeamsFn")
	var a orgTeamsArgs
	if err := decode.Lax(req, &a); err != nil {
		return mcpresult.Fail(err)
	}
	if a.Org == nil {
		return mcpresult.Fail(errors.New("org name is required"))
	}
	port, err := h.forge.For(ctx)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return mcpresult.Render(appsearch.New(searchadapter.New(port)).OrgTeams(ctx, *a.Org,
		appsearch.TeamQuery{Keyword: a.Keyword, Paging: paging(a.Page, a.Limit)}))
}

func (h handlers) SearchReposFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "SearchReposFn", func(ctx context.Context, s *appsearch.Service, a reposArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Repos(ctx, appsearch.RepoQuery{
			Keyword: a.Keyword, Sort: a.Sort, Order: a.Order, Paging: paging(a.Page, a.Limit),
		}))
	})
}
