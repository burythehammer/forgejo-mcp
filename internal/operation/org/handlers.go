// SPDX-License-Identifier: GPL-3.0-or-later

package org

import (
	"context"
	"errors"
	"fmt"

	apporg "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/org"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/orgadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
)

// handlers serve the org, membership and team tools. They hold the factory
// rather than a port so each call is bound to the credential of its own
// request. Each handler is the MCP adapter of one use case in internal/app/org: decode
// the arguments, check the ones the tool requires, run the use case, render the
// result.
type handlers struct {
	forge forge.Factory
}

// Argument structs, one per tool. `json` names the MCP argument and
// `required:"true"` marks the required ones; mcptest.ArgsMatchSchema keeps
// them in agreement with the tool definitions. A required string is a pointer,
// so that a missing or non-string value reads as absent, as the original type
// assertions read it. The paging numbers and the team_id are pointers for the
// same reason: the original handlers read them only as JSON numbers.

type createOrgArgs struct {
	Name        *string `json:"name" required:"true"`
	FullName    string  `json:"full_name"`
	Description string  `json:"description"`
	Website     string  `json:"website"`
	Location    string  `json:"location"`
	Visibility  string  `json:"visibility"`
}

type orgArgs struct {
	Org *string `json:"org" required:"true"`
}

type myOrgsArgs struct {
	Page  *float64 `json:"page" required:"true"`
	Limit *float64 `json:"limit" required:"true"`
}

type userOrgsArgs struct {
	User  *string  `json:"user" required:"true"`
	Page  *float64 `json:"page" required:"true"`
	Limit *float64 `json:"limit" required:"true"`
}

type editOrgArgs struct {
	Org         *string `json:"org" required:"true"`
	FullName    string  `json:"full_name"`
	Description string  `json:"description"`
	Website     string  `json:"website"`
	Location    string  `json:"location"`
	Visibility  string  `json:"visibility"`
}

type membersArgs struct {
	Org   *string  `json:"org" required:"true"`
	Page  *float64 `json:"page" required:"true"`
	Limit *float64 `json:"limit" required:"true"`
}

type membershipArgs struct {
	Org  *string `json:"org" required:"true"`
	User *string `json:"user" required:"true"`
}

type teamsArgs struct {
	Org   *string  `json:"org" required:"true"`
	Page  *float64 `json:"page" required:"true"`
	Limit *float64 `json:"limit" required:"true"`
}

type createTeamArgs struct {
	Org                     *string `json:"org" required:"true"`
	Name                    *string `json:"name" required:"true"`
	Description             string  `json:"description"`
	Permission              string  `json:"permission"`
	CanCreateOrgRepo        bool    `json:"can_create_org_repo"`
	IncludesAllRepositories bool    `json:"includes_all_repositories"`
}

type teamMemberArgs struct {
	TeamID *float64 `json:"team_id" required:"true"`
	User   *string  `json:"user" required:"true"`
}

type teamRepoArgs struct {
	TeamID *float64 `json:"team_id" required:"true"`
	Org    *string  `json:"org" required:"true"`
	Repo   *string  `json:"repo" required:"true"`
}

// The messages a missing required argument produces. They are the ones the
// original handlers gave, and the tests pin them.
var (
	errOrgName  = errors.New("organization name is required")
	errUserName = errors.New("username is required")
	errTeamName = errors.New("team name is required")
	errTeamID   = errors.New("team_id is required")
	errRepoName = errors.New("repository name is required")
)

// pageOf reads a paging number, defaulting it as the original handlers did:
// an absent page is page 1 and an absent limit is limit 100, while a number
// given is kept as it is, zero included.
func pageOf(p *float64) int {
	if p == nil {
		return 1
	}
	return int(*p)
}

// limitOf passes an absent limit on as 0: the use case applies the default,
// lowered to the instance ceiling.
func limitOf(l *float64) int {
	if l == nil {
		return 0
	}
	return int(*l)
}

func paging(page, limit *float64) apporg.Paging {
	return apporg.Paging{Page: pageOf(page), Limit: limitOf(limit)}
}

// run is the shared shape of a tool call: decode the arguments into A, then
// hand them and the use cases to do. The use cases reach the credential's port
// only after their input is checked (see apporg.Source).
func run[A any](ctx context.Context, h handlers, req mcp.CallToolRequest, name string,
	do func(context.Context, *apporg.Service, A) (*mcp.CallToolResult, error),
) (*mcp.CallToolResult, error) {
	log.Debugf("Called %s", name)
	var args A
	if err := decode.Lax(req, &args); err != nil {
		return mcpresult.Fail(err)
	}
	return do(ctx, apporg.New(h.source()), args)
}

// source returns the lookup of this request's port, which the use cases call
// once their input is checked.
func (h handlers) source() apporg.Source {
	return func(ctx context.Context) (apporg.Port, error) {
		port, err := h.forge.For(ctx)
		if err != nil {
			return nil, err
		}
		return orgadapter.New(port), nil
	}
}

// message sends a use case's outcome as the given text.
func message(text string, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(text)
}

func (h handlers) CreateOrgFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "CreateOrgFn", func(ctx context.Context, s *apporg.Service, a createOrgArgs) (*mcp.CallToolResult, error) {
		if a.Name == nil {
			return mcpresult.Fail(errOrgName)
		}
		return mcpresult.Render(s.CreateOrg(ctx, apporg.CreateOrgInput{
			Name: *a.Name, FullName: a.FullName, Description: a.Description,
			Website: a.Website, Location: a.Location, Visibility: a.Visibility,
		}))
	})
}

func (h handlers) GetOrgFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "GetOrgFn", func(ctx context.Context, s *apporg.Service, a orgArgs) (*mcp.CallToolResult, error) {
		if a.Org == nil {
			return mcpresult.Fail(errOrgName)
		}
		return mcpresult.Render(s.GetOrg(ctx, *a.Org))
	})
}

func (h handlers) ListMyOrgsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListMyOrgsFn", func(ctx context.Context, s *apporg.Service, a myOrgsArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.MyOrgs(ctx, paging(a.Page, a.Limit)))
	})
}

func (h handlers) ListUserOrgsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListUserOrgsFn", func(ctx context.Context, s *apporg.Service, a userOrgsArgs) (*mcp.CallToolResult, error) {
		if a.User == nil {
			return mcpresult.Fail(errUserName)
		}
		return mcpresult.Render(s.UserOrgs(ctx, *a.User, paging(a.Page, a.Limit)))
	})
}

func (h handlers) EditOrgFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "EditOrgFn", func(ctx context.Context, s *apporg.Service, a editOrgArgs) (*mcp.CallToolResult, error) {
		if a.Org == nil {
			return mcpresult.Fail(errOrgName)
		}
		return mcpresult.Render(s.EditOrg(ctx, *a.Org, apporg.EditOrgInput{
			FullName: a.FullName, Description: a.Description, Website: a.Website,
			Location: a.Location, Visibility: a.Visibility,
		}))
	})
}

func (h handlers) DeleteOrgFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "DeleteOrgFn", func(ctx context.Context, s *apporg.Service, a orgArgs) (*mcp.CallToolResult, error) {
		if a.Org == nil {
			return mcpresult.Fail(errOrgName)
		}
		err := s.DeleteOrg(ctx, *a.Org)
		return message(fmt.Sprintf("Organization '%s' deleted successfully", *a.Org), err)
	})
}

func (h handlers) ListOrgMembersFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListOrgMembersFn", func(ctx context.Context, s *apporg.Service, a membersArgs) (*mcp.CallToolResult, error) {
		if a.Org == nil {
			return mcpresult.Fail(errOrgName)
		}
		return mcpresult.Render(s.Members(ctx, *a.Org, paging(a.Page, a.Limit)))
	})
}

func (h handlers) CheckOrgMembershipFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "CheckOrgMembershipFn", func(ctx context.Context, s *apporg.Service, a membershipArgs) (*mcp.CallToolResult, error) {
		if a.Org == nil {
			return mcpresult.Fail(errOrgName)
		}
		if a.User == nil {
			return mcpresult.Fail(errUserName)
		}
		isMember, err := s.IsMember(ctx, *a.Org, *a.User)
		return mcpresult.Render(map[string]interface{}{
			"org":       *a.Org,
			"user":      *a.User,
			"is_member": isMember,
		}, err)
	})
}

func (h handlers) RemoveOrgMemberFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "RemoveOrgMemberFn", func(ctx context.Context, s *apporg.Service, a membershipArgs) (*mcp.CallToolResult, error) {
		if a.Org == nil {
			return mcpresult.Fail(errOrgName)
		}
		if a.User == nil {
			return mcpresult.Fail(errUserName)
		}
		err := s.RemoveMember(ctx, *a.Org, *a.User)
		return message(fmt.Sprintf("User '%s' removed from organization '%s'", *a.User, *a.Org), err)
	})
}

func (h handlers) ListOrgTeamsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListOrgTeamsFn", func(ctx context.Context, s *apporg.Service, a teamsArgs) (*mcp.CallToolResult, error) {
		if a.Org == nil {
			return mcpresult.Fail(errOrgName)
		}
		return mcpresult.Render(s.Teams(ctx, *a.Org, paging(a.Page, a.Limit)))
	})
}

func (h handlers) CreateOrgTeamFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "CreateOrgTeamFn", func(ctx context.Context, s *apporg.Service, a createTeamArgs) (*mcp.CallToolResult, error) {
		if a.Org == nil {
			return mcpresult.Fail(errOrgName)
		}
		if a.Name == nil {
			return mcpresult.Fail(errTeamName)
		}
		return mcpresult.Render(s.CreateTeam(ctx, *a.Org, apporg.CreateTeamInput{
			Name: *a.Name, Description: a.Description, Permission: a.Permission,
			CanCreateOrgRepo: a.CanCreateOrgRepo, IncludesAllRepositories: a.IncludesAllRepositories,
		}))
	})
}

func (h handlers) AddTeamMemberFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "AddTeamMemberFn", func(ctx context.Context, s *apporg.Service, a teamMemberArgs) (*mcp.CallToolResult, error) {
		if a.TeamID == nil {
			return mcpresult.Fail(errTeamID)
		}
		if a.User == nil {
			return mcpresult.Fail(errUserName)
		}
		teamID := int64(*a.TeamID)
		err := s.AddTeamMember(ctx, teamID, *a.User)
		return message(fmt.Sprintf("User '%s' added to team %d", *a.User, teamID), err)
	})
}

func (h handlers) RemoveTeamMemberFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "RemoveTeamMemberFn", func(ctx context.Context, s *apporg.Service, a teamMemberArgs) (*mcp.CallToolResult, error) {
		if a.TeamID == nil {
			return mcpresult.Fail(errTeamID)
		}
		if a.User == nil {
			return mcpresult.Fail(errUserName)
		}
		teamID := int64(*a.TeamID)
		err := s.RemoveTeamMember(ctx, teamID, *a.User)
		return message(fmt.Sprintf("User '%s' removed from team %d", *a.User, teamID), err)
	})
}

func (h handlers) AddTeamRepoFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "AddTeamRepoFn", func(ctx context.Context, s *apporg.Service, a teamRepoArgs) (*mcp.CallToolResult, error) {
		if a.TeamID == nil {
			return mcpresult.Fail(errTeamID)
		}
		if a.Org == nil {
			return mcpresult.Fail(errOrgName)
		}
		if a.Repo == nil {
			return mcpresult.Fail(errRepoName)
		}
		teamID := int64(*a.TeamID)
		err := s.AddTeamRepo(ctx, teamID, *a.Org, *a.Repo)
		return message(fmt.Sprintf("Repository '%s/%s' added to team %d", *a.Org, *a.Repo, teamID), err)
	})
}

func (h handlers) RemoveTeamRepoFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "RemoveTeamRepoFn", func(ctx context.Context, s *apporg.Service, a teamRepoArgs) (*mcp.CallToolResult, error) {
		if a.TeamID == nil {
			return mcpresult.Fail(errTeamID)
		}
		if a.Org == nil {
			return mcpresult.Fail(errOrgName)
		}
		if a.Repo == nil {
			return mcpresult.Fail(errRepoName)
		}
		teamID := int64(*a.TeamID)
		err := s.RemoveTeamRepo(ctx, teamID, *a.Org, *a.Repo)
		return message(fmt.Sprintf("Repository '%s/%s' removed from team %d", *a.Org, *a.Repo, teamID), err)
	})
}
