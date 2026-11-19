// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Search = (*Adapter)(nil)

func (a *Adapter) SearchUsers(_ context.Context, opt forgejo_sdk.SearchUsersOption) ([]*forgejo_sdk.User, error) {
	users, resp, err := a.client.SearchUsers(opt)
	return users, statusErr(resp, err)
}

func (a *Adapter) SearchOrgTeams(_ context.Context, org string, opt forgejo_sdk.SearchTeamsOptions) ([]*forgejo_sdk.Team, error) {
	teams, resp, err := a.client.SearchOrgTeams(org, &opt)
	return teams, statusErr(resp, err)
}

func (a *Adapter) SearchRepos(_ context.Context, opt forgejo_sdk.SearchRepoOptions) ([]*forgejo_sdk.Repository, error) {
	repos, resp, err := a.client.SearchRepos(opt)
	return repos, statusErr(resp, err)
}
