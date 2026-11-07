// SPDX-License-Identifier: GPL-3.0-or-later

package org

import (
	"context"
	"errors"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	apporg "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/org"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/orgadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/numarg"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
)

// handlers serve the org, membership and team tools. They hold the factory
// rather than a port so each call is bound to the credential of its own
// request. Each tool is the MCP adapter of one use case in internal/app/org:
// the pipeline decodes and validates the arguments, the use case runs, and the
// result is rendered.
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

// paging reads the page and limit arguments through the numeric-argument
// module. An absent argument is read as 0, which the module makes page 1 or the
// use case's default limit; a value that is not a whole number in range is refused.
func paging(page, limit *float64) (apporg.Paging, error) {
	p, l, err := decode.PagingArgs(orZero(page), orZero(limit))
	return apporg.Paging{Page: p, Limit: l}, err
}

// orZero reads an optional number as 0 when it was not sent.
func orZero(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

// service is the Binder for every org tool. The use cases reach the
// credential's port only after their input is checked (see apporg.Source), so
// binding does not look up a credential.
func (h handlers) service(context.Context) (*apporg.Service, error) {
	return apporg.New(h.source()), nil
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

// Validate checks the name before the credential lookup.
func (a createOrgArgs) Validate() error {
	if a.Name == nil {
		return errOrgName
	}
	return nil
}

// Validate checks the organization before the credential lookup.
func (a orgArgs) Validate() error {
	if a.Org == nil {
		return errOrgName
	}
	return nil
}

// Validate checks the paging arguments before the credential lookup.
func (a myOrgsArgs) Validate() error {
	_, err := paging(a.Page, a.Limit)
	return err
}

// Validate checks the user, then the paging arguments, before the credential
// lookup.
func (a userOrgsArgs) Validate() error {
	if a.User == nil {
		return errUserName
	}
	_, err := paging(a.Page, a.Limit)
	return err
}

// Validate checks the organization before the credential lookup.
func (a editOrgArgs) Validate() error {
	if a.Org == nil {
		return errOrgName
	}
	return nil
}

// Validate checks the organization, then the paging arguments, before the
// credential lookup.
func (a membersArgs) Validate() error {
	if a.Org == nil {
		return errOrgName
	}
	_, err := paging(a.Page, a.Limit)
	return err
}

// Validate checks the organization, then the user, before the credential
// lookup.
func (a membershipArgs) Validate() error {
	if a.Org == nil {
		return errOrgName
	}
	if a.User == nil {
		return errUserName
	}
	return nil
}

// Validate checks the organization, then the paging arguments, before the
// credential lookup.
func (a teamsArgs) Validate() error {
	if a.Org == nil {
		return errOrgName
	}
	_, err := paging(a.Page, a.Limit)
	return err
}

// Validate checks the organization, then the team name, before the credential
// lookup.
func (a createTeamArgs) Validate() error {
	if a.Org == nil {
		return errOrgName
	}
	if a.Name == nil {
		return errTeamName
	}
	return nil
}

// Validate checks the team ID, then the user, before the credential lookup.
func (a teamMemberArgs) Validate() error {
	if a.TeamID == nil {
		return errTeamID
	}
	if a.User == nil {
		return errUserName
	}
	return nil
}

// Validate checks the team ID, the organization, then the repository, before
// the credential lookup.
func (a teamRepoArgs) Validate() error {
	if a.TeamID == nil {
		return errTeamID
	}
	if a.Org == nil {
		return errOrgName
	}
	if a.Repo == nil {
		return errRepoName
	}
	return nil
}

func createOrg(ctx context.Context, s *apporg.Service, a createOrgArgs) (*mcp.CallToolResult, error) {
	return mcpresult.Render(s.CreateOrg(ctx, apporg.CreateOrgInput{
		Name: *a.Name, FullName: a.FullName, Description: a.Description,
		Website: a.Website, Location: a.Location, Visibility: a.Visibility,
	}))
}

func getOrg(ctx context.Context, s *apporg.Service, a orgArgs) (*mcp.CallToolResult, error) {
	return mcpresult.Render(s.GetOrg(ctx, *a.Org))
}

func listMyOrgs(ctx context.Context, s *apporg.Service, a myOrgsArgs) (*mcp.CallToolResult, error) {
	pg, err := paging(a.Page, a.Limit)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return orgPage(s.MyOrgs(ctx, pg))
}

func listUserOrgs(ctx context.Context, s *apporg.Service, a userOrgsArgs) (*mcp.CallToolResult, error) {
	pg, err := paging(a.Page, a.Limit)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return orgPage(s.UserOrgs(ctx, *a.User, pg))
}

func editOrg(ctx context.Context, s *apporg.Service, a editOrgArgs) (*mcp.CallToolResult, error) {
	return mcpresult.Render(s.EditOrg(ctx, *a.Org, apporg.EditOrgInput{
		FullName: a.FullName, Description: a.Description, Website: a.Website,
		Location: a.Location, Visibility: a.Visibility,
	}))
}

func deleteOrg(ctx context.Context, s *apporg.Service, a orgArgs) (*mcp.CallToolResult, error) {
	err := s.DeleteOrg(ctx, *a.Org)
	return message(fmt.Sprintf("Organization '%s' deleted successfully", *a.Org), err)
}

func listOrgMembers(ctx context.Context, s *apporg.Service, a membersArgs) (*mcp.CallToolResult, error) {
	pg, err := paging(a.Page, a.Limit)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return memberPage(s.Members(ctx, *a.Org, pg))
}

func checkOrgMembership(ctx context.Context, s *apporg.Service, a membershipArgs) (*mcp.CallToolResult, error) {
	isMember, err := s.IsMember(ctx, *a.Org, *a.User)
	return mcpresult.Render(map[string]interface{}{
		"org":       *a.Org,
		"user":      *a.User,
		"is_member": isMember,
	}, err)
}

func removeOrgMember(ctx context.Context, s *apporg.Service, a membershipArgs) (*mcp.CallToolResult, error) {
	err := s.RemoveMember(ctx, *a.Org, *a.User)
	return message(fmt.Sprintf("User '%s' removed from organization '%s'", *a.User, *a.Org), err)
}

func listOrgTeams(ctx context.Context, s *apporg.Service, a teamsArgs) (*mcp.CallToolResult, error) {
	pg, err := paging(a.Page, a.Limit)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return teamPage(s.Teams(ctx, *a.Org, pg))
}

func createOrgTeam(ctx context.Context, s *apporg.Service, a createTeamArgs) (*mcp.CallToolResult, error) {
	return mcpresult.Render(s.CreateTeam(ctx, *a.Org, apporg.CreateTeamInput{
		Name: *a.Name, Description: a.Description, Permission: a.Permission,
		CanCreateOrgRepo: a.CanCreateOrgRepo, IncludesAllRepositories: a.IncludesAllRepositories,
	}))
}

func addTeamMember(ctx context.Context, s *apporg.Service, a teamMemberArgs) (*mcp.CallToolResult, error) {
	teamID := numarg.Truncate(*a.TeamID)
	err := s.AddTeamMember(ctx, teamID, *a.User)
	return message(fmt.Sprintf("User '%s' added to team %d", *a.User, teamID), err)
}

func removeTeamMember(ctx context.Context, s *apporg.Service, a teamMemberArgs) (*mcp.CallToolResult, error) {
	teamID := numarg.Truncate(*a.TeamID)
	err := s.RemoveTeamMember(ctx, teamID, *a.User)
	return message(fmt.Sprintf("User '%s' removed from team %d", *a.User, teamID), err)
}

func addTeamRepo(ctx context.Context, s *apporg.Service, a teamRepoArgs) (*mcp.CallToolResult, error) {
	teamID := numarg.Truncate(*a.TeamID)
	err := s.AddTeamRepo(ctx, teamID, *a.Org, *a.Repo)
	return message(fmt.Sprintf("Repository '%s/%s' added to team %d", *a.Org, *a.Repo, teamID), err)
}

func removeTeamRepo(ctx context.Context, s *apporg.Service, a teamRepoArgs) (*mcp.CallToolResult, error) {
	teamID := numarg.Truncate(*a.TeamID)
	err := s.RemoveTeamRepo(ctx, teamID, *a.Org, *a.Repo)
	return message(fmt.Sprintf("Repository '%s/%s' removed from team %d", *a.Org, *a.Repo, teamID), err)
}

// orgPage renders one page of organizations in the wire form of the list tools.
func orgPage(b list.Bounded[*model.Organization], err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcpresult.Fail(err)
	}
	return mcpresult.Render(model.OrganizationPage{Organizations: b.Items, PageMeta: model.PageMetaOf(b)}, nil)
}

// memberPage renders one page of organization members in its wire form.
func memberPage(b list.Bounded[*model.User], err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcpresult.Fail(err)
	}
	return mcpresult.Render(model.MemberPage{Members: b.Items, PageMeta: model.PageMetaOf(b)}, nil)
}

// teamPage renders one page of organization teams in its wire form.
func teamPage(b list.Bounded[*model.Team], err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcpresult.Fail(err)
	}
	return mcpresult.Render(model.TeamPage{Teams: b.Items, PageMeta: model.PageMetaOf(b)}, nil)
}
