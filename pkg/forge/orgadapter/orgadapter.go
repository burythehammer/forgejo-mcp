// SPDX-License-Identifier: GPL-3.0-or-later

// Package orgadapter implements the organization use cases' port (app/org.Port)
// over the Forgejo port (forge.Orgs). It is the one place that translates
// between repo-owned types and SDK types for the organization domain: inputs
// become SDK options, SDK results become app/model values. Results are
// converted through their JSON form, which is exactly what a tool renders, so a
// field the repo-owned type shares by JSON name cannot drift in value.
package orgadapter

import (
	"context"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
	apporg "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/org"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/bridge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// New returns the use cases' port over o.
// Source is the part of the Forgejo port the organization domain uses: its org
// methods and the instance's pagination ceiling.
type Source interface {
	forge.Orgs
	forge.Ceilings
}

func New(o Source) apporg.Port { return adapter{o: o} }

// MaxResponseItems reads the instance's pagination ceiling through the source.
func (a adapter) MaxResponseItems(ctx context.Context) (int, bool) {
	return a.o.MaxResponseItems(ctx)
}

type adapter struct{ o Source }

var _ apporg.Port = adapter{}

func listOptions(p apporg.Paging) forgejo_sdk.ListOptions {
	return forgejo_sdk.ListOptions{Page: p.Page, PageSize: p.Limit}
}

func (a adapter) CreateOrg(ctx context.Context, in apporg.CreateOrgInput) (*model.Organization, error) {
	org, err := a.o.CreateOrg(ctx, forgejo_sdk.CreateOrgOption{
		Name: in.Name, FullName: in.FullName, Description: in.Description,
		Website: in.Website, Location: in.Location, Visibility: forgejo_sdk.VisibleType(in.Visibility),
	})
	return bridge.Convert[*model.Organization](org, err)
}

func (a adapter) GetOrg(ctx context.Context, org string) (*model.Organization, error) {
	o, err := a.o.GetOrg(ctx, org)
	return bridge.Convert[*model.Organization](o, err)
}

func (a adapter) EditOrg(ctx context.Context, org string, in apporg.EditOrgInput) error {
	return a.o.EditOrg(ctx, org, forgejo_sdk.EditOrgOption{
		FullName: in.FullName, Description: in.Description, Website: in.Website,
		Location: in.Location, Visibility: forgejo_sdk.VisibleType(in.Visibility),
	})
}

func (a adapter) DeleteOrg(ctx context.Context, org string) error {
	return a.o.DeleteOrg(ctx, org)
}

func (a adapter) ListMyOrgs(ctx context.Context, p apporg.Paging) ([]*model.Organization, error) {
	orgs, err := a.o.ListMyOrgs(ctx, forgejo_sdk.ListOrgsOptions{ListOptions: listOptions(p)})
	return bridge.Convert[[]*model.Organization](orgs, err)
}

func (a adapter) ListUserOrgs(ctx context.Context, user string, p apporg.Paging) ([]*model.Organization, error) {
	orgs, err := a.o.ListUserOrgs(ctx, user, forgejo_sdk.ListOrgsOptions{ListOptions: listOptions(p)})
	return bridge.Convert[[]*model.Organization](orgs, err)
}

func (a adapter) ListOrgMembers(ctx context.Context, org string, p apporg.Paging) ([]*model.User, error) {
	members, err := a.o.ListOrgMembers(ctx, org, forgejo_sdk.ListOrgMembershipOption{ListOptions: listOptions(p)})
	return bridge.Convert[[]*model.User](members, err)
}

func (a adapter) CheckOrgMembership(ctx context.Context, org, user string) (bool, error) {
	return a.o.CheckOrgMembership(ctx, org, user)
}

func (a adapter) RemoveOrgMember(ctx context.Context, org, user string) error {
	return a.o.RemoveOrgMember(ctx, org, user)
}

func (a adapter) ListOrgTeams(ctx context.Context, org string, p apporg.Paging) ([]*model.Team, error) {
	teams, err := a.o.ListOrgTeams(ctx, org, forgejo_sdk.ListTeamsOptions{ListOptions: listOptions(p)})
	return bridge.Convert[[]*model.Team](teams, err)
}

func (a adapter) CreateTeam(ctx context.Context, org string, in apporg.CreateTeamInput) (*model.Team, error) {
	team, err := a.o.CreateTeam(ctx, org, forgejo_sdk.CreateTeamOption{
		Name:                    in.Name,
		Description:             in.Description,
		Permission:              forgejo_sdk.AccessMode(in.Permission),
		CanCreateOrgRepo:        in.CanCreateOrgRepo,
		IncludesAllRepositories: in.IncludesAllRepositories,
	})
	return bridge.Convert[*model.Team](team, err)
}

func (a adapter) AddTeamMember(ctx context.Context, teamID int64, user string) error {
	return a.o.AddTeamMember(ctx, teamID, user)
}

func (a adapter) RemoveTeamMember(ctx context.Context, teamID int64, user string) error {
	return a.o.RemoveTeamMember(ctx, teamID, user)
}

func (a adapter) AddTeamRepository(ctx context.Context, teamID int64, org, repo string) error {
	return a.o.AddTeamRepository(ctx, teamID, org, repo)
}

func (a adapter) RemoveTeamRepository(ctx context.Context, teamID int64, org, repo string) error {
	return a.o.RemoveTeamRepository(ctx, teamID, org, repo)
}
