// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"net/http"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Orgs = (*Adapter)(nil)

func (a *Adapter) CreateOrg(_ context.Context, opt forgejo_sdk.CreateOrgOption) (*forgejo_sdk.Organization, error) {
	org, resp, err := a.client.CreateOrg(opt)
	return org, statusErr(resp, err)
}

func (a *Adapter) GetOrg(_ context.Context, name string) (*forgejo_sdk.Organization, error) {
	org, resp, err := a.client.GetOrg(name)
	return org, statusErr(resp, err)
}

func (a *Adapter) EditOrg(_ context.Context, name string, opt forgejo_sdk.EditOrgOption) error {
	resp, err := a.client.EditOrg(name, opt)
	return statusErr(resp, err)
}

func (a *Adapter) DeleteOrg(_ context.Context, name string) error {
	resp, err := a.client.DeleteOrg(name)
	return statusErr(resp, err)
}

func (a *Adapter) ListMyOrgs(_ context.Context, opt forgejo_sdk.ListOrgsOptions) ([]*forgejo_sdk.Organization, error) {
	orgs, resp, err := a.client.ListMyOrgs(opt)
	return orgs, statusErr(resp, err)
}

func (a *Adapter) ListUserOrgs(_ context.Context, user string, opt forgejo_sdk.ListOrgsOptions) ([]*forgejo_sdk.Organization, error) {
	orgs, resp, err := a.client.ListUserOrgs(user, opt)
	return orgs, statusErr(resp, err)
}

func (a *Adapter) ListOrgMembers(_ context.Context, org string, opt forgejo_sdk.ListOrgMembershipOption) ([]*forgejo_sdk.User, error) {
	users, resp, err := a.client.ListOrgMembership(org, opt)
	return users, statusErr(resp, err)
}

// CheckOrgMembership is raw so that a refusal keeps Forgejo's message: a 404 is
// "not a member" when the org exists and not found when it does not (see
// notFoundOr), and any other status that is not 204 is a failure.
func (a *Adapter) CheckOrgMembership(ctx context.Context, org, user string) (bool, error) {
	path, err := forge.APIPath("orgs", org, "members", user)
	if err != nil {
		return false, err
	}
	status, body, err := a.raw.doStatus(ctx, http.MethodGet, path, nil)
	if err != nil {
		return false, err
	}
	if status == http.StatusNotFound {
		// "Not a member" only when the org exists; a missing org is not found.
		_, probe := a.GetOrg(ctx, org)
		return false, notFoundOr(probe, orgNotFound(org))
	}
	if err := answered(status, body, http.StatusNoContent); err != nil {
		return false, err
	}
	return true, nil
}

func (a *Adapter) RemoveOrgMember(_ context.Context, org, user string) error {
	resp, err := a.client.DeleteOrgMembership(org, user)
	return statusErr(resp, err)
}

func (a *Adapter) ListOrgTeams(_ context.Context, org string, opt forgejo_sdk.ListTeamsOptions) ([]*forgejo_sdk.Team, error) {
	teams, resp, err := a.client.ListOrgTeams(org, opt)
	return teams, statusErr(resp, err)
}

func (a *Adapter) CreateTeam(_ context.Context, org string, opt forgejo_sdk.CreateTeamOption) (*forgejo_sdk.Team, error) {
	team, resp, err := a.client.CreateTeam(org, opt)
	return team, statusErr(resp, err)
}

func (a *Adapter) AddTeamMember(_ context.Context, teamID int64, user string) error {
	resp, err := a.client.AddTeamMember(teamID, user)
	return statusErr(resp, err)
}

func (a *Adapter) RemoveTeamMember(_ context.Context, teamID int64, user string) error {
	resp, err := a.client.RemoveTeamMember(teamID, user)
	return statusErr(resp, err)
}

func (a *Adapter) AddTeamRepository(_ context.Context, teamID int64, org, repo string) error {
	resp, err := a.client.AddTeamRepository(teamID, org, repo)
	return statusErr(resp, err)
}

func (a *Adapter) RemoveTeamRepository(_ context.Context, teamID int64, org, repo string) error {
	resp, err := a.client.RemoveTeamRepository(teamID, org, repo)
	return statusErr(resp, err)
}
